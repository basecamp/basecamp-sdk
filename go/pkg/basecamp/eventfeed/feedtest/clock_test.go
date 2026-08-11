package feedtest

import (
	"testing"
	"time"
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
