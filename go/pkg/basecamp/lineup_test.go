package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// TestLineupService_MarkerWritesReturnTheMarker: bc3's Lineup::MarkersController
// renders the saved marker on create (201) and update (200), so both return it.
func TestLineupService_MarkerWritesReturnTheMarker(t *testing.T) {
	const marker = `{"id":39446,"name":"Launch","date":"2026-11-20","created_at":"2026-10-15T08:00:00.000Z","updated_at":"2026-10-15T09:00:00.000Z"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/99999/lineup/markers.json":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && r.URL.Path == "/99999/lineup/markers/39446":
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(marker))
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	svc := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999").Lineup()

	created, err := svc.CreateMarker(context.Background(), &CreateMarkerRequest{Name: "Launch", Date: "2026-11-20"})
	if err != nil {
		t.Fatalf("CreateMarker failed: %v", err)
	}
	if created == nil || created.ID != 39446 || created.Name != "Launch" || created.Date != "2026-11-20" || created.CreatedAt.IsZero() {
		t.Errorf("unexpected created marker: %+v", created)
	}

	updated, err := svc.UpdateMarker(context.Background(), 39446, &UpdateMarkerRequest{Name: "Launch"})
	if err != nil {
		t.Fatalf("UpdateMarker failed: %v", err)
	}
	if updated == nil || updated.ID != 39446 || updated.UpdatedAt.IsZero() {
		t.Errorf("unexpected updated marker: %+v", updated)
	}
}
