package eventfeed

import (
	"testing"
	"time"
)

func TestSystemClock_FiringRemovesTimerFromRegistry(t *testing.T) {
	c := SystemClock()
	timer := c.NewTimer(time.Millisecond, "backoff")
	<-timer.C()
	// Deregistration happens before delivery, so by the time the firing is
	// received the timer is no longer outstanding.
	if got := c.Outstanding(); len(got) != 0 {
		t.Errorf("Outstanding() after firing = %v, want empty", got)
	}
}

func TestSystemClock_StopRemovesTimerFromRegistry(t *testing.T) {
	c := SystemClock()
	staleness := c.NewTimer(time.Hour, "staleness")
	repair := c.NewTimer(time.Hour, "repair-poll")
	if got := c.Outstanding(); len(got) != 2 || got[0] != "staleness" || got[1] != "repair-poll" {
		t.Fatalf("Outstanding() = %v, want [staleness repair-poll] in creation order", got)
	}
	if !staleness.Stop() {
		t.Error("Stop() on a pending timer = false, want true")
	}
	if got := c.Outstanding(); len(got) != 1 || got[0] != "repair-poll" {
		t.Errorf("Outstanding() after Stop = %v, want [repair-poll]", got)
	}
	if staleness.Stop() {
		t.Error("second Stop() = true, want false")
	}
	if !repair.Stop() {
		t.Error("Stop() on the remaining timer = false, want true")
	}
	if got := c.Outstanding(); len(got) != 0 {
		t.Errorf("Outstanding() = %v, want empty", got)
	}
}

// TestSystemTimer_LostStopRaceStillDeregisters pins the race branch:
// time.Timer.Stop reports false the moment the firing callback is started,
// but that callback can still be parked on the registry lock ahead of its
// own remove. A Stop that returns without deregistering on that verdict
// leaves Outstanding() reporting a timer that already fired — the exact-set
// assertions after teardown read exactly that. The test holds the registry
// lock to park the callback deliberately; on fixed code Stop joins the wait
// and deregisters, on unfixed code it returns immediately with the entry
// still live.
func TestSystemTimer_LostStopRaceStillDeregisters(t *testing.T) {
	c := SystemClock().(*systemClock)
	tm := c.NewTimer(10*time.Millisecond, "backoff")
	c.mu.Lock()
	// Let the timer lapse while the lock pins its callback before remove.
	time.Sleep(100 * time.Millisecond)
	stopDone := make(chan bool, 1)
	go func() { stopDone <- tm.Stop() }()
	select {
	case stopped := <-stopDone:
		// Stop returned while the registry lock was held, so it cannot have
		// removed anything: if the entry is still live, Stop reported a
		// fired timer while leaving it outstanding.
		if stopped {
			c.mu.Unlock()
			t.Fatal("Stop() = true for a timer whose deadline lapsed 90ms ago")
		}
		if len(c.live) != 0 {
			c.mu.Unlock()
			t.Fatal("Stop returned without deregistering the fired-but-unremoved timer")
		}
		c.mu.Unlock()
	case <-time.After(50 * time.Millisecond):
		// Fixed behavior: Stop is parked on the registry lock to do its own
		// removal. Release it and let both it and the callback finish.
		c.mu.Unlock()
		if stopped := <-stopDone; stopped {
			t.Error("Stop() = true for a timer whose deadline lapsed")
		}
	}
	deadline := time.Now().Add(time.Second)
	for len(c.Outstanding()) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("Outstanding() = %v after a lost-race Stop, want empty", c.Outstanding())
		}
		time.Sleep(time.Millisecond)
	}
}
