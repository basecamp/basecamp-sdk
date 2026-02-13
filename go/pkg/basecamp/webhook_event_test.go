package basecamp

import (
	"encoding/json"
	"testing"
)

func TestWebhookEvent_UnmarshalTodoCreated(t *testing.T) {
	data := loadWebhooksFixture(t, "event-todo-created.json")

	var event WebhookEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("failed to unmarshal event-todo-created.json: %v", err)
	}

	if event.ID != 9007199254741001 {
		t.Errorf("expected ID 9007199254741001, got %d", event.ID)
	}
	if event.Kind != "todo_created" {
		t.Errorf("expected kind 'todo_created', got %q", event.Kind)
	}
	if event.CreatedAt != "2022-11-22T16:00:00.000Z" {
		t.Errorf("expected created_at '2022-11-22T16:00:00.000Z', got %q", event.CreatedAt)
	}

	// Recording
	rec := event.Recording
	if rec.ID != 9007199254741200 {
		t.Errorf("expected recording ID 9007199254741200, got %d", rec.ID)
	}
	if rec.Status != "active" {
		t.Errorf("expected recording status 'active', got %q", rec.Status)
	}
	if rec.Title != "Ship the feature" {
		t.Errorf("expected recording title 'Ship the feature', got %q", rec.Title)
	}
	if rec.Type != "Todo" {
		t.Errorf("expected recording type 'Todo', got %q", rec.Type)
	}
	if !rec.InheritsStatus {
		t.Error("expected inherits_status to be true")
	}
	if rec.CommentsCount != 0 {
		t.Errorf("expected comments_count 0, got %d", rec.CommentsCount)
	}
	if rec.Content != "<div>Ship the feature by Friday</div>" {
		t.Errorf("unexpected content: %q", rec.Content)
	}

	// Parent
	if rec.Parent == nil {
		t.Fatal("expected non-nil parent")
	}
	if rec.Parent.ID != 9007199254741100 {
		t.Errorf("expected parent ID 9007199254741100, got %d", rec.Parent.ID)
	}
	if rec.Parent.Title != "Launch Checklist" {
		t.Errorf("expected parent title 'Launch Checklist', got %q", rec.Parent.Title)
	}
	if rec.Parent.Type != "Todolist" {
		t.Errorf("expected parent type 'Todolist', got %q", rec.Parent.Type)
	}

	// Bucket
	if rec.Bucket == nil {
		t.Fatal("expected non-nil bucket")
	}
	if rec.Bucket.ID != 2085958500 {
		t.Errorf("expected bucket ID 2085958500, got %d", rec.Bucket.ID)
	}
	if rec.Bucket.Name != "The Leto Experiment" {
		t.Errorf("expected bucket name 'The Leto Experiment', got %q", rec.Bucket.Name)
	}

	// Creator (top-level)
	if event.Creator.ID != 1049715914 {
		t.Errorf("expected creator ID 1049715914, got %d", event.Creator.ID)
	}
	if event.Creator.Name != "Annie Bryan" {
		t.Errorf("expected creator name 'Annie Bryan', got %q", event.Creator.Name)
	}
	if event.Creator.EmailAddress != "annie@honcho.com" {
		t.Errorf("expected creator email 'annie@honcho.com', got %q", event.Creator.EmailAddress)
	}

	// Recording creator
	if rec.Creator == nil {
		t.Fatal("expected non-nil recording creator")
	}
	if rec.Creator.ID != 1049715914 {
		t.Errorf("expected recording creator ID 1049715914, got %d", rec.Creator.ID)
	}
	if rec.Creator.Company == nil {
		t.Fatal("expected non-nil recording creator company")
	}
	if rec.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected company name 'Honcho Design', got %q", rec.Creator.Company.Name)
	}

	// No copy field
	if event.Copy != nil {
		t.Error("expected nil copy for todo_created event")
	}
}

func TestWebhookEvent_UnmarshalMessageCopied(t *testing.T) {
	data := loadWebhooksFixture(t, "event-message-copied.json")

	var event WebhookEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("failed to unmarshal event-message-copied.json: %v", err)
	}

	if event.ID != 9007199254741002 {
		t.Errorf("expected ID 9007199254741002, got %d", event.ID)
	}
	if event.Kind != "message_copied" {
		t.Errorf("expected kind 'message_copied', got %q", event.Kind)
	}

	// Details should contain source_bucket_id
	if event.Details == nil {
		t.Fatal("expected non-nil details")
	}
	detailsMap, ok := event.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details to be a map, got %T", event.Details)
	}
	sourceBucketID, ok := detailsMap["source_bucket_id"]
	if !ok {
		t.Fatal("expected source_bucket_id in details")
	}
	// JSON numbers unmarshal as float64
	if sourceBucketID.(float64) != 2085958500 {
		t.Errorf("expected source_bucket_id 2085958500, got %v", sourceBucketID)
	}

	// Recording
	if event.Recording.Type != "Message" {
		t.Errorf("expected recording type 'Message', got %q", event.Recording.Type)
	}
	if event.Recording.Title != "Project Update" {
		t.Errorf("expected recording title 'Project Update', got %q", event.Recording.Title)
	}

	// Copy
	if event.Copy == nil {
		t.Fatal("expected non-nil copy for message_copied event")
	}
	if event.Copy.ID != 9007199254741350 {
		t.Errorf("expected copy ID 9007199254741350, got %d", event.Copy.ID)
	}
	if event.Copy.Bucket.ID != 2085958500 {
		t.Errorf("expected copy bucket ID 2085958500, got %d", event.Copy.Bucket.ID)
	}
	if event.Copy.URL == "" {
		t.Error("expected non-empty copy URL")
	}
	if event.Copy.AppURL == "" {
		t.Error("expected non-empty copy AppURL")
	}

	// Creator is different in this fixture
	if event.Creator.Name != "Matt Donahue" {
		t.Errorf("expected creator name 'Matt Donahue', got %q", event.Creator.Name)
	}
}

func TestWebhookEvent_UnmarshalUnknownFuture(t *testing.T) {
	data := loadWebhooksFixture(t, "event-unknown-future.json")

	var event WebhookEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("failed to unmarshal event-unknown-future.json: %v", err)
	}

	if event.ID != 9007199254741099 {
		t.Errorf("expected ID 9007199254741099, got %d", event.ID)
	}
	if event.Kind != "new_thing_activated" {
		t.Errorf("expected kind 'new_thing_activated', got %q", event.Kind)
	}

	// Details should contain future fields
	if event.Details == nil {
		t.Fatal("expected non-nil details")
	}
	detailsMap, ok := event.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected details to be a map, got %T", event.Details)
	}
	futureField, ok := detailsMap["future_field"]
	if !ok {
		t.Fatal("expected future_field in details")
	}
	if futureField != "something_new" {
		t.Errorf("expected future_field 'something_new', got %v", futureField)
	}
	nested, ok := detailsMap["nested"]
	if !ok {
		t.Fatal("expected nested in details")
	}
	nestedMap, ok := nested.(map[string]any)
	if !ok {
		t.Fatal("expected nested to be a map")
	}
	if nestedMap["deeply"] != true {
		t.Errorf("expected nested.deeply to be true, got %v", nestedMap["deeply"])
	}

	// Unknown recording type should still parse fine
	if event.Recording.Type != "NewRecordingType" {
		t.Errorf("expected recording type 'NewRecordingType', got %q", event.Recording.Type)
	}
	if event.Recording.Title != "A Future Thing" {
		t.Errorf("expected recording title 'A Future Thing', got %q", event.Recording.Title)
	}

	// No copy field
	if event.Copy != nil {
		t.Error("expected nil copy for future event")
	}
}

func TestParseEventKind(t *testing.T) {
	tests := []struct {
		kind       string
		wantType   string
		wantAction string
	}{
		{"todo_created", "todo", "created"},
		{"todo_completed", "todo", "completed"},
		{"message_created", "message", "created"},
		{"message_copied", "message", "copied"},
		{"question_answer_created", "question_answer", "created"},
		{"todolist_group_archived", "todolist_group", "archived"},
		{"new_thing_activated", "new_thing", "activated"},
		{"singleword", "singleword", ""},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			gotType, gotAction := ParseEventKind(tt.kind)
			if gotType != tt.wantType {
				t.Errorf("ParseEventKind(%q) type = %q, want %q", tt.kind, gotType, tt.wantType)
			}
			if gotAction != tt.wantAction {
				t.Errorf("ParseEventKind(%q) action = %q, want %q", tt.kind, gotAction, tt.wantAction)
			}
		})
	}
}
