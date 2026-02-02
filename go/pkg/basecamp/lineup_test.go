package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func lineupFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "lineup")
}

func loadLineupFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(lineupFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestLineupMarker_UnmarshalCreate(t *testing.T) {
	data := loadLineupFixture(t, "create.json")

	var marker LineupMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		t.Fatalf("failed to unmarshal create.json: %v", err)
	}

	if marker.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", marker.ID)
	}
	if marker.Status != "active" {
		t.Errorf("expected status 'active', got %q", marker.Status)
	}
	if marker.Type != "Lineup::Marker" {
		t.Errorf("expected type 'Lineup::Marker', got %q", marker.Type)
	}
	if marker.Title != "Product Launch" {
		t.Errorf("expected title 'Product Launch', got %q", marker.Title)
	}
	if marker.Color != "blue" {
		t.Errorf("expected color 'blue', got %q", marker.Color)
	}
	if marker.StartsOn != "2024-03-01" {
		t.Errorf("expected starts_on '2024-03-01', got %q", marker.StartsOn)
	}
	if marker.EndsOn != "2024-03-15" {
		t.Errorf("expected ends_on '2024-03-15', got %q", marker.EndsOn)
	}
	if marker.Description != "<div>Launch phase for the new product</div>" {
		t.Errorf("unexpected description: %q", marker.Description)
	}
	if marker.URL != "https://3.basecampapi.com/195539477/lineup/markers/1069479400.json" {
		t.Errorf("unexpected URL: %q", marker.URL)
	}
	if marker.AppURL != "https://3.basecamp.com/195539477/lineup" {
		t.Errorf("unexpected AppURL: %q", marker.AppURL)
	}

	// Verify timestamps are parsed
	if marker.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if marker.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}

	// Verify creator
	if marker.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if marker.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", marker.Creator.ID)
	}
	if marker.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", marker.Creator.Name)
	}
	if marker.Creator.EmailAddress != "victor@honchodesign.com" {
		t.Errorf("expected Creator.EmailAddress 'victor@honchodesign.com', got %q", marker.Creator.EmailAddress)
	}
}

func TestLineupMarker_UnmarshalUpdate(t *testing.T) {
	data := loadLineupFixture(t, "update.json")

	var marker LineupMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		t.Fatalf("failed to unmarshal update.json: %v", err)
	}

	if marker.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", marker.ID)
	}
	if marker.Title != "Product Launch - Extended" {
		t.Errorf("expected title 'Product Launch - Extended', got %q", marker.Title)
	}
	if marker.Color != "green" {
		t.Errorf("expected color 'green', got %q", marker.Color)
	}
	if marker.EndsOn != "2024-03-31" {
		t.Errorf("expected ends_on '2024-03-31', got %q", marker.EndsOn)
	}
	if marker.Description != "<div>Extended launch phase for the new product</div>" {
		t.Errorf("unexpected description: %q", marker.Description)
	}
}

func TestCreateMarkerRequest_Marshal(t *testing.T) {
	req := CreateMarkerRequest{
		Name: "Sprint 1",
		Date: "2024-04-01",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateMarkerRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "Sprint 1" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	if data["date"] != "2024-04-01" {
		t.Errorf("unexpected date: %v", data["date"])
	}

	// Round-trip test
	var roundtrip CreateMarkerRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Errorf("expected name %q, got %q", req.Name, roundtrip.Name)
	}
	if roundtrip.Date != req.Date {
		t.Errorf("expected date %q, got %q", req.Date, roundtrip.Date)
	}
}

func TestUpdateMarkerRequest_Marshal(t *testing.T) {
	req := UpdateMarkerRequest{
		Name: "Updated Sprint",
		Date: "2024-04-21",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMarkerRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "Updated Sprint" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	if data["date"] != "2024-04-21" {
		t.Errorf("unexpected date: %v", data["date"])
	}

	// Round-trip test
	var roundtrip UpdateMarkerRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Errorf("expected name %q, got %q", req.Name, roundtrip.Name)
	}
	if roundtrip.Date != req.Date {
		t.Errorf("expected date %q, got %q", req.Date, roundtrip.Date)
	}
}

func TestUpdateMarkerRequest_MarshalPartial(t *testing.T) {
	// Test with only some fields
	req := UpdateMarkerRequest{
		Name: "Just updating name",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMarkerRequest: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["name"] != "Just updating name" {
		t.Errorf("unexpected name: %v", data["name"])
	}
	// Optional fields should be omitted
	if _, ok := data["date"]; ok {
		t.Error("expected date to be omitted")
	}
}
