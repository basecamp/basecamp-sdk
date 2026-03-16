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

func TestLineupMarker_UnmarshalList(t *testing.T) {
	data := loadLineupFixture(t, "list.json")

	var markers []LineupMarker
	if err := json.Unmarshal(data, &markers); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(markers) != 2 {
		t.Fatalf("expected 2 markers, got %d", len(markers))
	}

	m := markers[0]
	if m.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", m.ID)
	}
	if m.Name != "Product Launch" {
		t.Errorf("expected name 'Product Launch', got %q", m.Name)
	}
	if m.Date != "2024-03-01" {
		t.Errorf("expected date '2024-03-01', got %q", m.Date)
	}
	if m.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if m.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
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
