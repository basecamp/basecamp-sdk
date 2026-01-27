package basecamp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// unmarshalWithNumbers decodes JSON into a map preserving numbers as json.Number
// which can be cleanly converted to int64 without float64 precision loss.
func unmarshalWithNumbers(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return result, decoder.Decode(&result)
}

func messagesFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "messages")
}

func loadMessagesFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(messagesFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestMessage_UnmarshalList(t *testing.T) {
	data := loadMessagesFixture(t, "list.json")

	var messages []Message
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	// Verify first message
	m1 := messages[0]
	if m1.ID != 1069479351 {
		t.Errorf("expected ID 1069479351, got %d", m1.ID)
	}
	if m1.Status != "active" {
		t.Errorf("expected status 'active', got %q", m1.Status)
	}
	if m1.Type != "Message" {
		t.Errorf("expected type 'Message', got %q", m1.Type)
	}
	if m1.Subject != "We won Leto!" {
		t.Errorf("expected subject 'We won Leto!', got %q", m1.Subject)
	}
	if m1.Content != "<div>Hello everyone! We got the Leto Laptop project! Time to get started.</div>" {
		t.Errorf("unexpected content: %q", m1.Content)
	}
	if m1.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/messages/1069479351.json" {
		t.Errorf("unexpected URL: %q", m1.URL)
	}
	if m1.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/messages/1069479351" {
		t.Errorf("unexpected AppURL: %q", m1.AppURL)
	}

	// Verify parent (message board)
	if m1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if m1.Parent.ID != 1069479338 {
		t.Errorf("expected Parent.ID 1069479338, got %d", m1.Parent.ID)
	}
	if m1.Parent.Title != "Message Board" {
		t.Errorf("expected Parent.Title 'Message Board', got %q", m1.Parent.Title)
	}
	if m1.Parent.Type != "Message::Board" {
		t.Errorf("expected Parent.Type 'Message::Board', got %q", m1.Parent.Type)
	}

	// Verify bucket
	if m1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if m1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", m1.Bucket.ID)
	}
	if m1.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", m1.Bucket.Name)
	}
	if m1.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", m1.Bucket.Type)
	}

	// Verify creator
	if m1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if m1.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", m1.Creator.ID)
	}
	if m1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", m1.Creator.Name)
	}

	// Verify category (message type)
	if m1.Category == nil {
		t.Fatal("expected Category to be non-nil")
	}
	if m1.Category.ID != 1069479340 {
		t.Errorf("expected Category.ID 1069479340, got %d", m1.Category.ID)
	}
	if m1.Category.Name != "Announcement" {
		t.Errorf("expected Category.Name 'Announcement', got %q", m1.Category.Name)
	}

	// Verify second message
	m2 := messages[1]
	if m2.ID != 1069479360 {
		t.Errorf("expected ID 1069479360, got %d", m2.ID)
	}
	if m2.Subject != "Kickoff meeting scheduled" {
		t.Errorf("expected subject 'Kickoff meeting scheduled', got %q", m2.Subject)
	}
	if m2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second message")
	}
	if m2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", m2.Creator.Name)
	}
	// Verify creator with company
	if m2.Creator.Company == nil {
		t.Fatal("expected Creator.Company to be non-nil for second message")
	}
	if m2.Creator.Company.Name != "Honcho Design" {
		t.Errorf("expected Creator.Company.Name 'Honcho Design', got %q", m2.Creator.Company.Name)
	}
	// Second message has no category
	if m2.Category != nil {
		t.Errorf("expected second message Category to be nil, got %+v", m2.Category)
	}
}

func TestMessage_UnmarshalGet(t *testing.T) {
	data := loadMessagesFixture(t, "get.json")

	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if message.ID != 1069479351 {
		t.Errorf("expected ID 1069479351, got %d", message.ID)
	}
	if message.Status != "active" {
		t.Errorf("expected status 'active', got %q", message.Status)
	}
	if message.Type != "Message" {
		t.Errorf("expected type 'Message', got %q", message.Type)
	}
	if message.Subject != "We won Leto!" {
		t.Errorf("expected subject 'We won Leto!', got %q", message.Subject)
	}
	expectedContent := "<div>Hello everyone! We got the Leto Laptop project! Time to get started.</div>"
	if message.Content != expectedContent {
		t.Errorf("unexpected content: %q", message.Content)
	}

	// Verify timestamps are parsed
	if message.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if message.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator with full details
	if message.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if message.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", message.Creator.ID)
	}
	if message.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", message.Creator.Name)
	}
	if message.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", message.Creator.EmailAddress)
	}
	if message.Creator.Title != "Chief Strategist" {
		t.Errorf("expected Creator.Title 'Chief Strategist', got %q", message.Creator.Title)
	}
	if !message.Creator.Owner {
		t.Error("expected Creator.Owner to be true")
	}
	if !message.Creator.Admin {
		t.Error("expected Creator.Admin to be true")
	}

	// Verify category
	if message.Category == nil {
		t.Fatal("expected Category to be non-nil")
	}
	if message.Category.Name != "Announcement" {
		t.Errorf("expected Category.Name 'Announcement', got %q", message.Category.Name)
	}
}

func TestCreateMessageRequest_Marshal(t *testing.T) {
	req := CreateMessageRequest{
		Subject:    "Project update",
		Content:    "<div>Here's our weekly update...</div>",
		Status:     "active",
		CategoryID: 1069479340,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateMessageRequest: %v", err)
	}

	data, err := unmarshalWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["subject"] != "Project update" {
		t.Errorf("unexpected subject: %v", data["subject"])
	}
	if data["content"] != "<div>Here's our weekly update...</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}
	if data["status"] != "active" {
		t.Errorf("unexpected status: %v", data["status"])
	}
	// CategoryID should be serialized as category_id
	categoryID, _ := data["category_id"].(json.Number).Int64()
	if categoryID != 1069479340 {
		t.Errorf("unexpected category_id: %v", data["category_id"])
	}

	// Round-trip test
	var roundtrip CreateMessageRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Subject != req.Subject {
		t.Errorf("expected subject %q, got %q", req.Subject, roundtrip.Subject)
	}
	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
	if roundtrip.CategoryID != req.CategoryID {
		t.Errorf("expected category_id %d, got %d", req.CategoryID, roundtrip.CategoryID)
	}
}

func TestCreateMessageRequest_MarshalMinimal(t *testing.T) {
	// Test with only required field
	req := CreateMessageRequest{
		Subject: "Quick note",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateMessageRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["subject"] != "Quick note" {
		t.Errorf("unexpected subject: %v", data["subject"])
	}
	// Optional fields with omitempty should not be present
	if _, ok := data["content"]; ok {
		t.Error("expected content to be omitted")
	}
	if _, ok := data["status"]; ok {
		t.Error("expected status to be omitted")
	}
	if _, ok := data["category_id"]; ok {
		t.Error("expected category_id to be omitted")
	}
}

func TestUpdateMessageRequest_Marshal(t *testing.T) {
	req := UpdateMessageRequest{
		Subject:    "Updated subject",
		Content:    "<div>Updated content</div>",
		Status:     "drafted",
		CategoryID: 1069479341,
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMessageRequest: %v", err)
	}

	data, err := unmarshalWithNumbers(out)
	if err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["subject"] != "Updated subject" {
		t.Errorf("unexpected subject: %v", data["subject"])
	}
	if data["content"] != "<div>Updated content</div>" {
		t.Errorf("unexpected content: %v", data["content"])
	}
	if data["status"] != "drafted" {
		t.Errorf("unexpected status: %v", data["status"])
	}
	categoryID, _ := data["category_id"].(json.Number).Int64()
	if categoryID != 1069479341 {
		t.Errorf("unexpected category_id: %v", data["category_id"])
	}

	// Round-trip test
	var roundtrip UpdateMessageRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Subject != req.Subject {
		t.Errorf("expected subject %q, got %q", req.Subject, roundtrip.Subject)
	}
	if roundtrip.Content != req.Content {
		t.Errorf("expected content %q, got %q", req.Content, roundtrip.Content)
	}
}

func TestUpdateMessageRequest_MarshalPartial(t *testing.T) {
	// Test with only some fields
	req := UpdateMessageRequest{
		Subject: "Just updating subject",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMessageRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["subject"] != "Just updating subject" {
		t.Errorf("unexpected subject: %v", data["subject"])
	}
	// Optional fields should be omitted
	if _, ok := data["content"]; ok {
		t.Error("expected content to be omitted")
	}
	if _, ok := data["status"]; ok {
		t.Error("expected status to be omitted")
	}
	if _, ok := data["category_id"]; ok {
		t.Error("expected category_id to be omitted")
	}
}
