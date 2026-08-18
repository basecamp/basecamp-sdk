package eventfeed

import "container/list"

// DefaultDedupeCapacity is the default delivered-id LRU capacity
// (EVENT_FEED_DEDUPE_CAPACITY, 10,000 event ids).
const DefaultDedupeCapacity = 10_000

// dedupe is a bounded LRU of actually-delivered event ids — never position
// ordering (SPEC.md §23 "Dedupe"). Every delivery — poll page, drain, or
// streaming — checks it before delivering and records the delivered id, so
// poll-vs-push duplication is suppressed by id regardless of which lane
// delivered first. A buffered live event with an id at or below the current
// position is still delivered — it was never served by poll.
type dedupe struct {
	capacity int
	// order holds ids most-recently-seen first.
	order *list.List
	index map[int64]*list.Element
}

// newDedupe returns an LRU bounded at capacity ids. Capacity must be
// positive (validated at construction; there is no dedupe-disabled mode).
func newDedupe(capacity int) *dedupe {
	return &dedupe{
		capacity: capacity,
		order:    list.New(),
		index:    make(map[int64]*list.Element, capacity),
	}
}

// Seen tests and records id in one step: a hit refreshes the id's recency
// and returns true; a miss records it — evicting the least-recently-seen id
// past capacity — and returns false.
func (d *dedupe) Seen(id int64) bool {
	if el, ok := d.index[id]; ok {
		d.order.MoveToFront(el)
		return true
	}
	d.index[id] = d.order.PushFront(id)
	if d.order.Len() > d.capacity {
		back := d.order.Back()
		d.order.Remove(back)
		delete(d.index, back.Value.(int64))
	}
	return false
}

// Len reports the number of tracked ids.
func (d *dedupe) Len() int {
	return d.order.Len()
}
