package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	maxRetries = 5
	baseDelay  = 1 * time.Second
	maxJitter  = 100 * time.Millisecond

	// DefaultUserAgent is the default User-Agent header value.
	DefaultUserAgent = "basecamp-sdk-go/1.0"
)

// Client is an HTTP client for the Basecamp API.
type Client struct {
	httpClient    *http.Client
	tokenProvider TokenProvider
	cfg           *Config
	cache         *Cache
	userAgent     string
	verbose       bool

	// Services
	projects *ProjectsService
	todos    *TodosService
}

// Response wraps an API response.
type Response struct {
	Data       json.RawMessage
	StatusCode int
	Headers    http.Header
	FromCache  bool
}

// UnmarshalData unmarshals the response data into the given value.
func (r *Response) UnmarshalData(v any) error {
	return json.Unmarshal(r.Data, v)
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = c
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) ClientOption {
	return func(client *Client) {
		client.userAgent = ua
	}
}

// WithVerbose enables verbose output for debugging.
func WithVerbose(v bool) ClientOption {
	return func(client *Client) {
		client.verbose = v
	}
}

// WithCache sets a custom cache.
func WithCache(cache *Cache) ClientOption {
	return func(client *Client) {
		client.cache = cache
	}
}

// NewClient creates a new API client.
func NewClient(cfg *Config, tokenProvider TokenProvider, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		tokenProvider: tokenProvider,
		cfg:           cfg,
		userAgent:     DefaultUserAgent,
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Initialize cache if enabled and not overridden
	if c.cache == nil && cfg.CacheEnabled {
		c.cache = NewCache(cfg.CacheDir)
	}

	return c
}

// SetVerbose enables or disables verbose output.
func (c *Client) SetVerbose(v bool) {
	c.verbose = v
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string) (*Response, error) {
	return c.doRequest(ctx, "GET", path, nil)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (*Response, error) {
	return c.doRequest(ctx, "POST", path, body)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) (*Response, error) {
	return c.doRequest(ctx, "PUT", path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (*Response, error) {
	return c.doRequest(ctx, "DELETE", path, nil)
}

// GetAll fetches all pages for a paginated resource.
func (c *Client) GetAll(ctx context.Context, path string) ([]json.RawMessage, error) {
	var allResults []json.RawMessage
	url := c.buildURL(path)
	maxPages := 10000
	page := 0

	for page = 1; page <= maxPages; page++ {
		resp, err := c.doRequestURL(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		// Parse response as array
		var items []json.RawMessage
		if err := json.Unmarshal(resp.Data, &items); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		allResults = append(allResults, items...)

		// Check for next page
		nextURL := parseNextLink(resp.Headers.Get("Link"))
		if nextURL == "" {
			break
		}
		url = nextURL
	}

	if page > maxPages {
		fmt.Fprintf(os.Stderr, "[basecamp-sdk] Warning: pagination capped at %d pages; results may be incomplete\n", maxPages)
	}

	return allResults, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*Response, error) {
	url := c.buildURL(path)
	return c.doRequestURL(ctx, method, url, body)
}

func (c *Client) doRequestURL(ctx context.Context, method, url string, body any) (*Response, error) {
	var attempt int
	var lastErr error

	for attempt = 1; attempt <= maxRetries; attempt++ {
		resp, err := c.singleRequest(ctx, method, url, body, attempt)
		if err == nil {
			return resp, nil
		}

		// Check if error is retryable
		if apiErr, ok := err.(*Error); ok {
			if !apiErr.Retryable {
				return nil, err
			}
			lastErr = err

			// Calculate backoff delay
			delay := c.backoffDelay(attempt)
			if c.verbose {
				fmt.Printf("[basecamp-sdk] Retry %d/%d in %v: %s\n", attempt, maxRetries, delay, err)
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				continue
			}
		}

		return nil, err
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}

func (c *Client) singleRequest(ctx context.Context, method, url string, body any, attempt int) (*Response, error) {
	// Get access token
	token, err := c.tokenProvider.AccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Build request body
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add ETag for cached GET requests
	var cacheKey string
	if method == "GET" && c.cache != nil {
		cacheKey = c.cache.Key(url, c.cfg.AccountID, token)
		if etag := c.cache.GetETag(cacheKey); etag != "" {
			req.Header.Set("If-None-Match", etag)
			if c.verbose {
				fmt.Printf("[basecamp-sdk] Cache: If-None-Match %s\n", etag)
			}
		}
	}

	if c.verbose {
		fmt.Printf("[basecamp-sdk] %s %s (attempt %d)\n", method, url, attempt)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrNetwork(err)
	}
	defer resp.Body.Close()

	if c.verbose {
		fmt.Printf("[basecamp-sdk] HTTP %d\n", resp.StatusCode)
	}

	// Handle response based on status code
	switch resp.StatusCode {
	case http.StatusNotModified: // 304
		if cacheKey != "" {
			if c.verbose {
				fmt.Println("[basecamp-sdk] Cache hit: 304 Not Modified")
			}
			cached := c.cache.GetBody(cacheKey)
			if cached != nil {
				return &Response{
					Data:       cached,
					StatusCode: http.StatusOK,
					Headers:    resp.Header,
					FromCache:  true,
				}, nil
			}
		}
		return nil, ErrAPI(304, "304 received but no cached response available")

	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// Cache GET responses with ETag
		if method == "GET" && cacheKey != "" {
			if etag := resp.Header.Get("ETag"); etag != "" {
				c.cache.Set(cacheKey, respBody, etag)
				if c.verbose {
					fmt.Printf("[basecamp-sdk] Cache: stored with ETag %s\n", etag)
				}
			}
		}

		return &Response{
			Data:       respBody,
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
		}, nil

	case http.StatusTooManyRequests: // 429
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, ErrRateLimit(retryAfter)

	case http.StatusUnauthorized: // 401
		// Try token refresh on first 401
		if attempt == 1 {
			if authMgr, ok := c.tokenProvider.(*AuthManager); ok {
				if err := authMgr.Refresh(ctx); err == nil {
					// Retry with new token
					return nil, &Error{
						Code:      CodeAuth,
						Message:   "Token refreshed",
						Retryable: true,
					}
				}
			}
		}
		return nil, ErrAuth("Authentication failed")

	case http.StatusForbidden: // 403
		// Check if this might be a scope issue
		if method != "GET" {
			return nil, ErrForbiddenScope()
		}
		return nil, ErrForbidden("Access denied")

	case http.StatusNotFound: // 404
		return nil, ErrNotFound("Resource", url)

	case http.StatusInternalServerError: // 500
		return nil, ErrAPI(500, "Server error (500)")

	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout: // 502, 503, 504
		return nil, &Error{
			Code:       CodeAPI,
			Message:    fmt.Sprintf("Gateway error (%d)", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Retryable:  true,
		}

	default:
		respBody, _ := io.ReadAll(resp.Body)
		var apiErr struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(respBody, &apiErr) == nil {
			msg := apiErr.Error
			if msg == "" {
				msg = apiErr.Message
			}
			if msg != "" {
				return nil, ErrAPI(resp.StatusCode, msg)
			}
		}
		return nil, ErrAPI(resp.StatusCode, fmt.Sprintf("Request failed (HTTP %d)", resp.StatusCode))
	}
}

func (c *Client) buildURL(path string) string {
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// If path already has account ID prefix, use it directly
	if strings.HasPrefix(path, "/"+c.cfg.AccountID+"/") {
		return c.cfg.BaseURL + path
	}

	// Check if this is an account-relative path (most API calls)
	// Skip account ID for authorization endpoints
	if strings.HasPrefix(path, "/.well-known/") || strings.HasPrefix(path, "/authorization/") {
		return c.cfg.BaseURL + path
	}

	// Add account ID for regular API paths
	if c.cfg.AccountID != "" {
		return c.cfg.BaseURL + "/" + c.cfg.AccountID + path
	}

	return c.cfg.BaseURL + path
}

func (c *Client) backoffDelay(attempt int) time.Duration {
	// Exponential backoff: base * 2^(attempt-1)
	delay := baseDelay * time.Duration(1<<(attempt-1))

	// Add jitter (0-100ms)
	jitter := time.Duration(rand.Int63n(int64(maxJitter)))

	return delay + jitter
}

// parseNextLink extracts the next URL from a Link header.
func parseNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			// Extract URL between < and >
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start >= 0 && end > start {
				return part[start+1 : end]
			}
		}
	}

	return ""
}

// parseRetryAfter parses the Retry-After header value.
func parseRetryAfter(header string) int {
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		return seconds
	}
	return 0
}

// ProjectPath builds a path relative to a project (bucket).
func (c *Client) ProjectPath(resource string) string {
	if c.cfg.ProjectID == "" {
		return ""
	}
	return "/buckets/" + c.cfg.ProjectID + resource
}

// RequireProject returns an error if no project is configured.
func (c *Client) RequireProject() error {
	if c.cfg.ProjectID == "" {
		return ErrUsageHint(
			"No project specified",
			"Use --project or set BASECAMP_PROJECT_ID",
		)
	}
	return nil
}

// RequireAccount returns an error if no account is configured.
func (c *Client) RequireAccount() error {
	if c.cfg.AccountID == "" {
		return ErrUsageHint(
			"No account configured",
			"Set BASECAMP_ACCOUNT_ID environment variable",
		)
	}
	return nil
}

// Config returns the client configuration.
func (c *Client) Config() *Config {
	return c.cfg
}

// Projects returns the ProjectsService for project operations.
func (c *Client) Projects() *ProjectsService {
	if c.projects == nil {
		c.projects = NewProjectsService(c)
	}
	return c.projects
}

// Todos returns the TodosService for todo operations.
func (c *Client) Todos() *TodosService {
	if c.todos == nil {
		c.todos = NewTodosService(c)
	}
	return c.todos
}
