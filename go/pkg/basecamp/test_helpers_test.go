package basecamp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

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
