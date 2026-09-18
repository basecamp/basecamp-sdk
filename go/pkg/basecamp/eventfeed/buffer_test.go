package eventfeed

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"testing"
)

// TestLiveBufferAddClearsEvictedSlots pins the eviction half of the live
// buffer's memory ceiling. SPEC.md §23 publishes the cable lane's worst case
// as (pump depth + 3 + EVENT_FEED_LIVE_BUFFER_CAPACITY) × EVENT_FEED_MAX_FRAME_BYTES
// retained, plus one frame's transient decode allocation;
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

// TestLiveBufferGrowsAfterAPartialWrapInOrder pins FIFO order across the one
// layout the ring's growth rule did not account for: a store that has not yet
// reached capacity, whose head a drain's shifts have moved off zero, and
// whose tail has since circled back below the head. Growing the backing then
// changes the modulus every existing index is taken against, and a wrapped
// entry that was logically last becomes physically stranded — with capacity
// 3, add 1 and 2, shift 1, add 3 (wraps to index 0), add 4 (appends index 2)
// read back as 2, 4, 3. fatalScan interleaves admissions with drain shifts,
// so this is the drain's own delivery order, not a contrived one.
func TestLiveBufferGrowsAfterAPartialWrapInOrder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		capacity int
		adds     int
		shifts   int
		more     int
	}{
		{"capacity 3: fill 2, shift 1, add 2", 3, 2, 1, 2},
		{"capacity 5: fill 3, shift 2, add 3", 5, 3, 2, 3},
		{"capacity 4: fill 1, shift 1, add 4", 4, 1, 1, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newLiveBuffer(tc.capacity, nil)
			var next int64 = 1
			var want []int64
			for range tc.adds {
				b.add(Event{ID: next})
				want = append(want, next)
				next++
			}
			for range tc.shifts {
				if ev, ok := b.shift(); !ok || ev.ID != want[0] {
					t.Fatalf("shift = %+v (%t), want id %d", ev, ok, want[0])
				}
				want = want[1:]
			}
			for range tc.more {
				if dropped := b.add(Event{ID: next}); len(dropped) != 0 {
					t.Fatalf("add %d dropped %v below capacity %d", next, dropped, tc.capacity)
				}
				want = append(want, next)
				next++
			}
			got := b.snapshot()
			ids := make([]int64, len(got))
			for i, ev := range got {
				ids[i] = ev.ID
			}
			if fmt.Sprint(ids) != fmt.Sprint(want) {
				t.Fatalf("logical order = %v, want %v", ids, want)
			}
			for i, id := range want {
				ev, ok := b.shift()
				if !ok || ev.ID != id {
					t.Fatalf("shift %d = %+v (%t), want id %d", i, ev, ok, id)
				}
			}
		})
	}
}

// TestLiveBufferPaysOnlyForTheEventsItAdmits is the MaxCapacity comment's
// live-buffer cost claim, written as an assertion. That comment carries the
// shared ceiling on the live buffer "because the two are one published
// contract, not because it allocates up front" — an account that holds only
// while the buffer's cost is a function of the events it admits and of
// nothing else. newLiveBuffer stores no backing at all, and add grows it by
// append while filling, so an idle consumer's buffer stays small no matter
// what capacity it was configured with. That is the property here, and it is
// the one a "simplification" to make([]Event, 0, capacity) silently removes:
// nothing else in the suite reads the store's physical size.
//
// Bytes rather than testing.AllocsPerRun's count, for the reason #921 paid
// for: make([]Event, 0, capacity) is ONE allocation whether the capacity is 1
// or a million, so a count is identical in exactly the case that must fail.
// TestSustainedOverflowRetainsTheBacking above counts allocations because its
// claim — that a full window of overflow reallocates the backing — is about
// how MANY allocations happen; this claim is about how BIG one is.
//
// The assertion is invariance in the capacity, not a byte total: the events
// are built before the measurement and carry no strings, so the only
// allocations inside it are the liveBuffer struct and append's growth of the
// store. Every capacity is compared against the smallest one that can still
// hold the admitted events, so an unrelated fixed-size allocation added to
// newLiveBuffer moves all the cells together and stays green, while a
// newLiveBuffer or an add that began sizing the store by the capacity does
// not. The capacity and the admitted count vary independently — a grid, not a
// diagonal — because a test that only ever set them equal could not tell
// "pays for its capacity" from "pays for what it admits", which is the whole
// claim. The rising floor across admitted counts is the other half: the cost
// does move, with the events, which is what makes the invariance above an
// observation rather than a measurement of nothing.
func TestLiveBufferPaysOnlyForTheEventsItAdmits(t *testing.T) {
	var previous uint64
	for _, admitted := range []int{0, 1, 16, 1000} {
		smallest := max(admitted, 1)
		floor := bytesAllocatedByAdmitting(t, smallest, admitted)
		if floor == 0 {
			t.Fatalf("admitting %d events into a capacity-%d buffer allocated no measurable bytes: the construction was optimized away, so this test can no longer observe it", admitted, smallest)
		}
		for _, capacity := range []int{smallest + 1, 10 * smallest, DefaultLiveBufferCapacity, MaxCapacity} {
			if capacity < admitted {
				continue
			}
			if got := bytesAllocatedByAdmitting(t, capacity, admitted); got != floor {
				t.Errorf("admitting %d events allocated %d bytes at capacity %d and %d bytes at capacity %d: the buffer now sizes its store by the capacity rather than growing to it", admitted, got, capacity, floor, smallest)
			}
		}
		if admitted > 0 && floor <= previous {
			t.Errorf("admitting %d events allocated %d bytes, not more than the %d bytes the previous, smaller admission did: the buffer no longer pays for the events it admits, so the invariance above is measuring nothing", admitted, floor, previous)
		}
		previous = floor
	}
}

// liveBufferSink keeps each measured buffer and its grown store reachable, so
// the allocations being measured cannot be optimized away.
var liveBufferSink *liveBuffer

// bytesAllocatedByAdmitting measures one buffer at one (capacity, admitted)
// pair. The events are built outside the measured region so their own cost
// cannot enter it, and none of them carries a string. TotalAlloc is a
// process-wide counter and this package's suite runs connectors on their own
// goroutines, so an unrelated allocation landing between the two reads can
// only ADD to a measurement: the lowest of several is the floor, and the
// floors are what the configurations are compared at.
func bytesAllocatedByAdmitting(t *testing.T, capacity, admitted int) uint64 {
	t.Helper()

	if capacity < admitted {
		t.Fatalf("capacity %d cannot hold %d events: this measurement admits without dropping, so a drop would charge it for the dropped-ids slices instead of the store", capacity, admitted)
	}
	events := make([]Event, admitted)
	for i := range events {
		events[i] = Event{ID: int64(i + 1)}
	}

	lowest := uint64(math.MaxUint64)
	for range 5 {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		b := newLiveBuffer(capacity, nil)
		for _, ev := range events {
			b.add(ev)
		}
		runtime.ReadMemStats(&after)
		liveBufferSink = b
		lowest = min(lowest, after.TotalAlloc-before.TotalAlloc)
	}
	return lowest
}

// TestLiveBufferCapacityIsDecoupledFromTheDedupeCapacity is
// DefaultLiveBufferCapacity's claim, written as an assertion. The two
// defaults are the same number, which is exactly why the claim needs one:
// nothing about the shipped pair distinguishes "chosen separately and found
// to agree" from "derived from the dedupe capacity", and every cell that sets
// them equal is blind to a wiring swap, a clamp of one by the other, or a
// newLoop that hands one capacity to both constructs.
//
// Behavior rather than bytes, deliberately, and the choice is not the
// arbitrary one it looks like. TestLiveBufferPaysOnlyForTheEventsItAdmits
// above establishes that the live buffer's cost does not move with its own
// capacity — so a byte grid over the two capacities would stay green under
// newLiveBuffer(cfg.dedupeCapacity), the single most likely way to break this
// claim. What is observable is which capacity bounds which population: the
// buffer drops at its own configured capacity, the LRU evicts at its own, and
// each does so while the other is set somewhere else entirely.
//
// The one thing this cannot reach is the constant's derivation. Writing
// DefaultLiveBufferCapacity = DefaultDedupeCapacity would make the two
// numbers formally coupled and leave every observation below unchanged,
// because untyped constants carry no evidence of where they came from. That
// half of the claim is a statement about intent and stays a comment.
func TestLiveBufferCapacityIsDecoupledFromTheDedupeCapacity(t *testing.T) {
	for _, tc := range []struct {
		name               string
		dedupe, liveBuffer int
	}{
		{"the shipped defaults, equal and therefore the one cell that discriminates nothing", DefaultDedupeCapacity, DefaultLiveBufferCapacity},
		{"live buffer far below the dedupe capacity", 64, 3},
		{"live buffer far above the dedupe capacity", 3, 64},
		{"dedupe capacity at its minimum", 1, 8},
		{"live buffer capacity at its minimum", 8, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newLoop(context.Background(), &config{
				dedupeCapacity:     tc.dedupe,
				liveBufferCapacity: tc.liveBuffer,
			}, testHooks{})

			for i := range tc.liveBuffer {
				if dropped := l.buffer.add(Event{ID: int64(i + 1)}); len(dropped) != 0 {
					t.Fatalf("admitting event %d of %d dropped %v: the live buffer is bounded somewhere other than its own capacity of %d, with the dedupe capacity at %d", i+1, tc.liveBuffer, dropped, tc.liveBuffer, tc.dedupe)
				}
			}
			dropped := l.buffer.add(Event{ID: int64(tc.liveBuffer + 1)})
			if len(dropped) != 1 || dropped[0] != 1 {
				t.Fatalf("the admit past the live buffer capacity of %d dropped %v, want the oldest event alone ([1]), with the dedupe capacity at %d", tc.liveBuffer, dropped, tc.dedupe)
			}
			if l.buffer.size != tc.liveBuffer {
				t.Errorf("buffer occupancy = %d after %d admits, want its own capacity %d (dedupe capacity %d)", l.buffer.size, tc.liveBuffer+1, tc.liveBuffer, tc.dedupe)
			}

			for id := int64(1); id <= int64(tc.dedupe); id++ {
				if l.dedupe.Seen(id) {
					t.Fatalf("Seen(%d) = true while filling a dedupe capacity of %d: an id was evicted early, so the LRU is bounded somewhere other than its own capacity, with the live buffer capacity at %d", id, tc.dedupe, tc.liveBuffer)
				}
			}
			if l.dedupe.Len() != tc.dedupe {
				t.Fatalf("dedupe Len() = %d after recording %d ids, want its own capacity %d (live buffer capacity %d)", l.dedupe.Len(), tc.dedupe, tc.dedupe, tc.liveBuffer)
			}
			if l.dedupe.Seen(int64(tc.dedupe + 1)) {
				t.Fatalf("Seen(%d) = true, want false: the id past the dedupe capacity was never recorded", tc.dedupe+1)
			}
			if l.dedupe.Len() != tc.dedupe {
				t.Errorf("dedupe Len() = %d after recording %d ids, want its own capacity %d (live buffer capacity %d)", l.dedupe.Len(), tc.dedupe+1, tc.dedupe, tc.liveBuffer)
			}
			// Probed last: Seen both tests and records, so a miss here
			// re-records the evicted id.
			if l.dedupe.Seen(1) {
				t.Errorf("Seen(1) = true after %d ids passed a dedupe capacity of %d, want false: the oldest delivery was not evicted", tc.dedupe+1, tc.dedupe)
			}
		})
	}
}
