package basecamp

import (
	"context"
	"time"
)

// Hooks provides observability callbacks for SDK operations.
// Implementations can use these hooks for logging, metrics, tracing, etc.
//
// There are two levels of hooks:
//   - Operation-level: OnOperationStart/OnOperationEnd for semantic SDK operations
//   - Request-level: OnRequestStart/OnRequestEnd for HTTP requests
//
// Operation hooks provide business-meaningful spans (e.g., "Todos.Complete"),
// while request hooks capture transport-level details (retries, cache, timing).
//
// The context passed to hooks allows correlation of nested operations and requests.
type Hooks interface {
	// OnOperationStart is called when a semantic SDK operation begins.
	// Returns a context that will be passed to OnOperationEnd.
	OnOperationStart(ctx context.Context, op OperationInfo) context.Context

	// OnOperationEnd is called when a semantic SDK operation completes.
	// The ctx is the one returned from OnOperationStart.
	OnOperationEnd(ctx context.Context, op OperationInfo, err error, duration time.Duration)

	// OnRequestStart is called before an HTTP request is sent.
	// The returned context is used for the request and passed to OnRequestEnd.
	OnRequestStart(ctx context.Context, info RequestInfo) context.Context

	// OnRequestEnd is called after an HTTP request completes.
	// It receives the same context returned by OnRequestStart.
	OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult)

	// OnRetry is called before a retry attempt.
	OnRetry(ctx context.Context, info RequestInfo, attempt int, err error)
}

// GatingHooks extends Hooks with request gating capability.
// Implementations can reject operations before they execute,
// enabling patterns like circuit breakers, bulkheads, and rate limiters.
type GatingHooks interface {
	Hooks
	// OnOperationGate is called before OnOperationStart.
	// Returns a new context (which may contain cleanup functions like bulkhead
	// release) and an error. Return non-nil error to reject the operation.
	// The returned context should be used for the operation and passed to
	// OnOperationEnd for proper cleanup.
	OnOperationGate(ctx context.Context, op OperationInfo) (context.Context, error)
}

// RequestInfo contains information about an HTTP request.
type RequestInfo struct {
	Method string
	URL    string
	// Attempt is the current attempt number (1-indexed).
	Attempt int
}

// OperationInfo describes a semantic SDK operation.
// This carries more meaning than raw HTTP requests, enabling
// business-level tracing and metrics (e.g., "Todos.Complete" not "POST /url").
type OperationInfo struct {
	// Service is the logical service (e.g., "Projects", "Todos").
	Service string
	// Operation is the specific method (e.g., "List", "Create", "Complete").
	Operation string
	// ResourceType is the Basecamp resource type (e.g., "project", "todo").
	ResourceType string
	// IsMutation indicates if this operation modifies state.
	IsMutation bool
	// BucketID is the project/bucket ID if applicable.
	BucketID int64
	// ResourceID is the specific resource ID if applicable.
	ResourceID int64
}

// RequestResult contains the result of an HTTP request.
type RequestResult struct {
	// StatusCode is the HTTP status code (0 if request failed before response).
	StatusCode int
	// Duration is the time taken for the request.
	Duration time.Duration
	// Error is non-nil if the request failed.
	Error error
	// FromCache indicates the response was served from cache.
	FromCache bool
	// Retryable indicates whether this error will be retried.
	Retryable bool
}

// NoopHooks is a no-op implementation of Hooks.
// All methods are empty and designed to be inlined by the compiler,
// resulting in zero overhead when no observability is needed.
type NoopHooks struct{}

// Ensure NoopHooks implements Hooks at compile time.
var _ Hooks = NoopHooks{}

// OnOperationStart does nothing and returns the context unchanged.
func (NoopHooks) OnOperationStart(ctx context.Context, _ OperationInfo) context.Context { return ctx }

// OnOperationEnd does nothing.
func (NoopHooks) OnOperationEnd(context.Context, OperationInfo, error, time.Duration) {}

// OnRequestStart does nothing and returns the context unchanged.
func (NoopHooks) OnRequestStart(ctx context.Context, _ RequestInfo) context.Context { return ctx }

// OnRequestEnd does nothing.
func (NoopHooks) OnRequestEnd(context.Context, RequestInfo, RequestResult) {}

// OnRetry does nothing.
func (NoopHooks) OnRetry(context.Context, RequestInfo, int, error) {}

// ChainHooks combines multiple Hooks implementations.
// Start events are called in order, end events are called in reverse order.
// This allows proper nesting of spans/traces.
type ChainHooks struct {
	hooks []Hooks
}

// NewChainHooks creates a ChainHooks from the given hooks.
// Nil hooks are filtered out. If all hooks are nil, returns NoopHooks.
func NewChainHooks(hooks ...Hooks) Hooks {
	filtered := make([]Hooks, 0, len(hooks))
	for _, h := range hooks {
		if h != nil {
			// Skip NoopHooks as they add no value
			if _, isNoop := h.(NoopHooks); !isNoop {
				filtered = append(filtered, h)
			}
		}
	}
	if len(filtered) == 0 {
		return NoopHooks{}
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return &ChainHooks{hooks: filtered}
}

// OnOperationStart calls all hooks in order.
func (c *ChainHooks) OnOperationStart(ctx context.Context, op OperationInfo) context.Context {
	for _, h := range c.hooks {
		ctx = h.OnOperationStart(ctx, op)
	}
	return ctx
}

// OnOperationEnd calls all hooks in reverse order.
func (c *ChainHooks) OnOperationEnd(ctx context.Context, op OperationInfo, err error, duration time.Duration) {
	for i := len(c.hooks) - 1; i >= 0; i-- {
		c.hooks[i].OnOperationEnd(ctx, op, err, duration)
	}
}

// OnRequestStart calls all hooks in order.
func (c *ChainHooks) OnRequestStart(ctx context.Context, info RequestInfo) context.Context {
	for _, h := range c.hooks {
		ctx = h.OnRequestStart(ctx, info)
	}
	return ctx
}

// OnRequestEnd calls all hooks in reverse order.
func (c *ChainHooks) OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult) {
	for i := len(c.hooks) - 1; i >= 0; i-- {
		c.hooks[i].OnRequestEnd(ctx, info, result)
	}
}

// OnRetry calls all hooks in order.
func (c *ChainHooks) OnRetry(ctx context.Context, info RequestInfo, attempt int, err error) {
	for _, h := range c.hooks {
		h.OnRetry(ctx, info, attempt, err)
	}
}

// OnOperationGate calls the first GatingHooks implementation in the chain.
// Only ONE gater should exist in a chain (typically resilienceHooks which
// internally manages circuit breaker, bulkhead, and rate limiter).
// Returns the context from the gater (which may contain cleanup functions)
// and any error.
func (c *ChainHooks) OnOperationGate(ctx context.Context, op OperationInfo) (context.Context, error) {
	for _, h := range c.hooks {
		if gater, ok := h.(GatingHooks); ok {
			return gater.OnOperationGate(ctx, op)
		}
	}
	return ctx, nil
}

// WithHooks sets the observability hooks for the client.
// Pass nil to disable hooks (uses NoopHooks).
func WithHooks(hooks Hooks) ClientOption {
	return func(c *Client) {
		if hooks == nil {
			c.hooks = NoopHooks{}
		} else {
			c.hooks = hooks
		}
	}
}
