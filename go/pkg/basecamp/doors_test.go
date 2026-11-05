package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// The fixtures are bc3's documented external-link examples, the same ones the
// fixture-completeness guard validates against the Door schema
// (spec/fixtures/manifest.yaml).
func loadDoorFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "spec", "fixtures", "doors", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", path, err)
	}
	return data
}

type doorRequest struct {
	method string
	path   string
	body   map[string]any
}

// testDoorsServer answers every request with status and body, and records the
// last request it saw.
func testDoorsServer(t *testing.T, status int, body []byte) (*DoorsService, *doorRequest) {
	t.Helper()
	got := &doorRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.body = nil
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			if err := json.Unmarshal(raw, &got.body); err != nil {
				t.Errorf("request body is not JSON: %v", err)
			}
		}
		if body != nil {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if body != nil {
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").Doors(), got
}

func TestDoorsService_List(t *testing.T) {
	svc, got := testDoorsServer(t, http.StatusOK, loadDoorFixture(t, "list.json"))

	result, err := svc.List(context.Background(), 2085958504)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/99999/buckets/2085958504/dock/doors.json" {
		t.Errorf("request = %s %s, want GET /99999/buckets/2085958504/dock/doors.json", got.method, got.path)
	}
	if len(result.Doors) != 1 {
		t.Fatalf("len(Doors) = %d, want 1", len(result.Doors))
	}
	d := result.Doors[0]
	if d.ID != 1069480290 || d.Type != "Door" || d.Title != "Design system" {
		t.Errorf("door = %+v", d)
	}
	if d.Position != 8 {
		t.Errorf("Position = %d, want 8", d.Position)
	}
}

// TestDoor_DecodesTheFullShape guards the door partial's quirk: `url` is the
// outside address (rendered after the recording partial, overwriting its API
// URL), and the service struct and description ride along.
func TestDoor_DecodesTheFullShape(t *testing.T) {
	svc, got := testDoorsServer(t, http.StatusOK, loadDoorFixture(t, "get.json"))

	d, err := svc.Get(context.Background(), 1069480290)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/99999/dock/doors/1069480290" {
		t.Errorf("request = %s %s, want GET /99999/dock/doors/1069480290", got.method, got.path)
	}
	if want := "https://www.figma.com/file/abc123/Design-system"; d.URL != want {
		t.Errorf("URL = %q, want the outside address %q", d.URL, want)
	}
	if d.Service.Code != "figma" || d.Service.Name != "Figma" || len(d.Service.ValidPatterns) != 1 {
		t.Errorf("Service = %+v", d.Service)
	}
	if want := "<div>Shared Figma workspace</div>"; d.Description != want {
		t.Errorf("Description = %q, want %q", d.Description, want)
	}
	if d.Bucket == nil || d.Bucket.ID != 2085958504 {
		t.Errorf("Bucket = %+v", d.Bucket)
	}
	if d.Creator == nil || d.Creator.ID != 1049715913 {
		t.Errorf("Creator = %+v", d.Creator)
	}
	if d.VisibleToClients {
		t.Errorf("VisibleToClients = true, want false")
	}
}

func TestDoorsService_GetNotFound(t *testing.T) {
	svc, _ := testDoorsServer(t, http.StatusNotFound, []byte(`{"error":"Not found"}`))

	_, err := svc.Get(context.Background(), 1)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeNotFound {
		t.Fatalf("err = %v, want a not_found *Error", err)
	}
}

// TestDoorsService_CreateWrapsTheDoor pins the body bc3 reads: the link's
// fields under `door` (params.require(:door)) and visible_to_clients beside it.
func TestDoorsService_CreateWrapsTheDoor(t *testing.T) {
	svc, got := testDoorsServer(t, http.StatusCreated, loadDoorFixture(t, "get.json"))
	visible := true

	d, err := svc.Create(context.Background(), 2085958504, &CreateDoorRequest{
		Service:          "github",
		URL:              "https://github.com/honcho/leto-laptop",
		Title:            "Source code",
		VisibleToClients: &visible,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.ID != 1069480290 {
		t.Errorf("ID = %d", d.ID)
	}
	if got.method != http.MethodPost || got.path != "/99999/buckets/2085958504/dock/doors.json" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	door, ok := got.body["door"].(map[string]any)
	if !ok {
		t.Fatalf("body has no door wrapper: %v", got.body)
	}
	if door["service"] != "github" || door["url"] != "https://github.com/honcho/leto-laptop" || door["title"] != "Source code" {
		t.Errorf("door = %v", door)
	}
	if _, present := door["description"]; present {
		t.Errorf("description sent although unset: %v", door)
	}
	if _, present := door["visible_to_clients"]; present {
		t.Errorf("visible_to_clients sent inside the wrapper: %v", door)
	}
	if got.body["visible_to_clients"] != true {
		t.Errorf("visible_to_clients = %v, want true at the top level", got.body["visible_to_clients"])
	}
}

// TestDoorsService_CreateOmitsVisibility leaves the default to the server,
// which keeps a new link private to the team.
func TestDoorsService_CreateOmitsVisibility(t *testing.T) {
	svc, got := testDoorsServer(t, http.StatusCreated, loadDoorFixture(t, "get.json"))

	if _, err := svc.Create(context.Background(), 2085958504, &CreateDoorRequest{
		Service: "other", URL: "https://example.com",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, present := got.body["visible_to_clients"]; present {
		t.Errorf("visible_to_clients sent although nil: %v", got.body)
	}
}

func TestDoorsService_CreateRequiresURLAndService(t *testing.T) {
	svc, got := testDoorsServer(t, http.StatusCreated, loadDoorFixture(t, "get.json"))

	for _, req := range []*CreateDoorRequest{nil, {Service: "figma"}, {URL: "https://example.com"}} {
		_, err := svc.Create(context.Background(), 1, req)
		var e *Error
		if !errors.As(err, &e) || e.Code != CodeUsage {
			t.Errorf("Create(%+v) err = %v, want a usage error", req, err)
		}
	}
	if got.method != "" {
		t.Errorf("a request was sent: %s %s", got.method, got.path)
	}
}

func TestDoorsService_CreateValidationError(t *testing.T) {
	svc, _ := testDoorsServer(t, http.StatusUnprocessableEntity, []byte(`{"errors":{"url":["is invalid"]}}`))

	_, err := svc.Create(context.Background(), 1, &CreateDoorRequest{Service: "other", URL: "javascript:alert(1)"})
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeValidation {
		t.Fatalf("err = %v, want a validation *Error", err)
	}
	if msgs := e.FieldErrors["url"]; len(msgs) != 1 || msgs[0] != "is invalid" {
		t.Errorf("FieldErrors = %v", e.FieldErrors)
	}
}

func TestDoorsService_UpdateSendsOnlyWhatChanges(t *testing.T) {
	svc, got := testDoorsServer(t, http.StatusOK, loadDoorFixture(t, "get.json"))
	cleared := ""

	if _, err := svc.Update(context.Background(), 1069480290, &UpdateDoorRequest{
		Title:       "Firmware",
		Description: &cleared,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.method != http.MethodPut || got.path != "/99999/dock/doors/1069480290" {
		t.Errorf("request = %s %s", got.method, got.path)
	}
	door, ok := got.body["door"].(map[string]any)
	if !ok {
		t.Fatalf("body has no door wrapper: %v", got.body)
	}
	if len(door) != 2 || door["title"] != "Firmware" || door["description"] != "" {
		t.Errorf("door = %v, want only title and the cleared description", door)
	}
	if _, present := got.body["visible_to_clients"]; present {
		t.Errorf("visible_to_clients sent although nil: %v", got.body)
	}
}

// TestDoorsService_UpdateVisibilityAlone sends no door wrapper: bc3 keeps every
// door field when `door` is absent.
func TestDoorsService_UpdateVisibilityAlone(t *testing.T) {
	svc, got := testDoorsServer(t, http.StatusOK, loadDoorFixture(t, "get.json"))
	visible := true

	if _, err := svc.Update(context.Background(), 1069480290, &UpdateDoorRequest{VisibleToClients: &visible}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(got.body) != 1 || got.body["visible_to_clients"] != true {
		t.Errorf("body = %v, want only visible_to_clients", got.body)
	}
}

func TestDoorsService_UpdateForbidden(t *testing.T) {
	svc, _ := testDoorsServer(t, http.StatusForbidden, []byte(`{"error":"Forbidden"}`))

	_, err := svc.Update(context.Background(), 1, &UpdateDoorRequest{Title: "x"})
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeForbidden {
		t.Fatalf("err = %v, want a forbidden *Error", err)
	}
}

func TestDoorsService_Delete(t *testing.T) {
	svc, got := testDoorsServer(t, http.StatusNoContent, nil)

	if err := svc.Delete(context.Background(), 1069480290); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/99999/dock/doors/1069480290" {
		t.Errorf("request = %s %s, want DELETE /99999/dock/doors/1069480290", got.method, got.path)
	}
}

func TestDoorsService_DeleteNotFound(t *testing.T) {
	svc, _ := testDoorsServer(t, http.StatusNotFound, []byte(`{"error":"Not found"}`))

	err := svc.Delete(context.Background(), 1)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeNotFound {
		t.Fatalf("err = %v, want a not_found *Error", err)
	}
}
