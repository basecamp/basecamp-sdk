package basecamp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockTokenProvider provides a static token for benchmarks.
type mockTokenProvider struct{}

func (m *mockTokenProvider) AccessToken(ctx context.Context) (string, error) {
	return "benchmark-token", nil
}

func (m *mockTokenProvider) Refresh(ctx context.Context) error {
	return nil
}

// BenchmarkClientCreation measures the overhead of creating a new Client.
func BenchmarkClientCreation(b *testing.B) {
	cfg := &Config{
		BaseURL:      "https://3.basecampapi.com",
		AccountID:    "12345",
		CacheEnabled: false,
	}
	tp := &mockTokenProvider{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewClient(cfg, tp)
	}
}

// BenchmarkClientCreationWithOptions measures client creation with all options.
func BenchmarkClientCreationWithOptions(b *testing.B) {
	cfg := &Config{
		BaseURL:      "https://3.basecampapi.com",
		AccountID:    "12345",
		CacheEnabled: false,
	}
	tp := &mockTokenProvider{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewClient(cfg, tp,
			WithTimeout(60*time.Second),
			WithMaxRetries(3),
			WithBaseDelay(500*time.Millisecond),
			WithMaxJitter(50*time.Millisecond),
			WithUserAgent("benchmark-agent/1.0"),
		)
	}
}

// BenchmarkJSONMarshalProject measures JSON marshaling of a Project.
func BenchmarkJSONMarshalProject(b *testing.B) {
	project := &Project{
		ID:             12345,
		Status:         "active",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Name:           "Benchmark Project",
		Description:    "A project used for benchmarking JSON serialization performance",
		Purpose:        "company_hq",
		ClientsEnabled: true,
		BookmarkURL:    "https://3.basecamp.com/12345/bookmarks/67890",
		URL:            "https://3.basecampapi.com/12345/projects/67890.json",
		AppURL:         "https://3.basecamp.com/12345/projects/67890",
		Dock: []DockItem{
			{ID: 1, Title: "Message Board", Name: "message_board", Enabled: true, URL: "https://example.com/1"},
			{ID: 2, Title: "To-dos", Name: "todoset", Enabled: true, URL: "https://example.com/2"},
			{ID: 3, Title: "Schedule", Name: "schedule", Enabled: true, URL: "https://example.com/3"},
		},
		Bookmarked: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(project)
	}
}

// BenchmarkJSONUnmarshalProject measures JSON unmarshaling of a Project.
func BenchmarkJSONUnmarshalProject(b *testing.B) {
	projectJSON := []byte(`{
		"id": 12345,
		"status": "active",
		"created_at": "2024-01-15T10:30:00Z",
		"updated_at": "2024-01-15T12:00:00Z",
		"name": "Benchmark Project",
		"description": "A project used for benchmarking JSON serialization performance",
		"purpose": "company_hq",
		"clients_enabled": true,
		"bookmark_url": "https://3.basecamp.com/12345/bookmarks/67890",
		"url": "https://3.basecampapi.com/12345/projects/67890.json",
		"app_url": "https://3.basecamp.com/12345/projects/67890",
		"dock": [
			{"id": 1, "title": "Message Board", "name": "message_board", "enabled": true, "url": "https://example.com/1"},
			{"id": 2, "title": "To-dos", "name": "todoset", "enabled": true, "url": "https://example.com/2"},
			{"id": 3, "title": "Schedule", "name": "schedule", "enabled": true, "url": "https://example.com/3"}
		],
		"bookmarked": true
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var p Project
		_ = json.Unmarshal(projectJSON, &p)
	}
}

// BenchmarkJSONMarshalTodo measures JSON marshaling of a Todo.
func BenchmarkJSONMarshalTodo(b *testing.B) {
	now := time.Now()
	todo := &Todo{
		ID:          12345,
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
		Title:       "Implement benchmark infrastructure",
		Content:     "Implement benchmark infrastructure",
		Description: "Write comprehensive benchmarks for the SDK to enable PGO",
		Completed:   false,
		Position:    1,
		DueOn:       "2024-12-31",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(todo)
	}
}

// BenchmarkJSONUnmarshalTodo measures JSON unmarshaling of a Todo.
func BenchmarkJSONUnmarshalTodo(b *testing.B) {
	todoJSON := []byte(`{
		"id": 12345,
		"status": "active",
		"created_at": "2024-01-15T10:30:00Z",
		"updated_at": "2024-01-15T12:00:00Z",
		"title": "Implement benchmark infrastructure",
		"content": "Implement benchmark infrastructure",
		"description": "Write comprehensive benchmarks for the SDK to enable PGO",
		"completed": false,
		"position": 1,
		"due_on": "2024-12-31"
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var t Todo
		_ = json.Unmarshal(todoJSON, &t)
	}
}

// BenchmarkJSONMarshalProjectList measures marshaling a list of projects.
func BenchmarkJSONMarshalProjectList(b *testing.B) {
	projects := make([]Project, 50)
	now := time.Now()
	for i := range projects {
		projects[i] = Project{
			ID:          int64(i + 1),
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
			Name:        "Project " + string(rune('A'+i%26)),
			Description: "Description for benchmark project",
			URL:         "https://example.com/projects/" + string(rune('A'+i%26)),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(projects)
	}
}

// BenchmarkJSONUnmarshalProjectList measures unmarshaling a list of projects.
func BenchmarkJSONUnmarshalProjectList(b *testing.B) {
	// Build JSON for 50 projects
	var buf bytes.Buffer
	buf.WriteString("[")
	for i := 0; i < 50; i++ {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(`{"id":`)
		buf.WriteString(string(rune('0' + i%10)))
		buf.WriteString(`,"status":"active","name":"Project `)
		buf.WriteString(string(rune('A' + i%26)))
		buf.WriteString(`","description":"Description","url":"https://example.com"}`)
	}
	buf.WriteString("]")
	projectsJSON := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var projects []Project
		_ = json.Unmarshal(projectsJSON, &projects)
	}
}

// BenchmarkBuildURL measures URL building performance.
func BenchmarkBuildURL(b *testing.B) {
	cfg := &Config{
		BaseURL:      "https://3.basecampapi.com",
		AccountID:    "12345",
		CacheEnabled: false,
	}
	client := NewClient(cfg, &mockTokenProvider{})

	paths := []string{
		"/projects.json",
		"/buckets/67890/todolists.json",
		"/buckets/67890/todos/11111.json",
		"/people.json",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		_ = client.buildURL(path)
	}
}

// BenchmarkParseNextLink measures Link header parsing.
func BenchmarkParseNextLink(b *testing.B) {
	linkHeaders := []string{
		`<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next", <https://3.basecampapi.com/12345/projects.json?page=10>; rel="last"`,
		`<https://3.basecampapi.com/12345/projects.json?page=1>; rel="first", <https://3.basecampapi.com/12345/projects.json?page=3>; rel="next"`,
		`<https://3.basecampapi.com/12345/projects.json?page=5>; rel="next"`,
		"", // no next link
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header := linkHeaders[i%len(linkHeaders)]
		_ = parseNextLink(header)
	}
}

// BenchmarkParseRetryAfter measures Retry-After header parsing.
func BenchmarkParseRetryAfter(b *testing.B) {
	headers := []string{
		"60",
		"120",
		"Wed, 21 Oct 2025 07:28:00 GMT",
		"",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header := headers[i%len(headers)]
		_ = parseRetryAfter(header)
	}
}

// BenchmarkHTTPRoundTrip measures full HTTP request/response cycle.
func BenchmarkHTTPRoundTrip(b *testing.B) {
	// Create a test server that returns a simple JSON response
	response := []byte(`{"id":12345,"status":"active","name":"Test Project"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	}))
	defer server.Close()

	cfg := &Config{
		BaseURL:      server.URL,
		AccountID:    "12345",
		CacheEnabled: false,
	}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Get(ctx, "/projects/12345.json")
	}
}

// BenchmarkHTTPRoundTripParallel measures concurrent HTTP performance.
func BenchmarkHTTPRoundTripParallel(b *testing.B) {
	response := []byte(`{"id":12345,"status":"active","name":"Test Project"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response)
	}))
	defer server.Close()

	cfg := &Config{
		BaseURL:      server.URL,
		AccountID:    "12345",
		CacheEnabled: false,
	}
	client := NewClient(cfg, &mockTokenProvider{})
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = client.Get(ctx, "/projects/12345.json")
		}
	})
}

// BenchmarkConfigLoad measures configuration loading.
func BenchmarkConfigLoad(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := DefaultConfig()
		cfg.LoadConfigFromEnv()
	}
}

// BenchmarkCacheKey measures cache key generation.
func BenchmarkCacheKey(b *testing.B) {
	cache := NewCache("/tmp/benchmark-cache")
	urls := []string{
		"https://3.basecampapi.com/12345/projects.json",
		"https://3.basecampapi.com/12345/buckets/67890/todos.json",
		"https://3.basecampapi.com/12345/people.json?page=2",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := urls[i%len(urls)]
		_ = cache.Key(url, "12345", "token-abc123")
	}
}

// BenchmarkErrorCreation measures error type creation.
func BenchmarkErrorCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch i % 5 {
		case 0:
			_ = ErrAPI(500, "Internal server error")
		case 1:
			_ = ErrNotFound("Project", "12345")
		case 2:
			_ = ErrAuth("Token expired")
		case 3:
			_ = ErrRateLimit(60)
		case 4:
			_ = ErrNetwork(io.EOF)
		}
	}
}

// BenchmarkBackoffDelay measures backoff calculation.
func BenchmarkBackoffDelay(b *testing.B) {
	cfg := &Config{
		BaseURL:      "https://3.basecampapi.com",
		AccountID:    "12345",
		CacheEnabled: false,
	}
	client := NewClient(cfg, &mockTokenProvider{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		attempt := (i % 5) + 1
		_ = client.backoffDelay(attempt)
	}
}
