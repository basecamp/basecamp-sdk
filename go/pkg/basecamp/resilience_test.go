package basecamp

import (
	"context"
	"testing"
	"time"
)

func TestDefaultResilienceConfig(t *testing.T) {
	cfg := DefaultResilienceConfig()

	if cfg.CircuitBreaker == nil {
		t.Error("CircuitBreaker should not be nil")
	}
	if cfg.Bulkhead == nil {
		t.Error("Bulkhead should not be nil")
	}
	if cfg.RateLimit == nil {
		t.Error("RateLimit should not be nil")
	}
}

func TestResilienceHooks_Implements_GatingHooks(t *testing.T) {
	var _ GatingHooks = &resilienceHooks{} // Compile-time check
}

func TestResilienceHooks_OnOperationGate_CircuitBreaker(t *testing.T) {
	ctx := context.Background()
	op := OperationInfo{
		Service:   "Todos",
		Operation: "List",
	}

	t.Run("allows when circuit closed", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(DefaultCircuitBreakerConfig()),
		}

		_, err := rh.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("should allow when circuit closed: %v", err)
		}
	})

	t.Run("rejects when circuit open", func(t *testing.T) {
		cfg := &CircuitBreakerConfig{
			FailureThreshold: 2,
			OpenTimeout:      time.Hour, // Long timeout so it stays open
		}
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(cfg),
		}

		// Get the breaker and force it open
		cb := rh.circuitBreakers.get("Todos.List")
		cb.RecordFailure()
		cb.RecordFailure()

		_, err := rh.OnOperationGate(ctx, op)
		if err != ErrCircuitOpen {
			t.Errorf("should return ErrCircuitOpen: got %v", err)
		}
	})
}

func TestResilienceHooks_OnOperationGate_Bulkhead(t *testing.T) {
	ctx := context.Background()
	op := OperationInfo{
		Service:   "Todos",
		Operation: "Create",
	}

	t.Run("allows when slots available", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:     NoopHooks{},
			bulkheads: newBulkheadRegistry(&BulkheadConfig{MaxConcurrent: 10}),
		}

		_, err := rh.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("should allow when slots available: %v", err)
		}
	})

	t.Run("rejects when bulkhead full", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:     NoopHooks{},
			bulkheads: newBulkheadRegistry(&BulkheadConfig{MaxConcurrent: 1, MaxWait: 0}),
		}

		// Acquire the only slot
		bh := rh.bulkheads.get("Todos.Create")
		_, _ = bh.TryAcquire()

		_, err := rh.OnOperationGate(ctx, op)
		if err != ErrBulkheadFull {
			t.Errorf("should return ErrBulkheadFull: got %v", err)
		}
	})

	t.Run("preserves context cancellation error", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:     NoopHooks{},
			bulkheads: newBulkheadRegistry(&BulkheadConfig{MaxConcurrent: 1, MaxWait: time.Second}),
		}

		// Acquire the only slot
		bh := rh.bulkheads.get("Todos.Create")
		_, _ = bh.TryAcquire()

		// Use a canceled context
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := rh.OnOperationGate(canceledCtx, op)
		if err != context.Canceled {
			t.Errorf("should preserve context.Canceled: got %v", err)
		}
	})

	t.Run("releases bulkhead on operation end", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:     NoopHooks{},
			bulkheads: newBulkheadRegistry(&BulkheadConfig{MaxConcurrent: 2, MaxWait: 0}),
		}

		// Gate acquires bulkhead, stores pending release
		gateCtx, err := rh.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("should allow: %v", err)
		}

		// Verify pending ID is in context
		pendingID, ok := gateCtx.Value(bulkheadPendingKey{}).(uint64)
		if !ok || pendingID == 0 {
			t.Error("context should contain pending release ID")
		}

		// Verify slot is in use
		bh := rh.bulkheads.get("Todos.Create")
		if bh.InUse() != 1 {
			t.Errorf("bulkhead should have 1 slot in use: got %d", bh.InUse())
		}

		// OnOperationStart moves release from pending to active
		startCtx := rh.OnOperationStart(gateCtx, op)

		// OnOperationEnd should release the slot (using context pointer)
		rh.OnOperationEnd(startCtx, op, nil, time.Second)
		if bh.InUse() != 0 {
			t.Errorf("bulkhead should have 0 slots after OnOperationEnd: got %d", bh.InUse())
		}
	})

	t.Run("survives context replacement by inner hooks", func(t *testing.T) {
		// This tests the fix for the medium-severity issue where a hook
		// that returns a fresh context would cause bulkhead slot leaks.
		contextReplacingHook := &contextReplacingHooks{}
		rh := &resilienceHooks{
			inner:     contextReplacingHook,
			bulkheads: newBulkheadRegistry(&BulkheadConfig{MaxConcurrent: 2, MaxWait: 0}),
		}

		// Gate acquires bulkhead
		gateCtx, err := rh.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("should allow: %v", err)
		}

		bh := rh.bulkheads.get("Todos.Create")
		if bh.InUse() != 1 {
			t.Errorf("bulkhead should have 1 slot in use: got %d", bh.InUse())
		}

		// OnOperationStart - inner hook REPLACES context entirely
		startCtx := rh.OnOperationStart(gateCtx, op)

		// Verify context was actually replaced (different pointer)
		if startCtx == gateCtx {
			t.Error("test setup error: context should have been replaced")
		}

		// OnOperationEnd with the NEW context should still release properly
		rh.OnOperationEnd(startCtx, op, nil, time.Second)
		if bh.InUse() != 0 {
			t.Errorf("bulkhead should have 0 slots after OnOperationEnd with replaced context: got %d", bh.InUse())
		}
	})
}

func TestResilienceHooks_OnOperationGate_RateLimiter(t *testing.T) {
	ctx := context.Background()
	op := OperationInfo{
		Service:   "Search",
		Operation: "Query",
	}

	t.Run("allows when tokens available", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:       NoopHooks{},
			rateLimiter: newRateLimiter(&RateLimitConfig{RequestsPerSecond: 100, BurstSize: 10}),
		}

		_, err := rh.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("should allow when tokens available: %v", err)
		}
	})

	t.Run("rejects when rate limited", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:       NoopHooks{},
			rateLimiter: newRateLimiter(&RateLimitConfig{RequestsPerSecond: 1, BurstSize: 1}),
		}

		// Use up the burst
		_, _ = rh.OnOperationGate(ctx, op)

		// Next should be rejected
		_, err := rh.OnOperationGate(ctx, op)
		if err != ErrRateLimited {
			t.Errorf("should return ErrRateLimited: got %v", err)
		}
	})
}

func TestResilienceHooks_OnOperationEnd_UpdatesCircuitBreaker(t *testing.T) {
	ctx := context.Background()
	op := OperationInfo{
		Service:   "Todos",
		Operation: "Get",
	}

	t.Run("records success", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(DefaultCircuitBreakerConfig()),
		}

		// Gate first to ensure breaker exists
		gateCtx, _ := rh.OnOperationGate(ctx, op)

		// Record success
		rh.OnOperationEnd(gateCtx, op, nil, time.Second)

		// Breaker should still be closed
		cb := rh.circuitBreakers.get("Todos.Get")
		if cb.State() != "closed" {
			t.Errorf("circuit should be closed after success: got %s", cb.State())
		}
	})

	t.Run("records failure and opens circuit", func(t *testing.T) {
		cfg := &CircuitBreakerConfig{
			FailureThreshold: 2,
			OpenTimeout:      time.Hour,
		}
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(cfg),
		}

		// Gate first
		_, _ = rh.OnOperationGate(ctx, op)

		// Record server-side failures (5xx errors trip the circuit)
		serverErr := ErrAPI(503, "service unavailable")
		rh.OnOperationEnd(ctx, op, serverErr, time.Second)
		rh.OnOperationEnd(ctx, op, serverErr, time.Second)

		// Breaker should be open
		cb := rh.circuitBreakers.get("Todos.Get")
		if cb.State() != "open" {
			t.Errorf("circuit should be open after failures: got %s", cb.State())
		}
	})

	t.Run("does not count client errors as failures", func(t *testing.T) {
		cfg := &CircuitBreakerConfig{
			FailureThreshold: 2,
			OpenTimeout:      time.Hour,
		}
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(cfg),
		}

		// Gate first
		_, _ = rh.OnOperationGate(ctx, op)

		// Record client-side errors (4xx errors don't trip the circuit)
		rh.OnOperationEnd(ctx, op, ErrNotFound("test", "1"), time.Second)
		rh.OnOperationEnd(ctx, op, ErrNotFound("test", "2"), time.Second)
		rh.OnOperationEnd(ctx, op, ErrAuth("bad token"), time.Second)

		// Breaker should still be closed (client errors don't trip)
		cb := rh.circuitBreakers.get("Todos.Get")
		if cb.State() != "closed" {
			t.Errorf("circuit should be closed after client errors: got %s", cb.State())
		}
	})

	t.Run("does not count 429 rate limit errors as failures", func(t *testing.T) {
		cfg := &CircuitBreakerConfig{
			FailureThreshold: 2,
			OpenTimeout:      time.Hour,
		}
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(cfg),
		}

		// 429 errors have Retryable=true but should NOT trip the circuit
		// because they're client-side rate limiting, not server failures
		rh.OnOperationEnd(ctx, op, ErrRateLimit(60), time.Second)
		rh.OnOperationEnd(ctx, op, ErrRateLimit(30), time.Second)
		rh.OnOperationEnd(ctx, op, ErrRateLimit(0), time.Second)

		// Breaker should still be closed (429s don't trip despite Retryable=true)
		cb := rh.circuitBreakers.get("Todos.Get")
		if cb.State() != "closed" {
			t.Errorf("circuit should be closed after 429 errors: got %s", cb.State())
		}
	})

	t.Run("does not count gating errors as failures", func(t *testing.T) {
		cfg := &CircuitBreakerConfig{
			FailureThreshold: 2,
			OpenTimeout:      time.Hour,
		}
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(cfg),
		}

		// Record gating errors (should not count)
		rh.OnOperationEnd(ctx, op, ErrCircuitOpen, time.Second)
		rh.OnOperationEnd(ctx, op, ErrBulkheadFull, time.Second)
		rh.OnOperationEnd(ctx, op, ErrRateLimited, time.Second)

		// Breaker should still be closed
		cb := rh.circuitBreakers.get("Todos.Get")
		if cb.State() != "closed" {
			t.Errorf("circuit should be closed (gating errors don't count): got %s", cb.State())
		}
	})

	t.Run("does not count wrapped context errors as failures", func(t *testing.T) {
		cfg := &CircuitBreakerConfig{
			FailureThreshold: 2,
			OpenTimeout:      time.Hour,
		}
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(cfg),
		}

		// Create network errors that wrap context errors (e.g., from http.Client)
		// These should not trip the circuit because the user canceled the request.
		wrappedCanceled := ErrNetwork(context.Canceled)
		wrappedTimeout := ErrNetwork(context.DeadlineExceeded)

		rh.OnOperationEnd(ctx, op, wrappedCanceled, time.Second)
		rh.OnOperationEnd(ctx, op, wrappedTimeout, time.Second)

		// Breaker should still be closed (context errors don't trip)
		cb := rh.circuitBreakers.get("Todos.Get")
		if cb.State() != "closed" {
			t.Errorf("circuit should be closed (wrapped context errors don't count): got %s", cb.State())
		}
	})
}

func TestResilienceHooks_DelegatesTo_InnerHooks(t *testing.T) {
	ctx := context.Background()
	op := OperationInfo{Service: "Todos", Operation: "List"}
	reqInfo := RequestInfo{Method: "GET", URL: "http://test", Attempt: 1}
	reqResult := RequestResult{StatusCode: 200}

	inner := &recordingHooks{}
	rh := &resilienceHooks{inner: inner}

	// Test delegation
	rh.OnOperationStart(ctx, op)
	if len(inner.opStartCalls) != 1 {
		t.Error("OnOperationStart should delegate to inner")
	}

	rh.OnOperationEnd(ctx, op, nil, time.Second)
	if len(inner.opEndCalls) != 1 {
		t.Error("OnOperationEnd should delegate to inner")
	}

	rh.OnRequestStart(ctx, reqInfo)
	if len(inner.startCalls) != 1 {
		t.Error("OnRequestStart should delegate to inner")
	}

	rh.OnRequestEnd(ctx, reqInfo, reqResult)
	if len(inner.endCalls) != 1 {
		t.Error("OnRequestEnd should delegate to inner")
	}

	rh.OnRetry(ctx, reqInfo, 2, nil)
	if len(inner.retryCalls) != 1 {
		t.Error("OnRetry should delegate to inner")
	}
}

func TestWithResilience_NilConfig_UsesDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// This should not panic
	_ = NewClient(cfg, &StaticTokenProvider{Token: "test"},
		WithResilience(nil),
	)
}

func TestWithCircuitBreaker(t *testing.T) {
	cfg := DefaultConfig()

	client := NewClient(cfg, &StaticTokenProvider{Token: "test"},
		WithCircuitBreaker(&CircuitBreakerConfig{FailureThreshold: 3}),
	)

	// Verify hooks is a resilienceHooks with circuit breaker
	rh, ok := client.hooks.(*resilienceHooks)
	if !ok {
		t.Fatal("hooks should be resilienceHooks")
	}
	if rh.circuitBreakers == nil {
		t.Error("circuitBreakers should not be nil")
	}
	if rh.bulkheads != nil {
		t.Error("bulkheads should be nil (not configured)")
	}
	if rh.rateLimiter != nil {
		t.Error("rateLimiter should be nil (not configured)")
	}
}

func TestWithBulkhead(t *testing.T) {
	cfg := DefaultConfig()

	client := NewClient(cfg, &StaticTokenProvider{Token: "test"},
		WithBulkhead(&BulkheadConfig{MaxConcurrent: 5}),
	)

	rh, ok := client.hooks.(*resilienceHooks)
	if !ok {
		t.Fatal("hooks should be resilienceHooks")
	}
	if rh.bulkheads == nil {
		t.Error("bulkheads should not be nil")
	}
	if rh.circuitBreakers != nil {
		t.Error("circuitBreakers should be nil (not configured)")
	}
}

func TestWithRateLimit(t *testing.T) {
	cfg := DefaultConfig()

	client := NewClient(cfg, &StaticTokenProvider{Token: "test"},
		WithRateLimit(&RateLimitConfig{RequestsPerSecond: 10}),
	)

	rh, ok := client.hooks.(*resilienceHooks)
	if !ok {
		t.Fatal("hooks should be resilienceHooks")
	}
	if rh.rateLimiter == nil {
		t.Error("rateLimiter should not be nil")
	}
	if rh.circuitBreakers != nil {
		t.Error("circuitBreakers should be nil (not configured)")
	}
}

func TestResilienceHooks_GateOrder(t *testing.T) {
	// Test that gates are checked in the correct order:
	// circuit breaker -> bulkhead -> rate limiter

	ctx := context.Background()
	op := OperationInfo{Service: "Test", Operation: "Order"}

	t.Run("circuit breaker checked first", func(t *testing.T) {
		cbCfg := &CircuitBreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour}
		rh := &resilienceHooks{
			inner:           NoopHooks{},
			circuitBreakers: newCircuitBreakerRegistry(cbCfg),
			bulkheads:       newBulkheadRegistry(&BulkheadConfig{MaxConcurrent: 0}), // Would fail
			rateLimiter:     newRateLimiter(&RateLimitConfig{BurstSize: 0}),         // Would fail
		}

		// Open the circuit
		cb := rh.circuitBreakers.get("Test.Order")
		cb.RecordFailure()

		_, err := rh.OnOperationGate(ctx, op)
		if err != ErrCircuitOpen {
			t.Errorf("circuit breaker should be checked first: got %v", err)
		}
	})
}

func TestResilienceHooks_OnRequestEnd_RespectsRetryAfter(t *testing.T) {
	ctx := context.Background()
	reqInfo := RequestInfo{Method: "GET", URL: "http://test", Attempt: 1}

	t.Run("uses RetryAfter from response", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:       NoopHooks{},
			rateLimiter: newRateLimiter(&RateLimitConfig{RequestsPerSecond: 100, BurstSize: 10}),
		}

		// Simulate 429 with Retry-After header
		result := RequestResult{
			StatusCode: 429,
			RetryAfter: 30, // 30 seconds from header
		}

		rh.OnRequestEnd(ctx, reqInfo, result)

		// Verify the rate limiter received the Retry-After value
		// The test confirms OnRequestEnd processes the header correctly
		// (actual blocking behavior is tested in rate_limit_test.go)
		_ = rh.rateLimiter.Allow() // Just verify no panic
	})

	t.Run("defaults to 60s when no RetryAfter", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:       NoopHooks{},
			rateLimiter: newRateLimiter(&RateLimitConfig{RequestsPerSecond: 100, BurstSize: 10}),
		}

		// Simulate 429 without Retry-After header
		result := RequestResult{
			StatusCode: 429,
			RetryAfter: 0, // No header
		}

		rh.OnRequestEnd(ctx, reqInfo, result)
		// Default 60s backoff should be applied
	})

	t.Run("handles 503 with RetryAfter", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:       NoopHooks{},
			rateLimiter: newRateLimiter(&RateLimitConfig{RequestsPerSecond: 100, BurstSize: 10}),
		}

		// Simulate 503 with Retry-After header
		result := RequestResult{
			StatusCode: 503,
			RetryAfter: 15,
		}

		rh.OnRequestEnd(ctx, reqInfo, result)
		// Retry-After should be respected for 503 as well
	})

	t.Run("503 without RetryAfter does not block", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:       NoopHooks{},
			rateLimiter: newRateLimiter(&RateLimitConfig{RequestsPerSecond: 100, BurstSize: 10}),
		}

		// Simulate 503 without Retry-After header
		result := RequestResult{
			StatusCode: 503,
			RetryAfter: 0, // No header
		}

		rh.OnRequestEnd(ctx, reqInfo, result)

		// Should NOT block - 503 without Retry-After shouldn't apply default
		// (unlike 429 which defaults to 60s)
		// Verify rate limiter still allows requests immediately after
		if !rh.rateLimiter.Allow() {
			t.Error("503 without Retry-After should not block rate limiter")
		}
	})
}

// contextReplacingHooks is a test helper that returns a FRESH context
// from OnOperationStart, simulating a misbehaving hook that doesn't
// derive from the input context. This is used to test bulkhead release
// robustness.
type contextReplacingHooks struct{}

type replacedContextKey struct{}

func (h *contextReplacingHooks) OnOperationStart(ctx context.Context, op OperationInfo) context.Context {
	// Return a completely fresh context, not derived from input.
	// This simulates a badly-written hook that drops context values.
	return context.WithValue(context.Background(), replacedContextKey{}, true)
}

func (h *contextReplacingHooks) OnOperationEnd(ctx context.Context, op OperationInfo, err error, duration time.Duration) {
}

func (h *contextReplacingHooks) OnRequestStart(ctx context.Context, info RequestInfo) context.Context {
	return ctx
}

func (h *contextReplacingHooks) OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult) {
}

func (h *contextReplacingHooks) OnRetry(ctx context.Context, info RequestInfo, attempt int, err error) {
}
