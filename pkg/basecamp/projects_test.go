package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fixturesDir returns the path to the fixtures directory.
func fixturesDir() string {
	return filepath.Join("..", "..", "spec", "fixtures", "projects")
}

// loadFixture reads a fixture file and returns its contents.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(fixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestProject_UnmarshalList(t *testing.T) {
	data := loadFixture(t, "list.json")

	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}

	// Verify first project (basic, no client fields)
	p1 := projects[0]
	if p1.ID != 2085958499 {
		t.Errorf("expected ID 2085958499, got %d", p1.ID)
	}
	if p1.Name != "The Leto Laptop" {
		t.Errorf("expected name 'The Leto Laptop', got %q", p1.Name)
	}
	if p1.Status != "active" {
		t.Errorf("expected status 'active', got %q", p1.Status)
	}
	if p1.Purpose != "topic" {
		t.Errorf("expected purpose 'topic', got %q", p1.Purpose)
	}
	if p1.ClientCompany != nil {
		t.Errorf("expected nil ClientCompany for first project")
	}
	if len(p1.Dock) != 8 {
		t.Errorf("expected 8 dock items, got %d", len(p1.Dock))
	}

	// Verify second project (has client_company and clientside)
	p2 := projects[1]
	if p2.ID != 2085958500 {
		t.Errorf("expected ID 2085958500, got %d", p2.ID)
	}
	if p2.ClientCompany == nil {
		t.Fatal("expected ClientCompany for second project")
	}
	if p2.ClientCompany.ID != 1033447818 {
		t.Errorf("expected ClientCompany.ID 1033447818, got %d", p2.ClientCompany.ID)
	}
	if p2.ClientCompany.Name != "Leto Brand" {
		t.Errorf("expected ClientCompany.Name 'Leto Brand', got %q", p2.ClientCompany.Name)
	}
	if p2.Clientside == nil {
		t.Fatal("expected Clientside for second project")
	}
	if p2.Clientside.URL == "" {
		t.Error("expected non-empty Clientside.URL")
	}
}

func TestProject_UnmarshalGet(t *testing.T) {
	data := loadFixture(t, "get.json")

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if project.ID != 2085958499 {
		t.Errorf("expected ID 2085958499, got %d", project.ID)
	}
	if project.Name != "The Leto Laptop" {
		t.Errorf("expected name 'The Leto Laptop', got %q", project.Name)
	}
	if project.Description != "Laptop product launch." {
		t.Errorf("expected description 'Laptop product launch.', got %q", project.Description)
	}
	if project.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if project.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestDockItem_Unmarshal(t *testing.T) {
	data := loadFixture(t, "get.json")

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if len(project.Dock) != 8 {
		t.Fatalf("expected 8 dock items, got %d", len(project.Dock))
	}

	// Check enabled dock item (Message Board)
	mb := project.Dock[0]
	if mb.Name != "message_board" {
		t.Errorf("expected name 'message_board', got %q", mb.Name)
	}
	if mb.Title != "Message Board" {
		t.Errorf("expected title 'Message Board', got %q", mb.Title)
	}
	if !mb.Enabled {
		t.Error("expected Message Board to be enabled")
	}
	if mb.Position == nil || *mb.Position != 1 {
		t.Errorf("expected position 1, got %v", mb.Position)
	}

	// Check disabled dock item (Questionnaire)
	q := project.Dock[5]
	if q.Name != "questionnaire" {
		t.Errorf("expected name 'questionnaire', got %q", q.Name)
	}
	if q.Enabled {
		t.Error("expected Questionnaire to be disabled")
	}
	// Position is null in JSON, should be nil in Go
	if q.Position != nil {
		t.Errorf("expected nil position for disabled item, got %d", *q.Position)
	}
}

func TestCreateProjectRequest_Marshal(t *testing.T) {
	data := loadFixture(t, "create-request.json")

	// Unmarshal fixture to verify it matches our struct
	var req CreateProjectRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Name != "Marketing Campaign" {
		t.Errorf("expected name 'Marketing Campaign', got %q", req.Name)
	}
	if req.Description != "For Client: Xyz Corp Conference" {
		t.Errorf("expected description 'For Client: Xyz Corp Conference', got %q", req.Description)
	}

	// Re-marshal and verify round-trip
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateProjectRequest: %v", err)
	}

	var roundtrip CreateProjectRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name || roundtrip.Description != req.Description {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateProjectRequest_Marshal(t *testing.T) {
	data := loadFixture(t, "update-request.json")

	var req UpdateProjectRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Name != "Marketing Campaign" {
		t.Errorf("expected name 'Marketing Campaign', got %q", req.Name)
	}
	if req.Description != "For Client: Xyz Corp Conference" {
		t.Errorf("expected description 'For Client: Xyz Corp Conference', got %q", req.Description)
	}
	if req.Admissions != "team" {
		t.Errorf("expected admissions 'team', got %q", req.Admissions)
	}
	if req.ScheduleAttributes == nil {
		t.Fatal("expected ScheduleAttributes")
	}
	if req.ScheduleAttributes.StartDate != "2022-01-01" {
		t.Errorf("expected start_date '2022-01-01', got %q", req.ScheduleAttributes.StartDate)
	}
	if req.ScheduleAttributes.EndDate != "2022-04-01" {
		t.Errorf("expected end_date '2022-04-01', got %q", req.ScheduleAttributes.EndDate)
	}
}

func TestErrorResponse_Unmarshal(t *testing.T) {
	data := loadFixture(t, "error-limit.json")

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &errResp); err != nil {
		t.Fatalf("failed to unmarshal error-limit.json: %v", err)
	}

	expected := "The project limit for this account has been reached."
	if errResp.Error != expected {
		t.Errorf("expected error %q, got %q", expected, errResp.Error)
	}
}

func TestProject_TimestampParsing(t *testing.T) {
	data := loadFixture(t, "get.json")

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	// Verify ISO8601 timestamps parse correctly
	// created_at: "2022-10-28T08:23:58.169Z"
	if project.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", project.CreatedAt.Year())
	}
	if project.CreatedAt.Month() != 10 {
		t.Errorf("expected month 10, got %d", project.CreatedAt.Month())
	}
	if project.CreatedAt.Day() != 28 {
		t.Errorf("expected day 28, got %d", project.CreatedAt.Day())
	}
}
