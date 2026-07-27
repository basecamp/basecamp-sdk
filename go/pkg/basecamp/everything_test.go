package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func everythingTestClient(t *testing.T, handler http.HandlerFunc) (*EverythingService, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	return client.ForAccount("99999").Everything(), server.URL
}

// TestEverythingService_Messages_MultiPage exercises Link-header following and
// X-Total-Count metadata across two pages of the /messages.json root.
func TestEverythingService_Messages_MultiPage(t *testing.T) {
	var serverURL string
	var page1, page2 int
	svc, url := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/messages.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "3")
		if r.URL.Query().Get("page") == "2" {
			page2++
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`[{"id":3,"type":"Message","title":"Third","url":"https://x/3.json","status":"active","visible_to_clients":false,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","inherits_status":true,"app_url":"https://x/3","bucket":{"id":9,"name":"P","type":"Project"},"parent":{"id":8,"title":"MB","type":"Message::Board"},"creator":{"id":1,"name":"A"}}]`))
			return
		}
		page1++
		w.Header().Set("Link", fmt.Sprintf(`<%s/99999/messages.json?page=2>; rel="next"`, serverURL))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[{"id":1,"type":"Message","title":"First","subject":"First subject","category":{"id":77,"name":"FYI","icon":"💡"},"boosts_count":4,"boosts_url":"https://x/1/boosts.json","url":"https://x/1.json","status":"active","visible_to_clients":false,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","inherits_status":true,"app_url":"https://x/1","bucket":{"id":9,"name":"P","type":"Project"},"parent":{"id":8,"title":"MB","type":"Message::Board"},"creator":{"id":1,"name":"A"}},{"id":2,"type":"Message","title":"Second","url":"https://x/2.json","status":"active","visible_to_clients":false,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","inherits_status":true,"app_url":"https://x/2","bucket":{"id":9,"name":"P","type":"Project"},"parent":{"id":8,"title":"MB","type":"Message::Board"},"creator":{"id":1,"name":"A"}}]`))
	})
	serverURL = url

	result, err := svc.Messages(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page1 != 1 || page2 != 1 {
		t.Fatalf("expected one hit per page, got page1=%d page2=%d", page1, page2)
	}
	if len(result.Recordings) != 3 {
		t.Fatalf("expected 3 recordings across pages, got %d", len(result.Recordings))
	}
	if result.Recordings[0].ID != 1 || result.Recordings[2].ID != 3 {
		t.Errorf("unexpected ordering across pages: %d..%d", result.Recordings[0].ID, result.Recordings[2].ID)
	}
	if result.Recordings[0].Bucket == nil || result.Recordings[0].Bucket.Name != "P" {
		t.Errorf("expected embedded bucket, got %+v", result.Recordings[0].Bucket)
	}
	// Type-specific message fields must decode (not dropped by the generic
	// recording projection): subject and the boostable counts.
	if result.Recordings[0].Subject != "First subject" {
		t.Errorf("expected message subject decoded, got %q", result.Recordings[0].Subject)
	}
	if result.Recordings[0].BoostsCount != 4 || result.Recordings[0].BoostsURL == "" {
		t.Errorf("expected boost data decoded, got count=%d url=%q", result.Recordings[0].BoostsCount, result.Recordings[0].BoostsURL)
	}
	if result.Recordings[0].Category == nil || result.Recordings[0].Category.Name != "FYI" {
		t.Errorf("expected message category decoded, got %+v", result.Recordings[0].Category)
	}
	if result.Meta.TotalCount != 3 {
		t.Errorf("expected TotalCount 3, got %d", result.Meta.TotalCount)
	}
}

// TestEverythingService_OverdueTodos_Unpaginated verifies the overdue list is a
// complete oldest-first array with NO Link-following, even if a Link header is
// present.
func TestEverythingService_OverdueTodos_Unpaginated(t *testing.T) {
	var hits int
	svc, _ := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/99999/todos/overdue.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		// A Link header is present but must NOT be followed for the overdue list.
		w.Header().Set("Link", `</99999/todos/overdue.json?page=2>; rel="next"`)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[
			{"id":10,"content":"Oldest","due_on":"2025-01-01","completed":false},
			{"id":11,"content":"Newer","due_on":"2025-06-01","completed":false}
		]`))
	})

	todos, err := svc.OverdueTodos(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hits != 1 {
		t.Errorf("expected exactly 1 request (no Link-following), got %d", hits)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 overdue todos, got %d", len(todos))
	}
	if todos[0].ID != 10 || todos[1].ID != 11 {
		t.Errorf("expected oldest-first ordering preserved, got %d, %d", todos[0].ID, todos[1].ID)
	}
}

// TestEverythingService_Files_PerVariantDecode proves the heterogeneous
// /files.json feed decodes all three variants — a full Upload recording, a
// Basecamp Document recording, and a rich-text Attachment envelope — in one
// non-empty array, including a float-spelled and a null width.
func TestEverythingService_Files_PerVariantDecode(t *testing.T) {
	svc, _ := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[
			{"id":900,"type":"Upload","title":"logo.png","filename":"logo.png","content_type":"image/png","byte_size":1281,"width":1024.0,"height":768.0,"url":"https://x/uploads/900.json","download_url":"https://x/d/900","app_download_url":"https://storage/900","bucket":{"id":9,"name":"P","type":"Project"}},
			{"id":901,"type":"Document","title":"Spec","url":"https://x/documents/901.json","content":"<div>Body</div>","content_attachments":[{"sgid":"sgid-doc","content_type":"image/png"}],"bucket":{"id":9,"name":"P","type":"Project"}},
			{"id":902,"type":"Attachment","attachable_sgid":"sgid-902","filename":"chart.avif","content_type":"image/avif","byte_size":4096,"width":null,"height":null,"download_url":"https://storage/blobs/902","parent":{"id":800,"title":"A message","type":"Message"}}
		]`))
	})

	result, err := svc.Files(context.Background(), 1, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result.Files))
	}
	up := result.Files[0]
	if strv(up.Type) != "Upload" || strv(up.Filename) != "logo.png" || up.AppDownloadURL == nil {
		t.Errorf("upload variant not decoded: %+v", up)
	}
	if up.Width == nil || *up.Width != 1024 {
		t.Errorf("expected float-spelled width 1024, got %v", up.Width)
	}
	// Presence-faithful strings: the Upload variant omits attachable_sgid, so it
	// must decode as nil (absent), not a fabricated empty string, per SPEC §10.
	if up.AttachableSGID != nil {
		t.Errorf("upload variant should not carry attachable_sgid, got %q", *up.AttachableSGID)
	}
	// Presence-faithful pointers: the Upload carries byte_size, the Document
	// omits it (must decode nil, not a fabricated 0), per SPEC §10.
	if up.ByteSize == nil || *up.ByteSize != 1281 {
		t.Errorf("expected upload byte_size 1281, got %v", up.ByteSize)
	}
	doc := result.Files[1]
	if strv(doc.Type) != "Document" || strv(doc.Title) != "Spec" {
		t.Errorf("document variant not decoded: %+v", doc)
	}
	// The Document body (content + content_attachments) must decode, not be
	// dropped by the superset (finding #7).
	if strv(doc.Content) != "<div>Body</div>" {
		t.Errorf("expected document content to decode, got %q", strv(doc.Content))
	}
	if doc.ContentAttachments == nil || len(*doc.ContentAttachments) != 1 {
		t.Errorf("expected 1 document content attachment, got %v", doc.ContentAttachments)
	}
	if doc.ByteSize != nil {
		t.Errorf("expected nil byte_size on the document variant, got %v", *doc.ByteSize)
	}
	// Presence-faithful strings: the Document variant omits filename entirely, so
	// it must be nil (absent) rather than an empty-string sentinel.
	if doc.Filename != nil {
		t.Errorf("expected nil filename on the document variant, got %q", *doc.Filename)
	}
	att := result.Files[2]
	if strv(att.Type) != "Attachment" || strv(att.AttachableSGID) != "sgid-902" || att.Parent == nil {
		t.Errorf("attachment variant not decoded: %+v", att)
	}
	if att.Width != nil || att.Height != nil {
		t.Errorf("expected nil width/height for non-image attachment, got w=%v h=%v", att.Width, att.Height)
	}
}

// TestEverythingService_Files_Filters verifies the kind and people_ids[] query
// parameters reach the wire.
func TestEverythingService_Files_Filters(t *testing.T) {
	var captured string
	svc, _ := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := svc.Files(context.Background(), 1, &EverythingFilesOptions{Kind: "images", PeopleIDs: []int64{101, 202}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q, _ := url.ParseQuery(captured)
	if q.Get("kind") != "images" {
		t.Errorf("expected kind=images, got query %q", captured)
	}
	ids := q["people_ids[]"]
	if len(ids) != 2 || ids[0] != "101" || ids[1] != "202" {
		t.Errorf("expected people_ids[]=101,202, got %v (query %q)", ids, captured)
	}
}

// TestEverythingService_Messages_PassesPageParam verifies that a positive page
// is sent as the ?page= query param (regression: previously page>1 silently
// fetched page 1).
func TestEverythingService_Messages_PassesPageParam(t *testing.T) {
	var captured string
	svc, _ := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[]`))
	})
	if _, err := svc.Messages(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q, _ := url.ParseQuery(captured); q.Get("page") != "2" {
		t.Errorf("expected page=2 forwarded, got query %q", captured)
	}
}

// TestEverythingService_Files_PassesPageAndKindParams verifies the Files feed
// sends the kind, people_ids[], and page query params (not the forwards feed).
func TestEverythingService_Files_PassesPageAndKindParams(t *testing.T) {
	var captured string
	svc, _ := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[]`))
	})
	if _, err := svc.Files(context.Background(), 3, &EverythingFilesOptions{Kind: "pdfs"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q, _ := url.ParseQuery(captured)
	if q.Get("page") != "3" || q.Get("kind") != "pdfs" {
		t.Errorf("expected page=3&kind=pdfs, got %q", captured)
	}
}

// TestEverythingFile_OptionalStringsPreserveAbsence proves the SPEC §10 contract
// for optional strings: an absent field and an explicitly-empty field must not
// collapse. A Document with no filename decodes to nil (and re-marshals omitted),
// while a recording with an explicit "" summary/description decodes to a non-nil
// pointer to "" and re-marshals as an explicit empty string.
func TestEverythingFile_OptionalStringsPreserveAbsence(t *testing.T) {
	// filename absent entirely; description present but explicitly empty.
	wire := `{"id":901,"type":"Document","title":"Spec","description":""}`

	var f EverythingFile
	if err := json.Unmarshal([]byte(wire), &f); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	// Absent filename -> nil (not a fabricated empty string).
	if f.Filename != nil {
		t.Errorf("expected absent filename to decode as nil, got %q", *f.Filename)
	}
	// Explicitly-empty description -> non-nil pointer to "".
	if f.Description == nil {
		t.Fatal("expected explicit empty description to decode as non-nil *string")
	}
	if *f.Description != "" {
		t.Errorf("expected empty description, got %q", *f.Description)
	}

	out, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(out)
	// Absent filename must stay omitted on the way back out.
	if strings.Contains(s, "filename") {
		t.Errorf("expected absent filename to remain omitted, got %s", s)
	}
	// Explicit empty description must survive the round trip as "".
	if !strings.Contains(s, `"description":""`) {
		t.Errorf("expected explicit empty description to round-trip as \"\", got %s", s)
	}
}

// TestEverythingService_TypeSpecificFeedFields proves the check-in and forward
// feeds surface their type-specific projections (group_on; from/subject/replies)
// through the generic recording element rather than dropping them.
func TestEverythingService_TypeSpecificFeedFields(t *testing.T) {
	// Check-ins: group_on.
	svc, _ := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/checkins.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[{"id":1,"type":"Question::Answer","title":"How's it going?","group_on":"2026-07-20","status":"active","visible_to_clients":false,"created_at":"2026-07-20T00:00:00Z","updated_at":"2026-07-20T00:00:00Z","inherits_status":true,"url":"https://x/1.json","app_url":"https://x/1","bucket":{"id":9,"name":"P","type":"Project"},"creator":{"id":1,"name":"A"}}]`))
	})
	ci, err := svc.Checkins(context.Background(), 1)
	if err != nil {
		t.Fatalf("checkins error: %v", err)
	}
	if len(ci.Recordings) != 1 || ci.Recordings[0].GroupOn != "2026-07-20" {
		t.Errorf("expected check-in group_on 2026-07-20, got %+v", ci.Recordings)
	}

	// Forwards: from, subject, replies_count/url.
	svc2, _ := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/forwards.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[{"id":2,"type":"Inbox::Forward","title":"FW: Invoice","subject":"FW: Invoice","from":"vendor@example.com","replies_count":2,"replies_url":"https://x/2/replies.json","status":"active","visible_to_clients":false,"created_at":"2026-07-20T00:00:00Z","updated_at":"2026-07-20T00:00:00Z","inherits_status":true,"url":"https://x/2.json","app_url":"https://x/2","bucket":{"id":9,"name":"P","type":"Project"},"creator":{"id":1,"name":"A"}}]`))
	})
	fw, err := svc2.Forwards(context.Background(), 1)
	if err != nil {
		t.Fatalf("forwards error: %v", err)
	}
	if len(fw.Recordings) != 1 {
		t.Fatalf("expected 1 forward, got %d", len(fw.Recordings))
	}
	f := fw.Recordings[0]
	if f.Subject != "FW: Invoice" || f.From != "vendor@example.com" || f.RepliesCount != 2 || f.RepliesURL == "" {
		t.Errorf("expected forward type-specific fields, got subject=%q from=%q replies=%d url=%q", f.Subject, f.From, f.RepliesCount, f.RepliesURL)
	}
}

// TestEverythingService_Boosts_RecordingCarriesBucket proves the everything
// /boosts.json feed renders each boost's recording through the FULL recording
// projection. EverythingBoost.Recording is a *Recording (not the reduced Parent
// shape the shared Boost keeps for source compatibility), so it must carry the
// bucket, creator, and type-specific fields — routed through the real Boosts()
// -> everythingBoostFromGenerated pipeline.
func TestEverythingService_Boosts_RecordingCarriesBucket(t *testing.T) {
	svc, _ := everythingTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/boosts.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[
			{"id":5001,"content":"👏","created_at":"2024-01-15T10:00:00Z","booster":{"id":1,"name":"Victor Cooper"},"recording":{"id":800,"type":"Message","status":"active","title":"A message","subject":"A message","url":"https://3.basecampapi.com/99999/buckets/9/messages/800.json","creator":{"id":7,"name":"Ann Perkins"},"bucket":{"id":9,"name":"The Leto Laptop","type":"Project"}}}
		]`))
	})

	result, err := svc.Boosts(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Boosts) != 1 {
		t.Fatalf("expected 1 boost, got %d", len(result.Boosts))
	}
	b := result.Boosts[0]
	if b.Recording == nil {
		t.Fatal("expected the boosted recording to decode")
	}
	if b.Recording.Bucket == nil || b.Recording.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected the boosted recording to carry its bucket, got %+v", b.Recording.Bucket)
	}
	// Full projection (not reduced parent): creator, url, and the type-specific
	// message subject must all decode.
	if b.Recording.Creator == nil || b.Recording.Creator.Name != "Ann Perkins" {
		t.Errorf("expected full projection creator, got %+v", b.Recording.Creator)
	}
	if b.Recording.URL == "" || b.Recording.Subject != "A message" {
		t.Errorf("expected full projection url+subject, got url=%q subject=%q", b.Recording.URL, b.Recording.Subject)
	}
}
