package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadSpecFixture reads a shared fixture under spec/fixtures by its manifest
// path. Those fixtures are validated against the generated schemas by
// `make check-fixture-coverage`, so a test built on one cannot drift from the
// contract the way an inline literal can.
func loadSpecFixture(t *testing.T, relPath string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "spec", "fixtures", filepath.FromSlash(relPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", relPath, err)
	}
	return data
}

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

func TestUpcomingSchedule_DecodesTheReducedCalendarProjection(t *testing.T) {
	// The shared spec fixture, which is the key set BC3 actually renders through
	// app/views/api/schedules/calendar/. The previous version of these tests
	// decoded an invented body carrying `title` (which this endpoint never
	// sends) and omitting `content`, `recurring`, `completion_url`, `completed`,
	// `repeating` and `comments_count` (which it always does), so it could not
	// have caught the mismatch it was there to guard.
	data := loadSpecFixture(t, "schedules/upcoming.json")

	var resp UpcomingScheduleResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp.ScheduleEntries) != 1 {
		t.Fatalf("expected 1 schedule entry, got %d", len(resp.ScheduleEntries))
	}
	if len(resp.RecurringScheduleEntryOccurrences) != 1 {
		t.Fatalf("expected 1 recurring occurrence, got %d", len(resp.RecurringScheduleEntryOccurrences))
	}
	if len(resp.Assignables) != 2 {
		t.Fatalf("expected 2 assignables, got %d", len(resp.Assignables))
	}

	// A timed entry. `recurring` is the key no other schedule-entry projection
	// carries, and the reason this needs its own shape rather than ScheduleEntry.
	entry := resp.ScheduleEntries[0]
	if entry.Summary != "Team Meeting" {
		t.Errorf("expected Summary 'Team Meeting', got %q", entry.Summary)
	}
	if entry.Recurring {
		t.Error("expected Recurring false in schedule_entries: BC3 selects that array with recurrence_schedule IS NULL")
	}
	if entry.StartsAt.Hour() != 6 {
		t.Errorf("expected StartsAt hour 6, got %d", entry.StartsAt.Hour())
	}
	// The bucket is id + name only — no Type field exists on this shape, which
	// is the nested omission that broke a strict decode against TodoBucket.
	if entry.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", entry.Bucket.Name)
	}
	if len(entry.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(entry.Participants))
	}
	if entry.Creator.AvatarUrl == "" {
		t.Error("expected Creator.AvatarUrl to be populated")
	}
	if entry.CommentsCount != 2 {
		t.Errorf("expected CommentsCount 2, got %d", entry.CommentsCount)
	}

	// An all-day occurrence: recurring, and both bounds are bare dates, which is
	// why these fields are types.FlexibleTime rather than time.Time.
	occurrence := resp.RecurringScheduleEntryOccurrences[0]
	if !occurrence.Recurring {
		t.Error("expected Recurring true in recurring_schedule_entry_occurrences")
	}
	if !occurrence.AllDay {
		t.Error("expected AllDay true")
	}
	if occurrence.StartsAt.Year() != 2026 || occurrence.StartsAt.Month() != 6 || occurrence.StartsAt.Day() != 8 {
		t.Errorf("expected StartsAt 2026-06-08, got %v", occurrence.StartsAt)
	}
	if occurrence.EndsAt.Year() != 2026 || occurrence.EndsAt.Month() != 6 || occurrence.EndsAt.Day() != 8 {
		t.Errorf("expected EndsAt 2026-06-08, got %v", occurrence.EndsAt)
	}

	// A completed to-do. Its text is Content, not Title, and its Type is the
	// lowercase short recordable name.
	todo := resp.Assignables[0]
	if todo.Content != "Ship the hardware" {
		t.Errorf("expected Content 'Ship the hardware', got %q", todo.Content)
	}
	if todo.Type != "todo" {
		t.Errorf("expected Type 'todo', got %q", todo.Type)
	}
	if todo.Parent.Title != "Launch: Hardware" {
		t.Errorf("expected Parent.Title 'Launch: Hardware', got %q", todo.Parent.Title)
	}
	if !todo.Completed {
		t.Error("expected Completed true")
	}
	if todo.Completion == nil {
		t.Fatal("expected Completion to be present on a completed assignable")
	}
	if todo.Completion.Creator.Name != "Steve Marsh" {
		t.Errorf("expected Completion.Creator.Name 'Steve Marsh', got %q", todo.Completion.Creator.Name)
	}
	if todo.StartsOn == nil || todo.StartsOn.String() != "2026-06-01" {
		t.Errorf("expected StartsOn 2026-06-01, got %v", todo.StartsOn)
	}

	// A card: no completion block, null dates, and a RELATIVE completion_url —
	// BC3 renders the non-to-do branch through a `_path` helper, which emits no
	// host.
	card := resp.Assignables[1]
	if card.Type != "card" {
		t.Errorf("expected Type 'card', got %q", card.Type)
	}
	if card.Completion != nil {
		t.Error("expected Completion to be absent on an incomplete assignable")
	}
	if card.Completed {
		t.Error("expected Completed false")
	}
	if card.CompletionUrl != "/999/buckets/2085958499/steps/1069479526/completions.json" {
		t.Errorf("unexpected CompletionUrl %q", card.CompletionUrl)
	}
	if len(card.Assignees) != 0 {
		t.Errorf("expected 0 assignees, got %d", len(card.Assignees))
	}
}

func TestUpcomingSchedule_EmptyEnvelopeDecodes(t *testing.T) {
	data := `{"schedule_entries": [], "recurring_schedule_entry_occurrences": [], "assignables": []}`

	var resp UpcomingScheduleResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp.ScheduleEntries) != 0 || len(resp.RecurringScheduleEntryOccurrences) != 0 || len(resp.Assignables) != 0 {
		t.Errorf("expected three empty arrays, got %d/%d/%d",
			len(resp.ScheduleEntries), len(resp.RecurringScheduleEntryOccurrences), len(resp.Assignables))
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

// Both window bounds are required, and the local guard has to say so itself.
// types.ParseDate("") returns a zero Date and a NIL error by design, so parsing
// alone accepts exactly the input this operation must reject: an empty bound
// would sail past the check and come back as the server-side 400 the guard
// exists to prevent. Both missing-bound cases are covered because a guard that
// only checks the first argument is the usual way this half-works.
func TestUpcomingSchedule_RejectsMissingWindowBounds(t *testing.T) {
	cases := []struct {
		name      string
		startDate string
		endDate   string
		wantIn    string
	}{
		{"both empty", "", "", "window_starts_on"},
		{"start empty", "", "2026-06-30", "window_starts_on"},
		{"end empty", "2026-06-01", "", "window_ends_on"},
		{"start malformed", "june", "2026-06-30", "window_starts_on"},
		{"end malformed", "2026-06-01", "2026", "window_ends_on"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"schedule_entries":[],"recurring_schedule_entry_occurrences":[],"assignables":[]}`))
			}))
			t.Cleanup(srv.Close)

			cfg := DefaultConfig()
			cfg.BaseURL = srv.URL
			client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

			_, err := client.ForAccount("999").Reports().UpcomingSchedule(context.Background(), tc.startDate, tc.endDate)
			if err == nil {
				t.Fatal("expected a local usage error, got none")
			}
			if called {
				t.Error("the request reached the server; the local guard should have stopped it")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("expected the error to name %q, got %q", tc.wantIn, err.Error())
			}
		})
	}
}
