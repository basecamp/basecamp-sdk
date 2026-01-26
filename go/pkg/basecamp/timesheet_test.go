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

func TestTimesheetReportOptions_BuildQueryParams(t *testing.T) {
	service := &TimesheetService{}

	tests := []struct {
		name     string
		opts     *TimesheetReportOptions
		expected string
	}{
		{
			name:     "nil options",
			opts:     nil,
			expected: "",
		},
		{
			name:     "empty options",
			opts:     &TimesheetReportOptions{},
			expected: "",
		},
		{
			name: "from only",
			opts: &TimesheetReportOptions{
				From: "2024-01-01",
			},
			expected: "?from=2024-01-01",
		},
		{
			name: "to only",
			opts: &TimesheetReportOptions{
				To: "2024-01-31",
			},
			expected: "?to=2024-01-31",
		},
		{
			name: "person_id only",
			opts: &TimesheetReportOptions{
				PersonID: 1049715914,
			},
			expected: "?person_id=1049715914",
		},
		{
			name: "from and to",
			opts: &TimesheetReportOptions{
				From: "2024-01-01",
				To:   "2024-01-31",
			},
			expected: "?from=2024-01-01&to=2024-01-31",
		},
		{
			name: "all options",
			opts: &TimesheetReportOptions{
				From:     "2024-01-01",
				To:       "2024-01-31",
				PersonID: 1049715914,
			},
			expected: "?from=2024-01-01&person_id=1049715914&to=2024-01-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.buildQueryParams(tt.opts)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
