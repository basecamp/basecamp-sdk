package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func schedulesFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "schedules")
}

func loadSchedulesFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(schedulesFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestSchedule_UnmarshalGet(t *testing.T) {
	data := loadSchedulesFixture(t, "get.json")

	var schedule Schedule
	if err := json.Unmarshal(data, &schedule); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if schedule.ID != 1069479342 {
		t.Errorf("expected ID 1069479342, got %d", schedule.ID)
	}
	if schedule.Status != "active" {
		t.Errorf("expected status 'active', got %q", schedule.Status)
	}
	if schedule.Type != "Schedule" {
		t.Errorf("expected type 'Schedule', got %q", schedule.Type)
	}
	if schedule.Title != "Schedule" {
		t.Errorf("expected title 'Schedule', got %q", schedule.Title)
	}
	if schedule.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/schedules/1069479342.json" {
		t.Errorf("unexpected URL: %q", schedule.URL)
	}
	if schedule.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/schedules/1069479342" {
		t.Errorf("unexpected AppURL: %q", schedule.AppURL)
	}
	if schedule.Position != 2 {
		t.Errorf("expected position 2, got %d", schedule.Position)
	}
	if !schedule.IncludeDueAssignments {
		t.Error("expected IncludeDueAssignments to be true")
	}
	if schedule.EntriesCount != 5 {
		t.Errorf("expected entries_count 5, got %d", schedule.EntriesCount)
	}
	if schedule.EntriesURL != "https://3.basecampapi.com/195539477/buckets/2085958499/schedules/1069479342/entries.json" {
		t.Errorf("unexpected EntriesURL: %q", schedule.EntriesURL)
	}

	// Verify timestamps are parsed
	if schedule.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if schedule.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify bucket
	if schedule.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if schedule.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", schedule.Bucket.ID)
	}
	if schedule.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", schedule.Bucket.Name)
	}
	if schedule.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", schedule.Bucket.Type)
	}

	// Verify creator
	if schedule.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if schedule.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", schedule.Creator.ID)
	}
	if schedule.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", schedule.Creator.Name)
	}
}

func TestScheduleEntry_UnmarshalList(t *testing.T) {
	data := loadSchedulesFixture(t, "entries_list.json")

	var entries []ScheduleEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal entries_list.json: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Verify first entry
	e1 := entries[0]
	if e1.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", e1.ID)
	}
	if e1.Status != "active" {
		t.Errorf("expected status 'active', got %q", e1.Status)
	}
	if e1.Type != "Schedule::Entry" {
		t.Errorf("expected type 'Schedule::Entry', got %q", e1.Type)
	}
	if e1.Title != "Project Kickoff Meeting" {
		t.Errorf("expected title 'Project Kickoff Meeting', got %q", e1.Title)
	}
	if e1.Summary != "Project Kickoff Meeting" {
		t.Errorf("expected summary 'Project Kickoff Meeting', got %q", e1.Summary)
	}
	if e1.AllDay {
		t.Error("expected AllDay to be false for first entry")
	}
	if e1.Description != "<div>Discuss project goals and timeline.</div>" {
		t.Errorf("unexpected description: %q", e1.Description)
	}
	if e1.CommentsCount != 2 {
		t.Errorf("expected CommentsCount 2, got %d", e1.CommentsCount)
	}

	// Verify timestamps
	if e1.StartsAt.IsZero() {
		t.Error("expected StartsAt to be non-zero")
	}
	if e1.EndsAt.IsZero() {
		t.Error("expected EndsAt to be non-zero")
	}

	// Verify parent (schedule)
	if e1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if e1.Parent.ID != 1069479342 {
		t.Errorf("expected Parent.ID 1069479342, got %d", e1.Parent.ID)
	}
	if e1.Parent.Title != "Schedule" {
		t.Errorf("expected Parent.Title 'Schedule', got %q", e1.Parent.Title)
	}
	if e1.Parent.Type != "Schedule" {
		t.Errorf("expected Parent.Type 'Schedule', got %q", e1.Parent.Type)
	}

	// Verify bucket
	if e1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if e1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", e1.Bucket.ID)
	}

	// Verify creator
	if e1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if e1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", e1.Creator.Name)
	}

	// Verify participants
	if len(e1.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(e1.Participants))
	}
	if e1.Participants[0].Name != "Victor Cooper" {
		t.Errorf("expected first participant 'Victor Cooper', got %q", e1.Participants[0].Name)
	}
	if e1.Participants[1].Name != "Annie Bryan" {
		t.Errorf("expected second participant 'Annie Bryan', got %q", e1.Participants[1].Name)
	}

	// Verify second entry (all-day event)
	e2 := entries[1]
	if e2.ID != 1069479410 {
		t.Errorf("expected ID 1069479410, got %d", e2.ID)
	}
	if e2.Title != "Design Review" {
		t.Errorf("expected title 'Design Review', got %q", e2.Title)
	}
	if !e2.AllDay {
		t.Error("expected AllDay to be true for second entry")
	}
	if !e2.VisibleToClients {
		t.Error("expected VisibleToClients to be true for second entry")
	}
	if len(e2.Participants) != 0 {
		t.Errorf("expected 0 participants for second entry, got %d", len(e2.Participants))
	}
	if e2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", e2.Creator.Name)
	}
}

func TestScheduleEntry_UnmarshalGet(t *testing.T) {
	data := loadSchedulesFixture(t, "entry_get.json")

	var entry ScheduleEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal entry_get.json: %v", err)
	}

	if entry.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", entry.ID)
	}
	if entry.Status != "active" {
		t.Errorf("expected status 'active', got %q", entry.Status)
	}
	if entry.Type != "Schedule::Entry" {
		t.Errorf("expected type 'Schedule::Entry', got %q", entry.Type)
	}
	if entry.Title != "Project Kickoff Meeting" {
		t.Errorf("expected title 'Project Kickoff Meeting', got %q", entry.Title)
	}
	if entry.Summary != "Project Kickoff Meeting" {
		t.Errorf("expected summary 'Project Kickoff Meeting', got %q", entry.Summary)
	}
	if entry.AllDay {
		t.Error("expected AllDay to be false")
	}
	if entry.Description != "<div>Discuss project goals and timeline.</div>" {
		t.Errorf("unexpected description: %q", entry.Description)
	}
	if entry.CommentsCount != 2 {
		t.Errorf("expected CommentsCount 2, got %d", entry.CommentsCount)
	}
	if entry.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/schedule_entries/1069479400.json" {
		t.Errorf("unexpected URL: %q", entry.URL)
	}
	if entry.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/schedule_entries/1069479400" {
		t.Errorf("unexpected AppURL: %q", entry.AppURL)
	}
	if entry.SubscriptionURL != "https://3.basecampapi.com/195539477/buckets/2085958499/recordings/1069479400/subscription.json" {
		t.Errorf("unexpected SubscriptionURL: %q", entry.SubscriptionURL)
	}
	if entry.CommentsURL != "https://3.basecampapi.com/195539477/buckets/2085958499/recordings/1069479400/comments.json" {
		t.Errorf("unexpected CommentsURL: %q", entry.CommentsURL)
	}

	// Verify timestamps
	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
	if entry.StartsAt.IsZero() {
		t.Error("expected StartsAt to be non-zero")
	}
	if entry.EndsAt.IsZero() {
		t.Error("expected EndsAt to be non-zero")
	}

	// Verify participants
	if len(entry.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(entry.Participants))
	}
}

func TestCreateScheduleEntryRequest_Marshal(t *testing.T) {
	req := CreateScheduleEntryRequest{
		Summary:        "Team Meeting",
		StartsAt:       "2022-11-10T14:00:00.000Z",
		EndsAt:         "2022-11-10T15:00:00.000Z",
		Description:    "<div>Weekly sync</div>",
		ParticipantIDs: []int64{1049715914, 1049715915},
		AllDay:         false,
		Notify:         true,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateScheduleEntryRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["summary"] != "Team Meeting" {
		t.Errorf("unexpected summary: %v", data["summary"])
	}
	if data["starts_at"] != "2022-11-10T14:00:00.000Z" {
		t.Errorf("unexpected starts_at: %v", data["starts_at"])
	}
	if data["ends_at"] != "2022-11-10T15:00:00.000Z" {
		t.Errorf("unexpected ends_at: %v", data["ends_at"])
	}
	if data["description"] != "<div>Weekly sync</div>" {
		t.Errorf("unexpected description: %v", data["description"])
	}
	if data["notify"] != true {
		t.Errorf("unexpected notify: %v", data["notify"])
	}

	// Check participant_ids
	pids, ok := data["participant_ids"].([]interface{})
	if !ok {
		t.Fatalf("expected participant_ids to be array, got %T", data["participant_ids"])
	}
	if len(pids) != 2 {
		t.Errorf("expected 2 participant_ids, got %d", len(pids))
	}

	// Round-trip test
	var roundtrip CreateScheduleEntryRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Summary != req.Summary {
		t.Errorf("expected summary %q, got %q", req.Summary, roundtrip.Summary)
	}
	if roundtrip.StartsAt != req.StartsAt {
		t.Errorf("expected starts_at %q, got %q", req.StartsAt, roundtrip.StartsAt)
	}
	if roundtrip.EndsAt != req.EndsAt {
		t.Errorf("expected ends_at %q, got %q", req.EndsAt, roundtrip.EndsAt)
	}
}

func TestCreateScheduleEntryRequest_MarshalMinimal(t *testing.T) {
	// Test with only required fields
	req := CreateScheduleEntryRequest{
		Summary:  "Quick Meeting",
		StartsAt: "2022-11-10T14:00:00.000Z",
		EndsAt:   "2022-11-10T15:00:00.000Z",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateScheduleEntryRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["summary"] != "Quick Meeting" {
		t.Errorf("unexpected summary: %v", data["summary"])
	}
	if data["starts_at"] != "2022-11-10T14:00:00.000Z" {
		t.Errorf("unexpected starts_at: %v", data["starts_at"])
	}
	if data["ends_at"] != "2022-11-10T15:00:00.000Z" {
		t.Errorf("unexpected ends_at: %v", data["ends_at"])
	}
	// Optional fields with omitempty should not be present
	if _, ok := data["description"]; ok {
		t.Error("expected description to be omitted")
	}
	if _, ok := data["participant_ids"]; ok {
		t.Error("expected participant_ids to be omitted")
	}
}

func TestUpdateScheduleEntryRequest_Marshal(t *testing.T) {
	req := UpdateScheduleEntryRequest{
		Summary:        "Updated Meeting",
		StartsAt:       "2022-11-11T10:00:00.000Z",
		EndsAt:         "2022-11-11T11:00:00.000Z",
		Description:    "<div>Changed time</div>",
		ParticipantIDs: []int64{1049715914},
		AllDay:         false,
		Notify:         true,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateScheduleEntryRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["summary"] != "Updated Meeting" {
		t.Errorf("unexpected summary: %v", data["summary"])
	}
	if data["starts_at"] != "2022-11-11T10:00:00.000Z" {
		t.Errorf("unexpected starts_at: %v", data["starts_at"])
	}
	if data["ends_at"] != "2022-11-11T11:00:00.000Z" {
		t.Errorf("unexpected ends_at: %v", data["ends_at"])
	}
	if data["description"] != "<div>Changed time</div>" {
		t.Errorf("unexpected description: %v", data["description"])
	}
	if data["notify"] != true {
		t.Errorf("unexpected notify: %v", data["notify"])
	}

	// Round-trip test
	var roundtrip UpdateScheduleEntryRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Summary != req.Summary {
		t.Errorf("expected summary %q, got %q", req.Summary, roundtrip.Summary)
	}
}

func TestUpdateScheduleEntryRequest_MarshalPartial(t *testing.T) {
	// Test with only some fields
	req := UpdateScheduleEntryRequest{
		Summary: "Just updating title",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateScheduleEntryRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["summary"] != "Just updating title" {
		t.Errorf("unexpected summary: %v", data["summary"])
	}
	// Optional fields should be omitted
	if _, ok := data["starts_at"]; ok {
		t.Error("expected starts_at to be omitted")
	}
	if _, ok := data["ends_at"]; ok {
		t.Error("expected ends_at to be omitted")
	}
	if _, ok := data["description"]; ok {
		t.Error("expected description to be omitted")
	}
	if _, ok := data["participant_ids"]; ok {
		t.Error("expected participant_ids to be omitted")
	}
}

func TestUpdateScheduleSettingsRequest_Marshal(t *testing.T) {
	// Test with include_due_assignments set to true
	req := UpdateScheduleSettingsRequest{
		IncludeDueAssignments: true,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateScheduleSettingsRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["include_due_assignments"] != true {
		t.Errorf("expected include_due_assignments to be true, got %v", data["include_due_assignments"])
	}

	// Round-trip test
	var roundtrip UpdateScheduleSettingsRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.IncludeDueAssignments != req.IncludeDueAssignments {
		t.Errorf("expected IncludeDueAssignments %v, got %v", req.IncludeDueAssignments, roundtrip.IncludeDueAssignments)
	}

	// Test with include_due_assignments set to false
	reqFalse := UpdateScheduleSettingsRequest{
		IncludeDueAssignments: false,
	}

	outFalse, err := json.Marshal(reqFalse)
	if err != nil {
		t.Fatalf("failed to marshal UpdateScheduleSettingsRequest with false: %v", err)
	}

	var dataFalse map[string]interface{}
	if err := json.Unmarshal(outFalse, &dataFalse); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// The field should still be present even when false (no omitempty)
	if dataFalse["include_due_assignments"] != false {
		t.Errorf("expected include_due_assignments to be false, got %v", dataFalse["include_due_assignments"])
	}
}
