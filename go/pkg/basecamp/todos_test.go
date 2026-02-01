package basecamp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// unmarshalTodosWithNumbers decodes JSON into a map preserving numbers as json.Number
// which can be cleanly converted to int64 without float64 precision loss.
func unmarshalTodosWithNumbers(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return result, decoder.Decode(&result)
}

func todosFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "todos")
}

func loadTodosFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(todosFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTodo_UnmarshalList(t *testing.T) {
	data := loadTodosFixture(t, "list.json")

	var todos []Todo
	if err := json.Unmarshal(data, &todos); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(todos) != 2 {
		t.Errorf("expected 2 todos, got %d", len(todos))
	}

	// Verify first todo
	t1 := todos[0]
	if t1.ID != 1069479520 {
		t.Errorf("expected ID 1069479520, got %d", t1.ID)
	}
	if t1.Status != "active" {
		t.Errorf("expected status 'active', got %q", t1.Status)
	}
	if t1.Type != "Todo" {
		t.Errorf("expected type 'Todo', got %q", t1.Type)
	}

	// Verify content is plain text (not wrapped in HTML tags)
	expectedContent := "Program Leto locator  microcontroller unit"
	if t1.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, t1.Content)
	}
	// Title should match content for todos
	if t1.Title != expectedContent {
		t.Errorf("expected title %q, got %q", expectedContent, t1.Title)
	}

	// Verify description is empty (no HTML when not set)
	if t1.Description != "" {
		t.Errorf("expected empty description, got %q", t1.Description)
	}

	if t1.URL != "https://3.basecampapi.com/195539477/buckets/2085958500/todos/1069479520.json" {
		t.Errorf("unexpected URL: %q", t1.URL)
	}
	if t1.AppURL != "https://3.basecamp.com/195539477/buckets/2085958500/todos/1069479520" {
		t.Errorf("unexpected AppURL: %q", t1.AppURL)
	}

	// Verify parent (todolist)
	if t1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if t1.Parent.ID != 1069479519 {
		t.Errorf("expected Parent.ID 1069479519, got %d", t1.Parent.ID)
	}
	if t1.Parent.Title != "Hardware" {
		t.Errorf("expected Parent.Title 'Hardware', got %q", t1.Parent.Title)
	}
	if t1.Parent.Type != "Todolist" {
		t.Errorf("expected Parent.Type 'Todolist', got %q", t1.Parent.Type)
	}

	// Verify bucket
	if t1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if t1.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", t1.Bucket.ID)
	}
	if t1.Bucket.Name != "The Leto Locator" {
		t.Errorf("expected Bucket.Name 'The Leto Locator', got %q", t1.Bucket.Name)
	}
	if t1.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", t1.Bucket.Type)
	}

	// Verify creator
	if t1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if t1.Creator.ID != 1049715915 {
		t.Errorf("expected Creator.ID 1049715915, got %d", t1.Creator.ID)
	}
	if t1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", t1.Creator.Name)
	}

	// Verify assignees
	if len(t1.Assignees) != 1 {
		t.Fatalf("expected 1 assignee, got %d", len(t1.Assignees))
	}
	if t1.Assignees[0].ID != 1049715920 {
		t.Errorf("expected assignee ID 1049715920, got %d", t1.Assignees[0].ID)
	}
	if t1.Assignees[0].Name != "Steve Marsh" {
		t.Errorf("expected assignee name 'Steve Marsh', got %q", t1.Assignees[0].Name)
	}

	// Verify dates
	if t1.DueOn != "2022-12-01" {
		t.Errorf("expected due_on '2022-12-01', got %q", t1.DueOn)
	}
	if t1.Completed {
		t.Error("expected completed to be false")
	}
	if t1.Position != 1 {
		t.Errorf("expected position 1, got %d", t1.Position)
	}

	// Verify second todo
	t2 := todos[1]
	if t2.ID != 1069479521 {
		t.Errorf("expected ID 1069479521, got %d", t2.ID)
	}
	// Content should be plain text
	if t2.Content != "Assemble 25 units for testing" {
		t.Errorf("expected content 'Assemble 25 units for testing', got %q", t2.Content)
	}
	if t2.StartsOn != "2022-11-25" {
		t.Errorf("expected starts_on '2022-11-25', got %q", t2.StartsOn)
	}
	if t2.DueOn != "2022-12-15" {
		t.Errorf("expected due_on '2022-12-15', got %q", t2.DueOn)
	}
	// Second todo has no assignees
	if len(t2.Assignees) != 0 {
		t.Errorf("expected 0 assignees for second todo, got %d", len(t2.Assignees))
	}
}

func TestTodo_UnmarshalGet(t *testing.T) {
	data := loadTodosFixture(t, "get.json")

	var todo Todo
	if err := json.Unmarshal(data, &todo); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if todo.ID != 1069479520 {
		t.Errorf("expected ID 1069479520, got %d", todo.ID)
	}
	if todo.Status != "active" {
		t.Errorf("expected status 'active', got %q", todo.Status)
	}
	if todo.Type != "Todo" {
		t.Errorf("expected type 'Todo', got %q", todo.Type)
	}

	// KEY TEST: Content should be plain text, not wrapped in HTML
	expectedContent := "Program Leto locator  microcontroller unit"
	if todo.Content != expectedContent {
		t.Errorf("expected plain text content %q, got %q", expectedContent, todo.Content)
	}
	if todo.Title != expectedContent {
		t.Errorf("expected title %q, got %q", expectedContent, todo.Title)
	}

	// Description should be empty string when not set (not HTML-wrapped)
	if todo.Description != "" {
		t.Errorf("expected empty description, got %q", todo.Description)
	}

	// Verify timestamps are parsed
	if todo.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if todo.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if todo.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if todo.Creator.ID != 1049715915 {
		t.Errorf("expected Creator.ID 1049715915, got %d", todo.Creator.ID)
	}
	if todo.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", todo.Creator.Name)
	}
	if todo.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", todo.Creator.EmailAddress)
	}
	if todo.Creator.Title != "Chief Strategist" {
		t.Errorf("expected Creator.Title 'Chief Strategist', got %q", todo.Creator.Title)
	}
	if !todo.Creator.Owner {
		t.Error("expected Creator.Owner to be true")
	}
	if !todo.Creator.Admin {
		t.Error("expected Creator.Admin to be true")
	}

	// Verify assignees
	if len(todo.Assignees) != 1 {
		t.Fatalf("expected 1 assignee, got %d", len(todo.Assignees))
	}
	if todo.Assignees[0].Name != "Steve Marsh" {
		t.Errorf("expected assignee name 'Steve Marsh', got %q", todo.Assignees[0].Name)
	}
}

func TestCreateTodoRequest_Marshal(t *testing.T) {
	req := CreateTodoRequest{
		Content:     "Review hardware schematics",
		Description: "<div>Check for power consumption issues</div>",
		AssigneeIDs: []int64{1049715920},
		Notify:      true,
		DueOn:       "2022-12-10",
		StartsOn:    "2022-11-28",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTodoRequest: %v", err)
	}

	data, err := unmarshalTodosWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// KEY TEST: Content should be plain text (the todo title)
	if data["content"] != "Review hardware schematics" {
		t.Errorf("expected plain text content 'Review hardware schematics', got %v", data["content"])
	}

	// Description can contain HTML (for extended notes)
	if data["description"] != "<div>Check for power consumption issues</div>" {
		t.Errorf("expected HTML description, got %v", data["description"])
	}

	if data["notify"] != true {
		t.Errorf("expected notify true, got %v", data["notify"])
	}
	if data["due_on"] != "2022-12-10" {
		t.Errorf("expected due_on '2022-12-10', got %v", data["due_on"])
	}
	if data["starts_on"] != "2022-11-28" {
		t.Errorf("expected starts_on '2022-11-28', got %v", data["starts_on"])
	}

	// Verify assignee_ids
	assigneeIDs, ok := data["assignee_ids"].([]interface{})
	if !ok {
		t.Fatalf("expected assignee_ids to be array, got %T", data["assignee_ids"])
	}
	if len(assigneeIDs) != 1 {
		t.Errorf("expected 1 assignee_id, got %d", len(assigneeIDs))
	}

	// Round-trip test
	var roundtrip CreateTodoRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
	if roundtrip.Description != req.Description {
		t.Errorf("expected description %q, got %q", req.Description, roundtrip.Description)
	}
}

func TestCreateTodoRequest_MarshalMinimal(t *testing.T) {
	// Test with only required field (content)
	req := CreateTodoRequest{
		Content: "Simple task",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTodoRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Content is required and should be plain text
	if data["content"] != "Simple task" {
		t.Errorf("expected content 'Simple task', got %v", data["content"])
	}

	// Optional fields with omitempty should not be present
	if _, ok := data["description"]; ok {
		t.Error("expected description to be omitted")
	}
	if _, ok := data["due_on"]; ok {
		t.Error("expected due_on to be omitted")
	}
	if _, ok := data["starts_on"]; ok {
		t.Error("expected starts_on to be omitted")
	}
	if _, ok := data["assignee_ids"]; ok {
		t.Error("expected assignee_ids to be omitted")
	}
}

// TestCreateTodoRequest_ContentIsPlainText verifies that Content should be
// plain text (the todo title), NOT HTML-wrapped. This is critical because
// the Basecamp UI displays Content directly without HTML rendering.
func TestCreateTodoRequest_ContentIsPlainText(t *testing.T) {
	// The fixture file shows the expected format
	data := loadTodosFixture(t, "create-request.json")

	var req CreateTodoRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	// Content should be plain text - NO HTML tags
	expectedContent := "Review hardware schematics"
	if req.Content != expectedContent {
		t.Errorf("Content should be plain text.\nExpected: %q\nGot: %q", expectedContent, req.Content)
	}

	// Verify content does NOT start with HTML tag
	if len(req.Content) > 0 && req.Content[0] == '<' {
		t.Errorf("Content should NOT be HTML-wrapped, but starts with '<': %q", req.Content)
	}

	// Description CAN contain HTML (for extended notes)
	expectedDescription := "<div>Check for power consumption issues</div>"
	if req.Description != expectedDescription {
		t.Errorf("Description should contain HTML.\nExpected: %q\nGot: %q", expectedDescription, req.Description)
	}
}

func TestUpdateTodoRequest_Marshal(t *testing.T) {
	req := UpdateTodoRequest{
		Content:     "Review hardware schematics (updated)",
		Description: "<div>Check for power consumption and heat issues</div>",
		AssigneeIDs: []int64{1049715920, 1049715915},
		DueOn:       "2022-12-15",
		StartsOn:    "2022-12-01",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateTodoRequest: %v", err)
	}

	data, err := unmarshalTodosWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Content should be plain text
	if data["content"] != "Review hardware schematics (updated)" {
		t.Errorf("expected plain text content, got %v", data["content"])
	}

	// Description can contain HTML
	if data["description"] != "<div>Check for power consumption and heat issues</div>" {
		t.Errorf("expected HTML description, got %v", data["description"])
	}

	if data["due_on"] != "2022-12-15" {
		t.Errorf("expected due_on '2022-12-15', got %v", data["due_on"])
	}
	if data["starts_on"] != "2022-12-01" {
		t.Errorf("expected starts_on '2022-12-01', got %v", data["starts_on"])
	}

	// Round-trip test
	var roundtrip UpdateTodoRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
	if roundtrip.Description != req.Description {
		t.Errorf("expected description %q, got %q", req.Description, roundtrip.Description)
	}
}

func TestUpdateTodoRequest_MarshalPartial(t *testing.T) {
	// Test with only some fields (partial update)
	req := UpdateTodoRequest{
		Content: "Just updating the title",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateTodoRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["content"] != "Just updating the title" {
		t.Errorf("unexpected content: %v", data["content"])
	}

	// Optional fields should be omitted
	if _, ok := data["description"]; ok {
		t.Error("expected description to be omitted")
	}
	if _, ok := data["due_on"]; ok {
		t.Error("expected due_on to be omitted")
	}
	if _, ok := data["starts_on"]; ok {
		t.Error("expected starts_on to be omitted")
	}
	if _, ok := data["assignee_ids"]; ok {
		t.Error("expected assignee_ids to be omitted")
	}
}

// TestUpdateTodoRequest_ContentIsPlainText verifies that Content in update
// requests should also be plain text.
func TestUpdateTodoRequest_ContentIsPlainText(t *testing.T) {
	data := loadTodosFixture(t, "update-request.json")

	var req UpdateTodoRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	// Content should be plain text - NO HTML tags
	expectedContent := "Review hardware schematics (updated)"
	if req.Content != expectedContent {
		t.Errorf("Content should be plain text.\nExpected: %q\nGot: %q", expectedContent, req.Content)
	}

	// Verify content does NOT start with HTML tag
	if len(req.Content) > 0 && req.Content[0] == '<' {
		t.Errorf("Content should NOT be HTML-wrapped, but starts with '<': %q", req.Content)
	}

	// Description CAN contain HTML
	expectedDescription := "<div>Check for power consumption and heat issues</div>"
	if req.Description != expectedDescription {
		t.Errorf("Description should contain HTML.\nExpected: %q\nGot: %q", expectedDescription, req.Description)
	}
}

func TestTodoListOptions_Defaults(t *testing.T) {
	opts := &TodoListOptions{}

	// Verify default values
	if opts.Status != "" {
		t.Errorf("expected empty status by default, got %q", opts.Status)
	}
	if opts.Limit != 0 {
		t.Errorf("expected 0 limit by default, got %d", opts.Limit)
	}
	if opts.Page != 0 {
		t.Errorf("expected 0 page by default, got %d", opts.Page)
	}
}

func TestTodoListOptions_StatusFilter(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"completed", "completed"},
		{"pending", "pending"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &TodoListOptions{Status: tt.status}
			if opts.Status != tt.status {
				t.Errorf("expected status %q, got %q", tt.status, opts.Status)
			}
		})
	}
}
