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
