// Package basecamp provides a Go SDK for the Basecamp 3 API.
//
// This file implements the bridge pattern connecting generated code
// (from oapi-codegen) with hand-written runtime behaviors (auth, cache, retry).
package basecamp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// BridgeClient wraps the generated client with hand-written runtime behaviors.
// It provides the connection between generated HTTP operations and the
// hand-written authentication, caching, and error handling logic.
type BridgeClient struct {
	gen     *generated.ClientWithResponses
	cfg     *Config
	auth    TokenProvider
	cache   *Cache
	logger  *slog.Logger
	rawHTTP *http.Client
}

// BridgeOption configures a BridgeClient.
type BridgeOption func(*BridgeClient)

// WithBridgeLogger sets a custom logger for the bridge client.
func WithBridgeLogger(l *slog.Logger) BridgeOption {
	return func(b *BridgeClient) {
		if l != nil {
			b.logger = l
		}
	}
}

// WithBridgeCache sets a custom cache for the bridge client.
func WithBridgeCache(c *Cache) BridgeOption {
	return func(b *BridgeClient) {
		b.cache = c
	}
}

// NewBridgeClient creates a new bridge client that wraps the generated
// client with hand-written runtime behaviors.
func NewBridgeClient(cfg *Config, auth TokenProvider, opts ...BridgeOption) (*BridgeClient, error) {
	b := &BridgeClient{
		cfg:    cfg,
		auth:   auth,
		logger: slog.New(discardHandler{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(b)
	}

	// Initialize cache if enabled and not overridden
	if b.cache == nil && cfg.CacheEnabled {
		b.cache = NewCache(cfg.CacheDir)
	}

	// Create HTTP client with caching transport
	b.rawHTTP = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cachingTransport{
			base:   http.DefaultTransport,
			cache:  b.cache,
			cfg:    cfg,
			auth:   auth,
			logger: b.logger,
		},
	}

	// Build server URL with account ID
	serverURL := cfg.BaseURL
	if cfg.AccountID != "" {
		serverURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(cfg.BaseURL, "/"), cfg.AccountID)
	}

	// Create generated client with our custom HTTP client and auth editor
	genClient, err := generated.NewClientWithResponses(
		serverURL,
		generated.WithHTTPClient(b.rawHTTP),
		generated.WithRequestEditorFn(b.authEditor),
		generated.WithLogger(b.logger),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generated client: %w", err)
	}

	b.gen = genClient
	return b, nil
}

// authEditor injects authentication and standard headers into requests.
func (b *BridgeClient) authEditor(ctx context.Context, req *http.Request) error {
	token, err := b.auth.AccessToken(ctx)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "application/json")

	return nil
}

// Generated returns the underlying generated client for direct access
// when the bridge layer isn't needed.
func (b *BridgeClient) Generated() *generated.ClientWithResponses {
	return b.gen
}

// Config returns the configuration used by this bridge client.
func (b *BridgeClient) Config() *Config {
	return b.cfg
}

// ============================================================================
// Caching Transport
// ============================================================================

// cachingTransport wraps http.RoundTripper with ETag caching.
type cachingTransport struct {
	base   http.RoundTripper
	cache  *Cache
	cfg    *Config
	auth   TokenProvider
	logger *slog.Logger
}

func (t *cachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only cache GET requests
	if req.Method != "GET" || t.cache == nil {
		return t.base.RoundTrip(req)
	}

	// Get token for cache key (we need this for per-user caching)
	token, _ := t.auth.AccessToken(req.Context())

	cacheKey := t.cache.Key(req.URL.String(), t.cfg.AccountID, token)

	// Add If-None-Match header if we have a cached ETag
	if etag := t.cache.GetETag(cacheKey); etag != "" {
		req.Header.Set("If-None-Match", etag)
		if t.logger != nil {
			t.logger.Debug("cache conditional request", "etag", etag)
		}
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Handle 304 Not Modified
	if resp.StatusCode == http.StatusNotModified {
		if t.logger != nil {
			t.logger.Debug("cache hit", "status", 304)
		}
		if cached := t.cache.GetBody(cacheKey); cached != nil {
			// Replace response body with cached content
			_ = resp.Body.Close()
			resp.Body = io.NopCloser(strings.NewReader(string(cached)))
			resp.StatusCode = http.StatusOK
			resp.Status = "200 OK (cached)"
		}
		return resp, nil
	}

	// Cache successful responses with ETags
	if resp.StatusCode == http.StatusOK {
		if etag := resp.Header.Get("ETag"); etag != "" {
			// Read and cache the body
			body, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				return nil, err
			}

			_ = t.cache.Set(cacheKey, body, etag)
			if t.logger != nil {
				t.logger.Debug("cache stored", "etag", etag)
			}

			// Replace body with buffer
			resp.Body = io.NopCloser(strings.NewReader(string(body)))
		}
	}

	return resp, nil
}

// ============================================================================
// Response Handling Helpers
// ============================================================================

// MapHTTPError converts HTTP response status codes to structured errors.
// This is exported so services can use it when processing generated responses.
func MapHTTPError(resp *http.Response) error {
	if resp == nil {
		return nil
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrAuth("Authentication failed")
	case http.StatusForbidden:
		return ErrForbidden("Access denied")
	case http.StatusNotFound:
		path := ""
		if resp.Request != nil {
			path = resp.Request.URL.Path
		}
		return ErrNotFound("Resource", path)
	case http.StatusTooManyRequests:
		return ErrRateLimit(parseRetryAfter(resp.Header.Get("Retry-After")))
	case http.StatusUnprocessableEntity:
		return ErrAPI(422, "Validation failed")
	case http.StatusInternalServerError:
		return ErrAPI(500, "Server error")
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return &Error{
			Code:       CodeAPI,
			Message:    fmt.Sprintf("Gateway error (%d)", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Retryable:  true,
		}
	default:
		if resp.StatusCode >= 400 {
			return ErrAPI(resp.StatusCode, fmt.Sprintf("Request failed (HTTP %d)", resp.StatusCode))
		}
		return nil
	}
}

// ============================================================================
// Type Conversion Helpers
// ============================================================================

// DerefString safely dereferences a string pointer, returning empty string if nil.
func DerefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// DerefInt64 safely dereferences an int64 pointer, returning 0 if nil.
func DerefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// DerefInt32 safely dereferences an int32 pointer, returning 0 if nil.
func DerefInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

// DerefBool safely dereferences a bool pointer, returning false if nil.
func DerefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

// DerefInt safely dereferences an int pointer, returning 0 if nil.
func DerefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// PtrString returns a pointer to the given string.
func PtrString(s string) *string {
	return &s
}

// PtrInt64 returns a pointer to the given int64.
func PtrInt64(i int64) *int64 {
	return &i
}

// PtrBool returns a pointer to the given bool.
func PtrBool(b bool) *bool {
	return &b
}

// ParseTimestamp parses an ISO8601 timestamp string into time.Time.
// Returns zero time if parsing fails or input is nil.
func ParseTimestamp(s *string) time.Time {
	if s == nil || *s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		// Try parsing without timezone
		t, err = time.Parse("2006-01-02T15:04:05", *s)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

// ParseTimestampPtr parses an ISO8601 timestamp string into *time.Time.
// Returns nil if parsing fails or input is nil/empty.
func ParseTimestampPtr(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t := ParseTimestamp(s)
	if t.IsZero() {
		return nil
	}
	return &t
}

// ============================================================================
// Domain Type Converters
// ============================================================================

// TodoFromGenerated converts a generated Todo to a domain Todo.
func TodoFromGenerated(g *generated.Todo) *Todo {
	if g == nil {
		return nil
	}

	t := &Todo{
		ID:          DerefInt64(g.Id),
		Status:      DerefString(g.Status),
		Title:       DerefString(g.Title),
		Content:     DerefString(g.Content),
		Description: DerefString(g.Description),
		Completed:   DerefBool(g.Completed),
		URL:         DerefString(g.Url),
		AppURL:      DerefString(g.AppUrl),
		BookmarkURL: DerefString(g.BookmarkUrl),
		DueOn:       DerefString(g.DueOn),
		StartsOn:    DerefString(g.StartsOn),
		Position:    int(DerefInt32(g.Position)),
		InheritsVis: DerefBool(g.InheritsStatus),
		Type:        DerefString(g.Type),
		CreatedAt:   ParseTimestamp(g.CreatedAt),
		UpdatedAt:   ParseTimestamp(g.UpdatedAt),
	}

	// Convert nested types
	if g.Creator != nil {
		t.Creator = PersonFromGenerated(g.Creator)
	}
	if g.Parent != nil {
		t.Parent = ParentFromGenerated(g.Parent)
	}
	if g.Bucket != nil {
		t.Bucket = BucketFromGenerated(g.Bucket)
	}
	if g.Assignees != nil {
		t.Assignees = make([]Person, len(*g.Assignees))
		for i, a := range *g.Assignees {
			if p := PersonFromGenerated(&a); p != nil {
				t.Assignees[i] = *p
			}
		}
	}

	return t
}

// PersonFromGenerated converts a generated Person to a domain Person.
func PersonFromGenerated(g *generated.Person) *Person {
	if g == nil {
		return nil
	}

	p := &Person{
		ID:             DerefInt64(g.Id),
		Name:           DerefString(g.Name),
		EmailAddress:   DerefString(g.EmailAddress),
		PersonableType: DerefString(g.PersonableType),
		Title:          DerefString(g.Title),
		Bio:            DerefString(g.Bio),
		Location:       DerefString(g.Location),
		CreatedAt:      DerefString(g.CreatedAt),
		UpdatedAt:      DerefString(g.UpdatedAt),
		Admin:          DerefBool(g.Admin),
		Owner:          DerefBool(g.Owner),
		Client:         DerefBool(g.Client),
		TimeZone:       DerefString(g.TimeZone),
		AvatarURL:      DerefString(g.AvatarUrl),
		AttachableSGID: DerefString(g.AttachableSgid),
	}

	// Convert company if present
	if g.Company != nil {
		p.Company = &PersonCompany{
			ID:   DerefInt64(g.Company.Id),
			Name: DerefString(g.Company.Name),
		}
	}

	return p
}

// ParentFromGenerated converts a generated TodoParent to a domain Parent.
func ParentFromGenerated(g *generated.TodoParent) *Parent {
	if g == nil {
		return nil
	}

	return &Parent{
		ID:     DerefInt64(g.Id),
		Title:  DerefString(g.Title),
		Type:   DerefString(g.Type),
		URL:    DerefString(g.Url),
		AppURL: DerefString(g.AppUrl),
	}
}

// BucketFromGenerated converts a generated TodoBucket to a domain Bucket.
func BucketFromGenerated(g *generated.TodoBucket) *Bucket {
	if g == nil {
		return nil
	}

	return &Bucket{
		ID:   DerefInt64(g.Id),
		Name: DerefString(g.Name),
		Type: DerefString(g.Type),
	}
}

// ProjectFromGenerated converts a generated Project to a domain Project.
func ProjectFromGenerated(g *generated.Project) *Project {
	if g == nil {
		return nil
	}

	p := &Project{
		ID:             DerefInt64(g.Id),
		Name:           DerefString(g.Name),
		Description:    DerefString(g.Description),
		Status:         DerefString(g.Status),
		Purpose:        DerefString(g.Purpose),
		ClientsEnabled: DerefBool(g.ClientsEnabled),
		URL:            DerefString(g.Url),
		AppURL:         DerefString(g.AppUrl),
		BookmarkURL:    DerefString(g.BookmarkUrl),
		Bookmarked:     DerefBool(g.Bookmarked),
		CreatedAt:      ParseTimestamp(g.CreatedAt),
		UpdatedAt:      ParseTimestamp(g.UpdatedAt),
	}

	// Convert dock items
	if g.Dock != nil {
		p.Dock = make([]DockItem, len(*g.Dock))
		for i, d := range *g.Dock {
			p.Dock[i] = DockItem{
				ID:      DerefInt64(d.Id),
				Title:   DerefString(d.Title),
				Name:    DerefString(d.Name),
				Enabled: DerefBool(d.Enabled),
				URL:     DerefString(d.Url),
				AppURL:  DerefString(d.AppUrl),
			}
			if d.Position != nil {
				pos := int(DerefInt32(d.Position))
				p.Dock[i].Position = &pos
			}
		}
	}

	// Convert client company
	if g.ClientCompany != nil {
		p.ClientCompany = &ClientCompany{
			ID:   DerefInt64(g.ClientCompany.Id),
			Name: DerefString(g.ClientCompany.Name),
		}
	}

	// Convert clientside (deprecated)
	if g.Clientside != nil {
		p.Clientside = &Clientside{
			URL:    DerefString(g.Clientside.Url),
			AppURL: DerefString(g.Clientside.AppUrl),
		}
	}

	return p
}

// ============================================================================
// Slice Converters
// ============================================================================

// TodosFromGenerated converts a slice of generated Todos to domain Todos.
func TodosFromGenerated(gs *[]generated.Todo) []Todo {
	if gs == nil {
		return nil
	}

	result := make([]Todo, len(*gs))
	for i, g := range *gs {
		if t := TodoFromGenerated(&g); t != nil {
			result[i] = *t
		}
	}
	return result
}

// ProjectsFromGenerated converts a slice of generated Projects to domain Projects.
func ProjectsFromGenerated(gs *[]generated.Project) []Project {
	if gs == nil {
		return nil
	}

	result := make([]Project, len(*gs))
	for i, g := range *gs {
		if p := ProjectFromGenerated(&g); p != nil {
			result[i] = *p
		}
	}
	return result
}

// PeopleFromGenerated converts a slice of generated Persons to domain Persons.
func PeopleFromGenerated(gs *[]generated.Person) []Person {
	if gs == nil {
		return nil
	}

	result := make([]Person, len(*gs))
	for i, g := range *gs {
		if p := PersonFromGenerated(&g); p != nil {
			result[i] = *p
		}
	}
	return result
}

// ============================================================================
// Operation Metadata Access
// ============================================================================

// IsOperationIdempotent returns whether the given operation is idempotent
// and safe to retry based on the generated OperationMetadata.
func IsOperationIdempotent(operationId string) bool {
	return generated.IsIdempotent(operationId)
}

// GetOperationMetadata returns metadata for the given operation ID,
// including idempotency and sensitive parameter information.
func GetOperationMetadata(operationId string) (generated.OperationMetadata, bool) {
	return generated.GetOperationMetadata(operationId)
}

// ============================================================================
// Bridge Service Methods
// ============================================================================

// BridgeListProjects uses the bridge to list projects with domain types.
func (b *BridgeClient) BridgeListProjects(ctx context.Context, status *string) ([]Project, error) {
	params := &generated.ListProjectsParams{
		Status: status,
	}

	resp, err := b.gen.ListProjectsWithResponse(ctx, params)
	if err != nil {
		return nil, ErrNetwork(err)
	}

	if err := MapHTTPError(resp.HTTPResponse); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, ErrAPI(resp.StatusCode(), "unexpected response format")
	}

	return ProjectsFromGenerated(resp.JSON200.Projects), nil
}

// BridgeGetProject uses the bridge to get a single project.
func (b *BridgeClient) BridgeGetProject(ctx context.Context, projectId int64) (*Project, error) {
	resp, err := b.gen.GetProjectWithResponse(ctx, projectId)
	if err != nil {
		return nil, ErrNetwork(err)
	}

	if err := MapHTTPError(resp.HTTPResponse); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, ErrAPI(resp.StatusCode(), "unexpected response format")
	}

	return ProjectFromGenerated(resp.JSON200.Project), nil
}

// BridgeListTodos uses the bridge to list todos with domain types.
func (b *BridgeClient) BridgeListTodos(ctx context.Context, projectId, todolistId int64, status *string) ([]Todo, error) {
	params := &generated.ListTodosParams{
		Status: status,
	}

	resp, err := b.gen.ListTodosWithResponse(ctx, projectId, todolistId, params)
	if err != nil {
		return nil, ErrNetwork(err)
	}

	if err := MapHTTPError(resp.HTTPResponse); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, ErrAPI(resp.StatusCode(), "unexpected response format")
	}

	return TodosFromGenerated(resp.JSON200.Todos), nil
}

// BridgeGetTodo uses the bridge to get a single todo.
func (b *BridgeClient) BridgeGetTodo(ctx context.Context, projectId, todoId int64) (*Todo, error) {
	resp, err := b.gen.GetTodoWithResponse(ctx, projectId, todoId)
	if err != nil {
		return nil, ErrNetwork(err)
	}

	if err := MapHTTPError(resp.HTTPResponse); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, ErrAPI(resp.StatusCode(), "unexpected response format")
	}

	return TodoFromGenerated(resp.JSON200.Todo), nil
}

// BridgeCompleteTodo uses the bridge to mark a todo as complete.
func (b *BridgeClient) BridgeCompleteTodo(ctx context.Context, projectId, todoId int64) error {
	resp, err := b.gen.CompleteTodoWithResponse(ctx, projectId, todoId)
	if err != nil {
		return ErrNetwork(err)
	}

	return MapHTTPError(resp.HTTPResponse)
}

// BridgeUncompleteTodo uses the bridge to mark a todo as incomplete.
func (b *BridgeClient) BridgeUncompleteTodo(ctx context.Context, projectId, todoId int64) error {
	resp, err := b.gen.UncompleteTodoWithResponse(ctx, projectId, todoId)
	if err != nil {
		return ErrNetwork(err)
	}

	return MapHTTPError(resp.HTTPResponse)
}
