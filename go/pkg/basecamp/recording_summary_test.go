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

func (s *summaryServer) setRoute(route func(w http.ResponseWriter, r *http.Request, path string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.route = route
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
	client.campfires().now = func() time.Time { return *clock }
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

// chatServer serves an account-wide Campfire listing and a line under one
// Campfire; every other Campfire answers 404 for the line.
func chatServer(t *testing.T, listing *atomic.Pointer[[]byte], foundUnder int64, lineStatus func(campfireID int64) int) func(w http.ResponseWriter, r *http.Request, path string) bool {
	line := loadSummaryFixture(t, "campfires/line_get.json")
	return func(w http.ResponseWriter, _ *http.Request, path string) bool {
		if path == "/195539477/chats" {
			writeJSON(w, http.StatusOK, *listing.Load())
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
		var id int64
		for _, c := range campfire {
			id = id*10 + int64(c-'0')
		}
		if lineStatus != nil {
			if status := lineStatus(id); status != 0 {
				writeJSON(w, status, []byte(`{"error":"nope"}`))
				return true
			}
		}
		if id == foundUnder {
			writeJSON(w, http.StatusOK, line)
			return true
		}
		writeJSON(w, http.StatusNotFound, []byte(`{"error":"Record not found"}`))
		return true
	}
}

func lineRef() RecordingRef {
	return RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, EventType: "chat.line.created"}
}

func TestSummarize_ChatLineFoundUnderSecondCampfire(t *testing.T) {
	var listing atomic.Pointer[[]byte]
	// Two Campfires in the line's bucket (a ping and the project's), one in
	// another project that must never be tried.
	body := campfireListJSON(t, [2]int64{1069479400, letoLocator}, [2]int64{1069479340, letoLaptop}, [2]int64{1069479345, letoLaptop})
	listing.Store(&body)
	account, srv, _ := newSummaryClient(t, chatServer(t, &listing, 1069479345, nil))

	got, err := account.Recordings().Summarize(context.Background(), lineRef())
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	want := []string{
		"/195539477/chats",
		"/195539477/chats/1069479340/lines/1069479350",
		"/195539477/chats/1069479345/lines/1069479350",
	}
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

	// The same by recording type, each Chat::Lines subtype.
	for _, typ := range []string{"Chat::Lines::Text", "Chat::Lines::RichText", "Chat::Lines::Code", "Chat::Lines::Upload", "Chat::Lines::Integration"} {
		if _, err := account.Recordings().Summarize(context.Background(), RecordingRef{BucketID: letoLaptop, RecordingID: 1069479350, RecordingType: typ}); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
	if n := srv.count("/195539477/chats/"); n != 2+5*2 {
		t.Errorf("line reads = %d, want 2 for the first call and 2 per typed call", n)
	}
	if n := srv.count("/195539477/chats"); n-srv.count("/195539477/chats/") != 1 {
		t.Errorf("the Campfire listing was fetched %d times within the TTL, want 1", n-srv.count("/195539477/chats/"))
	}
}

func TestSummarize_ChatLineListingIsCachedTenMinutes(t *testing.T) {
	var listing atomic.Pointer[[]byte]
	body := campfireListJSON(t, [2]int64{1069479345, letoLaptop})
	listing.Store(&body)
	account, srv, clock := newSummaryClient(t, chatServer(t, &listing, 1069479345, nil))

	listings := func() int { return srv.count("/195539477/chats") - srv.count("/195539477/chats/") }
	for i := 0; i < 3; i++ {
		if _, err := account.Recordings().Summarize(context.Background(), lineRef()); err != nil {
			t.Fatal(err)
		}
	}
	if listings() != 1 {
		t.Fatalf("listings = %d after three lookups, want 1", listings())
	}
	*clock = clock.Add(CampfireIndexTTL - time.Second)
	if _, err := account.Recordings().Summarize(context.Background(), lineRef()); err != nil {
		t.Fatal(err)
	}
	if listings() != 1 {
		t.Fatalf("listings = %d just inside the TTL, want 1", listings())
	}
	*clock = clock.Add(2 * time.Second)
	if _, err := account.Recordings().Summarize(context.Background(), lineRef()); err != nil {
		t.Fatal(err)
	}
	if listings() != 2 {
		t.Fatalf("listings = %d past the TTL, want 2", listings())
	}

	// A second AccountClient from the same Client shares the index.
	other := account.parent.ForAccount(summaryAccount)
	if _, err := other.Recordings().Summarize(context.Background(), lineRef()); err != nil {
		t.Fatal(err)
	}
	if listings() != 2 {
		t.Fatalf("listings = %d from a sibling AccountClient, want 2 (shared index)", listings())
	}
}

func TestSummarize_ChatLineUnresolvedIsDistinct(t *testing.T) {
	var listing atomic.Pointer[[]byte]
	body := campfireListJSON(t, [2]int64{1069479340, letoLaptop}, [2]int64{1069479345, letoLaptop})
	listing.Store(&body)
	account, srv, clock := newSummaryClient(t, chatServer(t, &listing, 0, nil)) // found under none
	listings := func() int { return srv.count("/195539477/chats") - srv.count("/195539477/chats/") }

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
	if apiErr, ok := errors.AsType[*Error](err); ok {
		t.Fatalf("unresolved surfaced as an API error: %+v", apiErr)
	}
	// The listing was fresh, so the miss did not re-list.
	if listings() != 1 {
		t.Fatalf("listings = %d on a fresh index, want 1", listings())
	}

	// Once the listing is older than the refresh floor, a miss re-lists once
	// and tries only what is new — here, a Campfire created after the cache
	// filled, which is where the line was all along.
	*clock = clock.Add(campfireIndexMinRefresh)
	body2 := campfireListJSON(t, [2]int64{1069479340, letoLaptop}, [2]int64{1069479345, letoLaptop}, [2]int64{1069479399, letoLaptop})
	listing.Store(&body2)
	srv.setRoute(chatServer(t, &listing, 1069479399, nil))
	got, err := account.Recordings().Summarize(context.Background(), lineRef())
	if err != nil {
		t.Fatalf("after refresh: %v", err)
	}
	if got.CampfireID != 1069479399 {
		t.Fatalf("CampfireID = %d, want the newly listed Campfire", got.CampfireID)
	}
	if listings() != 2 {
		t.Fatalf("listings = %d, want 2 (one refresh on miss)", listings())
	}
	reqs := srv.requests()
	tail := reqs[len(reqs)-4:]
	want := []string{
		"/195539477/chats/1069479340/lines/1069479350",
		"/195539477/chats/1069479345/lines/1069479350",
		"/195539477/chats",
		"/195539477/chats/1069479399/lines/1069479350",
	}
	if !reflect.DeepEqual(tail, want) {
		t.Fatalf("requests = %v, want tail %v", reqs, want)
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
			var listing atomic.Pointer[[]byte]
			body := campfireListJSON(t, [2]int64{1069479340, letoLaptop}, [2]int64{1069479345, letoLaptop})
			listing.Store(&body)
			failing := func(id int64) int {
				if id == 1069479340 {
					return status
				}
				return 0
			}
			account, srv, _ := newSummaryClient(t, chatServer(t, &listing, 1069479345, failing))
			_, err := account.Recordings().Summarize(context.Background(), lineRef())
			apiErr, ok := errors.AsType[*Error](err)
			if !ok || apiErr.HTTPStatus != status {
				t.Fatalf("err = %v, want the %d from the first candidate", err, status)
			}
			if errors.Is(err, ErrRecordingUnresolved) {
				t.Fatal("a failed read must not read as unresolved")
			}
			if srv.count("/195539477/chats/1069479345/") != 0 {
				t.Fatalf("the loop went on past a failed read: %v", srv.requests())
			}
		})
	}
}

func TestSummarize_ChatLineListingFailurePassesThrough(t *testing.T) {
	account, srv, _ := newSummaryClient(t, func(w http.ResponseWriter, _ *http.Request, path string) bool {
		if path != "/195539477/chats" {
			return false
		}
		writeJSON(w, http.StatusServiceUnavailable, []byte(`{"error":"down"}`))
		return true
	})
	_, err := account.Recordings().Summarize(context.Background(), lineRef())
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.HTTPStatus != http.StatusServiceUnavailable || errors.Is(err, ErrRecordingUnresolved) {
		t.Fatalf("err = %v, want the listing's 503", err)
	}
	// A failed listing caches nothing: the next call lists again.
	_, _ = account.Recordings().Summarize(context.Background(), lineRef())
	if n := srv.count("/195539477/chats"); n != 2 {
		t.Fatalf("listings = %d, want 2", n)
	}
}

func TestSummarize_ChatLineListingIsSingleFlight(t *testing.T) {
	var listing atomic.Pointer[[]byte]
	body := campfireListJSON(t, [2]int64{1069479345, letoLaptop})
	listing.Store(&body)
	release := make(chan struct{})
	var listCalls atomic.Int32
	inner := chatServer(t, &listing, 1069479345, nil)
	account, _, _ := newSummaryClient(t, func(w http.ResponseWriter, r *http.Request, path string) bool {
		if path == "/195539477/chats" {
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
}

func TestSummarize_ChatLineHonoursCancellationWhileWaiting(t *testing.T) {
	var listing atomic.Pointer[[]byte]
	body := campfireListJSON(t, [2]int64{1069479345, letoLaptop})
	listing.Store(&body)
	started := make(chan struct{})
	release := make(chan struct{})
	inner := chatServer(t, &listing, 1069479345, nil)
	var once sync.Once
	account, _, _ := newSummaryClient(t, func(w http.ResponseWriter, r *http.Request, path string) bool {
		if path == "/195539477/chats" {
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
}

func TestSummarize_ChatLineCandidatesAreBounded(t *testing.T) {
	pairs := make([][2]int64, 0, MaxCampfireCandidates+5)
	for i := int64(0); i < MaxCampfireCandidates+5; i++ {
		pairs = append(pairs, [2]int64{7000 + i, letoLaptop})
	}
	var listing atomic.Pointer[[]byte]
	body := campfireListJSON(t, pairs...)
	listing.Store(&body)
	account, srv, _ := newSummaryClient(t, chatServer(t, &listing, 0, nil))
	_, err := account.Recordings().Summarize(context.Background(), lineRef())
	var unresolved *UnresolvedRecordingError
	if !errors.As(err, &unresolved) {
		t.Fatalf("err = %v", err)
	}
	if len(unresolved.CampfireIDs) != MaxCampfireCandidates || srv.count("/195539477/chats/") != MaxCampfireCandidates {
		t.Fatalf("tried %d candidates, want the bound %d", len(unresolved.CampfireIDs), MaxCampfireCandidates)
	}
}

// TestMentions_RoundTripThroughCommentAndSummary is the card's round trip: an
// id goes in through CreateWithMentions, the sgid BC3 serves for that person
// is what gets written, and Summarize on the resulting comment reads the same
// id back out.
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
			var c map[string]any
			_ = json.Unmarshal(commentTemplate, &c)
			if p := posted.Load(); p != nil {
				c["content"] = *p
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
	if sum.Content != wantContent {
		t.Fatalf("summary content %q", sum.Content)
	}

	// The same round trip with the mention BC3 would render on read — the
	// avatar figure and content-type added — resolves to the same id.
	rendered := renderedMention(fixtureSGIDPerson, 1049715915, "Victor Cooper")
	if ids := MentionedPersonIDs("<div>" + rendered + " On it.</div>"); !reflect.DeepEqual(ids, []int64{1049715915}) {
		t.Fatalf("rendered mention reads back %v", ids)
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
