package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func messageTypesFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "message_types")
}

func loadMessageTypesFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(messageTypesFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestMessageType_UnmarshalList(t *testing.T) {
	data := loadMessageTypesFixture(t, "list.json")

	var types []MessageType
	if err := json.Unmarshal(data, &types); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(types) != 4 {
		t.Errorf("expected 4 message types, got %d", len(types))
	}

	// Verify first type
	t1 := types[0]
	if t1.ID != 1069479340 {
		t.Errorf("expected ID 1069479340, got %d", t1.ID)
	}
	if t1.Name != "Announcement" {
		t.Errorf("expected name 'Announcement', got %q", t1.Name)
	}
	if t1.Icon != "📢" {
		t.Errorf("expected icon '📢', got %q", t1.Icon)
	}

	// Verify timestamps are parsed
	if t1.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if t1.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify second type
	t2 := types[1]
	if t2.ID != 1069479341 {
		t.Errorf("expected ID 1069479341, got %d", t2.ID)
	}
	if t2.Name != "FYI" {
		t.Errorf("expected name 'FYI', got %q", t2.Name)
	}

	// Verify third type
	t3 := types[2]
	if t3.ID != 1069479342 {
		t.Errorf("expected ID 1069479342, got %d", t3.ID)
	}
	if t3.Name != "Heartbeat" {
		t.Errorf("expected name 'Heartbeat', got %q", t3.Name)
	}

	// Verify fourth type
	t4 := types[3]
	if t4.ID != 1069479343 {
		t.Errorf("expected ID 1069479343, got %d", t4.ID)
	}
	if t4.Name != "Question" {
		t.Errorf("expected name 'Question', got %q", t4.Name)
	}
}

func TestMessageType_UnmarshalGet(t *testing.T) {
	data := loadMessageTypesFixture(t, "get.json")

	var msgType MessageType
	if err := json.Unmarshal(data, &msgType); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if msgType.ID != 1069479340 {
		t.Errorf("expected ID 1069479340, got %d", msgType.ID)
	}
	if msgType.Name != "Announcement" {
		t.Errorf("expected name 'Announcement', got %q", msgType.Name)
	}
	if msgType.Icon != "📢" {
		t.Errorf("expected icon '📢', got %q", msgType.Icon)
	}

	// Verify timestamps are parsed
	if msgType.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if msgType.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
}

func TestCreateMessageTypeRequest_Marshal(t *testing.T) {
	req := CreateMessageTypeRequest{
		Name: "Update",
		Icon: "🔄",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateMessageTypeRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "Update" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	if data["icon"] != "🔄" {
		t.Errorf("unexpected icon: %v", data["icon"])
	}

	// Round-trip test
	var roundtrip CreateMessageTypeRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Errorf("expected name %q, got %q", req.Name, roundtrip.Name)
	}
	if roundtrip.Icon != req.Icon {
		t.Errorf("expected icon %q, got %q", req.Icon, roundtrip.Icon)
	}
}

func TestUpdateMessageTypeRequest_Marshal(t *testing.T) {
	req := UpdateMessageTypeRequest{
		Name: "Important Update",
		Icon: "⚠️",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMessageTypeRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "Important Update" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	if data["icon"] != "⚠️" {
		t.Errorf("unexpected icon: %v", data["icon"])
	}

	// Round-trip test
	var roundtrip UpdateMessageTypeRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Errorf("expected name %q, got %q", req.Name, roundtrip.Name)
	}
	if roundtrip.Icon != req.Icon {
		t.Errorf("expected icon %q, got %q", req.Icon, roundtrip.Icon)
	}
}

func TestUpdateMessageTypeRequest_MarshalPartial(t *testing.T) {
	// Test with only name
	req := UpdateMessageTypeRequest{
		Name: "New Name Only",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMessageTypeRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "New Name Only" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	// Icon should be omitted
	if _, ok := data["icon"]; ok {
		t.Error("expected icon to be omitted")
	}
}
