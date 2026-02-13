package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func timesheetFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "timesheet")
}

func loadTimesheetFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(timesheetFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTimesheetEntry_UnmarshalReport(t *testing.T) {
	data := loadTimesheetFixture(t, "report.json")

	var entries []TimesheetEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal report.json: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Verify first entry
	e1 := entries[0]
	if e1.ID != 9007199254741001 {
		t.Errorf("expected ID 9007199254741001, got %d", e1.ID)
	}
	if e1.Date != "2024-01-15" {
		t.Errorf("expected date '2024-01-15', got %q", e1.Date)
	}
	if e1.Hours != "2.5" {
		t.Errorf("expected hours '2.5', got %q", e1.Hours)
	}
	if e1.Description != "Worked on project setup and initial planning" {
		t.Errorf("unexpected description: %q", e1.Description)
	}
	if e1.BillableStatus != "billable" {
		t.Errorf("expected billable_status 'billable', got %q", e1.BillableStatus)
	}

	// Verify creator
	if e1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if e1.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", e1.Creator.ID)
	}
	if e1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", e1.Creator.Name)
	}

	// Verify parent (todo)
	if e1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if e1.Parent.ID != 1069479345 {
		t.Errorf("expected Parent.ID 1069479345, got %d", e1.Parent.ID)
	}
	if e1.Parent.Title != "Design homepage mockups" {
		t.Errorf("expected Parent.Title 'Design homepage mockups', got %q", e1.Parent.Title)
	}
	if e1.Parent.Type != "Todo" {
		t.Errorf("expected Parent.Type 'Todo', got %q", e1.Parent.Type)
	}

	// Verify bucket
	if e1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if e1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", e1.Bucket.ID)
	}
	if e1.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", e1.Bucket.Name)
	}

	// Verify timestamps are parsed
	if e1.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if e1.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify second entry with different creator
	e2 := entries[1]
	if e2.ID != 9007199254741002 {
		t.Errorf("expected ID 9007199254741002, got %d", e2.ID)
	}
	if e2.Hours != "4.0" {
		t.Errorf("expected hours '4.0', got %q", e2.Hours)
	}
	if e2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second entry")
	}
	if e2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", e2.Creator.Name)
	}
	// Verify creator with company
	if e2.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil for second entry")
	}
	if e2.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", e2.Creator.Company.Name)
	}

	// Verify third entry has non_billable status
	e3 := entries[2]
	if e3.BillableStatus != "non_billable" {
		t.Errorf("expected billable_status 'non_billable', got %q", e3.BillableStatus)
	}
	// Third entry is for a different project
	if e3.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil for third entry")
	}
	if e3.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", e3.Bucket.ID)
	}
}

func TestTimesheetEntry_UnmarshalProjectReport(t *testing.T) {
	data := loadTimesheetFixture(t, "project_report.json")

	var entries []TimesheetEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal project_report.json: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// All entries should be for the same project
	for i, e := range entries {
		if e.Bucket == nil {
			t.Fatalf("entry %d: expected Bucket to be non-nil", i)
		}
		if e.Bucket.ID != 2085958499 {
			t.Errorf("entry %d: expected Bucket.ID 2085958499, got %d", i, e.Bucket.ID)
		}
	}
}

func TestTimesheetEntry_UnmarshalRecordingReport(t *testing.T) {
	data := loadTimesheetFixture(t, "recording_report.json")

	var entries []TimesheetEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal recording_report.json: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	// Verify the single entry
	e := entries[0]
	if e.ID != 9007199254741001 {
		t.Errorf("expected ID 9007199254741001, got %d", e.ID)
	}
	if e.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if e.Parent.ID != 1069479345 {
		t.Errorf("expected Parent.ID 1069479345, got %d", e.Parent.ID)
	}
}

func TestTimesheetEntry_UnmarshalGet(t *testing.T) {
	data := loadTimesheetFixture(t, "get.json")

	var entry TimesheetEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if entry.ID != 9007199254741099 {
		t.Errorf("expected ID 9007199254741099, got %d", entry.ID)
	}
	if entry.Date != "2024-05-16" {
		t.Errorf("expected date '2024-05-16', got %q", entry.Date)
	}
	if entry.Hours != "1:30" {
		t.Errorf("expected hours '1:30', got %q", entry.Hours)
	}
	if entry.Description != "Client meeting prep" {
		t.Errorf("expected description 'Client meeting prep', got %q", entry.Description)
	}
	if entry.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if entry.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", entry.Creator.Name)
	}
	if entry.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if entry.Parent.Type != "Todo" {
		t.Errorf("expected Parent.Type 'Todo', got %q", entry.Parent.Type)
	}
	if entry.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if entry.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", entry.Bucket.Name)
	}
	if entry.Person == nil {
		t.Fatal("expected Person to be non-nil")
	}
	if entry.Person.ID != 1049715914 {
		t.Errorf("expected Person.ID 1049715914, got %d", entry.Person.ID)
	}
	if entry.Person.Name != "Victor Cooper" {
		t.Errorf("expected Person.Name 'Victor Cooper', got %q", entry.Person.Name)
	}
}

func TestCreateTimesheetEntryRequest_Marshal(t *testing.T) {
	req := CreateTimesheetEntryRequest{
		Date:        "2024-05-16",
		Hours:       "1:30",
		Description: "Client meeting prep",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTimesheetEntryRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["date"] != "2024-05-16" {
		t.Errorf("expected date '2024-05-16', got %v", data["date"])
	}
	if data["hours"] != "1:30" {
		t.Errorf("expected hours '1:30', got %v", data["hours"])
	}
	if data["description"] != "Client meeting prep" {
		t.Errorf("expected description 'Client meeting prep', got %v", data["description"])
	}
}

func TestCreateTimesheetEntryRequest_MarshalMinimal(t *testing.T) {
	req := CreateTimesheetEntryRequest{
		Date:  "2024-05-16",
		Hours: "2.0",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTimesheetEntryRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["date"] != "2024-05-16" {
		t.Errorf("expected date '2024-05-16', got %v", data["date"])
	}
	if data["hours"] != "2.0" {
		t.Errorf("expected hours '2.0', got %v", data["hours"])
	}
	if _, ok := data["description"]; ok {
		t.Error("expected description to be omitted")
	}
	if _, ok := data["person_id"]; ok {
		t.Error("expected person_id to be omitted")
	}
}

func TestCreateTimesheetEntryRequest_FromFixture(t *testing.T) {
	data := loadTimesheetFixture(t, "create-request.json")

	var req CreateTimesheetEntryRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Date != "2024-05-16" {
		t.Errorf("expected date '2024-05-16', got %q", req.Date)
	}
	if req.Hours != "1:30" {
		t.Errorf("expected hours '1:30', got %q", req.Hours)
	}
	if req.Description != "Client meeting prep" {
		t.Errorf("expected description 'Client meeting prep', got %q", req.Description)
	}
}

func TestTimesheetEntry_UnmarshalCreateResponse(t *testing.T) {
	data := loadTimesheetFixture(t, "create-response.json")

	var entry TimesheetEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal create-response.json: %v", err)
	}

	if entry.ID != 9007199254741099 {
		t.Errorf("expected ID 9007199254741099, got %d", entry.ID)
	}
	if entry.Date != "2024-05-16" {
		t.Errorf("expected date '2024-05-16', got %q", entry.Date)
	}
	if entry.Hours != "1:30" {
		t.Errorf("expected hours '1:30', got %q", entry.Hours)
	}
	if entry.Description != "Client meeting prep" {
		t.Errorf("expected description 'Client meeting prep', got %q", entry.Description)
	}
	if entry.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if entry.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", entry.Creator.ID)
	}
	if entry.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if entry.Parent.ID != 1069479345 {
		t.Errorf("expected Parent.ID 1069479345, got %d", entry.Parent.ID)
	}
	if entry.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if entry.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", entry.Bucket.ID)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if entry.Person == nil {
		t.Fatal("expected Person to be non-nil")
	}
	if entry.Person.ID != 1049715914 {
		t.Errorf("expected Person.ID 1049715914, got %d", entry.Person.ID)
	}
}

func TestUpdateTimesheetEntryRequest_Marshal(t *testing.T) {
	req := UpdateTimesheetEntryRequest{
		Hours:       "2.5",
		Description: "Updated description",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateTimesheetEntryRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["hours"] != "2.5" {
		t.Errorf("expected hours '2.5', got %v", data["hours"])
	}
	if data["description"] != "Updated description" {
		t.Errorf("expected description 'Updated description', got %v", data["description"])
	}
	// date and person_id should be omitted
	if _, ok := data["date"]; ok {
		t.Error("expected date to be omitted")
	}
	if _, ok := data["person_id"]; ok {
		t.Error("expected person_id to be omitted")
	}
}

func TestUpdateTimesheetEntryRequest_FromFixture(t *testing.T) {
	data := loadTimesheetFixture(t, "update-request.json")

	var req UpdateTimesheetEntryRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Hours != "2.5" {
		t.Errorf("expected hours '2.5', got %q", req.Hours)
	}
	if req.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %q", req.Description)
	}
}

func TestTimesheetEntry_UnmarshalUpdateResponse(t *testing.T) {
	data := loadTimesheetFixture(t, "update-response.json")

	var entry TimesheetEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to unmarshal update-response.json: %v", err)
	}

	if entry.ID != 9007199254741099 {
		t.Errorf("expected ID 9007199254741099, got %d", entry.ID)
	}
	if entry.Hours != "2.5" {
		t.Errorf("expected hours '2.5', got %q", entry.Hours)
	}
	if entry.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %q", entry.Description)
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
	if entry.Person == nil {
		t.Fatal("expected Person to be non-nil")
	}
	if entry.Person.ID != 1049715914 {
		t.Errorf("expected Person.ID 1049715914, got %d", entry.Person.ID)
	}
}

func TestTimesheetReportOptions_BuildTimesheetParams(t *testing.T) {
	service := &TimesheetService{}

	tests := []struct {
		name         string
		opts         *TimesheetReportOptions
		expectNil    bool
		expectedFrom string
		expectedTo   string
		expectedPID  int64
	}{
		{
			name:      "nil options",
			opts:      nil,
			expectNil: true,
		},
		{
			name:         "from only",
			opts:         &TimesheetReportOptions{From: "2024-01-01"},
			expectedFrom: "2024-01-01",
		},
		{
			name:       "to only",
			opts:       &TimesheetReportOptions{To: "2024-01-31"},
			expectedTo: "2024-01-31",
		},
		{
			name:        "person_id only",
			opts:        &TimesheetReportOptions{PersonID: 1049715914},
			expectedPID: 1049715914,
		},
		{
			name:         "all options",
			opts:         &TimesheetReportOptions{From: "2024-01-01", To: "2024-01-31", PersonID: 1049715914},
			expectedFrom: "2024-01-01",
			expectedTo:   "2024-01-31",
			expectedPID:  1049715914,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildTimesheetParams(tt.opts)
			if tt.expectNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil params")
			}
			if result.From != tt.expectedFrom {
				t.Errorf("expected From %q, got %q", tt.expectedFrom, result.From)
			}
			if result.To != tt.expectedTo {
				t.Errorf("expected To %q, got %q", tt.expectedTo, result.To)
			}
			if result.PersonId != tt.expectedPID {
				t.Errorf("expected PersonId %d, got %d", tt.expectedPID, result.PersonId)
			}
		})
	}
}
