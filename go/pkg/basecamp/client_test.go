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
