// Package types provides shared types used across the Basecamp SDK.
package types

import (
	"fmt"
	"strings"
	"time"
)

// FlexibleTime is a time.Time that can unmarshal from RFC3339, RFC3339Nano,
// or date-only ("2006-01-02") strings. Date-only values are treated as midnight
// UTC. This supports API responses where all-day schedule entries return dates
// without times.
//
// It remembers which form it parsed and marshals that form back: a bare date
// re-emits as a bare date, a timestamp as a timestamp. BC3 re-parses a
// timestamp in the account's own time zone, so an all-day bound rendered as
// "2026-08-04T00:00:00Z" lands on the previous day anywhere west of UTC — a
// read-modify-write that widened the date on input and narrowed it on output
// moved the entry (#633).
type FlexibleTime struct {
	time.Time
	dateOnly bool
}

// DateOnly reports whether the value was parsed from a bare date, in which
// case it marshals back as one.
func (ft FlexibleTime) DateOnly() bool {
	return ft.dateOnly
}

// UnmarshalJSON implements json.Unmarshaler for FlexibleTime.
func (ft *FlexibleTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	ft.dateOnly = false
	if s == "null" || s == "" {
		ft.Time = time.Time{}
		return nil
	}

	// Try RFC3339 first (most common)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		ft.Time = t
		return nil
	}

	// Try RFC3339Nano for fractional seconds (e.g., "2022-11-01T10:00:00.000Z")
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		ft.Time = t
		return nil
	}

	// Try date-only → midnight UTC
	if t, err := time.Parse("2006-01-02", s); err == nil {
		ft.Time = t
		ft.dateOnly = true
		return nil
	}

	return fmt.Errorf("cannot parse %q as RFC3339, RFC3339Nano, or date-only", s)
}

// MarshalJSON implements json.Marshaler for FlexibleTime.
// Zero times marshal as null; a value parsed from a bare date marshals as that
// date; any other non-zero time uses time.Time's JSON encoding.
func (ft FlexibleTime) MarshalJSON() ([]byte, error) {
	if ft.IsZero() {
		return []byte("null"), nil
	}
	if ft.dateOnly {
		return []byte(`"` + ft.Format("2006-01-02") + `"`), nil
	}
	return ft.Time.MarshalJSON()
}
