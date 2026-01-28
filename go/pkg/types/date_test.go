package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		input    string
		expected Date
		wantErr  bool
	}{
		{"2024-01-15", Date{2024, time.January, 15}, false},
		{"2024-12-31", Date{2024, time.December, 31}, false},
		{"1999-06-01", Date{1999, time.June, 1}, false},
		{"", Date{}, false},
		{"invalid", Date{}, true},
		{"2024-13-01", Date{}, true}, // invalid month
		{"2024-01-32", Date{}, true}, // invalid day
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseDate(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDateString(t *testing.T) {
	tests := []struct {
		date     Date
		expected string
	}{
		{Date{2024, time.January, 15}, "2024-01-15"},
		{Date{2024, time.December, 31}, "2024-12-31"},
		{Date{999, time.June, 1}, "0999-06-01"},
		{Date{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.date.String(); got != tt.expected {
				t.Errorf("Date.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDateIsZero(t *testing.T) {
	var zero Date
	if !zero.IsZero() {
		t.Error("zero Date should be zero")
	}
	d := Date{2024, time.January, 1}
	if d.IsZero() {
		t.Error("non-zero Date should not be zero")
	}
}

func TestDateComparisons(t *testing.T) {
	d1 := Date{2024, time.January, 15}
	d2 := Date{2024, time.January, 16}
	d3 := Date{2024, time.February, 15}
	d4 := Date{2025, time.January, 15}

	// Before
	if !d1.Before(d2) {
		t.Error("d1 should be before d2")
	}
	if !d1.Before(d3) {
		t.Error("d1 should be before d3")
	}
	if !d1.Before(d4) {
		t.Error("d1 should be before d4")
	}
	if d2.Before(d1) {
		t.Error("d2 should not be before d1")
	}

	// After
	if !d2.After(d1) {
		t.Error("d2 should be after d1")
	}

	// Equal
	if !d1.Equal(Date{2024, time.January, 15}) {
		t.Error("d1 should equal itself")
	}
	if d1.Equal(d2) {
		t.Error("d1 should not equal d2")
	}

	// Compare
	if d1.Compare(d2) != -1 {
		t.Error("d1.Compare(d2) should be -1")
	}
	if d2.Compare(d1) != 1 {
		t.Error("d2.Compare(d1) should be 1")
	}
	if d1.Compare(d1) != 0 {
		t.Error("d1.Compare(d1) should be 0")
	}
}

func TestDateArithmetic(t *testing.T) {
	d := Date{2024, time.January, 15}

	// AddDays
	if got := d.AddDays(10); got != (Date{2024, time.January, 25}) {
		t.Errorf("AddDays(10) = %v, want 2024-01-25", got)
	}
	if got := d.AddDays(-10); got != (Date{2024, time.January, 5}) {
		t.Errorf("AddDays(-10) = %v, want 2024-01-05", got)
	}

	// AddMonths
	if got := d.AddMonths(1); got != (Date{2024, time.February, 15}) {
		t.Errorf("AddMonths(1) = %v, want 2024-02-15", got)
	}

	// AddYears
	if got := d.AddYears(1); got != (Date{2025, time.January, 15}) {
		t.Errorf("AddYears(1) = %v, want 2025-01-15", got)
	}

	// DaysSince
	d2 := Date{2024, time.January, 20}
	if got := d2.DaysSince(d); got != 5 {
		t.Errorf("DaysSince = %d, want 5", got)
	}
}

func TestDateJSON(t *testing.T) {
	type wrapper struct {
		DueOn Date `json:"due_on"`
	}

	t.Run("marshal", func(t *testing.T) {
		w := wrapper{DueOn: Date{2024, time.January, 15}}
		data, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		expected := `{"due_on":"2024-01-15"}`
		if string(data) != expected {
			t.Errorf("got %s, want %s", data, expected)
		}
	})

	t.Run("marshal zero", func(t *testing.T) {
		w := wrapper{}
		data, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		expected := `{"due_on":null}`
		if string(data) != expected {
			t.Errorf("got %s, want %s", data, expected)
		}
	})

	t.Run("unmarshal", func(t *testing.T) {
		var w wrapper
		err := json.Unmarshal([]byte(`{"due_on":"2024-01-15"}`), &w)
		if err != nil {
			t.Fatal(err)
		}
		expected := Date{2024, time.January, 15}
		if w.DueOn != expected {
			t.Errorf("got %v, want %v", w.DueOn, expected)
		}
	})

	t.Run("unmarshal null", func(t *testing.T) {
		var w wrapper
		err := json.Unmarshal([]byte(`{"due_on":null}`), &w)
		if err != nil {
			t.Fatal(err)
		}
		if !w.DueOn.IsZero() {
			t.Errorf("expected zero date, got %v", w.DueOn)
		}
	})

	t.Run("unmarshal empty string", func(t *testing.T) {
		var w wrapper
		err := json.Unmarshal([]byte(`{"due_on":""}`), &w)
		if err != nil {
			t.Fatal(err)
		}
		if !w.DueOn.IsZero() {
			t.Errorf("expected zero date, got %v", w.DueOn)
		}
	})
}

func TestDateWeekday(t *testing.T) {
	// 2024-01-15 is a Monday
	d := Date{2024, time.January, 15}
	if d.Weekday() != time.Monday {
		t.Errorf("expected Monday, got %v", d.Weekday())
	}
}

func TestDateOf(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	tm := time.Date(2024, time.January, 15, 23, 59, 59, 0, loc)
	d := DateOf(tm)
	expected := Date{2024, time.January, 15}
	if d != expected {
		t.Errorf("DateOf = %v, want %v", d, expected)
	}
}

func TestDateIn(t *testing.T) {
	d := Date{2024, time.January, 15}
	loc, _ := time.LoadLocation("America/New_York")
	tm := d.In(loc)

	if tm.Year() != 2024 || tm.Month() != time.January || tm.Day() != 15 {
		t.Errorf("wrong date: %v", tm)
	}
	if tm.Hour() != 0 || tm.Minute() != 0 || tm.Second() != 0 {
		t.Errorf("expected midnight, got %v", tm)
	}
	if tm.Location() != loc {
		t.Errorf("wrong location: %v", tm.Location())
	}
}
