package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func todosetsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "todosets")
}

func loadTodosetsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(todosetsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTodoset_UnmarshalGet(t *testing.T) {
	data := loadTodosetsFixture(t, "get.json")

	var todoset Todoset
	if err := json.Unmarshal(data, &todoset); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if todoset.ID != 1069479338 {
		t.Errorf("expected ID 1069479338, got %d", todoset.ID)
	}
	if todoset.Name != "To-dos" {
		t.Errorf("expected name 'To-dos', got %q", todoset.Name)
	}
	if todoset.Title != "To-dos" {
		t.Errorf("expected title 'To-dos', got %q", todoset.Title)
	}
	if todoset.Type != "Todoset" {
		t.Errorf("expected type 'Todoset', got %q", todoset.Type)
	}
	if todoset.Status != "active" {
		t.Errorf("expected status 'active', got %q", todoset.Status)
	}
	if todoset.TodolistsCount != 3 {
		t.Errorf("expected todolists_count 3, got %d", todoset.TodolistsCount)
	}
	if todoset.CompletedRatio != "5/15" {
		t.Errorf("expected completed_ratio '5/15', got %q", todoset.CompletedRatio)
	}
	if todoset.CompletedCount != 5 {
		t.Errorf("expected completed_count 5, got %d", todoset.CompletedCount)
	}
	if todoset.OnScheduleCount != 3 {
		t.Errorf("expected on_schedule_count 3, got %d", todoset.OnScheduleCount)
	}
	if todoset.OverScheduleCount != 2 {
		t.Errorf("expected over_schedule_count 2, got %d", todoset.OverScheduleCount)
	}
	if todoset.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if todoset.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", todoset.Bucket.ID)
	}
	if todoset.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if todoset.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", todoset.Creator.Name)
	}
}

func TestTodoset_TimestampParsing(t *testing.T) {
	data := loadTodosetsFixture(t, "get.json")

	var todoset Todoset
	if err := json.Unmarshal(data, &todoset); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if todoset.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if todoset.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if todoset.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", todoset.CreatedAt.Year())
	}
}
