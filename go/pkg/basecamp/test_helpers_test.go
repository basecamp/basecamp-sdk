package basecamp

import (
	"bytes"
	"encoding/json"
)

// unmarshalWithNumbers decodes JSON into a map preserving numbers as json.Number
// which can be cleanly converted to int64 without float64 precision loss.
// This is useful for testing JSON serialization where large IDs need to be preserved exactly.
func unmarshalWithNumbers(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return result, decoder.Decode(&result)
}
