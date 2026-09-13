package eventfeed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
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
// one, never a torn write — and the staged file is fsynced before the rename,
// the directory after it, because without that pair the claim above is false
// on real filesystems: a crash can persist the rename's metadata before the
// staged data blocks, leaving a zero-length or garbage file where BOTH
// versions used to be. That outcome is not the priced one. A position lost to
// a crash costs a re-entry at an older position or the present — the store is
// write-through and advisory, the in-memory position authoritative within a
// run — but a TORN file fails the next Load, which is Terminal(checkpoint_load)
// with zero wire attempts: a feed that will not start, over a file whose whole
// job was surviving the crash. The cost is two fsyncs per accepted poll page.
// The file is written 0600 in a directory created 0700 on demand; a checkpoint
// is not a secret, but this file commonly sits beside token caches and there is
// no reason for it to be wider than they are.
//
// # Concurrency
//
// Safe for concurrent use by any number of goroutines in one process, and by
// any number of store instances over one path: the lock that serializes the
// read-modify-write belongs to the file — every store constructed over one
// canonical path shares it — not to the instance. One connector is one writer
// per key per run, so what the shared lock is for is the setup this store is
// shaped for: two connectors on different filter lineages sharing one
// checkpoint file, each with its own store. Under a per-instance lock both
// would read the same old object and rename a different single-lineage update
// into place, silently dropping one lineage's cursor.
//
// It is deliberately NOT safe across processes: there is no advisory locking,
// so two processes sharing one file can lose an update (last writer wins for
// the lineages it holds). SPEC.md §23 documents the file store as
// single-process; a server-side advisory checkpoint API is deferred until a
// multi-host connector needs a shared cursor.
//
// # No delete
//
// There is no delete method, matching the seam: after a 409 the connector
// re-enters via since= and the next page's Save overwrites under the same key,
// and after a filter change the key itself changed, so the old lineage simply
// goes cold.
type FileCheckpointStore struct {
	// path is the JSON file itself, not a directory: the canonical spelling
	// the lock is keyed on, so the file this store operates on and the file
	// its lock protects cannot drift apart — see canonicalStorePath.
	path string
	// mu serializes Save's read-modify-write against itself and against Load.
	// It is the lock for the canonical path, shared with every other store
	// instance over the same file, not a lock of this instance's own.
	mu *sync.RWMutex
	// pathErr, when set, is the construction-time path resolution failure;
	// every Load and Save reports it instead of touching the filesystem.
	pathErr error
}

// FileCheckpointStore fills the CheckpointStore seam.
var _ CheckpointStore = (*FileCheckpointStore)(nil)

// maxCheckpointStoreBytes bounds a single read of the store file.
//
// The file is one JSON object of {FlatKey: position}. A flat key is the four
// identity strings — origin, account, consumer namespace, filter key — so a
// generous entry is a few hundred bytes, and 8 MiB is room for tens of
// thousands of lineages in one file. A store past that is not a store this
// code wrote; refusing it is a bounded, reported failure
// (Terminal(checkpoint_load) with zero wire attempts) instead of an
// unbounded allocation on a path that runs before the first mint.
const maxCheckpointStoreBytes = 8 << 20

// NewFileCheckpointStore returns a store backed by the JSON file at path. The
// file and its parent directories are created on the first successful Save;
// constructing a store touches the filesystem not at all, so a path that does
// not yet exist is normal and loads as Missing.
//
// A relative path is resolved once, here, against the working directory at
// construction — the same resolution the lock is keyed on. Keeping the two
// together is the point: were the store to hold the relative spelling and
// re-resolve it at every write, a process that later changed directory would
// have it writing a different file while holding the original file's lock,
// and a store constructed for that new file would take a different lock —
// two unserialized writers over one file, which is precisely the lost update
// the per-path lock exists to prevent. Errors therefore name the resolved
// path rather than the spelling the caller passed.
func NewFileCheckpointStore(path string) *FileCheckpointStore {
	canonical, err := canonicalStorePath(path)
	if err != nil {
		// Fail on use, not silently on a drifting spelling: see
		// canonicalStorePath. The mutex is private — this store never
		// touches the file, so it shares serialization with nothing.
		return &FileCheckpointStore{path: filepath.Clean(path), pathErr: err, mu: new(sync.RWMutex)}
	}
	return &FileCheckpointStore{path: canonical, mu: pathLock(canonical)}
}

// pathLocks holds one lock per canonical store path, shared by every store
// instance over that path.
//
// Entries are never evicted, and that is the intended shape: a process holds
// one entry — a path string and a mutex — per distinct checkpoint file it
// opens, and store paths come from a host's configuration, never from feed
// data, so the set is bounded by how many checkpoint files the host names.
// Eviction would need refcounting and would reintroduce the very lost update
// the registry exists to prevent.
var (
	pathLocksMu sync.Mutex
	pathLocks   = map[string]*sync.RWMutex{}
)

// pathLock returns the lock serializing every store over path's canonical
// spelling, creating it on first use.
func pathLock(path string) *sync.RWMutex {
	key := storeLockKey(path)

	pathLocksMu.Lock()
	defer pathLocksMu.Unlock()
	lock, ok := pathLocks[key]
	if !ok {
		lock = new(sync.RWMutex)
		pathLocks[key] = lock
	}
	return lock
}

// storeLockKey is the lock-registry key for path: the canonical path, CASE
// FOLDED. It is deliberately coarser than the path the store reads and writes
// (#761).
//
// The registry's whole job is that one file takes one mutex. On a
// case-insensitive filesystem — APFS as macOS ships it, and NTFS — "feed.json"
// and "Feed.json" ARE one file, and keying the registry on the exact spelling
// handed them two mutexes: two unserialized read-modify-write cycles over one
// file, which is the lost update the registry exists to prevent, reachable by
// nothing more exotic than two call sites disagreeing about capitalization.
//
// Folding unconditionally rather than detecting the filesystem is deliberate.
// Detection would have to probe a directory that need not exist yet (the file
// is created on the first Save) and would answer per-mount, so it is both
// unreliable and far more machinery than the thing it saves. What folding
// costs on a case-sensitive filesystem is that two genuinely distinct files
// differing only in case share one mutex — they serialize with each other
// instead of running concurrently. The two error directions are not
// comparable: over-serializing costs a few microseconds on a local write that
// is already serialized against itself, and under-serializing silently drops a
// lineage's cursor.
//
// Case folding only, and SIMPLE folding at that. strings.ToLower was the
// first cut and missed the orbits lowercasing cannot see — ſ (U+017F)
// case-folds together with S and s on APFS/NTFS while ToLower leaves it
// alone, so two spellings of one physical file took two mutexes. Each rune
// now maps to the minimum of its unicode.SimpleFold orbit, which covers
// every one-rune fold. The honest edges that remain: full-fold multi-rune
// expansions (ß against ss) and Unicode normalization are real further axes
// of APFS equivalence that this deliberately does not chase — byte-identity
// of the configured spelling is the documented guarantee, and an alias this
// folding cannot see degrades to the documented cross-process case: last
// writer wins for the lineages it holds.
func storeLockKey(path string) string {
	canonical, err := canonicalStorePath(path)
	if err != nil {
		canonical = filepath.Clean(path)
	}
	return strings.Map(minSimpleFold, canonical)
}

// minSimpleFold maps r to the smallest rune in its unicode.SimpleFold orbit —
// one representative per simple case-fold equivalence class.
func minSimpleFold(r rune) rune {
	least := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		if f < least {
			least = f
		}
	}
	return least
}

// canonicalStorePath is a store file's lock identity: the cleaned absolute
// path, so "state/checkpoints.json", "./state/checkpoints.json" and
// "<cwd>/state/../state/checkpoints.json" all name one lock. When the working
// directory cannot be read — the only way filepath.Abs fails — the error is
// the answer: a relative spelling retained here would name a DIFFERENT file
// after every later chdir while keeping the old spelling's lock, unserialized
// against a store constructed for the new file — the identity-split class.
// The store constructed over such a path fails on use instead.
//
// Symlinks are deliberately not resolved. Resolving them requires the file to
// exist, and this store's file is created on the first Save, so a resolved key
// would change identity mid-life and two stores constructed either side of
// that first Save would take different locks — strictly worse than a stable
// spelling. Two spellings that alias one file only through a symlinked
// ancestor therefore behave like the documented cross-process case: last
// writer wins for the lineages it holds.
func canonicalStorePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("eventfeed: resolving checkpoint store path %q: %w", path, err)
	}
	return absolute, nil
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
	// The read-side sibling of Save's input gate: FlatKey encodes an invalid
	// component to U+FFFD silently, so an invalid key would MATCH a lineage
	// legitimately saved under the replacement rune — another consumer's
	// cursor returned for a key that was never valid. Usage, before any
	// lookup.
	if s.pathErr != nil {
		return "", false, s.pathErr
	}
	for _, in := range []struct{ field, value string }{
		{"checkpoint origin", key.Origin},
		{"checkpoint account id", key.AccountID},
		{"checkpoint consumer namespace", key.ConsumerNamespace},
		{"checkpoint filter key", key.FilterKey},
	} {
		if err := checkIdentityText(in.field, in.value); err != nil {
			return "", false, err
		}
	}
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
	if s.pathErr != nil {
		return s.pathErr
	}
	if position == "" {
		return fmt.Errorf(
			"eventfeed: refusing to save an empty checkpoint position for %s; "+
				"Load would have to report it as a store failure",
			s.path)
	}
	// The write-side sibling of the load gates: json.Marshal does not refuse
	// invalid UTF-8, it silently swaps each invalid sequence for U+FFFD on
	// the way OUT — a caller's opaque position would mutate before it ever
	// reached disk, where the load gates can no longer tell mutation from
	// data. A caller-supplied invalid value is the caller's bug, so the
	// verdict is usage, exactly as checkIdentityText rules the same inputs
	// at construction; the labels below are the rendering, never the values.
	for _, in := range []struct{ field, value string }{
		{"checkpoint origin", key.Origin},
		{"checkpoint account id", key.AccountID},
		{"checkpoint consumer namespace", key.ConsumerNamespace},
		{"checkpoint filter key", key.FilterKey},
		{"checkpoint position", position},
	} {
		if err := checkIdentityText(in.field, in.value); err != nil {
			return err
		}
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
	payload := append(data, '\n')
	// The write is bound by the same limit the read is, and refuses BEFORE
	// replacing the last usable file. Without this the store bricks itself
	// through its own supported path: a Save whose result crosses the cap
	// renames a file that every later Load rejects — and every later Save
	// too, since Save reads before it writes — so there is no in-band way
	// back. The operator's only recovery is deleting the file, which discards
	// every other lineage's cursor with it.
	//
	// Reaching the cap is accretion rather than an adversary: there is no
	// delete, so a filter change mints a new FlatKey and leaves the old
	// lineage in the file for the life of the process. Refusing degrades to
	// the outcome the seam already documents for a failed save — reported
	// through Observer.CheckpointSaveFailed, the connector continues past it —
	// instead of an unrecoverable one. The rendering carries sizes only; the
	// position that pushed it over is caller- and server-supplied text.
	if int64(len(payload)) > maxCheckpointStoreBytes {
		return fmt.Errorf(
			"eventfeed: refusing to save to checkpoint store %s: the result would be %d bytes, past the %d-byte limit a load enforces; "+
				"the previous file is left in place",
			s.path, len(payload), maxCheckpointStoreBytes)
	}
	return s.writeAtomic(payload)
}

// read loads the whole file. It reports (entries, true, nil) when the file
// exists and parses, (nil, false, nil) when it does not exist — the only
// Missing-shaped filesystem condition — and an error for everything else.
//
// Errors name the path and the parse failure only; the file's contents are
// never echoed, since a corrupt store holds arbitrary bytes.
func (s *FileCheckpointStore) read() (map[string]string, bool, error) {
	// Stat before opening, and reject anything that is not a regular file.
	//
	// os.ReadFile followed whatever the path named. A FIFO blocks the open
	// until some writer appears, which hangs Load — and Load runs before the
	// first mint, so the whole feed hangs with it, with no timeout to escape
	// through (ctx is deliberately not honored here). A character device reads
	// without end: /dev/zero fills memory until the process dies. Neither is a
	// store this code can make sense of, so neither is worth following.
	//
	// Stat rather than Lstat, so a symlink to a regular file still works — an
	// operator pointing the store through a symlink is ordinary — and stat
	// never blocks, including on a FIFO, which is what lets the type be
	// checked before an open that could hang.
	//
	// The failure this guards is a misconfiguration, not an adversary: a path
	// aimed at /dev/null, a leftover FIFO, a device node. A live attacker
	// swapping the path between this stat and the open below would need write
	// access to the store's own 0700 directory, which is already enough to
	// rewrite the checkpoint outright — a shorter path to the same damage. The
	// fd-level re-check after the open is kept because it is nearly free, not
	// because that race is the point.
	info, err := os.Stat(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("eventfeed: reading checkpoint store %s: %w", s.path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s is not a regular file (mode %s); refusing to read it",
			s.path, info.Mode().Type())
	}

	f, err := os.Open(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("eventfeed: reading checkpoint store %s: %w", s.path, err)
	}
	defer f.Close() //nolint:errcheck // read-only; nothing to report on close

	if fi, ferr := f.Stat(); ferr == nil && !fi.Mode().IsRegular() {
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s is not a regular file (mode %s); refusing to read it",
			s.path, fi.Mode().Type())
	}

	// Read one byte past the cap so an over-size file is detected from what
	// was actually read, rather than trusted from the size stat reported —
	// which is zero for several things that nonetheless produce bytes.
	data, err := io.ReadAll(io.LimitReader(f, maxCheckpointStoreBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("eventfeed: reading checkpoint store %s: %w", s.path, err)
	}
	if int64(len(data)) > maxCheckpointStoreBytes {
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s exceeds the %d-byte limit; refusing to read it",
			s.path, maxCheckpointStoreBytes)
	}

	// Checked BEFORE unmarshaling, because encoding/json does not refuse
	// invalid UTF-8 — it silently replaces each invalid sequence with
	// U+FFFD. A corrupt or hand-edited store would therefore load a MUTATED
	// opaque position as if it were valid, polling with a checkpoint that
	// was never saved, and malformed keys could collapse onto valid
	// replacement-character keys. Corruption takes the documented Failed
	// path; it never silently changes checkpoint data.
	if !utf8.Valid(data) {
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s is not valid UTF-8; refusing to load mutated positions", s.path)
	}
	// The escape door into the same mutation: "\ud800" is pure ASCII, so
	// the byte gate above passes it, and the decoder would U+FFFD it just
	// the same. The gate claims corruption detection, so this door closes
	// with the other one (hasLoneSurrogateEscape).
	if hasLoneSurrogateEscape(data) {
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s carries a lone-surrogate escape; refusing to load mutated positions", s.path)
	}
	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s is not a JSON object of positions: %w", s.path, err)
	}
	// encoding/json keeps the LAST value for a duplicated key without error,
	// so a store naming one decoded FlatKey twice loaded whichever position
	// happened to be written later — ambiguity resumed as if it were data.
	// The detector counts MEMBERS with the decoder's own tokenizer
	// (escape-resolved, exactly as the decoder resolves keys), never value
	// tokens: an earlier string-token count let null-valued entries cancel a
	// duplicate's surplus — one duplicated key plus two null positions was
	// six string tokens against three decoded entries, and the equality
	// held. A null position itself is tolerated here: it decodes to "", and
	// the empty-position rule already fails the lineage that carries it at
	// lookup.
	if n, cerr := topLevelMemberCount(data); cerr != nil || n != len(entries) {
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s repeats a key; refusing to choose between its positions", s.path)
	}
	// Every key must be a canonical FlatKey. Corruption that rewrites a
	// lineage's key into some other valid JSON string would otherwise decode
	// cleanly and surface as MISSING — the connector entering at the present
	// and skipping history over damage the tri-state contract says must be
	// Failed. A well-formed key belonging to another lineage is not
	// corruption; a key outside the grammar is.
	for k := range entries {
		if !isCanonicalFlatKey(k) {
			return nil, false, fmt.Errorf(
				"eventfeed: checkpoint store %s carries a malformed key; refusing to report corruption as a missing checkpoint", s.path)
		}
	}
	if entries == nil {
		// JSON `null` unmarshals into a nil map without error. A store that
		// holds `null` is corrupt, not empty — `{}` is empty.
		return nil, false, fmt.Errorf(
			"eventfeed: checkpoint store %s holds JSON null, not an object of positions", s.path)
	}
	return entries, true, nil
}

// isCanonicalFlatKey reports whether k is exactly the form FlatKey writes:
// the compact JSON array of four strings, in the package's own encoding. The
// round-trip is the whole check — parse, re-encode with the same writer,
// compare — so any spelling Save could not have produced (extra whitespace,
// a different escape of the same text, the wrong arity or types) is
// malformed by construction.
func isCanonicalFlatKey(k string) bool {
	var parts []string
	if err := json.Unmarshal([]byte(k), &parts); err != nil || len(parts) != 4 {
		return false
	}
	rebuilt := CheckpointKey{
		Origin:            parts[0],
		AccountID:         parts[1],
		ConsumerNamespace: parts[2],
		FilterKey:         parts[3],
	}
	return rebuilt.FlatKey() == k
}

// writeAtomic replaces the store file with data via a temp file in the same
// directory plus a rename, which is atomic within a filesystem. The temp file
// is uniquely named so that a concurrent writer — another process, since every
// store over this path is serialized — cannot corrupt this one's staging file,
// and it is removed on every failure path so a failed save leaves no debris
// beside the store.
//
// A symlinked store path is written THROUGH, not replaced. read follows the
// link (Stat, deliberately — an operator pointing the store through a symlink
// is ordinary), so the rename must land the bytes at the target the link
// names: renaming onto the link's own directory entry would leave the target
// untouched and silently turn the configured symlink into a regular file
// after the first save, splitting consumers that address the link from those
// that address the target. Resolving also keeps the rename same-filesystem
// when the link crosses one — the staging temp lives beside the TARGET. Lock
// identity is unaffected: it stays the configured spelling by design
// (canonicalStorePath).
func (s *FileCheckpointStore) writeAtomic(data []byte) error {
	target, err := resolveStorePath(s.path)
	if err != nil {
		return fmt.Errorf("eventfeed: resolving checkpoint store path %s: %w", s.path, err)
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("eventfeed: creating checkpoint store directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(target)+".tmp-*")
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
	// Sync before rename, directory after: see the Durability section. A
	// rename whose staged blocks never reached stable storage can outlive a
	// crash as a zero-length file that bricks every later Load.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("eventfeed: syncing the staged checkpoint store for %s: %w", s.path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("eventfeed: closing the staged checkpoint store for %s: %w", s.path, err)
	}
	// os.CreateTemp already creates at 0600, but that is subject to the
	// process umask; the mode is part of the contract, so set it outright.
	if err := os.Chmod(tmpPath, 0o600); err != nil { // #nosec G703 -- store path is caller-configured
		return fmt.Errorf("eventfeed: setting mode on the staged checkpoint store for %s: %w", s.path, err)
	}
	if err := os.Rename(tmpPath, target); err != nil { // #nosec G703 -- store path is caller-configured
		return fmt.Errorf("eventfeed: replacing checkpoint store %s: %w", s.path, err)
	}
	// The rename itself lives in the directory; syncing it is what makes the
	// replacement durable rather than merely atomic. A failure here reports
	// Failed — the data file is already renamed, and a retried save rewrites
	// the same content, so the caller loses nothing but the certainty.
	//
	// Not on Windows: FlushFileBuffers rejects directory handles, so the
	// sync would turn EVERY successful replacement into a reported failure.
	// NTFS's rename metadata is journaled without an explicit directory
	// flush; the file-content Sync above still ran everywhere.
	if runtime.GOOS == "windows" {
		return nil
	}
	dirf, err := os.Open(dir) // #nosec G703 -- store path is caller-configured
	if err != nil {
		return fmt.Errorf("eventfeed: opening checkpoint store directory %s to sync it: %w", dir, err)
	}
	syncErr := dirf.Sync()
	_ = dirf.Close()
	if syncErr != nil {
		return fmt.Errorf("eventfeed: syncing checkpoint store directory %s: %w", dir, syncErr)
	}
	return nil
}

// resolveStorePath follows a symlinked FINAL component to the path a write
// must replace, so writeAtomic and read agree on where the store lives. Only
// the last element is walked — parent directories are followed by the
// filesystem itself on every open — and a dangling link resolves to its
// nonexistent target, where the first save then creates the file, exactly as
// an open through the link would. The chain bound matches the kernels'
// ELOOP-class limit; filepath.EvalSymlinks is not usable here because it
// requires the full path to exist, and this store's file is created on the
// first save.
func resolveStorePath(path string) (string, error) {
	for range 40 {
		fi, err := os.Lstat(path)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			// Absent (first save) or a real file: this is the entry to
			// replace. A stat error other than absence surfaces on the
			// open/rename that follows, with its own context.
			return path, nil //nolint:nilerr // deliberate: the follow-up open/rename reports the failure with better context
		}
		dest, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(filepath.Dir(path), dest)
		}
		path = dest
	}
	return "", errors.New("too many levels of symbolic links")
}
