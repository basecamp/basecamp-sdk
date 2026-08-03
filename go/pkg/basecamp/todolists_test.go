package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func todolistsFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "todolists")
}

func loadTodolistsFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(todolistsFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestTodolist_UnmarshalList(t *testing.T) {
	data := loadTodolistsFixture(t, "list.json")

	var todolists []Todolist
	if err := json.Unmarshal(data, &todolists); err != nil {
		t.Fatalf("failed to unmarshal list.json: %v", err)
	}

	if len(todolists) != 2 {
		t.Errorf("expected 2 todolists, got %d", len(todolists))
	}

	// Verify first todolist
	tl1 := todolists[0]
	if tl1.ID != 1069479519 {
		t.Errorf("expected ID 1069479519, got %d", tl1.ID)
	}
	if tl1.Name != "Hardware" {
		t.Errorf("expected name 'Hardware', got %q", tl1.Name)
	}
	if tl1.Title != "Hardware" {
		t.Errorf("expected title 'Hardware', got %q", tl1.Title)
	}
	if tl1.Type != "Todolist" {
		t.Errorf("expected type 'Todolist', got %q", tl1.Type)
	}
	if tl1.Status != "active" {
		t.Errorf("expected status 'active', got %q", tl1.Status)
	}
	if tl1.CompletedRatio != "0/3" {
		t.Errorf("expected completed_ratio '0/3', got %q", tl1.CompletedRatio)
	}
	if tl1.Position != 1 {
		t.Errorf("expected position 1, got %d", tl1.Position)
	}

	// Verify second todolist has description
	tl2 := todolists[1]
	if tl2.ID != 1069479522 {
		t.Errorf("expected ID 1069479522, got %d", tl2.ID)
	}
	if tl2.Name != "Software" {
		t.Errorf("expected name 'Software', got %q", tl2.Name)
	}
	if tl2.Description != "Mobile and web app development tasks" {
		t.Errorf("expected description 'Mobile and web app development tasks', got %q", tl2.Description)
	}
}

func TestTodolist_UnmarshalGet(t *testing.T) {
	data := loadTodolistsFixture(t, "get.json")

	var todolist Todolist
	if err := json.Unmarshal(data, &todolist); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if todolist.ID != 1069479519 {
		t.Errorf("expected ID 1069479519, got %d", todolist.ID)
	}
	if todolist.Name != "Hardware" {
		t.Errorf("expected name 'Hardware', got %q", todolist.Name)
	}
	if todolist.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if todolist.Parent.ID != 1069479338 {
		t.Errorf("expected Parent.ID 1069479338, got %d", todolist.Parent.ID)
	}
	if todolist.Parent.Type != "Todoset" {
		t.Errorf("expected Parent.Type 'Todoset', got %q", todolist.Parent.Type)
	}
	if todolist.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if todolist.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", todolist.Bucket.ID)
	}
	if todolist.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if todolist.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", todolist.Creator.Name)
	}
	if todolist.TodosURL == "" {
		t.Error("expected non-empty TodosURL")
	}
	if todolist.GroupsURL == "" {
		t.Error("expected non-empty GroupsURL")
	}
}

func TestCreateTodolistRequest_Marshal(t *testing.T) {
	data := loadTodolistsFixture(t, "create-request.json")

	var req CreateTodolistRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal create-request.json: %v", err)
	}

	if req.Name != "Launch Tasks" {
		t.Errorf("expected name 'Launch Tasks', got %q", req.Name)
	}
	if req.Description != "Tasks for product launch" {
		t.Errorf("expected description 'Tasks for product launch', got %q", req.Description)
	}

	// Round-trip test
	out, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateTodolistRequest: %v", err)
	}

	var roundtrip CreateTodolistRequest
	if err := json.Unmarshal(out, &roundtrip); err != nil {
		t.Fatalf("failed to unmarshal round-trip: %v", err)
	}

	if roundtrip.Name != req.Name || roundtrip.Description != req.Description {
		t.Error("round-trip mismatch")
	}
}

func TestUpdateTodolistRequest_Marshal(t *testing.T) {
	data := loadTodolistsFixture(t, "update-request.json")

	var req UpdateTodolistRequest
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("failed to unmarshal update-request.json: %v", err)
	}

	if req.Name != "Updated Launch Tasks" {
		t.Errorf("expected name 'Updated Launch Tasks', got %q", req.Name)
	}
	if req.Description != "Updated description for launch tasks" {
		t.Errorf("expected description 'Updated description for launch tasks', got %q", req.Description)
	}
}

// --- httptest-based service contract tests ---

// testTodolistsServer creates an httptest.Server and a TodolistsService wired to it.
func testTodolistsServer(t *testing.T, handler http.HandlerFunc) *TodolistsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Todolists()
}

func TestTodolistsService_Get(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/todolists/1069479519" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	todolist, err := svc.Get(context.Background(), 1069479519)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todolist.ID != 1069479519 {
		t.Errorf("expected ID 1069479519, got %d", todolist.ID)
	}
	if todolist.Name != "Hardware" {
		t.Errorf("expected name 'Hardware', got %q", todolist.Name)
	}
	if todolist.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if todolist.Parent.ID != 1069479338 {
		t.Errorf("expected Parent.ID 1069479338, got %d", todolist.Parent.ID)
	}
	if todolist.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if todolist.Bucket.ID != 2085958500 {
		t.Errorf("expected Bucket.ID 2085958500, got %d", todolist.Bucket.ID)
	}
	if todolist.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if todolist.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", todolist.Creator.Name)
	}
}

// #544 consolidated Todolist, TodolistGroup and the TodolistOrGroup union into
// one flat shape. This pins the to-do-list variant end to end through the SDK:
// the list half of the structural discriminator is set (GroupsURL) and the
// group half is not (GroupPositionURL), and Color and CommentsAppURL — which
// the pre-#544 projections modelled on neither variant — arrive populated.
//
// Type is asserted only to document that it reads "Todolist" here just as it
// does on a group, which is exactly why nothing branches on it.
func TestTodolistsService_GetDecodesTheFlatListVariant(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	todolist, err := svc.Get(context.Background(), 1069479519)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const wantGroupsURL = "https://3.basecampapi.com/195539477/buckets/2085958500/todolists/1069479519/groups.json"
	if todolist.GroupsURL != wantGroupsURL {
		t.Errorf("GroupsURL: got %q, want %q — a list's parent is a Todoset, so groups_url is the variant marker", todolist.GroupsURL, wantGroupsURL)
	}
	if todolist.GroupPositionURL != "" {
		t.Errorf("GroupPositionURL: got %q, want empty — the two discriminators are mutually exclusive and this recording is a list", todolist.GroupPositionURL)
	}
	if todolist.Color != "blue" {
		t.Errorf("Color: got %q, want %q", todolist.Color, "blue")
	}
	const wantCommentsAppURL = "https://3.basecamp.com/195539477/buckets/2085958500/recordings/1069479519/comments"
	if todolist.CommentsAppURL != wantCommentsAppURL {
		t.Errorf("CommentsAppURL: got %q, want %q", todolist.CommentsAppURL, wantCommentsAppURL)
	}
	if todolist.Type != "Todolist" {
		t.Errorf("Type: got %q, want %q", todolist.Type, "Todolist")
	}
	// description is @required and never null: format_api_content returns ""
	// for a blank rich text, and description_attachments is [] alongside it.
	if todolist.Description != "" {
		t.Errorf("Description: got %q, want %q for this fixture", todolist.Description, "")
	}
	if todolist.DescriptionAttachments == nil {
		t.Error("DescriptionAttachments: got nil, want a non-nil empty slice — the server sent [], and collapsing that into nil loses the present-but-empty state")
	}
}

// The todoset-scoped list returns the same flat shape, one element per list.
func TestTodolistsService_ListDecodesTheFlatShape(t *testing.T) {
	fixture := loadTodolistsFixture(t, "list.json")
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/todosets/1069479338/todolists.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	result, err := svc.List(context.Background(), 1069479338, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Todolists) != 2 {
		t.Fatalf("expected 2 todolists, got %d", len(result.Todolists))
	}

	first := result.Todolists[0]
	if first.Name != "Hardware" {
		t.Errorf("Todolists[0].Name: got %q, want %q", first.Name, "Hardware")
	}
	if first.GroupsURL == "" {
		t.Error("Todolists[0].GroupsURL: got empty, want the list variant's discriminator")
	}
	if first.GroupPositionURL != "" {
		t.Errorf("Todolists[0].GroupPositionURL: got %q, want empty", first.GroupPositionURL)
	}
	if first.Color != "blue" {
		t.Errorf("Todolists[0].Color: got %q, want %q", first.Color, "blue")
	}
	if first.CommentsAppURL == "" {
		t.Error("Todolists[0].CommentsAppURL: got empty, want the in-app comments URL")
	}

	// color is null on this element: the key is always emitted, and a null
	// decodes to "" rather than leaking a nil pointer to callers.
	second := result.Todolists[1]
	if second.Description != "Mobile and web app development tasks" {
		t.Errorf("Todolists[1].Description: got %q, want %q", second.Description, "Mobile and web app development tasks")
	}
	if second.Color != "" {
		t.Errorf("Todolists[1].Color: got %q, want empty for a null color", second.Color)
	}
}

// --- Update / Edit / Replace triad ---

// patchTodolistFixture returns the fixture with the given top-level keys
// overwritten, so a test can state the current server state it needs.
func patchTodolistFixture(t *testing.T, base []byte, patch map[string]any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(base, &m); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	for k, v := range patch {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal patched fixture: %v", err)
	}
	return b
}

// capturedTodolistRequest records one request seen by testTodolistsCaptureServer.
type capturedTodolistRequest struct {
	method string
	path   string
	body   map[string]any
}

// testTodolistsCaptureServer serves getBody for GETs and putBody for PUTs
// while recording every request's method, path, and (for PUTs) decoded body.
// The extra hooks, when non-nil, are installed on the client.
func testTodolistsCaptureServer(t *testing.T, getBody, putBody []byte, hooks Hooks) (*TodolistsService, *[]capturedTodolistRequest) {
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
		if r.Method == "GET" {
			w.Write(getBody)
		} else {
			w.Write(putBody)
		}
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
	return client.ForAccount("99999").Todolists(), reqs
}

// A name-only update must not erase the description. BC3's
// TodolistsController#update rebuilds the recordable from only the permitted
// params, so a sparse PUT that omits description clears it server-side; the
// merge-safe composite carries the fetched value over.
func TestTodolistsService_UpdateMergesUnsetFields(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	getBody := patchTodolistFixture(t, fixture, map[string]any{
		"description": "<p>Ship the hardware</p>",
	})
	svc, reqs := testTodolistsCaptureServer(t, getBody, fixture, nil)

	todolist, err := svc.Update(context.Background(), 1069479519, &UpdateTodolistRequest{
		Name: "Renamed list",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todolist.ID != 1069479519 {
		t.Errorf("expected ID 1069479519, got %d", todolist.ID)
	}

	if len(*reqs) != 2 {
		t.Fatalf("expected 2 requests (GET then PUT), got %d: %+v", len(*reqs), *reqs)
	}
	if (*reqs)[0].method != "GET" || (*reqs)[1].method != "PUT" {
		t.Fatalf("expected GET then PUT, got %s then %s", (*reqs)[0].method, (*reqs)[1].method)
	}
	if (*reqs)[0].path != "/99999/todolists/1069479519" {
		t.Errorf("unexpected GET path: %s", (*reqs)[0].path)
	}
	if (*reqs)[1].path != "/99999/todolists/1069479519" {
		t.Errorf("unexpected PUT path: %s", (*reqs)[1].path)
	}

	body := (*reqs)[1].body
	if body["name"] != "Renamed list" {
		t.Errorf("expected name 'Renamed list', got %v", body["name"])
	}
	desc, ok := body["description"]
	if !ok {
		t.Fatalf("description missing from the PUT body — BC3 would clear it; body=%v", body)
	}
	if desc != "<p>Ship the hardware</p>" {
		t.Errorf("expected preserved description '<p>Ship the hardware</p>', got %v", desc)
	}
}

// A description-only update carries the fetched name over. name is
// presence-validated server-side, so dropping it would be a 422.
func TestTodolistsService_UpdateDescriptionOnlyKeepsName(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Update(context.Background(), 1069479519, &UpdateTodolistRequest{
		Description: "<p>new description</p>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	if body["name"] != "Hardware" {
		t.Errorf("expected preserved name 'Hardware', got %v", body["name"])
	}
	if body["description"] != "<p>new description</p>" {
		t.Errorf("expected description '<p>new description</p>', got %v", body["description"])
	}
}

// Both fields set: both overlay the fetched state.
func TestTodolistsService_UpdateOverlaysBothFields(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Update(context.Background(), 1069479519, &UpdateTodolistRequest{
		Name:        "Updated Name",
		Description: "Updated description",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	if body["name"] != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %v", body["name"])
	}
	if body["description"] != "Updated description" {
		t.Errorf("expected description 'Updated description', got %v", body["description"])
	}
}

func TestTodolistsService_UpdateNilRequestIsUsageError(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Update(context.Background(), 1069479519, nil)
	if err == nil {
		t.Fatal("expected a usage error for a nil request")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage || apiErr.Message != "update request is required" {
		t.Errorf("expected usage error %q, got %v", "update request is required", err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestTodolistsService_UpdateHooksObserveGetAndReplace(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, _ := testTodolistsCaptureServer(t, fixture, fixture, recorder)

	_, err := svc.Update(context.Background(), 1069479519, &UpdateTodolistRequest{Name: "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ops := make([]string, 0, len(recorder.opStartCalls))
	for _, op := range recorder.opStartCalls {
		ops = append(ops, op.Service+"."+op.Operation)
	}
	if len(ops) != 2 || ops[0] != "Todolists.Get" || ops[1] != "Todolists.Replace" {
		t.Errorf("expected operations [Todolists.Get Todolists.Replace], got %v", ops)
	}
	if len(recorder.opEndCalls) != 2 {
		t.Errorf("expected 2 OnOperationEnd calls, got %d", len(recorder.opEndCalls))
	}
}

func TestTodolistsService_Edit(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	getBody := patchTodolistFixture(t, fixture, map[string]any{
		"description": "<div>keep me</div>",
	})
	svc, reqs := testTodolistsCaptureServer(t, getBody, fixture, nil)

	todolist, err := svc.Edit(context.Background(), 1069479519, func(f *TodolistFields) error {
		if f.Name != "Hardware" {
			t.Errorf("expected Name from the GET, got %q", f.Name)
		}
		if f.Description != "<div>keep me</div>" {
			t.Errorf("expected Description from the GET, got %q", f.Description)
		}
		f.Name = "🚨 " + f.Name
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todolist.ID != 1069479519 {
		t.Errorf("expected ID 1069479519, got %d", todolist.ID)
	}

	body := (*reqs)[len(*reqs)-1].body
	if body["name"] != "🚨 Hardware" {
		t.Errorf("expected prefixed name '🚨 Hardware', got %v", body["name"])
	}
	if body["description"] != "<div>keep me</div>" {
		t.Errorf("expected preserved description '<div>keep me</div>', got %v", body["description"])
	}
}

// Clearing the description means present-and-empty on the wire, never JSON
// null (SPEC §18 body compaction) and never omitted.
func TestTodolistsService_EditClearsDescriptionExplicitly(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	getBody := patchTodolistFixture(t, fixture, map[string]any{
		"description": "<div>old</div>",
	})
	svc, reqs := testTodolistsCaptureServer(t, getBody, fixture, nil)

	_, err := svc.Edit(context.Background(), 1069479519, func(f *TodolistFields) error {
		f.Description = ""
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[len(*reqs)-1].body
	desc, ok := body["description"]
	if !ok {
		t.Fatalf("expected description present-and-empty in the PUT body, got it omitted; body=%v", body)
	}
	if desc != "" {
		t.Errorf("expected an empty description, got %v (%T)", desc, desc)
	}
	if desc == nil {
		t.Error("expected \"\", not JSON null")
	}
	if body["name"] != "Hardware" {
		t.Errorf("expected the untouched name 'Hardware' carried over, got %v", body["name"])
	}
}

func TestTodolistsService_EditClosureErrorAbortsWithoutPUT(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	wantErr := errors.New("nope")
	_, err := svc.Edit(context.Background(), 1069479519, func(f *TodolistFields) error {
		f.Name = "should never be written"
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the closure error, got %v", err)
	}

	for _, r := range *reqs {
		if r.method == "PUT" {
			t.Fatal("expected no PUT after a closure error")
		}
	}
}

// name is required: clearing it in the closure is a usage error, not a PUT
// the server would answer with a 422.
func TestTodolistsService_EditEmptyNameIsUsageError(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Edit(context.Background(), 1069479519, func(f *TodolistFields) error {
		f.Name = ""
		return nil
	})
	if err == nil {
		t.Fatal("expected a usage error for an empty name")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage || apiErr.Message != "todolist name is required" {
		t.Errorf("expected usage error %q, got %v", "todolist name is required", err)
	}
	for _, r := range *reqs {
		if r.method == "PUT" {
			t.Fatal("expected no PUT when the name is empty")
		}
	}
}

func TestTodolistsService_EditNilFuncIsUsageError(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Edit(context.Background(), 1069479519, nil)
	if err == nil {
		t.Fatal("expected a usage error for a nil edit function")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage || apiErr.Message != "edit function is required" {
		t.Errorf("expected usage error %q, got %v", "edit function is required", err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestTodolistsService_EditHooksObserveGetAndReplace(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, _ := testTodolistsCaptureServer(t, fixture, fixture, recorder)

	_, err := svc.Edit(context.Background(), 1069479519, func(f *TodolistFields) error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ops := make([]string, 0, len(recorder.opStartCalls))
	for _, op := range recorder.opStartCalls {
		ops = append(ops, op.Service+"."+op.Operation)
	}
	if len(ops) != 2 || ops[0] != "Todolists.Get" || ops[1] != "Todolists.Replace" {
		t.Errorf("expected operations [Todolists.Get Todolists.Replace], got %v", ops)
	}
}

// Replace is the server-native verbatim PUT: one request, no read-before-write,
// and an omitted description stays omitted (the server clears it).
func TestTodolistsService_ReplaceSendsSparseVerbatim(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	recorder := &recordingHooks{}
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, recorder)

	todolist, err := svc.Replace(context.Background(), 1069479519, &ReplaceTodolistRequest{
		Name: "The whole new list",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if todolist.ID != 1069479519 {
		t.Errorf("expected ID 1069479519, got %d", todolist.ID)
	}

	if len(*reqs) != 1 || (*reqs)[0].method != "PUT" {
		t.Fatalf("expected exactly one PUT, got %+v", *reqs)
	}
	if (*reqs)[0].path != "/99999/todolists/1069479519" {
		t.Errorf("unexpected PUT path: %s", (*reqs)[0].path)
	}
	body := (*reqs)[0].body
	if body["name"] != "The whole new list" {
		t.Errorf("expected name 'The whole new list', got %v", body["name"])
	}
	if _, ok := body["description"]; ok {
		t.Errorf("expected description omitted from a sparse replace, got %v", body["description"])
	}

	if len(recorder.opStartCalls) != 1 ||
		recorder.opStartCalls[0].Service != "Todolists" || recorder.opStartCalls[0].Operation != "Replace" {
		t.Errorf("expected a single Todolists.Replace operation, got %+v", recorder.opStartCalls)
	}
}

func TestTodolistsService_ReplaceSendsDescriptionWhenSet(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Replace(context.Background(), 1069479519, &ReplaceTodolistRequest{
		Name:        "The whole new list",
		Description: "<p>and its description</p>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := (*reqs)[0].body
	if body["description"] != "<p>and its description</p>" {
		t.Errorf("expected description '<p>and its description</p>', got %v", body["description"])
	}
}

func TestTodolistsService_ReplaceRequiresName(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Replace(context.Background(), 1069479519, &ReplaceTodolistRequest{
		Description: "<p>orphaned</p>",
	})
	if err == nil {
		t.Fatal("expected a usage error for a missing name")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage || apiErr.Message != "todolist name is required" {
		t.Errorf("expected usage error %q, got %v", "todolist name is required", err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("expected no requests, got %+v", *reqs)
	}
}

func TestTodolistsService_ReplaceNilRequestIsUsageError(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	svc, reqs := testTodolistsCaptureServer(t, fixture, fixture, nil)

	_, err := svc.Replace(context.Background(), 1069479519, nil)
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

// ReplaceTodolistRequest.Name carries no omitempty: an empty name must still
// serialize as a present key, because the wire contract makes name required
// and a dropped key would read as a preserve rather than the 422 it is.
func TestReplaceTodolistRequest_NameAlwaysMarshals(t *testing.T) {
	out, err := json.Marshal(ReplaceTodolistRequest{})
	if err != nil {
		t.Fatalf("failed to marshal ReplaceTodolistRequest: %v", err)
	}
	if string(out) != `{"name":""}` {
		t.Errorf("expected {\"name\":\"\"}, got %s", out)
	}
}

// A name that comes back empty is a malformed response, not a value to
// preserve: the server presence-validates it, so no real todolist has one.
// Classifying by origin is what keeps this distinct from the caller passing an
// empty name, which fullBody still rejects as usage.
func TestFieldsFromTodolist_EmptyNameIsAMalformedResponse(t *testing.T) {
	_, err := fieldsFromTodolist(&Todolist{ID: 2, Name: "", Description: "<p>Ship it</p>"})
	if err == nil {
		t.Fatal("expected an empty name from the wire to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "malformed response") {
		t.Errorf("error must name the response as the fault, got %q", err)
	}
	var typed *Error
	if errors.As(err, &typed) && typed.Code == CodeUsage {
		t.Error("a malformed response must not be reported as caller misuse")
	}
}

// fieldsFromTodolist lifts exactly the writable set — {name, description} —
// off a fetched todolist.
func TestFieldsFromTodolist(t *testing.T) {
	f, err := fieldsFromTodolist(&Todolist{
		Name:        "Hardware",
		Description: "<p>Ship it</p>",
		Title:       "Hardware",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Name != "Hardware" {
		t.Errorf("expected Name 'Hardware', got %q", f.Name)
	}
	if f.Description != "<p>Ship it</p>" {
		t.Errorf("expected Description '<p>Ship it</p>', got %q", f.Description)
	}

	body, err := f.fullBody()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(body) != 2 {
		t.Errorf("expected exactly {name, description} in the body, got %v", body)
	}
	if body["name"] != "Hardware" || body["description"] != "<p>Ship it</p>" {
		t.Errorf("unexpected body: %v", body)
	}
}

func TestTodolistsService_Reposition(t *testing.T) {
	var receivedBody map[string]int
	called := false
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/99999/todosets/todolists/1069479519/position.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(204)
	})

	if err := svc.Reposition(context.Background(), 1069479519, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected the server to be called")
	}
	if receivedBody["position"] != 3 {
		t.Errorf("expected request body position 3, got %d", receivedBody["position"])
	}
}

func TestTodolistsService_Reposition_NotFound(t *testing.T) {
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	err := svc.Reposition(context.Background(), 999, 1)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Errorf("expected not_found error, got: %v", err)
	}
}

func TestTodolistsService_Reposition_PositionTooLow(t *testing.T) {
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when position is below 1")
	})

	if err := svc.Reposition(context.Background(), 1069479519, 0); err == nil {
		t.Fatal("expected usage error for position < 1")
	}
}

func TestTodolistsService_Reposition_PositionOutOfRange(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("positions above MaxInt32 are unrepresentable as int on 32-bit platforms")
	}
	svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called when position exceeds MaxInt32")
	})

	// Build MaxInt32+1 at runtime: the constant math.MaxInt32+1 overflows int on
	// 32-bit and would not compile there, even though the guard above skips it.
	position := math.MaxInt32
	position++
	if err := svc.Reposition(context.Background(), 1069479519, position); err == nil {
		t.Fatal("expected usage error for position > MaxInt32")
	}
}

func TestTodolist_TimestampParsing(t *testing.T) {
	data := loadTodolistsFixture(t, "get.json")

	var todolist Todolist
	if err := json.Unmarshal(data, &todolist); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if todolist.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if todolist.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if todolist.CreatedAt.Year() != 2022 {
		t.Errorf("expected year 2022, got %d", todolist.CreatedAt.Year())
	}
}

// TestTodolistsService_CreateVisibleToClients verifies the tri-state
// visible_to_clients flag reaches the wire correctly on create: nil omits the
// key, true is sent verbatim, and an explicit false is sent (not dropped).
func TestTodolistsService_CreateVisibleToClients(t *testing.T) {
	fixture := loadTodolistsFixture(t, "get.json")
	tru, fls := true, false
	cases := []struct {
		name    string
		value   *bool
		present bool
		want    bool
	}{
		{"nil omits the field", nil, false, false},
		{"true is sent", &tru, true, true},
		{"explicit false is sent, not dropped", &fls, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedBody map[string]any
			svc := testTodolistsServer(t, func(w http.ResponseWriter, r *http.Request) {
				receivedBody = decodeRequestBody(t, r)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				w.Write(fixture)
			})

			_, err := svc.Create(context.Background(), 200, &CreateTodolistRequest{
				Name:             "Launch",
				VisibleToClients: tc.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			val, ok := receivedBody["visible_to_clients"]
			if ok != tc.present {
				t.Fatalf("visible_to_clients present=%v, want %v (body=%v)", ok, tc.present, receivedBody)
			}
			if tc.present && val != tc.want {
				t.Errorf("visible_to_clients=%v, want %v", val, tc.want)
			}
		})
	}
}
