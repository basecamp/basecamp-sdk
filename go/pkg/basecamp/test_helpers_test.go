package basecamp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// loadUploadFixture parses spec/fixtures/uploads/get.json, substitutes the
// scheme+host of its download_url with testHost, and returns the rewritten
// metadata JSON body plus the path portion of the substituted download_url
// (e.g. "/999999999/blobs/abcd1234/download/logo.png").
//
// Hand-rolled Download tests use this to stay aligned with the canonical
// API response shape rather than inventing their own shape — the original
// UploadsService.Download bug survived CI because its tests invented a
// shape the server never returns.
func loadUploadFixture(t *testing.T, testHost string) (metadataBody []byte, downloadPath string) {
	t.Helper()
	fixturePath := filepath.Join("..", "..", "..", "spec", "fixtures", "uploads", "get.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("loadUploadFixture: read %s: %v", fixturePath, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("loadUploadFixture: parse: %v", err)
	}
	rawURL, ok := obj["download_url"].(string)
	if !ok {
		t.Fatalf("loadUploadFixture: download_url missing or not a string")
	}
	orig, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("loadUploadFixture: parse download_url %q: %v", rawURL, err)
	}
	host, err := url.Parse(testHost)
	if err != nil {
		t.Fatalf("loadUploadFixture: parse testHost %q: %v", testHost, err)
	}
	orig.Scheme = host.Scheme
	orig.Host = host.Host
	obj["download_url"] = orig.String()
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("loadUploadFixture: marshal: %v", err)
	}
	return out, orig.Path
}

// unmarshalWithNumbers decodes JSON into a map preserving numbers as json.Number
// which can be cleanly converted to int64 without float64 precision loss.
// This is useful for testing JSON serialization where large IDs need to be preserved exactly.
func unmarshalWithNumbers(data []byte) (map[string]any, error) {
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return result, decoder.Decode(&result)
}

// decodeRequestBody reads and JSON-decodes an HTTP request body into a map,
// preserving numbers as json.Number. Fails the test on any error.
func decodeRequestBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}
	return m
}

// intPtr returns a pointer to an int value. Used in tests for *int fields.
func intPtr(v int) *int { return &v }
