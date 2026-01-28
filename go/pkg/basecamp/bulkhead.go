package basecamp

import (
	"context"
	"sync"
	"time"
)

// BulkheadConfig configures concurrency limiting.
type BulkheadConfig struct {
	// MaxConcurrent is the maximum number of parallel requests.
	// Default: 10
	MaxConcurrent int

	// MaxWait is the maximum time to wait for a slot.
	// If zero, requests are rejected immediately when the bulkhead is full.
	// Default: 5s
	MaxWait time.Duration
}

// DefaultBulkheadConfig returns production-ready defaults.
func DefaultBulkheadConfig() *BulkheadConfig {
	return &BulkheadConfig{
		MaxConcurrent: 10,
		MaxWait:       5 * time.Second,
	}
}

// bulkhead implements the bulkhead (concurrency limiting) pattern.
// Uses a semaphore to limit concurrent requests.
// Thread-safe for concurrent access.
type bulkhead struct {
	config *BulkheadConfig
	sem    chan struct{}
}

// newBulkhead creates a new bulkhead with the given config.
func newBulkhead(config *BulkheadConfig) *bulkhead {
	if config == nil {
		config = DefaultBulkheadConfig()
	}
	// Apply defaults for zero values
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}
	if config.MaxWait < 0 {
		config.MaxWait = 5 * time.Second
	}

	return &bulkhead{
		config: config,
		sem:    make(chan struct{}, config.MaxConcurrent),
	}
}

// Acquire tries to acquire a slot in the bulkhead.
// Returns a release function that MUST be called when the request completes,
// or an error if the bulkhead is full and the timeout is exceeded.
//
// Usage:
//
//	release, err := bh.Acquire(ctx)
//	if err != nil {
//	    return err // Bulkhead full
//	}
//	defer release()
//	// ... perform request ...
func (b *bulkhead) Acquire(ctx context.Context) (release func(), err error) {
	// If no wait time, try immediately
	if b.config.MaxWait == 0 {
		select {
		case b.sem <- struct{}{}:
			return func() { <-b.sem }, nil
		default:
			return nil, ErrBulkheadFull
		}
	}

	// Create a timeout context if the parent context doesn't have a deadline
	// or if our MaxWait is shorter
	waitCtx := ctx
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > b.config.MaxWait {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, b.config.MaxWait)
		defer cancel()
	}

	select {
	case b.sem <- struct{}{}:
		return func() { <-b.sem }, nil
	case <-waitCtx.Done():
		// Check if it was the parent context or our timeout
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrBulkheadFull
	}
}

// TryAcquire tries to acquire a slot without waiting.
// Returns a release function and true if successful, nil and false otherwise.
func (b *bulkhead) TryAcquire() (release func(), ok bool) {
	select {
	case b.sem <- struct{}{}:
		return func() { <-b.sem }, true
	default:
		return nil, false
	}
}

// Available returns the number of available slots.
func (b *bulkhead) Available() int {
	return b.config.MaxConcurrent - len(b.sem)
}

// InUse returns the number of slots currently in use.
func (b *bulkhead) InUse() int {
	return len(b.sem)
}

// bulkheadRegistry manages per-scope bulkheads.
type bulkheadRegistry struct {
	config    *BulkheadConfig
	mu        sync.RWMutex
	bulkheads map[string]*bulkhead
}

// newBulkheadRegistry creates a new registry with the given config.
func newBulkheadRegistry(config *BulkheadConfig) *bulkheadRegistry {
	if config == nil {
		config = DefaultBulkheadConfig()
	}
	return &bulkheadRegistry{
		config:    config,
		bulkheads: make(map[string]*bulkhead),
	}
}

// get returns the bulkhead for the given scope, creating it if needed.
func (r *bulkheadRegistry) get(scope string) *bulkhead {
	// Fast path: check with read lock
	r.mu.RLock()
	bh, ok := r.bulkheads[scope]
	r.mu.RUnlock()
	if ok {
		return bh
	}

	// Slow path: create with write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if bh, ok = r.bulkheads[scope]; ok {
		return bh
	}

	bh = newBulkhead(r.config)
	r.bulkheads[scope] = bh
	return bh
}
