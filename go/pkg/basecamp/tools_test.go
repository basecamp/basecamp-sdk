package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func toolsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "tools")
}

func loadToolsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(toolsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTool_UnmarshalGet(t *testing.T) {
	data := loadToolsFixture(t, "get.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if tool.ID != 1069479339 {
		t.Errorf("expected ID 1069479339, got %d", tool.ID)
	}
	if tool.Status != "active" {
		t.Errorf("expected status 'active', got %q", tool.Status)
	}
	if tool.Title != "To-dos" {
		t.Errorf("expected title 'To-dos', got %q", tool.Title)
	}
	if tool.Name != "todoset" {
		t.Errorf("expected name 'todoset', got %q", tool.Name)
	}
	if !tool.Enabled {
		t.Error("expected Enabled to be true")
	}
	if tool.Position == nil || *tool.Position != 2 {
		t.Errorf("expected position 2, got %v", tool.Position)
	}
	if tool.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/todosets/1069479339.json" {
		t.Errorf("unexpected URL: %q", tool.URL)
	}
	if tool.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/todosets/1069479339" {
		t.Errorf("unexpected AppURL: %q", tool.AppURL)
	}

	// Verify timestamps are parsed
	if tool.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if tool.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify bucket
	if tool.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if tool.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", tool.Bucket.ID)
	}
	if tool.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", tool.Bucket.Name)
	}
	if tool.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", tool.Bucket.Type)
	}
}

func TestTool_UnmarshalCreate(t *testing.T) {
	data := loadToolsFixture(t, "create.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal create.json: %v", err)
	}

	if tool.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", tool.ID)
	}
	if tool.Title != "To-dos (copy)" {
		t.Errorf("expected title 'To-dos (copy)', got %q", tool.Title)
	}
	if tool.Name != "todoset" {
		t.Errorf("expected name 'todoset', got %q", tool.Name)
	}
	if !tool.Enabled {
		t.Error("expected Enabled to be true")
	}
	if tool.Position == nil || *tool.Position != 6 {
		t.Errorf("expected position 6, got %v", tool.Position)
	}
}

func TestTool_UnmarshalUpdate(t *testing.T) {
	data := loadToolsFixture(t, "update.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal update.json: %v", err)
	}

	if tool.ID != 1069479339 {
		t.Errorf("expected ID 1069479339, got %d", tool.ID)
	}
	if tool.Title != "Project Tasks" {
		t.Errorf("expected title 'Project Tasks', got %q", tool.Title)
	}
	if tool.Name != "todoset" {
		t.Errorf("expected name 'todoset', got %q", tool.Name)
	}
}

func TestTool_UnmarshalDisabled(t *testing.T) {
	data := loadToolsFixture(t, "disabled.json")

	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		t.Fatalf("failed to unmarshal disabled.json: %v", err)
	}

	if tool.ID != 1069479343 {
		t.Errorf("expected ID 1069479343, got %d", tool.ID)
	}
	if tool.Title != "Automatic Check-ins" {
		t.Errorf("expected title 'Automatic Check-ins', got %q", tool.Title)
	}
	if tool.Name != "questionnaire" {
		t.Errorf("expected name 'questionnaire', got %q", tool.Name)
	}
	if tool.Enabled {
		t.Error("expected Enabled to be false")
	}
	if tool.Position != nil {
		t.Errorf("expected position to be nil, got %v", tool.Position)
	}
}

func TestUpdateToolRequest_Marshal(t *testing.T) {
	req := UpdateToolRequest{
		Title: "Project Tasks",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateToolRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Project Tasks" {
		t.Errorf("expected title 'Project Tasks', got %v", data["name"])
	}

	// Round-trip test
	var roundtrip UpdateToolRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Title != req.Title {
		t.Errorf("expected title %q, got %q", req.Title, roundtrip.Title)
	}
}

func TestCloneToolRequest_Marshal(t *testing.T) {
	req := CloneToolRequest{SourceToolID: 123, Title: "Sprint Backlog"}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CloneToolRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["source_recording_id"] != float64(123) {
		t.Errorf("expected source_recording_id 123, got %v", data["source_recording_id"])
	}
	if data["title"] != "Sprint Backlog" {
		t.Errorf("expected title 'Sprint Backlog', got %v", data["title"])
	}

	// Round-trip test
	var roundtrip CloneToolRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.SourceToolID != req.SourceToolID {
		t.Errorf("expected SourceToolID %d, got %d", req.SourceToolID, roundtrip.SourceToolID)
	}
	if roundtrip.Title != req.Title {
		t.Errorf("expected Title %q, got %q", req.Title, roundtrip.Title)
	}
}

// newTestToolsService creates a ToolsService with minimal wiring for
// testing validation logic that runs before the generated client call.
func newTestToolsService() *ToolsService {
	c := &Client{hooks: NoopHooks{}}
	ac := &AccountClient{parent: c, accountID: "99999"}
	return NewToolsService(ac)
}

func TestCreate_NilRequest(t *testing.T) {
	svc := newTestToolsService()
	_, err := svc.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestCreate_EmptyTitle(t *testing.T) {
	svc := newTestToolsService()
	_, err := svc.Create(context.Background(), &CloneToolRequest{SourceToolID: 1})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage {
		t.Errorf("expected usage error, got: %v", err)
	}
}
