package basecamp

import (
	"testing"
	"time"
)

// TestBackoffDelaySaturates pins SPEC §7's backoff ceiling (#577).
//
// Before the fix, backoffDelay computed base * time.Duration(1<<(attempt-1)).
// The shift is evaluated in int: at attempt 64 it is 1<<63, which is negative,
// and past 64 Go defines an over-wide shift as 0. Multiplying a 1s base by
// either produces a delay that is negative or zero, and time.After / time.Sleep
// treat a non-positive duration as "no wait" — so a long failure streak stopped
// backing off entirely and hammered a server already answering 429/503.
//
// WithMaxRetries only rejects n < 0, so a caller can set the attempt count that
// reaches this. The bound is two-sided on purpose: a one-sided "never too long"
// check passes on exactly the attempts that tight-loop.
func TestBackoffDelaySaturates(t *testing.T) {
	client := &Client{httpOpts: HTTPOptions{
		BaseDelay: DefaultBaseDelay,
		MaxJitter: DefaultMaxJitter,
	}}

	// With a 1s base, attempt 6 is the first whose unclamped term (32s) exceeds
	// the 30s ceiling, so every attempt from there on must sit at the ceiling.
	for _, attempt := range []int{6, 10, 32, 33, 62, 63, 64, 65, 128, 1 << 30} {
		delay := client.backoffDelay(attempt)
		if delay < MaxBackoffDelay || delay > MaxBackoffDelay+DefaultMaxJitter {
			t.Errorf("backoffDelay(%d) = %v, want within [%v, %v]",
				attempt, delay, MaxBackoffDelay, MaxBackoffDelay+DefaultMaxJitter)
		}
	}

	// The unsaturated attempts keep their exact exponential values, so the
	// ceiling changes nothing on any path a shipped configuration reaches.
	for attempt, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second} {
		delay := client.backoffDelay(attempt)
		if delay < want || delay > want+DefaultMaxJitter {
			t.Errorf("backoffDelay(%d) = %v, want within [%v, %v]", attempt, delay, want, want+DefaultMaxJitter)
		}
	}
}

// TestBackoffDelayEdgeConfigurations covers the two configurations that make
// the clamp arithmetic itself interesting: a base delay above the ceiling
// (SPEC §7 requirement 3 — it is clamped, with no carve-out for the first
// sleep) and a zero base delay (must stay at zero rather than saturating,
// since MaxBackoffDelay/0 would otherwise divide by zero).
func TestBackoffDelayEdgeConfigurations(t *testing.T) {
	above := &Client{httpOpts: HTTPOptions{BaseDelay: 10 * time.Minute, MaxJitter: DefaultMaxJitter}}
	if delay := above.backoffDelay(1); delay < MaxBackoffDelay || delay > MaxBackoffDelay+DefaultMaxJitter {
		t.Errorf("base delay above the ceiling: backoffDelay(1) = %v, want the ceiling %v", delay, MaxBackoffDelay)
	}

	zero := &Client{httpOpts: HTTPOptions{BaseDelay: 0, MaxJitter: DefaultMaxJitter}}
	if delay := zero.backoffDelay(90); delay > DefaultMaxJitter {
		t.Errorf("zero base delay: backoffDelay(90) = %v, want at most the jitter %v", delay, DefaultMaxJitter)
	}
}
