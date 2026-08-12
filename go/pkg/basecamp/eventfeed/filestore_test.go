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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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
