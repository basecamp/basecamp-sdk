package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFlexibleTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSec int64 // expected Unix seconds (0 = check IsZero)
		wantErr bool
	}{
		{"RFC3339", `"2022-11-01T10:00:00Z"`, 1667296800, false},
		{"RFC3339Nano millis", `"2022-11-01T10:00:00.000Z"`, 1667296800, false},
		{"RFC3339Nano micros", `"2022-11-01T10:00:00.123456Z"`, 1667296800, false},
		{"date-only", `"2022-11-15"`, 1668470400, false},
		{"null", `null`, 0, false},
		{"empty string", `""`, 0, false},
		{"invalid", `"not-a-date"`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ft FlexibleTime
			err := json.Unmarshal([]byte(tt.input), &ft)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantSec == 0 {
				if !ft.IsZero() {
					t.Errorf("expected zero time, got %v", ft.Time)
				}
			} else {
				if ft.Unix() != tt.wantSec {
					t.Errorf("expected Unix %d, got %d (%v)", tt.wantSec, ft.Unix(), ft.Time)
				}
			}
		})
	}
}

func TestFlexibleTime_MarshalJSON(t *testing.T) {
	t.Run("zero time", func(t *testing.T) {
		ft := FlexibleTime{}
		data, err := json.Marshal(ft)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "null" {
			t.Errorf("expected null, got %s", data)
		}
	})

	t.Run("non-zero time", func(t *testing.T) {
		ft := FlexibleTime{Time: time.Date(2022, 11, 1, 10, 0, 0, 0, time.UTC)}
		data, err := json.Marshal(ft)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != `"2022-11-01T10:00:00Z"` {
			t.Errorf("expected RFC3339, got %s", data)
		}
	})
}

func TestFlexibleTime_DateOnlyMidnightUTC(t *testing.T) {
	var ft FlexibleTime
	if err := json.Unmarshal([]byte(`"2026-03-02"`), &ft); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ft.Hour() != 0 || ft.Minute() != 0 || ft.Second() != 0 {
		t.Errorf("expected midnight, got %v", ft.Time)
	}
	if ft.Location() != time.UTC {
		t.Errorf("expected UTC, got %v", ft.Location())
	}
	if ft.Year() != 2026 || ft.Month() != time.March || ft.Day() != 2 {
		t.Errorf("expected 2026-03-02, got %v", ft.Time)
	}
}

// Both directions, because a one-directional test passes today: the bare
// date must come back as a bare date (this failed before the fix), and a real
// timestamp must come back unchanged (this guards against "fixing" it by
// always emitting a date).
func TestFlexibleTime_RoundTripPreservesTheParsedForm(t *testing.T) {
	tests := []struct {
		name     string
		wire     string
		dateOnly bool
	}{
		{"bare date", `"2026-08-04"`, true},
		{"bare date at Go's zero time", `"0001-01-01"`, true},
		{"UTC timestamp", `"2026-08-04T00:00:00Z"`, false},
		{"offset timestamp", `"2026-08-04T09:30:00-07:00"`, false},
		{"fractional timestamp", `"2022-11-01T10:00:00.123456Z"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ft FlexibleTime
			if err := json.Unmarshal([]byte(tt.wire), &ft); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ft.DateOnly() != tt.dateOnly {
				t.Errorf("DateOnly() = %v, want %v", ft.DateOnly(), tt.dateOnly)
			}
			out, err := json.Marshal(ft)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(out) != tt.wire {
				t.Errorf("round trip rewrote %s as %s", tt.wire, out)
			}
		})
	}
}

// The form is a property of the last decode, not of the variable: a value
// reused across decodes must not carry a stale bare-date flag into a
// timestamp, or a stale timestamp into a null.
func TestFlexibleTime_ReuseResetsTheParsedForm(t *testing.T) {
	var ft FlexibleTime
	for _, wire := range []string{`"2026-08-04"`, `"2026-08-04T10:00:00Z"`, `"2026-08-05"`, `null`} {
		if err := json.Unmarshal([]byte(wire), &ft); err != nil {
			t.Fatalf("unexpected error decoding %s: %v", wire, err)
		}
		out, err := json.Marshal(ft)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(out) != wire {
			t.Errorf("after decoding %s, marshal gave %s", wire, out)
		}
	}
	if ft.DateOnly() {
		t.Error("a null must not read as date-only")
	}
}

// The flag must follow every decode path, not only JSON: a FlexibleTime that
// decoded a bare date and is then fed a timestamp through UnmarshalText (the
// path text-based decoders and map keys take) must not keep the date flag.
func TestFlexibleTime_TextRoundTripPreservesTheParsedForm(t *testing.T) {
	var ft FlexibleTime
	if err := json.Unmarshal([]byte(`"2026-08-04"`), &ft); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ft.UnmarshalText([]byte("2026-08-04T10:00:00Z")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ft.DateOnly() {
		t.Error("UnmarshalText of a timestamp left the bare-date flag set")
	}
	out, err := json.Marshal(ft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != `"2026-08-04T10:00:00Z"` {
		t.Errorf("expected the timestamp back, got %s", out)
	}

	if err := ft.UnmarshalText([]byte("2026-08-05")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text, err := ft.MarshalText()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(text) != "2026-08-05" {
		t.Errorf("expected MarshalText to emit the bare date, got %s", text)
	}
}
