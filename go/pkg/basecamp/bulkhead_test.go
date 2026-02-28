package basecamp

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBulkhead_AcquireAndRelease(t *testing.T) {
	bh := newBulkhead(&BulkheadConfig{MaxConcurrent: 2, MaxWait: 0})

	release, err := bh.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if bh.InUse() != 1 {
		t.Errorf("InUse = %d, want 1", bh.InUse())
	}
	if bh.Available() != 1 {
		t.Errorf("Available = %d, want 1", bh.Available())
	}

	release()
	if bh.InUse() != 0 {
		t.Errorf("InUse after release = %d, want 0", bh.InUse())
	}
}

func TestBulkhead_RejectionAtCapacity(t *testing.T) {
	bh := newBulkhead(&BulkheadConfig{MaxConcurrent: 1, MaxWait: 0})

	release, err := bh.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()

	_, err = bh.Acquire(context.Background())
	if err != ErrBulkheadFull {
		t.Errorf("second Acquire err = %v, want ErrBulkheadFull", err)
	}
}

func TestBulkhead_TimeoutWaiting(t *testing.T) {
	bh := newBulkhead(&BulkheadConfig{MaxConcurrent: 1, MaxWait: 50 * time.Millisecond})

	release, err := bh.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()

	start := time.Now()
	_, err = bh.Acquire(context.Background())
	elapsed := time.Since(start)

	if err != ErrBulkheadFull {
		t.Errorf("timed out Acquire err = %v, want ErrBulkheadFull", err)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("waited only %v, expected ~50ms", elapsed)
	}
}

func TestBulkhead_ContextCancellation(t *testing.T) {
	bh := newBulkhead(&BulkheadConfig{MaxConcurrent: 1, MaxWait: 5 * time.Second})

	release, err := bh.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = bh.Acquire(ctx)
	if err != context.Canceled {
		t.Errorf("cancelled Acquire err = %v, want context.Canceled", err)
	}
}

func TestBulkhead_TryAcquire(t *testing.T) {
	bh := newBulkhead(&BulkheadConfig{MaxConcurrent: 1, MaxWait: 0})

	release, ok := bh.TryAcquire()
	if !ok {
		t.Fatal("TryAcquire should succeed when available")
	}
	defer release()

	_, ok = bh.TryAcquire()
	if ok {
		t.Error("TryAcquire should fail at capacity")
	}
}

func TestBulkhead_ConcurrentGoroutinesCapped(t *testing.T) {
	const maxConc = 3
	bh := newBulkhead(&BulkheadConfig{MaxConcurrent: maxConc, MaxWait: 5 * time.Second})

	var peak atomic.Int32
	var current atomic.Int32
	var wg sync.WaitGroup

	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := bh.Acquire(context.Background())
			if err != nil {
				return
			}
			defer release()

			n := current.Add(1)
			// Record peak
			for {
				old := peak.Load()
				if n <= old || peak.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			current.Add(-1)
		}()
	}

	wg.Wait()

	if p := peak.Load(); p > maxConc {
		t.Errorf("peak concurrency = %d, want <= %d", p, maxConc)
	}
}

func TestBulkhead_RegistryReturnsSameInstance(t *testing.T) {
	reg := newBulkheadRegistry(nil)
	a := reg.get("scope")
	b := reg.get("scope")
	if a != b {
		t.Error("expected same bulkhead instance for same scope")
	}
}
