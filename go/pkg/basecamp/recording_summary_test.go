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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The demo account from spec/fixtures: every id below is the shared
// fixtures' own, and the two projects are its "The Leto Laptop" (2085958499)
// and "The Leto Locator" (2085958500).
const (
	summaryAccount = "195539477"
	letoLaptop     = int64(2085958499)
	letoLocator    = int64(2085958500)
)

func loadSummaryFixture(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "spec", "fixtures", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return data
}

// summaryServer records every request path (in order) and dispatches on the
// path with its ".json" suffix stripped, so single reads and lists match alike.
type summaryServer struct {
	t     *testing.T
	mu    sync.Mutex
	paths []string
	route func(w http.ResponseWriter, r *http.Request, path string) bool
}

func (s *summaryServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, ".json")
	s.mu.Lock()
	s.paths = append(s.paths, path)
	route := s.route
	s.mu.Unlock()
	if route != nil && route(w, r, path) {
		return
	}
	s.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	w.WriteHeader(http.StatusTeapot)
}

func (s *summaryServer) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...)
}

func (s *summaryServer) count(prefix string) int {
	n := 0
	for _, p := range s.requests() {
		if strings.HasPrefix(p, prefix) {
			n++
		}
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// newSummaryClient wires an AccountClient to a summaryServer. Retries are off
// so a 5xx is one request, and the Campfire index clock is injectable.
func newSummaryClient(t *testing.T, route func(w http.ResponseWriter, r *http.Request, path string) bool) (*AccountClient, *summaryServer, *time.Time) {
	t.Helper()
	srv := &summaryServer{t: t, route: route}
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithMaxRetries(0), WithBaseDelay(time.Millisecond))
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	clock := &now
	client.campfireOnce.Do(func() { client.campfireIdx = newCampfireIndexAt(func() time.Time { return *clock }) })
	return client.ForAccount(summaryAccount), srv, clock
}

// serveFixture answers one exact path with a fixture body.
func serveFixture(t *testing.T, path, fixture string) func(w http.ResponseWriter, r *http.Request, p string) bool {
	body := loadSummaryFixture(t, fixture)
	return func(w http.ResponseWriter, _ *http.Request, p string) bool {
		if p != path {
			return false
		}
		writeJSON(w, http.StatusOK, body)
		return true
	}
}

// campfireListJSON builds an account-wide Campfire listing from (id, bucket)
// pairs, in the given order.
func campfireListJSON(t *testing.T, pairs ...[2]int64) []byte {
	t.Helper()
	items := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		items = append(items, map[string]any{
			"id": p[0], "status": "active", "visible_to_clients": false,
			"created_at": "2022-10-28T15:25:00.000Z", "updated_at": "2022-10-28T15:25:00.000Z",
			"title": "Campfire", "inherits_status": true, "type": "Chat::Transcript",
			"url": "https://example.invalid/chats/" + itoa64(p[0]) + ".json", "app_url": "https://example.invalid/chats/" + itoa64(p[0]),
			"lines_url": "https://example.invalid/chats/" + itoa64(p[0]) + "/lines.json",
			"bucket":    map[string]any{"id": p[1], "name": "Project " + itoa64(p[1]), "type": "Project"},
			"creator":   map[string]any{"id": 1049715914, "name": "Victor Cooper"},
		})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// TestSummarize_RoutesEveryType drives one Summarize per routable type, by
// event type where the feed catalog names one and by recording type for the
// rest, and checks the read went to the route that type names and the
// projection carries the fixture's identity.
func TestSummarize_RoutesEveryType(t *testing.T) {
	cases := []struct {
		name      string
		ref       RecordingRef
		path      string
		fixture   string
		wantType  string
		wantTitle string
		creator   int64
		assignees int
		content   bool // the projection should carry rich text
	}{
		// The feed catalog, by event type — every type that names a recording.
		{"comment.created", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479361, EventType: "comment.created"}, "/195539477/comments/1069479361", "comments/get.json", "Comment", "Re: We won Leto!", 1049715915, 0, true},
		{"message.created", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479351, EventType: "message.created"}, "/195539477/messages/1069479351", "messages/get.json", "Message", "We won Leto!", 1049715914, 0, true},
		{"todo.created", RecordingRef{BucketID: letoLocator, RecordingID: 1069479520, EventType: "todo.created"}, "/195539477/todos/1069479520", "todos/get.json", "Todo", "Program Leto locator  microcontroller unit", 1049715915, 1, true},
		{"todo.completed", RecordingRef{BucketID: letoLocator, RecordingID: 1069479520, EventType: "todo.completed"}, "/195539477/todos/1069479520", "todos/get.json", "Todo", "Program Leto locator  microcontroller unit", 1049715915, 1, true},
		{"todo.assignment_changed", RecordingRef{BucketID: letoLocator, RecordingID: 1069479520, EventType: "todo.assignment_changed"}, "/195539477/todos/1069479520", "todos/get.json", "Todo", "Program Leto locator  microcontroller unit", 1049715915, 1, true},
		{"card.created", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, EventType: "card.created"}, "/195539477/card_tables/cards/1069479350", "cards/get.json", "Kanban::Card", "Implement user authentication", 1049715914, 1, true},
		{"card.completed", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, EventType: "card.completed"}, "/195539477/card_tables/cards/1069479350", "cards/get.json", "Kanban::Card", "Implement user authentication", 1049715914, 1, true},
		{"card.assignment_changed", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, EventType: "card.assignment_changed"}, "/195539477/card_tables/cards/1069479350", "cards/get.json", "Kanban::Card", "Implement user authentication", 1049715914, 1, true},
		{"card.moved (v2)", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, EventType: "card.moved"}, "/195539477/card_tables/cards/1069479350", "cards/get.json", "Kanban::Card", "Implement user authentication", 1049715914, 1, true},
		{"comment.updated (planned)", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479361, EventType: "comment.updated"}, "/195539477/comments/1069479361", "comments/get.json", "Comment", "Re: We won Leto!", 1049715915, 0, true},
		// The same reads by recording type.
		{"Comment", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479361, RecordingType: "Comment"}, "/195539477/comments/1069479361", "comments/get.json", "Comment", "Re: We won Leto!", 1049715915, 0, true},
		{"Message", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479351, RecordingType: "Message"}, "/195539477/messages/1069479351", "messages/get.json", "Message", "We won Leto!", 1049715914, 0, true},
		{"Todo", RecordingRef{BucketID: letoLocator, RecordingID: 1069479520, RecordingType: "Todo"}, "/195539477/todos/1069479520", "todos/get.json", "Todo", "Program Leto locator  microcontroller unit", 1049715915, 1, true},
		{"Kanban::Card", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, RecordingType: "Kanban::Card"}, "/195539477/card_tables/cards/1069479350", "cards/get.json", "Kanban::Card", "Implement user authentication", 1049715914, 1, true},
		// Types the feed does not catalog yet, reachable by recording type.
		{"Document", RecordingRef{BucketID: letoLocator, RecordingID: 1069479300, RecordingType: "Document"}, "/195539477/documents/1069479300", "documents/get.json", "Document", "Project Overview", 1049715915, 0, true},
		{"Upload", RecordingRef{BucketID: letoLocator, RecordingID: 1069479400, RecordingType: "Upload"}, "/195539477/uploads/1069479400", "uploads/get.json", "Upload", "logo.png", 1049715915, 0, true},
		{"Schedule::Entry", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479400, RecordingType: "Schedule::Entry"}, "/195539477/schedule_entries/1069479400", "schedules/entry_get.json", "Schedule::Entry", "Project Kickoff Meeting", 1049715914, 0, true},
		{"Question", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479410, RecordingType: "Question"}, "/195539477/questions/1069479410", "checkins/question.json", "Question", "What did you work on today?", 1049715914, 0, false},
		{"Question::Answer", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479450, RecordingType: "Question::Answer"}, "/195539477/question_answers/1069479450", "checkins/answer.json", "Question::Answer", "What did you work on today?", 1049715914, 0, true},
		{"Todolist", RecordingRef{BucketID: letoLocator, RecordingID: 1069479519, RecordingType: "Todolist"}, "/195539477/todolists/1069479519", "todolists/get.json", "Todolist", "Hardware", 1049715915, 0, false},
		{"Vault", RecordingRef{BucketID: letoLocator, RecordingID: 1069479098, RecordingType: "Vault"}, "/195539477/vaults/1069479098", "vaults/get.json", "Vault", "Docs & Files", 1049715915, 0, false},
		{"Inbox::Forward", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479380, RecordingType: "Inbox::Forward"}, "/195539477/inbox_forwards/1069479380", "forwards/get.json", "Inbox::Forward", "Project proposal from client", 1049715914, 0, true},
		{"Client::Approval", RecordingRef{BucketID: letoLocator, RecordingID: 1069479651, RecordingType: "Client::Approval"}, "/195539477/client/approvals/1069479651", "client_approvals/get.json", "Client::Approval", "New logo for the website", 1049715915, 0, true},
		{"Client::Correspondence", RecordingRef{BucketID: letoLocator, RecordingID: 1069479566, RecordingType: "Client::Correspondence"}, "/195539477/client/correspondences/1069479566", "client_correspondences/get.json", "Client::Correspondence", "Project kickoff!", 1049715929, 0, true},
		{"GoogleDocument", RecordingRef{BucketID: letoLocator, RecordingID: 1069480366, RecordingType: "GoogleDocument"}, "/195539477/google_documents/1069480366", "google_documents/get.json", "GoogleDocument", "Roadmap (draft)", 1049715915, 0, true},
		{"CloudFile", RecordingRef{BucketID: letoLocator, RecordingID: 1069480357, RecordingType: "CloudFile"}, "/195539477/cloud_files/1069480357", "cloud_files/get.json", "CloudFile", "Brand book draft", 1049715915, 0, true},
		{"Kanban::Step", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479360, RecordingType: "Kanban::Step"}, "/195539477/card_tables/steps/1069479360", "cards/step.json", "Kanban::Step", "Set up OAuth providers", 1049715914, 1, false},
		// The tool-shaped recordings: id-only reads, no feed event points at them yet.
		{"Questionnaire", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479400, RecordingType: "Questionnaire"}, "/195539477/questionnaires/1069479400", "checkins/questionnaire.json", "Questionnaire", "Automatic Check-ins", 1049715914, 0, false},
		{"Schedule", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479342, RecordingType: "Schedule"}, "/195539477/schedules/1069479342", "schedules/get.json", "Schedule", "Schedule", 1049715914, 0, false},
		{"Todoset", RecordingRef{BucketID: letoLocator, RecordingID: 1069479338, RecordingType: "Todoset"}, "/195539477/todosets/1069479338", "todosets/get.json", "Todoset", "To-dos", 1049715915, 0, false},
		{"Message::Board", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479338, RecordingType: "Message::Board"}, "/195539477/message_boards/1069479338", "message_boards/get.json", "Message::Board", "Message Board", 1049715914, 0, false},
		{"Kanban::Board", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479345, RecordingType: "Kanban::Board"}, "/195539477/card_tables/1069479345", "cards/card_table.json", "Kanban::Board", "Development Board", 1049715914, 0, false},
		{"Kanban::Column", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479347, RecordingType: "Kanban::Column"}, "/195539477/card_tables/columns/1069479347", "cards/column.json", "Kanban::Column", "In Progress", 1049715914, 0, true},
		{"Inbox", RecordingRef{BucketID: letoLaptop, RecordingID: 1069479342, RecordingType: "Inbox"}, "/195539477/inboxes/1069479342", "forwards/inbox.json", "Inbox", "Email Forwards", 1049715914, 0, false},
		{"Chat::Transcript", RecordingRef{BucketID: 2085958499, RecordingID: 1069479345, RecordingType: "Chat::Transcript"}, "/195539477/chats/1069479345", "campfires/get.json", "Chat::Transcript", "Campfire", 1049715914, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account, srv, _ := newSummaryClient(t, serveFixture(t, tc.path, tc.fixture))
			got, err := account.Recordings().Summarize(context.Background(), tc.ref)
			if err != nil {
				t.Fatalf("Summarize: %v", err)
			}
			if reqs := srv.requests(); !reflect.DeepEqual(reqs, []string{tc.path}) {
				t.Fatalf("requests = %v, want exactly [%s]", reqs, tc.path)
			}
			if got.ID != tc.ref.RecordingID {
				t.Errorf("ID = %d, want %d", got.ID, tc.ref.RecordingID)
			}
			if got.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tc.wantType)
			}
			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			if got.Status != "active" {
				t.Errorf("Status = %q, want active", got.Status)
			}
			if got.AppURL == "" {
				t.Error("AppURL is empty")
			}
			if got.Bucket == nil || got.Bucket.ID != tc.ref.BucketID {
				t.Errorf("Bucket = %+v, want id %d", got.Bucket, tc.ref.BucketID)
			}
			if got.Creator == nil || got.Creator.ID != tc.creator {
				t.Errorf("Creator = %+v, want id %d", got.Creator, tc.creator)
			}
			if len(got.Assignees) != tc.assignees {
				t.Errorf("Assignees = %d, want %d", len(got.Assignees), tc.assignees)
			}
			if (got.Content != "") != tc.content {
				t.Errorf("Content present = %v, want %v (%q)", got.Content != "", tc.content, got.Content)
			}
			if got.UpdatedAt.IsZero() {
				t.Error("UpdatedAt is zero")
			}
			if got.MentionedPersonIDs == nil {
				t.Error("MentionedPersonIDs is nil; want an empty slice")
			}
			if got.CampfireID != 0 {
				t.Errorf("CampfireID = %d on a non-chat type", got.CampfireID)
			}
		})
	}
}

func TestSummarize_RefusesUnroutableTypesWithoutARequest(t *testing.T) {
	account, srv, _ := newSummaryClient(t, nil)
	cases := []struct {
		name string
		ref  RecordingRef
		want error
	}{
		{"boost.created names no recording type", RecordingRef{BucketID: 1, RecordingID: 2, EventType: "boost.created"}, ErrNoRecordingType},
		{"unknown event type", RecordingRef{BucketID: 1, RecordingID: 2, EventType: "project.created"}, ErrUnknownRecordingType},
		{"event type without an action", RecordingRef{BucketID: 1, RecordingID: 2, EventType: "comment"}, ErrUnknownRecordingType},
		{"unknown recording type", RecordingRef{BucketID: 1, RecordingID: 2, RecordingType: "Client::Reply"}, ErrUnknownRecordingType},
		{"a reply whose read needs its forward's id", RecordingRef{BucketID: 1, RecordingID: 2, RecordingType: "Forward::Reply"}, ErrUnknownRecordingType},
		{"no type at all", RecordingRef{BucketID: 1, RecordingID: 2}, ErrUnknownRecordingType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := account.Recordings().Summarize(context.Background(), tc.ref)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			var routing *RecordingRoutingError
			if !errors.As(err, &routing) || routing.Ref != tc.ref {
				t.Fatalf("err = %#v, want a RecordingRoutingError carrying the ref", err)
			}
		})
	}
	if reqs := srv.requests(); len(reqs) != 0 {
		t.Fatalf("routing failures made requests: %v", reqs)
	}
	for _, ref := range []RecordingRef{{RecordingID: 1, EventType: "comment.created"}, {BucketID: 1, EventType: "comment.created"}} {
		if _, err := account.Recordings().Summarize(context.Background(), ref); err == nil {
			t.Errorf("expected a usage error for %+v", ref)
		}
	}
}

func TestSummarize_RecordingTypeWinsOverEventType(t *testing.T) {
	account, srv, _ := newSummaryClient(t, serveFixture(t, "/195539477/comments/1069479361", "comments/get.json"))
	got, err := account.Recordings().Summarize(context.Background(), RecordingRef{
		BucketID: letoLaptop, RecordingID: 1069479361, EventType: "card.created", RecordingType: "Comment",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "Comment" || srv.count("/195539477/comments/") != 1 {
		t.Fatalf("read %v, want the comment route", srv.requests())
	}
}

func TestSummarize_BucketMismatchIsAnError(t *testing.T) {
	account, _, _ := newSummaryClient(t, serveFixture(t, "/195539477/comments/1069479361", "comments/get.json"))
	ref := RecordingRef{BucketID: letoLocator, RecordingID: 1069479361, EventType: "comment.created"}
	_, err := account.Recordings().Summarize(context.Background(), ref)
	if !errors.Is(err, ErrBucketMismatch) {
		t.Fatalf("err = %v, want ErrBucketMismatch", err)
	}
	var mismatch *BucketMismatchError
	if !errors.As(err, &mismatch) || mismatch.BucketID != letoLaptop || mismatch.Ref != ref {
		t.Fatalf("err = %#v", err)
	}
}

func TestSummarize_ReadErrorsPassThrough(t *testing.T) {
	account, _, _ := newSummaryClient(t, func(w http.ResponseWriter, _ *http.Request, _ string) bool {
		writeJSON(w, http.StatusNotFound, []byte(`{"error":"Record not found"}`))
		return true
	})
	_, err := account.Recordings().Summarize(context.Background(), RecordingRef{BucketID: letoLaptop, RecordingID: 5, EventType: "comment.created"})
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeNotFound {
		t.Fatalf("err = %v, want the read's not_found", err)
	}
	if errors.Is(err, ErrRecordingUnresolved) {
		t.Fatal("a typed read's 404 must not read as a chat line unresolved")
	}
}

// chatFixture is the server side of chat line discovery: an account-wide
// Campfire listing, a project dock per bucket (a bucket absent from docks is
// not a project and answers 404), and the line under one Campfire; every
// other Campfire answers 404 for it, or whatever lineStatus says.
type chatFixture struct {
	listing    atomic.Pointer[[]byte]
	mu         sync.Mutex
	docks      map[int64]int64 // bucket -> the dock's chat tool id
	buckets    map[int64]int64 // campfire -> its bucket, for the served line
	foundUnder atomic.Int64
	lineStatus func(campfireID int64) int
	dockStatus int // non-zero: the project read answers this instead of a dock
}

func newChatFixture(t *testing.T, foundUnder int64, pairs ...[2]int64) *chatFixture {
	t.Helper()
	f := &chatFixture{docks: map[int64]int64{}, buckets: map[int64]int64{}}
	f.setListing(t, pairs...)
	f.foundUnder.Store(foundUnder)
	return f
}

func (f *chatFixture) setListing(t *testing.T, pairs ...[2]int64) {
	t.Helper()
	body := campfireListJSON(t, pairs...)
	f.listing.Store(&body)
	f.mu.Lock()
	for _, p := range pairs {
		f.buckets[p[0]] = p[1]
	}
	f.mu.Unlock()
}

// setDock names a bucket's dock Campfire (0 for a project with none).
func (f *chatFixture) setDock(bucket, campfire int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.docks[bucket] = campfire
	if campfire != 0 {
		f.buckets[campfire] = bucket
	}
}

func (f *chatFixture) route(t *testing.T) func(w http.ResponseWriter, r *http.Request, path string) bool {
	line := loadSummaryFixture(t, "campfires/line_get.json")
	project := loadSummaryFixture(t, "projects/get.json") // dock chat tool 1069479341, bucket 2085958499
	return func(w http.ResponseWriter, _ *http.Request, path string) bool {
		if bucket, ok := strings.CutPrefix(path, "/195539477/projects/"); ok {
			if f.dockStatus != 0 {
				writeJSON(w, f.dockStatus, []byte(`{"error":"nope"}`))
				return true
			}
			f.mu.Lock()
			chatID, isProject := f.docks[parseID(bucket)]
			f.mu.Unlock()
			if !isProject {
				writeJSON(w, http.StatusNotFound, []byte(`{"error":"Record not found"}`))
				return true
			}
			var p map[string]any
			_ = json.Unmarshal(project, &p)
			p["id"] = parseID(bucket)
			for _, item := range p["dock"].([]any) {
				if entry := item.(map[string]any); entry["name"] == "chat" {
					entry["id"] = chatID
				}
			}
			writeJSON(w, http.StatusOK, mustJSON(p))
			return true
		}
		if path == "/195539477/chats" {
			writeJSON(w, http.StatusOK, *f.listing.Load())
			return true
		}
		rest, ok := strings.CutPrefix(path, "/195539477/chats/")
		if !ok {
			return false
		}
		campfire, lineID, ok := strings.Cut(rest, "/lines/")
		if !ok || lineID != "1069479350" {
			return false
		}
		id := parseID(campfire)
		if f.lineStatus != nil {
			if status := f.lineStatus(id); status != 0 {
				writeJSON(w, status, []byte(`{"error":"nope"}`))
				return true
			}
		}
		if id == f.foundUnder.Load() {
			// The served line belongs to the Campfire's bucket.
			var l map[string]any
			_ = json.Unmarshal(line, &l)
			f.mu.Lock()
			bucketID, known := f.buckets[id]
			f.mu.Unlock()
			if known {
				l["bucket"].(map[string]any)["id"] = bucketID
			}
			writeJSON(w, http.StatusOK, mustJSON(l))
			return true
		}
		writeJSON(w, http.StatusNotFound, []byte(`{"error":"Record not found"}`))
		return true
	}
}

func parseID(s string) int64 {
	var id int64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
	}
	return id
}

func lineRef() RecordingRef {
	return RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, EventType: "chat.line.created"}
}

const (
	projectRead = "/195539477/projects/2085958499"
	listingRead = "/195539477/chats"
)

func lineRead(campfireID int64) string {
	return "/195539477/chats/" + itoa64(campfireID) + "/lines/1069479350"
}

func TestSummarize_ChatLineFoundViaProjectDock(t *testing.T) {
	fx := newChatFixture(t, 1069479341, [2]int64{1069479341, letoLaptop})
	fx.setDock(letoLaptop, 1069479341)
	account, srv, _ := newSummaryClient(t, fx.route(t))

	got, err := account.Recordings().Summarize(context.Background(), lineRef())
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	// The dock answered; the account-wide listing was never needed.
	if want := []string{projectRead, lineRead(1069479341)}; !reflect.DeepEqual(srv.requests(), want) {
		t.Fatalf("requests = %v, want %v", srv.requests(), want)
	}
	if got.CampfireID != 1069479341 {
		t.Errorf("CampfireID = %d", got.CampfireID)
	}
	// The dock is cached: a second line in the same bucket costs one read.
	if _, err := account.Recordings().Summarize(context.Background(), lineRef()); err != nil {
		t.Fatal(err)
	}
	if want := []string{projectRead, lineRead(1069479341), lineRead(1069479341)}; !reflect.DeepEqual(srv.requests(), want) {
		t.Fatalf("requests = %v, want %v", srv.requests(), want)
	}
}

func TestSummarize_ChatLineFoundUnderSecondCampfire(t *testing.T) {
	// A bucket that is not a project (a Circle, say): no dock. Two Campfires
	// in the line's bucket, one in another project that must never be tried.
	fx := newChatFixture(t, 1069479345, [2]int64{1069479400, letoLocator}, [2]int64{1069479340, letoLaptop}, [2]int64{1069479345, letoLaptop})
	account, srv, _ := newSummaryClient(t, fx.route(t))

	got, err := account.Recordings().Summarize(context.Background(), lineRef())
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	want := []string{projectRead, listingRead, lineRead(1069479340), lineRead(1069479345)}
	if reqs := srv.requests(); !reflect.DeepEqual(reqs, want) {
		t.Fatalf("requests = %v, want %v", reqs, want)
	}
	if got.CampfireID != 1069479345 {
		t.Errorf("CampfireID = %d, want 1069479345", got.CampfireID)
	}
	if got.Type != "Chat::Lines::Text" || got.Content != "Hello everyone!" || got.Title != "Hello everyone!" {
		t.Errorf("projection = %+v", got)
	}
	if got.Parent == nil || got.Parent.ID != 1069479345 || got.Parent.Type != "Chat::Transcript" {
		t.Errorf("Parent = %+v, want the Campfire", got.Parent)
	}
	if got.Creator == nil || got.Creator.ID != 1049715914 {
		t.Errorf("Creator = %+v", got.Creator)
	}

	// The same by recording type, each Chat::Lines subtype, from the cache.
	for _, typ := range []string{"Chat::Lines::Text", "Chat::Lines::RichText", "Chat::Lines::Code", "Chat::Lines::Upload", "Chat::Lines::Integration"} {
		if _, err := account.Recordings().Summarize(context.Background(), RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, RecordingType: typ}); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
	if n := srv.count("/195539477/chats/"); n != 2+5*2 {
		t.Errorf("line reads = %d, want 2 for the first call and 2 per typed call", n)
	}
	if n := srv.count(listingRead) - srv.count(listingRead+"/"); n != 1 {
		t.Errorf("listings = %d within the TTL, want 1", n)
	}
	if n := srv.count(projectRead); n != 1 {
		t.Errorf("project reads = %d within the TTL, want 1", n)
	}
}

func TestSummarize_ChatLineDockMissFallsBackToListing(t *testing.T) {
	// The dock names a Campfire the line is not in (it was posted in a ping
	// listed for the same bucket); the listing supplies the rest.
	fx := newChatFixture(t, 1069479345, [2]int64{1069479341, letoLaptop}, [2]int64{1069479345, letoLaptop})
	fx.setDock(letoLaptop, 1069479341)
	account, srv, _ := newSummaryClient(t, fx.route(t))
	got, err := account.Recordings().Summarize(context.Background(), lineRef())
	if err != nil {
		t.Fatal(err)
	}
	// The dock's Campfire is not tried twice when the listing repeats it.
	want := []string{projectRead, lineRead(1069479341), listingRead, lineRead(1069479345)}
	if !reflect.DeepEqual(srv.requests(), want) {
		t.Fatalf("requests = %v, want %v", srv.requests(), want)
	}
	if got.CampfireID != 1069479345 {
		t.Errorf("CampfireID = %d", got.CampfireID)
	}
}

func TestSummarize_ChatLineSourcesAreCachedTenMinutes(t *testing.T) {
	fx := newChatFixture(t, 1069479345, [2]int64{1069479345, letoLaptop})
	account, srv, clock := newSummaryClient(t, fx.route(t))

	listings := func() int { return srv.count(listingRead) - srv.count(listingRead+"/") }
	for i := 0; i < 3; i++ {
		if _, err := account.Recordings().Summarize(context.Background(), lineRef()); err != nil {
			t.Fatal(err)
		}
	}
	if listings() != 1 || srv.count(projectRead) != 1 {
		t.Fatalf("listings = %d, project reads = %d after three lookups, want 1 each", listings(), srv.count(projectRead))
	}
	*clock = clock.Add(CampfireIndexTTL - time.Second)
	if _, err := account.Recordings().Summarize(context.Background(), lineRef()); err != nil {
		t.Fatal(err)
	}
	if listings() != 1 || srv.count(projectRead) != 1 {
		t.Fatalf("re-read just inside the TTL: listings = %d, project reads = %d", listings(), srv.count(projectRead))
	}
	*clock = clock.Add(2 * time.Second)
	if _, err := account.Recordings().Summarize(context.Background(), lineRef()); err != nil {
		t.Fatal(err)
	}
	if listings() != 2 || srv.count(projectRead) != 2 {
		t.Fatalf("past the TTL: listings = %d, project reads = %d, want 2 each", listings(), srv.count(projectRead))
	}

	// A second AccountClient from the same Client shares the sources.
	other := account.parent.ForAccount(summaryAccount)
	if _, err := other.Recordings().Summarize(context.Background(), lineRef()); err != nil {
		t.Fatal(err)
	}
	if listings() != 2 || srv.count(projectRead) != 2 {
		t.Fatalf("a sibling AccountClient re-read the sources")
	}
}

func TestSummarize_ChatLineUnresolvedIsDistinct(t *testing.T) {
	fx := newChatFixture(t, 0, [2]int64{1069479340, letoLaptop}, [2]int64{1069479345, letoLaptop}) // found under none
	account, srv, clock := newSummaryClient(t, fx.route(t))
	listings := func() int { return srv.count(listingRead) - srv.count(listingRead+"/") }

	_, err := account.Recordings().Summarize(context.Background(), lineRef())
	if !errors.Is(err, ErrRecordingUnresolved) {
		t.Fatalf("err = %v, want ErrRecordingUnresolved", err)
	}
	var unresolved *UnresolvedRecordingError
	if !errors.As(err, &unresolved) {
		t.Fatalf("err = %#v", err)
	}
	if unresolved.BucketID != letoLaptop || unresolved.RecordingID != 1069479350 || !reflect.DeepEqual(unresolved.CampfireIDs, []int64{1069479340, 1069479345}) {
		t.Fatalf("unresolved = %+v", unresolved)
	}
	if unresolved.Refreshed || unresolved.StaleCampfireIDs != nil {
		t.Fatalf("a conclusion on sources read this very call claims a refresh: %+v", unresolved)
	}
	if apiErr, ok := errors.AsType[*Error](err); ok {
		t.Fatalf("unresolved surfaced as an API error: %+v", apiErr)
	}
	if errors.Is(err, ErrCampfireDiscoveryIncomplete) {
		t.Fatal("a complete search must not read as incomplete")
	}
	if listings() != 1 || srv.count(projectRead) != 1 {
		t.Fatalf("sources read this call were re-read on the miss: listings = %d, project reads = %d", listings(), srv.count(projectRead))
	}

	// Inside the refresh floor the cached sources stand and the error says so.
	*clock = clock.Add(campfireIndexMinRefresh - time.Second)
	_, err = account.Recordings().Summarize(context.Background(), lineRef())
	if !errors.As(err, &unresolved) || unresolved.Refreshed {
		t.Fatalf("err = %#v, want unresolved with Refreshed=false inside the floor", err)
	}
	if listings() != 1 || srv.count(projectRead) != 1 {
		t.Fatalf("the floor did not hold: listings = %d, project reads = %d", listings(), srv.count(projectRead))
	}

	// Past the floor, a miss re-reads both sources once and tries only what
	// is new — here a Campfire created after the cache filled, which is where
	// the line was all along.
	*clock = clock.Add(time.Second)
	fx.setListing(t, [2]int64{1069479340, letoLaptop}, [2]int64{1069479345, letoLaptop}, [2]int64{1069479399, letoLaptop})
	fx.foundUnder.Store(1069479399)
	got, err := account.Recordings().Summarize(context.Background(), lineRef())
	if err != nil {
		t.Fatalf("after refresh: %v", err)
	}
	if got.CampfireID != 1069479399 {
		t.Fatalf("CampfireID = %d, want the newly listed Campfire", got.CampfireID)
	}
	reqs := srv.requests()
	// Both cached sources are tried first; then the dock is re-read, then the
	// listing.
	want := []string{lineRead(1069479340), lineRead(1069479345), projectRead, listingRead, lineRead(1069479399)}
	if tail := reqs[len(reqs)-len(want):]; !reflect.DeepEqual(tail, want) {
		t.Fatalf("requests = %v, want tail %v", reqs, want)
	}

	// A refreshed miss reports the Campfires the cache had that the sources no
	// longer list: visibility changed, and 404 alone cannot say so.
	*clock = clock.Add(campfireIndexMinRefresh)
	fx.setListing(t, [2]int64{1069479345, letoLaptop})
	fx.foundUnder.Store(0)
	_, err = account.Recordings().Summarize(context.Background(), lineRef())
	if !errors.As(err, &unresolved) || !unresolved.Refreshed {
		t.Fatalf("err = %#v, want unresolved with Refreshed=true past the floor", err)
	}
	if !reflect.DeepEqual(unresolved.CampfireIDs, []int64{1069479340, 1069479345, 1069479399}) || !reflect.DeepEqual(unresolved.StaleCampfireIDs, []int64{1069479340, 1069479399}) {
		t.Fatalf("unresolved = %+v", unresolved)
	}

	// A bucket with no visible Campfire at all is unresolved too, with an
	// empty candidate list — never a panic, never a generic failure.
	_, err = account.Recordings().Summarize(context.Background(), RecordingRef{BucketID: 4242, RecordingID: 1069479350, EventType: "chat.line.created"})
	if !errors.As(err, &unresolved) || len(unresolved.CampfireIDs) != 0 || unresolved.BucketID != 4242 {
		t.Fatalf("err = %#v, want unresolved with no candidates", err)
	}
}

func TestSummarize_ChatLineReadFailureIsNotUnresolved(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError, http.StatusUnauthorized} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			fx := newChatFixture(t, 1069479345, [2]int64{1069479340, letoLaptop}, [2]int64{1069479345, letoLaptop})
			fx.lineStatus = func(id int64) int {
				if id == 1069479340 {
					return status
				}
				return 0
			}
			account, srv, _ := newSummaryClient(t, fx.route(t))
			_, err := account.Recordings().Summarize(context.Background(), lineRef())
			apiErr, ok := errors.AsType[*Error](err)
			if !ok || apiErr.HTTPStatus != status {
				t.Fatalf("err = %v, want the %d from the first candidate", err, status)
			}
			if errors.Is(err, ErrRecordingUnresolved) {
				t.Fatal("a failed read must not read as unresolved")
			}
			if srv.count(lineRead(1069479345)) != 0 {
				t.Fatalf("the loop went on past a failed read: %v", srv.requests())
			}
		})
	}
	t.Run("project read failure", func(t *testing.T) {
		fx := newChatFixture(t, 1069479345, [2]int64{1069479345, letoLaptop})
		fx.dockStatus = http.StatusForbidden
		account, srv, _ := newSummaryClient(t, fx.route(t))
		_, err := account.Recordings().Summarize(context.Background(), lineRef())
		apiErr, ok := errors.AsType[*Error](err)
		if !ok || apiErr.HTTPStatus != http.StatusForbidden || errors.Is(err, ErrRecordingUnresolved) {
			t.Fatalf("err = %v, want the dock read's 403", err)
		}
		if len(srv.requests()) != 1 {
			t.Fatalf("discovery went on past a failed dock read: %v", srv.requests())
		}
	})
}

func TestSummarize_ChatLineListingFailurePassesThrough(t *testing.T) {
	account, srv, _ := newSummaryClient(t, func(w http.ResponseWriter, _ *http.Request, path string) bool {
		switch path {
		case projectRead:
			writeJSON(w, http.StatusNotFound, []byte(`{"error":"Record not found"}`))
		case listingRead:
			writeJSON(w, http.StatusServiceUnavailable, []byte(`{"error":"down"}`))
		default:
			return false
		}
		return true
	})
	_, err := account.Recordings().Summarize(context.Background(), lineRef())
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.HTTPStatus != http.StatusServiceUnavailable || errors.Is(err, ErrRecordingUnresolved) {
		t.Fatalf("err = %v, want the listing's 503", err)
	}
	// A failed listing caches nothing: the next call lists again.
	_, _ = account.Recordings().Summarize(context.Background(), lineRef())
	if n := srv.count(listingRead); n != 2 {
		t.Fatalf("listings = %d, want 2", n)
	}
}

func TestSummarize_ChatLineDiscoveryIncompleteIsNotUnresolved(t *testing.T) {
	t.Run("bucket over the candidate budget", func(t *testing.T) {
		pairs := make([][2]int64, 0, MaxCampfireCandidates+5)
		for i := int64(0); i < MaxCampfireCandidates+5; i++ {
			pairs = append(pairs, [2]int64{7000 + i, letoLaptop})
		}
		fx := newChatFixture(t, 7000+MaxCampfireCandidates+2, pairs...) // the line is past the budget
		account, srv, _ := newSummaryClient(t, fx.route(t))
		_, err := account.Recordings().Summarize(context.Background(), lineRef())
		if !errors.Is(err, ErrCampfireDiscoveryIncomplete) || errors.Is(err, ErrRecordingUnresolved) {
			t.Fatalf("err = %v, want incomplete, not unresolved", err)
		}
		var incomplete *CampfireDiscoveryIncompleteError
		if !errors.As(err, &incomplete) || incomplete.BucketID != letoLaptop || incomplete.RecordingID != 1069479350 {
			t.Fatalf("err = %#v", err)
		}
		if n := srv.count("/195539477/chats/"); n != MaxCampfireCandidates {
			t.Fatalf("tried %d candidates, want the budget %d", n, MaxCampfireCandidates)
		}
	})
	t.Run("listing over its cap", func(t *testing.T) {
		pairs := make([][2]int64, 0, MaxCampfireListing+1)
		for i := int64(0); i <= MaxCampfireListing; i++ {
			pairs = append(pairs, [2]int64{9000 + i, letoLocator})
		}
		fx := newChatFixture(t, 0, pairs...)
		account, srv, _ := newSummaryClient(t, fx.route(t))
		_, err := account.Recordings().Summarize(context.Background(), lineRef())
		if !errors.Is(err, ErrCampfireDiscoveryIncomplete) || errors.Is(err, ErrRecordingUnresolved) {
			t.Fatalf("err = %v, want incomplete, not unresolved", err)
		}
		// An overflowing listing is not cached: the next call lists again.
		_, _ = account.Recordings().Summarize(context.Background(), lineRef())
		if n := srv.count(listingRead); n != 2 {
			t.Fatalf("listings = %d, want 2", n)
		}
	})
}

func TestSummarize_ChatLineHonoursCancellation(t *testing.T) {
	t.Run("while waiting on another caller's listing", func(t *testing.T) {
		fx := newChatFixture(t, 1069479345, [2]int64{1069479345, letoLaptop})
		started := make(chan struct{})
		release := make(chan struct{})
		inner := fx.route(t)
		var once sync.Once
		account, _, _ := newSummaryClient(t, func(w http.ResponseWriter, r *http.Request, path string) bool {
			if path == listingRead {
				once.Do(func() { close(started) })
				<-release
			}
			return inner(w, r, path)
		})
		go func() { _, _ = account.Recordings().Summarize(context.Background(), lineRef()) }()
		<-started
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := account.Recordings().Summarize(ctx, lineRef())
			done <- err
		}()
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a waiter did not observe its cancelled context")
		}
		close(release)
	})
	t.Run("with every source cached and no candidate to try", func(t *testing.T) {
		fx := newChatFixture(t, 0, [2]int64{1069479400, letoLocator})
		account, _, _ := newSummaryClient(t, fx.route(t))
		if _, err := account.Recordings().Summarize(context.Background(), lineRef()); !errors.Is(err, ErrRecordingUnresolved) {
			t.Fatalf("priming: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := account.Recordings().Summarize(ctx, lineRef())
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrRecordingUnresolved) {
			t.Fatalf("err = %v, want context.Canceled, not unresolved", err)
		}
	})
}

func TestSummarize_ChatLineListingIsSingleFlight(t *testing.T) {
	fx := newChatFixture(t, 1069479345, [2]int64{1069479345, letoLaptop})
	release := make(chan struct{})
	var listCalls atomic.Int32
	inner := fx.route(t)
	account, srv, _ := newSummaryClient(t, func(w http.ResponseWriter, r *http.Request, path string) bool {
		if path == listingRead {
			listCalls.Add(1)
			<-release // hold every listing until all callers are waiting
		}
		return inner(w, r, path)
	})

	const callers = 8
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = account.Recordings().Summarize(context.Background(), lineRef())
		}(i)
	}
	// Let the goroutines pile up behind the one listing, then release it.
	deadline := time.Now().Add(5 * time.Second)
	for listCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d: %v", i, err)
		}
	}
	if n := listCalls.Load(); n != 1 {
		t.Fatalf("listings = %d for %d concurrent callers, want 1", n, callers)
	}
	if n := srv.count(projectRead); n != 1 {
		t.Fatalf("project reads = %d for %d concurrent callers, want 1", n, callers)
	}
}

// TestMentions_RoundTripThroughCommentAndSummary is the card's round trip: an
// id goes in through CreateWithMentions, the sgid BC3 serves for that person
// is what gets written, and Summarize on the resulting comment reads the same
// id back out. The server is a mock composed from the shared fixtures — the
// person read's sgid, and BC3's documented rendering of a mention on read —
// so this proves the composition, not BC3 itself; the sgid decoder's own
// contract with Rails is pinned separately, on payloads Ruby produced.
func TestMentions_RoundTripThroughCommentAndSummary(t *testing.T) {
	person := loadSummaryFixture(t, "people/get.json") // id 1049715915, older sgid layout
	var posted atomic.Pointer[string]
	var peopleReads atomic.Int32
	commentTemplate := loadSummaryFixture(t, "comments/get.json")
	account, _, _ := newSummaryClient(t, func(w http.ResponseWriter, r *http.Request, path string) bool {
		switch {
		case path == "/195539477/people/1049715915":
			peopleReads.Add(1)
			writeJSON(w, http.StatusOK, person)
		case path == "/195539477/recordings/1069479351/comments" && r.Method == http.MethodPost:
			body := decodeRequestBody(t, r)
			content, _ := body["content"].(string)
			posted.Store(&content)
			// Echo the posted content back as the created comment.
			var c map[string]any
			_ = json.Unmarshal(commentTemplate, &c)
			c["content"] = content
			writeJSON(w, http.StatusCreated, mustJSON(c))
		case path == "/195539477/comments/1069479361":
			// On read, BC3 expands the bare tag into the rendered mention:
			// content-type, avatar figure, figcaption. Simulate that, so the
			// summary reads what a real response carries, not the echo.
			var c map[string]any
			_ = json.Unmarshal(commentTemplate, &c)
			if p := posted.Load(); p != nil {
				bare := `<bc-attachment sgid="` + fixtureSGIDPerson + `"></bc-attachment>`
				c["content"] = strings.ReplaceAll(*p, bare, renderedMention(fixtureSGIDPerson, 1049715915, "Victor Cooper"))
			}
			writeJSON(w, http.StatusOK, mustJSON(c))
		default:
			return false
		}
		return true
	})

	created, err := account.Comments().CreateWithMentions(context.Background(), 1069479351, "<div>On it.</div>", []int64{1049715915, 1049715915})
	if err != nil {
		t.Fatalf("CreateWithMentions: %v", err)
	}
	if peopleReads.Load() != 1 {
		t.Fatalf("people reads = %d, want 1 (deduped)", peopleReads.Load())
	}
	wantContent := `<div><bc-attachment sgid="` + fixtureSGIDPerson + `"></bc-attachment> On it.</div>`
	if p := posted.Load(); p == nil || *p != wantContent {
		t.Fatalf("posted %q, want %q", strOrNil(p), wantContent)
	}
	if ids := MentionedPersonIDs(created.Content); !reflect.DeepEqual(ids, []int64{1049715915}) {
		t.Fatalf("created comment mentions %v", ids)
	}

	sum, err := account.Recordings().Summarize(context.Background(), RecordingRef{BucketID: letoLaptop, RecordingID: 1069479361, EventType: "comment.created"})
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if !reflect.DeepEqual(sum.MentionedPersonIDs, []int64{1049715915}) {
		t.Fatalf("summary mentions %v, want [1049715915]", sum.MentionedPersonIDs)
	}
	if sum.Content == wantContent || !strings.Contains(sum.Content, `content-type="application/vnd.basecamp.mention"`) {
		t.Fatalf("the summary did not read the rendered form: %q", sum.Content)
	}
	// Expanding again against the rendered form adds nothing.
	again, err := account.Comments().ExpandMentions(context.Background(), sum.Content, []int64{1049715915})
	if err != nil || again != sum.Content || peopleReads.Load() != 1 {
		t.Fatalf("re-expansion changed the content or read people again: %v, reads %d", err, peopleReads.Load())
	}
}

func TestExpandMentions(t *testing.T) {
	person := loadSummaryFixture(t, "people/get.json")
	var peopleReads atomic.Int32
	account, _, _ := newSummaryClient(t, func(w http.ResponseWriter, _ *http.Request, path string) bool {
		switch path {
		case "/195539477/people/1049715915":
			peopleReads.Add(1)
			writeJSON(w, http.StatusOK, person)
		case "/195539477/people/404404":
			writeJSON(w, http.StatusNotFound, []byte(`{"error":"Record not found"}`))
		default:
			return false
		}
		return true
	})
	comments := account.Comments()

	t.Run("no ids is a no-op with no reads", func(t *testing.T) {
		got, err := comments.ExpandMentions(context.Background(), "<p>hi</p>", nil)
		if err != nil || got != "<p>hi</p>" || peopleReads.Load() != 0 {
			t.Fatalf("got %q, %v, reads %d", got, err, peopleReads.Load())
		}
	})
	t.Run("an id the content already mentions costs no read", func(t *testing.T) {
		content := "<p>" + renderedMention(fixtureSGIDPerson, 1049715915, "Victor Cooper") + " hi</p>"
		got, err := comments.ExpandMentions(context.Background(), content, []int64{1049715915})
		if err != nil || got != content || peopleReads.Load() != 0 {
			t.Fatalf("got %q, %v, reads %d", got, err, peopleReads.Load())
		}
	})
	t.Run("a person the account cannot resolve fails the expansion", func(t *testing.T) {
		_, err := comments.ExpandMentions(context.Background(), "<p>hi</p>", []int64{404404})
		apiErr, ok := errors.AsType[*Error](err)
		if !ok || apiErr.Code != CodeNotFound {
			t.Fatalf("err = %v, want the person read's not_found", err)
		}
	})
	t.Run("an invalid id is a usage error before any read", func(t *testing.T) {
		before := peopleReads.Load()
		_, err := comments.ExpandMentions(context.Background(), "<p>hi</p>", []int64{0})
		apiErr, ok := errors.AsType[*Error](err)
		if !ok || apiErr.Code != CodeUsage || peopleReads.Load() != before {
			t.Fatalf("err = %v, reads %d", err, peopleReads.Load()-before)
		}
	})
	t.Run("CreateWithMentions posts nothing when a lookup fails", func(t *testing.T) {
		_, err := comments.CreateWithMentions(context.Background(), 1, "<p>hi</p>", []int64{404404})
		if err == nil {
			t.Fatal("expected an error")
		}
		if _, err := comments.CreateWithMentions(context.Background(), 1, "", []int64{1049715915}); err == nil {
			t.Fatal("expected a usage error for empty content")
		}
	})
}

func strOrNil(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func TestSummarize_ChatLineDockRefreshIsNotBlockedByTheListing(t *testing.T) {
	// Two buckets age their docks independently while sharing one listing.
	// Bucket B's dock was cached without a chat tool; the project has one now.
	// The listing has since expired and is down. B's line must be found by the
	// dock's one re-read, with no listing fetch attempted first.
	const bucketA, bucketB = letoLaptop, letoLocator
	fx := newChatFixture(t, 1069479345, [2]int64{1069479345, bucketA}, [2]int64{1069479400, bucketB})
	fx.setDock(bucketA, 0)
	fx.setDock(bucketB, 0)
	account, srv, clock := newSummaryClient(t, fx.route(t))
	listings := func() int { return srv.count(listingRead) - srv.count(listingRead+"/") }

	// T: bucket A primes its dock and the shared listing.
	if _, err := account.Recordings().Summarize(context.Background(), lineRef()); err != nil {
		t.Fatalf("priming A: %v", err)
	}
	// T+9:30: bucket B primes its dock (empty); the listing is still cached.
	*clock = clock.Add(CampfireIndexTTL - 30*time.Second)
	fx.foundUnder.Store(1069479400)
	refB := RecordingRef{BucketID: bucketB, RecordingID: 1069479350, EventType: "chat.line.created"}
	if _, err := account.Recordings().Summarize(context.Background(), refB); err != nil {
		t.Fatalf("priming B: %v", err)
	}
	if listings() != 1 {
		t.Fatalf("listings = %d while priming, want 1", listings())
	}
	// T+10:30: the listing has expired; B's dock is a minute old — past the
	// floor, inside the TTL. The project gained a Campfire; the listing is down.
	*clock = clock.Add(time.Minute)
	fx.setDock(bucketB, 1069479777)
	fx.foundUnder.Store(1069479777)
	inner := fx.route(t)
	srv.mu.Lock()
	srv.route = func(w http.ResponseWriter, r *http.Request, path string) bool {
		if path == listingRead {
			writeJSON(w, http.StatusServiceUnavailable, []byte(`{"error":"down"}`))
			return true
		}
		return inner(w, r, path)
	}
	srv.mu.Unlock()
	got, err := account.Recordings().Summarize(context.Background(), refB)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got.CampfireID != 1069479777 {
		t.Fatalf("CampfireID = %d, want the dock's new Campfire", got.CampfireID)
	}
	if listings() != 1 {
		t.Fatalf("listings = %d: the expired listing was fetched although the dock refresh found the line", listings())
	}
}

func TestTTLCache_LoaderPanicReleasesTheKey(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[string, int](func() time.Time { return now }, time.Minute, time.Second, campfireIndexMaxItems)
	started := make(chan struct{})
	release := make(chan struct{})

	// The loader panics while a second caller waits on it.
	loaderDone := make(chan any, 1)
	go func() {
		defer func() { loaderDone <- recover() }()
		_, _ = cache.get(context.Background(), "k", false, func(context.Context) (int, error) {
			close(started)
			<-release
			panic("hook exploded")
		})
	}()
	<-started
	waiting := make(chan struct{})
	cache.onWait = func() { close(waiting) }
	waiterDone := make(chan error, 1)
	go func() {
		_, err := cache.get(context.Background(), "k", false, func(context.Context) (int, error) { return 1, nil })
		waiterDone <- err
	}()
	<-waiting // the waiter is on the in-flight path before the loader is released
	cache.onWait = nil
	close(release)
	if r := <-loaderDone; r == nil {
		t.Fatal("the panic did not propagate to the loading caller")
	}
	select {
	case err := <-waiterDone:
		if err == nil || !strings.Contains(err.Error(), "panicked") {
			t.Fatalf("waiter err = %v, want the loader's panic as an error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiter hung on a key whose loader panicked")
	}
	// The key is free: the next load runs and caches.
	hit, err := cache.get(context.Background(), "k", false, func(context.Context) (int, error) { return 7, nil })
	if err != nil || hit.value != 7 || hit.cached {
		t.Fatalf("after the panic: %+v err=%v", hit, err)
	}
	hit, err = cache.get(context.Background(), "k", false, func(context.Context) (int, error) { return 8, nil })
	if err != nil || hit.value != 7 || !hit.cached {
		t.Fatalf("the recovered load did not cache: %+v err=%v", hit, err)
	}
}

func TestTTLCache_WaiterGetsTheLoadItWaitedOnAsFresh(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[string, int](func() time.Time { return now }, time.Minute, time.Second, campfireIndexMaxItems)
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = cache.get(context.Background(), "k", false, func(context.Context) (int, error) {
			close(started)
			<-release
			return 42, nil
		})
	}()
	<-started
	type outcome struct {
		v      int
		cached bool
		err    error
	}
	waiting := make(chan struct{})
	cache.onWait = func() { close(waiting) }
	done := make(chan outcome, 1)
	go func() {
		hit, err := cache.get(context.Background(), "k", false, func(context.Context) (int, error) { return -1, nil })
		done <- outcome{hit.value, hit.cached, err}
	}()
	<-waiting
	close(release)
	got := <-done
	if got.err != nil || got.v != 42 || got.cached {
		t.Fatalf("waiter got %+v, want the awaited load's value reported fresh (cached=false)", got)
	}
}

func TestSummarize_ChatLineMentionsOnlyOnRichTextSubtypes(t *testing.T) {
	// The same content — a rendered mention — served as a code line, a plain
	// text line, a rich text line and an integration line. Only the two rich
	// text subtypes can carry a mention BC3 read as markup.
	mention := renderedMention(fixtureSGIDPerson, 1049715915, "Victor Cooper")
	for _, tc := range []struct {
		lineType string
		want     []int64
	}{
		{"Chat::Lines::Code", []int64{}},
		{"Chat::Lines::Text", []int64{}},
		{"Chat::Lines::RichText", []int64{1049715915}},
		{"Chat::Lines::Integration", []int64{1049715915}},
	} {
		t.Run(tc.lineType, func(t *testing.T) {
			fx := newChatFixture(t, 1069479341, [2]int64{1069479341, letoLaptop})
			fx.setDock(letoLaptop, 1069479341)
			inner := fx.route(t)
			account, _, _ := newSummaryClient(t, func(w http.ResponseWriter, r *http.Request, path string) bool {
				if path != lineRead(1069479341) {
					return inner(w, r, path)
				}
				var l map[string]any
				_ = json.Unmarshal(loadSummaryFixture(t, "campfires/line_get.json"), &l)
				l["type"] = tc.lineType
				l["content"] = "<div>" + mention + " look</div>"
				writeJSON(w, http.StatusOK, mustJSON(l))
				return true
			})
			got, err := account.Recordings().Summarize(context.Background(), lineRef())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.MentionedPersonIDs, tc.want) {
				t.Fatalf("MentionedPersonIDs = %v, want %v", got.MentionedPersonIDs, tc.want)
			}
		})
	}
}

func TestSummarize_ChatLineTakesTheListingAnotherCallerPopulated(t *testing.T) {
	// Caller B's listing peek misses (the listing has expired) and B goes to
	// re-read its dock. While that project read is in flight, caller A fetches
	// the listing, which now holds Campfire X in B's bucket. B's own listing
	// consultation then finds a fresh entry another caller populated; X answers
	// 404. B must report X as tried and current — not stale — because the
	// snapshot it concluded on is the one it was handed.
	const bucketA, bucketB = letoLaptop, letoLocator
	const campfireX, campfireY = int64(1069479888), int64(1069479777)
	fx := newChatFixture(t, campfireY, [2]int64{1069479345, bucketA})
	fx.setDock(bucketA, 0)
	fx.setDock(bucketB, campfireY)
	account, srv, clock := newSummaryClient(t, fx.route(t))
	listings := func() int { return srv.count(listingRead) - srv.count(listingRead+"/") }
	refA := lineRef()
	refB := RecordingRef{BucketID: bucketB, RecordingID: 1069479350, EventType: "chat.line.created"}

	// T: A primes its dock and the listing (unresolved: nothing in A's bucket
	// holds the line). T+9:30: B primes its dock by finding the line under Y,
	// so nothing refreshes the listing. T+10:30: the listing has expired; B's
	// dock is a minute old, past the floor.
	if _, err := account.Recordings().Summarize(context.Background(), refA); !errors.Is(err, ErrRecordingUnresolved) {
		t.Fatalf("priming A: %v", err)
	}
	*clock = clock.Add(CampfireIndexTTL - 30*time.Second)
	if _, err := account.Recordings().Summarize(context.Background(), refB); err != nil {
		t.Fatalf("priming B: %v", err)
	}
	if listings() != 1 {
		t.Fatalf("listings = %d after priming, want 1 (still the T listing)", listings())
	}
	*clock = clock.Add(time.Minute)
	fx.foundUnder.Store(0) // the line is now under nobody
	fx.setListing(t, [2]int64{1069479345, bucketA}, [2]int64{campfireX, bucketB})

	// B's dock refresh — its first project read from here on — is held until
	// A has fetched the listing.
	refreshStarted := make(chan struct{})
	aDone := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(aDone) }) }
	t.Cleanup(release) // a failed assertion must not leave the held handler blocking server shutdown
	var once sync.Once
	inner := fx.route(t)
	srv.mu.Lock()
	srv.route = func(w http.ResponseWriter, r *http.Request, path string) bool {
		if path == "/195539477/projects/"+itoa64(bucketB) {
			once.Do(func() { close(refreshStarted) })
			<-aDone
		}
		return inner(w, r, path)
	}
	srv.mu.Unlock()

	bDone := make(chan error, 1)
	go func() {
		_, err := account.Recordings().Summarize(context.Background(), refB)
		bDone <- err
	}()
	select {
	case <-refreshStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("B never reached its dock refresh; the interleaving under test did not happen")
	}
	if _, err := account.Recordings().Summarize(context.Background(), refA); !errors.Is(err, ErrRecordingUnresolved) {
		t.Fatalf("A: %v", err)
	}
	if listings() != 2 {
		t.Fatalf("listings = %d after A, want 2 (A fetched the expired listing)", listings())
	}
	release()

	var unresolved *UnresolvedRecordingError
	select {
	case err := <-bDone:
		if !errors.As(err, &unresolved) {
			t.Fatalf("B: %v, want unresolved", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("B did not finish")
	}
	if listings() != 2 {
		t.Fatalf("listings = %d, want 2: B must have taken the listing A populated rather than fetch its own", listings())
	}
	if !reflect.DeepEqual(unresolved.CampfireIDs, []int64{campfireY, campfireX}) {
		t.Fatalf("B tried %v, want [%d %d]: its dock's Y, then X from the listing A populated", unresolved.CampfireIDs, campfireY, campfireX)
	}
	if !unresolved.Refreshed {
		t.Fatal("B re-read its dock; Refreshed should say so")
	}
	if len(unresolved.StaleCampfireIDs) != 0 {
		t.Fatalf("B reported %v stale although the sources it was handed hold them", unresolved.StaleCampfireIDs)
	}
}

func TestTTLCache_WaiterOutlivesTheLoadersCancellation(t *testing.T) {
	// Caller 1 owns the load and is cancelled mid-way; caller 2, whose
	// context is live, was waiting on it. Caller 2 must not be handed caller
	// 1's cancellation: it re-acquires the key and loads for itself. Without
	// the re-acquire, caller 2 returns context.Canceled and the second load
	// never happens.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[string, int](func() time.Time { return now }, time.Minute, time.Second, campfireIndexMaxItems)
	var loads atomic.Int32
	started := make(chan struct{})
	load := func(ctx context.Context) (int, error) {
		n := loads.Add(1)
		if n == 1 {
			close(started)
			<-ctx.Done() // the first load respects its caller's cancellation
			return 0, ctx.Err()
		}
		return 42, nil
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := cache.get(ctx1, "k", false, load)
		firstDone <- err
	}()
	<-started

	waiting := make(chan struct{})
	cache.onWait = func() { close(waiting) }
	type outcome struct {
		hit ttlHit[int]
		err error
	}
	secondDone := make(chan outcome, 1)
	go func() {
		hit, err := cache.get(context.Background(), "k", false, load)
		secondDone <- outcome{hit, err}
	}()
	<-waiting
	cache.onWait = nil
	cancel1()

	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("caller 1: %v, want its own cancellation", err)
	}
	select {
	case got := <-secondDone:
		if got.err != nil || got.hit.value != 42 || got.hit.cached {
			t.Fatalf("caller 2 got %+v, %v; want its own successful load", got.hit, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("caller 2 never finished")
	}
	if loads.Load() != 2 {
		t.Fatalf("loads = %d, want 2: the cancelled one and caller 2's own", loads.Load())
	}
	// A waiter whose own context is dead keeps its own error, not a retry.
	ctx3, cancel3 := context.WithCancel(context.Background())
	cancel3()
	if _, err := cache.get(ctx3, "other", false, load); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled caller got %v, want context.Canceled", err)
	}
	// And a load's own (non-cancellation) error is shared with its waiters.
	boom := errors.New("boom")
	if _, err := cache.get(context.Background(), "boom", false, func(context.Context) (int, error) { return 0, boom }); !errors.Is(err, boom) {
		t.Fatalf("got %v, want the load's own error", err)
	}
}

func TestTTLCache_TransportTimeoutIsSharedNotRetried(t *testing.T) {
	// A load that fails with an error satisfying errors.Is(err,
	// context.DeadlineExceeded) — an http.Client.Timeout does — while the
	// loading caller's context is still live is the load's own failure. Every
	// waiter must share it; none may re-run the load, or N concurrent callers
	// become N sequential requests past any configured retry budget.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[string, int](func() time.Time { return now }, time.Minute, time.Second, campfireIndexMaxItems)
	timeout := fmt.Errorf("Get \"https://example.invalid/chats.json\": %w (Client.Timeout exceeded while awaiting headers)", context.DeadlineExceeded)
	var loads atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	load := func(context.Context) (int, error) {
		if loads.Add(1) == 1 {
			close(started)
			<-release
		}
		return 0, timeout
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := cache.get(context.Background(), "k", false, load)
		firstDone <- err
	}()
	<-started
	const waiters = 5
	waiting := make(chan struct{}, waiters)
	cache.onWait = func() { waiting <- struct{}{} }
	results := make(chan error, waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			_, err := cache.get(context.Background(), "k", false, load)
			results <- err
		}()
	}
	for i := 0; i < waiters; i++ {
		<-waiting
	}
	cache.onWait = nil
	close(release)
	if err := <-firstDone; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("loader: %v", err)
	}
	for i := 0; i < waiters; i++ {
		select {
		case err := <-results:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("waiter: %v, want the shared timeout", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a waiter never finished")
		}
	}
	if n := loads.Load(); n != 1 {
		t.Fatalf("loads = %d, want 1: the timeout is shared, not re-run per waiter", n)
	}
}

func TestTTLCache_ReacquireIsBounded(t *testing.T) {
	// Waiter W waits on owner 1, who is cancelled. W re-acquires — but a
	// fresh caller X takes the slot first and is cancelled too. W, having
	// already re-acquired once, returns that second owner-attributed failure
	// instead of loading a third time: a run of cancelled owners must not
	// become a queue of sequential loads behind one waiter.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[string, int](func() time.Time { return now }, time.Minute, time.Second, campfireIndexMaxItems)
	var loads atomic.Int32
	load := func(ctx context.Context) (int, error) {
		loads.Add(1)
		<-ctx.Done() // every owner in this test is cancelled mid-load
		return 0, ctx.Err()
	}
	start := func() (context.CancelFunc, chan error) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := cache.get(ctx, "k", false, load)
			done <- err
		}()
		return cancel, done
	}
	cancel1, owner1 := start()
	for loads.Load() < 1 {
		time.Sleep(time.Millisecond)
	}
	waiting := make(chan struct{}, 4)
	cache.onWait = func() { waiting <- struct{}{} }
	var xCancel context.CancelFunc
	var xDone chan error
	xStarted := make(chan struct{})
	var xOnce sync.Once
	cache.onReacquire = func() {
		// Between W's decision to re-acquire and its next lock, X arrives,
		// takes the slot, and is cancelled once W is waiting on it. Only
		// once: a W that re-acquires a second time (the bound removed) is
		// left to load for itself under a context nobody cancels, and the
		// test fails on W never finishing.
		xOnce.Do(func() {
			xCancel, xDone = start()
			for loads.Load() < 2 {
				time.Sleep(time.Millisecond)
			}
			close(xStarted)
		})
	}
	wDone := make(chan error, 1)
	go func() {
		_, err := cache.get(context.Background(), "k", false, load)
		wDone <- err
	}()
	<-waiting // W waits on owner 1
	cancel1()
	if err := <-owner1; !errors.Is(err, context.Canceled) {
		t.Fatalf("owner 1: %v", err)
	}
	<-xStarted
	<-waiting // W waits on X
	xCancel()
	if err := <-xDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("X: %v", err)
	}
	select {
	case err := <-wDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("W: %v, want the second owner's cancellation returned, not chased", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("W never finished")
	}
	if n := loads.Load(); n != 2 {
		t.Fatalf("loads = %d, want 2 (owner 1 and X): W must not load a third time", n)
	}
}

func TestTTLCache_OwnerCancelledAfterAGenuineFailureDoesNotRetryIt(t *testing.T) {
	// The owner's load fails on its own — a 403 — and then the owner's
	// context is cancelled (a deferred cancel, an operation-end hook) before
	// the result is published. A done owner context is coincidence here, not
	// cause: the failure is the load's own and every waiter shares it, with
	// exactly one load made.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[string, int](func() time.Time { return now }, time.Minute, time.Second, campfireIndexMaxItems)
	forbidden := &Error{Code: CodeForbidden, Message: "access denied", HTTPStatus: 403}
	var loads atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	ownerCtx, cancelOwner := context.WithCancel(context.Background())
	load := func(context.Context) (int, error) {
		if loads.Add(1) == 1 {
			close(started)
			<-release
			cancelOwner() // the owner walks away as its own load fails
		}
		return 0, forbidden
	}
	ownerDone := make(chan error, 1)
	go func() {
		_, err := cache.get(ownerCtx, "k", false, load)
		ownerDone <- err
	}()
	<-started
	waiting := make(chan struct{})
	cache.onWait = func() { close(waiting) }
	waiterDone := make(chan error, 1)
	go func() {
		_, err := cache.get(context.Background(), "k", false, load)
		waiterDone <- err
	}()
	<-waiting
	cache.onWait = nil
	close(release)
	if err := <-ownerDone; !errors.Is(err, forbidden) {
		t.Fatalf("owner: %v", err)
	}
	select {
	case err := <-waiterDone:
		if !errors.Is(err, forbidden) {
			t.Fatalf("waiter: %v, want the shared 403", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiter never finished")
	}
	if n := loads.Load(); n != 1 {
		t.Fatalf("loads = %d, want 1: a genuine failure is not retried because its owner happened to be cancelled", n)
	}
}

func TestTTLCache_SweepsExpiredEntriesOnLoad(t *testing.T) {
	// A long-lived client sees many keys. Once past their TTL they are not
	// kept for the client's lifetime: the next load on any key sweeps them.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	clock := &now
	cache := newTTLCache[string, int](func() time.Time { return *clock }, time.Minute, time.Second, campfireIndexMaxItems)
	one := func(context.Context) (int, error) { return 1, nil }
	for _, k := range []string{"a", "b", "c"} {
		if _, err := cache.get(context.Background(), k, false, one); err != nil {
			t.Fatal(err)
		}
	}
	*clock = clock.Add(30 * time.Second)
	if _, err := cache.get(context.Background(), "d", false, one); err != nil { // d is fresh at +30s
		t.Fatal(err)
	}
	count := func() int {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		return len(cache.entries)
	}
	if count() != 4 {
		t.Fatalf("entries = %d before expiry, want 4", count())
	}
	*clock = clock.Add(31 * time.Second) // a, b, c are past the TTL; d is not
	if hit, err := cache.get(context.Background(), "d", false, one); err != nil || !hit.cached {
		t.Fatalf("d should still be served from cache: %+v %v", hit, err)
	}
	if count() != 4 {
		t.Fatalf("entries = %d: a cache hit does no sweeping", count())
	}
	if _, err := cache.get(context.Background(), "e", false, one); err != nil { // a load: sweeps
		t.Fatal(err)
	}
	if count() != 2 {
		t.Fatalf("entries = %d after a load, want 2 (d and e)", count())
	}
}

func TestTTLCache_BoundEvictsOldestFirst(t *testing.T) {
	// More keys than the bound, none expired: the cache never holds more than
	// the bound, and what goes is the oldest-fetched, deterministically.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	clock := &now
	const bound = 4
	cache := newTTLCache[string, int](func() time.Time { return *clock }, time.Hour, time.Second, bound)
	count := func() int {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		return len(cache.entries)
	}
	has := func(k string) bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		_, ok := cache.entries[k]
		return ok
	}
	for i, k := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		*clock = clock.Add(time.Second) // strictly increasing fetch times
		v := i
		if _, err := cache.get(context.Background(), k, false, func(context.Context) (int, error) { return v, nil }); err != nil {
			t.Fatal(err)
		}
		if n := count(); n > bound {
			t.Fatalf("entries = %d after %q, bound is %d", n, k, bound)
		}
	}
	for _, gone := range []string{"a", "b", "c"} {
		if has(gone) {
			t.Fatalf("%q survived although it was among the oldest", gone)
		}
	}
	for _, kept := range []string{"d", "e", "f", "g"} {
		if !has(kept) {
			t.Fatalf("%q was evicted although newer entries should have been kept", kept)
		}
	}
	// Overwriting a present key takes no new room and evicts nothing: past the
	// refresh floor but inside the TTL, a refresh of "g" republishes it with
	// the cache at its bound, and all four survive.
	*clock = clock.Add(2 * time.Second)
	hit, err := cache.get(context.Background(), "g", true, func(context.Context) (int, error) { return 99, nil })
	if err != nil || hit.cached || hit.value != 99 {
		t.Fatalf("refresh of g: %+v %v", hit, err)
	}
	for _, kept := range []string{"d", "e", "f", "g"} {
		if !has(kept) {
			t.Fatalf("%q was evicted by an overwrite of a present key", kept)
		}
	}
	if count() != bound {
		t.Fatalf("entries = %d after the overwrite, want %d", count(), bound)
	}
}

func TestTTLCache_EvictionTieBreaksByPublicationOrder(t *testing.T) {
	// With every entry fetched at the same instant (a frozen clock) the
	// oldest publication goes first, so the choice is total, not whatever
	// map iteration visits first.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[string, int](func() time.Time { return now }, time.Hour, time.Second, 3)
	for _, k := range []string{"first", "second", "third", "fourth"} {
		if _, err := cache.get(context.Background(), k, false, func(context.Context) (int, error) { return 0, nil }); err != nil {
			t.Fatal(err)
		}
	}
	cache.mu.Lock()
	_, firstGone := cache.entries["first"]
	_, secondKept := cache.entries["second"]
	n := len(cache.entries)
	cache.mu.Unlock()
	if firstGone || !secondKept || n != 3 {
		t.Fatalf("first present=%v second present=%v entries=%d; want the earliest publication evicted", firstGone, secondKept, n)
	}
}

func TestTTLCache_WaiterKeepsItsValueAcrossEviction(t *testing.T) {
	// A waiter reads the load it waited on off the load record itself, so
	// even when the bound evicts that entry between its publication and the
	// waiter's read, the waiter still gets the value. The schedule is forced:
	// the waiter is held just after it wakes, "k" is confirmed published,
	// another key is published under a bound of one (evicting "k"), "k" is
	// confirmed gone, and only then is the waiter released.
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	cache := newTTLCache[string, int](func() time.Time { return now }, time.Hour, time.Second, 1)
	has := func(k string) bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		_, ok := cache.entries[k]
		return ok
	}
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = cache.get(context.Background(), "k", false, func(context.Context) (int, error) {
			close(started)
			<-release
			return 42, nil
		})
	}()
	<-started
	waiting := make(chan struct{})
	woken := make(chan struct{})
	proceed := make(chan struct{})
	cache.onWait = func() { close(waiting) }
	cache.onWoken = func() { close(woken); <-proceed }
	done := make(chan ttlHit[int], 1)
	go func() {
		hit, _ := cache.get(context.Background(), "k", false, func(context.Context) (int, error) { return -1, nil })
		done <- hit
	}()
	<-waiting
	cache.onWait = nil
	close(release)
	<-woken // "k" is published; the waiter is held before its read
	cache.onWoken = nil
	if !has("k") {
		t.Fatal("k should be in the cache right after publication")
	}
	if _, err := cache.get(context.Background(), "other", false, func(context.Context) (int, error) { return 7, nil }); err != nil {
		t.Fatal(err)
	}
	if has("k") {
		t.Fatal("the bound of one should have evicted k")
	}
	close(proceed)
	hit := <-done
	if hit.value != 42 || hit.cached {
		t.Fatalf("waiter got %+v, want the awaited load's value", hit)
	}
}
