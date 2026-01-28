package basecamp

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimelineEvent_Unmarshal(t *testing.T) {
	data := `{
		"id": 12345,
		"created_at": "2024-03-15T10:30:00Z",
		"kind": "message_created",
		"parent_recording_id": 67890,
		"url": "https://3.basecampapi.com/123/buckets/456/messages/789.json",
		"app_url": "https://3.basecamp.com/123/buckets/456/messages/789",
		"action": "created",
		"target": "message",
		"title": "Test Message",
		"summary_excerpt": "This is a test...",
		"creator": {
			"id": 111,
			"name": "Test User",
			"email_address": "test@example.com"
		},
		"bucket": {
			"id": 456,
			"name": "Test Project",
			"type": "Project"
		}
	}`

	var event TimelineEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if event.ID != 12345 {
		t.Errorf("expected ID 12345, got %d", event.ID)
	}
	if event.Kind != "message_created" {
		t.Errorf("expected Kind 'message_created', got %q", event.Kind)
	}
	if event.ParentRecordingID != 67890 {
		t.Errorf("expected ParentRecordingID 67890, got %d", event.ParentRecordingID)
	}
	if event.Action != "created" {
		t.Errorf("expected Action 'created', got %q", event.Action)
	}
	if event.Target != "message" {
		t.Errorf("expected Target 'message', got %q", event.Target)
	}
	if event.Title != "Test Message" {
		t.Errorf("expected Title 'Test Message', got %q", event.Title)
	}
	if event.SummaryExcerpt != "This is a test..." {
		t.Errorf("expected SummaryExcerpt 'This is a test...', got %q", event.SummaryExcerpt)
	}
	if event.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if event.Creator.Name != "Test User" {
		t.Errorf("expected Creator.Name 'Test User', got %q", event.Creator.Name)
	}
	if event.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if event.Bucket.Name != "Test Project" {
		t.Errorf("expected Bucket.Name 'Test Project', got %q", event.Bucket.Name)
	}

	// Check timestamp
	expectedTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	if !event.CreatedAt.Equal(expectedTime) {
		t.Errorf("expected CreatedAt %v, got %v", expectedTime, event.CreatedAt)
	}
}

func TestPersonProgressResponse_Unmarshal(t *testing.T) {
	data := `{
		"person": {
			"id": 111,
			"name": "Test User",
			"email_address": "test@example.com"
		},
		"events": [
			{
				"id": 12345,
				"kind": "todo_completed",
				"action": "completed",
				"title": "Test Todo"
			}
		]
	}`

	var resp PersonProgressResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Person == nil {
		t.Fatal("expected Person to be non-nil")
	}
	if resp.Person.Name != "Test User" {
		t.Errorf("expected Person.Name 'Test User', got %q", resp.Person.Name)
	}
	if len(resp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(resp.Events))
	}
	if resp.Events[0].Kind != "todo_completed" {
		t.Errorf("expected event Kind 'todo_completed', got %q", resp.Events[0].Kind)
	}
}
