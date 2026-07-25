package basecamp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		_, _ = w.Write([]byte(`[{"id":1,"type":"Message","title":"First","url":"https://x/1.json","status":"active","visible_to_clients":false,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","inherits_status":true,"app_url":"https://x/1","bucket":{"id":9,"name":"P","type":"Project"},"parent":{"id":8,"title":"MB","type":"Message::Board"},"creator":{"id":1,"name":"A"}},{"id":2,"type":"Message","title":"Second","url":"https://x/2.json","status":"active","visible_to_clients":false,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","inherits_status":true,"app_url":"https://x/2","bucket":{"id":9,"name":"P","type":"Project"},"parent":{"id":8,"title":"MB","type":"Message::Board"},"creator":{"id":1,"name":"A"}}]`))
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
			{"id":901,"type":"Document","title":"Spec","url":"https://x/documents/901.json","content_type":"text/html","bucket":{"id":9,"name":"P","type":"Project"}},
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
	if up.Type != "Upload" || up.Filename != "logo.png" || up.AppDownloadURL == "" {
		t.Errorf("upload variant not decoded: %+v", up)
	}
	if up.Width == nil || *up.Width != 1024 {
		t.Errorf("expected float-spelled width 1024, got %v", up.Width)
	}
	if up.AttachableSGID != "" {
		t.Errorf("upload variant should not carry attachable_sgid")
	}
	doc := result.Files[1]
	if doc.Type != "Document" || doc.Title != "Spec" {
		t.Errorf("document variant not decoded: %+v", doc)
	}
	att := result.Files[2]
	if att.Type != "Attachment" || att.AttachableSGID != "sgid-902" || att.Parent == nil {
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

// TestEverythingService_Messages_ForwardsPage verifies that a positive page is
// forwarded as the ?page= query param (regression: previously page>1 silently
// fetched page 1). Also covers the Files kind+page combination.
func TestEverythingService_Messages_ForwardsPage(t *testing.T) {
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

func TestEverythingService_Files_ForwardsPageAndKind(t *testing.T) {
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
