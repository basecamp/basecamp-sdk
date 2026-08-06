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
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// fixturesDir returns the path to the fixtures directory.
func fixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "projects")
}

// loadFixture reads a fixture file and returns its contents.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(fixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestProject_UnmarshalList(t *testing.T) {
	data := loadFixture(t, "list.json")

	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}

	// Verify first project (basic, no client fields)
	p1 := projects[0]
	if p1.ID != 2085958499 {
		t.Errorf("expected ID 2085958499, got %d", p1.ID)
	}
	if p1.Name != "The Leto Laptop" {
		t.Errorf("expected name 'The Leto Laptop', got %q", p1.Name)
	}
	if p1.Status != "active" {
		t.Errorf("expected status 'active', got %q", p1.Status)
	}
	if p1.Purpose != "topic" {
		t.Errorf("expected purpose 'topic', got %q", p1.Purpose)
	}
	if p1.StartDate != "2022-01-01" {
		t.Errorf("expected start_date '2022-01-01', got %q", p1.StartDate)
	}
	if p1.EndDate != "2022-04-01" {
		t.Errorf("expected end_date '2022-04-01', got %q", p1.EndDate)
	}
	if p1.ClientCompany != nil {
		t.Errorf("expected nil ClientCompany for first project")
	}
	if len(p1.Dock) != 8 {
		t.Errorf("expected 8 dock items, got %d", len(p1.Dock))
	}

	// Verify second project (has client_company and clientside)
	p2 := projects[1]
	if p2.ID != 2085958500 {
		t.Errorf("expected ID 2085958500, got %d", p2.ID)
	}
	if p2.ClientCompany == nil {
		t.Fatal("expected ClientCompany for second project")
	}
	if p2.ClientCompany.ID != 1033447818 {
		t.Errorf("expected ClientCompany.ID 1033447818, got %d", p2.ClientCompany.ID)
	}
	if p2.ClientCompany.Name != "Leto Brand" {
		t.Errorf("expected ClientCompany.Name 'Leto Brand', got %q", p2.ClientCompany.Name)
	}
	if p2.Clientside == nil {
		t.Fatal("expected Clientside for second project")
	}
	if p2.Clientside.URL == "" {
		t.Error("expected non-empty Clientside.URL")
	}
}

func TestProject_UnmarshalGet(t *testing.T) {
	data := loadFixture(t, "get.json")

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if project.ID != 2085958499 {
		t.Errorf("expected ID 2085958499, got %d", project.ID)
	}
	if project.Name != "The Leto Laptop" {
		t.Errorf("expected name 'The Leto Laptop', got %q", project.Name)
	}
	if project.Description != "Laptop product launch." {
		t.Errorf("expected description 'Laptop product launch.', got %q", project.Description)
	}
	if project.StartDate != "2022-01-01" {
		t.Errorf("expected start_date '2022-01-01', got %q", project.StartDate)
	}
	if project.EndDate != "2022-04-01" {
		t.Errorf("expected end_date '2022-04-01', got %q", project.EndDate)
	}
	if project.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if project.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestDockItem_Unmarshal(t *testing.T) {
	data := loadFixture(t, "get.json")

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if len(project.Dock) != 8 {
		t.Fatalf("expected 8 dock items, got %d", len(project.Dock))
	}

	// Check enabled dock item (Message Board)
	mb := project.Dock[0]
	if mb.Name != "message_board" {
		t.Errorf("expected name 'message_board', got %q", mb.Name)
	}
	if mb.Title != "Message Board" {
		t.Errorf("expected title 'Message Board', got %q", mb.Title)
	}
	if !mb.Enabled {
		t.Error("expected Message Board to be enabled")
	}
	if mb.Position == nil || *mb.Position != 1 {
		t.Errorf("expected position 1, got %v", mb.Position)
	}

	// Check disabled dock item (Questionnaire)
	q := project.Dock[5]
	if q.Name != "questionnaire" {
		t.Errorf("expected name 'questionnaire', got %q", q.Name)
	}
	if q.Enabled {
		t.Error("expected Questionnaire to be disabled")
	}
	// Position is null in JSON, should be nil in Go
	if q.Position != nil {
		t.Errorf("expected nil position for disabled item, got %d", *q.Position)
	}
}

func TestCreateProjectRequest_Marshal(t *testing.T) {
	data := loadFixture(t, "create-request.json")

	// Unmarshal fixture to verify it matches our struct
	var req CreateProjectRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Name != "Marketing Campaign" {
		t.Errorf("expected name 'Marketing Campaign', got %q", req.Name)
	}
	if req.Description != "For Client: Xyz Corp Conference" {
		t.Errorf("expected description 'For Client: Xyz Corp Conference', got %q", req.Description)
	}

	// Re-marshal and verify round-trip
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateProjectRequest: %v", err)
	}

	var roundtrip CreateProjectRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name || roundtrip.Description != req.Description {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateProjectRequest_Marshal(t *testing.T) {
	data := loadFixture(t, "update-request.json")

	var req UpdateProjectRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Name != "Marketing Campaign" {
		t.Errorf("expected name 'Marketing Campaign', got %q", req.Name)
	}
	if req.Description != "For Client: Xyz Corp Conference" {
		t.Errorf("expected description 'For Client: Xyz Corp Conference', got %q", req.Description)
	}
	if req.Admissions != "team" {
		t.Errorf("expected admissions 'team', got %q", req.Admissions)
	}
	if req.ScheduleAttributes == nil {
		t.Fatal("expected ScheduleAttributes")
	}
	if req.ScheduleAttributes.StartDate != "2022-01-01" {
		t.Errorf("expected start_date '2022-01-01', got %q", req.ScheduleAttributes.StartDate)
	}
	if req.ScheduleAttributes.EndDate != "2022-04-01" {
		t.Errorf("expected end_date '2022-04-01', got %q", req.ScheduleAttributes.EndDate)
	}
}

func TestErrorResponse_Unmarshal(t *testing.T) {
	data := loadFixture(t, "error-limit.json")

	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &errResp); err != nil {
		t.Fatalf("failed to unmarshal error-limit.json: %v", err)
	}

	expected := "The project limit for this account has been reached."
	if errResp.Error != expected {
		t.Errorf("expected error %q, got %q", expected, errResp.Error)
	}
}

func TestProject_TimestampParsing(t *testing.T) {
	data := loadFixture(t, "get.json")

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	// Verify ISO8601 timestamps parse correctly
	// created_at: "2022-10-28T08:23:58.169Z"
	if project.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", project.CreatedAt.Year())
	}
	if project.CreatedAt.Month() != 10 {
		t.Errorf("expected month 10, got %d", project.CreatedAt.Month())
	}
	if project.CreatedAt.Day() != 28 {
		t.Errorf("expected day 28, got %d", project.CreatedAt.Day())
	}
}

func testProjectsServer(t *testing.T, handler http.HandlerFunc) *ProjectsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Projects()
}

func TestProjectsService_UpdatePartial(t *testing.T) {
	fixture := loadFixture(t, "get.json")
	var receivedBody map[string]any
	svc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	_, err := svc.Update(context.Background(), 12345, &UpdateProjectRequest{
		Name: "My Project",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody["name"] != "My Project" {
		t.Errorf("expected name 'My Project', got %v", receivedBody["name"])
	}

	for _, field := range []string{"description", "admissions", "schedule_attributes"} {
		if _, ok := receivedBody[field]; ok {
			t.Errorf("expected %q to be omitted from partial update, but it was present: %v", field, receivedBody[field])
		}
	}
}

func TestProjectsService_UpdateEmptyScheduleAttributes(t *testing.T) {
	fixture := loadFixture(t, "get.json")
	var receivedBody map[string]any
	svc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
		receivedBody = decodeRequestBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	// Non-nil but empty ScheduleAttributes must not leak as {}
	_, err := svc.Update(context.Background(), 12345, &UpdateProjectRequest{
		Name:               "My Project",
		ScheduleAttributes: &ScheduleAttributes{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := receivedBody["schedule_attributes"]; ok {
		t.Errorf("expected schedule_attributes to be omitted for empty struct, but it was present: %v", receivedBody["schedule_attributes"])
	}
}

// TestProjectsService_UpdateRetriesOn503WithFullBody is a PUBLIC-service retry
// proof: the typed ProjectsService.Update serializes its body via marshalBody
// (a *bytes.Reader, which net/http snapshots into GetBody), so the generated
// client's doWithRetry replays the FULL body on a transient 503. Guards against
// SDK-owned serialized bodies losing retries (the naturally-idempotent PUT
// conformance case).
func TestProjectsService_UpdateRetriesOn503WithFullBody(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if len(bodies) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // retryable
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":12345,"name":"x"}`))
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithMaxRetries(3), WithBaseDelay(time.Millisecond))
	svc := client.ForAccount("99999").Projects()

	if _, err := svc.Update(context.Background(), 12345, &UpdateProjectRequest{Name: "x"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(bodies) != 2 {
		t.Fatalf("public UpdateProject made %d requests, want 2 (idempotent PUT must retry on 503)", len(bodies))
	}
	const want = `{"name":"x"}`
	for i, b := range bodies {
		if b != want {
			t.Errorf("request %d body = %q, want the full serialized body %q (the retry must replay it)", i+1, b, want)
		}
	}
}

// Trash, Archive and Unarchive are the three project status transitions. All
// three answer 204 with an empty body, so the wrapper's contract is "no error,
// nothing to decode" — plus the hook and gating plumbing every wrapper carries.
func TestProjectsService_StatusTransitions(t *testing.T) {
	for _, tc := range []struct {
		name       string
		call       func(*ProjectsService) error
		wantMethod string
		wantPath   string
	}{
		{
			name:       "Trash",
			call:       func(s *ProjectsService) error { return s.Trash(context.Background(), 42) },
			wantMethod: "DELETE",
			wantPath:   "/99999/projects/42",
		},
		{
			name:       "Archive",
			call:       func(s *ProjectsService) error { return s.Archive(context.Background(), 42) },
			wantMethod: "PUT",
			wantPath:   "/99999/projects/42/status/archived.json",
		},
		{
			name:       "Unarchive",
			call:       func(s *ProjectsService) error { return s.Unarchive(context.Background(), 42) },
			wantMethod: "PUT",
			wantPath:   "/99999/projects/42/status/active.json",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			svc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			})

			if err := tc.call(svc); err != nil {
				t.Fatalf("%s returned error: %v", tc.name, err)
			}
			if gotMethod != tc.wantMethod {
				t.Errorf("method = %q, want %q", gotMethod, tc.wantMethod)
			}
			if gotPath != tc.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tc.wantPath)
			}
		})
	}
}

// The admin pro pack can limit archiving to admins and the project's creator,
// which bc3 answers with `head :forbidden`.
func TestProjectsService_ArchiveForbidden(t *testing.T) {
	svc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Access denied"}`))
	})

	err := svc.Archive(context.Background(), 42)
	if err == nil {
		t.Fatal("Archive should have returned an error on 403")
	}

	var bcErr *Error
	if !errors.As(err, &bcErr) {
		t.Fatalf("error is not *basecamp.Error: %T", err)
	}
	if bcErr.Code != CodeForbidden {
		t.Errorf("code = %q, want %q", bcErr.Code, CodeForbidden)
	}
	if bcErr.HTTPStatus != 403 {
		t.Errorf("http status = %d, want 403", bcErr.HTTPStatus)
	}
}

// The only behavioural evidence for ProjectLimitError. No SDK gives 507 a named
// class, so it surfaces as a generic api_error carrying the status (SPEC.md §7).
func TestProjectsService_UnarchiveAtProjectLimit(t *testing.T) {
	svc := testProjectsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInsufficientStorage)
		_, _ = w.Write([]byte(`{"error":"The project limit for this account has been reached."}`))
	})

	err := svc.Unarchive(context.Background(), 42)
	if err == nil {
		t.Fatal("Unarchive should have returned an error on 507")
	}

	var bcErr *Error
	if !errors.As(err, &bcErr) {
		t.Fatalf("error is not *basecamp.Error: %T", err)
	}
	if bcErr.Code != CodeAPI {
		t.Errorf("code = %q, want %q", bcErr.Code, CodeAPI)
	}
	if bcErr.HTTPStatus != 507 {
		t.Errorf("http status = %d, want 507", bcErr.HTTPStatus)
	}
}

// The low-level grouped client surface (generated.Client.Projects()) is emitted
// from an explicit per-operation switch in go/templates/client.tmpl, NOT from the
// operation list — so adding an operation to the spec does not add it here, and
// nothing catches the omission: go-check-drift compares generated operations
// against this package's wrappers and never looks at the grouped surface.
// ArchiveProject and UnarchiveProject shipped without their template cases for
// exactly that reason (caught in review on #679), leaving ProjectsService
// asymmetric with RecordingsService, which has both.
//
// These method-value references are the cheap guard: they are compile-time only,
// so a regressed template breaks the build here instead of silently shipping a
// grouped client that cannot reach the operation.
var (
	_ = (*generated.ProjectsService).Archive
	_ = (*generated.ProjectsService).Unarchive
	_ = (*generated.ProjectsService).Trash
)
