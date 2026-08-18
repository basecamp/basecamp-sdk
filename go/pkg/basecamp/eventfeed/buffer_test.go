package eventfeed

import "testing"

// TestLiveBufferAddClearsEvictedSlots pins the eviction half of the live
// buffer's memory ceiling. SPEC.md §23 publishes the connector's worst case
// as (pump depth + EVENT_FEED_LIVE_BUFFER_CAPACITY) × EVENT_FEED_MAX_FRAME_BYTES;
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
	// Captured before the eviction: base still spans the backing array the
	// live slice is resliced from, so base[0] IS the vacated slot.
	base := b.events

	dropped := b.add(Event{ID: 3, Kind: "message", EventType: "message.created"})
	if len(dropped) != 1 || dropped[0] != 1 {
		t.Fatalf("dropped = %v, want [1]", dropped)
	}
	if base[0] != (Event{}) {
		t.Errorf("evicted slot = %+v, want the zero Event — a bare reslice keeps its payload reachable", base[0])
	}
	// The eviction itself is unchanged.
	if got := len(b.events); got != 2 {
		t.Fatalf("occupancy = %d, want 2", got)
	}
	if b.events[0].ID != 2 || b.events[1].ID != 3 {
		t.Fatalf("buffer = %d,%d, want 2,3", b.events[0].ID, b.events[1].ID)
	}
}
