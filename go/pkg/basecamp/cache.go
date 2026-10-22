package basecamp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Cache provides ETag-based HTTP caching.
type Cache struct {
	dir string
	mu  sync.RWMutex
}

// NewCache creates a new cache with the given directory.
func NewCache(dir string) *Cache {
	return &Cache{dir: dir}
}

// Key generates a cache key for a URL, account, and token.
// The key includes a token hash to ensure different auth contexts don't share cache.
func (c *Cache) Key(url, accountID, token string) string {
	tokenHash := ""
	if token != "" {
		h := sha256.Sum256([]byte(token))
		tokenHash = hex.EncodeToString(h[:8]) // First 16 chars
	}

	input := url + ":" + accountID + ":" + tokenHash
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

// Each entry is one file, responses/<key>.entry, holding the ETag, a newline,
// and the body. It is written to a uniquely named temporary file and renamed
// into place, so a reader in any process sees a whole entry or none, and two
// writers racing on one key leave one of their entries intact rather than one
// response's ETag beside another's body. That pairing would be persistent: the
// server keeps answering the newer ETag with 304, and the older body is served
// until the resource changes again. (Before #1030 the ETags lived together in
// etags.json and the bodies in responses/<key>.body, published separately.)

// GetETag returns the cached ETag for a key, or empty string if not found.
func (c *Cache) GetETag(key string) string {
	etag, _ := c.entry(key)
	return etag
}

// GetBody returns the cached response body for a key, or nil if not found.
func (c *Cache) GetBody(key string) []byte {
	_, body := c.entry(key)
	return body
}

// entry returns a key's ETag and body, both from one stored response. Both are
// empty when the key has no entry.
func (c *Cache) entry(key string) (etag string, body []byte) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.entryFile(key)) // #nosec G703 -- cache dir is caller-configured
	if err != nil {
		return "", nil
	}
	i := bytes.IndexByte(data, '\n')
	if i < 1 {
		return "", nil
	}
	return string(data[:i]), data[i+1:]
}

// Set stores a response body and ETag for a key.
func (c *Cache) Set(key string, body []byte, etag string) error {
	if etag == "" || strings.ContainsAny(etag, "\r\n") {
		return fmt.Errorf("basecamp: cache: unusable ETag %q", etag)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	responsesDir := filepath.Join(c.dir, "responses")
	if err := os.MkdirAll(responsesDir, 0700); err != nil { // #nosec G703 -- cache dir is caller-configured
		return err
	}

	tmp, err := os.CreateTemp(responsesDir, key+".*.tmp") // 0600
	if err != nil {
		return err
	}
	_, err = tmp.Write(append([]byte(etag+"\n"), body...))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), c.entryFile(key)) // #nosec G703 -- cache dir is caller-configured
	}
	if err != nil {
		_ = os.Remove(tmp.Name()) // #nosec G703 -- cache dir is caller-configured
		return err
	}

	// Drop this key's body in the old layout, if an earlier version left one.
	_ = os.Remove(filepath.Join(responsesDir, key+".body")) // #nosec G703 -- cache dir is caller-configured
	return nil
}

// Clear removes all cached data.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.RemoveAll(filepath.Join(c.dir, "responses")); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(c.dir, "etags.json")) // the old layout's index

	return nil
}

// Invalidate removes cached data for a specific key.
func (c *Cache) Invalidate(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.Remove(c.entryFile(key)); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = os.Remove(filepath.Join(c.dir, "responses", key+".body")) // the old layout
	return nil
}

func (c *Cache) entryFile(key string) string {
	return filepath.Join(c.dir, "responses", key+".entry")
}
