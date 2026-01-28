package basecamp

import (
	"sync"
	"time"
)

// Circuit breaker states
const (
	stateClosed   = iota // Normal operation, requests allowed
	stateOpen            // Failing, requests rejected
	stateHalfOpen        // Testing, limited requests allowed
)

// CircuitBreakerConfig configures the circuit breaker.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before the circuit opens.
	// Default: 5
	FailureThreshold int

	// SuccessThreshold is the number of successes to close from half-open.
	// Default: 2
	SuccessThreshold int

	// OpenTimeout is the time before transitioning from open to half-open.
	// Default: 30s
	OpenTimeout time.Duration

	// FailureRateThreshold is the percentage failure rate to trigger opening.
	// Only evaluated when SlidingWindowSize requests have been made.
	// Default: 50 (meaning 50%)
	FailureRateThreshold float64

	// SlidingWindowSize is the number of requests to consider for rate calculation.
	// Default: 10
	SlidingWindowSize int

	// Now is a function that returns the current time. Used for testing.
	// If nil, time.Now is used.
	Now func() time.Time
}

// DefaultCircuitBreakerConfig returns production-ready defaults.
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold:     5,
		SuccessThreshold:     2,
		OpenTimeout:          30 * time.Second,
		FailureRateThreshold: 50,
		SlidingWindowSize:    10,
	}
}

// circuitBreaker implements the circuit breaker pattern.
// Thread-safe for concurrent access.
type circuitBreaker struct {
	config *CircuitBreakerConfig

	mu              sync.Mutex
	state           int
	failures        int
	successes       int
	lastFailureTime time.Time

	// Sliding window for failure rate calculation
	window       []bool // true = success, false = failure
	windowIndex  int
	windowFilled bool
}

// newCircuitBreaker creates a new circuit breaker with the given config.
func newCircuitBreaker(config *CircuitBreakerConfig) *circuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	// Apply defaults for zero values
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 2
	}
	if config.OpenTimeout <= 0 {
		config.OpenTimeout = 30 * time.Second
	}
	if config.FailureRateThreshold <= 0 {
		config.FailureRateThreshold = 50
	}
	if config.SlidingWindowSize <= 0 {
		config.SlidingWindowSize = 10
	}

	return &circuitBreaker{
		config: config,
		state:  stateClosed,
		window: make([]bool, config.SlidingWindowSize),
	}
}

// now returns the current time using the config's Now function or time.Now.
func (cb *circuitBreaker) now() time.Time {
	if cb.config.Now != nil {
		return cb.config.Now()
	}
	return time.Now()
}

// Allow checks if a request should be allowed.
// Returns true if the request can proceed, false if it should be rejected.
func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		return true

	case stateOpen:
		// Check if we should transition to half-open
		if cb.now().Sub(cb.lastFailureTime) >= cb.config.OpenTimeout {
			cb.state = stateHalfOpen
			cb.successes = 0
			return true
		}
		return false

	case stateHalfOpen:
		// Allow limited requests in half-open state
		return true

	default:
		return true
	}
}

// RecordSuccess records a successful request.
func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Record in sliding window
	cb.recordInWindow(true)

	switch cb.state {
	case stateHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.state = stateClosed
			cb.failures = 0
			cb.successes = 0
		}
	case stateClosed:
		// Reset consecutive failure count on success
		cb.failures = 0
	}
}

// RecordFailure records a failed request.
func (cb *circuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = cb.now()

	// Record in sliding window
	cb.recordInWindow(false)

	switch cb.state {
	case stateClosed:
		cb.failures++
		// Check both consecutive failures and failure rate
		if cb.failures >= cb.config.FailureThreshold || cb.checkFailureRate() {
			cb.state = stateOpen
		}

	case stateHalfOpen:
		// Any failure in half-open state opens the circuit
		cb.state = stateOpen
		cb.successes = 0
	}
}

// recordInWindow adds a result to the sliding window.
func (cb *circuitBreaker) recordInWindow(success bool) {
	cb.window[cb.windowIndex] = success
	cb.windowIndex = (cb.windowIndex + 1) % len(cb.window)
	if cb.windowIndex == 0 {
		cb.windowFilled = true
	}
}

// checkFailureRate checks if the failure rate exceeds the threshold.
// Only evaluates when the window is filled.
func (cb *circuitBreaker) checkFailureRate() bool {
	if !cb.windowFilled {
		return false
	}

	failures := 0
	for _, success := range cb.window {
		if !success {
			failures++
		}
	}

	rate := float64(failures) / float64(len(cb.window)) * 100
	return rate >= cb.config.FailureRateThreshold
}

// State returns the current circuit breaker state as a string.
// Useful for debugging and metrics.
func (cb *circuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		return "closed"
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// circuitBreakerRegistry manages per-scope circuit breakers.
type circuitBreakerRegistry struct {
	config   *CircuitBreakerConfig
	mu       sync.RWMutex
	breakers map[string]*circuitBreaker
}

// newCircuitBreakerRegistry creates a new registry with the given config.
func newCircuitBreakerRegistry(config *CircuitBreakerConfig) *circuitBreakerRegistry {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	return &circuitBreakerRegistry{
		config:   config,
		breakers: make(map[string]*circuitBreaker),
	}
}

// get returns the circuit breaker for the given scope, creating it if needed.
func (r *circuitBreakerRegistry) get(scope string) *circuitBreaker {
	// Fast path: check with read lock
	r.mu.RLock()
	cb, ok := r.breakers[scope]
	r.mu.RUnlock()
	if ok {
		return cb
	}

	// Slow path: create with write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, ok = r.breakers[scope]; ok {
		return cb
	}

	cb = newCircuitBreaker(r.config)
	r.breakers[scope] = cb
	return cb
}
