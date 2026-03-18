package basecamp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func hillChartsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "hill_charts")
}

func loadHillChartsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(hillChartsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestHillChart_UnmarshalGet(t *testing.T) {
	data := loadHillChartsFixture(t, "get.json")

	var hc HillChart
	if err := json.Unmarshal(data, &hc); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if !hc.Enabled {
		t.Error("expected Enabled to be true")
	}
	if hc.Stale {
		t.Error("expected Stale to be false")
	}
	if hc.AppUpdateURL == "" {
		t.Error("expected non-empty AppUpdateURL")
	}
	if hc.AppVersionsURL == "" {
		t.Error("expected non-empty AppVersionsURL")
	}
	if len(hc.Dots) != 1 {
		t.Fatalf("expected 1 dot, got %d", len(hc.Dots))
	}

	dot := hc.Dots[0]
	if dot.ID != 1069479424 {
		t.Errorf("expected dot ID 1069479424, got %d", dot.ID)
	}
	if dot.Label != "Background and research" {
		t.Errorf("expected label 'Background and research', got %q", dot.Label)
	}
	if dot.Color != "blue" {
		t.Errorf("expected color 'blue', got %q", dot.Color)
	}
	if dot.Position != 0 {
		t.Errorf("expected position 0, got %d", dot.Position)
	}
	if dot.URL == "" {
		t.Error("expected non-empty URL")
	}
	if dot.AppURL == "" {
		t.Error("expected non-empty AppURL")
	}
}

func TestHillChart_TimestampParsing(t *testing.T) {
	data := loadHillChartsFixture(t, "get.json")

	var hc HillChart
	if err := json.Unmarshal(data, &hc); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if hc.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if hc.UpdatedAt.Year() != 2026 {
		t.Errorf("expected year 2026, got %d", hc.UpdatedAt.Year())
	}
}

func TestHillChart_UnmarshalUpdateSettings(t *testing.T) {
	data := loadHillChartsFixture(t, "update-settings.json")

	var hc HillChart
	if err := json.Unmarshal(data, &hc); err != nil {
		t.Fatalf("failed to unmarshal update-settings.json: %v", err)
	}

	if !hc.Enabled {
		t.Error("expected Enabled to be true")
	}
	if len(hc.Dots) != 2 {
		t.Fatalf("expected 2 dots, got %d", len(hc.Dots))
	}
	if hc.Dots[1].Label != "Design mockups" {
		t.Errorf("expected second dot label 'Design mockups', got %q", hc.Dots[1].Label)
	}
	if hc.Dots[1].Position != 42 {
		t.Errorf("expected second dot position 42, got %d", hc.Dots[1].Position)
	}
}

func TestHillChart_UnmarshalUpdateSettingsRequest(t *testing.T) {
	data := loadHillChartsFixture(t, "update-settings-request.json")

	var req struct {
		Tracked   []int64 `json:"tracked"`
		Untracked []int64 `json:"untracked"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-settings-request.json: %v", err)
	}

	if len(req.Tracked) != 1 || req.Tracked[0] != 1069479573 {
		t.Errorf("expected tracked [1069479573], got %v", req.Tracked)
	}
	if len(req.Untracked) != 1 || req.Untracked[0] != 1069479511 {
		t.Errorf("expected untracked [1069479511], got %v", req.Untracked)
	}
}
