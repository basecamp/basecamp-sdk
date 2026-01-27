package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// mockTokenProvider implements TokenProvider for testing.
type mockTokenProvider struct {
	token string
}

func (m *mockTokenProvider) AccessToken(ctx context.Context) (string, error) {
	return m.token, nil
}

func TestNewBridgeClient(t *testing.T) {
	cfg := &Config{
		BaseURL:      "https://api.example.com",
		AccountID:    "12345",
		CacheEnabled: false,
	}
	auth := &mockTokenProvider{token: "test-token"}

	client, err := NewBridgeClient(cfg, auth)
	if err != nil {
		t.Fatalf("NewBridgeClient failed: %v", err)
	}

	if client.Generated() == nil {
		t.Error("Generated client should not be nil")
	}

	if client.Config() != cfg {
		t.Error("Config should match input config")
	}
}

func TestNewBridgeClient_WithLogger(t *testing.T) {
	cfg := &Config{
		BaseURL:      "https://api.example.com",
		AccountID:    "12345",
		CacheEnabled: false,
	}
	auth := &mockTokenProvider{token: "test-token"}

	// Should not panic with nil logger
	_, err := NewBridgeClient(cfg, auth, WithBridgeLogger(nil))
	if err != nil {
		t.Fatalf("NewBridgeClient with nil logger failed: %v", err)
	}
}

func TestMapHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
		wantCode   string
	}{
		{"OK", http.StatusOK, false, ""},
		{"Created", http.StatusCreated, false, ""},
		{"NoContent", http.StatusNoContent, false, ""},
		{"Unauthorized", http.StatusUnauthorized, true, CodeAuth},
		{"Forbidden", http.StatusForbidden, true, CodeForbidden},
		{"NotFound", http.StatusNotFound, true, CodeNotFound},
		{"TooManyRequests", http.StatusTooManyRequests, true, CodeRateLimit},
		{"UnprocessableEntity", http.StatusUnprocessableEntity, true, CodeAPI},
		{"InternalServerError", http.StatusInternalServerError, true, CodeAPI},
		{"BadGateway", http.StatusBadGateway, true, CodeAPI},
		{"ServiceUnavailable", http.StatusServiceUnavailable, true, CodeAPI},
		{"GatewayTimeout", http.StatusGatewayTimeout, true, CodeAPI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com/test", nil)
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Request:    req,
			}
			err := MapHTTPError(resp)

			if tt.wantErr {
				if err == nil {
					t.Errorf("MapHTTPError() expected error for status %d", tt.statusCode)
				}
				if apiErr, ok := err.(*Error); ok {
					if apiErr.Code != tt.wantCode {
						t.Errorf("MapHTTPError() code = %v, want %v", apiErr.Code, tt.wantCode)
					}
				}
			} else {
				if err != nil {
					t.Errorf("MapHTTPError() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDerefHelpers(t *testing.T) {
	// Test DerefString
	s := "hello"
	if DerefString(&s) != "hello" {
		t.Error("DerefString failed for non-nil")
	}
	if DerefString(nil) != "" {
		t.Error("DerefString failed for nil")
	}

	// Test DerefInt64
	i64 := int64(9007199254740993)
	if DerefInt64(&i64) != 9007199254740993 {
		t.Error("DerefInt64 failed for non-nil")
	}
	if DerefInt64(nil) != 0 {
		t.Error("DerefInt64 failed for nil")
	}

	// Test DerefBool
	b := true
	if DerefBool(&b) != true {
		t.Error("DerefBool failed for non-nil")
	}
	if DerefBool(nil) != false {
		t.Error("DerefBool failed for nil")
	}

	// Test DerefInt
	i := 42
	if DerefInt(&i) != 42 {
		t.Error("DerefInt failed for non-nil")
	}
	if DerefInt(nil) != 0 {
		t.Error("DerefInt failed for nil")
	}
}

func TestPtrHelpers(t *testing.T) {
	s := PtrString("test")
	if *s != "test" {
		t.Error("PtrString failed")
	}

	i64 := PtrInt64(9007199254740993)
	if *i64 != 9007199254740993 {
		t.Error("PtrInt64 failed")
	}

	b := PtrBool(true)
	if *b != true {
		t.Error("PtrBool failed")
	}
}

func TestParseTimestamp(t *testing.T) {
	// Valid RFC3339
	ts := "2024-01-15T10:30:00Z"
	result := ParseTimestamp(&ts)
	if result.IsZero() {
		t.Error("ParseTimestamp failed for valid RFC3339")
	}
	if result.Year() != 2024 || result.Month() != 1 || result.Day() != 15 {
		t.Errorf("ParseTimestamp returned wrong date: %v", result)
	}

	// Valid without timezone
	ts2 := "2024-01-15T10:30:00"
	result2 := ParseTimestamp(&ts2)
	if result2.IsZero() {
		t.Error("ParseTimestamp failed for timestamp without timezone")
	}

	// Invalid
	invalid := "not-a-date"
	result3 := ParseTimestamp(&invalid)
	if !result3.IsZero() {
		t.Error("ParseTimestamp should return zero time for invalid input")
	}

	// Nil
	result4 := ParseTimestamp(nil)
	if !result4.IsZero() {
		t.Error("ParseTimestamp should return zero time for nil input")
	}

	// Empty
	empty := ""
	result5 := ParseTimestamp(&empty)
	if !result5.IsZero() {
		t.Error("ParseTimestamp should return zero time for empty input")
	}
}

func TestParseTimestampPtr(t *testing.T) {
	ts := "2024-01-15T10:30:00Z"
	result := ParseTimestampPtr(&ts)
	if result == nil {
		t.Error("ParseTimestampPtr should return non-nil for valid input")
	}

	empty := ""
	result2 := ParseTimestampPtr(&empty)
	if result2 != nil {
		t.Error("ParseTimestampPtr should return nil for empty input")
	}

	result3 := ParseTimestampPtr(nil)
	if result3 != nil {
		t.Error("ParseTimestampPtr should return nil for nil input")
	}
}

func TestPersonFromGenerated(t *testing.T) {
	id := int64(123)
	name := "John Doe"
	email := "john@example.com"
	admin := true
	companyId := int64(456)
	companyName := "Acme Inc"

	g := &generated.Person{
		Id:           &id,
		Name:         &name,
		EmailAddress: &email,
		Admin:        &admin,
		Company: &generated.PersonCompany{
			Id:   &companyId,
			Name: &companyName,
		},
	}

	p := PersonFromGenerated(g)

	if p == nil {
		t.Fatal("PersonFromGenerated returned nil")
	}
	if p.ID != 123 {
		t.Errorf("ID = %d, want 123", p.ID)
	}
	if p.Name != "John Doe" {
		t.Errorf("Name = %s, want John Doe", p.Name)
	}
	if p.EmailAddress != "john@example.com" {
		t.Errorf("EmailAddress = %s, want john@example.com", p.EmailAddress)
	}
	if p.Admin != true {
		t.Error("Admin should be true")
	}
	if p.Company == nil {
		t.Fatal("Company should not be nil")
	}
	if p.Company.ID != 456 {
		t.Errorf("Company.ID = %d, want 456", p.Company.ID)
	}
	if p.Company.Name != "Acme Inc" {
		t.Errorf("Company.Name = %s, want Acme Inc", p.Company.Name)
	}
}

func TestPersonFromGenerated_Nil(t *testing.T) {
	p := PersonFromGenerated(nil)
	if p != nil {
		t.Error("PersonFromGenerated(nil) should return nil")
	}
}

func TestTodoFromGenerated(t *testing.T) {
	id := int64(789)
	title := "Test Todo"
	content := "Do something"
	completed := true
	status := "active"
	createdAt := "2024-01-15T10:30:00Z"

	creatorId := int64(123)
	creatorName := "Jane Doe"

	g := &generated.Todo{
		Id:        &id,
		Title:     &title,
		Content:   &content,
		Completed: &completed,
		Status:    &status,
		CreatedAt: &createdAt,
		Creator: &generated.Person{
			Id:   &creatorId,
			Name: &creatorName,
		},
	}

	todo := TodoFromGenerated(g)

	if todo == nil {
		t.Fatal("TodoFromGenerated returned nil")
	}
	if todo.ID != 789 {
		t.Errorf("ID = %d, want 789", todo.ID)
	}
	if todo.Title != "Test Todo" {
		t.Errorf("Title = %s, want Test Todo", todo.Title)
	}
	if todo.Content != "Do something" {
		t.Errorf("Content = %s, want Do something", todo.Content)
	}
	if todo.Completed != true {
		t.Error("Completed should be true")
	}
	if todo.Status != "active" {
		t.Errorf("Status = %s, want active", todo.Status)
	}
	if todo.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if todo.Creator == nil {
		t.Fatal("Creator should not be nil")
	}
	if todo.Creator.Name != "Jane Doe" {
		t.Errorf("Creator.Name = %s, want Jane Doe", todo.Creator.Name)
	}
}

func TestProjectFromGenerated(t *testing.T) {
	id := int64(999)
	name := "Test Project"
	description := "A test project"
	status := "active"
	bookmarked := true
	createdAt := "2024-01-15T10:30:00Z"

	dockId := int64(1)
	dockTitle := "Messages"
	dockEnabled := true

	g := &generated.Project{
		Id:          &id,
		Name:        &name,
		Description: &description,
		Status:      &status,
		Bookmarked:  &bookmarked,
		CreatedAt:   &createdAt,
		Dock: &[]generated.DockItem{
			{
				Id:      &dockId,
				Title:   &dockTitle,
				Enabled: &dockEnabled,
			},
		},
	}

	p := ProjectFromGenerated(g)

	if p == nil {
		t.Fatal("ProjectFromGenerated returned nil")
	}
	if p.ID != 999 {
		t.Errorf("ID = %d, want 999", p.ID)
	}
	if p.Name != "Test Project" {
		t.Errorf("Name = %s, want Test Project", p.Name)
	}
	if p.Bookmarked != true {
		t.Error("Bookmarked should be true")
	}
	if len(p.Dock) != 1 {
		t.Fatalf("Dock length = %d, want 1", len(p.Dock))
	}
	if p.Dock[0].Title != "Messages" {
		t.Errorf("Dock[0].Title = %s, want Messages", p.Dock[0].Title)
	}
}

func TestSliceConverters(t *testing.T) {
	// Test TodosFromGenerated
	id1 := int64(1)
	id2 := int64(2)
	title1 := "Todo 1"
	title2 := "Todo 2"

	todos := &[]generated.Todo{
		{Id: &id1, Title: &title1},
		{Id: &id2, Title: &title2},
	}

	result := TodosFromGenerated(todos)
	if len(result) != 2 {
		t.Fatalf("TodosFromGenerated length = %d, want 2", len(result))
	}
	if result[0].Title != "Todo 1" {
		t.Errorf("result[0].Title = %s, want Todo 1", result[0].Title)
	}
	if result[1].Title != "Todo 2" {
		t.Errorf("result[1].Title = %s, want Todo 2", result[1].Title)
	}

	// Test nil input
	if TodosFromGenerated(nil) != nil {
		t.Error("TodosFromGenerated(nil) should return nil")
	}
}

func TestIsOperationIdempotent(t *testing.T) {
	// UpdateTodo should be idempotent based on the Smithy spec
	if !IsOperationIdempotent("UpdateTodo") {
		t.Error("UpdateTodo should be idempotent")
	}

	// CreateTodo should not be idempotent
	if IsOperationIdempotent("CreateTodo") {
		t.Error("CreateTodo should not be idempotent")
	}

	// Unknown operations should return false
	if IsOperationIdempotent("UnknownOperation") {
		t.Error("Unknown operations should not be idempotent")
	}
}

func TestGetOperationMetadata(t *testing.T) {
	meta, ok := GetOperationMetadata("UpdateTodo")
	if !ok {
		t.Error("GetOperationMetadata should find UpdateTodo")
	}
	if !meta.Idempotent {
		t.Error("UpdateTodo should be marked as idempotent")
	}

	_, ok = GetOperationMetadata("NonExistentOperation")
	if ok {
		t.Error("GetOperationMetadata should not find NonExistentOperation")
	}
}

func TestCachingTransport(t *testing.T) {
	// Create a test server that returns an ETag
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"abc123"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"abc123"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"test": "data"}`))
	}))
	defer server.Close()

	cfg := &Config{
		BaseURL:      server.URL,
		AccountID:    "12345",
		CacheEnabled: true,
		CacheDir:     t.TempDir(),
	}
	auth := &mockTokenProvider{token: "test-token"}
	cache := NewCache(cfg.CacheDir)

	transport := &cachingTransport{
		base:   http.DefaultTransport,
		cache:  cache,
		cfg:    cfg,
		auth:   auth,
		logger: nil,
	}

	client := &http.Client{Transport: transport}

	// First request should get cached
	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("First request status = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Give cache time to write
	time.Sleep(10 * time.Millisecond)

	// Second request should use cache (304 -> 200)
	req2, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	// Status should be 200 (our transport converts 304 to 200 with cached body)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Second request status = %d, want 200", resp2.StatusCode)
	}
	_ = resp2.Body.Close()
}
