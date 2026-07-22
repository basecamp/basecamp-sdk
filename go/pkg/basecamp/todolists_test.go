package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

// --- httptest-based service contract tests ---

// testTodolistsServer creates an httptest.Server and a TodolistsService wired to it.
func testTodolistsServer(t *testing.T, handler http.HandlerFunc) *TodolistsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Todolists()
}

func TestTodolistsService_Get(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/todolists/1069479519" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	todolist, err := svc.Get(context.Background(), 1069479519)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
}

func TestTodolistsService_Update(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	var receivedBody map[string]string
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/99999/todolists/1069479519" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	todolist, err := svc.Update(context.Background(), 1069479519, &UpdateTodolistRequest{
		Name:        "Updated Name",
		Description: "Updated description",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	if todolist.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if todolist.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if receivedBody["name"] != "Updated Name" {
		t.Errorf("expected request body name 'Updated Name', got %q", receivedBody["name"])
	}
	if receivedBody["description"] != "Updated description" {
		t.Errorf("expected request body description 'Updated description', got %q", receivedBody["description"])
	}
}

func TestTodolistsService_Reposition(t *testing.T) {
	var receivedBody map[string]int
	called := false
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/99999/todosets/todolists/1069479519/position.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(204)
	})

	if err := svc.Reposition(context.Background(), 1069479519, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected the server to be called")
	}
	if receivedBody["position"] != 3 {
		t.Errorf("expected request body position 3, got %d", receivedBody["position"])
	}
}

func TestTodolistsService_Reposition_NotFound(t *testing.T) {
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	err := svc.Reposition(context.Background(), 999, 1)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Errorf("expected not_found error, got: %v", err)
	}
}

func TestTodolistsService_Reposition_PositionTooLow(t *testing.T) {
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when position is below 1")
	})

	if err := svc.Reposition(context.Background(), 1069479519, 0); err == nil {
		t.Fatal("expected usage error for position < 1")
	}
}

func TestTodolistsService_Reposition_PositionOutOfRange(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("positions above MaxInt32 are unrepresentable as int on 32-bit platforms")
	}
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when position exceeds MaxInt32")
	})

	// Build MaxInt32+1 at runtime: the constant math.MaxInt32+1 overflows int on
	// 32-bit and would not compile there, even though the guard above skips it.
	position := math.MaxInt32
	position++
	if err := svc.Reposition(context.Background(), 1069479519, position); err == nil {
		t.Fatal("expected usage error for position > MaxInt32")
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
