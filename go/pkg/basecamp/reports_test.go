package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssignedTodosResponse_Unmarshal(t *testing.T) {
	data := `{
		"person": {
			"id": 111,
			"name": "Test User",
			"email_address": "test@example.com"
		},
		"grouped_by": "bucket",
		"todos": [
			{
				"id": 12345,
				"content": "Test todo",
				"completed": false,
				"due_on": "2024-03-20"
			}
		]
	}`

	var resp AssignedTodosResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Person == nil {
		t.Fatal("expected Person to be non-nil")
	}
	if resp.Person.Name != "Test User" {
		t.Errorf("expected Person.Name 'Test User', got %q", resp.Person.Name)
	}
	if resp.GroupedBy != "bucket" {
		t.Errorf("expected GroupedBy 'bucket', got %q", resp.GroupedBy)
	}
	if len(resp.Todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(resp.Todos))
	}
	if resp.Todos[0].Content != "Test todo" {
		t.Errorf("expected todo Content 'Test todo', got %q", resp.Todos[0].Content)
	}
}

func TestOverdueTodosResponse_Unmarshal(t *testing.T) {
	data := `{
		"under_a_week_late": [
			{"id": 1, "content": "Todo 1", "due_on": "2024-03-10"}
		],
		"over_a_week_late": [
			{"id": 2, "content": "Todo 2", "due_on": "2024-03-01"}
		],
		"over_a_month_late": [
			{"id": 3, "content": "Todo 3", "due_on": "2024-02-01"}
		],
		"over_three_months_late": [
			{"id": 4, "content": "Todo 4", "due_on": "2023-12-01"}
		]
	}`

	var resp OverdueTodosResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp.UnderAWeekLate) != 1 {
		t.Errorf("expected 1 todo under a week late, got %d", len(resp.UnderAWeekLate))
	}
	if len(resp.OverAWeekLate) != 1 {
		t.Errorf("expected 1 todo over a week late, got %d", len(resp.OverAWeekLate))
	}
	if len(resp.OverAMonthLate) != 1 {
		t.Errorf("expected 1 todo over a month late, got %d", len(resp.OverAMonthLate))
	}
	if len(resp.OverThreeMonthsLate) != 1 {
		t.Errorf("expected 1 todo over three months late, got %d", len(resp.OverThreeMonthsLate))
	}
}

func TestAssignable_Unmarshal(t *testing.T) {
	data := `{
		"id": 12345,
		"title": "Test Schedule Entry",
		"type": "ScheduleEntry",
		"url": "https://3.basecampapi.com/123/buckets/456/schedule_entries/789.json",
		"app_url": "https://3.basecamp.com/123/buckets/456/schedule_entries/789",
		"due_on": "2024-03-20",
		"starts_on": "2024-03-15",
		"bucket": {
			"id": 456,
			"name": "Test Project",
			"type": "Project"
		},
		"parent": {
			"id": 789,
			"title": "Schedule",
			"type": "Schedule"
		},
		"assignees": [
			{
				"id": 111,
				"name": "Test User"
			}
		]
	}`

	var assignable Assignable
	if err := json.Unmarshal([]byte(data), &assignable); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if assignable.ID != 12345 {
		t.Errorf("expected ID 12345, got %d", assignable.ID)
	}
	if assignable.Title != "Test Schedule Entry" {
		t.Errorf("expected Title 'Test Schedule Entry', got %q", assignable.Title)
	}
	if assignable.Type != "ScheduleEntry" {
		t.Errorf("expected Type 'ScheduleEntry', got %q", assignable.Type)
	}
	if assignable.DueOn != "2024-03-20" {
		t.Errorf("expected DueOn '2024-03-20', got %q", assignable.DueOn)
	}
	if assignable.StartsOn != "2024-03-15" {
		t.Errorf("expected StartsOn '2024-03-15', got %q", assignable.StartsOn)
	}
	if assignable.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if assignable.Bucket.Name != "Test Project" {
		t.Errorf("expected Bucket.Name 'Test Project', got %q", assignable.Bucket.Name)
	}
	if assignable.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if assignable.Parent.Title != "Schedule" {
		t.Errorf("expected Parent.Title 'Schedule', got %q", assignable.Parent.Title)
	}
	if len(assignable.Assignees) != 1 {
		t.Fatalf("expected 1 assignee, got %d", len(assignable.Assignees))
	}
	if assignable.Assignees[0].Name != "Test User" {
		t.Errorf("expected Assignee.Name 'Test User', got %q", assignable.Assignees[0].Name)
	}
}

func TestUpcomingScheduleResponse_Unmarshal(t *testing.T) {
	data := `{
		"schedule_entries": [
			{"id": 1, "summary": "Entry 1", "starts_at": "2022-11-01T10:00:00.000Z", "ends_at": "2022-11-01T11:00:00.000Z"},
			{"id": 4, "summary": "All Day", "starts_at": "2022-11-15", "ends_at": "2022-11-15", "all_day": true}
		],
		"recurring_schedule_entry_occurrences": [
			{"id": 2, "summary": "Recurring Entry", "starts_at": "2022-12-01T09:00:00Z", "ends_at": "2022-12-01T10:00:00Z"}
		],
		"assignables": [
			{"id": 3, "title": "Assignable 1"}
		]
	}`

	var resp UpcomingScheduleResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp.ScheduleEntries) != 2 {
		t.Fatalf("expected 2 schedule entries, got %d", len(resp.ScheduleEntries))
	}
	if len(resp.RecurringOccurrences) != 1 {
		t.Errorf("expected 1 recurring occurrence, got %d", len(resp.RecurringOccurrences))
	}
	if len(resp.Assignables) != 1 {
		t.Errorf("expected 1 assignable, got %d", len(resp.Assignables))
	}

	// Verify RFC3339 datetime entry
	e1 := resp.ScheduleEntries[0]
	if e1.StartsAt.IsZero() {
		t.Error("expected StartsAt to be non-zero for datetime entry")
	}
	if e1.StartsAt.Hour() != 10 {
		t.Errorf("expected StartsAt hour 10, got %d", e1.StartsAt.Hour())
	}

	// Verify date-only entry
	e2 := resp.ScheduleEntries[1]
	if e2.StartsAt.IsZero() {
		t.Error("expected StartsAt to be non-zero for date-only entry")
	}
	if e2.StartsAt.Year() != 2022 || e2.StartsAt.Month() != 11 || e2.StartsAt.Day() != 15 {
		t.Errorf("expected StartsAt 2022-11-15, got %v", e2.StartsAt)
	}
	if e2.EndsAt.Year() != 2022 || e2.EndsAt.Month() != 11 || e2.EndsAt.Day() != 15 {
		t.Errorf("expected EndsAt 2022-11-15, got %v", e2.EndsAt)
	}
}

func TestAssignedTodosOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    *AssignedTodosOptions
		groupBy string
	}{
		{"nil options", nil, ""},
		{"empty group by", &AssignedTodosOptions{}, ""},
		{"group by bucket", &AssignedTodosOptions{GroupBy: "bucket"}, "bucket"},
		{"group by date", &AssignedTodosOptions{GroupBy: "date"}, "date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts != nil && tt.opts.GroupBy != tt.groupBy {
				t.Errorf("expected GroupBy %q, got %q", tt.groupBy, tt.opts.GroupBy)
			}
		})
	}
}

func TestReportsService_Assignments(t *testing.T) {
	svc := testReportsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/my/assignments.json" {
			t.Fatalf("expected /99999/my/assignments.json, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"priorities": [{
				"id": 9007199254741623,
				"app_url": "https://3.basecamp.com/195539477/buckets/2085958504/todos/9007199254741623",
				"content": "Program the flux capacitor",
				"due_on": "2026-03-15",
				"bucket": {
					"id": 2085958504,
					"name": "The Leto Laptop",
					"app_url": "https://3.basecamp.com/195539477/buckets/2085958504"
				},
				"completed": false,
				"type": "todo",
				"assignees": [{ "id": 1049715913, "name": "Victor Cooper", "avatar_url": "https://bc3-production-assets-cdn.basecamp-static.com/people/1049715913/avatar.jpg" }],
				"comments_count": 0,
				"has_description": false,
				"priority_recording_id": 9007199254741700,
				"parent": {
					"id": 9007199254741601,
					"title": "Development tasks",
					"app_url": "https://3.basecamp.com/195539477/buckets/2085958504/todolists/9007199254741601"
				},
				"children": [{
					"id": 9007199254741800,
					"app_url": "https://3.basecamp.com/195539477/buckets/2085958504/cards/9007199254741800",
					"content": "Wire up cache fix",
					"bucket": {
						"id": 2085958504,
						"name": "The Leto Laptop",
						"app_url": "https://3.basecamp.com/195539477/buckets/2085958504"
					},
					"completed": false,
					"type": "card_step",
					"assignees": [{ "id": 1049715913, "name": "Victor Cooper", "avatar_url": "https://bc3-production-assets-cdn.basecamp-static.com/people/1049715913/avatar.jpg" }],
					"comments_count": 1,
					"has_description": true,
					"parent": {
						"id": 9007199254741701,
						"title": "Assignments API",
						"app_url": "https://3.basecamp.com/195539477/buckets/2085958504/card_tables/cards/9007199254741701"
					},
					"children": []
				}]
			}],
			"non_priorities": []
		}`))
	})

	result, err := svc.Assignments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Priorities) != 1 {
		t.Fatalf("expected 1 priority assignment, got %d", len(result.Priorities))
	}

	item := result.Priorities[0]
	if item.Content != "Program the flux capacitor" {
		t.Errorf("expected content to round-trip, got %q", item.Content)
	}
	if item.PriorityRecordingID == nil || *item.PriorityRecordingID != 9007199254741700 {
		t.Fatalf("expected priority_recording_id to be set, got %v", item.PriorityRecordingID)
	}
	if item.Bucket == nil || item.Bucket.AppURL == "" {
		t.Fatalf("expected bucket app_url to be present, got %+v", item.Bucket)
	}
	if len(item.Assignees) != 1 || item.Assignees[0].AvatarURL == "" {
		t.Fatalf("expected assignee avatar_url to round-trip, got %+v", item.Assignees)
	}
	if len(item.Children) != 1 {
		t.Fatalf("expected 1 child assignment, got %d", len(item.Children))
	}
	if item.Children[0].Content != "Wire up cache fix" {
		t.Errorf("expected child content to round-trip, got %q", item.Children[0].Content)
	}
}

func TestReportsService_AssignmentsNotFound(t *testing.T) {
	svc := testReportsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not found"}`))
	})

	_, err := svc.Assignments(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	apiErr := AsError(err)
	if apiErr.HTTPStatus != http.StatusNotFound {
		t.Fatalf("expected HTTP status 404, got %d", apiErr.HTTPStatus)
	}
	if apiErr.Code != CodeNotFound {
		t.Fatalf("expected CodeNotFound for 404 response, got %q", apiErr.Code)
	}
}

func TestReportsService_CompletedAssignments(t *testing.T) {
	svc := testReportsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/my/assignments/completed.json" {
			t.Fatalf("expected /99999/my/assignments/completed.json, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 9007199254741623,
				"app_url": "https://3.basecamp.com/195539477/buckets/2085958504/todos/9007199254741623",
				"content": "Program the flux capacitor",
				"due_on": "2026-03-15",
				"bucket": {
					"id": 2085958504,
					"name": "The Leto Laptop",
					"app_url": "https://3.basecamp.com/195539477/buckets/2085958504"
				},
				"completed": true,
				"type": "todo",
				"assignees": [{ "id": 1049715913, "name": "Victor Cooper", "avatar_url": "https://bc3-production-assets-cdn.basecamp-static.com/people/1049715913/avatar.jpg" }],
				"comments_count": 0,
				"has_description": false,
				"parent": {
					"id": 9007199254741601,
					"title": "Development tasks",
					"app_url": "https://3.basecamp.com/195539477/buckets/2085958504/todolists/9007199254741601"
				},
				"children": []
			}
		]`))
	})

	result, err := svc.CompletedAssignments(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 completed assignment, got %d", len(result))
	}
	if !result[0].Completed {
		t.Error("expected completed assignment to be marked completed")
	}
	if len(result[0].Assignees) != 1 || result[0].Assignees[0].AvatarURL == "" {
		t.Fatalf("expected assignee avatar_url to round-trip, got %+v", result[0].Assignees)
	}
}

func TestReportsService_CompletedAssignmentsNotFound(t *testing.T) {
	svc := testReportsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not found"}`))
	})

	_, err := svc.CompletedAssignments(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	apiErr := AsError(err)
	if apiErr.HTTPStatus != http.StatusNotFound {
		t.Fatalf("expected HTTP status 404, got %d", apiErr.HTTPStatus)
	}
	if apiErr.Code != CodeNotFound {
		t.Fatalf("expected CodeNotFound for 404 response, got %q", apiErr.Code)
	}
}

func TestReportsService_DueAssignments(t *testing.T) {
	svc := testReportsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/my/assignments/due.json" {
			t.Fatalf("expected /99999/my/assignments/due.json, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("scope"); got != "due_tomorrow" {
			t.Fatalf("expected scope=due_tomorrow, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 9007199254741623,
				"app_url": "https://3.basecamp.com/195539477/buckets/2085958504/todos/9007199254741623",
				"content": "Program the flux capacitor",
				"due_on": "2026-03-22",
				"bucket": {
					"id": 2085958504,
					"name": "The Leto Laptop",
					"app_url": "https://3.basecamp.com/195539477/buckets/2085958504"
				},
				"completed": false,
				"type": "todo",
				"assignees": [{ "id": 1049715913, "name": "Victor Cooper", "avatar_url": "https://bc3-production-assets-cdn.basecamp-static.com/people/1049715913/avatar.jpg" }],
				"comments_count": 0,
				"has_description": false,
				"parent": {
					"id": 9007199254741601,
					"title": "Development tasks",
					"app_url": "https://3.basecamp.com/195539477/buckets/2085958504/todolists/9007199254741601"
				},
				"children": []
			}
		]`))
	})

	result, err := svc.DueAssignments(context.Background(), &DueAssignmentsOptions{Scope: "due_tomorrow"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 due assignment, got %d", len(result))
	}
	if result[0].DueOn != "2026-03-22" {
		t.Errorf("expected due_on to round-trip, got %q", result[0].DueOn)
	}
	if len(result[0].Assignees) != 1 || result[0].Assignees[0].AvatarURL == "" {
		t.Fatalf("expected assignee avatar_url to round-trip, got %+v", result[0].Assignees)
	}
}

func TestReportsService_DueAssignmentsInvalidScope(t *testing.T) {
	svc := testReportsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"error": "Invalid scope 'invalid'. Valid options: overdue, due_today, due_tomorrow, due_later_this_week, due_next_week, due_later"
		}`))
	})

	_, err := svc.DueAssignments(context.Background(), &DueAssignmentsOptions{Scope: "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}

	apiErr := AsError(err)
	if apiErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected HTTP status 400, got %d", apiErr.HTTPStatus)
	}
	if apiErr.Code != CodeAPI {
		t.Fatalf("expected CodeAPI for 400 response, got %q", apiErr.Code)
	}
	if !strings.Contains(apiErr.Message, "Invalid scope 'invalid'") {
		t.Fatalf("expected server error message to be preserved, got %q", apiErr.Message)
	}
}

func testReportsServer(t *testing.T, handler http.HandlerFunc) *ReportsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Reports()
}
