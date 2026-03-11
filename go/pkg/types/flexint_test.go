package types

import (
	"encoding/json"
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
