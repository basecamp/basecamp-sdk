package basecamp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testMyAssignmentsServer(t *testing.T, handler http.HandlerFunc) *MyAssignmentsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.MyAssignments()
}

func TestMyAssignmentsService_List(t *testing.T) {
	fixture := []byte(`{
		"priorities": [
			{
				"id": 101,
				"app_url": "https://3.basecamp.com/99999/buckets/123/todos/101",
				"content": "Priority assignment",
				"starts_on": "2024-03-01",
				"due_on": "2024-03-05",
				"completed": false,
				"type": "Todo",
				"comments_count": 2,
				"has_description": true,
				"priority_recording_id": 7001,
				"bucket": {
					"id": 123,
					"name": "Client Project",
					"app_url": "https://3.basecamp.com/99999/buckets/123"
				},
				"parent": {
					"id": 456,
					"title": "Launch list",
					"app_url": "https://3.basecamp.com/99999/buckets/123/todolists/456"
				},
				"assignees": [
					{
						"id": 88,
						"name": "Pat Doe",
						"avatar_url": "https://example.test/avatar.png"
					}
				],
				"children": [
					{
						"id": 202,
						"content": "Nested assignment",
						"completed": true,
						"type": "Todo",
						"comments_count": 0,
						"has_description": false
					}
				]
			}
		],
		"non_priorities": [
			{
				"id": 303,
				"content": "Backlog assignment",
				"completed": true,
				"type": "Todo",
				"comments_count": 1,
				"has_description": false
			}
		]
	}`)

	svc := testMyAssignmentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/my/assignments.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	})

	result, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Priorities) != 1 {
		t.Fatalf("expected 1 priority assignment, got %d", len(result.Priorities))
	}
	if len(result.NonPriorities) != 1 {
		t.Fatalf("expected 1 non-priority assignment, got %d", len(result.NonPriorities))
	}

	priority := result.Priorities[0]
	if priority.Content != "Priority assignment" {
		t.Errorf("expected priority content, got %q", priority.Content)
	}
	if priority.StartsOn != "2024-03-01" {
		t.Errorf("expected starts_on 2024-03-01, got %q", priority.StartsOn)
	}
	if priority.DueOn != "2024-03-05" {
		t.Errorf("expected due_on 2024-03-05, got %q", priority.DueOn)
	}
	if priority.PriorityRecordingID == nil || *priority.PriorityRecordingID != 7001 {
		t.Fatalf("expected priority_recording_id 7001, got %v", priority.PriorityRecordingID)
	}
	if priority.Bucket == nil || priority.Bucket.Name != "Client Project" {
		t.Fatalf("expected bucket name Client Project, got %+v", priority.Bucket)
	}
	if priority.Parent == nil || priority.Parent.Title != "Launch list" {
		t.Fatalf("expected parent title Launch list, got %+v", priority.Parent)
	}
	if len(priority.Assignees) != 1 || priority.Assignees[0].Name != "Pat Doe" {
		t.Fatalf("expected assignee Pat Doe, got %+v", priority.Assignees)
	}
	if len(priority.Children) != 1 || priority.Children[0].Content != "Nested assignment" {
		t.Fatalf("expected nested assignment child, got %+v", priority.Children)
	}
	if !priority.Children[0].Completed {
		t.Error("expected nested assignment to be completed")
	}

	if !result.NonPriorities[0].Completed {
		t.Error("expected non-priority assignment to be completed")
	}
}

func TestMyAssignmentsService_Completed(t *testing.T) {
	fixture := []byte(`[
		{
			"id": 401,
			"content": "Completed assignment",
			"completed": true,
			"type": "Todo",
			"comments_count": 4,
			"has_description": true,
			"due_on": "2024-04-02"
		}
	]`)

	svc := testMyAssignmentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/my/assignments/completed.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	})

	result, err := svc.Completed(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 completed assignment, got %d", len(result))
	}
	if !result[0].Completed {
		t.Error("expected assignment to be completed")
	}
	if result[0].DueOn != "2024-04-02" {
		t.Errorf("expected due_on 2024-04-02, got %q", result[0].DueOn)
	}
}

func TestMyAssignmentsService_Due(t *testing.T) {
	fixture := []byte(`[
		{
			"id": 501,
			"content": "Due today assignment",
			"completed": false,
			"type": "Todo",
			"comments_count": 0,
			"has_description": false,
			"due_on": "2024-04-03"
		}
	]`)

	svc := testMyAssignmentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/my/assignments/due.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("scope"); got != "due_today" {
			t.Errorf("expected scope query due_today, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	})

	result, err := svc.Due(context.Background(), &DueOptions{Scope: "due_today"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 due assignment, got %d", len(result))
	}
	if result[0].DueOn != "2024-04-03" {
		t.Errorf("expected due_on 2024-04-03, got %q", result[0].DueOn)
	}
}

func TestMyAssignmentsService_ListUnauthorized(t *testing.T) {
	svc := testMyAssignmentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})

	_, err := svc.List(context.Background())
	if err == nil {
		t.Fatal("expected unauthorized error")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if apiErr.Code != CodeAuth {
		t.Errorf("expected auth error code, got %q", apiErr.Code)
	}
}
