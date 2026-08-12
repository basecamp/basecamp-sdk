package eventfeed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// FileCheckpointStore is the one built-in CheckpointStore: a single JSON file
// holding every lineage's durable position, keyed by CheckpointKey.FlatKey —
// the compact RFC 8259 JSON array of the four identity strings (SPEC.md §23
// "Checkpoint Identity"), e.g.
//
//	{
//	  "[\"https://3.basecampapi.com\",\"5951425\",\"openclaw\",\"srv1-9f2ab04e5c11d3a7\"]": "…"
//	}
//
// # One file, not one file per key
//
// The flat key carries a caller-configured origin and consumer namespace, so
// deriving a filename from it would mean sanitizing or hashing attacker-shaped
// text into a path. Keeping every lineage inside one JSON object obviates that
// question rather than armoring against it: keys are map keys, never path
// components, so no origin or namespace — however crafted, with separators,
// "..", NUL bytes, or absurd length — can name a file, escape the store's
// directory, or collide with another lineage under a sanitizer.
//
// # Durability
//
// Writes are temp-file-plus-rename within the store's own directory, so a
// concurrent reader sees either the complete previous file or the complete new
// one, never a torn write, and a crash mid-write leaves the previous file
// intact. There is no fsync: a checkpoint is advisory — the connector's
// in-memory position is authoritative within a run and the store is
// write-through durability only — so a position lost to an OS-level crash
// costs a re-entry at the present or at an older position, never correctness.
// The file is written 0600 in a directory created 0700 on demand; a checkpoint
// is not a secret, but this file commonly sits beside token caches and there is
// no reason for it to be wider than they are.
//
// # Concurrency
//
// Safe for concurrent use by any number of goroutines in one process. It is
// deliberately NOT safe across processes: there is no advisory locking, so two
// processes sharing one file can lose an update (last writer wins for the
// lineages it holds). SPEC.md §23 documents the file store as single-process; a
// server-side advisory checkpoint API is deferred until a multi-host connector
// needs a shared cursor. Within the connector's own usage this is moot — one
// writer per key per run.
//
// # No delete
//
// There is no delete method, matching the seam: after a 409 the connector
// re-enters via since= and the next page's Save overwrites under the same key,
// and after a filter change the key itself changed, so the old lineage simply
// goes cold.
type FileCheckpointStore struct {
	// path is the JSON file itself, not a directory.
	path string
	// mu serializes Save's read-modify-write against itself and against Load.
	mu sync.RWMutex
}

// FileCheckpointStore fills the CheckpointStore seam.
var _ CheckpointStore = (*FileCheckpointStore)(nil)

// NewFileCheckpointStore returns a store backed by the JSON file at path. The
// file and its parent directories are created on the first successful Save;
// constructing a store touches the filesystem not at all, so a path that does
// not yet exist is normal and loads as Missing.
func NewFileCheckpointStore(path string) *FileCheckpointStore {
	return &FileCheckpointStore{path: path}
}

// Load returns the stored position for key. The tri-state contract maps onto
// the filesystem as:
//
//	file absent                       → Missing  ("", false, nil)
//	file present, no entry for key    → Missing  ("", false, nil)
//	file present, entry for key       → Loaded   (position, true, nil)
//	file unreadable (permissions, …)  → Failed   (non-nil error)
//	file unparseable or not an object → Failed   (non-nil error)
//	entry present but an empty string → Failed   (non-nil error)
//
// The last three are deliberately not collapsed to Missing. Missing proceeds to
// a present-class entry, so reporting a corrupt or unreadable file as Missing
// would silently start the feed at the present and skip history; Failed is
// Terminal(checkpoint_load) with zero wire attempts, which is the honest
// outcome. An empty stored position is Failed for the same reason: seeding the
// connector with "" is indistinguishable from having no position at all.
//
// ctx is accepted for the seam's signature and deliberately not honored: the
// read is a single local file open, and failing it on an already-canceled
// context would turn a shutdown race into Terminal(checkpoint_load).
func (s *FileCheckpointStore) Load(_ context.Context, key CheckpointKey) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, present, err := s.read()
	if err != nil {
		return "", false, err
	}
	if !present {
		return "", false, nil
	}
	position, ok := entries[key.FlatKey()]
	switch {
	case !ok:
		return "", false, nil
	case position == "":
		return "", false, fmt.Errorf(
			"eventfeed: checkpoint store %s holds an empty position for this lineage; "+
				"an empty position cannot be told apart from having none, so it is a store failure, not a missing checkpoint",
			s.path)
	default:
		return position, true, nil
	}
}

// Save durably records position under key, replacing any previous value for
// that lineage and leaving every other lineage in the file untouched.
//
// A Save over a file that cannot be parsed fails rather than starting fresh: a
// silent rewrite would erase every other consumer's lineage in the same file,
// and unlike a failed save — which the connector reports through
// Observer.CheckpointSaveFailed and continues past — that loss is unrecoverable
// and unreported. Recovery from a corrupt file is the operator's: delete it.
//
// ctx is accepted for the seam's signature and deliberately not honored, so
// that a position already accepted by the connector is not dropped because the
// run's context was canceled between acceptance and write-through.
func (s *FileCheckpointStore) Save(_ context.Context, key CheckpointKey, position string) error {
	if position == "" {
		return fmt.Errorf(
			"eventfeed: refusing to save an empty checkpoint position for %s; "+
				"Load would have to report it as a store failure",
			s.path)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, present, err := s.read()
	if err != nil {
		return err
	}
	if !present {
		entries = make(map[string]string, 1)
	}
	entries[key.FlatKey()] = position

	// encoding/json sorts map keys, so the file is stable across saves and
	// diffable; the indentation is for the operator who has to read or prune it.
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("eventfeed: encoding checkpoint store %s: %w", s.path, err)
	}
	return s.writeAtomic(append(data, '\n'))
}

// read loads the whole file. It reports (entries, true, nil) when the file
// exists and parses, (nil, false, nil) when it does not exist — the only
// Missing-shaped filesystem condition — and an error for everything else.
//
// Errors name the path and the parse failure only; the file's contents are
// never echoed, since a corrupt store holds arbitrary bytes.
func (s *FileCheckpointStore) read() (map[string]string, bool, error) {
	data, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("eventfeed: reading checkpoint store %s: %w", s.path, err)
	}

	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s is not a JSON object of positions: %w", s.path, err)
	}
	if entries == nil {
		// JSON `null` unmarshals into a nil map without error. A store that
		// holds `null` is corrupt, not empty — `{}` is empty.
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s holds JSON null, not an object of positions", s.path)
	}
	return entries, true, nil
}

// writeAtomic replaces the store file with data via a temp file in the same
// directory plus a rename, which is atomic within a filesystem. The temp file
// is uniquely named so that a concurrent writer — another process, since this
// store's own writes are serialized — cannot corrupt this one's staging file,
// and it is removed on every failure path so a failed save leaves no debris
// beside the store.
func (s *FileCheckpointStore) writeAtomic(data []byte) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("eventfeed: creating checkpoint store directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("eventfeed: staging a checkpoint store write in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()
	// After a successful rename this removes nothing; on every failure path it
	// is the cleanup that keeps the directory to a single file.
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("eventfeed: writing the staged checkpoint store for %s: %w", s.path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("eventfeed: closing the staged checkpoint store for %s: %w", s.path, err)
	}
	// os.CreateTemp already creates at 0600, but that is subject to the
	// process umask; the mode is part of the contract, so set it outright.
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("eventfeed: setting mode on the staged checkpoint store for %s: %w", s.path, err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("eventfeed: replacing checkpoint store %s: %w", s.path, err)
	}
	return nil
}
