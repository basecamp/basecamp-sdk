package basecamp

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedAllowsRequests(t *testing.T) {
	cb := newCircuitBreaker(nil)
	if !cb.Allow() {
		t.Error("closed breaker should allow requests")
	}
	if cb.State() != "closed" {
		t.Errorf("State = %q, want %q", cb.State(), "closed")
	}
}

func TestCircuitBreaker_ConsecutiveFailureTrips(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		FailureThreshold:     3,
		SuccessThreshold:     1,
		OpenTimeout:          time.Hour,
		FailureRateThreshold: 100, // disable rate-based
		SlidingWindowSize:    100,
	}
	cb := newCircuitBreaker(cfg)

	for range 3 {
		cb.RecordFailure()
	}

	if cb.State() != "open" {
		t.Errorf("after %d failures, State = %q, want %q", 3, cb.State(), "open")
	}
	if cb.Allow() {
		t.Error("open breaker should reject requests")
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		FailureThreshold:     3,
		SuccessThreshold:     1,
		OpenTimeout:          time.Hour,
		FailureRateThreshold: 100,
		SlidingWindowSize:    100,
	}
	cb := newCircuitBreaker(cfg)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets consecutive count
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != "closed" {
		t.Errorf("State = %q, want %q (success should reset count)", cb.State(), "closed")
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	now := time.Now()
	cfg := &CircuitBreakerConfig{
		FailureThreshold:     2,
		SuccessThreshold:     1,
		OpenTimeout:          100 * time.Millisecond,
		FailureRateThreshold: 100,
		SlidingWindowSize:    100,
		Now:                  func() time.Time { return now },
	}
	cb := newCircuitBreaker(cfg)

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != "open" {
		t.Fatalf("State = %q, want open", cb.State())
	}

	// Advance time past open timeout
	now = now.Add(200 * time.Millisecond)

	if !cb.Allow() {
		t.Error("should transition to half-open and allow request")
	}
	if cb.State() != "half-open" {
		t.Errorf("State = %q, want half-open", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	now := time.Now()
	cfg := &CircuitBreakerConfig{
		FailureThreshold:     1,
		SuccessThreshold:     2,
		OpenTimeout:          100 * time.Millisecond,
		FailureRateThreshold: 100,
		SlidingWindowSize:    100,
		Now:                  func() time.Time { return now },
	}
	cb := newCircuitBreaker(cfg)

	cb.RecordFailure()
	now = now.Add(200 * time.Millisecond)
	cb.Allow() // transition to half-open

	cb.RecordSuccess()
	if cb.State() != "half-open" {
		t.Fatalf("after 1 success, State = %q, want half-open (need 2)", cb.State())
	}

	cb.RecordSuccess()
	if cb.State() != "closed" {
		t.Errorf("after 2 successes, State = %q, want closed", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	now := time.Now()
	cfg := &CircuitBreakerConfig{
		FailureThreshold:     1,
		SuccessThreshold:     2,
		OpenTimeout:          100 * time.Millisecond,
		FailureRateThreshold: 100,
		SlidingWindowSize:    100,
		Now:                  func() time.Time { return now },
	}
	cb := newCircuitBreaker(cfg)

	cb.RecordFailure()
	now = now.Add(200 * time.Millisecond)
	cb.Allow() // to half-open

	cb.RecordFailure()
	if cb.State() != "open" {
		t.Errorf("failure in half-open: State = %q, want open", cb.State())
	}
}

func TestCircuitBreaker_SlidingWindowFailureRate(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		FailureThreshold:     100, // high, so consecutive won't trigger
		SuccessThreshold:     1,
		OpenTimeout:          time.Hour,
		FailureRateThreshold: 60,
		SlidingWindowSize:    5,
	}
	cb := newCircuitBreaker(cfg)

	// Fill window: 3 failures, 2 successes = 60% failure rate
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure() // this fills the window (index wraps to 0)

	if cb.State() != "open" {
		t.Errorf("60%% failure rate with 60%% threshold: State = %q, want open", cb.State())
	}
}

func TestCircuitBreaker_ScopeIsolation(t *testing.T) {
	reg := newCircuitBreakerRegistry(&CircuitBreakerConfig{
		FailureThreshold:     2,
		SuccessThreshold:     1,
		OpenTimeout:          time.Hour,
		FailureRateThreshold: 100,
		SlidingWindowSize:    100,
	})

	cbA := reg.get("scopeA")
	cbB := reg.get("scopeB")

	cbA.RecordFailure()
	cbA.RecordFailure()

	if cbA.State() != "open" {
		t.Errorf("scopeA State = %q, want open", cbA.State())
	}
	if cbB.State() != "closed" {
		t.Errorf("scopeB State = %q, want closed (isolated)", cbB.State())
	}
}

func TestCircuitBreaker_RegistryReturnsSameInstance(t *testing.T) {
	reg := newCircuitBreakerRegistry(nil)
	a1 := reg.get("scope")
	a2 := reg.get("scope")
	if a1 != a2 {
		t.Error("expected same circuit breaker instance for same scope")
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := newCircuitBreaker(&CircuitBreakerConfig{
		FailureThreshold:     100,
		SuccessThreshold:     1,
		OpenTimeout:          time.Hour,
		FailureRateThreshold: 100,
		SlidingWindowSize:    100,
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			cb.Allow()
		}()
		go func() {
			defer wg.Done()
			cb.RecordSuccess()
		}()
		go func() {
			defer wg.Done()
			cb.RecordFailure()
		}()
	}
	wg.Wait()

	// Just verify no panic/race; state is indeterminate
	_ = cb.State()
}
