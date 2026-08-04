package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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

	// Verify second entry (all-day event with date-only starts_at/ends_at)
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
	// Verify date-only strings parse correctly (fixture uses "2022-11-15")
	if e2.StartsAt.IsZero() {
		t.Error("expected StartsAt to be non-zero for all-day entry")
	}
	if e2.StartsAt.Year() != 2022 || e2.StartsAt.Month() != 11 || e2.StartsAt.Day() != 15 {
		t.Errorf("expected StartsAt 2022-11-15, got %v", e2.StartsAt)
	}
	if e2.EndsAt.IsZero() {
		t.Error("expected EndsAt to be non-zero for all-day entry")
	}
	if e2.EndsAt.Year() != 2022 || e2.EndsAt.Month() != 11 || e2.EndsAt.Day() != 15 {
		t.Errorf("expected EndsAt 2022-11-15, got %v", e2.EndsAt)
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
		Notify:         true,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateScheduleEntryRequest: %v", err)
	}

	var data map[string]any
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
	pids, ok := data["participant_ids"].([]any)
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

// TestCreateScheduleEntryRequest_Subscriptions tests that Subscriptions
// field serializes correctly with specific person IDs.
func TestCreateScheduleEntryRequest_Subscriptions(t *testing.T) {
	req := CreateScheduleEntryRequest{
		Summary:       "Quiet Event",
		StartsAt:      "2022-11-10T14:00:00.000Z",
		EndsAt:        "2022-11-10T15:00:00.000Z",
		Subscriptions: &[]int64{111, 222},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateScheduleEntryRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	subs, ok := data["subscriptions"]
	if !ok {
		t.Fatal("expected subscriptions to be present")
	}
	arr, ok := subs.([]any)
	if !ok {
		t.Fatalf("expected subscriptions to be an array, got %T", subs)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(arr))
	}
	if int64(arr[0].(float64)) != 111 || int64(arr[1].(float64)) != 222 {
		t.Errorf("expected subscriptions [111, 222], got %v", arr)
	}
}

func TestCreateScheduleEntryRequest_SubscriptionsEmpty(t *testing.T) {
	req := CreateScheduleEntryRequest{
		Summary:       "Silent Event",
		StartsAt:      "2022-11-10T14:00:00.000Z",
		EndsAt:        "2022-11-10T15:00:00.000Z",
		Subscriptions: &[]int64{},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	subs, ok := data["subscriptions"]
	if !ok {
		t.Fatal("expected subscriptions to be present for empty slice")
	}
	arr, ok := subs.([]any)
	if !ok {
		t.Fatalf("expected subscriptions to be an array, got %T", subs)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty subscriptions array, got %d items", len(arr))
	}
}

func TestCreateScheduleEntryRequest_SubscriptionsNil(t *testing.T) {
	req := CreateScheduleEntryRequest{
		Summary:  "Default Event",
		StartsAt: "2022-11-10T14:00:00.000Z",
		EndsAt:   "2022-11-10T15:00:00.000Z",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := data["subscriptions"]; ok {
		t.Error("expected subscriptions to be omitted when nil")
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

	var data map[string]any
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

// TestScheduleEntryWriteRequests_WritableSetMatchesTheOperation pins the
// writable set of the schedule-entry write surface — exactly the nine members
// ReplaceScheduleEntryInput declares — across both request shapes, so a caller
// can move between the merge-safe composite and the verbatim replace without
// rewriting the call site.
//
// It also pins that every member is a POINTER. That is not style: three of the
// nine are addressed BY their zero value (participant_ids [] removes everyone,
// url "" drops the join link, highlighted false stops highlighting) and two more
// have zero values that are legitimate writes (description "" clears it, all_day
// false converts an all-day entry into a timed one). A zero-value guard — the
// shape UpdateDocumentRequest and UpdateTodolistRequest use — would make all
// five unreachable, and would silently hand each carve-out clear back to BC3's
// preserve-on-omission.
func TestScheduleEntryWriteRequests_WritableSetMatchesTheOperation(t *testing.T) {
	want := []string{"all_day", "description", "ends_at", "highlighted", "notify", "participant_ids", "starts_at", "summary", "url"}

	for _, tc := range []struct {
		name  string
		value any
	}{
		{"UpdateScheduleEntryRequest", UpdateScheduleEntryRequest{}},
		{"ReplaceScheduleEntryRequest", ReplaceScheduleEntryRequest{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := reflect.TypeOf(tc.value)
			got := make([]string, 0, typ.NumField())
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				got = append(got, name)
				if field.Type.Kind() != reflect.Pointer {
					t.Errorf("%s.%s is %s, but every member must be presence-bearing: nil is \"not addressed\" and the zero value is a legal write",
						tc.name, field.Name, field.Type)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("writable set is %v, want %v", got, want)
			}
		})
	}
}

// TestScheduleEntryFields_CarveOutsAreNotPlainMembers pins the other half of the
// shape: the four addressed-only fields are behind setters, so assignment — not
// value — is what puts them on the wire. A plain exported member could not
// express that, because "left alone" and "assigned the same value" would be the
// same struct.
func TestScheduleEntryFields_CarveOutsAreNotPlainMembers(t *testing.T) {
	typ := reflect.TypeOf(ScheduleEntryFields{})
	exported := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).IsExported() {
			exported = append(exported, typ.Field(i).Name)
		}
	}
	slices.Sort(exported)
	want := []string{"AllDay", "Description", "EndsAt", "StartsAt", "Summary"}
	if !slices.Equal(exported, want) {
		t.Errorf("exported ScheduleEntryFields members are %v, want exactly the full-state set %v", exported, want)
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

	var data map[string]any
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

	var dataFalse map[string]any
	if err := json.Unmarshal(outFalse, &dataFalse); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// The field should still be present even when false (no omitempty)
	if dataFalse["include_due_assignments"] != false {
		t.Errorf("expected include_due_assignments to be false, got %v", dataFalse["include_due_assignments"])
	}
}

// testSchedulesServer creates an httptest.Server and a SchedulesService wired to it.
func testSchedulesServer(t *testing.T, handler http.HandlerFunc) *SchedulesService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Schedules()
}

// The schedule-entry write surface: the merge-safe UpdateEntry, the
// read-modify-write EditEntry, and the verbatim ReplaceEntry.
//
// PUT /schedule_entries/{id} is a FULL REPLACE — Schedules::EntriesController#update
// rebuilds the recordable from only the submitted params — so what these tests
// pin is which bytes reach the wire. Three writable fields are exempt:
// PRESERVED_ON_OMISSION = %i[url highlighted] plus participant_ids (guarded
// since bc3#12425) are seeded from the existing record when the body does not
// address them. That makes omission MEANINGFUL for those three and destructive
// for every other field, and nothing but the request body distinguishes a
// preserve from a clear — both are 200s.

// The fixture's writable state, which every merge-safe call must carry back out
// untouched unless the caller says otherwise.
const (
	fixtureEntrySummary     = "Project Kickoff Meeting"
	fixtureEntryStartsAt    = "2022-11-01T10:00:00.000Z"
	fixtureEntryEndsAt      = "2022-11-01T11:00:00.000Z"
	fixtureEntryDescription = "<div>Discuss project goals and timeline.</div>"
	// The entry's own Basecamp API URL — emitted under "url", which is NOT the
	// join link. Echoing it into the request's url member would store the API
	// URL as the join link, so no PUT in this file may ever carry it.
	fixtureEntryAPIURL  = "https://3.basecampapi.com/195539477/buckets/2085958499/schedule_entries/1069479400.json"
	fixtureEntryJoinURL = "https://meet.example.com/team"
)

// patchScheduleEntryFixture returns the fixture JSON with the given fields
// replaced. A nil value deletes the key, which is how the read-side guards get
// an ABSENT field rather than a null one.
func patchScheduleEntryFixture(t *testing.T, base []byte, patch map[string]any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	for k, v := range patch {
		if v == nil {
			delete(m, k)
			continue
		}
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal patched fixture: %v", err)
	}
	return b
}

// scheduleEntryReadBack is the GET body the composites read: the spec fixture
// plus a populated join link and highlight. The fixture predates bc3#12502 and
// carries neither, and both must be present for the carve-out tests to mean
// anything — a composite that echoes the read-back needs something to echo.
func scheduleEntryReadBack(t *testing.T) []byte {
	t.Helper()
	return patchScheduleEntryFixture(t, loadSchedulesFixture(t, "entry_get.json"), map[string]any{
		"join_url":    fixtureEntryJoinURL,
		"highlighted": true,
	})
}

// capturedScheduleRequest records one request seen by testSchedulesCaptureServer.
type capturedScheduleRequest struct {
	method string
	path   string
	body   map[string]any
}

// testSchedulesCaptureServer serves getBody for GETs and putBody for PUTs while
// recording every request's method, path, and (for PUTs) decoded body. The
// hooks, when non-nil, are installed on the client.
func testSchedulesCaptureServer(t *testing.T, getBody, putBody []byte, hooks Hooks) (*SchedulesService, *[]capturedScheduleRequest) {
	t.Helper()
	reqs := &[]capturedScheduleRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cr := capturedScheduleRequest{method: r.Method, path: r.URL.Path}
		if r.Method == http.MethodPut {
			cr.body = decodeRequestBody(t, r)
		}
		*reqs = append(*reqs, cr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if r.Method == http.MethodGet {
			w.Write(getBody)
		} else {
			w.Write(putBody)
		}
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	var opts []ClientOption
	if hooks != nil {
		opts = append(opts, WithHooks(hooks))
	}
	client := NewClient(cfg, token, opts...)
	return client.ForAccount("99999").Schedules(), reqs
}

// lastPUT returns the body of the final PUT, failing if there was none.
func lastPUT(t *testing.T, reqs *[]capturedScheduleRequest) map[string]any {
	t.Helper()
	for i := len(*reqs) - 1; i >= 0; i-- {
		if (*reqs)[i].method == http.MethodPut {
			return (*reqs)[i].body
		}
	}
	t.Fatalf("expected a PUT, got %+v", *reqs)
	return nil
}

// assertNoPUT is the ordering assertion the read-side guards live or die by: a
// guard that fires after the PUT has already lost the field.
func assertNoPUT(t *testing.T, reqs *[]capturedScheduleRequest) {
	t.Helper()
	for _, r := range *reqs {
		if r.method == http.MethodPut {
			t.Fatalf("expected no PUT before the guard fired, got %+v", r)
		}
	}
}

// idsOf reads a decoded participant_ids array. decodeRequestBody uses
// json.Number, so the elements are not float64.
func idsOf(t *testing.T, value any) []int64 {
	t.Helper()
	arr, ok := value.([]any)
	if !ok {
		t.Fatalf("expected participant_ids to be an array, got %T (%v)", value, value)
	}
	ids := make([]int64, 0, len(arr))
	for _, item := range arr {
		n, ok := item.(json.Number)
		if !ok {
			t.Fatalf("expected a JSON number in participant_ids, got %T (%v)", item, item)
		}
		i, err := n.Int64()
		if err != nil {
			t.Fatalf("participant id %v is not an integer: %v", n, err)
		}
		ids = append(ids, i)
	}
	return ids
}

func strPtr(s string) *string     { return &s }
func boolPtr(b bool) *bool        { return &b }
func idsPtr(ids []int64) *[]int64 { return &ids }

// A summary-only update must carry the four unmentioned full-state fields back
// out. Omitting any of them is a silent erase, not a preserve — and omitting
// all_day would reset the NOT NULL DEFAULT false column, converting an all-day
// entry into a midnight-to-midnight timed one.
func TestSchedulesService_UpdateEntryMergesUnsetFields(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	entry, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
		Summary: strPtr("Kickoff, moved"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", entry.ID)
	}

	if len(*reqs) != 2 {
		t.Fatalf("expected 2 requests (GET then PUT), got %d: %+v", len(*reqs), *reqs)
	}
	if (*reqs)[0].method != http.MethodGet || (*reqs)[1].method != http.MethodPut {
		t.Fatalf("expected GET then PUT, got %s then %s", (*reqs)[0].method, (*reqs)[1].method)
	}

	body := (*reqs)[1].body
	if body["summary"] != "Kickoff, moved" {
		t.Errorf("expected the caller's summary, got %v", body["summary"])
	}
	if body["starts_at"] != fixtureEntryStartsAt {
		t.Errorf("expected preserved starts_at %q, got %v", fixtureEntryStartsAt, body["starts_at"])
	}
	if body["ends_at"] != fixtureEntryEndsAt {
		t.Errorf("expected preserved ends_at %q, got %v", fixtureEntryEndsAt, body["ends_at"])
	}
	if body["description"] != fixtureEntryDescription {
		t.Errorf("expected preserved description, got %v", body["description"])
	}
	allDay, ok := body["all_day"]
	if !ok {
		t.Error("expected all_day present: omitting it resets the column and un-all-days the entry")
	}
	if allDay != false {
		t.Errorf("expected preserved all_day false, got %v", allDay)
	}

	// The three carve-outs BC3 preserves server-side. The read-back carried a
	// populated join link, a true highlight and two participants; none may be
	// echoed, because resending is redundant at best and wrong if the read
	// raced a concurrent change.
	for _, key := range []string{"participant_ids", "url", "highlighted"} {
		if value, ok := body[key]; ok {
			t.Errorf("expected %q absent from an update that did not address it, got %v", key, value)
		}
	}
	if len(body) != 5 {
		t.Errorf("expected exactly the five full-state fields in the body, got %v", body)
	}
}

// The specific way echoing goes wrong: the response spells the join link
// join_url, and its "url" is the entry's own API URL. A composite that seeded
// the request's url member from the response's url would store an API URL as
// the entry's join link.
func TestSchedulesService_UpdateEntryNeverEchoesTheResponseAPIURL(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
		Summary: strPtr("Kickoff"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for key, value := range lastPUT(t, reqs) {
		if value == fixtureEntryAPIURL {
			t.Errorf("the entry's own API URL reached the wire under %q; join_url is the join link, url is not", key)
		}
	}
}

// An explicitly empty value in the carve-out class is an ADDRESS, not an
// absence: [] removes everyone, "" drops the join link, false stops
// highlighting. Every one must survive body compaction, because BC3 preserves
// what the request does not address — a compactor that dropped these would turn
// three clears into three no-ops.
func TestSchedulesService_UpdateEntryClearsCarveOuts(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
		URL:            strPtr(""),
		Highlighted:    boolPtr(false),
		ParticipantIDs: idsPtr([]int64{}),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The three are checked independently, and none of them fatal: they are the
	// same defect three times over, and a run that stopped at the first would
	// hide how wide the hole is.
	assertCarveOutsCleared(t, lastPUT(t, reqs))
}

// assertCarveOutsCleared checks that all three carve-out clears reached the
// wire as present, explicitly-empty values.
func assertCarveOutsCleared(t *testing.T, body map[string]any) {
	t.Helper()
	for _, key := range []string{"url", "highlighted", "participant_ids"} {
		value, ok := body[key]
		if !ok {
			t.Errorf("expected %q present on the wire: an explicit empty value is an address, not an absence — BC3 preserves what the body does not address", key)
			continue
		}
		if value == nil {
			t.Errorf("expected %q to carry an explicit empty value, got JSON null", key)
			continue
		}
		switch key {
		case "url":
			if value != "" {
				t.Errorf("expected url \"\", got %v", value)
			}
		case "highlighted":
			if value != false {
				t.Errorf("expected highlighted false, got %v", value)
			}
		case "participant_ids":
			if got := idsOf(t, value); len(got) != 0 {
				t.Errorf("expected an empty participant_ids array, got %v", got)
			}
		}
	}
}

// Addressing a carve-out applies it normally, and the three are independent
// rather than all-or-nothing: this caller said nothing about participants, so
// participant_ids stays off the wire while url and highlighted go on it.
func TestSchedulesService_UpdateEntryAddressesCarveOutsIndependently(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
		URL:         strPtr("https://meet.example.com/new-room"),
		Highlighted: boolPtr(true),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := lastPUT(t, reqs)
	if body["url"] != "https://meet.example.com/new-room" {
		t.Errorf("expected the caller's join link under url, got %v", body["url"])
	}
	if body["highlighted"] != true {
		t.Errorf("expected highlighted true, got %v", body["highlighted"])
	}
	if value, ok := body["participant_ids"]; ok {
		t.Errorf("expected participant_ids absent, got %v", value)
	}
	// The full state still rides along in full.
	if body["summary"] != fixtureEntrySummary || body["starts_at"] != fixtureEntryStartsAt {
		t.Errorf("expected the fetched full state resent, got %v", body)
	}
}

// The clear DocumentsService.Update cannot express. description is full state,
// so "" is not "unaddressed" here — it reaches the wire as a present, empty key.
// JSON null is out (SPEC §18) and omission would hand the clear back to the
// server's own rebuild, arriving as an accident rather than an intent.
func TestSchedulesService_UpdateEntryClearsDescription(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
		Description: strPtr(""),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := lastPUT(t, reqs)
	description, ok := body["description"]
	if !ok {
		t.Fatal("expected description present in the PUT body, but it was omitted")
	}
	if description == nil {
		t.Fatal("expected description \"\", got JSON null")
	}
	if description != "" {
		t.Errorf("expected description \"\", got %v", description)
	}
	if body["summary"] != fixtureEntrySummary {
		t.Errorf("expected preserved summary, got %v", body["summary"])
	}
}

// all_day false through the composite is a real write — it converts an all-day
// entry into a timed one — so the pointer must carry it rather than collapsing
// it into "unset".
func TestSchedulesService_UpdateEntrySendsExplicitAllDayFalse(t *testing.T) {
	get := patchScheduleEntryFixture(t, scheduleEntryReadBack(t), map[string]any{"all_day": true})
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
		AllDay: boolPtr(false),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := lastPUT(t, reqs)
	value, ok := body["all_day"]
	if !ok {
		t.Fatal("expected all_day present when explicitly set to false")
	}
	if value != false {
		t.Errorf("expected all_day false, got %v", value)
	}
}

func TestSchedulesService_UpdateEntryNilRequestIsUsageError(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	_, err := svc.UpdateEntry(context.Background(), 1069479400, nil)
	if err == nil {
		t.Fatal("expected usage error for a nil update request")
	}
	var usageErr *Error
	if !errors.As(err, &usageErr) || usageErr.Code != CodeUsage {
		t.Fatalf("expected CodeUsage, got %T %v", err, err)
	}
	// Refused before the read-before-write, so not even the GET is spent.
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestSchedulesService_UpdateEntryHooksObserveGetEntryAndReplaceEntry(t *testing.T) {
	get := scheduleEntryReadBack(t)
	recorder := &recordingHooks{}
	svc, _ := testSchedulesCaptureServer(t, get, get, recorder)

	if _, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
		Summary: strPtr("x"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The composite composes the public GetEntry and ReplaceEntry paths, so
	// hooks see the two wire operations under their native identities rather
	// than one synthetic composite.
	ops := make([]string, 0, len(recorder.opStartCalls))
	for _, op := range recorder.opStartCalls {
		ops = append(ops, op.Service+"."+op.Operation)
	}
	if len(ops) != 2 || ops[0] != "Schedules.GetEntry" || ops[1] != "Schedules.ReplaceEntry" {
		t.Errorf("expected operations [Schedules.GetEntry Schedules.ReplaceEntry], got %v", ops)
	}
	if len(recorder.opEndCalls) != 2 {
		t.Errorf("expected 2 OnOperationEnd calls, got %d", len(recorder.opEndCalls))
	}
}

func TestSchedulesService_EditEntrySeedsFullStateFromTheRead(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	entry, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		if f.Summary != fixtureEntrySummary {
			t.Errorf("expected Summary from the GET, got %q", f.Summary)
		}
		if f.StartsAt != fixtureEntryStartsAt {
			t.Errorf("expected StartsAt from the GET, got %q", f.StartsAt)
		}
		if f.Description != fixtureEntryDescription {
			t.Errorf("expected Description from the GET, got %q", f.Description)
		}
		// The carve-out getters expose the read-back so a caller can inspect
		// before deciding. Reading is not writing.
		if f.URL() != fixtureEntryJoinURL {
			t.Errorf("expected URL() to report the read-back join link, got %q", f.URL())
		}
		if !f.Highlighted() {
			t.Error("expected Highlighted() to report the read-back highlight")
		}
		if got := f.ParticipantIDs(); !slices.Equal(got, []int64{1049715914, 1049715915}) {
			t.Errorf("expected ParticipantIDs() to report the read-back participants, got %v", got)
		}
		f.Summary = "🚨 " + f.Summary
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", entry.ID)
	}

	body := lastPUT(t, reqs)
	if body["summary"] != "🚨 "+fixtureEntrySummary {
		t.Errorf("expected the prefixed summary, got %v", body["summary"])
	}
	if body["ends_at"] != fixtureEntryEndsAt {
		t.Errorf("expected preserved ends_at, got %v", body["ends_at"])
	}
}

// The untouched half of the dirty-set contract. The callback read all three
// carve-outs and assigned none, so none may appear in the PUT: the edit view
// cannot simply serialize whatever it was seeded with.
func TestSchedulesService_EditEntryUntouchedCarveOutsStayOffTheWire(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		_, _, _ = f.URL(), f.Highlighted(), f.ParticipantIDs()
		f.Summary = "Team Sync"
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := lastPUT(t, reqs)
	if body["summary"] != "Team Sync" {
		t.Errorf("expected the assigned summary, got %v", body["summary"])
	}
	for _, key := range []string{"participant_ids", "url", "highlighted"} {
		if value, ok := body[key]; ok {
			t.Errorf("expected %q absent after a block that never assigned it, got %v", key, value)
		}
	}
}

// The touched half, and the reason the contract is setter-invocation dirty
// tracking rather than a snapshot diff. The block assigns exactly the join link
// and highlight the GET returned, so a value-comparison implementation would
// conclude nothing changed and omit both — handing the write back to BC3's
// preserve-on-omission. Intent is not recoverable from the value:
// SetURL(f.URL()) is a write.
func TestSchedulesService_EditEntryTouchedCarveOutsAreSentEvenWhenUnchanged(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		f.SetURL(f.URL())
		f.SetHighlighted(f.Highlighted())
		f.SetParticipantIDs(f.ParticipantIDs())
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Checked independently and non-fatally: a value-comparison implementation
	// drops all three, and the proof of that is seeing all three.
	body := lastPUT(t, reqs)
	for _, key := range []string{"url", "highlighted", "participant_ids"} {
		if _, ok := body[key]; !ok {
			t.Errorf("expected %q on the wire: assigning the value the read returned is still a write, and value comparison is explicitly rejected", key)
		}
	}
	if url, ok := body["url"]; ok && url != fixtureEntryJoinURL {
		t.Errorf("expected url %q, got %v", fixtureEntryJoinURL, url)
	}
	if highlighted, ok := body["highlighted"]; ok && highlighted != true {
		t.Errorf("expected highlighted true, got %v", highlighted)
	}
	if ids, ok := body["participant_ids"]; ok {
		if got := idsOf(t, ids); !slices.Equal(got, []int64{1049715914, 1049715915}) {
			t.Errorf("expected the assigned participant ids, got %v", got)
		}
	}
}

// SetParticipantIDs(nil) is the same address as SetParticipantIDs([]int64{}):
// remove everyone. It must serialize as [], never as JSON null — null is not
// the documented clear, and a nil slice is what Go marshals to null.
func TestSchedulesService_EditEntryNilParticipantIDsSerializeAsEmptyArray(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		f.SetParticipantIDs(nil)
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, ok := lastPUT(t, reqs)["participant_ids"]
	if !ok {
		t.Fatal("expected participant_ids present")
	}
	if ids == nil {
		t.Fatal("expected participant_ids [], got JSON null")
	}
	if got := idsOf(t, ids); len(got) != 0 {
		t.Errorf("expected [], got %v", got)
	}
}

// The getter hands back a copy, so mutating it is not a write and does not
// corrupt the seeded state either.
func TestSchedulesService_EditEntryParticipantIDsGetterIsACopy(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		got := f.ParticipantIDs()
		got[0] = 42
		if again := f.ParticipantIDs(); again[0] == 42 {
			t.Error("mutating the returned slice changed the seeded state")
		}
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value, ok := lastPUT(t, reqs)["participant_ids"]; ok {
		t.Errorf("expected participant_ids absent after a getter-only block, got %v", value)
	}
}

func TestSchedulesService_EditEntryCallbackErrorAbortsWithoutPUT(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	wantErr := errors.New("nope")
	_, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		f.Summary = "should never be written"
		f.SetURL("")
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the callback error, got %v", err)
	}
	assertNoPUT(t, reqs)
}

func TestSchedulesService_EditEntryNilCallbackIsUsageError(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	_, err := svc.EditEntry(context.Background(), 1069479400, nil)
	if err == nil {
		t.Fatal("expected usage error for a nil edit callback")
	}
	var usageErr *Error
	if !errors.As(err, &usageErr) || usageErr.Code != CodeUsage {
		t.Fatalf("expected CodeUsage, got %T %v", err, err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestSchedulesService_EditEntryHooksObserveGetEntryAndReplaceEntry(t *testing.T) {
	get := scheduleEntryReadBack(t)
	recorder := &recordingHooks{}
	svc, _ := testSchedulesCaptureServer(t, get, get, recorder)

	if _, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ops := make([]string, 0, len(recorder.opStartCalls))
	for _, op := range recorder.opStartCalls {
		ops = append(ops, op.Service+"."+op.Operation)
	}
	if len(ops) != 2 || ops[0] != "Schedules.GetEntry" || ops[1] != "Schedules.ReplaceEntry" {
		t.Errorf("expected operations [Schedules.GetEntry Schedules.ReplaceEntry], got %v", ops)
	}
}

// bc3 renders starts_at_date_or_time, which is starts_at.to_date unless the
// entry is timed, so an all-day entry reads back as a BARE DATE. The composites
// carry that string through untouched.
//
// The trap this pins: ScheduleEntry.StartsAt is types.FlexibleTime, whose
// UnmarshalJSON accepts the bare date by treating it as midnight UTC, and whose
// MarshalJSON then renders time.Time's RFC3339 form. Round-tripping the DECODED
// value would therefore rewrite "2026-06-01" into "2026-06-01T00:00:00Z", which
// BC3 re-parses in the account's own zone — west of UTC that lands on the
// previous day and moves the entry. ScheduleEntryFields carries strings for
// exactly this reason, sourced from the raw response bytes rather than the
// decoded time.
func TestSchedulesService_EditEntryRoundTripsAnAllDayBareDate(t *testing.T) {
	get := patchScheduleEntryFixture(t, scheduleEntryReadBack(t), map[string]any{
		"all_day":   true,
		"starts_at": "2026-06-01",
		"ends_at":   "2026-06-03",
	})
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		if f.StartsAt != "2026-06-01" {
			t.Errorf("expected the bare date verbatim, got %q", f.StartsAt)
		}
		f.Summary = "Offsite"
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := lastPUT(t, reqs)
	if body["starts_at"] != "2026-06-01" {
		t.Errorf("expected starts_at \"2026-06-01\" verbatim, got %v", body["starts_at"])
	}
	if body["ends_at"] != "2026-06-03" {
		t.Errorf("expected ends_at \"2026-06-03\" verbatim, got %v", body["ends_at"])
	}
	if body["all_day"] != true {
		t.Errorf("expected all_day true, got %v", body["all_day"])
	}
}

// The other half of that finding, stated directly: this is what the composite
// would have sent had it re-rendered the decoded value. If FlexibleTime ever
// learns to preserve its source text, this test is the thing that says so.
func TestFlexibleTimeMarshalRewritesABareDate(t *testing.T) {
	var entry ScheduleEntry
	if err := json.Unmarshal([]byte(`{"starts_at":"2026-06-01","all_day":true}`), &entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := json.Marshal(entry.StartsAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) == `"2026-06-01"` {
		t.Skip("FlexibleTime now round-trips its source text; ScheduleEntryFields could carry the decoded value")
	}
	if string(out) != `"2026-06-01T00:00:00Z"` {
		t.Errorf("expected the midnight-UTC rewrite %q, got %s", "2026-06-01T00:00:00Z", out)
	}
}

func TestSchedulesService_ReplaceEntryIsExactlyOneRequest(t *testing.T) {
	get := scheduleEntryReadBack(t)
	recorder := &recordingHooks{}
	svc, reqs := testSchedulesCaptureServer(t, get, get, recorder)

	entry, err := svc.ReplaceEntry(context.Background(), 1069479400, &ReplaceScheduleEntryRequest{
		Summary:  strPtr("Team Meeting"),
		StartsAt: strPtr("2026-06-05T06:00:00Z"),
		EndsAt:   strPtr("2026-06-05T08:30:00Z"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", entry.ID)
	}

	// No GET: replace is the server-native verbatim PUT.
	if len(*reqs) != 1 || (*reqs)[0].method != http.MethodPut {
		t.Fatalf("expected exactly one PUT, got %+v", *reqs)
	}
	body := (*reqs)[0].body
	if body["summary"] != "Team Meeting" {
		t.Errorf("expected the summary sent verbatim, got %v", body["summary"])
	}
	// Unaddressed carve-outs must not appear at all. A compactor that emitted
	// null, or a default that emitted [] or false, would clear the value BC3 is
	// holding for us.
	for _, key := range []string{"participant_ids", "url", "highlighted", "description", "all_day", "notify"} {
		if value, ok := body[key]; ok {
			t.Errorf("expected %q omitted from a sparse replace, got %v", key, value)
		}
	}
	if len(body) != 3 {
		t.Errorf("expected exactly {summary, starts_at, ends_at}, got %v", body)
	}

	if len(recorder.opStartCalls) != 1 ||
		recorder.opStartCalls[0].Service != "Schedules" || recorder.opStartCalls[0].Operation != "ReplaceEntry" {
		t.Errorf("expected a single Schedules.ReplaceEntry operation, got %+v", recorder.opStartCalls)
	}
}

func TestSchedulesService_ReplaceEntrySendsExplicitEmptyCarveOuts(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	if _, err := svc.ReplaceEntry(context.Background(), 1069479400, &ReplaceScheduleEntryRequest{
		Summary:        strPtr("Team Meeting"),
		StartsAt:       strPtr("2026-06-05T06:00:00Z"),
		EndsAt:         strPtr("2026-06-05T08:30:00Z"),
		ParticipantIDs: idsPtr([]int64{}),
		URL:            strPtr(""),
		Highlighted:    boolPtr(false),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[0].body
	if url, ok := body["url"]; !ok || url != "" {
		t.Errorf("expected url \"\" present, got %v (present=%v)", url, ok)
	}
	if highlighted, ok := body["highlighted"]; !ok || highlighted != false {
		t.Errorf("expected highlighted false present, got %v (present=%v)", highlighted, ok)
	}
	ids, ok := body["participant_ids"]
	if !ok {
		t.Fatal("expected participant_ids [] present")
	}
	if got := idsOf(t, ids); len(got) != 0 {
		t.Errorf("expected [], got %v", got)
	}
}

// starts_at and ends_at are @required and are NOT on BC3's preserve list, and
// both columns are NOT NULL — so a body omitting either cannot succeed. Refuse
// it locally rather than spend a round-trip discovering that.
func TestSchedulesService_ReplaceEntryRequiresTheTimes(t *testing.T) {
	get := scheduleEntryReadBack(t)
	for _, tc := range []struct {
		name string
		req  *ReplaceScheduleEntryRequest
	}{
		{"no starts_at", &ReplaceScheduleEntryRequest{EndsAt: strPtr("2026-06-05T08:30:00Z")}},
		{"no ends_at", &ReplaceScheduleEntryRequest{StartsAt: strPtr("2026-06-05T06:00:00Z")}},
		{"neither", &ReplaceScheduleEntryRequest{Summary: strPtr("Team Meeting")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recordingHooks{}
			svc, reqs := testSchedulesCaptureServer(t, get, get, recorder)

			_, err := svc.ReplaceEntry(context.Background(), 1069479400, tc.req)
			if err == nil {
				t.Fatal("expected a usage error, but the call succeeded")
			}
			var usageErr *Error
			if !errors.As(err, &usageErr) || usageErr.Code != CodeUsage {
				t.Fatalf("expected CodeUsage, got %T %v", err, err)
			}
			if len(*reqs) != 0 {
				t.Fatalf("expected no requests, got %+v", *reqs)
			}
			// The body is built inside the hook envelope, so the refusal is
			// observable.
			if len(recorder.opStartCalls) != 1 || len(recorder.opEndCalls) != 1 {
				t.Errorf("expected the usage error to be observable to hooks, got %d starts / %d ends",
					len(recorder.opStartCalls), len(recorder.opEndCalls))
			}
		})
	}
}

func TestSchedulesService_ReplaceEntryNilRequestIsUsageError(t *testing.T) {
	get := scheduleEntryReadBack(t)
	svc, reqs := testSchedulesCaptureServer(t, get, get, nil)

	_, err := svc.ReplaceEntry(context.Background(), 1069479400, nil)
	if err == nil {
		t.Fatal("expected usage error for a nil replace request")
	}
	var usageErr *Error
	if !errors.As(err, &usageErr) || usageErr.Code != CodeUsage {
		t.Fatalf("expected CodeUsage, got %T %v", err, err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

// The read-side guards. summary, starts_at, ends_at and all_day are @required on
// the response, and the generated model carries all four as VALUES, so an absent
// key decodes to the zero value and encoding/json says nothing — the one shape a
// typed decoder does not catch. Writing that zero value back on a full-replace
// endpoint is the defect: a missing all_day would un-all-day the entry, a
// missing starts_at would erase its bounds, a missing summary would blank it.
//
// The assertion that matters is the ORDERING: no PUT. A guard that fires after
// the PUT has already lost the field.
func TestSchedulesService_UpdateEntryRefusesAMalformedReadBeforeWriting(t *testing.T) {
	base := scheduleEntryReadBack(t)
	for _, tc := range []struct {
		name  string
		patch map[string]any
	}{
		{"summary absent", map[string]any{"summary": nil}},
		{"summary empty", map[string]any{"summary": ""}},
		{"summary whitespace", map[string]any{"summary": "   "}},
		{"all_day absent", map[string]any{"all_day": nil}},
		{"starts_at absent", map[string]any{"starts_at": nil}},
		{"ends_at absent", map[string]any{"ends_at": nil}},
		{"starts_at empty", map[string]any{"starts_at": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			get := patchScheduleEntryFixture(t, base, tc.patch)
			svc, reqs := testSchedulesCaptureServer(t, get, base, nil)

			_, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
				Description: strPtr("<div>New agenda.</div>"),
			})
			if err == nil {
				t.Fatal("expected the call to fail, but it succeeded")
			}
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *Error, got %T: %v", err, err)
			}
			// api_error, not usage: the value arrived in a successful API
			// response, so nothing the caller passed is at fault.
			if apiErr.Code != CodeAPI {
				t.Errorf("expected code %q, got %q", CodeAPI, apiErr.Code)
			}
			if apiErr.HTTPStatus != 0 {
				t.Errorf("expected a statusless error, got HTTP %d", apiErr.HTTPStatus)
			}
			if apiErr.Retryable {
				t.Error("re-requesting cannot repair a malformed body")
			}
			if apiErr.Hint == "" {
				t.Error("expected a hint naming the deliberate-overwrite escape hatch")
			}
			assertNoPUT(t, reqs)
		})
	}
}

// A JSON null on an @required field is malformed for the same reason absence is,
// and it takes a separate path: encoding/json decodes null into a value-typed
// bool or FlexibleTime as the zero value without complaint.
func TestSchedulesService_UpdateEntryRefusesNullRequiredFields(t *testing.T) {
	base := scheduleEntryReadBack(t)
	rawNull := json.RawMessage("null")
	for _, key := range []string{"all_day", "starts_at", "ends_at"} {
		t.Run(key, func(t *testing.T) {
			get := patchScheduleEntryFixture(t, base, map[string]any{key: rawNull})
			svc, reqs := testSchedulesCaptureServer(t, get, base, nil)

			_, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
				Summary: strPtr("New summary"),
			})
			if err == nil {
				t.Fatal("expected the call to fail, but it succeeded")
			}
			var apiErr *Error
			if !errors.As(err, &apiErr) || apiErr.Code != CodeAPI {
				t.Fatalf("expected an api_error, got %T: %v", err, err)
			}
			assertNoPUT(t, reqs)
		})
	}
}

func TestSchedulesService_EditEntryRefusesAMalformedReadBeforeTheCallback(t *testing.T) {
	base := scheduleEntryReadBack(t)
	get := patchScheduleEntryFixture(t, base, map[string]any{"all_day": nil})
	svc, reqs := testSchedulesCaptureServer(t, get, base, nil)

	called := false
	_, err := svc.EditEntry(context.Background(), 1069479400, func(f *ScheduleEntryFields) error {
		called = true
		f.Description = "<div>New agenda.</div>"
		return nil
	})
	if err == nil {
		t.Fatal("expected the call to fail, but it succeeded")
	}
	if called {
		t.Error("the callback must not run on a malformed read")
	}
	assertNoPUT(t, reqs)
}

// The wrong-TYPED shapes, which encoding/json DOES catch — but reports as a raw
// decoder error, not the shape SPEC §6 defines for a malformed 2xx body. A
// caller switching on *Error would miss it entirely and it carries no hint. The
// composite normalizes it the way the Swift one normalizes DecodingError, so a
// malformed response looks the same in every SDK.
//
// The table spans several decoder error types on purpose, because
// getEntryWithBody classifies by ORIGIN rather than by an allowlist: created_at
// is time.Time, whose UnmarshalJSON returns *time.ParseError rather than an
// encoding/json sentinel, and a non-integral attachment dimension is rejected by
// types.FlexInt with a plain fmt.Errorf that is no named type at all.
func TestSchedulesService_UpdateEntryWrapsADecodeFailureAsStatuslessAPIError(t *testing.T) {
	base := scheduleEntryReadBack(t)
	for _, tc := range []struct {
		name  string
		patch map[string]any
	}{
		{"description is an object", map[string]any{"description": map[string]any{"a": 1}}},
		{"summary is a number", map[string]any{"summary": 42}},
		{"all_day is a string", map[string]any{"all_day": "yes"}},
		{"starts_at is unparseable", map[string]any{"starts_at": "the ides of March"}},
		{"created_at is not a timestamp", map[string]any{"created_at": "not-a-timestamp"}},
		{"attachment height is non-integral", map[string]any{
			"description_attachments": []any{map[string]any{"height": 1024.5}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			get := patchScheduleEntryFixture(t, base, tc.patch)
			svc, reqs := testSchedulesCaptureServer(t, get, base, nil)

			_, err := svc.UpdateEntry(context.Background(), 1069479400, &UpdateScheduleEntryRequest{
				Summary: strPtr("Q3 Kickoff"),
			})
			if err == nil {
				t.Fatal("expected the call to fail, but it succeeded")
			}
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *Error, got %T: %v", err, err)
			}
			if apiErr.Code != CodeAPI {
				t.Errorf("expected code %q, got %q", CodeAPI, apiErr.Code)
			}
			if apiErr.HTTPStatus != 0 {
				t.Errorf("expected a statusless error, got HTTP %d", apiErr.HTTPStatus)
			}
			if apiErr.Retryable {
				t.Error("re-requesting cannot repair a malformed body")
			}
			if apiErr.Hint == "" {
				t.Error("expected a hint naming the deliberate-overwrite escape hatch")
			}
			assertNoPUT(t, reqs)
		})
	}
}

// A transport or HTTP error must pass through untouched — the wrapper is for
// decode failures only, and swallowing everything would hide a 404 behind a
// "does not decode" message.
func TestSchedulesService_UpdateEntryPassesNonDecodeErrorsThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"Not Found"}`)
	}))
	t.Cleanup(srv.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	_, err := client.ForAccount("999").Schedules().UpdateEntry(context.Background(), 1069479400,
		&UpdateScheduleEntryRequest{Summary: strPtr("Q3 Kickoff")})
	if err == nil {
		t.Fatal("expected the call to fail, but it succeeded")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.HTTPStatus != http.StatusNotFound {
		t.Errorf("expected the 404 to survive, got HTTP %d (%s)", apiErr.HTTPStatus, apiErr.Message)
	}
}

func TestSchedulesService_CreateEntryPartial(t *testing.T) {
	fixture := loadSchedulesFixture(t, "entry_get.json")
	var receivedBody map[string]any
	svc := testSchedulesServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(fixture)
	})

	_, err := svc.CreateEntry(context.Background(), 12345, &CreateScheduleEntryRequest{
		Summary:  "Meeting",
		StartsAt: "2024-01-15T09:00:00Z",
		EndsAt:   "2024-01-15T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Notify should NOT be present when false (not explicitly requested)
	if _, ok := receivedBody["notify"]; ok {
		t.Errorf("expected notify to be omitted when not set, but it was present: %v", receivedBody["notify"])
	}
}

// TestSchedulesService_CreateEntryVisibleToClients verifies the tri-state
// visible_to_clients flag reaches the wire correctly on create: nil omits the
// key, true is sent verbatim, and an explicit false is sent (not dropped).
func TestSchedulesService_CreateEntryVisibleToClients(t *testing.T) {
	fixture := loadSchedulesFixture(t, "entry_get.json")
	tru, fls := true, false
	cases := []struct {
		name    string
		value   *bool
		present bool
		want    bool
	}{
		{"nil omits the field", nil, false, false},
		{"true is sent", &tru, true, true},
		{"explicit false is sent, not dropped", &fls, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedBody map[string]any
			svc := testSchedulesServer(t, func(w http.ResponseWriter, r *http.Request) {
				receivedBody = decodeRequestBody(t, r)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				w.Write(fixture)
			})

			_, err := svc.CreateEntry(context.Background(), 12345, &CreateScheduleEntryRequest{
				Summary:          "Meeting",
				StartsAt:         "2024-01-15T09:00:00Z",
				EndsAt:           "2024-01-15T10:00:00Z",
				VisibleToClients: tc.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			val, ok := receivedBody["visible_to_clients"]
			if ok != tc.present {
				t.Fatalf("visible_to_clients present=%v, want %v (body=%v)", ok, tc.present, receivedBody)
			}
			if tc.present && val != tc.want {
				t.Errorf("visible_to_clients=%v, want %v", val, tc.want)
			}
		})
	}
}

// The three members #641 added to CreateScheduleEntry must reach the wire from
// the PUBLIC request type, not merely from the generated one. The spec, the
// generated client and five other SDKs all carried them while
// CreateScheduleEntryRequest did not, so a Go caller could not use the
// functionality this closes — and no marshal test of the public struct would
// have noticed, because the gap was in the struct-to-generated-body threading.
// This asserts the observed request body.
func TestSchedulesService_CreateEntryJoinLinkHighlightAndStatus(t *testing.T) {
	fixture := loadSchedulesFixture(t, "entry_get.json")
	joinURL, drafted, highlighted := "https://zoom.us/j/999", "drafted", true

	var receivedBody map[string]any
	svc := testSchedulesServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(fixture)
	})

	_, err := svc.CreateEntry(context.Background(), 12345, &CreateScheduleEntryRequest{
		Summary:     "Kickoff call",
		StartsAt:    "2024-01-15T09:00:00Z",
		EndsAt:      "2024-01-15T10:00:00Z",
		URL:         &joinURL,
		Highlighted: &highlighted,
		Status:      &drafted,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The write spelling is `url`. BC3 strong-params drop `join_url` on write,
	// so sending it under the read spelling would be a silent no-op: a 201 with
	// no join link.
	if got := receivedBody["url"]; got != joinURL {
		t.Errorf("expected url=%q on the wire, got %v (body=%v)", joinURL, got, receivedBody)
	}
	if _, ok := receivedBody["join_url"]; ok {
		t.Error("join_url must not appear on the write path — BC3 discards it")
	}
	if got := receivedBody["highlighted"]; got != true {
		t.Errorf("expected highlighted=true, got %v", got)
	}
	if got := receivedBody["status"]; got != "drafted" {
		t.Errorf("expected status=drafted, got %v", got)
	}
}

// Unset means absent, not a zero value on the wire. `highlighted` matters most:
// schedule_entries.highlighted is NOT NULL, so an explicit null would make BC3
// raise rather than apply its false default.
func TestSchedulesService_CreateEntryOmitsUnsetJoinLinkHighlightAndStatus(t *testing.T) {
	fixture := loadSchedulesFixture(t, "entry_get.json")

	var receivedBody map[string]any
	svc := testSchedulesServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(fixture)
	})

	_, err := svc.CreateEntry(context.Background(), 12345, &CreateScheduleEntryRequest{
		Summary:  "Kickoff call",
		StartsAt: "2024-01-15T09:00:00Z",
		EndsAt:   "2024-01-15T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, key := range []string{"url", "highlighted", "status"} {
		if _, ok := receivedBody[key]; ok {
			t.Errorf("expected %q to be omitted when unset, got %v", key, receivedBody[key])
		}
	}
}
