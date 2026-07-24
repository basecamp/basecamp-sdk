package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

func searchFixturesDir() string {
	return filepath.Join("..", "..", "..", "spec", "fixtures", "search")
}

func loadSearchFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(searchFixturesDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func TestSearchResult_UnmarshalResults(t *testing.T) {
	data := loadSearchFixture(t, "results.json")

	var results []SearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to unmarshal results.json: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Verify first result (Message)
	r1 := results[0]
	if r1.ID != 1069479351 {
		t.Errorf("expected ID 1069479351, got %d", r1.ID)
	}
	if r1.Status != "active" {
		t.Errorf("expected status 'active', got %q", r1.Status)
	}
	if r1.Type != "Message" {
		t.Errorf("expected type 'Message', got %q", r1.Type)
	}
	if r1.Title != "We won Leto!" {
		t.Errorf("expected title 'We won Leto!', got %q", r1.Title)
	}
	if r1.Subject != "We won Leto!" {
		t.Errorf("expected subject 'We won Leto!', got %q", r1.Subject)
	}
	if r1.Content != "<div>Hello everyone! We got the Leto Laptop project! Time to get started.</div>" {
		t.Errorf("unexpected content: %q", r1.Content)
	}
	if r1.URL != "https://3.basecampapi.com/195539477/buckets/2085958499/messages/1069479351.json" {
		t.Errorf("unexpected URL: %q", r1.URL)
	}
	if r1.AppURL != "https://3.basecamp.com/195539477/buckets/2085958499/messages/1069479351" {
		t.Errorf("unexpected AppURL: %q", r1.AppURL)
	}

	// Verify parent (message board)
	if r1.Parent == nil {
		t.Fatal("expected Parent to be non-nil")
	}
	if r1.Parent.ID != 1069479338 {
		t.Errorf("expected Parent.ID 1069479338, got %d", r1.Parent.ID)
	}
	if r1.Parent.Title != "Message Board" {
		t.Errorf("expected Parent.Title 'Message Board', got %q", r1.Parent.Title)
	}
	if r1.Parent.Type != "Message::Board" {
		t.Errorf("expected Parent.Type 'Message::Board', got %q", r1.Parent.Type)
	}

	// Verify bucket
	if r1.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if r1.Bucket.ID != 2085958499 {
		t.Errorf("expected Bucket.ID 2085958499, got %d", r1.Bucket.ID)
	}
	if r1.Bucket.Name != "The Leto Laptop" {
		t.Errorf("expected Bucket.Name 'The Leto Laptop', got %q", r1.Bucket.Name)
	}
	if r1.Bucket.Type != "Project" {
		t.Errorf("expected Bucket.Type 'Project', got %q", r1.Bucket.Type)
	}

	// Verify creator
	if r1.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if r1.Creator.ID != 1049715914 {
		t.Errorf("expected Creator.ID 1049715914, got %d", r1.Creator.ID)
	}
	if r1.Creator.Name != "Victor Cooper" {
		t.Errorf("expected Creator.Name 'Victor Cooper', got %q", r1.Creator.Name)
	}

	// Verify second result (Todo)
	r2 := results[1]
	if r2.ID != 1069479400 {
		t.Errorf("expected ID 1069479400, got %d", r2.ID)
	}
	if r2.Type != "Todo" {
		t.Errorf("expected type 'Todo', got %q", r2.Type)
	}
	if r2.Title != "Design specs for Leto display" {
		t.Errorf("expected title 'Design specs for Leto display', got %q", r2.Title)
	}
	if r2.Description != "Create detailed specifications for the Leto laptop display panel" {
		t.Errorf("unexpected description: %q", r2.Description)
	}
	if r2.Parent == nil {
		t.Fatal("expected Parent to be non-nil for second result")
	}
	if r2.Parent.Type != "Todolist" {
		t.Errorf("expected Parent.Type 'Todolist', got %q", r2.Parent.Type)
	}
	if r2.Creator == nil {
		t.Fatal("expected Creator to be non-nil for second result")
	}
	if r2.Creator.Name != "Annie Bryan" {
		t.Errorf("expected Creator.Name 'Annie Bryan', got %q", r2.Creator.Name)
	}

	// Verify third result (Comment)
	r3 := results[2]
	if r3.ID != 1069479450 {
		t.Errorf("expected ID 1069479450, got %d", r3.ID)
	}
	if r3.Type != "Comment" {
		t.Errorf("expected type 'Comment', got %q", r3.Type)
	}
	if r3.Content != "<div>The Leto keyboard layout looks great. Let's finalize it.</div>" {
		t.Errorf("unexpected content for comment: %q", r3.Content)
	}

	// Verify timestamps are parsed
	if r1.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if r1.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be non-zero")
	}
}

func TestSearchMetadata_Unmarshal(t *testing.T) {
	data := loadSearchFixture(t, "metadata.json")

	var metadata SearchMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("failed to unmarshal metadata.json: %v", err)
	}

	if len(metadata.RecordingSearchTypes) != 4 {
		t.Fatalf("expected 4 recording search types, got %d", len(metadata.RecordingSearchTypes))
	}
	// The default "everything" option has a null key, preserved as a nil *string
	// (not collapsed to ""), so callers can distinguish it from a real key.
	if got := metadata.RecordingSearchTypes[0]; got.Key != nil || got.Value != "Everything" {
		t.Errorf("expected {Key:nil Value:\"Everything\"}, got %+v", got)
	}
	if got := metadata.RecordingSearchTypes[1]; got.Key == nil || *got.Key != "Kanban::Card" || got.Value != "Card tables" {
		t.Errorf("expected {Key:\"Kanban::Card\" Value:\"Card tables\"}, got %+v", got)
	}

	if len(metadata.FileSearchTypes) != 3 {
		t.Fatalf("expected 3 file search types, got %d", len(metadata.FileSearchTypes))
	}
	if got := metadata.FileSearchTypes[1]; got.Key == nil || *got.Key != "Image" || got.Value != "Images" {
		t.Errorf("expected {Key:\"Image\" Value:\"Images\"}, got %+v", got)
	}

	if metadata.DefaultCreatorLabel != "Anyone" {
		t.Errorf("unexpected default_creator_label: %q", metadata.DefaultCreatorLabel)
	}
	if metadata.DefaultBucketLabel != "All projects" {
		t.Errorf("unexpected default_bucket_label: %q", metadata.DefaultBucketLabel)
	}
	if metadata.DefaultCircleLabel != "All pings" {
		t.Errorf("unexpected default_circle_label: %q", metadata.DefaultCircleLabel)
	}
	if metadata.DefaultFileTypeLabel != "All files" {
		t.Errorf("unexpected default_file_type_label: %q", metadata.DefaultFileTypeLabel)
	}
	if metadata.DefaultTypeLabel != "Everything" {
		t.Errorf("unexpected default_type_label: %q", metadata.DefaultTypeLabel)
	}
}

// TestSearchParams_BracketedArrayWireEncoding drives the generated request
// builder directly (no SearchService) to prove the array filters serialize as
// bracketed repeated keys — bucket_ids[]=1&bucket_ids[]=2 — which is the only
// form Rails' `permit(bucket_ids: [])` accepts. It asserts on the decoded query
// (Go percent-encodes the brackets on the wire; url.Query decodes them back).
func TestSearchParams_BracketedArrayWireEncoding(t *testing.T) {
	params := &generated.SearchParams{
		Q:          "hello",
		BucketIds:  &[]int64{1, 2},
		TypeNames:  &[]string{"Message", "Todo"},
		CreatorIds: &[]int64{7},
	}

	req, err := generated.NewSearchRequest("https://example.test", "195539477", params)
	if err != nil {
		t.Fatalf("NewSearchRequest: %v", err)
	}

	values := req.URL.Query()

	if got := values["bucket_ids[]"]; !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Errorf("bucket_ids[] = %v, want [1 2]", got)
	}
	if got := values["type_names[]"]; !reflect.DeepEqual(got, []string{"Message", "Todo"}) {
		t.Errorf("type_names[] = %v, want [Message Todo]", got)
	}
	if got := values["creator_ids[]"]; !reflect.DeepEqual(got, []string{"7"}) {
		t.Errorf("creator_ids[] = %v, want [7]", got)
	}
	// Rails drops the bare and double-bracketed forms — assert their absence.
	if _, ok := values["bucket_ids"]; ok {
		t.Errorf("unexpected bare bucket_ids key: %v", values["bucket_ids"])
	}
	if _, ok := values["bucket_ids[][]"]; ok {
		t.Errorf("unexpected double-bracket bucket_ids[][] key")
	}
	if got := values.Get("q"); got != "hello" {
		t.Errorf("q = %q, want hello", got)
	}
}

// TestSearchParams_AllFieldsWireEncoding drives the generated request builder
// with every filter param — arrays, scalars, and the deprecated singulars —
// and asserts each lands on the wire with the right key/value.
func TestSearchParams_AllFieldsWireEncoding(t *testing.T) {
	params := &generated.SearchParams{
		Q:           "hello",
		TypeNames:   &[]string{"Message", "Todo"},
		BucketIds:   &[]int64{1, 2},
		CreatorIds:  &[]int64{7},
		FileType:    "Image",
		ExcludeChat: true,
		Since:       "last_30_days",
		Sort:        "recency",
		Type:        "Message",
		BucketId:    9,
		CreatorId:   3,
	}

	req, err := generated.NewSearchRequest("https://example.test", "195539477", params)
	if err != nil {
		t.Fatalf("NewSearchRequest: %v", err)
	}
	q := req.URL.Query()

	checks := map[string]string{
		"q":            "hello",
		"file_type":    "Image",
		"exclude_chat": "true",
		"since":        "last_30_days",
		"sort":         "recency",
		"type":         "Message",
		"bucket_id":    "9",
		"creator_id":   "3",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if got := q["type_names[]"]; !reflect.DeepEqual(got, []string{"Message", "Todo"}) {
		t.Errorf("type_names[] = %v, want [Message Todo]", got)
	}
	if got := q["bucket_ids[]"]; !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Errorf("bucket_ids[] = %v, want [1 2]", got)
	}
	if got := q["creator_ids[]"]; !reflect.DeepEqual(got, []string{"7"}) {
		t.Errorf("creator_ids[] = %v, want [7]", got)
	}
}

// TestSearchService_Search_AllFilters drives the full public wrapper (not just
// the generated request) with every SearchOptions field and asserts the wire.
func TestSearchService_Search_AllFilters(t *testing.T) {
	fixture := loadSearchFixture(t, "results.json")
	svc := testSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		checks := map[string]string{
			"q":            "leto",
			"file_type":    "PDF",
			"exclude_chat": "true",
			"since":        "last_7_days",
			"sort":         "recency",
			"type":         "Document",
			"bucket_id":    "42",
			"creator_id":   "5",
		}
		for key, want := range checks {
			if got := q.Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		if got := q["bucket_ids[]"]; !reflect.DeepEqual(got, []string{"1", "2"}) {
			t.Errorf("bucket_ids[] = %v, want [1 2]", got)
		}
		if got := q["type_names[]"]; !reflect.DeepEqual(got, []string{"Message"}) {
			t.Errorf("type_names[] = %v, want [Message]", got)
		}
		if got := q["creator_ids[]"]; !reflect.DeepEqual(got, []string{"7"}) {
			t.Errorf("creator_ids[] = %v, want [7]", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	_, err := svc.Search(context.Background(), "leto", &SearchOptions{
		TypeNames:   []string{"Message"},
		BucketIds:   []int64{1, 2},
		CreatorIds:  []int64{7},
		FileType:    "PDF",
		ExcludeChat: true,
		Since:       "last_7_days",
		Sort:        "recency",
		Type:        "Document",
		BucketID:    42,
		CreatorID:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSearchService_Search_OmitsEmptyArrayFilters guards the wrapper against
// leaking empty `bucket_ids[]=` params (which Rails normalizes to a bogus [0]
// filter). Neither a nil-opts search nor a partial-filter search may emit any
// array key that wasn't set.
func TestSearchService_Search_OmitsEmptyArrayFilters(t *testing.T) {
	fixture := loadSearchFixture(t, "results.json")

	// (a) nil opts — no array keys at all.
	svc := testSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for _, key := range []string{"type_names[]", "bucket_ids[]", "creator_ids[]"} {
			if q.Has(key) {
				t.Errorf("nil opts: unexpected %s in %q", key, r.URL.RawQuery)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	if _, err := svc.Search(context.Background(), "leto", nil); err != nil {
		t.Fatalf("nil opts: %v", err)
	}

	// (b) only bucket_ids set — the other two must be absent, no empty entries.
	svc2 := testSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q["bucket_ids[]"]; !reflect.DeepEqual(got, []string{"5"}) {
			t.Errorf("bucket_ids[] = %v, want [5]", got)
		}
		if q.Has("type_names[]") || q.Has("creator_ids[]") {
			t.Errorf("unexpected empty array keys in %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	})
	if _, err := svc2.Search(context.Background(), "leto", &SearchOptions{BucketIds: []int64{5}}); err != nil {
		t.Fatalf("partial opts: %v", err)
	}
}

func TestSearchOptions_Marshal(t *testing.T) {
	opts := SearchOptions{
		Sort: "recency",
	}

	out, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("failed to marshal SearchOptions: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	if data["Sort"] != "recency" {
		t.Errorf("unexpected Sort: %v", data["Sort"])
	}
}

func TestSearchResult_DifferentTypes(t *testing.T) {
	data := loadSearchFixture(t, "results.json")

	var results []SearchResult
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatalf("failed to unmarshal results.json: %v", err)
	}

	// Collect unique types
	types := make(map[string]bool)
	for _, r := range results {
		types[r.Type] = true
	}

	// Verify we have multiple types
	expectedTypes := []string{"Message", "Todo", "Comment"}
	for _, et := range expectedTypes {
		if !types[et] {
			t.Errorf("expected type %q in results", et)
		}
	}
}

// testSearchServer creates an httptest.Server and a SearchService wired to it.
func testSearchServer(t *testing.T, handler http.HandlerFunc) *SearchService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.Search()
}

func TestSearchService_Search_BestMatchSort(t *testing.T) {
	fixture := loadSearchFixture(t, "results.json")
	svc := testSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if got := r.URL.Query().Get("sort"); got != "best_match" {
			t.Errorf("expected sort=best_match, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	result, err := svc.Search(context.Background(), "leto", &SearchOptions{Sort: "best_match"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(result.Results))
	}

	// End-to-end projection proof: the polymorphic search projection carries a
	// result's rich text companion array through the wire and searchResultFromGenerated.
	// A given result carries only the array matching its type — content_attachments
	// for a Comment/Message, description_attachments for a Todo.
	for _, r := range result.Results {
		switch r.Type {
		case "Comment", "Message":
			if len(r.ContentAttachments) == 0 {
				t.Errorf("%s result: expected non-empty ContentAttachments", r.Type)
			}
			if r.DescriptionAttachments != nil {
				t.Errorf("%s result: expected no DescriptionAttachments, got %v", r.Type, r.DescriptionAttachments)
			}
		case "Todo":
			if len(r.DescriptionAttachments) == 0 {
				t.Errorf("Todo result: expected non-empty DescriptionAttachments")
			}
			if r.ContentAttachments != nil {
				t.Errorf("Todo result: expected no ContentAttachments, got %v", r.ContentAttachments)
			}
		}
	}
}

func TestSearchService_Search_NoSort(t *testing.T) {
	fixture := loadSearchFixture(t, "results.json")
	svc := testSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("sort") {
			t.Errorf("expected sort to be absent, got %q", r.URL.Query().Get("sort"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(fixture)
	})

	_, err := svc.Search(context.Background(), "leto", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchService_Search_DoesNotSetContentTypeOnGet(t *testing.T) {
	fixture := loadSearchFixture(t, "results.json")
	svc := testSearchServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
			http.Error(w, "wrong method", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("Content-Type"); got != "" {
			t.Errorf("expected no Content-Type for bodyless GET, got %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("expected Accept application/json, got %q", got)
		}
		if got := r.URL.Query().Get("q"); got != "leto" {
			t.Errorf("expected q=leto, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture)
	})

	_, err := svc.Search(context.Background(), "leto", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
