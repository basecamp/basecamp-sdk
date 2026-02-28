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

func TestCache_CorruptedEtagsJSON(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)

	// Write corrupted etags.json
	if err := os.WriteFile(filepath.Join(dir, "etags.json"), []byte("NOT JSON"), 0600); err != nil {
		t.Fatalf("writing corrupted file: %v", err)
	}

	// GetETag should return empty on corruption
	if got := c.GetETag("any-key"); got != "" {
		t.Errorf("GetETag with corrupted file = %q, want empty", got)
	}

	// Set should overwrite the corrupted file
	if err := c.Set("new-key", []byte("body"), `"fresh"`); err != nil {
		t.Fatalf("Set after corruption: %v", err)
	}

	if got := c.GetETag("new-key"); got != `"fresh"` {
		t.Errorf("GetETag after fix = %q, want %q", got, `"fresh"`)
	}
}
