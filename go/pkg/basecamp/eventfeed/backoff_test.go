package eventfeed

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// fixedRand returns a rand source that always yields v.
func fixedRand(v float64) func() float64 {
	return func() float64 { return v }
}

func TestBackoffEnvelope_DoublesFromBaseToCap(t *testing.T) {
	cases := []struct {
		index int
		want  time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 32 * time.Second},
		{7, 60 * time.Second}, // 2^6 = 64s crosses the cap
		{8, 60 * time.Second},
	}
	for _, tc := range cases {
		if got := backoffEnvelope(tc.index); got != tc.want {
			t.Errorf("backoffEnvelope(%d) = %v, want %v", tc.index, got, tc.want)
		}
	}
}

func TestBackoffEnvelope_SaturatesBeforeExponentiating(t *testing.T) {
	// SPEC §7's overflow rule: the index is compared against the cap-crossing
	// exponent BEFORE 2^(index−1) is evaluated. Evaluating the power first
	// overflows fixed-width integers after a long genuine outage (~64
	// consecutive failures) — these indices must all saturate at the cap, not
	// wrap into a tight loop.
	for _, index := range []int{64, 65, 100, 1 << 30} {
		if got := backoffEnvelope(index); got != backoffCap {
			t.Errorf("backoffEnvelope(%d) = %v, want %v", index, got, backoffCap)
		}
	}
}

func TestBackoffEnvelope_ClampsNonPositiveIndex(t *testing.T) {
	for _, index := range []int{0, -1, -100} {
		if got := backoffEnvelope(index); got != backoffBase {
			t.Errorf("backoffEnvelope(%d) = %v, want %v", index, got, backoffBase)
		}
	}
}

func TestFullJitterDelay_DrawsWithinEnvelope(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, index := range []int{1, 3, 7, 20} {
		envelope := backoffEnvelope(index)
		for i := 0; i < 1000; i++ {
			d := fullJitterDelay(index, rng.Float64)
			if d < 0 || d >= envelope {
				t.Fatalf("fullJitterDelay(%d) = %v, want in [0, %v)", index, d, envelope)
			}
		}
	}
}

func TestFullJitterDelay_SpansTheFullRange(t *testing.T) {
	if got := fullJitterDelay(3, fixedRand(0)); got != 0 {
		t.Errorf("fullJitterDelay(3) with r=0 = %v, want 0", got)
	}
	// r = 0.5 draws exactly half the 4s envelope.
	if got := fullJitterDelay(3, fixedRand(0.5)); got != 2*time.Second {
		t.Errorf("fullJitterDelay(3) with r=0.5 = %v, want 2s", got)
	}
}

func TestReconnectDelay_RetryAfterFloorsTheDraw(t *testing.T) {
	// Envelope for n=3 is 4s; r=0.75 draws 3s.
	if got := reconnectDelay(0, 3, fixedRand(0.75)); got != 3*time.Second {
		t.Errorf("reconnectDelay(no Retry-After) = %v, want 3s", got)
	}
	// A Retry-After below the draw does not shrink it.
	if got := reconnectDelay(time.Second, 3, fixedRand(0.75)); got != 3*time.Second {
		t.Errorf("reconnectDelay(1s floor under 3s draw) = %v, want 3s", got)
	}
	// A Retry-After above the draw floors it.
	if got := reconnectDelay(3500*time.Millisecond, 3, fixedRand(0.75)); got != 3500*time.Millisecond {
		t.Errorf("reconnectDelay(3.5s floor over 3s draw) = %v, want 3.5s", got)
	}
}

func TestReconnectDelay_RetryAfterWinsOutrightOverTheCap(t *testing.T) {
	// Server-directed waits are exempt from local caps (SPEC §7): a
	// Retry-After beyond the 60s cap is honored, not clamped.
	if got := reconnectDelay(5*time.Minute, 20, fixedRand(0.99)); got != 5*time.Minute {
		t.Errorf("reconnectDelay(5m Retry-After) = %v, want 5m", got)
	}
}

func TestPollRetryDelay_RetryAfterIsExactAndCapExempt(t *testing.T) {
	// A server-directed Retry-After is waited exactly — even beyond the cap.
	if got := pollRetryDelay(5*time.Minute, 1, fixedRand(0.5)); got != 5*time.Minute {
		t.Errorf("pollRetryDelay(5m Retry-After) = %v, want 5m", got)
	}
	// It is exact, not a floor: a Retry-After below the would-be draw wins.
	if got := pollRetryDelay(time.Second, 7, fixedRand(0.99)); got != time.Second {
		t.Errorf("pollRetryDelay(1s Retry-After) = %v, want exactly 1s", got)
	}
}

func TestPollRetryDelay_FullJitterOverTheFailureIndex(t *testing.T) {
	// Without Retry-After the wait is a full-jitter draw over k.
	if got := pollRetryDelay(0, 2, fixedRand(0.5)); got != time.Second {
		t.Errorf("pollRetryDelay(k=2, r=0.5) = %v, want 1s", got)
	}
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 1000; i++ {
		d := pollRetryDelay(0, 9, rng.Float64)
		if d < 0 || d >= backoffCap {
			t.Fatalf("pollRetryDelay(k=9) = %v, want in [0, %v)", d, backoffCap)
		}
	}
}

func TestRepairJitter_StaysWithinTwentyPercent(t *testing.T) {
	lo, hi := 48*time.Second, 72*time.Second // 60s ± 20%
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 1000; i++ {
		d := repairJitter(DefaultRepairInterval, rng.Float64)
		if d < lo || d >= hi {
			t.Fatalf("repairJitter(60s) = %v, want in [%v, %v)", d, lo, hi)
		}
	}
	if got := repairJitter(DefaultRepairInterval, fixedRand(0)); got != lo {
		t.Errorf("repairJitter(60s) with r=0 = %v, want %v", got, lo)
	}
	if got := repairJitter(DefaultRepairInterval, fixedRand(0.5)); got != DefaultRepairInterval {
		t.Errorf("repairJitter(60s) with r=0.5 = %v, want %v", got, DefaultRepairInterval)
	}
}

// TestRepairJitterSaturates is §23's "saturate before exponentiating" rule at
// the other end of the range. `math.MaxInt64` is the conventional Go spelling
// of "effectively never", and it is the one interval whose upward jitter draw
// cannot be represented: the product overflows `time.Duration`, and an
// out-of-range float-to-integer conversion is implementation-defined — on the
// common targets it lands negative, so the timer fires immediately and the
// connector repair-polls as fast as the walk completes. The failure is the
// exact inverse of what was asked for, which is why it is clamped rather than
// documented as a caller error.
//
// ARCHITECTURE-DEPENDENT, and worth knowing before concluding it is vacuous:
// out-of-range float-to-integer conversion is implementation-defined, and the
// two targets differ. On arm64 the hardware saturates, so this test passes
// against UN-FIXED code and proves nothing there. On amd64 it yields the
// indefinite integer, and the un-fixed function returns
// -2562047h47m16.854775808s. The red proof for this fix was therefore taken on
// x86_64, not on the author's arm64 machine — do not "verify" it locally on
// Apple silicon and conclude the clamp is unnecessary.
func TestRepairJitterSaturates(t *testing.T) {
	for _, draw := range []float64{0.5, 0.75, 0.9, 0.999999} {
		got := repairJitter(time.Duration(math.MaxInt64), fixedRand(draw))
		if got <= 0 {
			t.Fatalf("repairJitter(MaxInt64, draw=%v) = %s, want a positive saturated duration — a negative cadence fires at once", draw, got)
		}
		if got != time.Duration(math.MaxInt64) {
			t.Errorf("repairJitter(MaxInt64, draw=%v) = %s, want %s", draw, got, time.Duration(math.MaxInt64))
		}
	}
	// The downward half of the same interval is representable and must NOT be
	// clamped — saturating everything would silently disable the jitter.
	if got := repairJitter(time.Duration(math.MaxInt64), fixedRand(0)); got >= time.Duration(math.MaxInt64) {
		t.Errorf("repairJitter(MaxInt64, draw=0) = %s, want the -20%% draw, not the ceiling", got)
	}
	// The ordinary interval is untouched by the clamp.
	const day = 24 * time.Hour
	if got := repairJitter(day, fixedRand(1)); got <= day || got > time.Duration(1.2*float64(day)) {
		t.Errorf("repairJitter(24h, draw=1) = %s, want just under +20%%", got)
	}
}

func TestPollRetryDelay_UnusableValuesDrawTheJitterCurve(t *testing.T) {
	// §6's parsing algorithm never yields zero ("zero is read as 'no usable
	// value'"), so a conformant throttled always carries a positive value —
	// and an unusable zero or negative, whatever kind a nonconforming
	// adapter claims, draws local jitter rather than arming a zero-delay
	// tight loop.
	if got := pollRetryDelay(0, 2, fixedRand(0.5)); got != time.Second {
		t.Errorf("pollRetryDelay(0, k=2, r=0.5) = %v, want the 1s jitter draw", got)
	}
	if got := pollRetryDelay(-time.Second, 2, fixedRand(0.5)); got != time.Second {
		t.Errorf("pollRetryDelay(-1s, k=2, r=0.5) = %v, want the 1s jitter draw", got)
	}
}

// A positive interval must stay positive after jitter: at 1ns, any downward
// draw lands the float product in (0,1) and the Duration conversion floors
// it to zero — a timer that fires immediately, every cycle, tight-looping
// poll walks against a caller who asked for a positive interval.
func TestRepairJitter_KeepsPositiveIntervalsPositive(t *testing.T) {
	for _, draw := range []float64{0, 0.1, 0.25} {
		if got := repairJitter(time.Nanosecond, fixedRand(draw)); got < time.Nanosecond {
			t.Errorf("repairJitter(1ns, draw %v) = %v, want at least 1ns", draw, got)
		}
	}
}
