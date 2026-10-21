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
// The API computes some dimensions in floating point, so a value that is
// an integer in fact can arrive one ulp off it (752.0000000000001,
// 496.00000000000006). A number within FlexIntTolerance of an integer is
// that integer. Anything further off (1024.5, 1024.001) is a real
// fraction, and values outside int32 range (1e20) overflow; both are
// rejected to match the int32 schema in openapi.json.
type FlexInt int32

// FlexIntTolerance is how far from an integer a JSON number may sit and
// still read as that integer. It absorbs floating-point noise, which is
// at most a few ulps: an ulp is under 1e-13 at typical pixel sizes and
// under 5e-7 even at the int32 limit, so 1e-6 covers it everywhere in
// range while staying far below any fraction a dimension could mean.
const FlexIntTolerance = 1e-6

// UnmarshalJSON accepts any JSON number within FlexIntTolerance of an
// integer within int32 range.
func (fi *FlexInt) UnmarshalJSON(data []byte) error {
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	n := math.Round(f)
	if math.Abs(f-n) > FlexIntTolerance {
		return fmt.Errorf("flexint: %s is not an integer", string(data))
	}
	if n < math.MinInt32 || n > math.MaxInt32 {
		return fmt.Errorf("flexint: %s overflows int32", string(data))
	}
	*fi = FlexInt(n)
	return nil
}

// MarshalJSON writes the value as an integer.
func (fi FlexInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(int32(fi))
}
