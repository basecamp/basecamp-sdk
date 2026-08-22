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
		"recording_id": 4
	}`)
	var poll Event
	if err := json.Unmarshal(pollRow, &poll); err != nil {
		t.Fatalf("unmarshaling poll row: %v", err)
	}
	if poll.VisibleToClients != nil {
		t.Errorf("poll row VisibleToClients = %v, want nil (absent)", *poll.VisibleToClients)
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
}
