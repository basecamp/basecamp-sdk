package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpmock "github.com/basecamp/basecamp-sdk/go/internal/testing"
)

// These tests demonstrate using the httpmock.Registry for low-level HTTP
// mocking when testing code that makes direct HTTP calls (like the Client's
// Get/Post methods) rather than using the generated OpenAPI client.
//
// For testing the high-level service methods (Projects().List, etc.), use
// httptest.NewServer as shown in the _WithServer tests below.

// TestClient_Get_WithRegistry demonstrates using the Registry for direct HTTP calls.
func TestClient_Get_WithRegistry(t *testing.T) {
	reg := httpmock.NewRegistry(t)

	// Stub a GET request
	reg.Register(
		httpmock.REST("GET", "12345/custom-endpoint.json"),
		httpmock.RespondJSON(map[string]interface{}{
			"message": "Hello, World!",
		}),
	)

	cfg := DefaultConfig()
	cfg.BaseURL = "https://3.basecampapi.com"
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(reg))

	// Use the low-level Get method
	account := client.ForAccount("12345")
	resp, err := account.Get(context.Background(), "/custom-endpoint.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := resp.UnmarshalData(&result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["message"] != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", result["message"])
	}

	reg.Verify(t)
}

// TestClient_Post_WithRegistry demonstrates testing POST with the Registry.
func TestClient_Post_WithRegistry(t *testing.T) {
	reg := httpmock.NewRegistry(t)

	var capturedPayload map[string]interface{}

	// Stub POST and capture the payload
	reg.Register(
		httpmock.REST("POST", "12345/items.json"),
		httpmock.RESTPayload(201, `{"id": 1, "name": "Created"}`, func(payload map[string]interface{}) {
			capturedPayload = payload
		}),
	)

	cfg := DefaultConfig()
	cfg.BaseURL = "https://3.basecampapi.com"
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(reg))

	account := client.ForAccount("12345")
	_, err := account.Post(context.Background(), "/items.json", map[string]string{
		"name": "Test Item",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the captured payload
	if capturedPayload["name"] != "Test Item" {
		t.Errorf("expected name 'Test Item', got %v", capturedPayload["name"])
	}

	reg.Verify(t)
}

// TestClient_ErrorResponse_WithRegistry demonstrates testing error responses.
func TestClient_ErrorResponse_WithRegistry(t *testing.T) {
	reg := httpmock.NewRegistry(t)

	reg.Register(
		httpmock.REST("GET", "12345/missing.json"),
		httpmock.RespondNotFound(),
	)

	cfg := DefaultConfig()
	cfg.BaseURL = "https://3.basecampapi.com"
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(reg))

	account := client.ForAccount("12345")
	_, err := account.Get(context.Background(), "/missing.json")

	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	// The SDK returns an *Error for 404 responses
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.Code != CodeNotFound {
		t.Errorf("expected CodeNotFound, got %s", apiErr.Code)
	}

	reg.Verify(t)
}

// TestClient_MultipleRequests_WithRegistry demonstrates registering
// multiple stubs for successive calls to the same endpoint.
func TestClient_MultipleRequests_WithRegistry(t *testing.T) {
	reg := httpmock.NewRegistry(t)

	// Register multiple stubs for successive calls
	// Each stub is matched and consumed in order
	reg.Register(
		httpmock.REST("GET", "12345/counter.json"),
		httpmock.RespondJSON(map[string]int{"count": 1}),
	)
	reg.Register(
		httpmock.REST("GET", "12345/counter.json"),
		httpmock.RespondJSON(map[string]int{"count": 2}),
	)

	cfg := DefaultConfig()
	cfg.BaseURL = "https://3.basecampapi.com"
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token, WithTransport(reg))

	account := client.ForAccount("12345")

	// First call
	resp1, err := account.Get(context.Background(), "/counter.json")
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	var result1 map[string]int
	_ = resp1.UnmarshalData(&result1)
	if result1["count"] != 1 {
		t.Errorf("first call: expected count 1, got %d", result1["count"])
	}

	// Second call
	resp2, err := account.Get(context.Background(), "/counter.json")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	var result2 map[string]int
	_ = resp2.UnmarshalData(&result2)
	if result2["count"] != 2 {
		t.Errorf("second call: expected count 2, got %d", result2["count"])
	}

	reg.Verify(t)
}

// TestProjects_List_WithServer demonstrates testing service methods with httptest.
// This is the recommended pattern for testing high-level service methods that
// use the generated OpenAPI client internally.
func TestProjects_List_WithServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/12345/projects.json"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify auth header
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("expected Authorization header 'Bearer test-token', got %q", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "name": "Project One", "status": "active"},
			{"id": 2, "name": "Project Two", "status": "active"},
		})
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)

	account := client.ForAccount("12345")
	result, err := account.Projects().List(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(result.Projects))
	}
	if result.Projects[0].Name != "Project One" {
		t.Errorf("expected first project name 'Project One', got %q", result.Projects[0].Name)
	}
}

// TestProjects_Get_WithServer demonstrates testing a single resource fetch.
func TestProjects_Get_WithServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The OpenAPI generated client may not include .json extension
		// Check for both forms
		expectedPathWithExt := "/12345/projects/999.json"
		expectedPathNoExt := "/12345/projects/999"
		if r.URL.Path != expectedPathWithExt && r.URL.Path != expectedPathNoExt {
			t.Errorf("expected path %s or %s, got %s", expectedPathWithExt, expectedPathNoExt, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          999,
			"name":        "The Leto Laptop",
			"description": "Laptop product launch.",
			"status":      "active",
		})
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)

	account := client.ForAccount("12345")
	project, err := account.Projects().Get(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if project.ID != 999 {
		t.Errorf("expected project ID 999, got %d", project.ID)
	}
	if project.Name != "The Leto Laptop" {
		t.Errorf("expected project name 'The Leto Laptop', got %q", project.Name)
	}
}
