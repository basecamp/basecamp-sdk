package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	if event.CreatedAt == nil {
		t.Error("expected CreatedAt to be non-nil")
	} else if !event.CreatedAt.Equal(expectedTime) {
		t.Errorf("expected CreatedAt %v, got %v", expectedTime, *event.CreatedAt)
	}
}

// TestTimelineEvent_AdditiveFields exercises runtime decode of the additive
// fields: a non-empty avatars_sample, the schedule-entry data payload with a
// date-only (all_day) starts_at/ends_at, and BOTH heterogeneous attachment
// variants — a full Upload recording and a rich-text attachment/blob partial —
// in a single non-empty array. Empty arrays / shape-only tests do not prove
// polymorphic decode, so both variants carry real per-variant fields here.
func TestTimelineEvent_AdditiveFields(t *testing.T) {
	data := `[
		{
			"id": 1,
			"created_at": "2024-03-15T10:30:00Z",
			"kind": "chat_transcript_rollup",
			"avatars_sample": [
				"https://3.basecampapi.com/1/people/aaa/avatar",
				"https://3.basecampapi.com/1/people/bbb/avatar"
			]
		},
		{
			"id": 2,
			"created_at": "2024-03-15T10:31:00Z",
			"kind": "schedule_entry_created",
			"avatars_sample": [],
			"data": {
				"all_day": true,
				"starts_at": "2025-10-30",
				"ends_at": "2025-10-30"
			}
		},
		{
			"id": 3,
			"created_at": "2024-03-15T10:32:00Z",
			"kind": "upload_created",
			"avatars_sample": [],
			"attachments": [
				{
					"id": 900,
					"type": "Upload",
						"inherits_status": true, "created_at": "2024-03-15T10:30:00Z", "updated_at": "2024-03-15T10:31:00Z",
						"bookmark_url": "https://3.basecampapi.com/1/my/bookmarks/sgid-900.json", "subscription_url": "https://3.basecampapi.com/1/buckets/2/recordings/900/subscription.json",
						"comments_count": 3, "comments_url": "https://3.basecampapi.com/1/buckets/2/recordings/900/comments.json", "boosts_count": 5, "boosts_url": "https://3.basecampapi.com/1/buckets/2/recordings/900/boosts.json",
						"position": 2, "description": "<div>Schematic</div>", "description_attachments": [],
						"parent": { "id": 800, "title": "Assets", "type": "Vault", "url": "https://3.basecampapi.com/1/buckets/2/vaults/800.json", "app_url": "https://3.basecamp.com/1/buckets/2/vaults/800" },
						"bucket": { "id": 2, "name": "Test Project", "type": "Project" },
						"creator": { "id": 55, "name": "Uploader Person" },
					"status": "active",
					"visible_to_clients": false,
					"title": "Diagram",
					"filename": "diagram.png",
					"content_type": "image/png",
					"byte_size": 20480,
					"width": 1024.0,
					"height": 768.0,
					"url": "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
					"app_url": "https://3.basecamp.com/1/buckets/2/uploads/900",
					"download_url": "https://3.basecampapi.com/1/buckets/2/uploads/900/download/diagram.png",
					"app_download_url": "https://3.basecamp.com/1/buckets/2/uploads/900/download"
				}
			]
		},
		{
			"id": 4,
			"created_at": "2024-03-15T10:33:00Z",
			"kind": "comment_created",
			"avatars_sample": [],
			"attachments": [
				{
					"id": 500,
					"attachable_sgid": "sgid-attachable-500",
					"sgid": "sgid-500",
					"status_url": "https://3.basecampapi.com/1/attachments/sgid-500/status.json",
					"caption": "See attached",
					"filename": "notes.pdf",
					"content_type": "application/pdf",
					"byte_size": 4096,
					"key": "blobkey500",
					"width": null,
					"height": null,
					"previewable": true,
					"download_url": "https://3.basecampapi.com/1/blobs/blobkey500/download/notes.pdf",
					"preview_url": "https://3.basecampapi.com/1/blobs/blobkey500/previews/full",
					"thumbnail_url": "https://3.basecampapi.com/1/blobs/blobkey500/previews/card"
				}
			]
		}
	]`

	var events []TimelineEvent
	if err := json.Unmarshal([]byte(data), &events); err != nil {
		t.Fatalf("failed to unmarshal timeline events: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// avatars_sample non-empty
	if got := events[0].AvatarsSample; len(got) != 2 || got[0] == "" {
		t.Errorf("expected 2 avatar URLs, got %v", got)
	}

	// data with date-only all-day timing (FlexibleTime accepts date-only)
	ev := events[1]
	if ev.Data == nil {
		t.Fatal("expected data on schedule_entry_created event")
	}
	if !ev.Data.AllDay {
		t.Error("expected all_day true")
	}
	wantDate := time.Date(2025, 10, 30, 0, 0, 0, 0, time.UTC)
	if ev.Data.StartsAt == nil || ev.Data.EndsAt == nil {
		t.Fatalf("expected non-nil bounds, got starts=%v ends=%v", ev.Data.StartsAt, ev.Data.EndsAt)
	}
	if !ev.Data.StartsAt.Equal(wantDate) {
		t.Errorf("expected StartsAt %v, got %v", wantDate, ev.Data.StartsAt)
	}
	if !ev.Data.EndsAt.Equal(wantDate) {
		t.Errorf("expected EndsAt %v, got %v", wantDate, ev.Data.EndsAt)
	}

	// Upload-recording attachment variant
	up := events[2].Attachments
	if len(up) != 1 {
		t.Fatalf("expected 1 upload attachment, got %d", len(up))
	}
	if strv(up[0].Type) != "Upload" || strv(up[0].Filename) != "diagram.png" || up[0].AppDownloadURL == nil {
		t.Errorf("upload variant fields not decoded: %+v", up[0])
	}
	if up[0].Width == nil || *up[0].Width != 1024 {
		t.Errorf("expected float-spelled width 1024, got %v", up[0].Width)
	}
	// Presence-faithful: the Upload variant omits attachable_sgid, so it must be
	// nil (absent), not a fabricated empty string, per SPEC §10.
	if up[0].AttachableSGID != nil {
		t.Errorf("upload variant should not carry attachable_sgid, got %q", *up[0].AttachableSGID)
	}
	// Full uploads/_upload projection: the documented recording fields must decode
	// (they are not silently dropped). Spot-check across scalars, urls, counts,
	// and the nested parent/bucket/creator objects.
	if up[0].InheritsStatus == nil || !*up[0].InheritsStatus {
		t.Errorf("expected inherits_status true, got %v", up[0].InheritsStatus)
	}
	if up[0].CommentsCount == nil || *up[0].CommentsCount != 3 || up[0].BoostsCount == nil || *up[0].BoostsCount != 5 {
		t.Errorf("expected comments_count=3 boosts_count=5, got c=%v b=%v", up[0].CommentsCount, up[0].BoostsCount)
	}
	if up[0].Position == nil || *up[0].Position != 2 {
		t.Errorf("expected position 2, got %v", up[0].Position)
	}
	if strv(up[0].BookmarkURL) == "" || strv(up[0].SubscriptionURL) == "" || strv(up[0].CommentsURL) == "" || strv(up[0].BoostsURL) == "" {
		t.Errorf("expected recording urls decoded, got %+v", up[0])
	}
	if strv(up[0].Description) != "<div>Schematic</div>" {
		t.Errorf("expected description decoded, got %q", strv(up[0].Description))
	}
	if up[0].Parent == nil || up[0].Parent.ID != 800 || up[0].Parent.Type != "Vault" {
		t.Errorf("expected parent {800, Vault}, got %+v", up[0].Parent)
	}
	if up[0].Bucket == nil || up[0].Bucket.ID != 2 || up[0].Bucket.Name != "Test Project" {
		t.Errorf("expected bucket {2, Test Project}, got %+v", up[0].Bucket)
	}
	if up[0].Creator == nil || up[0].Creator.Name != "Uploader Person" {
		t.Errorf("expected creator 'Uploader Person', got %+v", up[0].Creator)
	}
	if up[0].CreatedAt == nil || up[0].UpdatedAt == nil {
		t.Errorf("expected created_at/updated_at decoded, got c=%v u=%v", up[0].CreatedAt, up[0].UpdatedAt)
	}
	// Presence-faithful: the fixture carries an explicit "description_attachments": [],
	// so the pointer must be non-nil (present) and empty, and re-marshal as [] not
	// be dropped by omitempty.
	if up[0].DescriptionAttachments == nil {
		t.Error("expected present empty description_attachments (non-nil), got nil")
	} else if len(*up[0].DescriptionAttachments) != 0 {
		t.Errorf("expected empty description_attachments, got %d", len(*up[0].DescriptionAttachments))
	}
	if b, _ := json.Marshal(up[0]); !strings.Contains(string(b), `"description_attachments":[]`) {
		t.Errorf("expected present empty array to round-trip as []; got %s", string(b))
	}

	// Rich-text attachment/blob variant
	att := events[3].Attachments
	if len(att) != 1 {
		t.Fatalf("expected 1 blob attachment, got %d", len(att))
	}
	if strv(att[0].AttachableSGID) != "sgid-attachable-500" || strv(att[0].Caption) != "See attached" || strv(att[0].Key) != "blobkey500" {
		t.Errorf("attachment variant fields not decoded: %+v", att[0])
	}
	if att[0].Previewable == nil || !*att[0].Previewable || strv(att[0].ThumbnailURL) == "" {
		t.Errorf("attachment variant preview fields not decoded: %+v", att[0])
	}
	if att[0].Width != nil || att[0].Height != nil {
		t.Errorf("expected nil width/height for non-image blob, got w=%v h=%v", att[0].Width, att[0].Height)
	}
	// Presence-faithful: the attachment variant carries no upload timestamps or
	// visibility, so those pointers stay nil (and re-marshal omits them).
	if att[0].CreatedAt != nil || att[0].VisibleToClients != nil {
		t.Errorf("expected nil upload-variant fields on attachment, got created_at=%v visible=%v", att[0].CreatedAt, att[0].VisibleToClients)
	}
}

// TestTimelineEventData_NullBounds verifies a schedule-entry event whose timing
// bounds are JSON null decodes cleanly (the bounds are required-and-nullable:
// always present, value may be null). Go decodes null to a nil pointer, which
// re-marshals as null rather than a fabricated instant; the static SDKs type
// the bounds `string | null` so they don't fail to decode.
func TestTimelineEventData_NullBounds(t *testing.T) {
	data := `[{"id":9,"created_at":"2024-03-15T10:31:00Z","kind":"schedule_entry_created","data":{"all_day":true,"starts_at":null,"ends_at":null}}]`
	var events []TimelineEvent
	if err := json.Unmarshal([]byte(data), &events); err != nil {
		t.Fatalf("failed to unmarshal event with null bounds: %v", err)
	}
	if events[0].Data == nil {
		t.Fatal("expected data to be present")
	}
	if !events[0].Data.AllDay {
		t.Error("expected all_day true")
	}
	if events[0].Data.StartsAt != nil || events[0].Data.EndsAt != nil {
		t.Errorf("expected null bounds to decode as nil, got starts=%v ends=%v", events[0].Data.StartsAt, events[0].Data.EndsAt)
	}
	// Null in, null out: the bounds are required-and-nullable, so the keys must
	// survive re-marshal carrying null, not a fabricated instant.
	b, err := json.Marshal(events[0].Data)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(b) != `{"all_day":true,"starts_at":null,"ends_at":null}` {
		t.Errorf("expected null bounds to re-marshal as null, got %s", b)
	}
}

// TestTimelineAttachment_PresenceFaithfulRoundTrip verifies that re-marshaling a
// decoded attachment-variant superset omits the absent upload-variant fields
// (no fabricated zero timestamp, no dropped explicit false) — the reason the
// optional timestamps/booleans are pointers.
func TestTimelineAttachment_PresenceFaithfulRoundTrip(t *testing.T) {
	// Attachment variant: explicit previewable:false, no created_at/updated_at,
	// no visible_to_clients.
	src := `{"id":500,"attachable_sgid":"sgid-500","filename":"notes.pdf","previewable":false}`
	var a TimelineAttachment
	if err := json.Unmarshal([]byte(src), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Previewable == nil || *a.Previewable != false {
		t.Fatalf("expected explicit previewable=false preserved, got %v", a.Previewable)
	}
	out, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	// Absent upload-variant fields must NOT be fabricated on re-marshal.
	for _, k := range []string{"created_at", "updated_at", "visible_to_clients"} {
		if _, present := round[k]; present {
			t.Errorf("re-marshal fabricated absent field %q: %s", k, out)
		}
	}
	// Explicit false must survive the round trip (not dropped by omitempty).
	if v, ok := round["previewable"]; !ok || v != false {
		t.Errorf("re-marshal dropped explicit previewable=false: %s", out)
	}
}

// wrappedPaginationHandler serves wrapped {person, events} responses with Link headers.
type wrappedPaginationHandler struct {
	pageSize  int
	total     int
	serverURL string
	pageCount int32
}

func (h *wrappedPaginationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&h.pageCount, 1)
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}

	start := (page - 1) * h.pageSize
	remaining := h.total - start
	if remaining <= 0 {
		remaining = 0
	}
	count := min(remaining, h.pageSize)

	events := make([]map[string]interface{}, count)
	for i := 0; i < count; i++ {
		events[i] = map[string]interface{}{
			"id":     start + i + 1,
			"action": "created",
			"target": "todo",
			"title":  fmt.Sprintf("Event %d", start+i+1),
		}
	}

	if start+count < h.total {
		nextURL := fmt.Sprintf("%s%s?page=%d", h.serverURL, r.URL.Path, page+1)
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
	}
	w.Header().Set("X-Total-Count", fmt.Sprintf("%d", h.total))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"person": map[string]interface{}{"id": 456, "name": "Jane Doe", "email_address": "jane@example.com"},
		"events": events,
	})
}

func TestPersonProgress_MultiPageWrapped(t *testing.T) {
	h := &wrappedPaginationHandler{pageSize: 3, total: 7}
	srv := httptest.NewServer(h)
	defer srv.Close()
	h.serverURL = srv.URL

	cfg := &Config{BaseURL: srv.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	account := client.ForAccount("999")
	ts := NewTimelineService(account)

	result, err := ts.PersonProgress(context.Background(), 456, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify person is preserved from page 1
	if result.Person == nil {
		t.Fatal("expected Person to be non-nil")
	}
	if result.Person.Name != "Jane Doe" {
		t.Errorf("expected Person.Name 'Jane Doe', got %q", result.Person.Name)
	}

	// Verify all 7 events accumulated across 3 pages
	if len(result.Events) != 7 {
		t.Fatalf("expected 7 events across 3 pages, got %d", len(result.Events))
	}
	for i, e := range result.Events {
		expected := fmt.Sprintf("Event %d", i+1)
		if e.Title != expected {
			t.Errorf("event[%d]: expected Title %q, got %q", i, expected, e.Title)
		}
	}

	// Verify metadata
	if result.Meta.TotalCount != 7 {
		t.Errorf("expected TotalCount 7, got %d", result.Meta.TotalCount)
	}
	if result.Meta.Truncated {
		t.Error("expected Truncated=false when all events fetched")
	}

	// Should have fetched 3 pages (3+3+1)
	pages := int(atomic.LoadInt32(&h.pageCount))
	if pages != 3 {
		t.Errorf("expected 3 page requests, got %d", pages)
	}
}

func TestPersonProgress_MultiPageWithLimit(t *testing.T) {
	h := &wrappedPaginationHandler{pageSize: 3, total: 9}
	srv := httptest.NewServer(h)
	defer srv.Close()
	h.serverURL = srv.URL

	cfg := &Config{BaseURL: srv.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	account := client.ForAccount("999")
	ts := NewTimelineService(account)

	result, err := ts.PersonProgress(context.Background(), 456, &TimelineListOptions{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 5 {
		t.Fatalf("expected 5 events (limited), got %d", len(result.Events))
	}
	if !result.Meta.Truncated {
		t.Error("expected Truncated=true when limited")
	}
}

// TestPersonProgress_LimitExactBoundaryNotTruncated pins the exact-boundary
// contract: when the limit equals the total and the final page carries no
// Link header, nothing was dropped and no pages remain — Truncated is false.
func TestPersonProgress_LimitExactBoundaryNotTruncated(t *testing.T) {
	h := &wrappedPaginationHandler{pageSize: 3, total: 6}
	srv := httptest.NewServer(h)
	defer srv.Close()
	h.serverURL = srv.URL

	cfg := &Config{BaseURL: srv.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	account := client.ForAccount("999")
	ts := NewTimelineService(account)

	result, err := ts.PersonProgress(context.Background(), 456, &TimelineListOptions{Limit: 6})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(result.Events))
	}
	if result.Meta.Truncated {
		t.Error("expected Truncated=false when limit exactly equals total and no next Link remains")
	}
	if pages := int(atomic.LoadInt32(&h.pageCount)); pages != 2 {
		t.Errorf("expected 2 page requests, got %d", pages)
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
// Every page returns {person: {...}, events: [...]}, matching the actual BC3 API.
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

	// Every page returns the wrapped person+events structure (matching the actual BC3 API)
	body := fmt.Sprintf(`{"person": {"id": 42, "name": "Test Person", "email_address": "test@example.com", "avatar_url": "", "admin": false, "owner": false}, "events": %s}`, eventsJSON)
	w.Write([]byte(body))
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

func TestProjectTimeline_DefaultLimitCaps(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 50, totalItems: 150, totalCount: 150}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.ProjectTimeline(t.Context(), 999, nil)
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

func TestProjectTimeline_ExplicitZeroLimitUsesDefault(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 50, totalItems: 150, totalCount: 150}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.ProjectTimeline(t.Context(), 999, &TimelineListOptions{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 100 {
		t.Errorf("expected 100 events (Limit:0 = default), got %d", len(result.Events))
	}
	if !result.Meta.Truncated {
		t.Error("expected Truncated true when capped at default limit")
	}
}

func TestProjectTimeline_UnlimitedFetchesAll(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 50, totalItems: 150, totalCount: 150}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.ProjectTimeline(t.Context(), 999, &TimelineListOptions{Limit: -1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 150 {
		t.Errorf("expected 150 events (unlimited), got %d", len(result.Events))
	}
	if result.Meta.Truncated {
		t.Error("expected Truncated false when all events fetched")
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

func TestPersonProgress_DefaultLimitCaps(t *testing.T) {
	h := &personProgressPaginationHandler{pageSize: 50, totalItems: 150, totalCount: 150}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.PersonProgress(t.Context(), 42, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 100 {
		t.Errorf("expected 100 events (default limit), got %d", len(result.Events))
	}
	if !result.Meta.Truncated {
		t.Error("expected Truncated true when capped at default limit")
	}
	if result.Person == nil || result.Person.Name != "Test Person" {
		t.Error("expected Person preserved from page 1")
	}
}

func TestPersonProgress_ExplicitZeroLimitUsesDefault(t *testing.T) {
	h := &personProgressPaginationHandler{pageSize: 50, totalItems: 150, totalCount: 150}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.PersonProgress(t.Context(), 42, &TimelineListOptions{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 100 {
		t.Errorf("expected 100 events (Limit:0 = default), got %d", len(result.Events))
	}
	if !result.Meta.Truncated {
		t.Error("expected Truncated true when capped at default limit")
	}
}

func TestPersonProgress_UnlimitedFetchesAll(t *testing.T) {
	h := &personProgressPaginationHandler{pageSize: 50, totalItems: 150, totalCount: 150}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.PersonProgress(t.Context(), 42, &TimelineListOptions{Limit: -1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 150 {
		t.Errorf("expected 150 events (unlimited), got %d", len(result.Events))
	}
	if result.Meta.Truncated {
		t.Error("expected Truncated false when all events fetched")
	}
}

func TestProgress_ExplicitZeroLimitUsesDefault(t *testing.T) {
	h := &timelinePaginationHandler{pageSize: 50, totalItems: 150, totalCount: 150}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	svc := newTestTimelineService(server.URL)
	result, err := svc.Progress(t.Context(), &TimelineListOptions{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Events) != 100 {
		t.Errorf("expected 100 events (Limit:0 = default), got %d", len(result.Events))
	}
	if !result.Meta.Truncated {
		t.Error("expected Truncated true when capped at default limit")
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
