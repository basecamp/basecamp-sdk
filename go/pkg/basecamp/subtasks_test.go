package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

func loadSubtasksFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "spec", "fixtures", "subtasks", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

// The fixtures are the documented examples from bc3's doc/api/sections/subtasks.md:
// the wire type stays "Kanban::Step", and the canonical urls say /subtasks.
func TestSubtask_UnmarshalGet(t *testing.T) {
	var subtask CardStep
	if err := json.Unmarshal(loadSubtasksFixture(t, "get.json"), &subtask); err != nil {
		t.Fatalf("failed to unmarshal get.json: %v", err)
	}

	if subtask.ID != 1069479879 {
		t.Errorf("expected ID 1069479879, got %d", subtask.ID)
	}
	if subtask.Type != "Kanban::Step" {
		t.Errorf("expected type Kanban::Step, got %q", subtask.Type)
	}
	if subtask.Title != "Outdoors, natural light" {
		t.Errorf("unexpected title %q", subtask.Title)
	}
	if subtask.Position != 3 {
		t.Errorf("expected position 3, got %d", subtask.Position)
	}
	if subtask.URL != "https://3.basecampapi.com/195539477/buckets/2085958504/subtasks/1069479879.json" {
		t.Errorf("unexpected URL %q", subtask.URL)
	}
	if subtask.CompletionURL != "https://3.basecampapi.com/195539477/subtasks/1069479879/completion.json" {
		t.Errorf("unexpected CompletionURL %q", subtask.CompletionURL)
	}
	if subtask.Parent == nil || subtask.Parent.Type != "Todo" || subtask.Parent.ID != 1069479876 {
		t.Errorf("expected a Todo parent 1069479876, got %+v", subtask.Parent)
	}
	if len(subtask.Assignees) != 1 || subtask.Assignees[0].Name != "Annie Bryan" {
		t.Errorf("expected one assignee Annie Bryan, got %+v", subtask.Assignees)
	}
}

// testSubtasksServer creates an httptest.Server and a SubtasksService wired to it.
func testSubtasksServer(t *testing.T, handler http.HandlerFunc) *SubtasksService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").Subtasks()
}

func TestSubtasksService_List(t *testing.T) {
	fixture := loadSubtasksFixture(t, "list.json")
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/recordings/200/subtasks.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("X-Total-Count", "2")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	result, err := svc.List(context.Background(), 200, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Subtasks) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(result.Subtasks))
	}
	if result.Meta.TotalCount != 2 {
		t.Errorf("expected TotalCount 2, got %d", result.Meta.TotalCount)
	}
	if result.Subtasks[0].ID != 1069479877 || result.Subtasks[0].Position != 1 {
		t.Errorf("expected first subtask 1069479877 at position 1, got %d at %d", result.Subtasks[0].ID, result.Subtasks[0].Position)
	}
	if result.Subtasks[1].Type != "Kanban::Step" {
		t.Errorf("expected wire type Kanban::Step, got %q", result.Subtasks[1].Type)
	}
	if result.Subtasks[1].Parent == nil || result.Subtasks[1].Parent.Title != "Shot list - indoor and outdoor" {
		t.Error("expected the parent to be mapped")
	}
}

func TestSubtasksService_List_Pagination(t *testing.T) {
	fixture := loadSubtasksFixture(t, "list.json")
	page2 := `[{"id":1069479880,"status":"active","visible_to_clients":false,"created_at":"2026-07-02T03:00:00.000Z","updated_at":"2026-07-02T03:00:00.000Z","title":"Page two","inherits_status":true,"type":"Kanban::Step","url":"https://3.basecampapi.com/195539477/buckets/2085958504/subtasks/1069479880.json","app_url":"https://3.basecamp.com/195539477/buckets/2085958504/todos/1069479876#__recording_1069479880","position":4,"parent":{"id":1069479876,"title":"Shot list","type":"Todo","url":"u","app_url":"a"},"bucket":{"id":2085958504,"name":"The Leto Laptop","type":"Project"},"creator":{"id":1,"name":"Matt Donahue"},"completed":false,"assignees":[]}]`

	var requestCount int
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if requestCount == 1 {
			w.Header().Set("X-Total-Count", "3")
			w.Header().Set("Link", fmt.Sprintf(`<%s/99999/recordings/200/subtasks.json?page=2>; rel="next"`, "http://"+r.Host))
			w.WriteHeader(200)
			w.Write(fixture)
		} else {
			w.WriteHeader(200)
			w.Write([]byte(page2))
		}
	})

	result, err := svc.List(context.Background(), 200, &SubtaskListOptions{Limit: -1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Subtasks) != 3 {
		t.Errorf("expected 3 subtasks across pages, got %d", len(result.Subtasks))
	}
	if requestCount != 2 {
		t.Errorf("expected 2 HTTP requests, got %d", requestCount)
	}
	if len(result.Subtasks) == 3 && result.Subtasks[2].ID != 1069479880 {
		t.Errorf("expected third subtask 1069479880, got %d", result.Subtasks[2].ID)
	}
}

func TestSubtasksService_List_SinglePage(t *testing.T) {
	fixture := loadSubtasksFixture(t, "list.json")
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("expected page=2, got %q", r.URL.RawQuery)
		}
		w.Header().Set("X-Total-Count", "100")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", fmt.Sprintf(`<%s/99999/recordings/200/subtasks.json?page=3>; rel="next"`, "http://"+r.Host))
		w.WriteHeader(200)
		w.Write(fixture)
	})

	result, err := svc.List(context.Background(), 200, &SubtaskListOptions{Page: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Subtasks) != 2 {
		t.Errorf("expected the one requested page (2 subtasks), got %d", len(result.Subtasks))
	}
}

func TestSubtasksService_Get(t *testing.T) {
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/subtasks/500" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(loadSubtasksFixture(t, "get.json"))
	})

	subtask, err := svc.Get(context.Background(), 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subtask.ID != 1069479879 {
		t.Errorf("expected ID 1069479879, got %d", subtask.ID)
	}
	if subtask.DueOn != "" {
		t.Errorf("expected no due date, got %q", subtask.DueOn)
	}
}

func TestSubtasksService_Get_NotFound(t *testing.T) {
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	_, err := svc.Get(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Errorf("expected not_found error, got: %v", err)
	}
}

func TestSubtasksService_Create(t *testing.T) {
	var received map[string]any
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/99999/recordings/200/subtasks.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(loadSubtasksFixture(t, "get.json"))
	})

	subtask, err := svc.Create(context.Background(), 200, &CreateSubtaskRequest{
		Title:       "Book the room",
		DueOn:       "2026-09-20",
		AssigneeIDs: []int64{30068628, 270913789},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subtask.ID != 1069479879 {
		t.Errorf("expected ID 1069479879, got %d", subtask.ID)
	}
	if received["title"] != "Book the room" {
		t.Errorf("expected title in body, got %v", received["title"])
	}
	if received["due_on"] != "2026-09-20" {
		t.Errorf("expected due_on in body, got %v", received["due_on"])
	}
	if ids, ok := received["assignee_ids"].([]any); !ok || len(ids) != 2 {
		t.Errorf("expected two assignee_ids in body, got %v", received["assignee_ids"])
	}
}

func TestSubtasksService_Create_Validation(t *testing.T) {
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("a request that fails client-side validation must not reach the wire")
	})

	for name, req := range map[string]*CreateSubtaskRequest{
		"nil request":   nil,
		"blank title":   {Title: ""},
		"malformed due": {Title: "x", DueOn: "20/09/2026"},
	} {
		_, err := svc.Create(context.Background(), 200, req)
		apiErr, ok := errors.AsType[*Error](err)
		if !ok || apiErr.Code != CodeUsage {
			t.Errorf("%s: expected usage error, got: %v", name, err)
		}
	}
}

// Update is a partial update: what the caller did not mention stays off the
// wire, and a pointer to "" is the explicit clear bc3 blank-casts to nil.
func TestSubtasksService_Update(t *testing.T) {
	var received map[string]any
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/99999/subtasks/500" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(loadSubtasksFixture(t, "get.json"))
	})

	subtask, err := svc.Update(context.Background(), 500, &UpdateSubtaskRequest{
		Title: "Book the big room",
		DueOn: Ptr(""),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if subtask.ID != 1069479879 {
		t.Errorf("expected ID 1069479879, got %d", subtask.ID)
	}
	if received["title"] != "Book the big room" {
		t.Errorf("expected title in body, got %v", received["title"])
	}
	if v, present := received["due_on"]; !present || v != "" {
		t.Errorf("expected an explicit due_on clear, got %v (present=%v)", v, present)
	}
	if _, present := received["assignee_ids"]; present {
		t.Error("assignee_ids was not mentioned and must stay off the wire")
	}
}

func TestSubtasksService_Update_ClearsAssignees(t *testing.T) {
	var raw string
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		raw = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(loadSubtasksFixture(t, "get.json"))
	})

	if _, err := svc.Update(context.Background(), 500, &UpdateSubtaskRequest{AssigneeIDs: []int64{}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw != `{"assignee_ids":[]}` {
		t.Errorf("expected an explicit empty assignee_ids, got %s", raw)
	}
}

func TestSubtasksService_Update_Validation(t *testing.T) {
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("a request that fails client-side validation must not reach the wire")
	})

	for name, req := range map[string]*UpdateSubtaskRequest{
		"nil request":   nil,
		"malformed due": {DueOn: Ptr("20/09/2026")},
	} {
		_, err := svc.Update(context.Background(), 500, req)
		apiErr, ok := errors.AsType[*Error](err)
		if !ok || apiErr.Code != CodeUsage {
			t.Errorf("%s: expected usage error, got: %v", name, err)
		}
	}
}

func TestSubtasksService_CompleteAndUncomplete(t *testing.T) {
	var methods []string
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/subtasks/500/completion.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		methods = append(methods, r.Method)
		w.WriteHeader(204)
	})

	if err := svc.Complete(context.Background(), 500); err != nil {
		t.Fatalf("complete: unexpected error: %v", err)
	}
	if err := svc.Uncomplete(context.Background(), 500); err != nil {
		t.Fatalf("uncomplete: unexpected error: %v", err)
	}
	if len(methods) != 2 || methods[0] != "POST" || methods[1] != "DELETE" {
		t.Errorf("expected POST then DELETE on the completion resource, got %v", methods)
	}
}

func TestSubtasksService_Reposition(t *testing.T) {
	var received map[string]any
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/99999/subtasks/500/position.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(204)
	})

	if err := svc.Reposition(context.Background(), 500, 4); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received["position"] != float64(4) {
		t.Errorf("expected position 4 in body, got %v", received["position"])
	}
}

// Positions are 1-based, and the wire carries an int32: a value the cast
// would wrap negative must be refused, not sent to the top of the list.
func TestSubtasksService_Reposition_RejectsOutOfRange(t *testing.T) {
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("an invalid position must not reach the wire")
	})

	for _, position := range []int{0, -1, math.MaxInt32 + 1} {
		err := svc.Reposition(context.Background(), 500, position)
		apiErr, ok := errors.AsType[*Error](err)
		if !ok || apiErr.Code != CodeUsage {
			t.Errorf("position %d: expected usage error, got: %v", position, err)
		}
	}
}

func TestSubtasksService_Delete(t *testing.T) {
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/99999/subtasks/500" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(204)
	})

	if err := svc.Delete(context.Background(), 500); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubtasksService_Delete_Forbidden(t *testing.T) {
	svc := testSubtasksServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	})

	err := svc.Delete(context.Background(), 500)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeForbidden {
		t.Errorf("expected forbidden error, got: %v", err)
	}
}

// The subtask accounting bc3 #12659 added to to-dos and cards flows through
// the wrappers, and stays absent on recordings that cannot hold subtasks.
func TestSubtaskCounts_PropagateFromGenerated(t *testing.T) {
	todo := todoFromGenerated(generated.Todo{
		SubtasksCount:          ptr(int32(101)),
		SubtasksCompletedCount: ptr(int32(7)),
		SubtasksUrl:            ptr("https://3.basecampapi.com/1/recordings/2/subtasks.json"),
	})
	if todo.SubtasksCount != 101 || todo.SubtasksCompletedCount != 7 || todo.SubtasksURL == "" {
		t.Errorf("todo subtask accounting did not propagate: %+v", todo)
	}

	card := cardFromGenerated(generated.Card{
		SubtasksCount:          ptr(int32(3)),
		SubtasksCompletedCount: ptr(int32(1)),
		SubtasksUrl:            ptr("https://3.basecampapi.com/1/recordings/3/subtasks.json"),
	})
	if card.SubtasksCount != 3 || card.SubtasksCompletedCount != 1 || card.SubtasksURL == "" {
		t.Errorf("card subtask accounting did not propagate: %+v", card)
	}

	message := recordingFromGenerated(generated.Recording{Type: "Message"})
	if message.SubtasksCount != nil || message.SubtasksCompletedCount != nil || message.SubtasksURL != nil {
		t.Error("a recording without subtask accounting must keep the fields nil")
	}
	rec := recordingFromGenerated(generated.Recording{
		Type:                   "Todo",
		SubtasksCount:          ptr(int32(0)),
		SubtasksCompletedCount: ptr(int32(0)),
		SubtasksUrl:            ptr("https://3.basecampapi.com/1/recordings/2/subtasks.json"),
	})
	if rec.SubtasksCount == nil || *rec.SubtasksCount != 0 || rec.SubtasksCompletedCount == nil || rec.SubtasksURL == nil {
		t.Errorf("an explicit zero count must round-trip on a recording: %+v", rec)
	}
}
