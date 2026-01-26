package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// templatesFixturesDir returns the path to the templates fixtures directory.
func templatesFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "templates")
}

// loadTemplatesFixture reads a fixture file and returns its contents.
func loadTemplatesFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(templatesFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTemplate_UnmarshalList(t *testing.T) {
	data := loadTemplatesFixture(t, "list.json")

	var templates []Template
	if err := json.Unmarshal(data, &templates); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(templates) != 2 {
		t.Errorf("expected 2 templates, got %d", len(templates))
	}

	// Verify first template
	t1 := templates[0]
	if t1.ID != 2085958501 {
		t.Errorf("expected ID 2085958501, got %d", t1.ID)
	}
	if t1.Name != "Project Template" {
		t.Errorf("expected name 'Project Template', got %q", t1.Name)
	}
	if t1.Status != "active" {
		t.Errorf("expected status 'active', got %q", t1.Status)
	}
	if t1.Description != "Standard project template for new initiatives." {
		t.Errorf("expected description 'Standard project template for new initiatives.', got %q", t1.Description)
	}

	// Verify second template
	t2 := templates[1]
	if t2.ID != 2085958502 {
		t.Errorf("expected ID 2085958502, got %d", t2.ID)
	}
	if t2.Name != "Client Onboarding" {
		t.Errorf("expected name 'Client Onboarding', got %q", t2.Name)
	}
}

func TestTemplate_UnmarshalGet(t *testing.T) {
	data := loadTemplatesFixture(t, "get.json")

	var template Template
	if err := json.Unmarshal(data, &template); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if template.ID != 2085958501 {
		t.Errorf("expected ID 2085958501, got %d", template.ID)
	}
	if template.Name != "Project Template" {
		t.Errorf("expected name 'Project Template', got %q", template.Name)
	}
	if template.Description != "Standard project template for new initiatives." {
		t.Errorf("expected description 'Standard project template for new initiatives.', got %q", template.Description)
	}
	if template.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if template.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestCreateTemplateRequest_Marshal(t *testing.T) {
	data := loadTemplatesFixture(t, "create-request.json")

	var req CreateTemplateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Name != "New Template" {
		t.Errorf("expected name 'New Template', got %q", req.Name)
	}
	if req.Description != "A new project template." {
		t.Errorf("expected description 'A new project template.', got %q", req.Description)
	}

	// Re-marshal and verify round-trip
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTemplateRequest: %v", err)
	}

	var roundtrip CreateTemplateRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name || roundtrip.Description != req.Description {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateTemplateRequest_Marshal(t *testing.T) {
	data := loadTemplatesFixture(t, "update-request.json")

	var req UpdateTemplateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Name != "Updated Template" {
		t.Errorf("expected name 'Updated Template', got %q", req.Name)
	}
	if req.Description != "Updated template description." {
		t.Errorf("expected description 'Updated template description.', got %q", req.Description)
	}
}

func TestProjectConstruction_Unmarshal(t *testing.T) {
	data := loadTemplatesFixture(t, "project_construction.json")

	var construction ProjectConstruction
	if err := json.Unmarshal(data, &construction); err != nil {
		t.Fatalf("failed to unmarshal project_construction.json: %v", err)
	}

	if construction.ID != 1234567890 {
		t.Errorf("expected ID 1234567890, got %d", construction.ID)
	}
	if construction.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", construction.Status)
	}
	if construction.URL == "" {
		t.Error("expected non-empty URL")
	}
	if construction.Project != nil {
		t.Error("expected nil Project for pending construction")
	}
}

func TestProjectConstruction_UnmarshalCompleted(t *testing.T) {
	data := loadTemplatesFixture(t, "project_construction_completed.json")

	var construction ProjectConstruction
	if err := json.Unmarshal(data, &construction); err != nil {
		t.Fatalf("failed to unmarshal project_construction_completed.json: %v", err)
	}

	if construction.ID != 1234567890 {
		t.Errorf("expected ID 1234567890, got %d", construction.ID)
	}
	if construction.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", construction.Status)
	}
	if construction.Project == nil {
		t.Fatal("expected non-nil Project for completed construction")
	}
	if construction.Project.ID != 2085958503 {
		t.Errorf("expected Project.ID 2085958503, got %d", construction.Project.ID)
	}
	if construction.Project.Name != "New Project from Template" {
		t.Errorf("expected Project.Name 'New Project from Template', got %q", construction.Project.Name)
	}
}

func TestTemplate_TimestampParsing(t *testing.T) {
	data := loadTemplatesFixture(t, "get.json")

	var template Template
	if err := json.Unmarshal(data, &template); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	// Verify ISO8601 timestamps parse correctly
	// created_at: "2022-10-28T08:23:58.169Z"
	if template.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", template.CreatedAt.Year())
	}
	if template.CreatedAt.Month() != 10 {
		t.Errorf("expected month 10, got %d", template.CreatedAt.Month())
	}
	if template.CreatedAt.Day() != 28 {
		t.Errorf("expected day 28, got %d", template.CreatedAt.Day())
	}
}
