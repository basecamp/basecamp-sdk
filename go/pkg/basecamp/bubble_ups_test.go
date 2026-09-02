package basecamp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBubbleUpsServiceCreateOmitsAtWhenNil(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/99999/recordings/456/bubble_up.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &gotBody)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	if err := client.ForAccount("99999").BubbleUps().Create(context.Background(), 456, nil); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, present := gotBody["at"]; present {
		t.Fatalf("expected no 'at' in body when scheduling omitted, got %v", gotBody)
	}
}

func TestBubbleUpsServiceCreateScheduled(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/99999/recordings/456/bubble_up.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	at := "2026-09-10T09:00:00Z"
	if err := client.ForAccount("99999").BubbleUps().Create(context.Background(), 456, &at); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if gotBody["at"] != at {
		t.Fatalf("expected 'at'=%q in body, got %v", at, gotBody)
	}
}

func TestBubbleUpsServiceCreateForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	if err := client.ForAccount("99999").BubbleUps().Create(context.Background(), 456, nil); err == nil {
		t.Fatal("expected forbidden error")
	}
}

func TestBubbleUpsServiceDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/99999/recordings/456/bubble_up.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	if err := client.ForAccount("99999").BubbleUps().Delete(context.Background(), 456); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}
