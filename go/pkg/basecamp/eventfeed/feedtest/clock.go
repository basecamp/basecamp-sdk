// Package feedtest provides deterministic fakes for testing eventfeed hosts
// and the connector itself. The same fakes are what the other SDKs'
// conformance harnesses mirror.
package feedtest

import (
	"sync"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// Clock is a deterministic virtual-time eventfeed.Clock. Time moves only
// through Advance, which honors SPEC.md §23's virtual-advance algorithm:
// advancing fires due timers in deadline order, re-evaluating after each
// fire; timers scheduled during the advance whose deadlines land inside the
// window also fire; ties break by creation order. AwaitTimer is the
// rendezvous for driving a loop running on another goroutine without
// wall-clock sleeps.
type Clock struct {
	// advancing keeps one advance atomic against another: c.mu is released
	// inside a window (that is what lets a recipient arm a follow-on timer),
	// so two goroutines advancing concurrently could otherwise interleave
	// their re-selections and rewind now to the earlier window's target.
	advancing sync.Mutex

	mu   sync.Mutex
	cond *sync.Cond
	now  time.Time
	seq  int
	live []*fakeTimer
}

var _ eventfeed.Clock = (*Clock)(nil)

// NewClock returns a virtual clock starting at a fixed instant.
func NewClock() *Clock {
	c := &Clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	c.cond = sync.NewCond(&c.mu)
	return c
}

// Now returns the current virtual time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// NewTimer arms a one-shot named timer due d from the current virtual time.
func (c *Clock) NewTimer(d time.Duration, name string) eventfeed.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{
		clock:    c,
		name:     name,
		delay:    d,
		deadline: c.now.Add(d),
		seq:      c.seq,
		c:        make(chan time.Time, 1),
		live:     true,
	}
	c.seq++
	c.live = append(c.live, t)
	c.cond.Broadcast()
	return t
}

// DueWithin returns the names of live timers due within d of the current
// virtual time — the INITIAL set an Advance(d) would fire, read from the
// clock's present state — in creation order. It is not a complete firing
// prediction: Advance re-evaluates after each fire, so a timer armed
// reentrantly by a firing's recipient can fire inside the same window without
// ever appearing here (TestClock_AdvanceFiresATimerArmedByAFiringsRecipient
// shows one). The asymmetry is what makes the empty answer exact and the
// non-empty answer a floor.
//
// Read under the same lock advance selects under and NewTimer arms under, so
// the answer is atomic with respect to both. That is what lets a caller turn a
// racy question into a decidable one: an EMPTY result means the advance fires
// nothing, and advance never unlocks unless it fires something, so no
// recipient can be woken by it and no timer it could arm can land inside its
// window. A non-empty result means the script is asking for a firing whose
// aftermath races the re-selection, which is the thing no cross-language
// fixture can mean the same way twice.
func (c *Clock) DueWithin(d time.Duration) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dueWithinLocked(c.now.Add(d))
}

// dueWithinLocked returns the names of live timers due at or before target,
// in creation order. The caller holds c.mu.
func (c *Clock) dueWithinLocked(target time.Time) []string {
	var names []string
	for _, t := range c.live {
		if !t.deadline.After(target) {
			names = append(names, t.name)
		}
	}
	return names
}

// AdvanceIfQuiet advances virtual time by d only if the window would fire
// nothing; otherwise it reports the INITIALLY due set — the same floor
// DueWithin reads, sufficient here because rejection needs only "non-empty",
// and an empty initial set means nothing fires at all — and leaves the clock
// untouched.
// It is DueWithin and Advance as ONE critical section, for the driver MUST
// in SPEC §23: an advance whose window would fire any timer is rejected.
// Deciding that with two separate lock acquisitions leaves a gap — a timer
// armed (or stopped) between the check and the movement changes what the
// accepted directive does, so an advance the guard accepted could fire.
// Under one hold of the clock's locks, an accepted advance provably fires
// nothing.
//
// What stays undecidable, stated honestly: whether a CONCURRENT arm lands
// before or after this critical section is still the arming goroutine's
// schedule — no clock operation can order another goroutine's lock
// acquisition. The invariant restored here is the decidable one: whichever
// side the arm lands, an ACCEPTED advance fired nothing, and an arm that
// lost the race is due at its own deadline, unfired and unharmed.
func (c *Clock) AdvanceIfQuiet(d time.Duration) ([]string, bool) {
	c.advancing.Lock()
	defer c.advancing.Unlock()
	c.mu.Lock()
	target := c.now.Add(d)
	if due := c.dueWithinLocked(target); len(due) > 0 {
		c.mu.Unlock()
		return due, false
	}
	c.now = target
	c.mu.Unlock()
	c.cond.Broadcast()
	return nil, true
}

// Outstanding returns the names of live (unfired, unstopped) timers, in
// creation order.
func (c *Clock) Outstanding() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, len(c.live))
	for i, t := range c.live {
		names[i] = t.name
	}
	return names
}

// Advance moves virtual time forward by d, firing due timers in deadline
// order (ties by creation order), re-evaluating the registry after each fire
// so a timer armed mid-advance with a deadline inside the window also fires.
// Firings are delivered on each timer's buffered channel; the fired timer is
// removed from the registry before delivery, and the clock is unlocked between
// firings so a recipient can arm its follow-on timer from the firing instant
// rather than from the window's end.
//
// Whether that follow-on timer lands before the re-selection that would fire
// it is up to the recipient's goroutine, so a caller that depends on a chained
// firing inside one window uses AdvanceSettling to say what to wait for.
func (c *Clock) Advance(d time.Duration) {
	c.advance(d, nil)
}

// AdvanceSettling advances exactly as Advance does, additionally running settle
// after each firing — with the clock unlocked, before the next due timer is
// selected. It is the rendezvous for the normative algorithm's reentrant
// clause when the recipient runs on another goroutine: settle is where the
// caller blocks until that goroutine has armed the timer the firing was
// supposed to arm (AwaitTimer is the usual body), so the re-selection below
// sees it and fires it if it is due inside the window. settle runs after every
// firing, including the last, and a body that can only be satisfied once
// should guard itself with a sync.Once.
func (c *Clock) AdvanceSettling(d time.Duration, settle func()) {
	c.advance(d, settle)
}

func (c *Clock) advance(d time.Duration, settle func()) {
	c.advancing.Lock()
	defer c.advancing.Unlock()

	c.mu.Lock()
	target := c.now.Add(d)
	for {
		next := c.earliestDue(target)
		if next == nil {
			break
		}
		if next.deadline.After(c.now) {
			c.now = next.deadline
		}
		c.removeLocked(next)
		// The channel is buffered and a timer is removed from the registry
		// before it is ever delivered to, so this send cannot block.
		next.c <- next.deadline

		// Unlocked across the firing's aftermath: a recipient woken by it
		// arms from now — the firing instant — and the next iteration's
		// selection can still find that timer inside the window.
		c.mu.Unlock()
		c.cond.Broadcast()
		if settle != nil {
			settle()
		}
		c.mu.Lock()
	}
	c.now = target
	c.mu.Unlock()
	c.cond.Broadcast()
}

// FireTimer fires the earliest-due outstanding timer with the given name
// WITHOUT advancing the clock (deadline ties break by creation order),
// returning the delay it was armed with — the tier-2 driver's fireTimer
// directive: jitter is asserted against a {min, max} envelope on the
// scheduled delay rather than through a cross-language RNG seam. The second
// return reports whether such a timer existed.
func (c *Clock) FireTimer(name string) (time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var next *fakeTimer
	for _, t := range c.live {
		if t.name != name {
			continue
		}
		if next == nil || t.deadline.Before(next.deadline) ||
			(t.deadline.Equal(next.deadline) && t.seq < next.seq) {
			next = t
		}
	}
	if next == nil {
		return 0, false
	}
	c.removeLocked(next)
	next.c <- c.now
	c.cond.Broadcast()
	return next.delay, true
}

// AwaitTimer blocks until a live timer with the given name is outstanding —
// the rendezvous for a loop running on another goroutine.
func (c *Clock) AwaitTimer(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for !c.hasLiveLocked(name) {
		c.cond.Wait()
	}
}

// earliestDue returns the live timer with the earliest deadline at or before
// target, breaking deadline ties by creation order. Callers hold c.mu.
func (c *Clock) earliestDue(target time.Time) *fakeTimer {
	var next *fakeTimer
	for _, t := range c.live {
		if t.deadline.After(target) {
			continue
		}
		if next == nil || t.deadline.Before(next.deadline) ||
			(t.deadline.Equal(next.deadline) && t.seq < next.seq) {
			next = t
		}
	}
	return next
}

func (c *Clock) hasLiveLocked(name string) bool {
	for _, t := range c.live {
		if t.name == name {
			return true
		}
	}
	return false
}

func (c *Clock) removeLocked(t *fakeTimer) {
	for i, live := range c.live {
		if live == t {
			c.live = append(c.live[:i], c.live[i+1:]...)
			t.live = false
			return
		}
	}
}

type fakeTimer struct {
	clock    *Clock
	name     string
	delay    time.Duration
	deadline time.Time
	seq      int
	c        chan time.Time
	live     bool
}

func (t *fakeTimer) C() <-chan time.Time {
	return t.c
}

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if !t.live {
		return false
	}
	t.clock.removeLocked(t)
	t.clock.cond.Broadcast()
	return true
}
