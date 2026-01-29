package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

// paginationHandler creates a handler that serves paginated responses.
// It returns the handler and a function to get the server URL (set after server creation).
type paginationHandler struct {
	pageSize   int
	totalItems int
	pageCount  int32
	serverURL  string
}

func (h *paginationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt32(&h.pageCount, 1)
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}

	// Calculate items for this page
	start := (page - 1) * h.pageSize
	remaining := h.totalItems - start
	if remaining <= 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]int{})
		return
	}

	count := h.pageSize
	if remaining < h.pageSize {
		count = remaining
	}

	items := make([]map[string]int, count)
	for i := 0; i < count; i++ {
		items[i] = map[string]int{"id": start + i + 1}
	}

	// Set next link if there are more items
	if start+count < h.totalItems {
		nextURL := fmt.Sprintf("%s%s?page=%d", h.serverURL, r.URL.Path, page+1)
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *paginationHandler) getPageCount() int {
	return int(atomic.LoadInt32(&h.pageCount))
}

// TestGetAllWithLimit_ZeroLimit tests that limit=0 fetches all pages.
func TestGetAllWithLimit_ZeroLimit(t *testing.T) {
	h := &paginationHandler{pageSize: 3, totalItems: 9}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	results, err := client.GetAllWithLimit(ctx, "/items.json", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 9 {
		t.Errorf("expected 9 items, got %d", len(results))
	}

	if h.getPageCount() != 3 {
		t.Errorf("expected 3 page requests, got %d", h.getPageCount())
	}
}

// TestGetAllWithLimit_ExactLimit tests limit matching exact page boundary.
func TestGetAllWithLimit_ExactLimit(t *testing.T) {
	// Use a large totalItems to ensure there's always a next page
	h := &paginationHandler{pageSize: 3, totalItems: 1000}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	// Request exactly 6 items (2 pages worth)
	results, err := client.GetAllWithLimit(ctx, "/items.json", 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 6 {
		t.Errorf("expected 6 items, got %d", len(results))
	}

	// Should have stopped after 2 pages
	if h.getPageCount() != 2 {
		t.Errorf("expected 2 page requests, got %d", h.getPageCount())
	}
}

// TestGetAllWithLimit_LimitMidPage tests limit in middle of a page.
func TestGetAllWithLimit_LimitMidPage(t *testing.T) {
	h := &paginationHandler{pageSize: 5, totalItems: 1000}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	// Request 7 items (page 1 has 5, page 2 has 5, but we want 7)
	results, err := client.GetAllWithLimit(ctx, "/items.json", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 7 {
		t.Errorf("expected 7 items, got %d", len(results))
	}

	// Should have fetched 2 pages (5 + 5 = 10, trimmed to 7)
	if h.getPageCount() != 2 {
		t.Errorf("expected 2 page requests, got %d", h.getPageCount())
	}

	// Verify we got the correct items (1-7)
	for i, raw := range results {
		var item map[string]int
		if err := json.Unmarshal(raw, &item); err != nil {
			t.Fatalf("failed to unmarshal item %d: %v", i, err)
		}
		expectedID := i + 1
		if item["id"] != expectedID {
			t.Errorf("item %d: expected id %d, got %d", i, expectedID, item["id"])
		}
	}
}

// TestGetAllWithLimit_LimitExceedsTotalItems tests limit larger than available items.
func TestGetAllWithLimit_LimitExceedsTotalItems(t *testing.T) {
	h := &paginationHandler{pageSize: 3, totalItems: 5}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	// Request 100 items but only 5 exist
	results, err := client.GetAllWithLimit(ctx, "/items.json", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("expected 5 items, got %d", len(results))
	}

	// Should have fetched 2 pages (3 + 2)
	if h.getPageCount() != 2 {
		t.Errorf("expected 2 page requests, got %d", h.getPageCount())
	}
}

// TestGetAllWithLimit_LimitOne tests limit=1 returns single item.
func TestGetAllWithLimit_LimitOne(t *testing.T) {
	h := &paginationHandler{pageSize: 10, totalItems: 100}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	results, err := client.GetAllWithLimit(ctx, "/items.json", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 item, got %d", len(results))
	}

	// Should only fetch 1 page
	if h.getPageCount() != 1 {
		t.Errorf("expected 1 page request, got %d", h.getPageCount())
	}

	// Verify it's the first item
	var item map[string]int
	if err := json.Unmarshal(results[0], &item); err != nil {
		t.Fatalf("failed to unmarshal item: %v", err)
	}
	if item["id"] != 1 {
		t.Errorf("expected id 1, got %d", item["id"])
	}
}

// TestGetAllWithLimit_SinglePageNoNext tests single page without Link header.
func TestGetAllWithLimit_SinglePageNoNext(t *testing.T) {
	h := &paginationHandler{pageSize: 3, totalItems: 3}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	// Request 10 items but only 3 exist in single page
	results, err := client.GetAllWithLimit(ctx, "/items.json", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 items, got %d", len(results))
	}

	if h.getPageCount() != 1 {
		t.Errorf("expected 1 page request, got %d", h.getPageCount())
	}
}

// TestGetAllWithLimit_EmptyResponse tests empty first page response.
func TestGetAllWithLimit_EmptyResponse(t *testing.T) {
	h := &paginationHandler{pageSize: 10, totalItems: 0}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	results, err := client.GetAllWithLimit(ctx, "/items.json", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 items, got %d", len(results))
	}

	if h.getPageCount() != 1 {
		t.Errorf("expected 1 page request, got %d", h.getPageCount())
	}
}

// accountPaginationHandler wraps paginationHandler to verify account ID in path.
type accountPaginationHandler struct {
	paginationHandler
	expectedAccountID string
	t                 *testing.T
}

func (h *accountPaginationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verify account ID is in path
	expectedPath := "/" + h.expectedAccountID + "/items.json"
	if r.URL.Path != expectedPath {
		h.t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
	}
	h.paginationHandler.ServeHTTP(w, r)
}

// TestAccountClient_GetAllWithLimit tests AccountClient.GetAllWithLimit.
func TestAccountClient_GetAllWithLimit(t *testing.T) {
	h := &accountPaginationHandler{
		paginationHandler: paginationHandler{pageSize: 3, totalItems: 100},
		expectedAccountID: "12345",
		t:                 t,
	}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	account := client.ForAccount("12345")
	ctx := context.Background()

	results, err := account.GetAllWithLimit(ctx, "/items.json", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("expected 5 items, got %d", len(results))
	}

	// Should have fetched 2 pages (3 + 3, trimmed to 5)
	if h.getPageCount() != 2 {
		t.Errorf("expected 2 page requests, got %d", h.getPageCount())
	}
}

// TestGetAll_UsesGetAllWithLimit tests that GetAll delegates to GetAllWithLimit.
func TestGetAll_UsesGetAllWithLimit(t *testing.T) {
	h := &paginationHandler{pageSize: 3, totalItems: 6}
	server := httptest.NewServer(h)
	defer server.Close()
	h.serverURL = server.URL

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	results, err := client.GetAll(ctx, "/items.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 6 {
		t.Errorf("expected 6 items, got %d", len(results))
	}

	if h.getPageCount() != 2 {
		t.Errorf("expected 2 page requests, got %d", h.getPageCount())
	}
}

// TestParseNextLink tests the Link header parser.
func TestParseNextLink(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "standard next link",
			header:   `<https://api.example.com/items?page=2>; rel="next"`,
			expected: "https://api.example.com/items?page=2",
		},
		{
			name:     "next with other rels",
			header:   `<https://api.example.com/items?page=1>; rel="first", <https://api.example.com/items?page=2>; rel="next", <https://api.example.com/items?page=10>; rel="last"`,
			expected: "https://api.example.com/items?page=2",
		},
		{
			name:     "no next link",
			header:   `<https://api.example.com/items?page=1>; rel="first", <https://api.example.com/items?page=10>; rel="last"`,
			expected: "",
		},
		{
			name:     "empty header",
			header:   "",
			expected: "",
		},
		{
			name:     "next link only",
			header:   `<https://3.basecampapi.com/12345/projects.json?page=5>; rel="next"`,
			expected: "https://3.basecampapi.com/12345/projects.json?page=5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNextLink(tt.header)
			if result != tt.expected {
				t.Errorf("parseNextLink(%q) = %q, want %q", tt.header, result, tt.expected)
			}
		})
	}
}
