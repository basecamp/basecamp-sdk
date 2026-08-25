package eventfeed

// Drives FileCheckpointStore, the one built-in CheckpointStore (SPEC.md §23:
// "a single JSON file keyed by the compact RFC 8259 JSON array of the four
// identity strings ... written atomically (temp + rename, 0600), documented
// as single-process").
//
// The load contract is tri-state and the distinctions are load-bearing:
// Failed is Terminal(checkpoint_load) with zero wire attempts, while Missing
// proceeds to a present-class entry. Collapsing an unreadable or corrupt file
// to Missing would silently start at the present and skip history, so every
// filesystem condition below is asserted against the interface's own return
// shape — (position, ok, err) — not against error strings.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The compile-time seam assertion lives in filestore.go, where a break in the
// contract fails the build rather than only the tests.

// storeKey builds a checkpoint identity distinguished only by its consumer
// namespace — the field that separates two independent consumers' lineages in
// one account.
func storeKey(namespace string) CheckpointKey {
	return CheckpointKey{
		Origin:            "https://3.basecampapi.com",
		AccountID:         "5951425",
		ConsumerNamespace: namespace,
		FilterKey:         Filters{}.FilterKey(),
	}
}

// storePath is a fresh, not-yet-created store file inside an isolated root.
func storePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "checkpoints.json")
}

// writeStoreFile seeds the store file with exact bytes, bypassing Save, so a
// test can stage a corrupt or hostile on-disk state.
func writeStoreFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("seeding store directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("seeding store file: %v", err)
	}
}

// requireNonRoot skips permission-denial tests under root, which defeats the
// mode bits they depend on.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod 0000 does not deny root, so this permission case cannot be staged")
	}
}

// filesUnder lists every regular file beneath root, relative to it — the
// probe for stray temp files and for anything a key tried to write outside
// the store file.
func filesUnder(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		found = append(found, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return found
}

func TestFileCheckpointStore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewFileCheckpointStore(storePath(t))
	key := storeKey("openclaw")

	if err := store.Save(ctx, key, "pos-1"); err != nil {
		t.Fatalf("Save() = %v, want nil (Saved)", err)
	}

	position, ok, err := store.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if !ok {
		t.Fatal("Load() ok = false, want true (Loaded)")
	}
	if position != "pos-1" {
		t.Errorf("Load() position = %q, want %q", position, "pos-1")
	}
}

// A symlinked store path is written THROUGH, not replaced. read follows the
// link (an operator pointing the store through a symlink is documented as
// ordinary), so Save must land the bytes at the link's target and leave the
// link standing — a rename onto the link's own entry would silently turn it
// into a regular file and leave every target-addressed consumer reading the
// stale checkpoint forever.
func TestFileCheckpointStore_SaveWritesThroughASymlinkedPath(t *testing.T) {
	ctx := context.Background()
	key := storeKey("openclaw")
	dir := t.TempDir()
	target := filepath.Join(dir, "real-checkpoints.json")
	link := filepath.Join(dir, "checkpoints.json")

	// Seed the target through its own path, then link to it.
	if err := NewFileCheckpointStore(target).Save(ctx, key, "pos-old"); err != nil {
		t.Fatalf("seeding the target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := NewFileCheckpointStore(link).Save(ctx, key, "pos-new"); err != nil {
		t.Fatalf("Save() through the link = %v, want nil", err)
	}

	switch fi, err := os.Lstat(link); {
	case err != nil:
		t.Errorf("after Save, Lstat(link) failed: %v — want the link left standing", err)
	case fi.Mode()&os.ModeSymlink == 0:
		t.Errorf("after Save, link mode = %v — the link was replaced, want it left standing", fi.Mode())
	}
	// The target holds the new position: link- and target-addressed
	// consumers stay one store.
	position, ok, err := NewFileCheckpointStore(target).Load(ctx, key)
	if err != nil || !ok || position != "pos-new" {
		t.Errorf("Load() via target = (%q, %v, %v), want (%q, true, nil)", position, ok, err, "pos-new")
	}
	position, ok, err = NewFileCheckpointStore(link).Load(ctx, key)
	if err != nil || !ok || position != "pos-new" {
		t.Errorf("Load() via link = (%q, %v, %v), want (%q, true, nil)", position, ok, err, "pos-new")
	}
}

// A DANGLING symlink behaves like an open through it would: the first save
// creates the file at the target, and the link keeps naming it.
func TestFileCheckpointStore_SaveCreatesADanglingSymlinkTarget(t *testing.T) {
	ctx := context.Background()
	key := storeKey("openclaw")
	dir := t.TempDir()
	target := filepath.Join(dir, "not-yet.json")
	link := filepath.Join(dir, "checkpoints.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := NewFileCheckpointStore(link).Save(ctx, key, "pos-1"); err != nil {
		t.Fatalf("Save() through the dangling link = %v, want nil", err)
	}
	switch fi, err := os.Lstat(link); {
	case err != nil:
		t.Errorf("after Save, Lstat(link) failed: %v — want the link left standing", err)
	case fi.Mode()&os.ModeSymlink == 0:
		t.Errorf("after Save, link mode = %v — want the link left standing", fi.Mode())
	}
	position, ok, err := NewFileCheckpointStore(target).Load(ctx, key)
	if err != nil || !ok || position != "pos-1" {
		t.Errorf("Load() via target = (%q, %v, %v), want (%q, true, nil)", position, ok, err, "pos-1")
	}
}

// encoding/json does not refuse invalid UTF-8 — it silently swaps each
// invalid sequence for U+FFFD — so without the raw-bytes check Load would
// resolve the real key and hand back a position that was never saved: silent
// checkpoint identity drift dressed as a successful load. Corruption must
// take the documented Failed path (Terminal(checkpoint_load)), never mutate.
func TestFileCheckpointStore_InvalidUTF8IsFailedNotMutated(t *testing.T) {
	path := storePath(t)
	key := storeKey("openclaw")
	// The store's real flat key mapped to a position holding one raw 0xFF
	// byte — the shape a truncated write or hand edit leaves behind.
	corrupt := "{" + strconv.Quote(key.FlatKey()) + ":\"pos-\xff-1\"}"
	writeStoreFile(t, path, corrupt)

	position, ok, err := NewFileCheckpointStore(path).Load(context.Background(), key)
	if err == nil {
		t.Fatalf("Load() = (%q, %v, nil), want Failed — the decoder handed back a mutated position", position, ok)
	}
	if position != "" || ok {
		t.Errorf("Load() = (%q, %v) alongside the failure, want zero values", position, ok)
	}
}

// A position survives a fresh store instance over the same path: the point of
// the store is durability across runs, not within one.
func TestFileCheckpointStore_RoundTripAcrossInstances(t *testing.T) {
	ctx := context.Background()
	path := storePath(t)
	key := storeKey("openclaw")

	if err := NewFileCheckpointStore(path).Save(ctx, key, "pos-1"); err != nil {
		t.Fatalf("Save() = %v, want nil", err)
	}

	position, ok, err := NewFileCheckpointStore(path).Load(ctx, key)
	if err != nil || !ok || position != "pos-1" {
		t.Errorf("Load() = (%q, %v, %v), want (%q, true, nil)", position, ok, err, "pos-1")
	}
}

// The tri-state table: every filesystem condition mapped to the interface's
// own outcome. Missing is (_, false, nil); Failed is a non-nil error; Loaded
// is (position, true, nil).
func TestFileCheckpointStore_LoadTriState(t *testing.T) {
	key := storeKey("openclaw")
	flat := key.FlatKey()
	otherFlat := storeKey("other-consumer").FlatKey()

	tests := []struct {
		name string
		// seed writes the on-disk state; an empty contents string means the
		// file is never created at all.
		seed func(t *testing.T, path string)
		// wantOK / wantPosition apply only when no error is expected.
		wantErr      bool
		wantOK       bool
		wantPosition string
		skipAsRoot   bool
	}{
		{
			name:   "file absent is Missing, not Failed",
			seed:   func(t *testing.T, path string) { t.Helper() },
			wantOK: false,
		},
		{
			name: "empty JSON object is Missing",
			seed: func(t *testing.T, path string) { t.Helper(); writeStoreFile(t, path, `{}`) },
		},
		{
			name: "other lineages only is Missing",
			seed: func(t *testing.T, path string) {
				t.Helper()
				writeStoreFile(t, path, `{`+jsonQuote(otherFlat)+`:"pos-other"}`)
			},
		},
		{
			name: "our key present is Loaded",
			seed: func(t *testing.T, path string) {
				t.Helper()
				writeStoreFile(t, path, `{`+jsonQuote(flat)+`:"pos-1"}`)
			},
			wantOK:       true,
			wantPosition: "pos-1",
		},
		{
			name:    "malformed JSON is Failed",
			seed:    func(t *testing.T, path string) { t.Helper(); writeStoreFile(t, path, "NOT JSON") },
			wantErr: true,
		},
		{
			name:    "truncated JSON is Failed",
			seed:    func(t *testing.T, path string) { t.Helper(); writeStoreFile(t, path, `{"a":`) },
			wantErr: true,
		},
		{
			name:    "JSON array instead of object is Failed",
			seed:    func(t *testing.T, path string) { t.Helper(); writeStoreFile(t, path, `["pos-1"]`) },
			wantErr: true,
		},
		{
			name:    "JSON null is Failed",
			seed:    func(t *testing.T, path string) { t.Helper(); writeStoreFile(t, path, `null`) },
			wantErr: true,
		},
		{
			name:    "empty file is Failed",
			seed:    func(t *testing.T, path string) { t.Helper(); writeStoreFile(t, path, "") },
			wantErr: true,
		},
		{
			name: "non-string position is Failed",
			seed: func(t *testing.T, path string) {
				t.Helper()
				writeStoreFile(t, path, `{`+jsonQuote(flat)+`:42}`)
			},
			wantErr: true,
		},
		{
			name: "empty stored position is Failed, not Loaded-with-empty",
			seed: func(t *testing.T, path string) {
				t.Helper()
				writeStoreFile(t, path, `{`+jsonQuote(flat)+`:""}`)
			},
			wantErr: true,
		},
		{
			name: "unreadable file is Failed, not Missing",
			seed: func(t *testing.T, path string) {
				t.Helper()
				writeStoreFile(t, path, `{`+jsonQuote(flat)+`:"pos-1"}`)
				if err := os.Chmod(path, 0o000); err != nil {
					t.Fatalf("chmod 0000: %v", err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			},
			wantErr:    true,
			skipAsRoot: true,
		},
		{
			name: "store path is a directory is Failed",
			seed: func(t *testing.T, path string) {
				t.Helper()
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatalf("staging directory at store path: %v", err)
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipAsRoot {
				requireNonRoot(t)
			}
			path := storePath(t)
			tc.seed(t, path)

			position, ok, err := NewFileCheckpointStore(path).Load(context.Background(), key)

			switch {
			case tc.wantErr:
				if err == nil {
					t.Fatalf("Load() = (%q, %v, nil), want Failed (non-nil error)", position, ok)
				}
			case tc.wantOK:
				if err != nil {
					t.Fatalf("Load() error = %v, want nil (Loaded)", err)
				}
				if !ok {
					t.Fatal("Load() ok = false, want true (Loaded)")
				}
				if position != tc.wantPosition {
					t.Errorf("Load() position = %q, want %q", position, tc.wantPosition)
				}
			default:
				if err != nil {
					t.Fatalf("Load() error = %v, want nil (Missing)", err)
				}
				if ok {
					t.Errorf("Load() ok = true (position %q), want false (Missing)", position)
				}
			}
		})
	}
}

// A missing file must be Missing specifically because it is absent — asserted
// on the error being nil rather than on any wrapped fs.ErrNotExist, since
// Missing carries no error at all.
func TestFileCheckpointStore_MissingIsNotAWrappedNotExist(t *testing.T) {
	_, ok, err := NewFileCheckpointStore(storePath(t)).Load(context.Background(), storeKey("openclaw"))
	if err != nil {
		t.Fatalf("Load() on a fresh root error = %v, want nil", err)
	}
	if ok {
		t.Error("Load() ok = true, want false")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Error("Load() reported fs.ErrNotExist; absence is Missing, which carries no error")
	}
}

// Two lineages share one file without cross-talk: each key reads back its own
// position, and neither save erases the other.
func TestFileCheckpointStore_KeyIsolation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "checkpoints.json")
	store := NewFileCheckpointStore(path)

	alpha := storeKey("alpha")
	beta := storeKey("beta")

	if err := store.Save(ctx, alpha, "pos-alpha"); err != nil {
		t.Fatalf("Save(alpha) = %v, want nil", err)
	}
	if err := store.Save(ctx, beta, "pos-beta"); err != nil {
		t.Fatalf("Save(beta) = %v, want nil", err)
	}

	for _, tc := range []struct {
		key  CheckpointKey
		want string
	}{{alpha, "pos-alpha"}, {beta, "pos-beta"}} {
		position, ok, err := store.Load(ctx, tc.key)
		if err != nil || !ok || position != tc.want {
			t.Errorf("Load(%s) = (%q, %v, %v), want (%q, true, nil)",
				tc.key.ConsumerNamespace, position, ok, err, tc.want)
		}
	}

	// A third, never-saved lineage is Missing — not another key's position.
	if position, ok, err := store.Load(ctx, storeKey("gamma")); err != nil || ok {
		t.Errorf("Load(gamma) = (%q, %v, %v), want (\"\", false, nil)", position, ok, err)
	}

	// A differing filter set is a different lineage under the same namespace.
	filtered := storeKey("alpha")
	filtered.FilterKey = Filters{Types: []string{"message.created"}}.FilterKey()
	if filtered.FilterKey == alpha.FilterKey {
		t.Fatal("test setup: filtered lineage collides with the unfiltered one")
	}
	if position, ok, err := store.Load(ctx, filtered); err != nil || ok {
		t.Errorf("Load(alpha/filtered) = (%q, %v, %v), want (\"\", false, nil)", position, ok, err)
	}

	if files := filesUnder(t, root); len(files) != 1 || files[0] != "checkpoints.json" {
		t.Errorf("files under root = %v, want exactly [checkpoints.json]", files)
	}
}

// Overwriting leaves exactly one file holding the new value — no stray temp
// file from the temp-then-rename, and no append-style accumulation.
func TestFileCheckpointStore_OverwriteIsAtomicAndLeavesOneFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "checkpoints.json")
	store := NewFileCheckpointStore(path)
	key := storeKey("openclaw")

	for _, position := range []string{"pos-1", "pos-2", "pos-3"} {
		if err := store.Save(ctx, key, position); err != nil {
			t.Fatalf("Save(%q) = %v, want nil", position, err)
		}
	}

	position, ok, err := store.Load(ctx, key)
	if err != nil || !ok || position != "pos-3" {
		t.Errorf("Load() = (%q, %v, %v), want (%q, true, nil)", position, ok, err, "pos-3")
	}

	files := filesUnder(t, root)
	if len(files) != 1 || files[0] != "checkpoints.json" {
		t.Errorf("files under root after 3 saves = %v, want exactly [checkpoints.json]", files)
	}

	// The file holds one entry for the key, not three.
	var onDisk map[string]string
	raw, err := os.ReadFile(path) // #nosec G304 -- test-owned temp path
	if err != nil {
		t.Fatalf("reading store file: %v", err)
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("store file is not a JSON object of strings: %v", err)
	}
	if len(onDisk) != 1 {
		t.Errorf("store file holds %d entries, want 1: %v", len(onDisk), onDisk)
	}
	if got := onDisk[key.FlatKey()]; got != "pos-3" {
		t.Errorf("on-disk position for the flat key = %q, want %q", got, "pos-3")
	}
}

// A key is data, never a path. Origins and namespaces crafted with separators
// and "..' traversal must not create, touch, or escape into anything outside
// the single store file.
func TestFileCheckpointStore_HostileKeysCannotEscapeTheStoreFile(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	root := filepath.Join(base, "state")
	path := filepath.Join(root, "checkpoints.json")

	// A sentinel a traversal would plausibly aim at.
	sentinel := filepath.Join(base, "sentinel")
	if err := os.WriteFile(sentinel, []byte("untouched"), 0o600); err != nil {
		t.Fatalf("seeding sentinel: %v", err)
	}

	hostile := []CheckpointKey{
		{Origin: "https://../../etc", AccountID: "1", ConsumerNamespace: "../../../sentinel", FilterKey: "srv1-0000000000000000"},
		{Origin: "https://x", AccountID: "../..", ConsumerNamespace: "a/b/c", FilterKey: "srv1-1111111111111111"},
		{Origin: "..", AccountID: "..", ConsumerNamespace: "..", FilterKey: ".."},
		{Origin: `https://x`, AccountID: `1`, ConsumerNamespace: "with\x00nul and /slashes/", FilterKey: "srv1-2222222222222222"},
		{Origin: "https://x", AccountID: "1", ConsumerNamespace: string(make([]byte, 4096)), FilterKey: "srv1-3333333333333333"},
	}

	store := NewFileCheckpointStore(path)
	for i, key := range hostile {
		if err := store.Save(ctx, key, "pos-hostile"); err != nil {
			t.Fatalf("Save(hostile[%d]) = %v, want nil", i, err)
		}
	}

	// Everything landed inside the one store file, and nothing else exists.
	files := filesUnder(t, base)
	want := map[string]bool{"sentinel": true, filepath.Join("state", "checkpoints.json"): true}
	if len(files) != len(want) {
		t.Fatalf("files under base = %v, want exactly %v", files, want)
	}
	for _, f := range files {
		if !want[f] {
			t.Errorf("unexpected file %q under base; a key escaped the store file", f)
		}
	}

	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "untouched" {
		t.Errorf("sentinel = (%q, %v), want (%q, nil)", data, err, "untouched")
	}

	// And each hostile key still round-trips: it was honored as an identity,
	// not sanitized into a collision.
	for i, key := range hostile {
		position, ok, err := store.Load(ctx, key)
		if err != nil || !ok || position != "pos-hostile" {
			t.Errorf("Load(hostile[%d]) = (%q, %v, %v), want (%q, true, nil)",
				i, position, ok, err, "pos-hostile")
		}
	}
}

// The store creates its directory on demand, at 0700, and writes the file at
// 0600 — it may sit beside token caches.
func TestFileCheckpointStore_CreatesDirectoryOnDemandWithTightModes(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "nested", "state")
	path := filepath.Join(dir, "checkpoints.json")

	if err := NewFileCheckpointStore(path).Save(context.Background(), storeKey("openclaw"), "pos-1"); err != nil {
		t.Fatalf("Save() into a non-existent directory = %v, want nil", err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat store directory: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("store directory mode = %#o, want 0700", perm)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat store file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("store file mode = %#o, want 0600", perm)
	}
}

// Save must not silently clobber a file it cannot parse: doing so would erase
// every other consumer's lineage in the same file. Failed here is
// CheckpointSaveFailed with the feed continuing, which is recoverable; a
// silent wipe is not.
func TestFileCheckpointStore_SaveOverCorruptFileIsFailed(t *testing.T) {
	path := storePath(t)
	writeStoreFile(t, path, "NOT JSON")

	if err := NewFileCheckpointStore(path).Save(context.Background(), storeKey("openclaw"), "pos-1"); err == nil {
		t.Fatal("Save() over a corrupt file = nil, want Failed (non-nil error)")
	}

	// The unparseable bytes are preserved for forensics, not overwritten.
	raw, err := os.ReadFile(path) // #nosec G304 -- test-owned temp path
	if err != nil {
		t.Fatalf("reading store file: %v", err)
	}
	if string(raw) != "NOT JSON" {
		t.Errorf("store file = %q after a failed save, want the original bytes preserved", raw)
	}
}

// An empty position is not a position. Accepting one on save would write a
// value that load must then reject as Failed — a lineage writable but not
// loadable.
func TestFileCheckpointStore_SaveRejectsEmptyPosition(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "checkpoints.json")

	if err := NewFileCheckpointStore(path).Save(context.Background(), storeKey("openclaw"), ""); err == nil {
		t.Fatal("Save(position=\"\") = nil, want Failed (non-nil error)")
	}
	if files := filesUnder(t, root); len(files) != 0 {
		t.Errorf("files under root = %v, want none: a rejected save must write nothing", files)
	}
}

func TestFileCheckpointStore_SaveIntoUnwritableDirectoryIsFailed(t *testing.T) {
	requireNonRoot(t)

	root := t.TempDir()
	dir := filepath.Join(root, "state")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("staging directory: %v", err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod 0500: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	path := filepath.Join(dir, "checkpoints.json")
	if err := NewFileCheckpointStore(path).Save(context.Background(), storeKey("openclaw"), "pos-1"); err == nil {
		t.Fatal("Save() into a read-only directory = nil, want Failed (non-nil error)")
	}
}

// Errors name the store path and the failure, never the surrounding run
// state. The key carries no ticket, so this pins that nothing ticket-shaped
// is smuggled in alongside it — and that a corrupt file's bytes are not
// echoed back into the error.
func TestFileCheckpointStore_ErrorsCarryNoFileContents(t *testing.T) {
	path := storePath(t)
	const secret = "SHOULD-NOT-APPEAR-IN-ERRORS"
	writeStoreFile(t, path, secret)

	_, _, err := NewFileCheckpointStore(path).Load(context.Background(), storeKey("openclaw"))
	if err == nil {
		t.Fatal("Load() over a corrupt file = nil error, want Failed")
	}
	if msg := err.Error(); strings.Contains(msg, secret) {
		t.Errorf("Load() error %q echoes the file contents", msg)
	}
}

// Concurrent savers over one instance are serialized: the connector is one
// writer per key, but the store must not corrupt its own file if a host
// shares an instance across lineages. Run under -race.
func TestFileCheckpointStore_ConcurrentSavesKeepTheFileParseable(t *testing.T) {
	ctx := context.Background()
	store := NewFileCheckpointStore(storePath(t))

	const lineages = 8
	var wg sync.WaitGroup
	for i := 0; i < lineages; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := storeKey("consumer-" + string(rune('a'+i)))
			for range 10 {
				if err := store.Save(ctx, key, "pos-final"); err != nil {
					t.Errorf("Save(consumer-%d) = %v, want nil", i, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < lineages; i++ {
		key := storeKey("consumer-" + string(rune('a'+i)))
		position, ok, err := store.Load(ctx, key)
		if err != nil || !ok || position != "pos-final" {
			t.Errorf("Load(consumer-%d) = (%q, %v, %v), want (%q, true, nil)",
				i, position, ok, err, "pos-final")
		}
	}
}

// Two stores constructed independently over one path are two objects over one
// file, and the file's update is a read-modify-write: without coordination by
// path both instances read the same absent-or-old object and each renames a
// single-lineage file into place, so one lineage's cursor is silently lost.
// Sharing a checkpoint file between lineages is the documented setup — that is
// why the file is keyed by the identity array rather than one file per lineage
// — so the loss is a real defect, not a misuse. Run under -race.
func TestFileCheckpointStore_ConcurrentSavesAcrossInstancesKeepEveryLineage(t *testing.T) {
	ctx := context.Background()
	path := storePath(t)
	lineages := []struct {
		key      CheckpointKey
		position string
	}{
		{storeKey("openclaw"), "pos-openclaw"},
		{storeKey("shadowfax"), "pos-shadowfax"},
	}

	// Each round starts from an absent file so both savers read the same empty
	// object: any overlap of the two read-modify-writes loses a lineage.
	for round := range 50 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("round %d: clearing the store file: %v", round, err)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, lineage := range lineages {
			store := NewFileCheckpointStore(path)
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if err := store.Save(ctx, lineage.key, lineage.position); err != nil {
					t.Errorf("Save(%s) = %v, want nil", lineage.position, err)
				}
			}()
		}
		close(start)
		wg.Wait()

		for _, lineage := range lineages {
			position, ok, err := NewFileCheckpointStore(path).Load(ctx, lineage.key)
			if err != nil || !ok || position != lineage.position {
				t.Fatalf("round %d: Load(%s) = (%q, %v, %v), want (%q, true, nil): "+
					"a concurrent save over the same path lost this lineage",
					round, lineage.position, position, ok, err, lineage.position)
			}
		}
	}
}

// The lock is keyed by the file, so spellings that name one file — relative,
// absolute, or routed through "..", the divergence two components reading one
// configured path actually produce — must land on one lock, and two different
// files must not.
func TestFileCheckpointStore_OnePathIsOneLockAcrossSpellings(t *testing.T) {
	dir := t.TempDir()
	direct := filepath.Join(dir, "checkpoints.json")
	roundabout := filepath.Join(dir, "nested", "..", "checkpoints.json")
	if pathLock(direct) != pathLock(roundabout) {
		t.Errorf("%q and %q took different locks, want one lock per file", direct, roundabout)
	}

	t.Chdir(dir)
	if pathLock("checkpoints.json") != pathLock(direct) {
		t.Error("the relative spelling of the store file took a different lock than the absolute one")
	}
	if pathLock(filepath.Join(dir, "other.json")) == pathLock(direct) {
		t.Error("two different store files share one lock")
	}
}

// The lock is keyed by the canonical absolute path, so the file operations
// must use that same path and not re-resolve a relative spelling against
// whatever the working directory happens to be later. A process that chdirs
// after constructing a store — a CLI resolving a workspace, a test using
// t.Chdir — would otherwise have the store write a DIFFERENT file while
// holding the original file's lock, and a store constructed for that new file
// takes a different lock: two writers, one file, no serialization, which is
// exactly the lost update the lock registry exists to prevent.
func TestFileCheckpointStore_RelativePathSurvivesAWorkingDirectoryChange(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	elsewhere := t.TempDir()
	key := storeKey("openclaw")

	t.Chdir(home)
	store := NewFileCheckpointStore("checkpoints.json")
	if err := store.Save(ctx, key, "pos-1"); err != nil {
		t.Fatalf("Save() before the chdir = %v, want nil", err)
	}

	t.Chdir(elsewhere)
	if err := store.Save(ctx, key, "pos-2"); err != nil {
		t.Fatalf("Save() after the chdir = %v, want nil", err)
	}
	if files := filesUnder(t, elsewhere); len(files) != 0 {
		t.Fatalf("files under the new working directory = %v, want none: "+
			"the store must keep writing the file whose lock it holds", files)
	}

	// A store constructed now over the same relative spelling names a
	// different file, and therefore takes a different lock. The two must stay
	// on their own files — under the defect they would share one file while
	// holding different locks.
	other := NewFileCheckpointStore("checkpoints.json")
	if other.mu == store.mu {
		t.Fatal("two stores over two different files share one lock")
	}
	if err := other.Save(ctx, storeKey("shadowfax"), "pos-other"); err != nil {
		t.Fatalf("Save() into the new working directory = %v, want nil", err)
	}

	homeFile := readStoreFile(t, filepath.Join(home, "checkpoints.json"))
	if got := homeFile[key.FlatKey()]; got != "pos-2" {
		t.Errorf("the original file holds %q for the original lineage, want %q", got, "pos-2")
	}
	if _, ok := homeFile[storeKey("shadowfax").FlatKey()]; ok {
		t.Error("the second store wrote into the first store's file")
	}
	elsewhereFile := readStoreFile(t, filepath.Join(elsewhere, "checkpoints.json"))
	if _, ok := elsewhereFile[key.FlatKey()]; ok {
		t.Error("the first store wrote into the second store's file after the chdir")
	}
}

// readStoreFile parses the store file at path, failing the test if it is
// absent or unparseable.
func readStoreFile(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- test-owned temp path
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return entries
}

// Concurrent readers and a writer must not race. Run under -race.
func TestFileCheckpointStore_ConcurrentLoadAndSave(t *testing.T) {
	ctx := context.Background()
	store := NewFileCheckpointStore(storePath(t))
	key := storeKey("openclaw")
	if err := store.Save(ctx, key, "pos-1"); err != nil {
		t.Fatalf("seeding Save() = %v, want nil", err)
	}

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 20 {
				if _, _, err := store.Load(ctx, key); err != nil {
					t.Errorf("Load() = %v, want nil", err)
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			for range 20 {
				if err := store.Save(ctx, key, "pos-1"); err != nil {
					t.Errorf("Save() = %v, want nil", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// jsonQuote renders s as a JSON string literal for staging file contents.
func jsonQuote(s string) string {
	quoted, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(quoted)
}

// ---------------------------------------------------------------------------
// #761 and the unbounded read.
// ---------------------------------------------------------------------------

// TestFileStore_OneFileTakesOneLockRegardlessOfCase is #761. On a
// case-insensitive filesystem — APFS as macOS ships it, NTFS — "feed.json" and
// "Feed.json" are ONE file, so two stores spelled differently must share the
// mutex that serializes the read-modify-write. Two mutexes over one file is
// the lost update the registry exists to prevent, and nothing more exotic than
// two call sites disagreeing about capitalization reaches it.
//
// Asserted on lock identity rather than by racing two savers: a race would
// reproduce the loss only probabilistically, and would pass on a
// case-sensitive filesystem where the two paths really are two files.
func TestFileStore_OneFileTakesOneLockRegardlessOfCase(t *testing.T) {
	dir := t.TempDir()
	lower := NewFileCheckpointStore(filepath.Join(dir, "feed.json"))
	upper := NewFileCheckpointStore(filepath.Join(dir, "Feed.json"))
	mixed := NewFileCheckpointStore(filepath.Join(dir, "FeEd.JsOn"))

	if lower.mu != upper.mu {
		t.Error(`stores over "feed.json" and "Feed.json" hold different locks; on a case-insensitive filesystem that is two writers over one file`)
	}
	if lower.mu != mixed.mu {
		t.Error(`stores over "feed.json" and "FeEd.JsOn" hold different locks`)
	}
	// The path each store actually reads and writes is NOT folded — only the
	// lock key is. Folding the path itself would make the store write a
	// different file than the caller named.
	if lower.path == upper.path {
		t.Errorf("both stores resolved to the same path %q; only the lock key may be case-folded", lower.path)
	}
	if got := filepath.Base(upper.path); got != "Feed.json" {
		t.Errorf("store path base = %q, want %q — the caller's spelling must survive", got, "Feed.json")
	}
}

// TestFileStore_RefusesNonRegularFiles covers the read that followed whatever
// the path named. A character device reads without end (/dev/zero fills memory
// until the process dies); a FIFO blocks the OPEN until a writer appears,
// which hangs Load — and Load runs before the first mint, with ctx
// deliberately not honored, so the whole feed hangs behind it with no timeout
// to escape through.
func TestFileStore_RefusesNonRegularFiles(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileCheckpointStore(filepath.Join(dir, "sub"))
		if err := os.Mkdir(store.path, 0o700); err != nil {
			t.Fatal(err)
		}
		assertNotRegularRefusal(t, store)
	})

	t.Run("fifo", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileCheckpointStore(filepath.Join(dir, "feed.json"))
		if err := syscall.Mkfifo(store.path, 0o600); err != nil {
			t.Skipf("mkfifo unavailable: %v", err)
		}
		assertNotRegularRefusal(t, store)
	})

	t.Run("character device", func(t *testing.T) {
		if _, err := os.Stat("/dev/zero"); err != nil {
			t.Skip("/dev/zero unavailable")
		}
		assertNotRegularRefusal(t, NewFileCheckpointStore("/dev/zero"))
	})
}

// assertNotRegularRefusal requires Load to refuse store's file as non-regular,
// under a bound, because the failure mode being guarded is a HANG rather than
// a wrong answer: a FIFO with no writer blocks the open forever, and /dev/zero
// reads forever. Un-bounded, a regression takes the whole package's timeout
// with it and names nothing — verified, and it is what the first draft of
// these tests did.
func assertNotRegularRefusal(t *testing.T, store *FileCheckpointStore) {
	t.Helper()

	type result struct {
		ok  bool
		err error
	}
	done := make(chan result, 1)
	go func() {
		_, ok, err := store.Load(context.Background(), CheckpointKey{})
		done <- result{ok, err}
	}()

	select {
	case got := <-done:
		if got.err == nil {
			t.Fatalf("Load = (ok=%v, nil); a non-regular file must be a store FAILURE, not Missing", got.ok)
		}
		if got.ok {
			t.Error("Load reported a loaded position from a non-regular file")
		}
		if !strings.Contains(got.err.Error(), "not a regular file") {
			t.Errorf("Load error = %v, want a not-a-regular-file refusal", got.err)
		}
	case <-time.After(10 * time.Second):
		// The goroutine is left blocked: there is nothing to cancel, since
		// this is exactly the property under test.
		t.Fatalf("Load over %s did not return; the file type must be refused before an open or read that cannot finish", store.path)
	}
}

// TestFileStore_RefusesOversizedFile bounds the allocation. Load runs before
// the first mint, so an absurd file is an unbounded allocation on the startup
// path; the bounded, reported failure is Terminal(checkpoint_load) with zero
// wire attempts.
func TestFileStore_RefusesOversizedFile(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(filepath.Join(dir, "feed.json"))

	f, err := os.Create(store.path)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse: the bytes are never written, so this costs no disk and no time,
	// but the read would still have to materialize them.
	if err := f.Truncate(maxCheckpointStoreBytes + 1); err != nil {
		_ = f.Close()
		t.Skipf("cannot create a sparse file of the required size: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, ok, err := store.Load(context.Background(), CheckpointKey{})
	if err == nil {
		t.Fatalf("Load = (ok=%v, nil); an over-size store must be a failure", ok)
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("Load error = %v, want a size-limit refusal", err)
	}
	// Nothing of the file's contents is echoed.
	if strings.Contains(err.Error(), "\x00\x00\x00") {
		t.Errorf("Load error echoed file contents: %v", err)
	}
}

// TestFileStore_SaveRefusesToWritePastTheReadLimit closes the other half of
// the bound above. The read enforces the cap; the write did not, so a Save
// whose result crossed it replaced a good file with one nothing can ever read
// again — every later Load AND every later Save fails on the same limit, and
// since Save's read-modify-write reads first, there is no in-band way back:
// the operator's only recovery is deleting the file, which discards every
// OTHER lineage's cursor with it. The store bricks itself, permanently,
// through its own supported path.
//
// The failure is accretion, not an adversary. There is no delete method — a
// filter change mints a new FlatKey and the old lineage "simply goes cold" in
// the file forever — so a long-lived host that varies filters only ever adds
// entries. Refusing the write instead degrades to the outcome the seam already
// documents for a failed save: reported through Observer.CheckpointSaveFailed,
// the connector continues, and the last usable file is still there.
func TestFileStore_SaveRefusesToWritePastTheReadLimit(t *testing.T) {
	ctx := context.Background()
	store := NewFileCheckpointStore(storePath(t))
	if err := store.Save(ctx, storeKey("first"), "pos-keep"); err != nil {
		t.Fatalf("seeding the store: %v", err)
	}

	// One position past the cap on its own: the smallest input that makes the
	// marshaled file exceed the limit, without staging millions of lineages.
	oversized := strings.Repeat("p", maxCheckpointStoreBytes+1)
	err := store.Save(ctx, storeKey("second"), oversized)
	if err == nil {
		t.Fatal("Save wrote a store past the read limit; it must refuse before replacing the last usable file")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("Save error = %v, want a size-limit refusal", err)
	}
	// The refusal must not echo the position it declined.
	if strings.Contains(err.Error(), "pppppppppp") {
		t.Errorf("Save error echoed the position: %v", err)
	}

	// The point of refusing: the previous file is intact and still readable.
	got, ok, loadErr := store.Load(ctx, storeKey("first"))
	if loadErr != nil {
		t.Fatalf("Load after the refused Save = %v, want the previous file intact", loadErr)
	}
	if !ok || got != "pos-keep" {
		t.Errorf("Load after the refused Save = (%q, %v), want (\"pos-keep\", true)", got, ok)
	}
	// And a subsequent ordinary Save still works — the store is not wedged.
	if err := store.Save(ctx, storeKey("third"), "pos-3"); err != nil {
		t.Errorf("Save after the refused Save = %v, want the store still usable", err)
	}
}

// A file just under the cap still loads: the bound must not be so eager that
// it refuses a legitimate store.
func TestFileStore_AcceptsFileUnderTheLimit(t *testing.T) {
	dir := t.TempDir()
	store := NewFileCheckpointStore(filepath.Join(dir, "feed.json"))
	key := CheckpointKey{Origin: "https://3.basecampapi.com", AccountID: "5951425", ConsumerNamespace: "openclaw", FilterKey: "srv1-0000000000000000"}
	if err := store.Save(context.Background(), key, "pos-1"); err != nil {
		t.Fatal(err)
	}
	// Pad with additional lineages until the file is substantial but legal.
	for i := range 500 {
		k := key
		k.ConsumerNamespace = fmt.Sprintf("consumer-%04d", i)
		if err := store.Save(context.Background(), k, fmt.Sprintf("pos-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := store.Load(context.Background(), key)
	if err != nil {
		t.Fatalf("Load = %v, want the stored position", err)
	}
	if !ok || got != "pos-1" {
		t.Errorf("Load = (%q, %v), want (\"pos-1\", true)", got, ok)
	}
}
