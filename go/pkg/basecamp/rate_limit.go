package basecamp

import (
	"context"
	"sync"
	"time"
)

// RateLimitConfig configures client-side rate limiting.
type RateLimitConfig struct {
	// RequestsPerSecond is the sustained rate of requests allowed.
	// Default: 50
	RequestsPerSecond float64

	// BurstSize is the maximum number of requests allowed in a burst.
	// Default: 10
	BurstSize int

	// RespectRetryAfter honors 429 Retry-After headers by blocking requests
	// until the server-specified time has passed.
	// Default: true
	RespectRetryAfter bool

	// Now is a function that returns the current time. Used for testing.
	// If nil, time.Now is used.
	Now func() time.Time
}

// DefaultRateLimitConfig returns production-ready defaults.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: 50,
		BurstSize:         10,
		RespectRetryAfter: true,
	}
}

// rateLimiter implements the token bucket algorithm for rate limiting.
// Thread-safe for concurrent access.
type rateLimiter struct {
	config *RateLimitConfig

	mu             sync.Mutex
	tokens         float64
	lastRefillTime time.Time

	// retryAfterUntil is set when a 429 with Retry-After is received.
	// No requests are allowed until this time.
	retryAfterUntil time.Time
}

// newRateLimiter creates a new rate limiter with the given config.
func newRateLimiter(config *RateLimitConfig) *rateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}
	// Apply defaults for zero values
	if config.RequestsPerSecond <= 0 {
		config.RequestsPerSecond = 50
	}
	if config.BurstSize <= 0 {
		config.BurstSize = 10
	}

	now := time.Now()
	if config.Now != nil {
		now = config.Now()
	}

	return &rateLimiter{
		config:         config,
		tokens:         float64(config.BurstSize), // Start with full bucket
		lastRefillTime: now,
	}
}

// now returns the current time using the config's Now function or time.Now.
func (r *rateLimiter) now() time.Time {
	if r.config.Now != nil {
		return r.config.Now()
	}
	return time.Now()
}

// refill adds tokens based on elapsed time since last refill.
// Must be called with the mutex held.
func (r *rateLimiter) refill() {
	now := r.now()
	elapsed := now.Sub(r.lastRefillTime)
	r.lastRefillTime = now

	// Add tokens based on elapsed time
	tokensToAdd := elapsed.Seconds() * r.config.RequestsPerSecond
	r.tokens += tokensToAdd

	// Cap at burst size
	if r.tokens > float64(r.config.BurstSize) {
		r.tokens = float64(r.config.BurstSize)
	}
}

// Allow checks if a request is allowed immediately.
// Returns true if the request can proceed, false if it should be rejected.
func (r *rateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check Retry-After block
	if r.config.RespectRetryAfter && !r.retryAfterUntil.IsZero() {
		if r.now().Before(r.retryAfterUntil) {
			return false
		}
		r.retryAfterUntil = time.Time{}
	}

	r.refill()

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Wait blocks until a request is allowed or the context is cancelled.
// Returns an error if the context is cancelled or the rate limit cannot be satisfied.
func (r *rateLimiter) Wait(ctx context.Context) error {
	for {
		r.mu.Lock()

		// Check Retry-After block
		if r.config.RespectRetryAfter && !r.retryAfterUntil.IsZero() {
			waitUntil := r.retryAfterUntil
			if r.now().Before(waitUntil) {
				r.mu.Unlock()
				// Wait until Retry-After expires
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(waitUntil.Sub(r.now())):
					continue
				}
			}
			r.retryAfterUntil = time.Time{}
		}

		r.refill()

		if r.tokens >= 1 {
			r.tokens--
			r.mu.Unlock()
			return nil
		}

		// Calculate wait time for one token
		tokensNeeded := 1 - r.tokens
		waitDuration := time.Duration(tokensNeeded/r.config.RequestsPerSecond*float64(time.Second)) + time.Millisecond

		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			// Loop and try again
		}
	}
}

// Reserve reserves a token and returns the duration to wait before using it.
// Returns 0 if a token is immediately available.
// Returns a negative duration if the reservation cannot be satisfied within
// a reasonable time (more than 1 second).
func (r *rateLimiter) Reserve() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check Retry-After block
	if r.config.RespectRetryAfter && !r.retryAfterUntil.IsZero() {
		if r.now().Before(r.retryAfterUntil) {
			return -1 // Cannot reserve during Retry-After
		}
		r.retryAfterUntil = time.Time{}
	}

	r.refill()

	if r.tokens >= 1 {
		r.tokens--
		return 0
	}

	// Calculate wait time for one token
	tokensNeeded := 1 - r.tokens
	waitDuration := time.Duration(tokensNeeded / r.config.RequestsPerSecond * float64(time.Second))

	// Don't allow reservations too far in the future
	if waitDuration > time.Second {
		return -1
	}

	// Reserve the token (go negative)
	r.tokens--
	return waitDuration
}

// SetRetryAfter sets a block until the given time due to a 429 response.
// This is called when the server returns a Retry-After header.
func (r *rateLimiter) SetRetryAfter(until time.Time) {
	if !r.config.RespectRetryAfter {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Only update if the new time is later than the current block
	if until.After(r.retryAfterUntil) {
		r.retryAfterUntil = until
	}
}

// SetRetryAfterDuration sets a block for the given duration.
// Convenience method for SetRetryAfter.
func (r *rateLimiter) SetRetryAfterDuration(d time.Duration) {
	r.SetRetryAfter(r.now().Add(d))
}

// Tokens returns the current number of available tokens.
// Useful for debugging and metrics.
func (r *rateLimiter) Tokens() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()
	return r.tokens
}

// RetryAfterRemaining returns the remaining duration of the Retry-After block,
// or 0 if there is no active block.
func (r *rateLimiter) RetryAfterRemaining() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.retryAfterUntil.IsZero() {
		return 0
	}

	remaining := r.retryAfterUntil.Sub(r.now())
	if remaining < 0 {
		return 0
	}
	return remaining
}
