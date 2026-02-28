package basecamp

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_AllowSuccess(t *testing.T) {
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         5,
	})

	for i := range 5 {
		if !rl.Allow() {
			t.Fatalf("Allow() failed on request %d, expected 5 burst tokens", i+1)
		}
	}
}

func TestRateLimiter_AllowExhaustion(t *testing.T) {
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         2,
	})

	rl.Allow()
	rl.Allow()

	if rl.Allow() {
		t.Error("Allow() should return false after burst exhaustion")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         2,
		Now:               func() time.Time { return now },
	})

	// Exhaust burst
	rl.Allow()
	rl.Allow()
	if rl.Allow() {
		t.Fatal("should be exhausted")
	}

	// Advance 200ms = 2 tokens at 10/s
	now = now.Add(200 * time.Millisecond)

	if !rl.Allow() {
		t.Error("expected Allow to succeed after refill")
	}
}

func TestRateLimiter_Wait_ContextCancellation(t *testing.T) {
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
	})
	rl.Allow() // exhaust

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(ctx)
	if err != context.Canceled {
		t.Errorf("Wait err = %v, want context.Canceled", err)
	}
}

func TestRateLimiter_Reserve_Immediate(t *testing.T) {
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         5,
	})

	d := rl.Reserve()
	if d != 0 {
		t.Errorf("Reserve() = %v, want 0 (immediate)", d)
	}
}

func TestRateLimiter_Reserve_Delayed(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         1,
		Now:               func() time.Time { return now },
	})

	rl.Allow() // exhaust burst

	d := rl.Reserve()
	if d <= 0 {
		t.Errorf("Reserve() = %v, want positive delay", d)
	}
	if d > time.Second {
		t.Errorf("Reserve() = %v, expected <= 1s", d)
	}
}

func TestRateLimiter_Reserve_TooFarInFuture(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 0.5, // 1 token per 2 seconds
		BurstSize:         1,
		Now:               func() time.Time { return now },
	})

	rl.Allow() // exhaust

	d := rl.Reserve()
	if d >= 0 {
		t.Errorf("Reserve() = %v, want negative (too far in future)", d)
	}
}

func TestRateLimiter_SetRetryAfterDuration(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         10,
		RespectRetryAfter: true,
		Now:               func() time.Time { return now },
	})

	rl.SetRetryAfterDuration(5 * time.Second)

	if rl.Allow() {
		t.Error("Allow should return false during Retry-After")
	}

	remaining := rl.RetryAfterRemaining()
	if remaining <= 0 || remaining > 5*time.Second {
		t.Errorf("RetryAfterRemaining = %v, want ~5s", remaining)
	}

	// Advance past retry-after
	now = now.Add(6 * time.Second)

	if !rl.Allow() {
		t.Error("Allow should succeed after Retry-After expires")
	}
}

func TestRateLimiter_SetRetryAfter_Disabled(t *testing.T) {
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         10,
		RespectRetryAfter: false,
	})

	rl.SetRetryAfterDuration(5 * time.Second)

	if !rl.Allow() {
		t.Error("Allow should ignore Retry-After when RespectRetryAfter=false")
	}
}

func TestRateLimiter_Reserve_DuringRetryAfter(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         10,
		RespectRetryAfter: true,
		Now:               func() time.Time { return now },
	})

	rl.SetRetryAfterDuration(5 * time.Second)

	d := rl.Reserve()
	if d >= 0 {
		t.Errorf("Reserve during Retry-After = %v, want negative", d)
	}
}

func TestRateLimiter_Tokens(t *testing.T) {
	rl := newRateLimiter(&RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         5,
	})

	tokens := rl.Tokens()
	if tokens != 5 {
		t.Errorf("initial Tokens = %v, want 5", tokens)
	}

	rl.Allow()
	tokens = rl.Tokens()
	if tokens < 3.9 || tokens > 4.1 {
		t.Errorf("Tokens after 1 Allow = %v, want ~4", tokens)
	}
}

func TestRateLimiter_DefaultConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	if cfg.RequestsPerSecond != 50 {
		t.Errorf("RequestsPerSecond = %v, want 50", cfg.RequestsPerSecond)
	}
	if cfg.BurstSize != 10 {
		t.Errorf("BurstSize = %d, want 10", cfg.BurstSize)
	}
	if !cfg.RespectRetryAfter {
		t.Error("RespectRetryAfter should default to true")
	}
}
