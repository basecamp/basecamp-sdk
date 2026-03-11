package types

import (
	"encoding/json"
	"fmt"
	"math"
)

// FlexInt is an int32 that unmarshals from any JSON number whose value
// is integral and fits in 32 bits. The BC3 API serializes pixel
// dimensions as floats (e.g. 1024.0); Go's encoding/json rejects those
// into plain int fields. FlexInt bridges this wire-format mismatch
// without lying in the spec.
//
// Non-integral values (1024.5) and out-of-range values (1e20) are
// rejected to match the int32 schema in openapi.json.
type FlexInt int32

// UnmarshalJSON accepts any JSON number whose value is an integer
// within int32 range.
func (fi *FlexInt) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	if f != math.Trunc(f) {
		return fmt.Errorf("flexint: %s is not an integer", string(data))
	}
	if f < math.MinInt32 || f > math.MaxInt32 {
		return fmt.Errorf("flexint: %s overflows int32", string(data))
	}
	*fi = FlexInt(f)
	return nil
}

// MarshalJSON writes the value as an integer.
func (fi FlexInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(int32(fi))
}
