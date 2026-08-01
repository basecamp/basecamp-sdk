package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func todolistGroupsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "todolist_groups")
}

func loadTodolistGroupsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(todolistGroupsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTodolistGroup_UnmarshalList(t *testing.T) {
	data := loadTodolistGroupsFixture(t, "list.json")

	var groups []TodolistGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	// Verify first group
	g1 := groups[0]
	if g1.ID != 1069479600 {
		t.Errorf("expected ID 1069479600, got %d", g1.ID)
	}
	if g1.Name != "Phase 1" {
		t.Errorf("expected name 'Phase 1', got %q", g1.Name)
	}
	if g1.Title != "Phase 1" {
		t.Errorf("expected title 'Phase 1', got %q", g1.Title)
	}
	if g1.Type != "Todolist" {
		t.Errorf("expected type 'Todolist', got %q", g1.Type)
	}
	if g1.Status != "active" {
		t.Errorf("expected status 'active', got %q", g1.Status)
	}
	if g1.CompletedRatio != "1/3" {
		t.Errorf("expected completed_ratio '1/3', got %q", g1.CompletedRatio)
	}
	if g1.Position != 1 {
		t.Errorf("expected position 1, got %d", g1.Position)
	}

	// Verify second group
	g2 := groups[1]
	if g2.ID != 1069479601 {
		t.Errorf("expected ID 1069479601, got %d", g2.ID)
	}
	if g2.Name != "Phase 2" {
		t.Errorf("expected name 'Phase 2', got %q", g2.Name)
	}
	if g2.Position != 2 {
		t.Errorf("expected position 2, got %d", g2.Position)
	}
}

func TestTodolistGroup_UnmarshalGet(t *testing.T) {
	data := loadTodolistGroupsFixture(t, "get.json")

	var group TodolistGroup
	if err := json.Unmarshal(data, &group); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if group.ID != 1069479600 {
		t.Errorf("expected ID 1069479600, got %d", group.ID)
	}
	if group.Name != "Phase 1" {
		t.Errorf("expected name 'Phase 1', got %q", group.Name)
	}
	if group.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if group.Parent.ID != 1069479519 {
		t.Errorf("expected Parent.ID 1069479519, got %d", group.Parent.ID)
	}
	if group.Parent.Type != "Todolist" {
		t.Errorf("expected Parent.Type 'Todolist', got %q", group.Parent.Type)
	}
	if group.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if group.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", group.Bucket.ID)
	}
	if group.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if group.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", group.Creator.Name)
	}
	if group.TodosURL == "" {
		t.Error("expected non-empty TodosURL")
	}
}

func TestCreateTodolistGroupRequest_Marshal(t *testing.T) {
	data := loadTodolistGroupsFixture(t, "create-request.json")

	var req CreateTodolistGroupRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Name != "Phase 3" {
		t.Errorf("expected name 'Phase 3', got %q", req.Name)
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTodolistGroupRequest: %v", err)
	}

	var roundtrip CreateTodolistGroupRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name {
		t.Error("round-trip mismatch")
	}
}

// --- httptest-based service contract tests ---

// testTodolistGroupsServer creates an httptest.Server and a TodolistGroupsService wired to it.
func testTodolistGroupsServer(t *testing.T, handler http.HandlerFunc) *TodolistGroupsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.TodolistGroups()
}

func TestTodolistGroupsService_Get(t *testing.T) {
	fixture := loadTodolistGroupsFixture(t, "get.json")
	svc := testTodolistGroupsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/todolists/1069479600" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	group, err := svc.Get(context.Background(), 1069479600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.ID != 1069479600 {
		t.Errorf("expected ID 1069479600, got %d", group.ID)
	}
	if group.Name != "Phase 1" {
		t.Errorf("expected name 'Phase 1', got %q", group.Name)
	}
	if group.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if group.Parent.ID != 1069479519 {
		t.Errorf("expected Parent.ID 1069479519, got %d", group.Parent.ID)
	}
	if group.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if group.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", group.Bucket.ID)
	}
	if group.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if group.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", group.Creator.Name)
	}
}

// testTodolistGroupsCaptureServer records every request's method, path, and
// (for PUTs) decoded body, answering all of them with body.
func testTodolistGroupsCaptureServer(t *testing.T, body []byte, hooks Hooks) (*TodolistGroupsService, *[]capturedTodolistRequest) {
	t.Helper()
	reqs := &[]capturedTodolistRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cr := capturedTodolistRequest{method: r.Method, path: r.URL.Path}
		if r.Method == "PUT" {
			cr.body = decodeRequestBody(t, r)
		}
		*reqs = append(*reqs, cr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(body)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	var opts []ClientOption
	if hooks != nil {
		opts = append(opts, WithHooks(hooks))
	}
	client := NewClient(cfg, token, opts...)
	return client.ForAccount("99999").TodolistGroups(), reqs
}

func TestTodolistGroupsService_Replace(t *testing.T) {
	fixture := loadTodolistGroupsFixture(t, "get.json")
	var receivedBody map[string]any
	svc := testTodolistGroupsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/99999/todolists/1069479600" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		receivedBody = decodeRequestBody(t, r)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	group, err := svc.Replace(context.Background(), 1069479600, &ReplaceTodolistGroupRequest{
		Name: "Updated Phase 1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if group.ID != 1069479600 {
		t.Errorf("expected ID 1069479600, got %d", group.ID)
	}
	if group.Name != "Phase 1" {
		t.Errorf("expected name 'Phase 1', got %q", group.Name)
	}
	if group.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if group.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if group.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if receivedBody["name"] != "Updated Phase 1" {
		t.Errorf("expected request body name 'Updated Phase 1', got %v", receivedBody["name"])
	}
}

// A group carries a description — it is a Todolist whose parent is a Todolist,
// rendered through the same todolists/_todolist.json.jbuilder partial — and
// BC3 rebuilds the recordable from the permitted params, so a caller must be
// able to send one or the replace erases it.
func TestTodolistGroupsService_ReplaceCarriesDescription(t *testing.T) {
	fixture := loadTodolistGroupsFixture(t, "get.json")
	svc, reqs := testTodolistGroupsCaptureServer(t, fixture, nil)

	_, err := svc.Replace(context.Background(), 1069479600, &ReplaceTodolistGroupRequest{
		Name:        "Updated Phase 1",
		Description: "<p>Ship the peripherals</p>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*reqs) != 1 || (*reqs)[0].method != "PUT" {
		t.Fatalf("expected exactly one PUT, got %+v", *reqs)
	}
	body := (*reqs)[0].body
	if body["name"] != "Updated Phase 1" {
		t.Errorf("expected name 'Updated Phase 1', got %v", body["name"])
	}
	desc, ok := body["description"]
	if !ok {
		t.Fatalf("description missing from the group PUT body — BC3 would clear it; body=%v", body)
	}
	if desc != "<p>Ship the peripherals</p>" {
		t.Errorf("expected description '<p>Ship the peripherals</p>', got %v", desc)
	}
}

// Replace is verbatim: an unset description stays omitted (the server clears
// it), and there is no read-before-write.
func TestTodolistGroupsService_ReplaceSendsSparseVerbatim(t *testing.T) {
	fixture := loadTodolistGroupsFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, reqs := testTodolistGroupsCaptureServer(t, fixture, recorder)

	_, err := svc.Replace(context.Background(), 1069479600, &ReplaceTodolistGroupRequest{
		Name: "Updated Phase 1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*reqs) != 1 || (*reqs)[0].method != "PUT" {
		t.Fatalf("expected exactly one PUT and no GET, got %+v", *reqs)
	}
	if _, ok := (*reqs)[0].body["description"]; ok {
		t.Errorf("expected description omitted from a sparse replace, got %v", (*reqs)[0].body["description"])
	}

	if len(recorder.opStartCalls) != 1 ||
		recorder.opStartCalls[0].Service != "TodolistGroups" || recorder.opStartCalls[0].Operation != "Replace" {
		t.Errorf("expected a single TodolistGroups.Replace operation, got %+v", recorder.opStartCalls)
	}
}

func TestTodolistGroupsService_ReplaceRequiresName(t *testing.T) {
	fixture := loadTodolistGroupsFixture(t, "get.json")
	svc, reqs := testTodolistGroupsCaptureServer(t, fixture, nil)

	_, err := svc.Replace(context.Background(), 1069479600, &ReplaceTodolistGroupRequest{
		Description: "<p>orphaned</p>",
	})
	if err == nil {
		t.Fatal("expected a usage error for a missing name")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage || apiErr.Message != "group name is required" {
		t.Errorf("expected usage error %q, got %v", "group name is required", err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestTodolistGroupsService_ReplaceNilRequestIsUsageError(t *testing.T) {
	fixture := loadTodolistGroupsFixture(t, "get.json")
	svc, reqs := testTodolistGroupsCaptureServer(t, fixture, nil)

	_, err := svc.Replace(context.Background(), 1069479600, nil)
	if err == nil {
		t.Fatal("expected a usage error for a nil request")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage || apiErr.Message != "replace request is required" {
		t.Errorf("expected usage error %q, got %v", "replace request is required", err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

// ReplaceTodolistGroupRequest.Name carries no omitempty: the wire contract
// makes name required, so an empty one must still serialize as a present key
// rather than reading as a preserve.
func TestReplaceTodolistGroupRequest_NameAlwaysMarshals(t *testing.T) {
	out, err := json.Marshal(ReplaceTodolistGroupRequest{})
	if err != nil {
		t.Fatalf("failed to marshal ReplaceTodolistGroupRequest: %v", err)
	}
	if string(out) != `{"name":""}` {
		t.Errorf("expected {\"name\":\"\"}, got %s", out)
	}
}

// TodolistGroupsService deliberately ships no merge-safe Update or Edit. This
// pins the reason so the gap cannot be closed by accident: the TodolistGroup
// projection models no description, so a composite built on
// TodolistGroupsService.Get would PUT a zero-value description and erase it on
// every call. Tracked by #544.
func TestTodolistGroupsService_ShipsNoMergeSafeComposite(t *testing.T) {
	if _, ok := reflect.TypeFor[TodolistGroup]().FieldByName("Description"); ok {
		t.Fatal("TodolistGroup now models Description — the reason for withholding a merge-safe group " +
			"Update/Edit is gone; see #544 and add the composite over the public Get")
	}

	svc := reflect.PointerTo(reflect.TypeFor[TodolistGroupsService]())
	for _, name := range []string{"Update", "Edit"} {
		if _, ok := svc.MethodByName(name); ok {
			t.Errorf("TodolistGroupsService.%s exists: a composite here cannot preserve a description "+
				"the group projection does not model, so it would erase it on every call", name)
		}
	}
	if _, ok := svc.MethodByName("Replace"); !ok {
		t.Error("expected TodolistGroupsService.Replace, the verbatim write this service does offer")
	}
}

// Merge-safe group writes go through the variant-agnostic Todolists composite:
// the endpoint is polymorphic, and a group body decodes into the todolist
// shape, so {name, description} survive the read-modify-write.
func TestTodolistsService_UpdatePreservesAGroupsDescription(t *testing.T) {
	groupBody := loadTodolistsFixture(t, "get.json")
	groupBody = patchTodolistFixture(t, groupBody, map[string]any{
		"name":        "Peripherals",
		"description": "<p>Ship the peripherals</p>",
		"parent": map[string]any{
			"id":      2,
			"title":   "Hardware",
			"type":    "Todolist",
			"url":     "https://3.basecampapi.com/999/buckets/1/todolists/2.json",
			"app_url": "https://3.basecamp.com/999/buckets/1/todolists/2",
		},
		"group_position_url": "https://3.basecampapi.com/999/buckets/1/todolists/groups/4/position.json",
	})

	svc, reqs := testTodolistsCaptureServer(t, groupBody, groupBody, nil)

	_, err := svc.Update(context.Background(), 1069479519, &UpdateTodolistRequest{
		Name: "Renamed group",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*reqs) != 2 {
		t.Fatalf("expected 2 requests (GET then PUT), got %d: %+v", len(*reqs), *reqs)
	}
	body := (*reqs)[1].body
	if body["name"] != "Renamed group" {
		t.Errorf("expected name 'Renamed group', got %v", body["name"])
	}
	if body["description"] != "<p>Ship the peripherals</p>" {
		t.Errorf("expected the group's description preserved, got %v", body["description"])
	}
}

func TestTodolistGroup_TimestampParsing(t *testing.T) {
	data := loadTodolistGroupsFixture(t, "get.json")

	var group TodolistGroup
	if err := json.Unmarshal(data, &group); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if group.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if group.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if group.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", group.CreatedAt.Year())
	}
}
