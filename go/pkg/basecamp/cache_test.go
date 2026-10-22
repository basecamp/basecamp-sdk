package basecamp

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCache_SetAndGetETag(t *testing.T) {
	c := NewCache(t.TempDir())
	key := c.Key("https://example.com/todos", "123", "token")

	if err := c.Set(key, []byte(`{"ok":true}`), `"abc123"`); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got := c.GetETag(key)
	if got != `"abc123"` {
		t.Errorf("GetETag = %q, want %q", got, `"abc123"`)
	}
}

func TestCache_GetETag_Miss(t *testing.T) {
	c := NewCache(t.TempDir())
	if got := c.GetETag("nonexistent"); got != "" {
		t.Errorf("GetETag miss = %q, want empty", got)
	}
}

func TestCache_GetBody(t *testing.T) {
	c := NewCache(t.TempDir())
	key := "test-key"
	body := []byte(`[{"id":1}]`)

	if err := c.Set(key, body, `"etag"`); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got := c.GetBody(key)
	if string(got) != string(body) {
		t.Errorf("GetBody = %q, want %q", got, body)
	}
}

func TestCache_GetBody_Miss(t *testing.T) {
	c := NewCache(t.TempDir())
	if got := c.GetBody("nonexistent"); got != nil {
		t.Errorf("GetBody miss = %v, want nil", got)
	}
}

func TestCache_Clear(t *testing.T) {
	c := NewCache(t.TempDir())
	key := "clear-key"

	if err := c.Set(key, []byte("data"), `"e"`); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := c.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if got := c.GetETag(key); got != "" {
		t.Errorf("GetETag after Clear = %q, want empty", got)
	}
	if got := c.GetBody(key); got != nil {
		t.Errorf("GetBody after Clear = %v, want nil", got)
	}
}

func TestCache_Invalidate(t *testing.T) {
	c := NewCache(t.TempDir())
	k1 := "key-1"
	k2 := "key-2"

	_ = c.Set(k1, []byte("body1"), `"e1"`)
	_ = c.Set(k2, []byte("body2"), `"e2"`)

	if err := c.Invalidate(k1); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	if got := c.GetETag(k1); got != "" {
		t.Errorf("GetETag invalidated = %q, want empty", got)
	}
	if got := c.GetBody(k1); got != nil {
		t.Errorf("GetBody invalidated = %v, want nil", got)
	}

	// k2 should still be present
	if got := c.GetETag(k2); got != `"e2"` {
		t.Errorf("GetETag k2 = %q, want %q", got, `"e2"`)
	}
}

func TestCache_NamespaceSeparation(t *testing.T) {
	c := NewCache(t.TempDir())

	k1 := c.Key("https://example.com/todos", "acct1", "tokenA")
	k2 := c.Key("https://example.com/todos", "acct2", "tokenA")
	k3 := c.Key("https://example.com/todos", "acct1", "tokenB")

	if k1 == k2 {
		t.Error("different accounts should produce different keys")
	}
	if k1 == k3 {
		t.Error("different tokens should produce different keys")
	}
}

func TestCache_ConcurrentSetAndGet(t *testing.T) {
	c := NewCache(t.TempDir())
	var wg sync.WaitGroup

	for range 20 {
		wg.Add(2)
		key := "concurrent-key"
		body := []byte("body")
		etag := `"e"`

		go func() {
			defer wg.Done()
			_ = c.Set(key, body, etag)
		}()

		go func() {
			defer wg.Done()
			_ = c.GetETag(key)
			_ = c.GetBody(key)
		}()
	}

	wg.Wait()
}

func TestCache_CorruptedEntry(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)

	// An entry with no ETag line is a miss, not a body
	if err := os.MkdirAll(filepath.Join(dir, "responses"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "responses", "any-key.entry"), []byte("NO NEWLINE"), 0600); err != nil {
		t.Fatalf("writing corrupted file: %v", err)
	}
	if etag, body := c.GetETag("any-key"), c.GetBody("any-key"); etag != "" || body != nil {
		t.Errorf("corrupted entry = (%q, %q), want a miss", etag, body)
	}

	// Set should overwrite the corrupted file
	if err := c.Set("any-key", []byte("body"), `"fresh"`); err != nil {
		t.Fatalf("Set after corruption: %v", err)
	}
	if got := c.GetETag("any-key"); got != `"fresh"` {
		t.Errorf("GetETag after fix = %q, want %q", got, `"fresh"`)
	}
}

func TestCache_SetRefusesAnETagThatCantBeStored(t *testing.T) {
	c := NewCache(t.TempDir())
	for _, etag := range []string{"", "\"a\nb\"", "\"a\rb\""} {
		if err := c.Set("k", []byte("body"), etag); err == nil {
			t.Errorf("Set with ETag %q succeeded, want an error", etag)
		}
	}
}

func TestCache_ReadsTheOldLayoutAsAMiss(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "responses"), 0700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "etags.json"), []byte(`{"k":"\"old\""}`), 0600)
	_ = os.WriteFile(filepath.Join(dir, "responses", "k.body"), []byte("old body"), 0600)

	c := NewCache(dir)
	if etag, body := c.entry("k"); etag != "" || body != nil {
		t.Errorf("old layout = (%q, %q), want a miss", etag, body)
	}
	if err := c.Set("k", []byte("new body"), `"new"`); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "responses", "k.body")); !os.IsNotExist(err) {
		t.Errorf("Set left the old layout's body behind: %v", err)
	}
}
