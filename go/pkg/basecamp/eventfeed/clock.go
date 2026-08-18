package eventfeed

import (
	"sync"
	"time"
)

// Clock is the injected time seam (SPEC.md §23 "Clock, Timers, and Virtual
// Time") — product seam, not a test hook: every delay the connector itself
// takes flows through it (delays inside a seam call are the generated
// operation's own machinery). It is the graduated successor of the
// device-flow WithDeviceClock/WithDeviceSleep precedent: monotonic now plus
// cancellable, NAMED, enumerable timers, so "no timer survives teardown" is
// an exact-set assertion rather than a test-only artifact. Timer names are
// §23's six kebab-case kinds: handshake-deadline, confirmation-deadline,
// backoff, staleness, repair-poll, poll-retry.
type Clock interface {
	// Now returns the current time. Monotonic discipline: readings are used
	// only for deltas, never persisted.
	Now() time.Time
	// NewTimer arms a one-shot named timer. Firing removes it from the
	// registry.
	NewTimer(d time.Duration, name string) Timer
	// Outstanding returns the names of live (unfired, unstopped) timers, in
	// creation order.
	Outstanding() []string
}

// Timer is a one-shot named timer.
type Timer interface {
	// C is the firing channel. A fired timer is no longer outstanding by the
	// time its firing is received.
	C() <-chan time.Time
	// Stop cancels the timer, removing it from the registry; it reports
	// whether the timer was still pending.
	Stop() bool
}

// SystemClock returns the real-time Clock: real timers behind the same
// enumerable registry the test clocks keep (feedtest.Clock is the
// deterministic counterpart).
func SystemClock() Clock {
	return &systemClock{}
}

type systemClock struct {
	mu   sync.Mutex
	live []*systemTimer
}

func (c *systemClock) Now() time.Time {
	return time.Now()
}

func (c *systemClock) NewTimer(d time.Duration, name string) Timer {
	st := &systemTimer{
		clock: c,
		name:  name,
		c:     make(chan time.Time, 1),
	}
	c.mu.Lock()
	c.live = append(c.live, st)
	c.mu.Unlock()
	st.timer = time.AfterFunc(d, func() {
		// Deregister BEFORE delivering so a receiver observing the firing
		// never sees the timer still outstanding.
		c.remove(st)
		st.c <- time.Now()
	})
	return st
}

func (c *systemClock) Outstanding() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, len(c.live))
	for i, st := range c.live {
		names[i] = st.name
	}
	return names
}

func (c *systemClock) remove(st *systemTimer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, live := range c.live {
		if live == st {
			c.live = append(c.live[:i], c.live[i+1:]...)
			return
		}
	}
}

type systemTimer struct {
	clock *systemClock
	timer *time.Timer
	name  string
	c     chan time.Time
}

func (st *systemTimer) C() <-chan time.Time {
	return st.c
}

func (st *systemTimer) Stop() bool {
	stopped := st.timer.Stop()
	if stopped {
		st.clock.remove(st)
	}
	return stopped
}
