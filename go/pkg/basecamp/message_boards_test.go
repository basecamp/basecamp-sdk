package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func messageBoardsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "message_boards")
}

func loadMessageBoardsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(messageBoardsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestMessageBoard_UnmarshalGet(t *testing.T) {
	data := loadMessageBoardsFixture(t, "get.json")

	var board MessageBoard
	if err := json.Unmarshal(data, &board); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if board.ID != 1069479338 {
		t.Errorf("expected ID 1069479338, got %d", board.ID)
	}
	if board.Status != "active" {
		t.Errorf("expected status 'active', got %q", board.Status)
	}
	if board.Type != "Message::Board" {
		t.Errorf("expected type 'Message::Board', got %q", board.Type)
	}
	if board.Title != "Message Board" {
		t.Errorf("expected title 'Message Board', got %q", board.Title)
	}
	if board.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/message_boards/1069479338.json" {
		t.Errorf("unexpected URL: %q", board.URL)
	}
	if board.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/message_boards/1069479338" {
		t.Errorf("unexpected AppURL: %q", board.AppURL)
	}

	// Verify messages count and URL
	if board.MessagesCount != 12 {
		t.Errorf("expected MessagesCount 12, got %d", board.MessagesCount)
	}
	if board.MessagesURL != "https://3.basecampapi.com/195539477/buckets/2085958499/message_boards/1069479338/messages.json" {
		t.Errorf("unexpected MessagesURL: %q", board.MessagesURL)
	}

	// Verify timestamps are parsed
	if board.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if board.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify bucket
	if board.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if board.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", board.Bucket.ID)
	}
	if board.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", board.Bucket.Name)
	}
	if board.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", board.Bucket.Type)
	}

	// Verify creator
	if board.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if board.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", board.Creator.ID)
	}
	if board.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", board.Creator.Name)
	}
	if board.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", board.Creator.EmailAddress)
	}
	if !board.Creator.Owner {
		t.Error("expected Creator.Owner to be true")
	}
	if !board.Creator.Admin {
		t.Error("expected Creator.Admin to be true")
	}
}
