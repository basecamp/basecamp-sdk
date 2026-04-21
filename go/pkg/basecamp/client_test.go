package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSingleRequest_204ReturnsValidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Get(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", resp.StatusCode)
	}
	if string(resp.Data) != "null" {
		t.Errorf("expected Data to be %q, got %q", "null", string(resp.Data))
	}
	if !json.Valid(resp.Data) {
		t.Error("expected Data to be valid JSON")
	}
}

func TestSingleRequest_204UnmarshalData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Get(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type Resource struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var result *Resource
	if err := resp.UnmarshalData(&result); err != nil {
		t.Fatalf("UnmarshalData should not error on 204 null body: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result from 204, got %+v", result)
	}
}

func TestSingleRequest_200EmptyBodyNotNormalized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// No body written — server bug, not 204
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Get(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected empty Data for 200 with no body, got %q", string(resp.Data))
	}
	if json.Valid(resp.Data) {
		t.Error("expected invalid JSON for 200 with empty body (not normalized to null)")
	}

	var result map[string]any
	if err := resp.UnmarshalData(&result); err == nil {
		t.Error("expected UnmarshalData error for empty 200 body, got nil")
	}
}

func TestSingleRequest_200WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":42,"name":"Test"}`))
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Get(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	type Resource struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var result Resource
	if err := resp.UnmarshalData(&result); err != nil {
		t.Fatalf("UnmarshalData failed: %v", err)
	}
	if result.ID != 42 {
		t.Errorf("expected ID 42, got %d", result.ID)
	}
	if result.Name != "Test" {
		t.Errorf("expected Name %q, got %q", "Test", result.Name)
	}
}

func TestSingleRequest_204Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Delete(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", resp.StatusCode)
	}
	if string(resp.Data) != "null" {
		t.Errorf("expected Data to be %q, got %q", "null", string(resp.Data))
	}
	if !json.Valid(resp.Data) {
		t.Error("expected Data to be valid JSON")
	}
}

func TestSingleRequest_204UnmarshalNonPointer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Get(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type Resource struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var result Resource
	if err := resp.UnmarshalData(&result); err != nil {
		t.Fatalf("UnmarshalData should not error on 204 null body: %v", err)
	}
	if result.ID != 0 || result.Name != "" {
		t.Errorf("expected zero-value Resource from 204, got %+v", result)
	}
}

func TestSingleRequest_204UnmarshalSlice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Get(context.Background(), "/items.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	type Item struct {
		ID int `json:"id"`
	}
	var items []Item
	if err := resp.UnmarshalData(&items); err != nil {
		t.Fatalf("UnmarshalData should not error on 204 null body: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil slice from 204, got %v", items)
	}
}

func TestSingleRequest_201WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":123,"name":"Created"}`))
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Post(context.Background(), "/test.json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	type Resource struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var result Resource
	if err := resp.UnmarshalData(&result); err != nil {
		t.Fatalf("UnmarshalData failed: %v", err)
	}
	if result.ID != 123 {
		t.Errorf("expected ID 123, got %d", result.ID)
	}
	if result.Name != "Created" {
		t.Errorf("expected Name %q, got %q", "Created", result.Name)
	}
}

func TestSingleRequest_201EmptyBodyNotNormalized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		// No body written — server bug, not 204
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	resp, err := client.Post(context.Background(), "/test.json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected empty Data for 201 with no body, got %q", string(resp.Data))
	}
	if json.Valid(resp.Data) {
		t.Error("expected invalid JSON for 201 with empty body (not normalized to null)")
	}

	var result map[string]any
	if err := resp.UnmarshalData(&result); err == nil {
		t.Error("expected UnmarshalData error for empty 201 body, got nil")
	}
}

func TestSingleRequest_ErrorIncludesRequestID(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		status        int
		body          string
		wantCode      string
		wantMessage   string
		wantHint      string
		wantRetryable bool
	}{
		{
			name:        "not found get",
			method:      http.MethodGet,
			status:      http.StatusNotFound,
			wantCode:    CodeNotFound,
			wantMessage: "Resource not found: ",
		},
		{
			name:          "rate limit post",
			method:        http.MethodPost,
			status:        http.StatusTooManyRequests,
			wantCode:      CodeRateLimit,
			wantMessage:   "Rate limited",
			wantHint:      "Try again later",
			wantRetryable: true,
		},
		{
			name:        "forbidden scope put",
			method:      http.MethodPut,
			status:      http.StatusForbidden,
			wantCode:    CodeForbidden,
			wantMessage: "Access denied: insufficient scope",
			wantHint:    "Re-authenticate with full scope",
		},
		{
			name:        "generic api message",
			method:      http.MethodDelete,
			status:      http.StatusTeapot,
			body:        `{"error":"brew stopped"}`,
			wantCode:    CodeAPI,
			wantMessage: "brew stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-Id", "req-raw-123")
				if tt.body != "" {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(tt.status)
				if tt.body != "" {
					_, _ = w.Write([]byte(tt.body))
				}
			}))
			defer server.Close()

			cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
			client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
			client.httpOpts.MaxRetries = 1

			var err error
			switch tt.method {
			case http.MethodGet:
				_, err = client.Get(context.Background(), "/test.json")
			case http.MethodPost:
				_, err = client.Post(context.Background(), "/test.json", map[string]any{"ok": true})
			case http.MethodPut:
				_, err = client.Put(context.Background(), "/test.json", map[string]any{"ok": true})
			case http.MethodDelete:
				_, err = client.Delete(context.Background(), "/test.json")
			default:
				t.Fatalf("unsupported method %s", tt.method)
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			apiErr, ok := err.(*Error)
			if !ok {
				t.Fatalf("expected *Error, got %T", err)
			}
			if apiErr.RequestID != "req-raw-123" {
				t.Fatalf("RequestID = %q, want %q", apiErr.RequestID, "req-raw-123")
			}
			if apiErr.Code != tt.wantCode {
				t.Fatalf("Code = %q, want %q", apiErr.Code, tt.wantCode)
			}
			if apiErr.Message != tt.wantMessage && !(tt.wantCode == CodeNotFound && len(apiErr.Message) >= len(tt.wantMessage) && apiErr.Message[:len(tt.wantMessage)] == tt.wantMessage) {
				t.Fatalf("Message = %q, want %q", apiErr.Message, tt.wantMessage)
			}
			if apiErr.Hint != tt.wantHint {
				t.Fatalf("Hint = %q, want %q", apiErr.Hint, tt.wantHint)
			}
			if apiErr.Retryable != tt.wantRetryable {
				t.Fatalf("Retryable = %v, want %v", apiErr.Retryable, tt.wantRetryable)
			}
		})
	}
}
