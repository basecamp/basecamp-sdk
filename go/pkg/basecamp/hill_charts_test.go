package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	if hc.VersionsURL != "https://3.basecampapi.com/195539477/buckets/2085958505/todosets/1069479829/hills/versions.json" {
		t.Errorf("unexpected VersionsURL %q", hc.VersionsURL)
	}
	if len(hc.Dots) != 1 {
		t.Fatalf("expected 1 dot, got %d", len(hc.Dots))
	}

	dot := hc.Dots[0]
	if dot.ID != 1069479863 {
		t.Errorf("expected dot ID 1069479863, got %d", dot.ID)
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

	// UpdatedAt is optional (*time.Time): nil-check before dereferencing.
	// `hc.UpdatedAt.IsZero()` still compiles against the pointer and would
	// panic here if the fixture ever dropped the key — see
	// TestNilOptionalTimestampPanicsOnValueReceiverCall.
	if hc.UpdatedAt == nil {
		t.Fatal("expected UpdatedAt to be present in get.json")
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

func testHillChartsServer(t *testing.T, handler http.HandlerFunc) *HillChartsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").HillCharts()
}

func TestHillChartsService_Get_VersionsURL(t *testing.T) {
	svc := testHillChartsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadHillChartsFixture(t, "get.json"))
	})

	hc, err := svc.Get(context.Background(), 1069479829)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc.VersionsURL != "https://3.basecampapi.com/195539477/buckets/2085958505/todosets/1069479829/hills/versions.json" {
		t.Errorf("VersionsURL not carried through the wrapper: %q", hc.VersionsURL)
	}
}

func TestHillChartsService_ListVersions(t *testing.T) {
	svc := testHillChartsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/99999/todosets/1069479829/hills/versions.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "1")
		w.Write(loadHillChartsFixture(t, "versions.json"))
	})

	result, err := svc.ListVersions(context.Background(), 1069479829, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Meta.TotalCount != 1 {
		t.Errorf("expected TotalCount 1, got %d", result.Meta.TotalCount)
	}
	if len(result.Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(result.Versions))
	}
	v := result.Versions[0]
	if v.ID != 1069480194 || v.Type != "Hill::Version" || v.Title != "Hill Chart: Progress update" {
		t.Errorf("unexpected version: %+v", v)
	}
	if v.Description != `<div dir="auto">Research is wrapping up. We've settled on the approach and start building next week.</div>` {
		t.Errorf("unexpected description %q", v.Description)
	}
	if v.DescriptionAttachments == nil || len(v.DescriptionAttachments) != 0 {
		t.Errorf("expected an empty, non-nil DescriptionAttachments, got %#v", v.DescriptionAttachments)
	}
	if v.CommentsURL != "https://3.basecampapi.com/195539477/buckets/2085958505/recordings/1069480194/comments.json" {
		t.Errorf("unexpected CommentsURL %q", v.CommentsURL)
	}
	if v.Parent == nil || v.Parent.ID != 1069479829 || v.Parent.Type != "Todoset" {
		t.Errorf("unexpected parent %+v", v.Parent)
	}
	if v.Bucket == nil || v.Bucket.ID != 2085958505 {
		t.Errorf("unexpected bucket %+v", v.Bucket)
	}
	if v.Creator == nil || v.Creator.Name != "Victor Cooper" {
		t.Errorf("unexpected creator %+v", v.Creator)
	}
	if len(v.Dots) != 1 {
		t.Fatalf("expected 1 dot, got %d", len(v.Dots))
	}
	dot := v.Dots[0]
	if dot.ID == nil || *dot.ID != 1069479863 || dot.Label != "Background and research" || dot.Color != "blue" || dot.Position != 40 {
		t.Errorf("unexpected dot %+v", dot)
	}
	if dot.URL == "" || dot.AppURL == "" {
		t.Errorf("expected the dot's url and app_url, got %+v", dot)
	}
}

func TestHillChartsService_ListVersions_FollowsPages(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, string(loadHillChartsFixture(t, "versions.json")))
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/99999/todosets/1/hills/versions.json?page=2>; rel="next"`, serverURL))
		fmt.Fprint(w, string(loadHillChartsFixture(t, "versions.json")))
	}))
	t.Cleanup(server.Close)
	serverURL = server.URL

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	svc := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999").HillCharts()

	result, err := svc.ListVersions(context.Background(), 1, &HillChartVersionListOptions{Limit: -1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Versions) != 2 {
		t.Fatalf("expected both pages' versions, got %d", len(result.Versions))
	}
}

func TestHillChartsService_GetVersion(t *testing.T) {
	svc := testHillChartsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/99999/todosets/1069479829/hills/versions/1069480194" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadHillChartsFixture(t, "version.json"))
	})

	v, err := svc.GetVersion(context.Background(), 1069479829, 1069480194)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.ID != 1069480194 || len(v.Dots) != 1 || v.Dots[0].Position != 40 {
		t.Errorf("unexpected version: %+v", v)
	}
}

func TestHillChartsService_GetVersion_TodolistGone(t *testing.T) {
	svc := testHillChartsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadHillChartsFixture(t, "version_todolist_gone.json"))
	})

	v, err := svc.GetVersion(context.Background(), 1069479829, 1069480194)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v.Dots) != 2 {
		t.Fatalf("expected 2 dots, got %d", len(v.Dots))
	}
	gone := v.Dots[1]
	if gone.ID != nil || gone.URL != "" || gone.AppURL != "" {
		t.Errorf("a todolist that is gone has no id, url or app_url: %+v", gone)
	}
	if gone.Label != "Old research list" || gone.Position != 100 {
		t.Errorf("unexpected dot %+v", gone)
	}

	out, err := json.Marshal(gone)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if id, present := round["id"]; !present || id != nil {
		t.Errorf("expected the gone dot to marshal \"id\": null, got %s", out)
	}
}

func TestHillChartsService_GetVersion_NotFound(t *testing.T) {
	svc := testHillChartsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Record not found"}`))
	})

	_, err := svc.GetVersion(context.Background(), 1, 2)
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Errorf("expected %s, got: %v", CodeNotFound, err)
	}
}

func TestHillChartsService_ListVersions_NotFound(t *testing.T) {
	svc := testHillChartsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Record not found"}`))
	})

	_, err := svc.ListVersions(context.Background(), 1, nil)
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Errorf("expected %s, got: %v", CodeNotFound, err)
	}
}
