package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// FlexibleInt64 is an int64 that unmarshals from either a JSON number or a
// JSON string containing an integer. The BC3 API sometimes serializes person
// IDs as strings (e.g. "12345") in notification responses while returning
// plain integers elsewhere. FlexibleInt64 bridges this wire-format mismatch
// without lying in the spec.
//
// Non-integral JSON numbers (1024.5) and values outside int64 range are
// rejected. Non-numeric strings (e.g. "basecamp" for system-generated
// entities) unmarshal to zero — the spec declares the field as integer,
// so a non-numeric value means the entity has no meaningful numeric ID.
// The number path uses json.Number to avoid float64 precision loss for
// values beyond 2^53.
type FlexibleInt64 int64

// UnmarshalJSON accepts a JSON number or a JSON string whose value is an
// integer within int64 range.
func (fi *FlexibleInt64) UnmarshalJSON(data []byte) error {
	// JSON string path (e.g. "12345").
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("flexibleint64: cannot parse %s: %w", string(data), err)
		}
		n, parseErr := strconv.ParseInt(s, 10, 64)
		if parseErr == nil {
			*fi = FlexibleInt64(n)
			return nil
		}
		// Distinguish numeric overflow (reject) from non-numeric sentinels (zero).
		// "9223372036854775808" is a range error — caller should know.
		// "basecamp" is a non-numeric sentinel for system-generated entities.
		var numErr *strconv.NumError
		if errors.As(parseErr, &numErr) && numErr.Err == strconv.ErrRange {
			return fmt.Errorf("flexibleint64: %q overflows int64", s)
		}
		*fi = 0
		return nil
	}

	// JSON number path — use json.Decoder with UseNumber to preserve
	// full int64 precision (float64 silently truncates beyond 2^53).
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var num json.Number
	if err := dec.Decode(&num); err != nil {
		return fmt.Errorf("flexibleint64: cannot parse %s: expected number or string", string(data))
	}
	n, err := num.Int64()
	if err != nil {
		return fmt.Errorf("flexibleint64: %s is not a valid int64: %w", string(data), err)
	}
	*fi = FlexibleInt64(n)
	return nil
}

// MarshalJSON writes the value as a JSON integer.
func (fi FlexibleInt64) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(fi))
}
