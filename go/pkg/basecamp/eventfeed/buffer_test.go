package eventfeed

import "testing"

// TestLiveBufferAddClearsEvictedSlots pins the eviction half of the live
// buffer's memory ceiling. SPEC.md §23 publishes the cable lane's worst case
// as (pump depth + 6 + EVENT_FEED_LIVE_BUFFER_CAPACITY) × EVENT_FEED_MAX_FRAME_BYTES;
// a reslice alone removes the evicted event LOGICALLY while the slice that
// results still points into the same backing array, whose prefix keeps that
// event's strings reachable until a later reallocation. Under sustained
// overflow — the one condition eviction happens under — that is a second
// buffer's worth of payload held by events which no longer count toward
// occupancy. shift already zeroes for exactly this reason.
func TestLiveBufferAddClearsEvictedSlots(t *testing.T) {
	b := newLiveBuffer(2, nil)
	b.add(Event{ID: 1, Kind: "message", EventType: "message.created"})
	b.add(Event{ID: 2, Kind: "message", EventType: "message.created"})

	dropped := b.add(Event{ID: 3, Kind: "message", EventType: "message.created"})
	if len(dropped) != 1 || dropped[0] != 1 {
		t.Fatalf("dropped = %v, want [1]", dropped)
	}
	// Under the ring the evicted slot is zeroed and then reused by the very
	// insert that evicted it, so the hazard this test was born for — a
	// vacated slot pinning its payload in the backing — cannot arise on add
	// at all. The pin survives as a scan: NO physical slot may retain the
	// evicted event.
	for i, slot := range b.events {
		if slot.ID == 1 {
			t.Errorf("physical slot %d still holds the evicted event %+v", i, slot)
		}
	}
	if b.size != 2 {
		t.Fatalf("occupancy = %d, want 2", b.size)
	}
	// Logical order survives the wrap.
	ev, ok := b.shift()
	if !ok || ev.ID != 2 {
		t.Fatalf("first shift = %+v (%t), want id 2", ev, ok)
	}
	ev, ok = b.shift()
	if !ok || ev.ID != 3 {
		t.Fatalf("second shift = %+v (%t), want id 3", ev, ok)
	}
}

func TestSustainedOverflowRetainsTheBacking(t *testing.T) {
	// At capacity, the reslice-and-append shape burns one slot of slice
	// capacity per admit, so sustained overflow periodically reallocates and
	// copies the entire backing — a full-capacity copy plus a transient
	// SECOND buffer's worth of retained payload at every growth step, the
	// exact retention class add's zeroing exists to prevent. (Not, as first
	// reported, an O(capacity) copy on EVERY admit — the copies amortize —
	// but the spikes and the doubled retention are real.) A full capacity's
	// worth of sustained overflow must allocate nothing beyond the one
	// dropped-ids slice each admit legitimately makes: any backing
	// reallocation shows up as extra allocations here.
	const capacity = 10000
	b := newLiveBuffer(capacity, nil)
	for i := range capacity {
		b.add(Event{ID: int64(i)})
	}
	id := int64(capacity)
	allocs := testing.AllocsPerRun(1, func() {
		for range capacity {
			b.add(Event{ID: id})
			id++
		}
	})
	if allocs > capacity {
		t.Fatalf("a full window of sustained overflow made %.0f allocations, want at most %d (one dropped-ids slice per admit): the backing was reallocated instead of retained", allocs, capacity)
	}
}
