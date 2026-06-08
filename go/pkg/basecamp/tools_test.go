package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestToolsServiceCreatePostsToBucketDock(t *testing.T) {
	const (
		accountID = "5245563"
		bucketID  = int64(33861629)
		toolType  = "Message::Board"
		title     = "Intervention Log / Journal"
	)

	expectedPath := fmt.Sprintf("/%s/buckets/%d/dock/tools.json", accountID, bucketID)

	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if r.Method != http.MethodPost || r.URL.Path != expectedPath {
			http.NotFound(w, r)
			return
		}

		body := decodeRequestBody(t, r)
		if got := body["tool_type"]; got != toolType {
			t.Fatalf("tool_type = %v, want %q", got, toolType)
		}
		if got := body["title"]; got != title {
			t.Fatalf("title = %v, want %q", got, title)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(loadToolsFixture(t, "create.json"))
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	var capturedOp OperationInfo
	hooks := &testHooks{
		onOperationStart: func(ctx context.Context, op OperationInfo) context.Context {
			capturedOp = op
			return ctx
		},
	}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithHooks(hooks))

	_, err := client.ForAccount(accountID).Tools().Create(
		context.Background(),
		bucketID,
		toolType,
		&CreateToolOptions{Title: title},
	)
	if err != nil {
		t.Fatalf("Create() error = %v; request path = %s; want bucket %d dock tools endpoint", err, capturedPath, bucketID)
	}
	if capturedOp.ResourceID != bucketID {
		t.Fatalf("Create() operation ResourceID = %d, want destination bucket %d", capturedOp.ResourceID, bucketID)
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
