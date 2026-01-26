package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func eventsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "events")
}

func loadEventsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(eventsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestEvent_UnmarshalList(t *testing.T) {
	data := loadEventsFixture(t, "list.json")

	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Verify first event (created)
	e1 := events[0]
	if e1.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", e1.ID)
	}
	if e1.RecordingID != 1069479351 {
		t.Errorf("expected RecordingID 1069479351, got %d", e1.RecordingID)
	}
	if e1.Action != "created" {
		t.Errorf("expected action 'created', got %q", e1.Action)
	}
	if e1.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}

	// Verify creator
	if e1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if e1.Creator.ID != 1049715915 {
		t.Errorf("expected Creator.ID 1049715915, got %d", e1.Creator.ID)
	}
	if e1.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", e1.Creator.Name)
	}
	if e1.Creator.EmailAddress != "annie@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'annie@honchodesign.com', got %q", e1.Creator.EmailAddress)
	}
	if !e1.Creator.Employee {
		t.Error("expected Creator.Employee to be true")
	}

	// Verify creator company
	if e1.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil")
	}
	if e1.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", e1.Creator.Company.Name)
	}

	// Verify second event (assignment_changed with details)
	e2 := events[1]
	if e2.ID != 1069479401 {
		t.Errorf("expected ID 1069479401, got %d", e2.ID)
	}
	if e2.Action != "assignment_changed" {
		t.Errorf("expected action 'assignment_changed', got %q", e2.Action)
	}
	if e2.Details == nil {
		t.Fatal("expected Details to be non-nil for assignment_changed event")
	}
	if len(e2.Details.AddedPersonIDs) != 2 {
		t.Errorf("expected 2 added person IDs, got %d", len(e2.Details.AddedPersonIDs))
	}
	if e2.Details.AddedPersonIDs[0] != 1049715923 {
		t.Errorf("expected first added person ID 1049715923, got %d", e2.Details.AddedPersonIDs[0])
	}
	if len(e2.Details.RemovedPersonIDs) != 0 {
		t.Errorf("expected 0 removed person IDs, got %d", len(e2.Details.RemovedPersonIDs))
	}

	// Verify third event (completed with notified recipients)
	e3 := events[2]
	if e3.ID != 1069479402 {
		t.Errorf("expected ID 1069479402, got %d", e3.ID)
	}
	if e3.Action != "completed" {
		t.Errorf("expected action 'completed', got %q", e3.Action)
	}
	if e3.Details == nil {
		t.Fatal("expected Details to be non-nil for completed event")
	}
	if len(e3.Details.NotifiedRecipientIDs) != 1 {
		t.Errorf("expected 1 notified recipient ID, got %d", len(e3.Details.NotifiedRecipientIDs))
	}
	if e3.Details.NotifiedRecipientIDs[0] != 1049715915 {
		t.Errorf("expected notified recipient ID 1049715915, got %d", e3.Details.NotifiedRecipientIDs[0])
	}

	// Verify different creator on third event
	if e3.Creator == nil {
		t.Fatal("expected Creator to be non-nil for third event")
	}
	if e3.Creator.Name != "Andrew Wong" {
		t.Errorf("expected Creator.Name 'Andrew Wong', got %q", e3.Creator.Name)
	}
}
