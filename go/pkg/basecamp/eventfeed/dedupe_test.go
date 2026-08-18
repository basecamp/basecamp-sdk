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

// TestDedupe_HitDoesNotRefreshRecency pins the contract this test previously
// asserted the negation of. SPEC.md §23 "Dedupe": the LRU holds
// "actually-delivered event ids", and "every delivery — poll page, drain, or
// streaming — checks the LRU before delivering and records the delivered id."
// A hit is the SUPPRESSED case: no delivery happened, so nothing about the
// delivery ordering changed.
//
// The old spelling was not a harmless difference. Refreshing on a hit pins an
// id that is being suppressed repeatedly — §23 says to expect exactly that,
// continuously, in both directions — at the front of the LRU, which evicts ids
// delivered once and never seen again, and those become eligible for the
// re-delivery the LRU exists to prevent.
func TestDedupe_HitDoesNotRefreshRecency(t *testing.T) {
	d := newDedupe(2)
	d.Seen(1)
	d.Seen(2)
	// A hit on 1 is a suppression, not a delivery: 1 stays the
	// least-recently-DELIVERED entry.
	if !d.Seen(1) {
		t.Fatal("Seen(1) = false, want true")
	}
	// ...so recording 3 evicts 1, not 2.
	d.Seen(3)
	// Order matters: Seen both tests AND records, so a probe that misses
	// re-records the id and evicts something. Check the survivor first — its
	// probe is a hit and changes nothing — then the evicted one.
	if !d.Seen(2) {
		t.Error("Seen(2) = false, want true (still within the last 2 deliveries)")
	}
	if d.Seen(1) {
		t.Error("Seen(1) = true, want false (evicted: a hit is not a delivery)")
	}
}
