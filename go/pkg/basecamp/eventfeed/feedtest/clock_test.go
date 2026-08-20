package feedtest

import (
	"sync"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

func TestClock_AdvanceFiresDueTimersInDeadlineOrder(t *testing.T) {
	c := NewClock()
	base := c.Now()
	slow := c.NewTimer(10*time.Millisecond, "repair-poll")
	fast := c.NewTimer(5*time.Millisecond, "staleness")

	c.Advance(20 * time.Millisecond)

	if got := <-fast.C(); !got.Equal(base.Add(5 * time.Millisecond)) {
		t.Errorf("fast fired at %v, want %v", got, base.Add(5*time.Millisecond))
	}
	if got := <-slow.C(); !got.Equal(base.Add(10 * time.Millisecond)) {
		t.Errorf("slow fired at %v, want %v", got, base.Add(10*time.Millisecond))
	}
	if got := c.Now(); !got.Equal(base.Add(20 * time.Millisecond)) {
		t.Errorf("Now() = %v, want %v", got, base.Add(20*time.Millisecond))
	}
	if got := c.Outstanding(); len(got) != 0 {
		t.Errorf("Outstanding() = %v, want empty", got)
	}
}

func TestClock_AdvanceLeavesUndueTimersOutstanding(t *testing.T) {
	c := NewClock()
	due := c.NewTimer(5*time.Millisecond, "backoff")
	undue := c.NewTimer(30*time.Millisecond, "repair-poll")

	c.Advance(10 * time.Millisecond)

	select {
	case <-due.C():
	default:
		t.Error("due timer did not fire")
	}
	select {
	case <-undue.C():
		t.Error("undue timer fired inside a 10ms advance")
	default:
	}
	if got := c.Outstanding(); len(got) != 1 || got[0] != "repair-poll" {
		t.Errorf("Outstanding() = %v, want [repair-poll]", got)
	}
}

func TestClock_EqualDeadlinesTieBreakByCreationOrder(t *testing.T) {
	c := NewClock()
	first := c.NewTimer(5*time.Millisecond, "handshake-deadline")
	second := c.NewTimer(5*time.Millisecond, "confirmation-deadline")

	// earliestDue must prefer the earlier-created timer on an exact
	// deadline tie (the virtual-advance algorithm's tie-break).
	target := c.Now().Add(5 * time.Millisecond)
	if got := c.earliestDue(target); got == nil || got.name != "handshake-deadline" {
		t.Fatalf("earliestDue picked %+v, want the first-created timer", got)
	}

	c.Advance(5 * time.Millisecond)
	select {
	case <-first.C():
	default:
		t.Error("first timer did not fire")
	}
	select {
	case <-second.C():
	default:
		t.Error("second timer did not fire")
	}
}

// The virtual-advance algorithm's reentrancy clause (conformance/event-feed
// README, normative): a timer armed during an advance whose deadline lands
// inside the window also fires — including one armed by the recipient of an
// earlier firing in that same window, which is the shape the state machine
// produces (a staleness firing tears the socket down and arms backoff). The
// recipient runs on another goroutine here, so AdvanceSettling supplies the
// rendezvous; without it the advance would end before the arming landed and
// the follow-on timer would be anchored at the window's end instead.
func TestClock_AdvanceFiresATimerArmedByAFiringsRecipient(t *testing.T) {
	c := NewClock()
	base := c.Now()
	first := c.NewTimer(5*time.Millisecond, "staleness")

	armed := make(chan eventfeed.Timer, 1)
	go func() {
		<-first.C()
		armed <- c.NewTimer(3*time.Millisecond, "backoff")
	}()

	// Only the first firing has a successor to wait for; the chained firing
	// arms nothing, so the wait must not be demanded again.
	var once sync.Once
	c.AdvanceSettling(20*time.Millisecond, func() {
		once.Do(func() { c.AwaitTimer("backoff") })
	})

	chained := <-armed
	select {
	case got := <-chained.C():
		if want := base.Add(8 * time.Millisecond); !got.Equal(want) {
			t.Errorf("the chained timer fired at %v, want %v — 3ms after the firing that armed it, "+
				"not 3ms after the window's end", got, want)
		}
	default:
		t.Errorf("the chained timer did not fire inside the 20ms window: outstanding %v", c.Outstanding())
	}
	if got := c.Now(); !got.Equal(base.Add(20 * time.Millisecond)) {
		t.Errorf("Now() = %v, want %v", got, base.Add(20*time.Millisecond))
	}
	if got := c.Outstanding(); len(got) != 0 {
		t.Errorf("Outstanding() = %v, want empty", got)
	}
}

func TestClock_StopRemovesTimerAndSuppressesFiring(t *testing.T) {
	c := NewClock()
	timer := c.NewTimer(5*time.Millisecond, "staleness")
	if !timer.Stop() {
		t.Fatal("Stop() on a pending timer = false, want true")
	}
	if timer.Stop() {
		t.Error("second Stop() = true, want false")
	}
	if got := c.Outstanding(); len(got) != 0 {
		t.Fatalf("Outstanding() after Stop = %v, want empty", got)
	}
	c.Advance(10 * time.Millisecond)
	select {
	case <-timer.C():
		t.Error("stopped timer fired")
	default:
	}
}

func TestClock_FireTimerFiresNamedTimerWithoutAdvancing(t *testing.T) {
	c := NewClock()
	base := c.Now()
	other := c.NewTimer(5*time.Millisecond, "staleness")
	later := c.NewTimer(20*time.Millisecond, "backoff")
	earlier := c.NewTimer(10*time.Millisecond, "backoff")

	d, ok := c.FireTimer("backoff")
	if !ok {
		t.Fatal("FireTimer(backoff) = false, want a firing")
	}
	if d != 10*time.Millisecond {
		t.Errorf("FireTimer returned scheduled delay %v, want 10ms (the earliest-due backoff)", d)
	}
	select {
	case <-earlier.C():
	default:
		t.Error("the earliest-due backoff timer did not fire")
	}
	select {
	case <-later.C():
		t.Error("the later backoff timer fired")
	default:
	}
	select {
	case <-other.C():
		t.Error("an unrelated timer fired")
	default:
	}
	if got := c.Now(); !got.Equal(base) {
		t.Errorf("Now() = %v, want %v — FireTimer must not advance the clock", got, base)
	}
	if got := c.Outstanding(); len(got) != 2 {
		t.Errorf("Outstanding() = %v, want the two unfired timers", got)
	}
}

func TestClock_FireTimerReportsMissingTimer(t *testing.T) {
	c := NewClock()
	c.NewTimer(time.Second, "staleness")
	if _, ok := c.FireTimer("backoff"); ok {
		t.Error("FireTimer(backoff) = true with no backoff timer outstanding")
	}
}

func TestClock_AwaitTimerRendezvousesWithAnotherGoroutine(t *testing.T) {
	c := NewClock()
	armed := make(chan struct{})
	go func() {
		c.AwaitTimer("backoff")
		close(armed)
	}()

	// Arming an unrelated timer must not release the wait.
	c.NewTimer(time.Second, "staleness")
	select {
	case <-armed:
		t.Fatal("AwaitTimer(backoff) returned before a backoff timer was armed")
	case <-time.After(10 * time.Millisecond):
	}

	c.NewTimer(time.Second, "backoff")
	select {
	case <-armed:
	case <-time.After(time.Second):
		t.Fatal("AwaitTimer(backoff) did not observe the armed timer")
	}
}
