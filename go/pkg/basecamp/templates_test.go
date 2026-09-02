package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// templatesFixturesDir returns the path to the templates fixtures directory.
func templatesFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "templates")
}

// loadTemplatesFixture reads a fixture file and returns its contents.
func loadTemplatesFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(templatesFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTemplate_UnmarshalList(t *testing.T) {
	data := loadTemplatesFixture(t, "list.json")

	var templates []Template
	if err := json.Unmarshal(data, &templates); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(templates) != 2 {
		t.Errorf("expected 2 templates, got %d", len(templates))
	}

	// Verify first template
	t1 := templates[0]
	if t1.ID != 2085958501 {
		t.Errorf("expected ID 2085958501, got %d", t1.ID)
	}
	if t1.Name != "Project Template" {
		t.Errorf("expected name 'Project Template', got %q", t1.Name)
	}
	if t1.Status != "active" {
		t.Errorf("expected status 'active', got %q", t1.Status)
	}
	if t1.Description != "Standard project template for new initiatives." {
		t.Errorf("expected description 'Standard project template for new initiatives.', got %q", t1.Description)
	}

	// Verify second template
	t2 := templates[1]
	if t2.ID != 2085958502 {
		t.Errorf("expected ID 2085958502, got %d", t2.ID)
	}
	if t2.Name != "Client Onboarding" {
		t.Errorf("expected name 'Client Onboarding', got %q", t2.Name)
	}
}

func TestTemplate_UnmarshalGet(t *testing.T) {
	data := loadTemplatesFixture(t, "get.json")

	var template Template
	if err := json.Unmarshal(data, &template); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if template.ID != 2085958501 {
		t.Errorf("expected ID 2085958501, got %d", template.ID)
	}
	if template.Name != "Project Template" {
		t.Errorf("expected name 'Project Template', got %q", template.Name)
	}
	if template.Description != "Standard project template for new initiatives." {
		t.Errorf("expected description 'Standard project template for new initiatives.', got %q", template.Description)
	}
	if template.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if template.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestCreateTemplateRequest_Marshal(t *testing.T) {
	data := loadTemplatesFixture(t, "create-request.json")

	var req CreateTemplateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Name != "New Template" {
		t.Errorf("expected name 'New Template', got %q", req.Name)
	}
	if req.Description != "A new project template." {
		t.Errorf("expected description 'A new project template.', got %q", req.Description)
	}

	// Re-marshal and verify round-trip
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTemplateRequest: %v", err)
	}

	var roundtrip CreateTemplateRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name || roundtrip.Description != req.Description {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateTemplateRequest_Marshal(t *testing.T) {
	data := loadTemplatesFixture(t, "update-request.json")

	var req UpdateTemplateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Name != "Updated Template" {
		t.Errorf("expected name 'Updated Template', got %q", req.Name)
	}
	if req.Description != "Updated template description." {
		t.Errorf("expected description 'Updated template description.', got %q", req.Description)
	}
}

func TestProjectConstruction_Unmarshal(t *testing.T) {
	data := loadTemplatesFixture(t, "project_construction.json")

	var construction ProjectConstruction
	if err := json.Unmarshal(data, &construction); err != nil {
		t.Fatalf("failed to unmarshal project_construction.json: %v", err)
	}

	if construction.ID != 1234567890 {
		t.Errorf("expected ID 1234567890, got %d", construction.ID)
	}
	if construction.Status != "pending" {
		t.Errorf("expected status 'pending', got %q", construction.Status)
	}
	if construction.URL == "" {
		t.Error("expected non-empty URL")
	}
	if construction.Project != nil {
		t.Error("expected nil Project for pending construction")
	}
}

func TestProjectConstruction_UnmarshalCompleted(t *testing.T) {
	data := loadTemplatesFixture(t, "project_construction_completed.json")

	var construction ProjectConstruction
	if err := json.Unmarshal(data, &construction); err != nil {
		t.Fatalf("failed to unmarshal project_construction_completed.json: %v", err)
	}

	if construction.ID != 1234567890 {
		t.Errorf("expected ID 1234567890, got %d", construction.ID)
	}
	if construction.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", construction.Status)
	}
	if construction.Project == nil {
		t.Fatal("expected non-nil Project for completed construction")
	}
	if construction.Project.ID != 2085958503 {
		t.Errorf("expected Project.ID 2085958503, got %d", construction.Project.ID)
	}
	if construction.Project.Name != "New Project from Template" {
		t.Errorf("expected Project.Name 'New Project from Template', got %q", construction.Project.Name)
	}
}

// testTemplatesServer wires a TemplatesService to an httptest server.
func testTemplatesServer(t *testing.T, handler http.HandlerFunc) *TemplatesService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Templates()
}

// TestTemplatesService_CreateProjectEnvelope verifies the request body nests the
// project parameters under a "project" envelope, as the project_constructions
// endpoint requires. A flat {"name","description"} body is rejected with 400.
func TestTemplatesService_CreateProjectEnvelope(t *testing.T) {
	fixture := loadTemplatesFixture(t, "project_construction.json")

	var receivedBody map[string]any
	var receivedMethod, receivedPath string
	svc := testTemplatesServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedBody = decodeRequestBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(fixture)
	})

	_, err := svc.CreateProject(context.Background(), 987, "New Project from Template", "Kick-off details")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if want := "/99999/templates/987/project_constructions.json"; receivedPath != want {
		t.Errorf("expected path %q, got %q", want, receivedPath)
	}

	project, ok := receivedBody["project"].(map[string]any)
	if !ok {
		t.Fatalf("expected params nested under \"project\" envelope, got body: %v", receivedBody)
	}
	if _, flat := receivedBody["name"]; flat {
		t.Error("expected no top-level \"name\"; params must be nested under \"project\"")
	}
	if project["name"] != "New Project from Template" {
		t.Errorf("expected project.name 'New Project from Template', got %v", project["name"])
	}
	if project["description"] != "Kick-off details" {
		t.Errorf("expected project.description 'Kick-off details', got %v", project["description"])
	}
}

func TestTemplatesService_GetLibrary(t *testing.T) {
	var receivedMethod, receivedPath string
	svc := testTemplatesServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"bucket":{"id":1,"name":"To-do List Templates","type":"TemplateLibrary"},
			"todoset":{"id":2,"title":"To-do List Templates","type":"Todoset","url":"https://example.test/todoset.json","app_url":"https://example.test/todoset"},
			"todolists":[{"id":3,"status":"active","visible_to_clients":false,"created_at":"2026-08-27T12:00:00Z","updated_at":"2026-08-27T12:00:00Z","title":"Project kickoff","inherits_status":true,"type":"Todolist","url":"https://example.test/list.json","app_url":"https://example.test/list","bubble_up_url":"https://example.test/bubble.json","parent":{"id":2,"title":"To-do List Templates","type":"Todoset","url":"https://example.test/todoset.json","app_url":"https://example.test/todoset"},"bucket":{"id":1,"name":"To-do List Templates","type":"TemplateLibrary"},"creator":{"id":4,"name":"Victor"},"description":"","description_attachments":[],"name":"Project kickoff","color":null,"comments_app_url":"https://example.test/comments"}]
		}`))
	})

	library, err := svc.GetLibrary(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != http.MethodGet || receivedPath != "/99999/template_library.json" {
		t.Fatalf("request = %s %s", receivedMethod, receivedPath)
	}
	if library.Bucket.Type != "TemplateLibrary" || library.Todoset.ID != 2 {
		t.Fatalf("unexpected library parents: %+v", library)
	}
	if len(library.Todolists) != 1 || library.Todolists[0].Name != "Project kickoff" {
		t.Fatalf("unexpected library todolists: %+v", library.Todolists)
	}
}

func TestTemplatesService_GetLibraryForbidden(t *testing.T) {
	svc := testTemplatesServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"Forbidden"}`))
	})

	_, err := svc.GetLibrary(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeForbidden || apiErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("unexpected forbidden error: %v", err)
	}
}

func TestTemplatesService_CreateLibraryCopy(t *testing.T) {
	var receivedBody map[string]any
	svc := testTemplatesServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":5,"status":"pending","source_recording_id":3,"destination_parent_id":9,"url":"https://example.test/copies/5.json"}`))
	})

	templateCopy, err := svc.CreateLibraryCopy(context.Background(), &CreateTemplateLibraryCopyRequest{
		TemplateRecordingID:   3,
		DestinationParentID:   9,
		AddingPeopleConfirmed: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["template_recording_id"] != json.Number("3") || receivedBody["destination_parent_id"] != json.Number("9") {
		t.Fatalf("unexpected request body: %v", receivedBody)
	}
	if receivedBody["adding_people_confirmed"] != true {
		t.Fatalf("confirmation missing from request body: %v", receivedBody)
	}
	if templateCopy.ID != 5 || templateCopy.Status != "pending" || templateCopy.DestinationTodolist != nil {
		t.Fatalf("unexpected copy: %+v", templateCopy)
	}
}

func TestTemplatesService_CreateLibraryCopyRequiresPeopleConfirmation(t *testing.T) {
	svc := testTemplatesServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error":"Adding people requires confirmation","people":[{"id":4,"name":"Victor","avatar_url":"https://example.test/avatar.png"}]}`))
	})

	_, err := svc.CreateLibraryCopy(context.Background(), &CreateTemplateLibraryCopyRequest{
		TemplateRecordingID: 3,
		DestinationParentID: 9,
	})
	if err == nil {
		t.Fatal("expected people confirmation validation error")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.Code != CodeValidation || apiErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("unexpected validation error: %+v", apiErr)
	}
	if apiErr.Message != "Adding people requires confirmation" {
		t.Fatalf("unexpected error message: %q", apiErr.Message)
	}
}

func TestTemplatesService_GetCompletedLibraryCopy(t *testing.T) {
	svc := testTemplatesServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/template_library/copies/5" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id":5,"status":"completed","source_recording_id":3,"destination_parent_id":9,"url":"https://example.test/copies/5.json",
			"destination_todolist":{"id":10,"status":"active","visible_to_clients":false,"created_at":"2026-08-27T12:00:00Z","updated_at":"2026-08-27T12:00:00Z","title":"Project kickoff","inherits_status":true,"type":"Todolist","url":"https://example.test/list.json","app_url":"https://example.test/list","bubble_up_url":"https://example.test/bubble.json","parent":{"id":9,"title":"To-dos","type":"Todoset","url":"https://example.test/todoset.json","app_url":"https://example.test/todoset"},"bucket":{"id":8,"name":"Project","type":"Project"},"creator":{"id":4,"name":"Victor"},"description":"","description_attachments":[],"name":"Project kickoff","color":null,"comments_app_url":"https://example.test/comments"}
		}`))
	})

	templateCopy, err := svc.GetLibraryCopy(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if templateCopy.Status != "completed" || templateCopy.DestinationTodolist == nil || templateCopy.DestinationTodolist.ID != 10 {
		t.Fatalf("unexpected completed copy: %+v", templateCopy)
	}
}

func TestTemplatesService_GetLibraryCopyNotFound(t *testing.T) {
	svc := testTemplatesServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Not found"}`))
	})

	_, err := svc.GetLibraryCopy(context.Background(), 404)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeNotFound || apiErr.HTTPStatus != http.StatusNotFound {
		t.Fatalf("unexpected not-found error: %v", err)
	}
}

func TestTemplate_TimestampParsing(t *testing.T) {
	data := loadTemplatesFixture(t, "get.json")

	var template Template
	if err := json.Unmarshal(data, &template); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	// Verify ISO8601 timestamps parse correctly
	// created_at: "2022-10-28T08:23:58.169Z"
	if template.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", template.CreatedAt.Year())
	}
	if template.CreatedAt.Month() != 10 {
		t.Errorf("expected month 10, got %d", template.CreatedAt.Month())
	}
	if template.CreatedAt.Day() != 28 {
		t.Errorf("expected day 28, got %d", template.CreatedAt.Day())
	}
}
