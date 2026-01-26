package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func searchFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "search")
}

func loadSearchFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(searchFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestSearchResult_UnmarshalResults(t *testing.T) {
	data := loadSearchFixture(t, "results.json")

	var results []SearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to unmarshal results.json: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Verify first result (Message)
	r1 := results[0]
	if r1.ID != 1069479351 {
		t.Errorf("expected ID 1069479351, got %d", r1.ID)
	}
	if r1.Status != "active" {
		t.Errorf("expected status 'active', got %q", r1.Status)
	}
	if r1.Type != "Message" {
		t.Errorf("expected type 'Message', got %q", r1.Type)
	}
	if r1.Title != "We won Leto!" {
		t.Errorf("expected title 'We won Leto!', got %q", r1.Title)
	}
	if r1.Subject != "We won Leto!" {
		t.Errorf("expected subject 'We won Leto!', got %q", r1.Subject)
	}
	if r1.Content != "<div>Hello everyone! We got the Leto Laptop project! Time to get started.</div>" {
		t.Errorf("unexpected content: %q", r1.Content)
	}
	if r1.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/messages/1069479351.json" {
		t.Errorf("unexpected URL: %q", r1.URL)
	}
	if r1.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/messages/1069479351" {
		t.Errorf("unexpected AppURL: %q", r1.AppURL)
	}

	// Verify parent (message board)
	if r1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if r1.Parent.ID != 1069479338 {
		t.Errorf("expected Parent.ID 1069479338, got %d", r1.Parent.ID)
	}
	if r1.Parent.Title != "Message Board" {
		t.Errorf("expected Parent.Title 'Message Board', got %q", r1.Parent.Title)
	}
	if r1.Parent.Type != "Message::Board" {
		t.Errorf("expected Parent.Type 'Message::Board', got %q", r1.Parent.Type)
	}

	// Verify bucket
	if r1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if r1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", r1.Bucket.ID)
	}
	if r1.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", r1.Bucket.Name)
	}
	if r1.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", r1.Bucket.Type)
	}

	// Verify creator
	if r1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if r1.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", r1.Creator.ID)
	}
	if r1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", r1.Creator.Name)
	}

	// Verify second result (Todo)
	r2 := results[1]
	if r2.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", r2.ID)
	}
	if r2.Type != "Todo" {
		t.Errorf("expected type 'Todo', got %q", r2.Type)
	}
	if r2.Title != "Design specs for Leto display" {
		t.Errorf("expected title 'Design specs for Leto display', got %q", r2.Title)
	}
	if r2.Description != "Create detailed specifications for the Leto laptop display panel" {
		t.Errorf("unexpected description: %q", r2.Description)
	}
	if r2.Parent == nil {
		t.Fatal("expected Parent to be non-nil for second result")
	}
	if r2.Parent.Type != "Todolist" {
		t.Errorf("expected Parent.Type 'Todolist', got %q", r2.Parent.Type)
	}
	if r2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second result")
	}
	if r2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", r2.Creator.Name)
	}

	// Verify third result (Comment)
	r3 := results[2]
	if r3.ID != 1069479450 {
		t.Errorf("expected ID 1069479450, got %d", r3.ID)
	}
	if r3.Type != "Comment" {
		t.Errorf("expected type 'Comment', got %q", r3.Type)
	}
	if r3.Content != "<div>The Leto keyboard layout looks great. Let's finalize it.</div>" {
		t.Errorf("unexpected content for comment: %q", r3.Content)
	}

	// Verify timestamps are parsed
	if r1.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if r1.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
}

func TestSearchMetadata_Unmarshal(t *testing.T) {
	data := loadSearchFixture(t, "metadata.json")

	var metadata SearchMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata.json: %v", err)
	}

	if len(metadata.Projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(metadata.Projects))
	}

	// Verify first project
	p1 := metadata.Projects[0]
	if p1.ID != 2085958499 {
		t.Errorf("expected ID 2085958499, got %d", p1.ID)
	}
	if p1.Name != "The Leto Laptop" {
		t.Errorf("expected name 'The Leto Laptop', got %q", p1.Name)
	}

	// Verify second project
	p2 := metadata.Projects[1]
	if p2.ID != 2085958500 {
		t.Errorf("expected ID 2085958500, got %d", p2.ID)
	}
	if p2.Name != "Marketing Campaign Q4" {
		t.Errorf("expected name 'Marketing Campaign Q4', got %q", p2.Name)
	}

	// Verify third project
	p3 := metadata.Projects[2]
	if p3.ID != 2085958501 {
		t.Errorf("expected ID 2085958501, got %d", p3.ID)
	}
	if p3.Name != "Internal Operations" {
		t.Errorf("expected name 'Internal Operations', got %q", p3.Name)
	}
}

func TestSearchOptions_Marshal(t *testing.T) {
	opts := SearchOptions{
		Sort: "created_at",
	}

	out, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("failed to marshal SearchOptions: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["Sort"] != "created_at" {
		t.Errorf("unexpected Sort: %v", data["Sort"])
	}
}

func TestSearchResult_DifferentTypes(t *testing.T) {
	data := loadSearchFixture(t, "results.json")

	var results []SearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to unmarshal results.json: %v", err)
	}

	// Collect unique types
	types := make(map[string]bool)
	for _, r := range results {
		types[r.Type] = true
	}

	// Verify we have multiple types
	expectedTypes := []string{"Message", "Todo", "Comment"}
	for _, et := range expectedTypes {
		if !types[et] {
			t.Errorf("expected type %q in results", et)
		}
	}
}
