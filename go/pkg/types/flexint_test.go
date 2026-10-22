package types

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestFlexInt_UnmarshalInt(t *testing.T) {
	var fi FlexInt
	if err := json.Unmarshal([]byte("1024"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 1024 {
		t.Errorf("expected 1024, got %d", fi)
	}
}

func TestFlexInt_UnmarshalFloat(t *testing.T) {
	var fi FlexInt
	if err := json.Unmarshal([]byte("1024.0"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 1024 {
		t.Errorf("expected 1024, got %d", fi)
	}
}

func TestFlexInt_UnmarshalZero(t *testing.T) {
	var fi FlexInt
	if err := json.Unmarshal([]byte("0"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 0 {
		t.Errorf("expected 0, got %d", fi)
	}
}

func TestFlexInt_UnmarshalNegative(t *testing.T) {
	var fi FlexInt
	if err := json.Unmarshal([]byte("-1"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != -1 {
		t.Errorf("expected -1, got %d", fi)
	}
}

func TestFlexInt_RejectsFractional(t *testing.T) {
	var fi FlexInt
	if err := json.Unmarshal([]byte("1024.5"), &fi); err == nil {
		t.Fatalf("expected error for fractional value, got %d", fi)
	}
}

func TestFlexInt_RejectsOverflow(t *testing.T) {
	var fi FlexInt
	if err := json.Unmarshal([]byte("1e20"), &fi); err == nil {
		t.Fatalf("expected error for overflow, got %d", fi)
	}
}

func TestFlexInt_RejectsNegativeOverflow(t *testing.T) {
	var fi FlexInt
	if err := json.Unmarshal([]byte("-3000000000"), &fi); err == nil {
		t.Fatalf("expected error for negative overflow, got %d", fi)
	}
}

func TestFlexInt_OmittedField(t *testing.T) {
	type wrapper struct {
		W FlexInt `json:"w,omitempty"`
	}
	var w wrapper
	if err := json.Unmarshal([]byte(`{}`), &w); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.W != 0 {
		t.Errorf("expected 0 for omitted field, got %d", w.W)
	}
}

func TestFlexInt_MarshalJSON(t *testing.T) {
	fi := FlexInt(1024)
	data, err := json.Marshal(fi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "1024" {
		t.Errorf("expected %q, got %q", "1024", string(data))
	}
}

func TestFlexInt_RoundTrip(t *testing.T) {
	// Unmarshal from float, marshal back as int
	var fi FlexInt
	if err := json.Unmarshal([]byte("768.0"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := json.Marshal(fi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "768" {
		t.Errorf("expected %q, got %q", "768", string(data))
	}
}

// The API computes some dimensions in floating point and can serve an
// integer one ulp off (Sentry BC-MCP-K/M: 752.0000000000001 failed every
// list_uploads, search and timeline call that carried it). Noise rounds to
// the integer; a real fraction, a non-number or an out-of-range value is
// still refused.
func TestFlexInt_UnmarshalTable(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want FlexInt
		err  string
	}{
		{in: "752", want: 752},
		{in: "752.0", want: 752},
		{in: "752.0000000000001", want: 752},
		{in: "496.00000000000006", want: 496},
		{in: "751.9999999999999", want: 752},
		{in: "0.0000000000001", want: 0},
		{in: "-0.0000000000001", want: 0},
		{in: "-752.0000000000001", want: -752},
		{in: "7.52e2", want: 752},
		{in: "752.0000009", want: 752},
		{in: "2147483647", want: math.MaxInt32},
		{in: "2147483647.0000002", want: math.MaxInt32},
		{in: "-2147483648", want: math.MinInt32},
		{in: "-2147483648.0000002", want: math.MinInt32},

		{in: "752.5", err: "flexint: 752.5 is not an integer"},
		{in: "752.001", err: "flexint: 752.001 is not an integer"},
		{in: "752.0000011", err: "flexint: 752.0000011 is not an integer"},
		{in: "-752.5", err: "flexint: -752.5 is not an integer"},
		{in: "0.5", err: "flexint: 0.5 is not an integer"},
		{in: "2147483647.5", err: "flexint: 2147483647.5 is not an integer"},
		{in: "2147483648", err: "flexint: 2147483648 overflows int32"},
		{in: "2147483647.9999998", err: "flexint: 2147483647.9999998 overflows int32"},
		{in: "-2147483649", err: "flexint: -2147483649 overflows int32"},
		// Ids are not FlexInt (they are int64, read by FlexibleInt64 without
		// float64), so an id-sized value here is a schema violation: refuse
		// it rather than round it into a wrong int32.
		{in: "9007199254740993", err: "flexint: 9007199254740993 overflows int32"},
		{in: "1e20", err: "flexint: 1e20 overflows int32"},
		{in: `"752"`, err: "cannot unmarshal string"},
		{in: "true", err: "cannot unmarshal bool"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			fi := FlexInt(-1)
			err := json.Unmarshal([]byte(tc.in), &fi)
			if tc.err == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if fi != tc.want {
					t.Errorf("got %d, want %d", fi, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got %d", tc.err, fi)
			}
			if !strings.Contains(err.Error(), tc.err) {
				t.Errorf("error %q does not contain %q", err, tc.err)
			}
			if fi != -1 {
				t.Errorf("a refused value must leave the target untouched, got %d", fi)
			}
		})
	}
}

// The case as served: an upload whose width is float noise decodes, through a
// nullable pointer field like the generated types use.
func TestFlexInt_FloatNoiseInsideAPayload(t *testing.T) {
	var upload struct {
		Width  *FlexInt `json:"width"`
		Height *FlexInt `json:"height"`
	}
	if err := json.Unmarshal([]byte(`{"width":752.0000000000001,"height":496.00000000000006}`), &upload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upload.Width == nil || *upload.Width != 752 || upload.Height == nil || *upload.Height != 496 {
		t.Errorf("got width=%v height=%v, want 752x496", upload.Width, upload.Height)
	}
}
