package eventfeed

import "testing"

func TestDedupe_TestAndRecord(t *testing.T) {
	d := newDedupe(DefaultDedupeCapacity)
	if d.Seen(1) {
		t.Error("Seen(1) on a fresh LRU = true, want false")
	}
	if !d.Seen(1) {
		t.Error("Seen(1) after recording = false, want true")
	}
	if d.Seen(2) {
		t.Error("Seen(2) = true, want false")
	}
	if d.Len() != 2 {
		t.Errorf("Len() = %d, want 2", d.Len())
	}
}

func TestDedupe_EvictsLeastRecentlySeenPastCapacity(t *testing.T) {
	d := newDedupe(3)
	for id := int64(1); id <= 3; id++ {
		d.Seen(id)
	}
	// Recording a fourth id evicts the least-recently-seen (1).
	if d.Seen(4) {
		t.Error("Seen(4) = true, want false")
	}
	if d.Len() != 3 {
		t.Errorf("Len() = %d, want 3 (capacity-bounded)", d.Len())
	}
	if d.Seen(1) {
		t.Error("Seen(1) after eviction = true, want false")
	}
	// The probe above re-recorded 1, evicting 2 — 3 and 4 survive.
	if !d.Seen(3) {
		t.Error("Seen(3) = false, want true")
	}
	if !d.Seen(4) {
		t.Error("Seen(4) = false, want true")
	}
}

func TestDedupe_HitRefreshesRecency(t *testing.T) {
	d := newDedupe(2)
	d.Seen(1)
	d.Seen(2)
	// A hit on 1 makes 2 the least-recently-seen entry...
	if !d.Seen(1) {
		t.Fatal("Seen(1) = false, want true")
	}
	// ...so recording 3 evicts 2, not 1.
	d.Seen(3)
	if !d.Seen(1) {
		t.Error("Seen(1) = false, want true (refreshed, not evicted)")
	}
	if d.Seen(2) {
		t.Error("Seen(2) = true, want false (evicted)")
	}
}
