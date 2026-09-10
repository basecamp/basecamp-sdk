package generated

import (
	"net/http"
	"testing"
	"time"
)

// The generated client's own copy of SPEC §6's Retry-After Parsing Algorithm.
// Before it existed the retry loop did a bare strconv.Atoi (#798): no HTTP-date
// form, an unchecked range error, and a seconds×time.Second product that
// wrapped negative for the largest int64 — an already-expired timer.
func TestParseRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		want   int
	}{
		{"absent", "", 0},
		{"seconds", "120", 120},
		{"leading zeros", "0120", 120},
		{"zero", "0", 0},
		{"negative", "-5", 0},
		{"signed", "+5", 0},
		{"fractional", "1.5", 0},
		{"partly numeric", "120junk", 0},
		{"unparseable", "sometime next week", 0},
		{"http-date in the past", "Wed, 09 Jun 2021 10:18:14 GMT", 0},
		{"ceiling", "2147483647", maxRetryAfterSeconds},
		{"one past the ceiling", "2147483648", maxRetryAfterSeconds},
		{"largest int64", "9223372036854775807", maxRetryAfterSeconds},
		{"one past the largest int64", "9223372036854775808", maxRetryAfterSeconds},
		{"digits beyond int64 range", "99999999999999999999", maxRetryAfterSeconds},
		{"far-future http-date", "Fri, 31 Dec 9999 23:59:59 GMT", maxRetryAfterSeconds},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRetryAfter(tc.header); got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %d, want %d", tc.header, got, tc.want)
			}
		})
	}
}

// One-sided, per SPEC §6 "Rounding": the parsed delay is never shorter than the
// time actually remaining, re-measured after the parse, which scheduling delay
// can only weaken toward vacuity and never turn red.
func TestParseRetryAfter_HTTPDateRoundsUp(t *testing.T) {
	target := time.Now().Add(2500 * time.Millisecond)
	got := parseRetryAfter(target.UTC().Format(http.TimeFormat))
	remaining := time.Until(target)
	if got <= 0 {
		t.Fatalf("parseRetryAfter(future date) = %d, want a positive delay", got)
	}
	if float64(got) < remaining.Seconds() {
		t.Errorf("parseRetryAfter(future date) = %ds, shorter than the %v actually remaining — the remainder must round UP", got, remaining)
	}
}
