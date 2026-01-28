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
	rh := &resilienceHooks{inner: NoopHooks{}}
	var _ GatingHooks = rh // Compile-time check
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

	t.Run("stores release function in context", func(t *testing.T) {
		rh := &resilienceHooks{
			inner:     NoopHooks{},
			bulkheads: newBulkheadRegistry(&BulkheadConfig{MaxConcurrent: 2, MaxWait: 0}),
		}

		// Gate should return context with release function
		resultCtx, err := rh.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("should allow: %v", err)
		}

		// Verify release function is in context
		release, ok := resultCtx.Value(bulkheadReleaseKey{}).(func())
		if !ok || release == nil {
			t.Error("context should contain bulkhead release function")
		}

		// Verify slot is in use
		bh := rh.bulkheads.get("Todos.Create")
		if bh.InUse() != 1 {
			t.Errorf("bulkhead should have 1 slot in use: got %d", bh.InUse())
		}

		// OnOperationEnd should release the slot
		rh.OnOperationEnd(resultCtx, op, nil, time.Second)
		if bh.InUse() != 0 {
			t.Errorf("bulkhead should have 0 slots after OnOperationEnd: got %d", bh.InUse())
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
	cfg.AccountID = "12345"

	// This should not panic
	_ = NewClient(cfg, &StaticTokenProvider{Token: "test"},
		WithResilience(nil),
	)
}

func TestWithCircuitBreaker(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AccountID = "12345"

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
	cfg.AccountID = "12345"

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
	cfg.AccountID = "12345"

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
