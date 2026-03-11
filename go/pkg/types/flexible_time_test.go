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
