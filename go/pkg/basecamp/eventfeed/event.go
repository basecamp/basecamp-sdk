package eventfeed

import (
	"encoding/json"
	"time"
)

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
	// PerformedByID is the agent that carried out a delegated action on the
	// creator's behalf; nil for a direct action. The effective performer —
	// what the performers/exclude_performers filters match — is this id when
	// set, else CreatorID. Both lanes carry the key, null-valued when there is
	// no delegation.
	PerformedByID *int64 `json:"performed_by_id"`
	// RecordingID is the recording the event references.
	RecordingID int64 `json:"recording_id"`
	// Details is the type-specific detail object, verbatim, for the types
	// that publish one (`card.moved` names its columns; `boost.created` names
	// the boost and what it landed on) — nil for every other type. It is
	// retained as raw bytes rather than decoded: the keys are per-type and
	// server-owned, and a decoded map would round ids through float64 (§10's
	// 64-bit integer contract). Decode it against the type's documented
	// shape.
	Details json.RawMessage `json:"details,omitempty"`
	// ActorType is the push-lane transport field naming the actor's kind
	// (ActorTypeAgent / ActorTypePerson): push payloads carry it, poll rows
	// omit it — empty means absent, which is why it is a string rather than
	// a pointer (the empty string is outside the vocabulary).
	ActorType string `json:"actor_type,omitempty"`
	// VisibleToClients is presence-bearing: push payloads carry it, poll rows
	// omit it — absent is not false, never a defaulted boolean.
	VisibleToClients *bool `json:"visible_to_clients,omitempty"`
}
