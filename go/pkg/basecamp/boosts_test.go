package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func boostsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "boosts")
}

func loadBoostsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(boostsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestBoost_UnmarshalList(t *testing.T) {
	data := loadBoostsFixture(t, "list.json")

	var boosts []Boost
	if err := json.Unmarshal(data, &boosts); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(boosts) != 2 {
		t.Errorf("expected 2 boosts, got %d", len(boosts))
	}

	// Verify first boost
	b1 := boosts[0]
	if b1.ID != 1069479500 {
		t.Errorf("expected ID 1069479500, got %d", b1.ID)
	}
	if b1.Content != "🎉" {
		t.Errorf("expected content '🎉', got %q", b1.Content)
	}
	if b1.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}

	// Verify booster
	if b1.Booster == nil {
		t.Fatal("expected Booster to be non-nil")
	}
	if b1.Booster.ID != 1049715914 {
		t.Errorf("expected Booster.ID 1049715914, got %d", b1.Booster.ID)
	}
	if b1.Booster.Name != "Victor Cooper" {
		t.Errorf("expected Booster.Name 'Victor Cooper', got %q", b1.Booster.Name)
	}

	// Verify recording
	if b1.Recording == nil {
		t.Fatal("expected Recording to be non-nil")
	}
	if b1.Recording.ID != 1069479345 {
		t.Errorf("expected Recording.ID 1069479345, got %d", b1.Recording.ID)
	}
	if b1.Recording.Title != "Hello everyone!" {
		t.Errorf("expected Recording.Title 'Hello everyone!', got %q", b1.Recording.Title)
	}
	if b1.Recording.Type != "Chat::Lines::Text" {
		t.Errorf("expected Recording.Type 'Chat::Lines::Text', got %q", b1.Recording.Type)
	}

	// Verify second boost
	b2 := boosts[1]
	if b2.ID != 1069479501 {
		t.Errorf("expected ID 1069479501, got %d", b2.ID)
	}
	if b2.Content != "👍" {
		t.Errorf("expected content '👍', got %q", b2.Content)
	}
	if b2.Booster == nil {
		t.Fatal("expected Booster to be non-nil for second boost")
	}
	if b2.Booster.Name != "Annie Bryan" {
		t.Errorf("expected Booster.Name 'Annie Bryan', got %q", b2.Booster.Name)
	}
}

func TestBoost_UnmarshalGet(t *testing.T) {
	data := loadBoostsFixture(t, "get.json")

	var boost Boost
	if err := json.Unmarshal(data, &boost); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if boost.ID != 1069479500 {
		t.Errorf("expected ID 1069479500, got %d", boost.ID)
	}
	if boost.Content != "🎉" {
		t.Errorf("expected content '🎉', got %q", boost.Content)
	}

	// Verify timestamps are parsed
	if boost.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}

	// Verify booster with full details
	if boost.Booster == nil {
		t.Fatal("expected Booster to be non-nil")
	}
	if boost.Booster.ID != 1049715914 {
		t.Errorf("expected Booster.ID 1049715914, got %d", boost.Booster.ID)
	}
	if boost.Booster.Name != "Victor Cooper" {
		t.Errorf("expected Booster.Name 'Victor Cooper', got %q", boost.Booster.Name)
	}
	if boost.Booster.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Booster.EmailAddress 'victor@honchodesign.com', got %q", boost.Booster.EmailAddress)
	}

	// Verify recording
	if boost.Recording == nil {
		t.Fatal("expected Recording to be non-nil")
	}
	if boost.Recording.ID != 1069479345 {
		t.Errorf("expected Recording.ID 1069479345, got %d", boost.Recording.ID)
	}
	if boost.Recording.Title != "Hello everyone!" {
		t.Errorf("expected Recording.Title 'Hello everyone!', got %q", boost.Recording.Title)
	}
	if boost.Recording.Type != "Chat::Lines::Text" {
		t.Errorf("expected Recording.Type 'Chat::Lines::Text', got %q", boost.Recording.Type)
	}
	if boost.Recording.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/chats/1069479345/lines/1069479350.json" {
		t.Errorf("unexpected Recording.URL: %q", boost.Recording.URL)
	}
	if boost.Recording.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/chats/1069479345/lines/1069479350" {
		t.Errorf("unexpected Recording.AppURL: %q", boost.Recording.AppURL)
	}
}

// newTestBoostsService creates a BoostsService with minimal wiring for
// testing validation logic that runs before the generated client call.
func newTestBoostsService() *BoostsService {
	c := &Client{hooks: NoopHooks{}}
	ac := &AccountClient{parent: c, accountID: "99999"}
	return NewBoostsService(ac)
}

func TestCreateRecordingBoost_EmptyContent(t *testing.T) {
	svc := newTestBoostsService()
	_, err := svc.CreateRecording(context.Background(), 1, 2, "")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeUsage {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestCreateEventBoost_EmptyContent(t *testing.T) {
	svc := newTestBoostsService()
	_, err := svc.CreateEvent(context.Background(), 1, 2, 3, "")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeUsage {
		t.Errorf("expected usage error, got: %v", err)
	}
}

func TestCreateRecordingBoost_ValidContent(t *testing.T) {
	svc := newTestBoostsService()
	// Should pass validation. With a nil gen client the call panics
	// after validation, which proves content was accepted.
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil gen client panics after passing validation
		}
	}()
	_, err := svc.CreateRecording(context.Background(), 1, 2, "🎉")
	if err != nil {
		var apiErr *Error
		if errors.As(err, &apiErr) && apiErr.Code == CodeUsage {
			t.Errorf("valid content should pass validation, got usage error: %v", err)
		}
	}
}

func TestCreateEventBoost_ValidContent(t *testing.T) {
	svc := newTestBoostsService()
	// Should pass validation. With a nil gen client the call panics
	// after validation, which proves content was accepted.
	defer func() {
		if r := recover(); r != nil {
			// Expected: nil gen client panics after passing validation
		}
	}()
	_, err := svc.CreateEvent(context.Background(), 1, 2, 3, "👍")
	if err != nil {
		var apiErr *Error
		if errors.As(err, &apiErr) && apiErr.Code == CodeUsage {
			t.Errorf("valid content should pass validation, got usage error: %v", err)
		}
	}
}

