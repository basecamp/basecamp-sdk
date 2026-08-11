package eventfeed

import (
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
