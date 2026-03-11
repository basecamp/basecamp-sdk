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
type FlexibleTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler for FlexibleTime.
func (ft *FlexibleTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
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
		return nil
	}

	return fmt.Errorf("cannot parse %q as RFC3339, RFC3339Nano, or date-only", s)
}

// MarshalJSON implements json.Marshaler for FlexibleTime.
// Zero times marshal as null; non-zero times use time.Time's JSON encoding.
func (ft FlexibleTime) MarshalJSON() ([]byte, error) {
	if ft.IsZero() {
		return []byte("null"), nil
	}
	return ft.Time.MarshalJSON()
}
