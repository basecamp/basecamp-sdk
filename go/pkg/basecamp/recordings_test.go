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

	// End-to-end projection proof: each Message recording carries its
	// content_attachments companion array through the wire (a non-nil pointer).
	if r1.ContentAttachments == nil || r2.ContentAttachments == nil {
		t.Fatalf("expected both recordings to carry ContentAttachments, got %v and %v",
			r1.ContentAttachments, r2.ContentAttachments)
	}
	if len(*r1.ContentAttachments) == 0 || len(*r2.ContentAttachments) == 0 {
		t.Errorf("expected both recordings' ContentAttachments to be populated, got %d and %d",
			len(*r1.ContentAttachments), len(*r2.ContentAttachments))
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

	// End-to-end projection proof: the generic recording projection carries the
	// recording's rich text companion array through the wire (a non-nil pointer).
	// This Message carries content_attachments (its content attribute) and no
	// description_attachments (a nil pointer, absent).
	if recording.ContentAttachments == nil {
		t.Fatal("expected non-nil ContentAttachments for the Message recording")
	}
	if len(*recording.ContentAttachments) == 0 {
		t.Fatal("expected non-empty ContentAttachments for the Message recording")
	}
	if recording.DescriptionAttachments != nil {
		t.Errorf("expected nil (absent) DescriptionAttachments, got %v", recording.DescriptionAttachments)
	}
	att := (*recording.ContentAttachments)[0]
	if att.ContentType != "image/png" || att.Width == nil || *att.Width != 1024 {
		t.Errorf("unexpected content attachment (float-spelled width should narrow to 1024): %+v", att)
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
		{RecordingTypeDoor, "Door"},
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

// TestRecording_UnmarshalDoor verifies that a type=Door recording (external
// link) decodes with the full door shape — the outside url, the service struct,
// the description, and the position — through the shared Recording projection.
func TestRecording_UnmarshalDoor(t *testing.T) {
	data := `[
		{
			"id": 1069480290,
			"status": "active",
			"visible_to_clients": false,
			"created_at": "2026-07-22T15:51:54.872Z",
			"updated_at": "2026-07-22T15:51:54.886Z",
			"title": "Design system",
			"inherits_status": true,
			"type": "Door",
			"url": "https://www.figma.com/file/abc123/Design-system",
			"app_url": "https://3.basecampapi.com/195539477/buckets/2085958504/dock/doors/1069480290",
			"position": 8,
			"bucket": {"id": 2085958504, "name": "The Leto Laptop", "type": "Project"},
			"creator": {"id": 1049715913, "name": "Victor Cooper"},
			"service": {
				"name": "Figma",
				"example_url": "https://www.figma.com/file/aGVsbG8gZmlnbWEgZmlsZQ",
				"code": "figma",
				"valid_patterns": ["(.*?\\.)?figma\\.com(\\/.*)?"],
				"supporting_text": "a file or project on Figma"
			},
			"description": "<div>Shared Figma workspace</div>"
		}
	]`

	var recordings []Recording
	if err := json.Unmarshal([]byte(data), &recordings); err != nil {
		t.Fatalf("failed to unmarshal door recording: %v", err)
	}
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(recordings))
	}
	d := recordings[0]
	if d.Type != "Door" {
		t.Errorf("expected type Door, got %q", d.Type)
	}
	if d.URL != "https://www.figma.com/file/abc123/Design-system" {
		t.Errorf("expected external url, got %q", d.URL)
	}
	if d.Position != 8 {
		t.Errorf("expected position 8, got %d", d.Position)
	}
	if d.Description != "<div>Shared Figma workspace</div>" {
		t.Errorf("unexpected description: %q", d.Description)
	}
	if d.Service == nil {
		t.Fatal("expected service struct to be non-nil for a Door")
	}
	if d.Service.Name != "Figma" || d.Service.Code != "figma" {
		t.Errorf("unexpected service name/code: %q/%q", d.Service.Name, d.Service.Code)
	}
	if len(d.Service.ValidPatterns) != 1 || d.Service.ValidPatterns[0] == "" {
		t.Errorf("expected 1 valid_pattern, got %v", d.Service.ValidPatterns)
	}
	if d.Service.SupportingText != "a file or project on Figma" {
		t.Errorf("unexpected supporting_text: %q", d.Service.SupportingText)
	}
}

// TestRecording_UnmarshalNonDoorOmitsDoorFields verifies a non-door recording
// leaves the door-specific fields empty (they are optional).
func TestRecording_UnmarshalNonDoorOmitsDoorFields(t *testing.T) {
	data := `{"id": 1, "type": "Message", "title": "Hi", "url": "https://x/1.json"}`
	var r Recording
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if r.Service != nil {
		t.Errorf("expected nil service for non-door, got %+v", r.Service)
	}
	if r.Description != "" || r.Position != 0 {
		t.Errorf("expected empty door fields, got description=%q position=%d", r.Description, r.Position)
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
