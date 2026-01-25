package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func todolistsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "todolists")
}

func loadTodolistsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(todolistsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTodolist_UnmarshalList(t *testing.T) {
	data := loadTodolistsFixture(t, "list.json")

	var todolists []Todolist
	if err := json.Unmarshal(data, &todolists); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(todolists) != 2 {
		t.Errorf("expected 2 todolists, got %d", len(todolists))
	}

	// Verify first todolist
	tl1 := todolists[0]
	if tl1.ID != 1069479519 {
		t.Errorf("expected ID 1069479519, got %d", tl1.ID)
	}
	if tl1.Name != "Hardware" {
		t.Errorf("expected name 'Hardware', got %q", tl1.Name)
	}
	if tl1.Title != "Hardware" {
		t.Errorf("expected title 'Hardware', got %q", tl1.Title)
	}
	if tl1.Type != "Todolist" {
		t.Errorf("expected type 'Todolist', got %q", tl1.Type)
	}
	if tl1.Status != "active" {
		t.Errorf("expected status 'active', got %q", tl1.Status)
	}
	if tl1.CompletedRatio != "0/3" {
		t.Errorf("expected completed_ratio '0/3', got %q", tl1.CompletedRatio)
	}
	if tl1.Position != 1 {
		t.Errorf("expected position 1, got %d", tl1.Position)
	}

	// Verify second todolist has description
	tl2 := todolists[1]
	if tl2.ID != 1069479522 {
		t.Errorf("expected ID 1069479522, got %d", tl2.ID)
	}
	if tl2.Name != "Software" {
		t.Errorf("expected name 'Software', got %q", tl2.Name)
	}
	if tl2.Description != "Mobile and web app development tasks" {
		t.Errorf("expected description 'Mobile and web app development tasks', got %q", tl2.Description)
	}
}

func TestTodolist_UnmarshalGet(t *testing.T) {
	data := loadTodolistsFixture(t, "get.json")

	var todolist Todolist
	if err := json.Unmarshal(data, &todolist); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if todolist.ID != 1069479519 {
		t.Errorf("expected ID 1069479519, got %d", todolist.ID)
	}
	if todolist.Name != "Hardware" {
		t.Errorf("expected name 'Hardware', got %q", todolist.Name)
	}
	if todolist.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if todolist.Parent.ID != 1069479338 {
		t.Errorf("expected Parent.ID 1069479338, got %d", todolist.Parent.ID)
	}
	if todolist.Parent.Type != "Todoset" {
		t.Errorf("expected Parent.Type 'Todoset', got %q", todolist.Parent.Type)
	}
	if todolist.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if todolist.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", todolist.Bucket.ID)
	}
	if todolist.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if todolist.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", todolist.Creator.Name)
	}
	if todolist.TodosURL == "" {
		t.Error("expected non-empty TodosURL")
	}
	if todolist.GroupsURL == "" {
		t.Error("expected non-empty GroupsURL")
	}
}

func TestCreateTodolistRequest_Marshal(t *testing.T) {
	data := loadTodolistsFixture(t, "create-request.json")

	var req CreateTodolistRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Name != "Launch Tasks" {
		t.Errorf("expected name 'Launch Tasks', got %q", req.Name)
	}
	if req.Description != "Tasks for product launch" {
		t.Errorf("expected description 'Tasks for product launch', got %q", req.Description)
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTodolistRequest: %v", err)
	}

	var roundtrip CreateTodolistRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name || roundtrip.Description != req.Description {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateTodolistRequest_Marshal(t *testing.T) {
	data := loadTodolistsFixture(t, "update-request.json")

	var req UpdateTodolistRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Name != "Updated Launch Tasks" {
		t.Errorf("expected name 'Updated Launch Tasks', got %q", req.Name)
	}
	if req.Description != "Updated description for launch tasks" {
		t.Errorf("expected description 'Updated description for launch tasks', got %q", req.Description)
	}
}

func TestTodolist_TimestampParsing(t *testing.T) {
	data := loadTodolistsFixture(t, "get.json")

	var todolist Todolist
	if err := json.Unmarshal(data, &todolist); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if todolist.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if todolist.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if todolist.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", todolist.CreatedAt.Year())
	}
}
