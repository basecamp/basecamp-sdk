package eventfeed

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEvent_VisibleToClientsIsPresenceBearing(t *testing.T) {
	// Push payloads carry visible_to_clients; poll rows omit it. Absent must
	// stay distinguishable from an explicit false — never a defaulted
	// boolean.
	pushRow := []byte(`{
		"id": 9007199254740993,
		"kind": "chat_line_created",
		"event_type": "chat.line.created",
		"action": "created",
		"created_at": "2026-08-01T12:34:56Z",
		"bucket_id": 2,
		"creator_id": 3,
		"performed_by_id": null,
		"actor_type": "person",
		"recording_id": 4,
		"visible_to_clients": false
	}`)
	var push Event
	if err := json.Unmarshal(pushRow, &push); err != nil {
		t.Fatalf("unmarshaling push row: %v", err)
	}
	if push.VisibleToClients == nil {
		t.Fatal("push row VisibleToClients = nil, want present false")
	}
	if *push.VisibleToClients {
		t.Error("push row *VisibleToClients = true, want false")
	}
	// A 64-bit id above 2^53 must survive intact (SPEC §10).
	if push.ID != 9007199254740993 {
		t.Errorf("ID = %d, want 9007199254740993", push.ID)
	}
	if want := time.Date(2026, 8, 1, 12, 34, 56, 0, time.UTC); !push.CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want %v", push.CreatedAt, want)
	}

	pollRow := []byte(`{
		"id": 1,
		"kind": "todo_completed",
		"event_type": "todo.completed",
		"action": "completed",
		"created_at": "2026-08-01T12:34:56Z",
		"bucket_id": 2,
		"creator_id": 3,
		"performed_by_id": 9007199254740992,
		"recording_id": 4,
		"details": {"boost_id": 9007199254740995, "boosted_event_id": null, "boosted_event_type": null}
	}`)
	var poll Event
	if err := json.Unmarshal(pollRow, &poll); err != nil {
		t.Fatalf("unmarshaling poll row: %v", err)
	}
	if poll.VisibleToClients != nil {
		t.Errorf("poll row VisibleToClients = %v, want nil (absent)", *poll.VisibleToClients)
	}
	if poll.ActorType != "" {
		t.Errorf("poll row ActorType = %q, want absent", poll.ActorType)
	}
	if poll.PerformedByID == nil || *poll.PerformedByID != 9007199254740992 {
		t.Errorf("poll row PerformedByID = %v, want 9007199254740992", poll.PerformedByID)
	}
	if push.PerformedByID != nil {
		t.Errorf("push row PerformedByID = %d, want nil (JSON null)", *push.PerformedByID)
	}
	// Details is retained verbatim, so a 64-bit id inside it survives a
	// consumer's own decode intact (§10) — a float64 map would have lost it.
	var details struct {
		BoostID int64 `json:"boost_id"`
	}
	if err := json.Unmarshal(poll.Details, &details); err != nil || details.BoostID != 9007199254740995 {
		t.Errorf("poll row Details = %s (%v), want boost_id 9007199254740995", poll.Details, err)
	}
	if push.Details != nil {
		t.Errorf("push row Details = %s, want nil (absent)", push.Details)
	}

	// The asymmetry round-trips: an absent field stays absent on re-encode.
	encoded, err := json.Marshal(poll)
	if err != nil {
		t.Fatalf("marshaling poll row: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("re-parsing encoded row: %v", err)
	}
	if _, present := fields["visible_to_clients"]; present {
		t.Error("re-encoded poll row carries visible_to_clients, want omitted")
	}
	if _, present := fields["actor_type"]; present {
		t.Error("re-encoded poll row carries actor_type, want omitted")
	}
	// performed_by_id is present on every row, null when there is no
	// delegation — the wire shape, not an omission.
	if raw, present := fields["performed_by_id"]; !present || string(raw) != "9007199254740992" {
		t.Errorf("re-encoded poll row performed_by_id = %s (present=%v), want 9007199254740992", raw, present)
	}
	encodedPush, err := json.Marshal(push)
	if err != nil {
		t.Fatalf("marshaling push row: %v", err)
	}
	var pushFields map[string]json.RawMessage
	if err := json.Unmarshal(encodedPush, &pushFields); err != nil {
		t.Fatalf("re-parsing encoded push row: %v", err)
	}
	if raw, present := pushFields["performed_by_id"]; !present || string(raw) != "null" {
		t.Errorf("re-encoded push row performed_by_id = %s (present=%v), want null", raw, present)
	}
	if _, present := pushFields["details"]; present {
		t.Error("re-encoded push row carries details, want omitted")
	}
}
