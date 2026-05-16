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
		w.Write([]byte(`{"unreads":[{"id":1,"title":"New comment"}],"reads":[],"memories":[]}`))
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
		w.Write([]byte(`{"unreads":[],"reads":[],"memories":[]}`))
	})

	_, err := svc.Get(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMyNotificationsService_Get_SentinelCreatorID(t *testing.T) {
	// The BC3 API returns system-generated notifications with creator.id: "basecamp"
	// and personable_type: "LocalPerson". normalizeJSON walks Person-shaped objects
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
			"memories": []
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
