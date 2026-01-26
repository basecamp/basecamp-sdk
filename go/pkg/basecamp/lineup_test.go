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
		Title:       "Sprint 1",
		StartsOn:    "2024-04-01",
		EndsOn:      "2024-04-14",
		Color:       "purple",
		Description: "<div>First sprint</div>",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateMarkerRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Sprint 1" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["starts_on"] != "2024-04-01" {
		t.Errorf("unexpected starts_on: %v", data["starts_on"])
	}
	if data["ends_on"] != "2024-04-14" {
		t.Errorf("unexpected ends_on: %v", data["ends_on"])
	}
	if data["color"] != "purple" {
		t.Errorf("unexpected color: %v", data["color"])
	}
	if data["description"] != "<div>First sprint</div>" {
		t.Errorf("unexpected description: %v", data["description"])
	}

	// Round-trip test
	var roundtrip CreateMarkerRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Title != req.Title {
		t.Errorf("expected title %q, got %q", req.Title, roundtrip.Title)
	}
	if roundtrip.StartsOn != req.StartsOn {
		t.Errorf("expected starts_on %q, got %q", req.StartsOn, roundtrip.StartsOn)
	}
	if roundtrip.EndsOn != req.EndsOn {
		t.Errorf("expected ends_on %q, got %q", req.EndsOn, roundtrip.EndsOn)
	}
}

func TestCreateMarkerRequest_MarshalMinimal(t *testing.T) {
	// Test with only required fields
	req := CreateMarkerRequest{
		Title:    "Quick marker",
		StartsOn: "2024-05-01",
		EndsOn:   "2024-05-07",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateMarkerRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Quick marker" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["starts_on"] != "2024-05-01" {
		t.Errorf("unexpected starts_on: %v", data["starts_on"])
	}
	if data["ends_on"] != "2024-05-07" {
		t.Errorf("unexpected ends_on: %v", data["ends_on"])
	}
	// Optional fields with omitempty should not be present
	if _, ok := data["color"]; ok {
		t.Error("expected color to be omitted")
	}
	if _, ok := data["description"]; ok {
		t.Error("expected description to be omitted")
	}
}

func TestUpdateMarkerRequest_Marshal(t *testing.T) {
	req := UpdateMarkerRequest{
		Title:       "Updated Sprint",
		StartsOn:    "2024-04-01",
		EndsOn:      "2024-04-21",
		Color:       "orange",
		Description: "<div>Extended sprint</div>",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMarkerRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Updated Sprint" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	if data["starts_on"] != "2024-04-01" {
		t.Errorf("unexpected starts_on: %v", data["starts_on"])
	}
	if data["ends_on"] != "2024-04-21" {
		t.Errorf("unexpected ends_on: %v", data["ends_on"])
	}
	if data["color"] != "orange" {
		t.Errorf("unexpected color: %v", data["color"])
	}
	if data["description"] != "<div>Extended sprint</div>" {
		t.Errorf("unexpected description: %v", data["description"])
	}

	// Round-trip test
	var roundtrip UpdateMarkerRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Title != req.Title {
		t.Errorf("expected title %q, got %q", req.Title, roundtrip.Title)
	}
	if roundtrip.EndsOn != req.EndsOn {
		t.Errorf("expected ends_on %q, got %q", req.EndsOn, roundtrip.EndsOn)
	}
}

func TestUpdateMarkerRequest_MarshalPartial(t *testing.T) {
	// Test with only some fields
	req := UpdateMarkerRequest{
		Title: "Just updating title",
	}

	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal UpdateMarkerRequest: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["title"] != "Just updating title" {
		t.Errorf("unexpected title: %v", data["title"])
	}
	// Optional fields should be omitted
	if _, ok := data["starts_on"]; ok {
		t.Error("expected starts_on to be omitted")
	}
	if _, ok := data["ends_on"]; ok {
		t.Error("expected ends_on to be omitted")
	}
	if _, ok := data["color"]; ok {
		t.Error("expected color to be omitted")
	}
	if _, ok := data["description"]; ok {
		t.Error("expected description to be omitted")
	}
}
