package basecamp

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
)

// normalizePersonIds walks a JSON-decoded value and normalizes Person-shaped
// objects. When an object has a "personable_type" field and its "id" is a
// string, the id is coerced to a number (0 for non-numeric sentinels like
// "basecamp") and the original label is preserved as "system_label".
// Numeric overflow strings are left as-is (the downstream FlexibleInt64
// decoder will reject them).
func normalizePersonIds(v any) {
	switch val := v.(type) {
	case map[string]any:
		if _, hasPT := val["personable_type"]; hasPT {
			if idStr, ok := val["id"].(string); ok {
				n, err := strconv.ParseInt(idStr, 10, 64)
				if err == nil {
					val["id"] = json.Number(idStr) // preserve as json.Number
				} else {
					var numErr *strconv.NumError
					if errors.As(err, &numErr) && numErr.Err == strconv.ErrRange {
						// Numeric overflow — leave as string, let decoder reject
					} else {
						// Non-numeric sentinel
						val["system_label"] = idStr
						val["id"] = json.Number("0")
					}
				}
				_ = n // used only for parse check
			}
		}
		for _, child := range val {
			normalizePersonIds(child)
		}
	case []any:
		for _, item := range val {
			normalizePersonIds(item)
		}
	}
}

// normalizeJSON parses raw JSON, normalizes Person-shaped objects, and
// re-serializes. Uses json.Number to preserve integer precision.
func normalizeJSON(data []byte) ([]byte, error) {
	// Short-circuit: skip the parse/re-serialize if no Person-shaped objects
	if !bytes.Contains(data, []byte(`"personable_type"`)) {
		return data, nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var raw any
	if err := dec.Decode(&raw); err != nil {
		return data, err
	}
	normalizePersonIds(raw)
	return json.Marshal(raw)
}
