package basecamp

import (
	"encoding/json"
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
			{"id": 1, "summary": "Entry 1"}
		],
		"recurring_schedule_entry_occurrences": [
			{"id": 2, "summary": "Recurring Entry"}
		],
		"assignables": [
			{"id": 3, "title": "Assignable 1"}
		]
	}`

	var resp UpcomingScheduleResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp.ScheduleEntries) != 1 {
		t.Errorf("expected 1 schedule entry, got %d", len(resp.ScheduleEntries))
	}
	if len(resp.RecurringOccurrences) != 1 {
		t.Errorf("expected 1 recurring occurrence, got %d", len(resp.RecurringOccurrences))
	}
	if len(resp.Assignables) != 1 {
		t.Errorf("expected 1 assignable, got %d", len(resp.Assignables))
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
