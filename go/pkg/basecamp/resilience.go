package basecamp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ResilienceConfig combines all resilience settings.
// Use DefaultResilienceConfig() for production-ready defaults.
type ResilienceConfig struct {
	// CircuitBreaker configuration. If nil, circuit breaker is disabled.
	CircuitBreaker *CircuitBreakerConfig

	// Bulkhead configuration. If nil, bulkhead is disabled.
	Bulkhead *BulkheadConfig

	// RateLimit configuration. If nil, rate limiting is disabled.
	RateLimit *RateLimitConfig
}

// DefaultResilienceConfig returns production-ready defaults for all resilience features.
func DefaultResilienceConfig() *ResilienceConfig {
	return &ResilienceConfig{
		CircuitBreaker: DefaultCircuitBreakerConfig(),
		Bulkhead:       DefaultBulkheadConfig(),
		RateLimit:      DefaultRateLimitConfig(),
	}
}

// resilienceHooks implements GatingHooks to provide resilience patterns.
// It wraps an inner Hooks implementation and adds gating behavior.
type resilienceHooks struct {
	inner           Hooks
	circuitBreakers *circuitBreakerRegistry
	bulkheads       *bulkheadRegistry
	rateLimiter     *rateLimiter

	// Bulkhead release tracking uses a two-phase approach for robustness:
	// 1. OnOperationGate stores release in pendingReleases (keyed by unique ID)
	// 2. OnOperationStart moves it to activeReleases (keyed by same ID)
	//    and stores the ID in the FINAL context returned to caller
	// This ensures proper cleanup even if inner hooks replace the context,
	// and avoids collisions when multiple operations share a context pointer.
	releaseCounter  atomic.Uint64
	pendingReleases sync.Map // map[uint64]func() - releases awaiting OnOperationStart
	activeReleases  sync.Map // map[uint64]func() - releases keyed by unique ID
}

// Ensure resilienceHooks implements GatingHooks at compile time.
var _ GatingHooks = (*resilienceHooks)(nil)

// bulkheadPendingKey is the context key for the pending release ID (before OnOperationStart).
type bulkheadPendingKey struct{}

// bulkheadActiveKey is the context key for the active release ID (after OnOperationStart).
// This ID is stored in the FINAL context returned from OnOperationStart, ensuring it
// survives even if inner hooks replaced the context.
type bulkheadActiveKey struct{}

// shouldTripCircuit returns true if the error should count as a circuit breaker failure.
// Only server-side errors (5xx, network) trip the circuit. Client-side errors (4xx,
// validation, auth, not-found) do not trip the circuit since they indicate problems
// with the request, not the service.
func shouldTripCircuit(err error) bool {
	// Gating errors never trip the circuit
	if err == ErrCircuitOpen || err == ErrBulkheadFull || err == ErrRateLimited {
		return false
	}

	// Context errors (canceled, deadline exceeded) don't indicate server problems.
	// Use errors.Is to catch wrapped errors (e.g., ErrNetwork wrapping context.Canceled).
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check if it's our structured Error type
	if e, ok := err.(*Error); ok {
		// Network errors trip the circuit UNLESS they wrap context errors
		// (which we already checked above with errors.Is)
		if e.Code == CodeNetwork {
			return true
		}
		// 5xx errors trip the circuit (including those marked Retryable)
		if e.HTTPStatus >= 500 {
			return true
		}
		// 4xx errors (auth, not-found, forbidden, rate-limit, usage) don't trip
		// even if marked Retryable (e.g., 429 is retryable but shouldn't open circuit)
		return false
	}

	// Unknown error types are assumed to be server-side failures
	return true
}

// OnOperationGate checks all resilience gates before allowing an operation.
// Gates are checked in order: circuit breaker, bulkhead, rate limiter.
// Returns a context that may contain cleanup functions (e.g., bulkhead release).
func (h *resilienceHooks) OnOperationGate(ctx context.Context, op OperationInfo) (context.Context, error) {
	scope := op.Service + "." + op.Operation

	// Check circuit breaker first (fast path rejection)
	if h.circuitBreakers != nil {
		cb := h.circuitBreakers.get(scope)
		if !cb.Allow() {
			return ctx, ErrCircuitOpen
		}
	}

	// Acquire bulkhead slot and store pending release ID in context.
	// The release is moved to activeReleases in OnOperationStart, and the ID
	// is stored in the final context. This ensures proper cleanup even if
	// inner hooks replace the context entirely.
	if h.bulkheads != nil {
		bh := h.bulkheads.get(scope)
		release, err := bh.Acquire(ctx)
		if err != nil {
			// Preserve context errors (canceled, deadline exceeded) rather than masking
			if ctx.Err() != nil {
				return ctx, ctx.Err()
			}
			return ctx, ErrBulkheadFull
		}
		// Store release in pending map with unique ID
		pendingID := h.releaseCounter.Add(1)
		h.pendingReleases.Store(pendingID, release)
		ctx = context.WithValue(ctx, bulkheadPendingKey{}, pendingID)
	}

	// Rate limit (fail fast if no tokens available)
	if h.rateLimiter != nil {
		if !h.rateLimiter.Allow() {
			// Release bulkhead if we acquired one (still in pending)
			if pendingID, ok := ctx.Value(bulkheadPendingKey{}).(uint64); ok {
				if release, loaded := h.pendingReleases.LoadAndDelete(pendingID); loaded {
					release.(func())()
				}
			}
			return ctx, ErrRateLimited
		}
	}

	return ctx, nil
}

// OnOperationStart delegates to the inner hooks and finalizes bulkhead tracking.
// After inner hooks run (which may replace the context), we store the release ID
// in the FINAL context, ensuring proper cleanup in OnOperationEnd.
func (h *resilienceHooks) OnOperationStart(ctx context.Context, op OperationInfo) context.Context {
	// First, let inner hooks process (they may replace ctx)
	resultCtx := h.inner.OnOperationStart(ctx, op)

	// Move bulkhead release from pending to active.
	// Key by unique ID (not context pointer) to avoid collisions when
	// multiple operations share the same context (e.g., context.Background()).
	// Store the ID in the FINAL context so OnOperationEnd can find it.
	if pendingID, ok := ctx.Value(bulkheadPendingKey{}).(uint64); ok {
		if release, loaded := h.pendingReleases.LoadAndDelete(pendingID); loaded {
			h.activeReleases.Store(pendingID, release)
			// Add our ID to the final context (survives even if hooks replaced ctx)
			resultCtx = context.WithValue(resultCtx, bulkheadActiveKey{}, pendingID)
		}
	}

	return resultCtx
}

// OnOperationEnd updates circuit breaker state, releases bulkhead, and delegates to inner hooks.
func (h *resilienceHooks) OnOperationEnd(ctx context.Context, op OperationInfo, err error, duration time.Duration) {
	scope := op.Service + "." + op.Operation

	// Release bulkhead slot if one was acquired (keyed by unique ID from context)
	if releaseID, ok := ctx.Value(bulkheadActiveKey{}).(uint64); ok {
		if release, loaded := h.activeReleases.LoadAndDelete(releaseID); loaded {
			release.(func())()
		}
	}

	// Update circuit breaker state based on result
	if h.circuitBreakers != nil {
		cb := h.circuitBreakers.get(scope)
		if err != nil && shouldTripCircuit(err) {
			cb.RecordFailure()
		} else if err == nil {
			cb.RecordSuccess()
		}
		// Note: client-side errors (validation, 4xx) neither trip nor heal the circuit
	}

	// Delegate to inner hooks
	h.inner.OnOperationEnd(ctx, op, err, duration)
}

// OnRequestStart delegates to the inner hooks.
func (h *resilienceHooks) OnRequestStart(ctx context.Context, info RequestInfo) context.Context {
	return h.inner.OnRequestStart(ctx, info)
}

// OnRequestEnd delegates to the inner hooks and handles Retry-After.
func (h *resilienceHooks) OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult) {
	// Handle 429/503 Retry-After headers
	if h.rateLimiter != nil {
		if result.StatusCode == 429 {
			retryAfter := result.RetryAfter
			if retryAfter <= 0 {
				// Default to 60 seconds for 429 if no Retry-After header provided
				retryAfter = 60
			}
			h.rateLimiter.SetRetryAfterDuration(time.Duration(retryAfter) * time.Second)
		} else if result.StatusCode == 503 && result.RetryAfter > 0 {
			// Only honor explicit Retry-After for 503 (no default)
			h.rateLimiter.SetRetryAfterDuration(time.Duration(result.RetryAfter) * time.Second)
		}
	}

	h.inner.OnRequestEnd(ctx, info, result)
}

// OnRetry delegates to the inner hooks.
func (h *resilienceHooks) OnRetry(ctx context.Context, info RequestInfo, attempt int, err error) {
	h.inner.OnRetry(ctx, info, attempt, err)
}

// WithResilience enables circuit breaker, bulkhead, and rate limiting.
// Pass nil to use DefaultResilienceConfig().
//
// Example:
//
//	client := basecamp.NewClient(cfg, tokenProvider,
//	    basecamp.WithResilience(nil), // Uses defaults
//	)
//
// Or with custom config:
//
//	client := basecamp.NewClient(cfg, tokenProvider,
//	    basecamp.WithResilience(&basecamp.ResilienceConfig{
//	        CircuitBreaker: &basecamp.CircuitBreakerConfig{
//	            FailureThreshold: 3,
//	            OpenTimeout:      10 * time.Second,
//	        },
//	        Bulkhead: &basecamp.BulkheadConfig{
//	            MaxConcurrent: 5,
//	        },
//	        RateLimit: &basecamp.RateLimitConfig{
//	            RequestsPerSecond: 10,
//	        },
//	    }),
//	)
func WithResilience(cfg *ResilienceConfig) ClientOption {
	return func(c *Client) {
		if cfg == nil {
			cfg = DefaultResilienceConfig()
		}

		rh := &resilienceHooks{
			inner: c.hooks,
		}

		if cfg.CircuitBreaker != nil {
			rh.circuitBreakers = newCircuitBreakerRegistry(cfg.CircuitBreaker)
		}
		if cfg.Bulkhead != nil {
			rh.bulkheads = newBulkheadRegistry(cfg.Bulkhead)
		}
		if cfg.RateLimit != nil {
			rh.rateLimiter = newRateLimiter(cfg.RateLimit)
		}

		c.hooks = rh
	}
}

// WithCircuitBreaker enables only the circuit breaker.
//
// Example:
//
//	client := basecamp.NewClient(cfg, tokenProvider,
//	    basecamp.WithCircuitBreaker(&basecamp.CircuitBreakerConfig{
//	        FailureThreshold: 10,
//	    }),
//	)
func WithCircuitBreaker(cfg *CircuitBreakerConfig) ClientOption {
	return func(c *Client) {
		if cfg == nil {
			cfg = DefaultCircuitBreakerConfig()
		}

		rh := &resilienceHooks{
			inner:           c.hooks,
			circuitBreakers: newCircuitBreakerRegistry(cfg),
		}

		c.hooks = rh
	}
}

// WithBulkhead enables only the bulkhead (concurrency limiter).
//
// Example:
//
//	client := basecamp.NewClient(cfg, tokenProvider,
//	    basecamp.WithBulkhead(&basecamp.BulkheadConfig{
//	        MaxConcurrent: 5,
//	        MaxWait:       10 * time.Second,
//	    }),
//	)
func WithBulkhead(cfg *BulkheadConfig) ClientOption {
	return func(c *Client) {
		if cfg == nil {
			cfg = DefaultBulkheadConfig()
		}

		rh := &resilienceHooks{
			inner:     c.hooks,
			bulkheads: newBulkheadRegistry(cfg),
		}

		c.hooks = rh
	}
}

// WithRateLimit enables only client-side rate limiting.
//
// Example:
//
//	client := basecamp.NewClient(cfg, tokenProvider,
//	    basecamp.WithRateLimit(&basecamp.RateLimitConfig{
//	        RequestsPerSecond: 10,
//	        BurstSize:         5,
//	    }),
//	)
func WithRateLimit(cfg *RateLimitConfig) ClientOption {
	return func(c *Client) {
		if cfg == nil {
			cfg = DefaultRateLimitConfig()
		}

		rh := &resilienceHooks{
			inner:       c.hooks,
			rateLimiter: newRateLimiter(cfg),
		}

		c.hooks = rh
	}
}
