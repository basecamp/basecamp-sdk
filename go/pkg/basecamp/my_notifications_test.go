package basecamp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testMyNotificationsServer(t *testing.T, handler http.HandlerFunc) *MyNotificationsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.MyNotifications()
}

func TestMyNotificationsService_Get(t *testing.T) {
	svc := testMyNotificationsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/my/readings.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"unreads":[{"id":1,"title":"New comment","created_at":"2026-07-21T00:00:00Z","updated_at":"2026-07-21T00:00:00Z"}],"reads":[],"memories":[],"bubble_ups_count":0,"scheduled_bubble_ups_count":0}`))
	})

	result, err := svc.Get(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Unreads) != 1 {
		t.Errorf("expected 1 unread, got %d", len(result.Unreads))
	}
	if result.Unreads[0].Title != "New comment" {
		t.Errorf("expected 'New comment', got %q", result.Unreads[0].Title)
	}
}

func TestMyNotificationsService_Get_WithPage(t *testing.T) {
	svc := testMyNotificationsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("expected page=2, got %q", r.URL.Query().Get("page"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"unreads":[],"reads":[],"memories":[],"bubble_ups_count":0,"scheduled_bubble_ups_count":0}`))
	})

	_, err := svc.Get(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestMyNotificationsService_Get_LimitBubbleUps verifies that
// WithLimitBubbleUps sends limit_bubble_ups=true and that a response which omits
// the scheduled_bubble_ups key (per the documented cap) still decodes, with the
// counts preserved.
func TestMyNotificationsService_Get_LimitBubbleUps(t *testing.T) {
	var capturedQuery string
	svc := testMyNotificationsServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		// scheduled_bubble_ups key intentionally omitted; counts still present.
		w.Write([]byte(`{
			"unreads": [], "reads": [], "memories": [],
			"bubble_ups": [{"id": 10, "title": "Bubbled", "created_at": "2026-07-21T00:00:00Z", "updated_at": "2026-07-21T00:00:00Z"}, {"id": 11, "title": "Bubbled 2", "created_at": "2026-07-21T00:00:00Z", "updated_at": "2026-07-21T00:00:00Z"}],
			"bubble_ups_count": 5,
			"scheduled_bubble_ups_count": 3
		}`))
	})

	result, err := svc.GetWithOptions(context.Background(), 0, WithLimitBubbleUps())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedQuery != "limit_bubble_ups=true" {
		t.Errorf("expected query limit_bubble_ups=true, got %q", capturedQuery)
	}
	if len(result.BubbleUps) != 2 {
		t.Errorf("expected 2 bubble_ups, got %d", len(result.BubbleUps))
	}
	if result.ScheduledBubbleUps != nil {
		t.Errorf("expected scheduled_bubble_ups omitted (nil), got %v", result.ScheduledBubbleUps)
	}
	if result.BubbleUpsCount != 5 {
		t.Errorf("expected bubble_ups_count 5, got %d", result.BubbleUpsCount)
	}
	if result.ScheduledBubbleUpsCount != 3 {
		t.Errorf("expected scheduled_bubble_ups_count 3, got %d", result.ScheduledBubbleUpsCount)
	}
}

// TestMyNotificationsService_BubbleUps_MultiPage exercises the dedicated
// bubble-ups endpoint across multiple pages, verifying Link-header following and
// generated pagination metadata (X-Total-Count), not just a single-page decode.
// Current bubble-ups appear first, scheduled bubble-ups follow.
func TestMyNotificationsService_BubbleUps_MultiPage(t *testing.T) {
	var serverURL string
	var page1Hits, page2Hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/my/readings/bubble_ups.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "3")
		if r.URL.Query().Get("page") == "2" {
			page2Hits++
			w.WriteHeader(200)
			// scheduled bubble-up, ordered after the current ones
			w.Write([]byte(`[{"id":30,"title":"Scheduled","bubble_up_at":"2026-08-01T00:00:00Z","created_at":"2026-07-21T00:00:00Z","updated_at":"2026-07-21T00:00:00Z"}]`))
			return
		}
		page1Hits++
		// page 1: two current bubble-ups, with a Link header to page 2
		w.Header().Set("Link", fmt.Sprintf(`<%s/99999/my/readings/bubble_ups.json?page=2>; rel="next"`, serverURL))
		w.WriteHeader(200)
		w.Write([]byte(`[{"id":10,"title":"Current A","created_at":"2026-07-21T00:00:00Z","updated_at":"2026-07-21T00:00:00Z"},{"id":11,"title":"Current B","created_at":"2026-07-21T00:00:00Z","updated_at":"2026-07-21T00:00:00Z"}]`))
	}))
	t.Cleanup(server.Close)
	serverURL = server.URL

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	svc := client.ForAccount("99999").MyNotifications()

	result, err := svc.BubbleUps(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page1Hits != 1 || page2Hits != 1 {
		t.Fatalf("expected one hit per page, got page1=%d page2=%d", page1Hits, page2Hits)
	}
	if len(result.BubbleUps) != 3 {
		t.Fatalf("expected 3 bubble_ups across pages, got %d", len(result.BubbleUps))
	}
	// Ordering: current bubble-ups first, then scheduled.
	if result.BubbleUps[0].ID != 10 || result.BubbleUps[1].ID != 11 || result.BubbleUps[2].ID != 30 {
		t.Errorf("unexpected ordering: %d, %d, %d", result.BubbleUps[0].ID, result.BubbleUps[1].ID, result.BubbleUps[2].ID)
	}
	if result.BubbleUps[2].BubbleUpAt == nil {
		t.Errorf("expected scheduled item to carry bubble_up_at")
	}
	if result.Meta.TotalCount != 3 {
		t.Errorf("expected TotalCount 3, got %d", result.Meta.TotalCount)
	}
}

// TestMyNotificationsService_BubbleUps_SinglePage verifies that a positive page
// disables auto-pagination and returns only that page (no Link-following).
func TestMyNotificationsService_BubbleUps_SinglePage(t *testing.T) {
	var hits int
	svc := testMyNotificationsServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("expected page=2, got %q", r.URL.Query().Get("page"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "3")
		// A Link header is present but must NOT be followed for an explicit page.
		w.Header().Set("Link", `</99999/my/readings/bubble_ups.json?page=3>; rel="next"`)
		w.WriteHeader(200)
		w.Write([]byte(`[{"id":30,"title":"Scheduled","created_at":"2026-07-21T00:00:00Z","updated_at":"2026-07-21T00:00:00Z"}]`))
	})

	result, err := svc.BubbleUps(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hits != 1 {
		t.Errorf("expected exactly 1 request for explicit page, got %d", hits)
	}
	if len(result.BubbleUps) != 1 {
		t.Errorf("expected 1 bubble_up for single page, got %d", len(result.BubbleUps))
	}
	// A next-page Link is present but not followed in explicit-page mode, so the
	// result must advertise itself as a partial view.
	if !result.Meta.Truncated {
		t.Error("expected Meta.Truncated=true when a next-page Link is present in single-page mode")
	}
}

func TestMyNotificationsService_Get_SentinelCreatorID(t *testing.T) {
	// The BC3 API returns system-generated notifications with creator.id: "basecamp"
	// and personable_type: "LocalPerson". The normalize pass walks Person-shaped objects
	// (anything carrying personable_type) and coerces the non-numeric id to 0 while
	// preserving the original label as system_label. The wrapper then decodes the
	// resulting numeric payload into Notification.Creator without error.
	svc := testMyNotificationsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"unreads": [{
				"id": 42,
				"title": "System notification",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
				"creator": {
					"id": "basecamp",
					"name": "Basecamp",
					"personable_type": "LocalPerson"
				}
			}],
			"reads": [],
			"memories": [],
			"bubble_ups_count": 0,
			"scheduled_bubble_ups_count": 0
		}`))
	})

	result, err := svc.Get(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error (sentinel creator.id should not crash): %v", err)
	}
	if len(result.Unreads) != 1 {
		t.Errorf("expected 1 unread, got %d", len(result.Unreads))
	}
	if result.Unreads[0].Title != "System notification" {
		t.Errorf("expected 'System notification', got %q", result.Unreads[0].Title)
	}
	// Creator now flows through the wrapper. Verify the sentinel was normalized:
	// id collapsed to 0, original label preserved as system_label.
	if result.Unreads[0].Creator == nil {
		t.Fatal("expected Creator to be populated after wrapper exposes the field")
	}
	if result.Unreads[0].Creator.ID != 0 {
		t.Errorf("expected sentinel creator.id to normalize to 0, got %d", result.Unreads[0].Creator.ID)
	}
	if result.Unreads[0].Creator.SystemLabel != "basecamp" {
		t.Errorf("expected system_label %q, got %q", "basecamp", result.Unreads[0].Creator.SystemLabel)
	}
	if result.Unreads[0].Creator.PersonableType != "LocalPerson" {
		t.Errorf("expected personable_type 'LocalPerson', got %q", result.Unreads[0].Creator.PersonableType)
	}
}

func TestMyNotificationsService_Get_StringCreatorIDWithoutPersonableType(t *testing.T) {
	// BC3 sometimes serializes a real person's notification creator/participants
	// id as a JSON string ("1049715914") on an object that has no
	// personable_type. Notification.Creator/Participants decode into Person.ID
	// (a plain int64), which cannot unmarshal a JSON string, so the whole Get
	// would fail. normalizeEmbeddedPeopleJSON coerces these ids by their
	// creator/participants position even without personable_type.
	svc := testMyNotificationsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{
			"unreads": [{
				"id": 7,
				"title": "Comment",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
				"creator": {
					"id": "1049715914",
					"name": "Jane"
				},
				"participants": [
					{"id": "2000000001", "name": "P1"},
					{"id": "basecamp", "name": "System"}
				]
			}],
			"reads": [],
			"memories": [],
			"bubble_ups_count": 0,
			"scheduled_bubble_ups_count": 0
		}`))
	})

	result, err := svc.Get(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error (string creator.id without personable_type should not crash): %v", err)
	}
	if len(result.Unreads) != 1 {
		t.Fatalf("expected 1 unread, got %d", len(result.Unreads))
	}
	n := result.Unreads[0]
	if n.Creator == nil {
		t.Fatal("expected Creator to be populated")
	}
	if n.Creator.ID != 1049715914 {
		t.Errorf("expected creator.id 1049715914, got %d", n.Creator.ID)
	}
	if n.Creator.Name != "Jane" {
		t.Errorf("expected creator name Jane, got %q", n.Creator.Name)
	}
	if len(n.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(n.Participants))
	}
	if n.Participants[0].ID != 2000000001 {
		t.Errorf("expected participant[0].id 2000000001, got %d", n.Participants[0].ID)
	}
	// The sentinel participant collapses to 0 with the label preserved.
	if n.Participants[1].ID != 0 {
		t.Errorf("expected sentinel participant[1].id to normalize to 0, got %d", n.Participants[1].ID)
	}
	if n.Participants[1].SystemLabel != "basecamp" {
		t.Errorf("expected participant[1].system_label %q, got %q", "basecamp", n.Participants[1].SystemLabel)
	}
}

func TestMyNotificationsService_MarkAsRead(t *testing.T) {
	var receivedBody map[string]any
	svc := testMyNotificationsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		receivedBody = decodeRequestBody(t, r)
		w.WriteHeader(200)
	})

	err := svc.MarkAsRead(context.Background(), []string{"sgid://bc3/Recording/123", "sgid://bc3/Recording/456"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	readables, ok := receivedBody["readables"].([]any)
	if !ok {
		t.Fatalf("expected readables array, got %T", receivedBody["readables"])
	}
	if len(readables) != 2 {
		t.Errorf("expected 2 readables, got %d", len(readables))
	}
	if fmt.Sprint(readables[0]) != "sgid://bc3/Recording/123" {
		t.Errorf("expected first readable 'sgid://bc3/Recording/123', got %v", readables[0])
	}
}

func TestMyNotificationsService_MarkAsRead_Empty(t *testing.T) {
	svc := testMyNotificationsServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	err := svc.MarkAsRead(context.Background(), []string{})
	if err == nil {
		t.Error("expected error for empty readables")
	}
}
