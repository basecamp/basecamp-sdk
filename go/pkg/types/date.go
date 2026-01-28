package types

import (
	"fmt"
	"strings"
	"time"
)

// Date represents a calendar date (year, month, day) without time or timezone.
// Use this for date-only fields like due_on and starts_on.
type Date struct {
	Year  int        // Year (e.g., 2024)
	Month time.Month // Month of the year (January = 1, ...)
	Day   int        // Day of the month, starting at 1
}

// ParseDate parses a string in YYYY-MM-DD format.
func ParseDate(s string) (Date, error) {
	if s == "" {
		return Date{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return Date{t.Year(), t.Month(), t.Day()}, nil
}

// DateOf returns the Date portion of a time.Time in that time's location.
func DateOf(t time.Time) Date {
	year, month, day := t.Date()
	return Date{year, month, day}
}

// Today returns today's date in the local timezone.
func Today() Date {
	return DateOf(time.Now())
}

// String returns the date in YYYY-MM-DD format.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// GoString returns a Go-syntax representation for debugging.
func (d Date) GoString() string {
	return fmt.Sprintf("basecamp.Date{Year: %d, Month: %d, Day: %d}", d.Year, d.Month, d.Day)
}

// IsZero reports whether the date is the zero value.
func (d Date) IsZero() bool {
	return d.Year == 0 && d.Month == 0 && d.Day == 0
}

// IsValid reports whether the date represents a valid calendar date.
func (d Date) IsValid() bool {
	if d.IsZero() {
		return true // Zero is valid (means unset)
	}
	return DateOf(d.In(time.UTC)) == d
}

// In returns the time.Time corresponding to midnight of the date in the given location.
func (d Date) In(loc *time.Location) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// UTC returns the time.Time corresponding to midnight UTC of the date.
func (d Date) UTC() time.Time {
	return d.In(time.UTC)
}

// Before reports whether d is before other.
func (d Date) Before(other Date) bool {
	if d.Year != other.Year {
		return d.Year < other.Year
	}
	if d.Month != other.Month {
		return d.Month < other.Month
	}
	return d.Day < other.Day
}

// After reports whether d is after other.
func (d Date) After(other Date) bool {
	return other.Before(d)
}

// Equal reports whether d and other represent the same date.
func (d Date) Equal(other Date) bool {
	return d.Year == other.Year && d.Month == other.Month && d.Day == other.Day
}

// Compare compares d and other. Returns -1 if d < other, 0 if equal, +1 if d > other.
func (d Date) Compare(other Date) int {
	if d.Before(other) {
		return -1
	}
	if d.After(other) {
		return 1
	}
	return 0
}

// AddDays returns the date n days from d.
func (d Date) AddDays(n int) Date {
	return DateOf(d.In(time.UTC).AddDate(0, 0, n))
}

// AddMonths returns the date n months from d.
func (d Date) AddMonths(n int) Date {
	return DateOf(d.In(time.UTC).AddDate(0, n, 0))
}

// AddYears returns the date n years from d.
func (d Date) AddYears(n int) Date {
	return DateOf(d.In(time.UTC).AddDate(n, 0, 0))
}

// DaysSince returns the number of days from s to d (d - s).
func (d Date) DaysSince(s Date) int {
	return int(d.In(time.UTC).Sub(s.In(time.UTC)).Hours() / 24)
}

// Weekday returns the day of the week.
func (d Date) Weekday() time.Weekday {
	return d.In(time.UTC).Weekday()
}

// MarshalJSON implements json.Marshaler.
// Zero dates marshal as null, valid dates as "YYYY-MM-DD".
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
// Accepts "YYYY-MM-DD" strings and null.
func (d *Date) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "null" || s == "" {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (d Date) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Date) UnmarshalText(data []byte) error {
	parsed, err := ParseDate(string(data))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
