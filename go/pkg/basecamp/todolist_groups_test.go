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

// The group variant of the same flat shape (#544). Every field asserted here
// is one the pre-#544 TodolistGroup projection did not model at all, so a group
// came back with an empty Description, a nil DescriptionAttachments, and no way
// to tell it apart from a list except by re-reading the raw body.
//
// GroupsURL must be empty and GroupPositionURL must not: that pair, never the
// Type string, is what distinguishes the variants. Type reads "Todolist" here —
// asserted so the reason nothing branches on it stays visible.
func TestTodolistGroupsService_GetDecodesTheGroupVariant(t *testing.T) {
	fixture := loadTodolistGroupsFixture(t, "get.json")
	svc := testTodolistGroupsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	group, err := svc.Get(context.Background(), 1069479600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const wantDescription = "<div>Phase one hardware work</div>"
	if group.Description != wantDescription {
		t.Errorf("Description: got %q, want %q — BC3 renders a group through todolists/_todolist.json.jbuilder, so it carries one like any list", group.Description, wantDescription)
	}
	if group.DescriptionAttachments == nil {
		t.Error("DescriptionAttachments: got nil, want a non-nil empty slice — the server sent [], and the old group projection had no such field at all")
	}
	const wantGroupPositionURL = "https://3.basecampapi.com/195539477/buckets/2085958500/todolists/groups/1069479600/position.json"
	if group.GroupPositionURL != wantGroupPositionURL {
		t.Errorf("GroupPositionURL: got %q, want %q — a group's parent is a Todolist, so this is the variant marker", group.GroupPositionURL, wantGroupPositionURL)
	}
	if group.GroupsURL != "" {
		t.Errorf("GroupsURL: got %q, want empty — the two discriminators are mutually exclusive and this recording is a group", group.GroupsURL)
	}
	const wantCommentsAppURL = "https://3.basecamp.com/195539477/buckets/2085958500/recordings/1069479600/comments"
	if group.CommentsAppURL != wantCommentsAppURL {
		t.Errorf("CommentsAppURL: got %q, want %q", group.CommentsAppURL, wantCommentsAppURL)
	}
	if group.Type != "Todolist" {
		t.Errorf("Type: got %q, want %q — a group reports the list type, which is why discrimination is structural", group.Type, "Todolist")
	}
	// color is null on a group in this fixture; the key is always emitted and a
	// null decodes to "" rather than leaking a nil pointer to callers.
	if group.Color != "" {
		t.Errorf("Color: got %q, want empty for a null color", group.Color)
	}
}

// The shared fixture's description_attachments is [], which proves the key
// survives but not that the elements do. Inject one real attachment and read it
// back through the same projection.
func TestTodolistGroupsService_GetDecodesDescriptionAttachments(t *testing.T) {
	fixture := patchTodolistFixture(t, loadTodolistGroupsFixture(t, "get.json"), map[string]any{
		"description_attachments": []any{
			map[string]any{
				"id":            1234,
				"sgid":          "BAh7CEkiCGdpZAY6BkVU",
				"filename":      "spec.pdf",
				"content_type":  "application/pdf",
				"byte_size":     91234,
				"download_url":  "https://3.basecampapi.com/195539477/blobs/BAh7CEkiCGdpZAY6BkVU/download/spec.pdf",
				"previewable":   false,
				"preview_url":   "",
				"thumbnail_url": "",
			},
		},
	})
	svc := testTodolistGroupsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	group, err := svc.Get(context.Background(), 1069479600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(group.DescriptionAttachments) != 1 {
		t.Fatalf("DescriptionAttachments: got %d elements, want 1", len(group.DescriptionAttachments))
	}
	att := group.DescriptionAttachments[0]
	if att.ID != 1234 {
		t.Errorf("DescriptionAttachments[0].ID: got %d, want 1234", att.ID)
	}
	if att.Filename != "spec.pdf" {
		t.Errorf("DescriptionAttachments[0].Filename: got %q, want %q", att.Filename, "spec.pdf")
	}
	if att.ContentType != "application/pdf" {
		t.Errorf("DescriptionAttachments[0].ContentType: got %q, want %q", att.ContentType, "application/pdf")
	}
}

// The group list returns an array of the same flat shape.
func TestTodolistGroupsService_ListDecodesTheFlatShape(t *testing.T) {
	fixture := loadTodolistGroupsFixture(t, "list.json")
	svc := testTodolistGroupsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/todolists/1069479519/groups.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	result, err := svc.List(context.Background(), 1069479519, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(result.Groups))
	}

	first := result.Groups[0]
	if first.Name != "Phase 1" {
		t.Errorf("Groups[0].Name: got %q, want %q", first.Name, "Phase 1")
	}
	if first.Description != "<div>Phase one hardware work</div>" {
		t.Errorf("Groups[0].Description: got %q, want %q", first.Description, "<div>Phase one hardware work</div>")
	}
	if first.GroupPositionURL == "" {
		t.Error("Groups[0].GroupPositionURL: got empty, want the group variant's discriminator")
	}
	if first.GroupsURL != "" {
		t.Errorf("Groups[0].GroupsURL: got %q, want empty", first.GroupsURL)
	}
	if first.CommentsAppURL == "" {
		t.Error("Groups[0].CommentsAppURL: got empty, want the in-app comments URL")
	}

	// An empty description is "" and never null, so it stays distinguishable
	// from the attachments array, which is present-and-empty beside it.
	second := result.Groups[1]
	if second.Description != "" {
		t.Errorf("Groups[1].Description: got %q, want %q", second.Description, "")
	}
	if second.DescriptionAttachments == nil {
		t.Error("Groups[1].DescriptionAttachments: got nil, want a non-nil empty slice")
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
//
// It also round-trips since #544: the response projection carries description,
// so the group handed back is the one that was written. Before the
// consolidation the request field existed but the reply dropped it, and a
// caller had no way to confirm what it had just written.
func TestTodolistGroupsService_ReplaceCarriesDescription(t *testing.T) {
	// The server echoes a body whose description is the one being written, so
	// asserting on the returned group tests the response projection rather than
	// the fixture's stock value.
	fixture := patchTodolistFixture(t, loadTodolistGroupsFixture(t, "get.json"), map[string]any{
		"name":        "Updated Phase 1",
		"description": "<p>Ship the peripherals</p>",
	})
	svc, reqs := testTodolistGroupsCaptureServer(t, fixture, nil)

	group, err := svc.Replace(context.Background(), 1069479600, &ReplaceTodolistGroupRequest{
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

	if group.Name != "Updated Phase 1" {
		t.Errorf("returned Name: got %q, want %q", group.Name, "Updated Phase 1")
	}
	if group.Description != "<p>Ship the peripherals</p>" {
		t.Errorf("returned Description: got %q, want %q — the description must round-trip, not just reach the wire", group.Description, "<p>Ship the peripherals</p>")
	}
	if group.GroupPositionURL == "" {
		t.Error("returned GroupPositionURL: got empty, want the group discriminator carried back by the replace projection")
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

// TodolistGroup is an alias for Todolist, not a struct of its own. Asserted at
// compile time: two distinct named struct types are never assignable to each
// other in Go, however identical their fields, so this declaration stops
// compiling the moment the alias is respelled as a separate definition.
var _ Todolist = TodolistGroup{}

// TodolistGroupsService deliberately ships no merge-safe Update or Edit. This
// pins the reason so the gap cannot be closed by accident, and so it reopens on
// its own the moment the reason expires. The failure messages are written to be
// read cold, months from now, by someone who has never seen this decision.
//
// The reason is NOT data loss any more. Before #544 the group projection
// modelled no description, so a composite reading through it would have PUT
// back a zero value and erased the description on every call. TodolistGroup is
// now an alias for Todolist and carries the field, so that hazard is gone. What
// remains is a cross-SDK parity argument, which is smaller but still real.
func TestTodolistGroupsService_ShipsNoMergeSafeComposite(t *testing.T) {
	// The alias is load-bearing for the reason below: "todolists.Update already
	// addresses the same route through the same projection" is only true while
	// the group and the list ARE one type. reflect.Type identity is what
	// distinguishes an alias from a separate-but-identical struct — the
	// compile-time assertion above catches the same regression earlier, and this
	// carries the explanation.
	if reflect.TypeFor[TodolistGroup]() != reflect.TypeFor[Todolist]() {
		t.Fatalf(`TodolistGroup is no longer the same type as Todolist (got %v vs %v).

#544 consolidated Todolist, TodolistGroup and the TodolistOrGroup union into one flat shape,
because BC3 has no group model: todolists/groups/{index,show}.json.jbuilder render
todolists/_todolist.json.jbuilder, so a group IS a Todolist whose parent is a Todolist. Go
spells that as "type TodolistGroup = Todolist". If it has drifted back into a struct of its
own, the projections can diverge again silently — which is exactly how the group surface
came to drop description in the first place.

Restore the alias, or, if the split is deliberate, re-derive every claim that rests on the
two being one type: the "no merge-safe composite" reason below and in SPEC section 5, the
note on TodolistGroupsService.Replace, and todolistFromGenerated serving both services.`,
			reflect.TypeFor[TodolistGroup](), reflect.TypeFor[Todolist]())
	}

	svc := reflect.PointerTo(reflect.TypeFor[TodolistGroupsService]())
	for _, name := range []string{"Update", "Edit"} {
		if _, ok := svc.MethodByName(name); ok {
			t.Errorf(`TodolistGroupsService.%s exists, but this service deliberately ships no merge-safe composite.

The old reason — TodolistGroup modelled no description, so a composite would have read "" and
PUT that back, erasing it on a full-replace endpoint — expired with #544. TodolistGroup is now
an alias for Todolist and carries the field, so %s would be safe to build. It is still withheld
for a different and smaller reason:

  1. The other five SDKs (TypeScript, Ruby, Python, Kotlin, Swift) expose no group write at
     all — their TodolistGroups surface is List/Create/Reposition. Go already diverges by
     offering Replace; adding a composite on top would widen that asymmetry, not close a gap.
  2. There is nothing to close. PUT /{accountId}/todolists/{id} is one polymorphic route, and
     todolists.Update/Edit already address it through the very same projection — literally the
     same Go type. A group round-trips {name, description} through them correctly, with no
     type-sniffing. %s would be a sixth spelling of a composite that already works.

Either remove %s, or land the cross-SDK group-write surface first and update SPEC section 5's
"Go asymmetry" paragraph, the note on TodolistGroupsService.Replace, and this guard together.

Context: #544 (flat-shape consolidation), #545 (the Todolists triad this guard came from).`,
				name, name, name, name)
		}
	}
	if _, ok := svc.MethodByName("Replace"); !ok {
		t.Error(`expected TodolistGroupsService.Replace, the verbatim write this service does offer.

Replace is the raw PUT, renamed from update so the destructive path is honestly named
(SPEC section 18 rule 6). Removing it would leave the group surface with no write at all
and silently drop the only reason ReplaceTodolistGroupRequest exists.`)
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
