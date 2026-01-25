package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func todolistGroupsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "todolist_groups")
}

func loadTodolistGroupsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(todolistGroupsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTodolistGroup_UnmarshalList(t *testing.T) {
	data := loadTodolistGroupsFixture(t, "list.json")

	var groups []TodolistGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	// Verify first group
	g1 := groups[0]
	if g1.ID != 1069479600 {
		t.Errorf("expected ID 1069479600, got %d", g1.ID)
	}
	if g1.Name != "Phase 1" {
		t.Errorf("expected name 'Phase 1', got %q", g1.Name)
	}
	if g1.Title != "Phase 1" {
		t.Errorf("expected title 'Phase 1', got %q", g1.Title)
	}
	if g1.Type != "Todolist" {
		t.Errorf("expected type 'Todolist', got %q", g1.Type)
	}
	if g1.Status != "active" {
		t.Errorf("expected status 'active', got %q", g1.Status)
	}
	if g1.CompletedRatio != "1/3" {
		t.Errorf("expected completed_ratio '1/3', got %q", g1.CompletedRatio)
	}
	if g1.Position != 1 {
		t.Errorf("expected position 1, got %d", g1.Position)
	}

	// Verify second group
	g2 := groups[1]
	if g2.ID != 1069479601 {
		t.Errorf("expected ID 1069479601, got %d", g2.ID)
	}
	if g2.Name != "Phase 2" {
		t.Errorf("expected name 'Phase 2', got %q", g2.Name)
	}
	if g2.Position != 2 {
		t.Errorf("expected position 2, got %d", g2.Position)
	}
}

func TestTodolistGroup_UnmarshalGet(t *testing.T) {
	data := loadTodolistGroupsFixture(t, "get.json")

	var group TodolistGroup
	if err := json.Unmarshal(data, &group); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if group.ID != 1069479600 {
		t.Errorf("expected ID 1069479600, got %d", group.ID)
	}
	if group.Name != "Phase 1" {
		t.Errorf("expected name 'Phase 1', got %q", group.Name)
	}
	if group.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if group.Parent.ID != 1069479519 {
		t.Errorf("expected Parent.ID 1069479519, got %d", group.Parent.ID)
	}
	if group.Parent.Type != "Todolist" {
		t.Errorf("expected Parent.Type 'Todolist', got %q", group.Parent.Type)
	}
	if group.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if group.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", group.Bucket.ID)
	}
	if group.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if group.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", group.Creator.Name)
	}
	if group.TodosURL == "" {
		t.Error("expected non-empty TodosURL")
	}
}

func TestCreateTodolistGroupRequest_Marshal(t *testing.T) {
	data := loadTodolistGroupsFixture(t, "create-request.json")

	var req CreateTodolistGroupRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Name != "Phase 3" {
		t.Errorf("expected name 'Phase 3', got %q", req.Name)
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTodolistGroupRequest: %v", err)
	}

	var roundtrip CreateTodolistGroupRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Error("round-trip mismatch")
	}
}

func TestTodolistGroup_TimestampParsing(t *testing.T) {
	data := loadTodolistGroupsFixture(t, "get.json")

	var group TodolistGroup
	if err := json.Unmarshal(data, &group); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if group.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if group.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if group.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", group.CreatedAt.Year())
	}
}
