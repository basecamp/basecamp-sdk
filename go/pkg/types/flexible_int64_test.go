package types

import (
	"encoding/json"
	"testing"
)

func TestFlexibleInt64_UnmarshalInt(t *testing.T) {
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte("12345"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 12345 {
		t.Errorf("expected 12345, got %d", fi)
	}
}

func TestFlexibleInt64_UnmarshalLargeNumericLiteral(t *testing.T) {
	// 2^53 + 1 as a JSON number (not string). json.Number preserves this
	// without float64 truncation.
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte("9007199254740993"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 9007199254740993 {
		t.Errorf("expected 9007199254740993, got %d", fi)
	}
}

func TestFlexibleInt64_UnmarshalString(t *testing.T) {
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte(`"12345"`), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 12345 {
		t.Errorf("expected 12345, got %d", fi)
	}
}

func TestFlexibleInt64_UnmarshalZero(t *testing.T) {
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte("0"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 0 {
		t.Errorf("expected 0, got %d", fi)
	}
}

func TestFlexibleInt64_UnmarshalNegative(t *testing.T) {
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte("-1"), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != -1 {
		t.Errorf("expected -1, got %d", fi)
	}
}

func TestFlexibleInt64_UnmarshalLargeViaString(t *testing.T) {
	// 2^53 + 1 — beyond float64 safe-integer range, must arrive as string.
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte(`"9007199254740993"`), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 9007199254740993 {
		t.Errorf("expected 9007199254740993, got %d", fi)
	}
}

func TestFlexibleInt64_RejectsFractional(t *testing.T) {
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte("1024.5"), &fi); err == nil {
		t.Fatalf("expected error for fractional value, got %d", fi)
	}
}

func TestFlexibleInt64_RejectsOverflow(t *testing.T) {
	// One beyond max int64 as a JSON number.
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte("9223372036854775808"), &fi); err == nil {
		t.Fatalf("expected error for numeric overflow, got %d", fi)
	}
}

func TestFlexibleInt64_RejectsStringOverflow(t *testing.T) {
	// One beyond max int64 as a JSON string — must error, not zero.
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte(`"9223372036854775808"`), &fi); err == nil {
		t.Fatalf("expected error for string overflow, got %d", fi)
	}
}

func TestFlexibleInt64_NonNumericStringZeros(t *testing.T) {
	// System-generated entities use sentinel strings like "basecamp" as
	// person IDs. These should unmarshal to zero, not error.
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte(`"basecamp"`), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != 0 {
		t.Errorf("expected 0 for non-numeric string, got %d", fi)
	}
}

func TestFlexibleInt64_OmittedField(t *testing.T) {
	type wrapper struct {
		ID FlexibleInt64 `json:"id,omitempty"`
	}
	var w wrapper
	if err := json.Unmarshal([]byte(`{}`), &w); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.ID != 0 {
		t.Errorf("expected 0 for omitted field, got %d", w.ID)
	}
}

func TestFlexibleInt64_MarshalJSON(t *testing.T) {
	fi := FlexibleInt64(12345)
	data, err := json.Marshal(fi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "12345" {
		t.Errorf("expected %q, got %q", "12345", string(data))
	}
}

func TestFlexibleInt64_RoundTrip(t *testing.T) {
	// Unmarshal from string, marshal back as integer.
	var fi FlexibleInt64
	if err := json.Unmarshal([]byte(`"42"`), &fi); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := json.Marshal(fi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "42" {
		t.Errorf("expected %q, got %q", "42", string(data))
	}
}
