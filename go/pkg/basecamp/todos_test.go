package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// unmarshalTodosWithNumbers is an alias for the shared unmarshalWithNumbers helper.
// This preserves the existing function name for backwards compatibility.
var unmarshalTodosWithNumbers = unmarshalWithNumbers

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

	// Description carries the rich-text HTML with inline attachments.
	if !strings.HasPrefix(todo.Description, "<div>Latest schematic") {
		t.Errorf("expected rich-text description, got %q", todo.Description)
	}
	if !strings.Contains(todo.Description, "<bc-attachment") {
		t.Errorf("expected description to reference bc-attachment, got %q", todo.Description)
	}

	// DescriptionAttachments decode directly through RichTextAttachment's
	// UnmarshalJSON (Todo has no custom decoder, so its embedded elements
	// invoke it automatically). The image entry's float-spelled "width":
	// 1024.0 decodes to 1024; the non-image entry's "width": null / "height":
	// null decode to nil rather than sentinel-zero.
	if len(todo.DescriptionAttachments) != 2 {
		t.Fatalf("expected 2 description attachments, got %d", len(todo.DescriptionAttachments))
	}
	img := todo.DescriptionAttachments[0]
	if img.ID != 1069480000 {
		t.Errorf("expected attachment ID 1069480000, got %d", img.ID)
	}
	if img.Filename != "leto-schematic.png" {
		t.Errorf("expected filename 'leto-schematic.png', got %q", img.Filename)
	}
	if img.ContentType != "image/png" {
		t.Errorf("expected content_type 'image/png', got %q", img.ContentType)
	}
	if img.ByteSize != 284111 {
		t.Errorf("expected byte_size 284111, got %d", img.ByteSize)
	}
	if img.Width == nil || *img.Width != 1024 {
		t.Errorf("expected width 1024 from float-spelled 1024.0, got %v", img.Width)
	}
	if img.Height == nil || *img.Height != 768 {
		t.Errorf("expected height 768, got %v", img.Height)
	}
	if !img.Previewable {
		t.Error("expected image attachment previewable")
	}
	pdf := todo.DescriptionAttachments[1]
	if pdf.ContentType != "application/pdf" {
		t.Errorf("expected content_type 'application/pdf', got %q", pdf.ContentType)
	}
	if pdf.Width != nil || pdf.Height != nil {
		t.Errorf("expected nil dimensions for null-dimension blob, got %v x %v", pdf.Width, pdf.Height)
	}
	if pdf.Previewable {
		t.Error("expected non-image attachment not previewable")
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
	assigneeIDs, ok := data["assignee_ids"].([]any)
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

	var data map[string]any
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

	var data map[string]any
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
		name      string
		status    string
		completed bool
	}{
		{name: "archived", status: "archived"},
		{name: "trashed", status: "trashed"},
		{name: "completed bool", completed: true},
		{name: "archived + completed", status: "archived", completed: true},
		{name: "empty", status: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &TodoListOptions{Status: tt.status, Completed: tt.completed}
			if opts.Status != tt.status {
				t.Errorf("expected status %q, got %q", tt.status, opts.Status)
			}
			if opts.Completed != tt.completed {
				t.Errorf("expected completed %t, got %t", tt.completed, opts.Completed)
			}
		})
	}
}

// TestCreateTodoRequest_CompletionSubscriberIDs tests that CompletionSubscriberIDs
// field serializes correctly.
func TestCreateTodoRequest_CompletionSubscriberIDs(t *testing.T) {
	req := CreateTodoRequest{
		Content:                 "Task with completion subscribers",
		CompletionSubscriberIDs: []int64{1049715920, 1049715915, 1049715914},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTodoRequest: %v", err)
	}

	data, err := unmarshalTodosWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Verify completion_subscriber_ids is present
	subscriberIDs, ok := data["completion_subscriber_ids"].([]any)
	if !ok {
		t.Fatalf("expected completion_subscriber_ids to be array, got %T", data["completion_subscriber_ids"])
	}
	if len(subscriberIDs) != 3 {
		t.Errorf("expected 3 completion_subscriber_ids, got %d", len(subscriberIDs))
	}

	// Verify IDs are preserved correctly
	expectedIDs := []int64{1049715920, 1049715915, 1049715914}
	for i, id := range subscriberIDs {
		num, ok := id.(json.Number)
		if !ok {
			t.Fatalf("expected completion_subscriber_ids[%d] to be json.Number, got %T", i, id)
		}
		parsed, err := num.Int64()
		if err != nil {
			t.Fatalf("failed to parse completion_subscriber_ids[%d]: %v", i, err)
		}
		if parsed != expectedIDs[i] {
			t.Errorf("expected completion_subscriber_ids[%d] = %d, got %d", i, expectedIDs[i], parsed)
		}
	}

	// Round-trip test
	var roundtrip CreateTodoRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}
	if len(roundtrip.CompletionSubscriberIDs) != 3 {
		t.Errorf("expected 3 completion_subscriber_ids after roundtrip, got %d", len(roundtrip.CompletionSubscriberIDs))
	}
}

// TestUpdateTodoRequest_CompletionSubscriberIDs tests that CompletionSubscriberIDs
// field serializes correctly in update requests.
func TestUpdateTodoRequest_CompletionSubscriberIDs(t *testing.T) {
	req := UpdateTodoRequest{
		Content:                 "Updated task with completion subscribers",
		CompletionSubscriberIDs: []int64{1049715920},
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateTodoRequest: %v", err)
	}

	data, err := unmarshalTodosWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	// Verify completion_subscriber_ids is present
	subscriberIDs, ok := data["completion_subscriber_ids"].([]any)
	if !ok {
		t.Fatalf("expected completion_subscriber_ids to be array, got %T", data["completion_subscriber_ids"])
	}
	if len(subscriberIDs) != 1 {
		t.Errorf("expected 1 completion_subscriber_id, got %d", len(subscriberIDs))
	}
}

// TestCreateTodoRequest_CompletionSubscriberIDs_Omitted tests that
// CompletionSubscriberIDs is omitted when empty (omitempty behavior).
func TestCreateTodoRequest_CompletionSubscriberIDs_Omitted(t *testing.T) {
	req := CreateTodoRequest{
		Content: "Task without completion subscribers",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTodoRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if _, ok := data["completion_subscriber_ids"]; ok {
		t.Error("expected completion_subscriber_ids to be omitted when empty")
	}
}

// -----------------------------------------------------------------------------
// Conversion function tests (todoFromGenerated)
// -----------------------------------------------------------------------------

// TestTodoFromGenerated_FullPopulated tests conversion with all fields set.
func TestTodoFromGenerated_FullPopulated(t *testing.T) {
	id := int64(12345)
	parentID := int64(11111)
	bucketID := int64(22222)
	creatorID := int64(33333)
	assigneeID := int64(44444)

	gt := generated.Todo{
		Id:             id,
		Status:         "active",
		Title:          "Test Todo",
		Type:           "Todo",
		Url:            "https://example.com/todo",
		AppUrl:         "https://example.com/app/todo",
		BookmarkUrl:    "https://example.com/bookmark",
		Content:        "Test content",
		Description:    "<div>Test description</div>",
		StartsOn:       types.Date{Year: 2024, Month: 1, Day: 15},
		DueOn:          types.Date{Year: 2024, Month: 2, Day: 28},
		Completed:      false,
		Position:       3,
		CreatedAt:      time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2024, 1, 5, 15, 30, 0, 0, time.UTC),
		InheritsStatus: true,
		Parent: generated.TodoParent{
			Id:     parentID,
			Title:  "Parent Todolist",
			Type:   "Todolist",
			Url:    "https://example.com/parent",
			AppUrl: "https://example.com/app/parent",
		},
		Bucket: generated.TodoBucket{
			Id:   bucketID,
			Name: "Test Project",
			Type: "Project",
		},
		Creator: generated.Person{
			Id:           types.FlexibleInt64(creatorID),
			Name:         "Test Creator",
			EmailAddress: "creator@example.com",
			AvatarUrl:    "https://example.com/avatar",
			Admin:        true,
			Owner:        true,
		},
		Assignees: []generated.Person{
			{
				Id:           types.FlexibleInt64(assigneeID),
				Name:         "Test Assignee",
				EmailAddress: "assignee@example.com",
			},
		},
	}

	todo := todoFromGenerated(gt)

	// Verify basic fields
	if todo.ID != id {
		t.Errorf("expected ID %d, got %d", id, todo.ID)
	}
	if todo.Status != "active" {
		t.Errorf("expected status 'active', got %q", todo.Status)
	}
	if todo.Title != "Test Todo" {
		t.Errorf("expected title 'Test Todo', got %q", todo.Title)
	}
	if todo.Type != "Todo" {
		t.Errorf("expected type 'Todo', got %q", todo.Type)
	}
	if todo.Content != "Test content" {
		t.Errorf("expected content 'Test content', got %q", todo.Content)
	}
	if todo.Description != "<div>Test description</div>" {
		t.Errorf("expected description with HTML, got %q", todo.Description)
	}

	// Verify date conversions
	if todo.StartsOn != "2024-01-15" {
		t.Errorf("expected starts_on '2024-01-15', got %q", todo.StartsOn)
	}
	if todo.DueOn != "2024-02-28" {
		t.Errorf("expected due_on '2024-02-28', got %q", todo.DueOn)
	}

	// Verify timestamps
	if todo.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if todo.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify Parent conversion
	if todo.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if todo.Parent.ID != parentID {
		t.Errorf("expected Parent.ID %d, got %d", parentID, todo.Parent.ID)
	}
	if todo.Parent.Title != "Parent Todolist" {
		t.Errorf("expected Parent.Title 'Parent Todolist', got %q", todo.Parent.Title)
	}

	// Verify Bucket conversion
	if todo.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if todo.Bucket.ID != bucketID {
		t.Errorf("expected Bucket.ID %d, got %d", bucketID, todo.Bucket.ID)
	}
	if todo.Bucket.Name != "Test Project" {
		t.Errorf("expected Bucket.Name 'Test Project', got %q", todo.Bucket.Name)
	}

	// Verify Creator conversion
	if todo.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if todo.Creator.ID != creatorID {
		t.Errorf("expected Creator.ID %d, got %d", creatorID, todo.Creator.ID)
	}
	if todo.Creator.Name != "Test Creator" {
		t.Errorf("expected Creator.Name 'Test Creator', got %q", todo.Creator.Name)
	}
	if !todo.Creator.Admin {
		t.Error("expected Creator.Admin to be true")
	}
	if !todo.Creator.Owner {
		t.Error("expected Creator.Owner to be true")
	}

	// Verify Assignees conversion
	if len(todo.Assignees) != 1 {
		t.Fatalf("expected 1 assignee, got %d", len(todo.Assignees))
	}
	if todo.Assignees[0].ID != assigneeID {
		t.Errorf("expected assignee ID %d, got %d", assigneeID, todo.Assignees[0].ID)
	}
	if todo.Assignees[0].Name != "Test Assignee" {
		t.Errorf("expected assignee name 'Test Assignee', got %q", todo.Assignees[0].Name)
	}

	// Verify other fields
	if todo.InheritsVis != true {
		t.Error("expected InheritsVis to be true")
	}
	if todo.Position != 3 {
		t.Errorf("expected position 3, got %d", todo.Position)
	}
}

// TestTodoFromGenerated_NilFields tests conversion with nil optional fields.
func TestTodoFromGenerated_NilFields(t *testing.T) {
	// Create a generated.Todo with zero ID and empty nested structs
	gt := generated.Todo{
		Id:      0, // zero ID
		Status:  "active",
		Title:   "Minimal Todo",
		Type:    "Todo",
		Content: "Content",
		Parent:  generated.TodoParent{}, // empty parent
		Bucket:  generated.TodoBucket{}, // empty bucket
		Creator: generated.Person{},     // empty creator
	}

	todo := todoFromGenerated(gt)

	// Zero ID should result in 0
	if todo.ID != 0 {
		t.Errorf("expected ID 0 for zero input, got %d", todo.ID)
	}

	// Empty nested structs should NOT create non-nil pointers
	// (the conversion checks for Id != nil || field != "")
	if todo.Parent != nil {
		t.Error("expected Parent to be nil for empty TodoParent")
	}
	if todo.Bucket != nil {
		t.Error("expected Bucket to be nil for empty TodoBucket")
	}
	if todo.Creator != nil {
		t.Error("expected Creator to be nil for empty Person")
	}
}

// TestTodoFromGenerated_ZeroDates tests conversion with zero/empty dates.
func TestTodoFromGenerated_ZeroDates(t *testing.T) {
	id := int64(12345)
	gt := generated.Todo{
		Id:       id,
		Status:   "active",
		Title:    "Todo without dates",
		Type:     "Todo",
		Content:  "Content",
		StartsOn: types.Date{}, // zero date
		DueOn:    types.Date{}, // zero date
	}

	todo := todoFromGenerated(gt)

	// Zero dates should result in empty strings
	if todo.StartsOn != "" {
		t.Errorf("expected empty starts_on for zero date, got %q", todo.StartsOn)
	}
	if todo.DueOn != "" {
		t.Errorf("expected empty due_on for zero date, got %q", todo.DueOn)
	}
}

// TestTodoFromGenerated_EmptyAssignees tests conversion with empty assignees array.
func TestTodoFromGenerated_EmptyAssignees(t *testing.T) {
	id := int64(12345)
	gt := generated.Todo{
		Id:        id,
		Status:    "active",
		Title:     "Todo without assignees",
		Type:      "Todo",
		Content:   "Content",
		Assignees: []generated.Person{}, // empty array
	}

	todo := todoFromGenerated(gt)

	// Empty assignees should remain nil or empty
	if len(todo.Assignees) != 0 {
		t.Errorf("expected 0 assignees, got %d", len(todo.Assignees))
	}
}

// TestTodoFromGenerated_MultipleAssignees tests conversion with multiple assignees.
func TestTodoFromGenerated_MultipleAssignees(t *testing.T) {
	id := int64(12345)
	id1 := int64(111)
	id2 := int64(222)
	id3 := int64(333)

	gt := generated.Todo{
		Id:      id,
		Status:  "active",
		Title:   "Todo with multiple assignees",
		Type:    "Todo",
		Content: "Content",
		Assignees: []generated.Person{
			{Id: types.FlexibleInt64(id1), Name: "Alice"},
			{Id: types.FlexibleInt64(id2), Name: "Bob"},
			{Id: types.FlexibleInt64(id3), Name: "Charlie"},
		},
	}

	todo := todoFromGenerated(gt)

	if len(todo.Assignees) != 3 {
		t.Fatalf("expected 3 assignees, got %d", len(todo.Assignees))
	}
	if todo.Assignees[0].Name != "Alice" {
		t.Errorf("expected assignee[0].Name 'Alice', got %q", todo.Assignees[0].Name)
	}
	if todo.Assignees[1].Name != "Bob" {
		t.Errorf("expected assignee[1].Name 'Bob', got %q", todo.Assignees[1].Name)
	}
	if todo.Assignees[2].Name != "Charlie" {
		t.Errorf("expected assignee[2].Name 'Charlie', got %q", todo.Assignees[2].Name)
	}
}

// TestTodoFromGenerated_CompletionSubscribers tests the nil-vs-empty contract:
// a server-sent [] becomes a non-nil zero-length slice, an absent property
// stays nil. Consumers doing fail-closed preservation depend on the
// distinction.
func TestTodoFromGenerated_CompletionSubscribers(t *testing.T) {
	// Present but empty: non-nil zero-length slice.
	gt := generated.Todo{
		Id:                    12345,
		Content:               "Content",
		CompletionSubscribers: []generated.Person{},
	}
	todo := todoFromGenerated(gt)
	if todo.CompletionSubscribers == nil {
		t.Error("expected non-nil CompletionSubscribers for server-sent []")
	}
	if len(todo.CompletionSubscribers) != 0 {
		t.Errorf("expected 0 completion subscribers, got %d", len(todo.CompletionSubscribers))
	}

	// Absent: stays nil.
	gt = generated.Todo{Id: 12345, Content: "Content"}
	todo = todoFromGenerated(gt)
	if todo.CompletionSubscribers != nil {
		t.Errorf("expected nil CompletionSubscribers for absent property, got %v", todo.CompletionSubscribers)
	}

	// Populated: mapped through personFromGenerated.
	gt = generated.Todo{
		Id:      12345,
		Content: "Content",
		CompletionSubscribers: []generated.Person{
			{Id: types.FlexibleInt64(555), Name: "Sub"},
		},
	}
	todo = todoFromGenerated(gt)
	if len(todo.CompletionSubscribers) != 1 || todo.CompletionSubscribers[0].ID != 555 {
		t.Errorf("expected completion subscriber 555, got %+v", todo.CompletionSubscribers)
	}
}

// TestTodoFromGenerated_DescriptionAttachments tests the nil-vs-empty
// contract for the description's inline files: a server-sent [] becomes a
// non-nil zero-length slice, an absent property stays nil. The API always
// sends the array, so clients enumerating inline files depend on the
// distinction the same way they do for CompletionSubscribers.
func TestTodoFromGenerated_DescriptionAttachments(t *testing.T) {
	// Present but empty: non-nil zero-length slice.
	gt := generated.Todo{
		Id:                     12345,
		Content:                "Content",
		DescriptionAttachments: []generated.RichTextAttachment{},
	}
	todo := todoFromGenerated(gt)
	if todo.DescriptionAttachments == nil {
		t.Error("expected non-nil DescriptionAttachments for server-sent []")
	}
	if len(todo.DescriptionAttachments) != 0 {
		t.Errorf("expected 0 description attachments, got %d", len(todo.DescriptionAttachments))
	}

	// Absent: stays nil.
	gt = generated.Todo{Id: 12345, Content: "Content"}
	todo = todoFromGenerated(gt)
	if todo.DescriptionAttachments != nil {
		t.Errorf("expected nil DescriptionAttachments for absent property, got %v", todo.DescriptionAttachments)
	}

	// Populated: every field is carried through. Id is a plain int64 (the
	// nine non-dimension fields are @required), and the generated
	// *types.FlexInt dimensions are narrowed into the public *int32.
	attachmentID := int64(987)
	w := types.FlexInt(800)
	h := types.FlexInt(600)
	gt = generated.Todo{
		Id:      12345,
		Content: "Content",
		DescriptionAttachments: []generated.RichTextAttachment{
			{
				Id:           attachmentID,
				Sgid:         "BAh7CEki",
				Filename:     "diagram.png",
				ContentType:  "image/png",
				ByteSize:     20480,
				DownloadUrl:  "https://example.com/download/diagram.png",
				Width:        &w,
				Height:       &h,
				Previewable:  true,
				PreviewUrl:   "https://example.com/preview/diagram.png",
				ThumbnailUrl: "https://example.com/thumb/diagram.png",
			},
		},
	}
	todo = todoFromGenerated(gt)
	if len(todo.DescriptionAttachments) != 1 {
		t.Fatalf("expected 1 description attachment, got %d", len(todo.DescriptionAttachments))
	}
	a := todo.DescriptionAttachments[0]
	if a.ID != attachmentID {
		t.Errorf("expected attachment ID %d, got %d", attachmentID, a.ID)
	}
	if a.SGID != "BAh7CEki" {
		t.Errorf("expected sgid 'BAh7CEki', got %q", a.SGID)
	}
	if a.Filename != "diagram.png" {
		t.Errorf("expected filename 'diagram.png', got %q", a.Filename)
	}
	if a.ContentType != "image/png" {
		t.Errorf("expected content_type 'image/png', got %q", a.ContentType)
	}
	if a.ByteSize != 20480 {
		t.Errorf("expected byte_size 20480, got %d", a.ByteSize)
	}
	if a.DownloadURL != "https://example.com/download/diagram.png" {
		t.Errorf("unexpected download_url %q", a.DownloadURL)
	}
	if a.Width == nil || *a.Width != 800 || a.Height == nil || *a.Height != 600 {
		t.Errorf("expected 800x600, got %v x %v", a.Width, a.Height)
	}
	if !a.Previewable {
		t.Error("expected previewable true")
	}
	if a.PreviewURL != "https://example.com/preview/diagram.png" {
		t.Errorf("unexpected preview_url %q", a.PreviewURL)
	}
	if a.ThumbnailURL != "https://example.com/thumb/diagram.png" {
		t.Errorf("unexpected thumbnail_url %q", a.ThumbnailURL)
	}

	// Nil generated dimensions (a non-image blob's null width/height) leave
	// the public *int32 dimensions nil rather than sentinel-zero.
	gt = generated.Todo{
		Id:      12345,
		Content: "Content",
		DescriptionAttachments: []generated.RichTextAttachment{
			{Id: 654, Filename: "spec.pdf", ContentType: "application/pdf"},
		},
	}
	todo = todoFromGenerated(gt)
	nonImage := todo.DescriptionAttachments[0]
	if nonImage.ID != 654 {
		t.Errorf("expected ID 654, got %d", nonImage.ID)
	}
	if nonImage.Width != nil || nonImage.Height != nil {
		t.Errorf("expected nil dimensions for non-image blob, got %v x %v", nonImage.Width, nonImage.Height)
	}
}

func TestTodoFromGenerated_CommentsCount(t *testing.T) {
	gt := generated.Todo{Id: 12345, Content: "Content", CommentsCount: 7}
	todo := todoFromGenerated(gt)
	if todo.CommentsCount != 7 {
		t.Errorf("expected CommentsCount 7, got %d", todo.CommentsCount)
	}
}

// TestTodoFromGenerated_PartialNestedFields tests conversion when nested structs
// have partial data (e.g., only ID set, or only name set).
func TestTodoFromGenerated_PartialNestedFields(t *testing.T) {
	parentID := int64(11111)
	creatorID := int64(33333)

	gt := generated.Todo{
		Status:  "active",
		Title:   "Todo with partial nested",
		Type:    "Todo",
		Content: "Content",
		Parent: generated.TodoParent{
			Id: parentID, // Only ID, no title
		},
		Bucket: generated.TodoBucket{
			Name: "Project Name", // Only name, no ID
		},
		Creator: generated.Person{
			Id: types.FlexibleInt64(creatorID), // Only ID, no name
		},
	}

	todo := todoFromGenerated(gt)

	// Parent should be created because ID is set
	if todo.Parent == nil {
		t.Fatal("expected Parent to be non-nil when ID is set")
	}
	if todo.Parent.ID != parentID {
		t.Errorf("expected Parent.ID %d, got %d", parentID, todo.Parent.ID)
	}
	if todo.Parent.Title != "" {
		t.Errorf("expected Parent.Title to be empty, got %q", todo.Parent.Title)
	}

	// Bucket should be created because Name is set
	if todo.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil when Name is set")
	}
	if todo.Bucket.Name != "Project Name" {
		t.Errorf("expected Bucket.Name 'Project Name', got %q", todo.Bucket.Name)
	}

	// Creator should be created because ID is set
	if todo.Creator == nil {
		t.Fatal("expected Creator to be non-nil when ID is set")
	}
	if todo.Creator.ID != creatorID {
		t.Errorf("expected Creator.ID %d, got %d", creatorID, todo.Creator.ID)
	}
}

// -----------------------------------------------------------------------------
// Service-level tests
// -----------------------------------------------------------------------------

func testTodosServer(t *testing.T, handler http.HandlerFunc) *TodosService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Todos()
}

func TestTodosService_List_QueryParameters(t *testing.T) {
	fixture := loadTodosFixture(t, "list.json")

	tests := []struct {
		name          string
		opts          *TodoListOptions
		wantStatus    string
		wantCompleted string
	}{
		{name: "nil options", opts: nil},
		{name: "completed bool", opts: &TodoListOptions{Completed: true}, wantCompleted: "true"},
		{name: "archived status", opts: &TodoListOptions{Status: "archived"}, wantStatus: "archived"},
		{name: "trashed status", opts: &TodoListOptions{Status: "trashed"}, wantStatus: "trashed"},
		{name: "archived + completed", opts: &TodoListOptions{Status: "archived", Completed: true}, wantStatus: "archived", wantCompleted: "true"},
		{name: "trashed + completed", opts: &TodoListOptions{Status: "trashed", Completed: true}, wantStatus: "trashed", wantCompleted: "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery url.Values
			svc := testTodosServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("expected GET, got %s", r.Method)
				}
				gotQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Total-Count", "2")
				w.WriteHeader(200)
				_, _ = w.Write(fixture)
			})

			result, err := svc.List(context.Background(), 1069479519, tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Todos) != 2 {
				t.Fatalf("expected 2 todos, got %d", len(result.Todos))
			}
			if tt.wantStatus == "" {
				if gotQuery.Has("status") {
					t.Fatalf("expected status to be absent, got %q", gotQuery.Get("status"))
				}
			} else if got := gotQuery.Get("status"); got != tt.wantStatus {
				t.Fatalf("status query = %q, want %q", got, tt.wantStatus)
			}
			if tt.wantCompleted == "" {
				if gotQuery.Has("completed") {
					t.Fatalf("expected completed to be absent, got %q", gotQuery.Get("completed"))
				}
			} else if got := gotQuery.Get("completed"); got != tt.wantCompleted {
				t.Fatalf("completed query = %q, want %q", got, tt.wantCompleted)
			}
		})
	}
}

// TestTodosService_Get_DescriptionAttachments exercises the full service
// path — wire JSON → generated.Todo (dimensions as *types.FlexInt) →
// todoFromGenerated → public Todo — and then re-encodes the result. It pins
// the two dimension behaviors that matter for `todos show --json`: the BC3
// API's float-spelled "width": 1024.0 normalizes to integer 1024, and a
// non-image blob's "width": null / "height": null re-encode as explicit
// null (the *int32 dimensions carry no omitempty), never a sentinel 0 or a
// dropped key. Every @required field survives the round-trip.
func TestTodosService_Get_DescriptionAttachments(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	svc := testTodosServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(fixture)
	})

	todo, err := svc.Get(context.Background(), 1069479520)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(todo.DescriptionAttachments) != 2 {
		t.Fatalf("expected 2 description attachments, got %d", len(todo.DescriptionAttachments))
	}
	img := todo.DescriptionAttachments[0]
	if img.ID != 1069480000 {
		t.Errorf("expected image attachment ID 1069480000, got %d", img.ID)
	}
	if img.Width == nil || *img.Width != 1024 {
		t.Errorf("expected image width 1024 (from float-spelled 1024.0), got %v", img.Width)
	}
	if img.Height == nil || *img.Height != 768 {
		t.Errorf("expected image height 768, got %v", img.Height)
	}
	pdf := todo.DescriptionAttachments[1]
	if pdf.Width != nil || pdf.Height != nil {
		t.Errorf("expected nil dimensions for non-image blob, got %v x %v", pdf.Width, pdf.Height)
	}

	// Re-encode and confirm the wire shape is faithful.
	out, err := json.Marshal(todo)
	if err != nil {
		t.Fatalf("failed to marshal todo: %v", err)
	}
	decoded, err := unmarshalTodosWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to decode marshaled todo: %v", err)
	}
	atts, ok := decoded["description_attachments"].([]any)
	if !ok || len(atts) != 2 {
		t.Fatalf("expected 2 marshaled description_attachments, got %T %v", decoded["description_attachments"], decoded["description_attachments"])
	}

	first, ok := atts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first attachment to be an object, got %T", atts[0])
	}
	if num, ok := first["width"].(json.Number); !ok || num.String() != "1024" {
		t.Errorf("expected marshaled image width 1024, got %v (%T)", first["width"], first["width"])
	}
	for _, k := range []string{"id", "sgid", "filename", "content_type", "byte_size", "download_url", "previewable", "preview_url", "thumbnail_url"} {
		if _, present := first[k]; !present {
			t.Errorf("expected marshaled attachment to carry required field %q", k)
		}
	}

	second, ok := atts[1].(map[string]any)
	if !ok {
		t.Fatalf("expected second attachment to be an object, got %T", atts[1])
	}
	wv, present := second["width"]
	if !present {
		t.Error("expected non-image attachment to keep an explicit width key")
	}
	if wv != nil {
		t.Errorf("expected non-image width to marshal as null, got %v (%T)", wv, wv)
	}
	hv, present := second["height"]
	if !present {
		t.Error("expected non-image attachment to keep an explicit height key")
	}
	if hv != nil {
		t.Errorf("expected non-image height to marshal as null, got %v (%T)", hv, hv)
	}
}

// TestTodosService_Get_DescriptionAttachmentsEmptyPreserved pins the
// nil-vs-empty contract through the service path: a server-sent [] decodes to
// a non-nil zero-length slice and re-encodes as [] (not dropped, not null).
func TestTodosService_Get_DescriptionAttachmentsEmptyPreserved(t *testing.T) {
	base := loadTodosFixture(t, "get.json")
	fixture := patchTodoFixture(t, base, map[string]any{
		"description_attachments": []any{},
	})
	svc := testTodosServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write(fixture)
	})

	todo, err := svc.Get(context.Background(), 1069479520)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todo.DescriptionAttachments == nil {
		t.Error("expected non-nil empty slice for server-sent []")
	}
	if len(todo.DescriptionAttachments) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(todo.DescriptionAttachments))
	}

	out, err := json.Marshal(todo)
	if err != nil {
		t.Fatalf("failed to marshal todo: %v", err)
	}
	decoded, err := unmarshalTodosWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to decode marshaled todo: %v", err)
	}
	atts, ok := decoded["description_attachments"].([]any)
	if !ok {
		t.Fatalf("expected description_attachments array, got %T", decoded["description_attachments"])
	}
	if len(atts) != 0 {
		t.Errorf("expected [] preserved on re-encode, got %v", atts)
	}
}

func TestTodosService_List_RejectsInvalidStatus(t *testing.T) {
	// Server must never be reached — validation happens before the request.
	svc := testTodosServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called for invalid status; got %s %s", r.Method, r.URL.Path)
	})

	for _, status := range []string{"completed", "pending", "active", "something-else"} {
		t.Run(status, func(t *testing.T) {
			_, err := svc.List(context.Background(), 1069479519, &TodoListOptions{Status: status})
			if err == nil {
				t.Fatalf("expected usage error for Status=%q, got nil", status)
			}
			apiErr, ok := errors.AsType[*Error](err)
			if !ok || apiErr.Code != CodeUsage {
				t.Fatalf("expected CodeUsage for Status=%q, got %T %v", status, err, err)
			}
		})
	}
}

// patchTodoFixture returns the fixture JSON with the given fields replaced.
func patchTodoFixture(t *testing.T, base []byte, patch map[string]any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	for k, v := range patch {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal patched fixture: %v", err)
	}
	return b
}

// capturedTodoRequest records one request seen by testTodosCaptureServer.
type capturedTodoRequest struct {
	method string
	path   string
	body   map[string]any
}

// testTodosCaptureServer serves getBody for GETs and putBody for PUTs while
// recording every request's method, path, and (for PUTs) decoded body.
// The extra hooks, when non-nil, are installed on the client.
func testTodosCaptureServer(t *testing.T, getBody, putBody []byte, hooks Hooks) (*TodosService, *[]capturedTodoRequest) {
	t.Helper()
	reqs := &[]capturedTodoRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cr := capturedTodoRequest{method: r.Method, path: r.URL.Path}
		if r.Method == "PUT" {
			cr.body = decodeRequestBody(t, r)
		}
		*reqs = append(*reqs, cr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if r.Method == "GET" {
			w.Write(getBody)
		} else {
			w.Write(putBody)
		}
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	var opts []ClientOption
	if hooks != nil {
		opts = append(opts, WithHooks(hooks))
	}
	client := NewClient(cfg, token, opts...)
	return client.ForAccount("99999").Todos(), reqs
}

func TestTodosService_UpdateMergesUnsetFields(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	getBody := patchTodoFixture(t, fixture, map[string]any{
		"description": "<div>existing description</div>",
	})
	svc, reqs := testTodosCaptureServer(t, getBody, fixture, nil)

	// Content-only update: everything else must be carried over from the GET.
	todo, err := svc.Update(context.Background(), 1069479520, &UpdateTodoRequest{
		Content: "new title",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todo.ID != 1069479520 {
		t.Errorf("expected ID 1069479520, got %d", todo.ID)
	}

	if len(*reqs) != 2 {
		t.Fatalf("expected 2 requests (GET then PUT), got %d", len(*reqs))
	}
	if (*reqs)[0].method != "GET" || (*reqs)[1].method != "PUT" {
		t.Fatalf("expected GET then PUT, got %s then %s", (*reqs)[0].method, (*reqs)[1].method)
	}

	body := (*reqs)[1].body
	if body["content"] != "new title" {
		t.Errorf("expected content 'new title', got %v", body["content"])
	}
	if body["description"] != "<div>existing description</div>" {
		t.Errorf("expected preserved description, got %v", body["description"])
	}
	if body["due_on"] != "2022-12-01" {
		t.Errorf("expected preserved due_on 2022-12-01, got %v", body["due_on"])
	}
	// Assignees from the GET are carried over as assignee_ids (Person → id).
	ids, ok := body["assignee_ids"].([]any)
	if !ok || len(ids) != 1 || ids[0].(json.Number).String() != "1049715920" {
		t.Errorf("expected preserved assignee_ids [1049715920], got %v", body["assignee_ids"])
	}
	// completion_subscribers is [] in the fixture — sent as an explicit empty list.
	subs, ok := body["completion_subscriber_ids"].([]any)
	if !ok || len(subs) != 0 {
		t.Errorf("expected completion_subscriber_ids [], got %v", body["completion_subscriber_ids"])
	}
	// Notify was not requested — never carried from the GET, never sent.
	if _, ok := body["notify"]; ok {
		t.Errorf("expected notify to be omitted, got %v", body["notify"])
	}
}

func TestTodosService_UpdateClearsAssignees(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	svc, reqs := testTodosCaptureServer(t, fixture, fixture, nil)

	// An empty non-nil slice means "clear all assignees" — this must be sent
	// to the API as assignee_ids:[], not omitted.
	_, err := svc.Update(context.Background(), 1069479520, &UpdateTodoRequest{
		AssigneeIDs: []int64{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	ids, ok := body["assignee_ids"]
	if !ok {
		t.Fatal("expected assignee_ids to be present in request body, but it was omitted")
	}
	arr, ok := ids.([]any)
	if !ok {
		t.Fatalf("expected assignee_ids to be an array, got %T", ids)
	}
	if len(arr) != 0 {
		t.Errorf("expected empty assignee_ids array, got %v", arr)
	}
	// Content was not passed — carried over from the GET, not dropped.
	if body["content"] != "Program Leto locator  microcontroller unit" {
		t.Errorf("expected preserved content, got %v", body["content"])
	}
}

func TestTodosService_UpdateClearsCompletionSubscribers(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	getBody := patchTodoFixture(t, fixture, map[string]any{
		"completion_subscribers": []map[string]any{{"id": 555, "name": "Sub"}},
	})
	svc, reqs := testTodosCaptureServer(t, getBody, fixture, nil)

	// An empty non-nil slice means "clear all completion subscribers" — this must
	// be sent to the API as completion_subscriber_ids:[], not omitted.
	_, err := svc.Update(context.Background(), 1069479520, &UpdateTodoRequest{
		CompletionSubscriberIDs: []int64{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	ids, ok := body["completion_subscriber_ids"]
	if !ok {
		t.Fatal("expected completion_subscriber_ids to be present in request body, but it was omitted")
	}
	arr, ok := ids.([]any)
	if !ok {
		t.Fatalf("expected completion_subscriber_ids to be an array, got %T", ids)
	}
	if len(arr) != 0 {
		t.Errorf("expected empty completion_subscriber_ids array, got %v", arr)
	}
}

func TestTodosService_UpdateNotifyOnlyWhenTrue(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	svc, reqs := testTodosCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Update(context.Background(), 1069479520, &UpdateTodoRequest{
		Content: "notify them",
		Notify:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	if body["notify"] != true {
		t.Errorf("expected notify true in body, got %v", body["notify"])
	}
}

func TestTodosService_UpdateHooksObserveGetAndReplace(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, _ := testTodosCaptureServer(t, fixture, fixture, recorder)

	_, err := svc.Update(context.Background(), 1069479520, &UpdateTodoRequest{Content: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ops := make([]string, 0, len(recorder.opStartCalls))
	for _, op := range recorder.opStartCalls {
		ops = append(ops, op.Service+"."+op.Operation)
	}
	if len(ops) != 2 || ops[0] != "Todos.Get" || ops[1] != "Todos.Replace" {
		t.Errorf("expected operations [Todos.Get Todos.Replace], got %v", ops)
	}
	if len(recorder.opEndCalls) != 2 {
		t.Errorf("expected 2 OnOperationEnd calls, got %d", len(recorder.opEndCalls))
	}
}

func TestTodosService_Edit(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	getBody := patchTodoFixture(t, fixture, map[string]any{
		"description": "<div>keep me</div>",
	})
	svc, reqs := testTodosCaptureServer(t, getBody, fixture, nil)

	todo, err := svc.Edit(context.Background(), 1069479520, func(f *TodoFields) error {
		f.Content = "🚨 " + f.Content
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todo.ID != 1069479520 {
		t.Errorf("expected ID 1069479520, got %d", todo.ID)
	}

	body := (*reqs)[len(*reqs)-1].body
	if body["content"] != "🚨 Program Leto locator  microcontroller unit" {
		t.Errorf("expected prefixed content, got %v", body["content"])
	}
	if body["description"] != "<div>keep me</div>" {
		t.Errorf("expected preserved description, got %v", body["description"])
	}
}

func TestTodosService_EditClearsDateByOmission(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	svc, reqs := testTodosCaptureServer(t, fixture, fixture, nil)

	// Clearing a date means setting it empty; the wire encoding is omission
	// (the server clears an omitted date, and "" would be a format error).
	_, err := svc.Edit(context.Background(), 1069479520, func(f *TodoFields) error {
		if f.DueOn != "2022-12-01" {
			t.Errorf("expected DueOn from GET, got %q", f.DueOn)
		}
		f.DueOn = ""
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	if _, ok := body["due_on"]; ok {
		t.Errorf("expected due_on omitted from PUT body, got %v", body["due_on"])
	}
	// Non-date fields are still sent in full.
	if body["content"] != "Program Leto locator  microcontroller unit" {
		t.Errorf("expected preserved content, got %v", body["content"])
	}
}

func TestTodosService_EditClearsDescriptionAndIDsExplicitly(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	getBody := patchTodoFixture(t, fixture, map[string]any{
		"description": "<div>old</div>",
	})
	svc, reqs := testTodosCaptureServer(t, getBody, fixture, nil)

	_, err := svc.Edit(context.Background(), 1069479520, func(f *TodoFields) error {
		f.Description = ""
		f.AssigneeIDs = nil
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clears are present-and-empty in the PUT body — the full-state builder
	// always emits description and both ID lists.
	body := (*reqs)[len(*reqs)-1].body
	desc, ok := body["description"]
	if !ok || desc != "" {
		t.Errorf("expected empty description present in body, got %v (present=%v)", desc, ok)
	}
	ids, ok := body["assignee_ids"].([]any)
	if !ok || len(ids) != 0 {
		t.Errorf("expected assignee_ids [] present in body, got %v", body["assignee_ids"])
	}
	subs, ok := body["completion_subscriber_ids"].([]any)
	if !ok || len(subs) != 0 {
		t.Errorf("expected completion_subscriber_ids [] present in body, got %v", body["completion_subscriber_ids"])
	}
}

func TestTodosService_EditClosureErrorAbortsWithoutPUT(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	svc, reqs := testTodosCaptureServer(t, fixture, fixture, nil)

	wantErr := errors.New("nope")
	_, err := svc.Edit(context.Background(), 1069479520, func(f *TodoFields) error {
		f.Content = "should never be written"
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected closure error, got %v", err)
	}

	for _, r := range *reqs {
		if r.method == "PUT" {
			t.Fatal("expected no PUT after closure error")
		}
	}
}

func TestTodosService_EditHooksObserveGetAndReplace(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, _ := testTodosCaptureServer(t, fixture, fixture, recorder)

	_, err := svc.Edit(context.Background(), 1069479520, func(f *TodoFields) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ops := make([]string, 0, len(recorder.opStartCalls))
	for _, op := range recorder.opStartCalls {
		ops = append(ops, op.Service+"."+op.Operation)
	}
	if len(ops) != 2 || ops[0] != "Todos.Get" || ops[1] != "Todos.Replace" {
		t.Errorf("expected operations [Todos.Get Todos.Replace], got %v", ops)
	}
}

func TestTodosService_ReplaceSendsSparseVerbatim(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, reqs := testTodosCaptureServer(t, fixture, fixture, recorder)

	todo, err := svc.Replace(context.Background(), 1069479520, &ReplaceTodoRequest{
		Content: "the whole new todo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todo.ID != 1069479520 {
		t.Errorf("expected ID 1069479520, got %d", todo.ID)
	}

	// No GET: replace is the server-native verbatim PUT.
	if len(*reqs) != 1 || (*reqs)[0].method != "PUT" {
		t.Fatalf("expected exactly one PUT, got %+v", *reqs)
	}
	body := (*reqs)[0].body
	if body["content"] != "the whole new todo" {
		t.Errorf("expected content in body, got %v", body["content"])
	}
	// Unset fields are omitted — the server clears them.
	for _, field := range []string{"description", "assignee_ids", "completion_subscriber_ids", "notify", "due_on", "starts_on"} {
		if _, ok := body[field]; ok {
			t.Errorf("expected %q omitted from sparse replace, got %v", field, body[field])
		}
	}

	// Hooks observe a single Todos.Replace operation.
	if len(recorder.opStartCalls) != 1 ||
		recorder.opStartCalls[0].Service != "Todos" || recorder.opStartCalls[0].Operation != "Replace" {
		t.Errorf("expected single Todos.Replace operation, got %+v", recorder.opStartCalls)
	}
}

func TestTodosService_ReplaceRequiresContent(t *testing.T) {
	fixture := loadTodosFixture(t, "get.json")
	svc, reqs := testTodosCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Replace(context.Background(), 1069479520, &ReplaceTodoRequest{})
	if err == nil {
		t.Fatal("expected usage error for missing content")
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestTodosService_Reposition(t *testing.T) {
	var receivedBody map[string]any
	svc := testTodosServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		receivedBody = decodeRequestBody(t, r)
		w.WriteHeader(204)
	})

	err := svc.Reposition(context.Background(), 1069479520, 3, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fmt.Sprint(receivedBody["position"]) != "3" {
		t.Errorf("expected position 3, got %v", receivedBody["position"])
	}
	if _, exists := receivedBody["parent_id"]; exists {
		t.Error("expected parent_id to be omitted when nil")
	}
}

func TestTodosService_RepositionWithParentID(t *testing.T) {
	var receivedBody map[string]any
	svc := testTodosServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)
		w.WriteHeader(204)
	})

	parentID := int64(99999)
	err := svc.Reposition(context.Background(), 1069479520, 1, &parentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fmt.Sprint(receivedBody["position"]) != "1" {
		t.Errorf("expected position 1, got %v", receivedBody["position"])
	}
	if fmt.Sprint(receivedBody["parent_id"]) != "99999" {
		t.Errorf("expected parent_id 99999, got %v", receivedBody["parent_id"])
	}
}
