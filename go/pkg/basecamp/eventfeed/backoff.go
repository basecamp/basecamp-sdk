package eventfeed

import (
	"math"
	"time"
)

// Two-lane retry timing (SPEC.md §23 "Clock, Timers, and Virtual Time").
// The reconnect-cycle `backoff` timer and the in-walk `poll-retry` timer
// share one full-jitter formula — uniform(0, min(60s, 1s × 2^(index−1))) —
// over two SEPARATE indices: the failed-cycle count n and the
// consecutive-poll-failure index k. This is deliberately not SPEC §7's
// per-request formula: §7 governs attempts inside a seam call; this governs
// cycles between them.

const (
	// backoffBase is the full-jitter base (EVENT_FEED_BACKOFF_BASE, 1s).
	backoffBase = time.Second
	// backoffCap caps the locally computed jitter range
	// (EVENT_FEED_BACKOFF_CAP, 60s). Server-directed Retry-After is exempt.
	backoffCap = 60 * time.Second
	// backoffSaturationExponent is the cap-crossing exponent: the smallest e
	// with backoffBase × 2^e ≥ backoffCap (1s × 2^6 = 64s ≥ 60s). Any index
	// whose exponent reaches it already saturates the range at the cap.
	backoffSaturationExponent = 6
	// repairJitterFraction is the ±20% jitter applied to each repair-poll
	// cycle.
	repairJitterFraction = 0.20
)

// DefaultRepairInterval is the default repair-poll cadence
// (EVENT_FEED_REPAIR_INTERVAL, 60s; ±20% jitter applied per cycle).
const DefaultRepairInterval = 60 * time.Second

// fullJitterDelay draws a full-jitter delay uniform(0, min(cap, base ×
// 2^(index−1))) for the 1-based failure index. r is a uniform [0, 1) source.
func fullJitterDelay(index int, r func() float64) time.Duration {
	return time.Duration(r() * float64(backoffEnvelope(index)))
}

// backoffEnvelope is the jitter range's exclusive upper bound for the 1-based
// failure index: min(cap, base × 2^(index−1)), saturating BEFORE
// exponentiating (SPEC §7's overflow rule): the index is compared against the
// cap-crossing exponent before 2^(index−1) is evaluated, so a long genuine
// outage (~64 consecutive failures) cannot overflow into a tight loop.
func backoffEnvelope(index int) time.Duration {
	if index < 1 {
		index = 1
	}
	if index-1 >= backoffSaturationExponent {
		return backoffCap
	}
	d := backoffBase << (index - 1)
	if d > backoffCap {
		return backoffCap
	}
	return d
}

// reconnectDelay selects the `backoff` timer's wait for the 1-based
// failed-cycle count n: a full-jitter draw, floored by a server-directed
// Retry-After — which wins outright when it exceeds the cap (server-directed
// waits are exempt from local caps, per SPEC §7).
func reconnectDelay(retryAfter time.Duration, n int, r func() float64) time.Duration {
	d := fullJitterDelay(n, r)
	if retryAfter > d {
		return retryAfter
	}
	return d
}

// pollRetryDelay selects the `poll-retry` timer's wait for the 1-based
// consecutive-poll-failure index k — a counter separate from the
// reconnect-cycle count, reset by any successful poll page and by socket
// teardown. A server-directed Retry-After is waited exactly and is exempt
// from local caps; otherwise the wait is a full-jitter draw over k.
func pollRetryDelay(retryAfter time.Duration, k int, r func() float64) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	return fullJitterDelay(k, r)
}

// repairJitter draws a repair-poll cadence uniform in
// [interval × (1 − 0.20), interval × (1 + 0.20)). r is a uniform [0, 1)
// source.
// It SATURATES before converting back, which is §23's "saturate before
// exponentiating" rule applied at the other end of the range. A caller that
// spells "effectively never repair" the conventional Go way — a repair
// interval at or near `math.MaxInt64` — makes the upward jitter draw overflow
// `time.Duration`, and an out-of-range float-to-integer conversion is
// implementation-defined: on the common targets it lands negative, so the
// timer fires at once and the connector repair-polls as fast as the walk
// completes. The failure is therefore the exact inverse of the request, which
// is what makes the clamp worth having rather than a caller error to
// document. Only the upper end can overflow: the interval is validated
// positive, so the downward draw is bounded by the interval itself.
func repairJitter(interval time.Duration, r func() float64) time.Duration {
	span := 2*r() - 1 // uniform [-1, 1)
	jittered := float64(interval) * (1 + repairJitterFraction*span)
	if jittered >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(jittered)
}
