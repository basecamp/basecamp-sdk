package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

// times builds request timestamps from successive gaps in milliseconds, so a
// case reads as the gaps it is about rather than as absolute instants.
func times(gapsMs ...int) []time.Time {
	base := time.Unix(0, 0)
	out := []time.Time{base}
	for _, ms := range gapsMs {
		base = base.Add(time.Duration(ms) * time.Millisecond)
		out = append(out, base)
	}
	return out
}

func ptr(i int) *int { return &i }

// The four bounds branches every runner must share, plus the every-gap rule.
// Each case names the behavior that regressed on #563 and #568.
func TestCheckDelayGaps(t *testing.T) {
	cases := []struct {
		name        string
		requestTime []time.Time
		min         time.Duration
		index       *int
		wantFail    bool
		wantMsg     string
	}{
		{
			// The #568 defect: three runners measured gap 0 and stopped, so a
			// second backoff that never happened passed unnoticed.
			name:        "omitted index catches a later failing gap",
			requestTime: times(1000, 5),
			min:         500 * time.Millisecond,
			index:       nil,
			wantFail:    true,
			wantMsg:     "at gap 1",
		},
		{
			name:        "omitted index passes when every gap clears the minimum",
			requestTime: times(1000, 2000, 800),
			min:         500 * time.Millisecond,
			index:       nil,
			wantFail:    false,
		},
		{
			// The residual false-green: an "every gap" rule with no gaps must
			// not wave the run through. A fully dropped retry lands here.
			name:        "omitted index fails when there are no gaps at all",
			requestTime: times(),
			min:         500 * time.Millisecond,
			index:       nil,
			wantFail:    true,
			wantMsg:     "only 1 request(s) were made",
		},
		{
			name:        "named gap fails when the run never produced it",
			requestTime: times(1000),
			min:         500 * time.Millisecond,
			index:       ptr(1),
			wantFail:    true,
			wantMsg:     "Expected a delay at gap 1, but only 2 request(s) were made",
		},
		{
			name:        "named gap fails on a single-request run",
			requestTime: times(),
			min:         500 * time.Millisecond,
			index:       ptr(0),
			wantFail:    true,
			wantMsg:     "Expected a delay at gap 0, but only 1 request(s) were made",
		},
		{
			// Negative must be rejected categorically, not wrapped to the end
			// the way headerPresent's index does.
			name:        "negative gap index is rejected",
			requestTime: times(1000, 2000),
			min:         500 * time.Millisecond,
			index:       ptr(-1),
			wantFail:    true,
			wantMsg:     "must be non-negative",
		},
		{
			// gap+1 overflows to negative for MaxInt, sailing past a
			// `gap+1 >= len` guard into an out-of-range read.
			name:        "MaxInt gap index fails without overflowing",
			requestTime: times(1000, 2000),
			min:         500 * time.Millisecond,
			index:       ptr(math.MaxInt),
			wantFail:    true,
			wantMsg:     "Expected a delay at gap",
		},
		{
			// A zero minimum still requires the gap to EXIST. Runners that
			// gate the check on a truthy min skip this case entirely.
			name:        "zero minimum still asserts the gap exists",
			requestTime: times(),
			min:         0,
			index:       nil,
			wantFail:    true,
			wantMsg:     "only 1 request(s) were made",
		},
		{
			name:        "zero minimum passes once a gap exists",
			requestTime: times(5),
			min:         0,
			index:       nil,
			wantFail:    false,
		},
		{
			name:        "named gap passes when it clears the minimum",
			requestTime: times(5, 2000),
			min:         500 * time.Millisecond,
			index:       ptr(1),
			wantFail:    false,
		},
		{
			name:        "named gap fails when it is below the minimum",
			requestTime: times(2000, 5),
			min:         500 * time.Millisecond,
			index:       ptr(1),
			wantFail:    true,
			wantMsg:     "at gap 1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkDelayGaps(tc.requestTime, tc.min, tc.index)
			if tc.wantFail {
				if got == "" {
					t.Fatalf("expected a failure message, got a pass")
				}
				if !strings.Contains(got, tc.wantMsg) {
					t.Fatalf("expected message containing %q, got %q", tc.wantMsg, got)
				}
			} else if got != "" {
				t.Fatalf("expected a pass, got failure %q", got)
			}
		})
	}
}
