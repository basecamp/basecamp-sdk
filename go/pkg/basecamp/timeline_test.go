package basecamp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimelineEvent_Unmarshal(t *testing.T) {
	data := `{
		"id": 12345,
		"created_at": "2024-03-15T10:30:00Z",
		"kind": "message_created",
		"parent_recording_id": 67890,
		"url": "https://3.basecampapi.com/123/buckets/456/messages/789.json",
		"app_url": "https://3.basecamp.com/123/buckets/456/messages/789",
		"action": "created",
		"target": "message",
		"title": "Test Message",
		"summary_excerpt": "This is a test...",
		"creator": {
			"id": 111,
			"name": "Test User",
			"email_address": "test@example.com"
		},
		"bucket": {
			"id": 456,
			"name": "Test Project",
			"type": "Project"
		}
	}`

	var event TimelineEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if event.ID != 12345 {
		t.Errorf("expected ID 12345, got %d", event.ID)
	}
	if event.Kind != "message_created" {
		t.Errorf("expected Kind 'message_created', got %q", event.Kind)
	}
	if event.ParentRecordingID != 67890 {
		t.Errorf("expected ParentRecordingID 67890, got %d", event.ParentRecordingID)
	}
	if event.Action != "created" {
		t.Errorf("expected Action 'created', got %q", event.Action)
	}
	if event.Target != "message" {
		t.Errorf("expected Target 'message', got %q", event.Target)
	}
	if event.Title != "Test Message" {
		t.Errorf("expected Title 'Test Message', got %q", event.Title)
	}
	if event.SummaryExcerpt != "This is a test..." {
		t.Errorf("expected SummaryExcerpt 'This is a test...', got %q", event.SummaryExcerpt)
	}
	if event.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}
	if event.Creator.Name != "Test User" {
		t.Errorf("expected Creator.Name 'Test User', got %q", event.Creator.Name)
	}
	if event.Bucket == nil {
		t.Fatal("expected Bucket to be non-nil")
	}
	if event.Bucket.Name != "Test Project" {
		t.Errorf("expected Bucket.Name 'Test Project', got %q", event.Bucket.Name)
	}

	// Check timestamp
	expectedTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	if !event.CreatedAt.Equal(expectedTime) {
		t.Errorf("expected CreatedAt %v, got %v", expectedTime, event.CreatedAt)
	}
}

func TestPersonProgressResult_Unmarshal(t *testing.T) {
	data := `{
		"person": {
			"id": 111,
			"name": "Test User",
			"email_address": "test@example.com"
		},
		"events": [
			{
				"id": 12345,
				"kind": "todo_completed",
				"action": "completed",
				"title": "Test Todo"
			}
		]
	}`

	var resp PersonProgressResult
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Person == nil {
		t.Fatal("expected Person to be non-nil")
	}
	if resp.Person.Name != "Test User" {
		t.Errorf("expected Person.Name 'Test User', got %q", resp.Person.Name)
	}
	if len(resp.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(resp.Events))
	}
	if resp.Events[0].Kind != "todo_completed" {
		t.Errorf("expected event Kind 'todo_completed', got %q", resp.Events[0].Kind)
	}
}

// timelineEventJSON returns a JSON timeline event with the given ID.
func timelineEventJSON(id int) string {
	return fmt.Sprintf(`{
		"id": %d,
		"created_at": "2024-03-15T10:30:00Z",
		"kind": "message_created",
		"action": "created",
		"target": "message",
		"title": "Event %d",
		"summary_excerpt": "excerpt",
		"url": "https://example.com/event/%d.json",
		"app_url": "https://example.com/event/%d",
		"creator": {"id": 1, "name": "User"},
		"bucket": {"id": 1, "name": "Project", "type": "Project"}
	}`, id, id, id, id)
}

// timelinePaginationHandler serves paginated timeline event responses.
type timelinePaginationHandler struct {
	pageSize   int
	totalItems int
	totalCount int // value for X-Total-Count header
	pageCount  int32
	serverURL  string
}

func (h *timelinePaginationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&h.pageCount, 1)
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}

	start := (page - 1) * h.pageSize
	remaining := h.totalItems - start
	if remaining <= 0 {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return
	}

	count := min(remaining, h.pageSize)

	// Build JSON array of timeline events
	items := "["
	for i := 0; i < count; i++ {
		if i > 0 {
			items += ","
		}
		items += timelineEventJSON(start + i + 1)
	}
	items += "]"

	if start+count < h.totalItems {
		nextURL := fmt.Sprintf("%s%s?page=%d", h.serverURL, r.URL.Path, page+1)
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
	}

	if h.totalCount > 0 {
		w.Header().Set("X-Total-Count", strconv.Itoa(h.totalCount))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(items))
}

func (h *timelinePaginationHandler) getPageCount() int {
	return int(atomic.LoadInt32(&h.pageCount))
}

// personProgressPaginationHandler serves paginated person progress responses.
// Page 1 returns {person: {...}, events: [...]}, subsequent pages return event arrays.
type personProgressPaginationHandler struct {
	pageSize   int
	totalItems int
	totalCount int
	pageCount  int32
	serverURL  string
}

func (h *personProgressPaginationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&h.pageCount, 1)
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}

	start := (page - 1) * h.pageSize
	remaining := h.totalItems - start
	if remaining <= 0 {
		remaining = 0
	}
	count := min(remaining, h.pageSize)

	eventsJSON := "["
	for i := 0; i < count; i++ {
		if i > 0 {
			eventsJSON += ","
		}
		eventsJSON += timelineEventJSON(start + i + 1)
	}
	eventsJSON += "]"

	if start+count < h.totalItems {
		nextURL := fmt.Sprintf("%s%s?page=%d", h.serverURL, r.URL.Path, page+1)
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
	}

	if h.totalCount > 0 {
		w.Header().Set("X-Total-Count", strconv.Itoa(h.totalCount))
	}

	w.Header().Set("Content-Type", "application/json")

	if page == 1 {
		// First page returns the wrapped person+events structure
		body := fmt.Sprintf(`{"person": {"id": 42, "name": "Test Person", "email_address": "test@example.com", "avatar_url": "", "admin": false, "owner": false}, "events": %s}`, eventsJSON)
		w.Write([]byte(body))
	} else {
		// Subsequent pages return just the events array
		w.Write([]byte(eventsJSON))
	}
}

func (h *personProgressPaginationHandler) getPageCount() int {
	return int(atomic.LoadInt32(&h.pageCount))
}

func newTestTimelineService(serverURL string) *TimelineService {
	cfg := &Config{BaseURL: serverURL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	account := client.ForAccount("12345")
	return account.Timeline()
}

func TestProgress_NilOpts_FollowsPagination(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 5, totalItems: 12, totalCount: 12}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.Progress(t.Context(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 12 {
		t.Errorf("expected 12 events, got %d", len(result.Events))
	}
	if result.Meta.TotalCount != 12 {
		t.Errorf("expected TotalCount 12, got %d", result.Meta.TotalCount)
	}
	if result.Meta.Truncated {
		t.Error("expected Truncated false")
	}
}

func TestProgress_SinglePage(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 5, totalItems: 12, totalCount: 12}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.Progress(t.Context(), &TimelineListOptions{Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 5 {
		t.Errorf("expected 5 events (single page), got %d", len(result.Events))
	}
	if result.Meta.TotalCount != 12 {
		t.Errorf("expected TotalCount 12, got %d", result.Meta.TotalCount)
	}
	if h.getPageCount() != 1 {
		t.Errorf("expected 1 page request, got %d", h.getPageCount())
	}
}

func TestProgress_WithLimit(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 5, totalItems: 20, totalCount: 20}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.Progress(t.Context(), &TimelineListOptions{Limit: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 7 {
		t.Errorf("expected 7 events, got %d", len(result.Events))
	}
	if result.Meta.TotalCount != 20 {
		t.Errorf("expected TotalCount 20, got %d", result.Meta.TotalCount)
	}
}

func TestProjectTimeline_NilOpts_FollowsPagination(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 5, totalItems: 12, totalCount: 12}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.ProjectTimeline(t.Context(), 999, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 12 {
		t.Errorf("expected 12 events, got %d", len(result.Events))
	}
	if result.Meta.TotalCount != 12 {
		t.Errorf("expected TotalCount 12, got %d", result.Meta.TotalCount)
	}
}

func TestProjectTimeline_SinglePage(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 5, totalItems: 12, totalCount: 12}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.ProjectTimeline(t.Context(), 999, &TimelineListOptions{Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 5 {
		t.Errorf("expected 5 events (single page), got %d", len(result.Events))
	}
	if h.getPageCount() != 1 {
		t.Errorf("expected 1 page request, got %d", h.getPageCount())
	}
}

func TestProjectTimeline_WithLimit(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 5, totalItems: 20, totalCount: 20}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.ProjectTimeline(t.Context(), 999, &TimelineListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(result.Events))
	}
}

func TestPersonProgress_NilOpts_FollowsPagination(t *testing.T) {
	h := &personProgressPaginationHandler{pageSize: 5, totalItems: 12, totalCount: 12}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.PersonProgress(t.Context(), 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Person == nil {
		t.Fatal("expected Person to be non-nil")
	}
	if result.Person.Name != "Test Person" {
		t.Errorf("expected Person.Name 'Test Person', got %q", result.Person.Name)
	}
	if len(result.Events) != 12 {
		t.Errorf("expected 12 events, got %d", len(result.Events))
	}
	if result.Meta.TotalCount != 12 {
		t.Errorf("expected TotalCount 12, got %d", result.Meta.TotalCount)
	}
}

func TestPersonProgress_SinglePage(t *testing.T) {
	h := &personProgressPaginationHandler{pageSize: 5, totalItems: 12, totalCount: 12}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.PersonProgress(t.Context(), 42, &TimelineListOptions{Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 5 {
		t.Errorf("expected 5 events (single page), got %d", len(result.Events))
	}
	if h.getPageCount() != 1 {
		t.Errorf("expected 1 page request, got %d", h.getPageCount())
	}
}

func TestPersonProgress_WithLimit(t *testing.T) {
	h := &personProgressPaginationHandler{pageSize: 5, totalItems: 20, totalCount: 20}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.PersonProgress(t.Context(), 42, &TimelineListOptions{Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(result.Events))
	}
}

func TestProgress_UnlimitedFetchesAll(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 5, totalItems: 15, totalCount: 15}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.Progress(t.Context(), &TimelineListOptions{Limit: -1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 15 {
		t.Errorf("expected 15 events, got %d", len(result.Events))
	}
}

func TestProgress_DefaultLimitCaps(t *testing.T) {
	// More items than DefaultTimelineLimit (100)
	h := &timelinePaginationHandler{pageSize: 50, totalItems: 150, totalCount: 150}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.Progress(t.Context(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 100 {
		t.Errorf("expected 100 events (default limit), got %d", len(result.Events))
	}
	if !result.Meta.Truncated {
		t.Error("expected Truncated true when capped at default limit")
	}
}

func TestProgress_VerifyEventIDs(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 3, totalItems: 7, totalCount: 7}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.Progress(t.Context(), &TimelineListOptions{Limit: -1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(result.Events))
	}

	for i, e := range result.Events {
		expectedID := int64(i + 1)
		if e.ID != expectedID {
			t.Errorf("event %d: expected ID %d, got %d", i, expectedID, e.ID)
		}
	}
}

func TestPersonProgress_EmptyEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"person": {"id": 42, "name": "Test Person", "email_address": "", "avatar_url": "", "admin": false, "owner": false}, "events": []}`))
	}))
	defer server.Close()

	svc := newTestTimelineService(server.URL)
	result, err := svc.PersonProgress(t.Context(), 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(result.Events))
	}
}
