package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testMyAssignmentsServer(t *testing.T, handler http.HandlerFunc) *MyAssignmentsService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	account := client.ForAccount("99999")
	return account.MyAssignments()
}

func TestMyAssignmentsService_Get(t *testing.T) {
	svc := testMyAssignmentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/my/assignments.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"priorities":[{"id":1,"content":"Priority task"}],"non_priorities":[{"id":2,"content":"Regular task"}]}`))
	})

	result, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Priorities) != 1 {
		t.Fatalf("expected 1 priority, got %d", len(result.Priorities))
	}
	if len(result.NonPriorities) != 1 {
		t.Fatalf("expected 1 non-priority, got %d", len(result.NonPriorities))
	}
	if result.Priorities[0].Content != "Priority task" {
		t.Errorf("expected 'Priority task', got %q", result.Priorities[0].Content)
	}
}

func TestMyAssignmentsService_Due_WithScope(t *testing.T) {
	svc := testMyAssignmentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scope") != "overdue" {
			t.Errorf("expected scope=overdue, got %q", r.URL.Query().Get("scope"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[{"id":1,"content":"Overdue task","due_on":"2024-01-01"}]`))
	})

	result, err := svc.Due(context.Background(), "overdue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 assignment, got %d", len(result))
	}
}

func TestMyAssignmentsService_Due_NoScope(t *testing.T) {
	svc := testMyAssignmentsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("scope") {
			t.Errorf("expected scope param to be absent, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`[]`))
	})

	result, err := svc.Due(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 assignments, got %d", len(result))
	}
}
