package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func recordingsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "recordings")
}

func loadRecordingsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(recordingsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestRecording_UnmarshalList(t *testing.T) {
	data := loadRecordingsFixture(t, "list.json")

	var recordings []Recording
	if err := json.Unmarshal(data, &recordings); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(recordings) != 2 {
		t.Errorf("expected 2 recordings, got %d", len(recordings))
	}

	// Verify first recording
	r1 := recordings[0]
	if r1.ID != 1069479351 {
		t.Errorf("expected ID 1069479351, got %d", r1.ID)
	}
	if r1.Status != "active" {
		t.Errorf("expected status 'active', got %q", r1.Status)
	}
	if r1.VisibleToClients {
		t.Error("expected VisibleToClients to be false")
	}
	if r1.Type != "Message" {
		t.Errorf("expected type 'Message', got %q", r1.Type)
	}
	if r1.Title != "We won Leto!" {
		t.Errorf("expected title 'We won Leto!', got %q", r1.Title)
	}
	if !r1.InheritsStatus {
		t.Error("expected InheritsStatus to be true")
	}
	if r1.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/messages/1069479351.json" {
		t.Errorf("unexpected URL: %q", r1.URL)
	}
	if r1.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/messages/1069479351" {
		t.Errorf("unexpected AppURL: %q", r1.AppURL)
	}

	// Verify parent
	if r1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if r1.Parent.ID != 1069479338 {
		t.Errorf("expected Parent.ID 1069479338, got %d", r1.Parent.ID)
	}
	if r1.Parent.Title != "Message Board" {
		t.Errorf("expected Parent.Title 'Message Board', got %q", r1.Parent.Title)
	}
	if r1.Parent.Type != "Message::Board" {
		t.Errorf("expected Parent.Type 'Message::Board', got %q", r1.Parent.Type)
	}

	// Verify bucket
	if r1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if r1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", r1.Bucket.ID)
	}
	if r1.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", r1.Bucket.Name)
	}
	if r1.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", r1.Bucket.Type)
	}

	// Verify creator
	if r1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if r1.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", r1.Creator.ID)
	}
	if r1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", r1.Creator.Name)
	}

	// Verify second recording
	r2 := recordings[1]
	if r2.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", r2.ID)
	}
	if !r2.VisibleToClients {
		t.Error("expected VisibleToClients to be true for second recording")
	}
	if r2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second recording")
	}
	if r2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", r2.Creator.Name)
	}
}

func TestRecording_UnmarshalGet(t *testing.T) {
	data := loadRecordingsFixture(t, "get.json")

	var recording Recording
	if err := json.Unmarshal(data, &recording); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if recording.ID != 1069479351 {
		t.Errorf("expected ID 1069479351, got %d", recording.ID)
	}
	if recording.Status != "active" {
		t.Errorf("expected status 'active', got %q", recording.Status)
	}
	if recording.VisibleToClients {
		t.Error("expected VisibleToClients to be false")
	}
	if recording.Type != "Message" {
		t.Errorf("expected type 'Message', got %q", recording.Type)
	}
	if recording.Title != "We won Leto!" {
		t.Errorf("expected title 'We won Leto!', got %q", recording.Title)
	}

	// Verify timestamps are parsed
	if recording.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if recording.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if recording.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if recording.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", recording.Creator.ID)
	}
	if recording.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", recording.Creator.EmailAddress)
	}
	if recording.Creator.Title != "Chief Strategist" {
		t.Errorf("expected Creator.Title 'Chief Strategist', got %q", recording.Creator.Title)
	}
	if !recording.Creator.Admin {
		t.Error("expected Creator.Admin to be true")
	}
	if !recording.Creator.Owner {
		t.Error("expected Creator.Owner to be true")
	}
}

func TestRecording_UnmarshalClientVisibility(t *testing.T) {
	data := loadRecordingsFixture(t, "client_visibility.json")

	var recording Recording
	if err := json.Unmarshal(data, &recording); err != nil {
		t.Fatalf("failed to unmarshal client_visibility.json: %v", err)
	}

	if recording.ID != 1069479351 {
		t.Errorf("expected ID 1069479351, got %d", recording.ID)
	}
	if !recording.VisibleToClients {
		t.Error("expected VisibleToClients to be true after update")
	}
}

func TestSetClientVisibilityRequest_Marshal(t *testing.T) {
	req := SetClientVisibilityRequest{
		VisibleToClients: true,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal SetClientVisibilityRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	visible, ok := data["visible_to_clients"].(bool)
	if !ok {
		t.Fatal("expected visible_to_clients to be a boolean")
	}
	if !visible {
		t.Error("expected visible_to_clients to be true")
	}

	// Test false case
	reqFalse := SetClientVisibilityRequest{
		VisibleToClients: false,
	}

	outFalse, err := json.Marshal(reqFalse)
	if err != nil {
		t.Fatalf("failed to marshal SetClientVisibilityRequest (false): %v", err)
	}

	var dataFalse map[string]any
	if err := json.Unmarshal(outFalse, &dataFalse); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	visibleFalse, ok := dataFalse["visible_to_clients"].(bool)
	if !ok {
		t.Fatal("expected visible_to_clients to be a boolean")
	}
	if visibleFalse {
		t.Error("expected visible_to_clients to be false")
	}
}

func TestRecordingType_Constants(t *testing.T) {
	// Verify recording type constants are correct
	tests := []struct {
		typ      RecordingType
		expected string
	}{
		{RecordingTypeComment, "Comment"},
		{RecordingTypeDocument, "Document"},
		{RecordingTypeKanbanCard, "Kanban::Card"},
		{RecordingTypeKanbanStep, "Kanban::Step"},
		{RecordingTypeMessage, "Message"},
		{RecordingTypeQuestionAnswer, "Question::Answer"},
		{RecordingTypeScheduleEntry, "Schedule::Entry"},
		{RecordingTypeTodo, "Todo"},
		{RecordingTypeTodolist, "Todolist"},
		{RecordingTypeUpload, "Upload"},
		{RecordingTypeVault, "Vault"},
	}

	for _, tc := range tests {
		if string(tc.typ) != tc.expected {
			t.Errorf("RecordingType %v: expected %q, got %q", tc.typ, tc.expected, string(tc.typ))
		}
	}
}

func TestRecordingsListOptions_BuildsQueryParams(t *testing.T) {
	// This is a structural test to ensure the options fields exist
	opts := RecordingsListOptions{
		Bucket:    []int64{1, 2, 3},
		Status:    "archived",
		Sort:      "updated_at",
		Direction: "asc",
	}

	if len(opts.Bucket) != 3 {
		t.Errorf("expected 3 bucket IDs, got %d", len(opts.Bucket))
	}
	if opts.Status != "archived" {
		t.Errorf("expected status 'archived', got %q", opts.Status)
	}
	if opts.Sort != "updated_at" {
		t.Errorf("expected sort 'updated_at', got %q", opts.Sort)
	}
	if opts.Direction != "asc" {
		t.Errorf("expected direction 'asc', got %q", opts.Direction)
	}
}
