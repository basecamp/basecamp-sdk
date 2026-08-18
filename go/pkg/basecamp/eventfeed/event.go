package eventfeed

import "time"

// Event is one feed row (SPEC.md §23 "Consumer Surface") — a wake-up signal:
// enough to route, not enough to act. Feed payloads are never current
// resource state; consumers refetch the referenced recording through
// canonical resource APIs before acting.
//
// There is no collision with basecamp.Event (the recording-events service):
// eventfeed.Event lives in its own namespace.
type Event struct {
	// ID is the feed-global event id (strict event-id order on the poll lane).
	ID int64 `json:"id"`
	// Kind is the event kind.
	Kind string `json:"kind"`
	// EventType is the cataloged event type (e.g. "message.created").
	EventType string `json:"event_type"`
	// Action is the action that produced the event.
	Action string `json:"action"`
	// CreatedAt is the event's creation time (ISO 8601 on the wire).
	CreatedAt time.Time `json:"created_at"`
	// BucketID is the bucket (project) the recording lives in.
	BucketID int64 `json:"bucket_id"`
	// CreatorID is the person who caused the event.
	CreatorID int64 `json:"creator_id"`
	// RecordingID is the recording the event references.
	RecordingID int64 `json:"recording_id"`
	// VisibleToClients is presence-bearing: push payloads carry it, poll rows
	// omit it — absent is not false, never a defaulted boolean.
	VisibleToClients *bool `json:"visible_to_clients,omitempty"`
}
