package main

import (
	"fmt"
	"time"
)

// checkDelayGaps validates one `delayBetweenRequests` assertion against the
// recorded request times, returning "" when it holds and a failure message
// otherwise.
//
// Gap i is the interval between request i and request i+1, so N requests
// yield N-1 gaps. The contract in conformance/schema.json:
//
//   - A NAMED index selects exactly that gap. It is bounds-checked
//     unconditionally: a gap the run never produced is a failure, not a
//     silent pass. The whole point of a timing pin is to catch a dropped
//     backoff, and a dropped backoff is precisely what removes the gap.
//   - An OMITTED index requires the minimum on EVERY gap. Zero gaps means
//     nothing was measured, so an omitted-index assertion on a single-request
//     run fails too — the same reasoning, one step further: if every retry
//     were dropped, an "every gap" rule with no gaps left would otherwise
//     wave the run through.
//   - Negative indexes are rejected rather than wrapping to the end the way
//     headerPresent and friends do. There is no sensible "last gap" when the
//     point of naming one is to pin a specific backoff.
//
// The bounds test compares against the last valid gap rather than adding one
// to the index: `index+1 >= len(requestTimes)` overflows for math.MaxInt and
// wraps negative, sailing through the guard into an out-of-range read.
func checkDelayGaps(requestTimes []time.Time, minDelay time.Duration, index *int) string {
	gaps := len(requestTimes) - 1

	if index != nil {
		gap := *index
		if gap < 0 {
			return fmt.Sprintf("delayBetweenRequests gap index must be non-negative, got %d", gap)
		}
		if gap >= gaps {
			return fmt.Sprintf("Expected a delay at gap %d, but only %d request(s) were made", gap, len(requestTimes))
		}
		if delay := requestTimes[gap+1].Sub(requestTimes[gap]); delay < minDelay {
			return fmt.Sprintf("Expected delay >= %v at gap %d, got %v", minDelay, gap, delay)
		}
		return ""
	}

	if gaps < 1 {
		return fmt.Sprintf("Expected a delay between requests, but only %d request(s) were made", len(requestTimes))
	}
	for i := 0; i < gaps; i++ {
		if delay := requestTimes[i+1].Sub(requestTimes[i]); delay < minDelay {
			return fmt.Sprintf("Expected delay >= %v at gap %d, got %v", minDelay, i, delay)
		}
	}
	return ""
}
