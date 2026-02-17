package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Note: Default retry/backoff values are now in http.go as exported constants.

// DefaultUserAgent is the default User-Agent header value.
const DefaultUserAgent = "basecamp-sdk-go/" + Version

// Client is an HTTP client for the Basecamp API.
// Client holds shared resources and is used to create AccountClient instances
// for specific Basecamp accounts via the ForAccount method.
//
// Client is safe for concurrent use after construction. Do not modify
// the Config after the client is in use by multiple goroutines.
type Client struct {
	httpClient    *http.Client
	tokenProvider TokenProvider
	authStrategy  AuthStrategy
	cfg           *Config
	cache         *Cache
	userAgent     string
	logger        *slog.Logger
	httpOpts      HTTPOptions
	hooks         Hooks

	// Generated client (single shared instance, account passed per operation)
	genOnce sync.Once
	gen     *generated.ClientWithResponses

	// Authorization service (account-independent)
	authMu        sync.Mutex
	authorization *AuthorizationService
}

// AccountClient is an HTTP client bound to a specific Basecamp account.
// Create an AccountClient using Client.ForAccount(accountID).
// AccountClient is safe for concurrent use.
//
// The Basecamp API requires an account ID in the URL path
// (e.g., https://3.basecampapi.com/12345/projects.json). This SDK passes the
// account ID as a path parameter to each generated client operation, matching
// the OpenAPI spec's /{accountId}/... path structure.
//
// AccountClient shares the parent Client's generated API client and HTTP
// resources. Creating multiple AccountClients via ForAccount is lightweight.
type AccountClient struct {
	parent    *Client
	accountID string
	mu        sync.Mutex // protects lazy service initialization

	// Services (lazy-initialized, protected by mu)
	projects              *ProjectsService
	todos                 *TodosService
	todosets              *TodosetsService
	todolists             *TodolistsService
	todolistGroups        *TodolistGroupsService
	people                *PeopleService
	comments              *CommentsService
	messages              *MessagesService
	messageBoards         *MessageBoardsService
	messageTypes          *MessageTypesService
	webhooks              *WebhooksService
	events                *EventsService
	search                *SearchService
	templates             *TemplatesService
	tools                 *ToolsService
	lineup                *LineupService
	subscriptions         *SubscriptionsService
	boosts                *BoostsService
	campfires             *CampfiresService
	timesheet             *TimesheetService
	schedules             *SchedulesService
	forwards              *ForwardsService
	recordings            *RecordingsService
	checkins              *CheckinsService
	vaults                *VaultsService
	documents             *DocumentsService
	uploads               *UploadsService
	cardTables            *CardTablesService
	cards                 *CardsService
	cardColumns           *CardColumnsService
	cardSteps             *CardStepsService
	attachments           *AttachmentsService
	clientApprovals       *ClientApprovalsService
	clientCorrespondences *ClientCorrespondencesService
	clientReplies         *ClientRepliesService
	timeline              *TimelineService
	reports               *ReportsService
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

// WithLogger sets a custom slog logger for debug output.
// By default, the client uses a no-op logger (silent).
// Passing nil is safe and will use the default no-op logger.
func WithLogger(l *slog.Logger) ClientOption {
	return func(client *Client) {
		if l != nil {
			client.logger = l
		}
	}
}

// WithCache sets a custom cache.
func WithCache(cache *Cache) ClientOption {
	return func(client *Client) {
		client.cache = cache
	}
}

// WithAuthStrategy sets a custom authentication strategy.
// The default strategy is BearerAuth, which sets the Authorization header
// with a Bearer token from the token provider.
//
// Use this to implement alternative auth schemes such as cookie-based auth,
// API keys, or mutual TLS.
func WithAuthStrategy(strategy AuthStrategy) ClientOption {
	return func(client *Client) {
		client.authStrategy = strategy
	}
}

// NewClient creates a new API client with spec-driven defaults.
//
// The client automatically:
//   - Retries failed GET requests with exponential backoff
//   - Does NOT retry POST/PUT/DELETE on 429/5xx (to avoid duplicating data)
//   - Retries mutations once after successful 401 token refresh
//   - Respects Retry-After headers on 429 responses
//   - Follows pagination via Link headers
//
// Configuration options:
//   - WithTimeout(d)      - Request timeout (default: 30s)
//   - WithMaxRetries(n)   - Max retry attempts for GET (default: 5)
//   - WithCache(c)        - Enable ETag-based caching
//   - WithTransport(t)    - Custom http.RoundTripper
//   - WithLogger(l)       - slog.Logger for debug output
func NewClient(cfg *Config, tokenProvider TokenProvider, opts ...ClientOption) *Client {
	// Deep-copy the config to prevent post-construction mutation.
	// The client captures configuration at construction time.
	cfgCopy := *cfg
	c := &Client{
		tokenProvider: tokenProvider,
		cfg:           &cfgCopy,
		userAgent:     DefaultUserAgent,
		logger:        slog.New(discardHandler{}),
		hooks:         NoopHooks{},
		httpOpts:      DefaultHTTPOptions(),
	}

	// Apply options (may modify httpOpts)
	for _, opt := range opts {
		opt(c)
	}

	// Default to BearerAuth if no custom auth strategy was provided
	if c.authStrategy == nil {
		c.authStrategy = &BearerAuth{TokenProvider: c.tokenProvider}
	}

	// Create HTTP client with configured options
	transport := c.httpOpts.Transport
	if transport == nil {
		transport = newDefaultTransport()
	}

	// Wrap transport with logging transport
	transport = &loggingTransport{inner: transport, client: c}

	c.httpClient = &http.Client{
		Timeout:   c.httpOpts.Timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			// Strip Authorization header when redirecting to a different origin
			// to prevent credential leakage to third-party hosts.
			if len(via) > 0 && !isSameOrigin(req.URL.String(), via[0].URL.String()) {
				req.Header.Del("Authorization")
			}
			return nil
		},
	}

	// Validate configuration
	// Skip HTTPS validation for localhost (used in tests)
	if c.cfg.BaseURL != "" && !isLocalhost(c.cfg.BaseURL) {
		if err := requireHTTPS(c.cfg.BaseURL); err != nil {
			panic("basecamp: base URL must use HTTPS: " + c.cfg.BaseURL)
		}
	}
	if c.httpOpts.Timeout <= 0 {
		panic("basecamp: timeout must be positive")
	}
	if c.httpOpts.MaxRetries < 0 {
		panic("basecamp: max retries must be non-negative")
	}
	if c.httpOpts.MaxPages <= 0 {
		panic("basecamp: max pages must be positive")
	}

	// Initialize cache if enabled and not overridden
	if c.cache == nil && cfg.CacheEnabled {
		c.cache = NewCache(cfg.CacheDir)
	}

	return c
}

// ForAccount returns an AccountClient bound to the specified Basecamp account.
// The AccountClient shares the parent Client's HTTP transport, token provider,
// and other resources, but is configured to make API calls for the given account.
//
// The accountID must be a numeric string (e.g., "12345"). ForAccount panics if
// the accountID is empty or contains non-digit characters.
//
// Example:
//
//	client := basecamp.NewClient(cfg, tokenProvider)
//	account := client.ForAccount("12345")
//	projects, err := account.Projects().List(ctx, nil)
func (c *Client) ForAccount(accountID string) *AccountClient {
	if accountID == "" {
		panic("basecamp: ForAccount requires non-empty account ID")
	}
	for _, r := range accountID {
		if r < '0' || r > '9' {
			panic("basecamp: ForAccount requires numeric account ID, got: " + accountID)
		}
	}

	// Initialize shared generated client on first use (thread-safe)
	c.initGeneratedClient()

	return &AccountClient{
		parent:    c,
		accountID: accountID,
	}
}

// AccountID returns the account ID this client is bound to.
func (ac *AccountClient) AccountID() string {
	return ac.accountID
}

// Get performs an account-scoped GET request.
func (ac *AccountClient) Get(ctx context.Context, path string) (*Response, error) {
	return ac.parent.doRequest(ctx, "GET", ac.accountPath(path), nil)
}

// Post performs an account-scoped POST request with a JSON body.
func (ac *AccountClient) Post(ctx context.Context, path string, body any) (*Response, error) {
	return ac.parent.doRequest(ctx, "POST", ac.accountPath(path), body)
}

// Put performs an account-scoped PUT request with a JSON body.
func (ac *AccountClient) Put(ctx context.Context, path string, body any) (*Response, error) {
	return ac.parent.doRequest(ctx, "PUT", ac.accountPath(path), body)
}

// Delete performs an account-scoped DELETE request.
func (ac *AccountClient) Delete(ctx context.Context, path string) (*Response, error) {
	return ac.parent.doRequest(ctx, "DELETE", ac.accountPath(path), nil)
}

// GetAll fetches all pages for an account-scoped paginated resource.
func (ac *AccountClient) GetAll(ctx context.Context, path string) ([]json.RawMessage, error) {
	return ac.parent.GetAllWithLimit(ctx, ac.accountPath(path), 0)
}

// GetAllWithLimit fetches pages for an account-scoped paginated resource up to a limit.
// If limit is 0, it fetches all pages (same as GetAll).
// If limit > 0, it stops after collecting at least limit items.
func (ac *AccountClient) GetAllWithLimit(ctx context.Context, path string, limit int) ([]json.RawMessage, error) {
	return ac.parent.GetAllWithLimit(ctx, ac.accountPath(path), limit)
}

// accountPath prepends the account ID to the path.
// Absolute URLs are returned unchanged (e.g., pagination Link headers).
// Paths already prefixed with the account ID are returned unchanged.
//
// Callers should pass account-less paths (e.g., "/projects.json").
// If a path is already prefixed (e.g., "/12345/projects.json"), it is
// returned as-is to avoid double-prefixing.
func (ac *AccountClient) accountPath(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Guard against double-prefixing if caller already included account ID.
	// Check for /{accountId}/, /{accountId}?, or /{accountId} (exact).
	prefix := "/" + ac.accountID
	if strings.HasPrefix(path, prefix) {
		rest := path[len(prefix):]
		if rest == "" || rest[0] == '/' || rest[0] == '?' {
			return path
		}
	}
	return "/" + ac.accountID + path
}

// initGeneratedClient initializes the shared generated OpenAPI client.
// Uses sync.Once to ensure the client is only created once.
// The account ID is passed as a parameter to each operation, not baked into the URL.
func (c *Client) initGeneratedClient() {
	c.genOnce.Do(func() {
		serverURL := strings.TrimSuffix(c.cfg.BaseURL, "/")
		authEditor := func(ctx context.Context, req *http.Request) error {
			if err := c.authStrategy.Authenticate(ctx, req); err != nil {
				return err
			}
			req.Header.Set("User-Agent", c.userAgent)
			// Only set Content-Type if not already set (preserves binary upload content types)
			if req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Accept", "application/json")
			return nil
		}
		gen, err := generated.NewClientWithResponses(serverURL,
			generated.WithHTTPClient(c.httpClient),
			generated.WithRequestEditorFn(authEditor))
		if err != nil {
			panic(fmt.Sprintf("basecamp: failed to create generated client: %v", err))
		}
		c.gen = gen
	})
}

// discardHandler is a slog.Handler that discards all log records.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (h discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h discardHandler) WithGroup(string) slog.Handler           { return h }

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
	return c.GetAllWithLimit(ctx, path, 0)
}

// GetAllWithLimit fetches pages for a paginated resource up to a limit.
// If limit is 0, it fetches all pages (same as GetAll).
// If limit > 0, it stops after collecting at least limit items.
func (c *Client) GetAllWithLimit(ctx context.Context, path string, limit int) ([]json.RawMessage, error) {
	var allResults []json.RawMessage
	baseURL, err := c.buildURL(path)
	if err != nil {
		return nil, err
	}
	url := baseURL
	var page int

	for page = 1; page <= c.httpOpts.MaxPages; page++ {
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

		// Check if we've reached the limit
		if limit > 0 && len(allResults) >= limit {
			// Trim to exactly the limit
			allResults = allResults[:limit]
			break
		}

		// Check for next page
		nextURL := parseNextLink(resp.Headers.Get("Link"))
		if nextURL == "" {
			break
		}
		// Resolve relative URLs against the current page URL (handles path-relative links)
		nextURL = resolveURL(url, nextURL)
		// Validate same-origin against initial baseURL to prevent SSRF / token leakage
		if !isSameOrigin(nextURL, baseURL) {
			return nil, fmt.Errorf("pagination Link header points to different origin: %s", nextURL)
		}
		url = nextURL
	}

	if page > c.httpOpts.MaxPages {
		c.logger.Warn("pagination capped", "maxPages", c.httpOpts.MaxPages)
	}

	return allResults, nil
}

// FollowPagination fetches additional pages following Link headers from an HTTP response.
// This is used after calling the generated client for the first page.
// The httpResp should be from the generated client's *WithResponse method.
// firstPageCount is the number of items already collected from the first page.
// limit is the maximum total items to return (0 = unlimited).
// Returns raw JSON items from subsequent pages only (first page items are handled by caller).
//
// Request URL requirement: httpResp.Request.URL is required for same-origin validation.
// If the response has no Request (e.g., manually constructed), pagination returns an
// error even for absolute Link headers. This fail-closed behavior prevents SSRF and
// token leakage when the original request origin cannot be verified.
//
// Security: Link headers are resolved against the current page URL and validated
// for same-origin against the original request to prevent SSRF and token leakage.
func (c *Client) FollowPagination(ctx context.Context, httpResp *http.Response, firstPageCount, limit int) ([]json.RawMessage, error) {
	if httpResp == nil {
		return nil, nil
	}

	// Check if we already have enough items
	if limit > 0 && firstPageCount >= limit {
		return nil, nil
	}

	// Get next page URL from Link header
	nextLink := parseNextLink(httpResp.Header.Get("Link"))
	if nextLink == "" {
		return nil, nil
	}

	// Security: Require httpResp.Request.URL for same-origin validation.
	// Without the original request URL, we cannot verify Link headers are same-origin,
	// which could allow SSRF or token leakage to malicious servers.
	if httpResp.Request == nil || httpResp.Request.URL == nil {
		return nil, fmt.Errorf("cannot follow pagination: response has no request URL (required for same-origin validation)")
	}
	baseURL := httpResp.Request.URL.String()

	// Resolve relative Link URLs against the current page
	nextURL := resolveURL(baseURL, nextLink)

	// Guard against resolution failures (shouldn't happen with valid baseURL, but be safe)
	parsedURL, err := url.Parse(nextURL)
	if err != nil || !parsedURL.IsAbs() {
		return nil, fmt.Errorf("failed to resolve Link header URL %q against %q", nextLink, baseURL)
	}

	// Validate same-origin for the FIRST Link header before making any request.
	// This prevents the first Link header from redirecting to a malicious origin.
	if !isSameOrigin(baseURL, nextURL) {
		return nil, fmt.Errorf("pagination Link header points to different origin: %s", nextURL)
	}

	var allResults []json.RawMessage
	currentCount := firstPageCount
	var page int

	for page = 2; page <= c.httpOpts.MaxPages && nextURL != ""; page++ {
		// Track current page URL for relative URL resolution in this iteration
		currentPageURL := nextURL

		resp, err := c.doRequestURL(ctx, "GET", nextURL, nil)
		if err != nil {
			return nil, err
		}

		// Parse response as array
		var items []json.RawMessage
		if err := json.Unmarshal(resp.Data, &items); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		allResults = append(allResults, items...)
		currentCount += len(items)

		// Check if we've reached the limit
		if limit > 0 && currentCount >= limit {
			// Trim to exactly the limit (accounting for first page)
			excess := currentCount - limit
			if excess > 0 && len(allResults) > excess {
				allResults = allResults[:len(allResults)-excess]
			}
			break
		}

		// Get next page URL, resolved against current page
		nextLink = parseNextLink(resp.Headers.Get("Link"))
		if nextLink == "" {
			break
		}
		nextURL = resolveURL(currentPageURL, nextLink)

		// Validate same-origin against original request
		if !isSameOrigin(baseURL, nextURL) {
			return nil, fmt.Errorf("pagination Link header points to different origin: %s", nextURL)
		}
	}

	if page > c.httpOpts.MaxPages {
		c.logger.Warn("pagination capped", "maxPages", c.httpOpts.MaxPages)
	}

	return allResults, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*Response, error) {
	url, err := c.buildURL(path)
	if err != nil {
		return nil, err
	}
	return c.doRequestURL(ctx, method, url, body)
}

func (c *Client) doRequestURL(ctx context.Context, method, url string, body any) (*Response, error) {
	// Mutations (POST/PUT/DELETE): Don't retry on 429/5xx to avoid duplicating data.
	// Only retry once after successful 401 token refresh.
	if method != "GET" {
		resp, err := c.singleRequest(ctx, method, url, body, 1)
		if err == nil {
			return resp, nil
		}
		// Only retry if this was a 401 that triggered successful token refresh
		if apiErr, ok := err.(*Error); ok && apiErr.Retryable && apiErr.Code == CodeAuth {
			c.logger.Debug("token refreshed, retrying mutation", "method", method)
			info := RequestInfo{Method: method, URL: url, Attempt: 1}
			c.hooks.OnRetry(ctx, info, 2, err)
			return c.singleRequest(ctx, method, url, body, 2)
		}
		return nil, err
	}

	// GET requests: Full retry with exponential backoff
	var attempt int
	var lastErr error

	for attempt = 1; attempt <= c.httpOpts.MaxRetries; attempt++ {
		resp, err := c.singleRequest(ctx, method, url, body, attempt)
		if err == nil {
			return resp, nil
		}

		// Check for retryable error with server-specified delay
		var delay time.Duration
		if re, ok := err.(*retryableError); ok {
			lastErr = re.err
			if re.retryAfter > 0 {
				// Use server-specified Retry-After delay
				delay = re.retryAfter
			} else {
				delay = c.backoffDelay(attempt)
			}
		} else if apiErr, ok := err.(*Error); ok {
			if !apiErr.Retryable {
				return nil, err
			}
			lastErr = err
			delay = c.backoffDelay(attempt)
		} else {
			return nil, err
		}

		c.logger.Debug("retrying request", "attempt", attempt, "maxRetries", c.httpOpts.MaxRetries, "delay", delay, "error", lastErr)

		// Notify hooks about the retry
		info := RequestInfo{Method: method, URL: url, Attempt: attempt}
		c.hooks.OnRetry(ctx, info, attempt+1, lastErr)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			continue
		}
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.httpOpts.MaxRetries, lastErr)
}

func (c *Client) singleRequest(ctx context.Context, method, url string, body any, attempt int) (*Response, error) {
	// Add attempt number to context for hooks in transport layer
	ctx = contextWithAttempt(ctx, attempt)

	// Build request body
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	// Create request - hooks are called in transport layer (loggingTransport)
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if err := c.authStrategy.Authenticate(ctx, req); err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add ETag for cached GET requests. Derive cache key from the Authorization
	// header applied by the auth strategy, so each credential gets its own namespace.
	var cacheKey string
	if method == "GET" && c.cache != nil {
		cacheKey = c.cache.Key(url, "", req.Header.Get("Authorization")) // URL already includes account when needed
		if etag := c.cache.GetETag(cacheKey); etag != "" {
			req.Header.Set("If-None-Match", etag)
			c.logger.Debug("cache conditional request", "etag", etag)
		}
	}

	c.logger.Debug("http request", "method", method, "url", url, "attempt", attempt)

	// Execute request (hooks are called in transport layer)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrNetwork(err)
	}
	defer func() { _ = resp.Body.Close() }()

	c.logger.Debug("http response", "status", resp.StatusCode)

	// Handle response based on status code
	switch resp.StatusCode {
	case http.StatusNotModified: // 304
		if cacheKey != "" {
			c.logger.Debug("cache hit", "status", 304)
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
		respBody, err := limitedReadAll(resp.Body, MaxResponseBodyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		// HTTP 204 No Content has no body by definition. Normalize to JSON null
		// so callers can always unmarshal 204 responses without error.
		// UnmarshalData on a 204 will succeed: pointers become nil, structs
		// become zero-value, slices become nil. Callers that need to distinguish
		// 204 from a body response should check Response.StatusCode.
		// Note: we do NOT normalize empty bodies for 200/201 — that's a server
		// bug and callers should see the unmarshal error.
		if resp.StatusCode == http.StatusNoContent {
			respBody = json.RawMessage("null")
		}

		// Cache GET responses with ETag
		if method == "GET" && cacheKey != "" {
			if etag := resp.Header.Get("ETag"); etag != "" {
				_ = c.cache.Set(cacheKey, respBody, etag) // Ignore cache write errors
				c.logger.Debug("cache stored", "etag", etag)
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
		respBody, _ := limitedReadAll(resp.Body, MaxErrorBodyBytes)
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
				// Truncate error messages to prevent information leakage and unbounded memory growth
				return nil, ErrAPI(resp.StatusCode, truncateString(msg, MaxErrorMessageBytes))
			}
		}
		return nil, ErrAPI(resp.StatusCode, fmt.Sprintf("Request failed (HTTP %d)", resp.StatusCode))
	}
}

func (c *Client) buildURL(path string) (string, error) {
	// Absolute URLs: enforce HTTPS and return as-is (e.g., pagination Link headers)
	if strings.HasPrefix(path, "https://") {
		return path, nil
	}
	if strings.HasPrefix(path, "http://") {
		return "", fmt.Errorf("URL must use HTTPS, got: %s", path)
	}
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Normalize BaseURL to avoid double slashes when concatenating
	base := strings.TrimSuffix(c.cfg.BaseURL, "/")
	return base + path, nil
}

func (c *Client) backoffDelay(attempt int) time.Duration {
	// Exponential backoff: base * 2^(attempt-1)
	delay := c.httpOpts.BaseDelay * time.Duration(1<<(attempt-1))

	// Add jitter
	jitter := time.Duration(rand.Int63n(int64(c.httpOpts.MaxJitter))) // #nosec G404 -- jitter doesn't need cryptographic randomness

	return delay + jitter
}

// parseNextLink extracts the next URL from a Link header.
func parseNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	for part := range strings.SplitSeq(linkHeader, ",") {
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
// It handles both seconds (integer) and HTTP-date formats.
// Returns 0 if the header is empty or cannot be parsed.
func parseRetryAfter(header string) int {
	if header == "" {
		return 0
	}
	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return seconds
	}
	// Try parsing as HTTP-date (e.g., "Wed, 21 Oct 2015 07:28:00 GMT")
	if t, err := http.ParseTime(header); err == nil {
		seconds := int(time.Until(t).Seconds())
		if seconds > 0 {
			return seconds
		}
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

// Config returns a copy of the client configuration.
//
// Modifying the returned Config has no effect on the client.
// This prevents race conditions from post-construction modification.
func (c *Client) Config() Config {
	return *c.cfg
}

// Authorization returns the AuthorizationService for authorization operations.
// This is the only service available directly on Client, as it doesn't require
// an account context. All other services require an AccountClient via ForAccount.
func (c *Client) Authorization() *AuthorizationService {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if c.authorization == nil {
		c.authorization = NewAuthorizationService(c)
	}
	return c.authorization
}

// Projects returns the ProjectsService for project operations.
func (ac *AccountClient) Projects() *ProjectsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.projects == nil {
		ac.projects = NewProjectsService(ac)
	}
	return ac.projects
}

// Todos returns the TodosService for todo operations.
func (ac *AccountClient) Todos() *TodosService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.todos == nil {
		ac.todos = NewTodosService(ac)
	}
	return ac.todos
}

// Todosets returns the TodosetsService for todoset operations.
func (ac *AccountClient) Todosets() *TodosetsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.todosets == nil {
		ac.todosets = NewTodosetsService(ac)
	}
	return ac.todosets
}

// Todolists returns the TodolistsService for todolist operations.
func (ac *AccountClient) Todolists() *TodolistsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.todolists == nil {
		ac.todolists = NewTodolistsService(ac)
	}
	return ac.todolists
}

// TodolistGroups returns the TodolistGroupsService for todolist group operations.
func (ac *AccountClient) TodolistGroups() *TodolistGroupsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.todolistGroups == nil {
		ac.todolistGroups = NewTodolistGroupsService(ac)
	}
	return ac.todolistGroups
}

// People returns the PeopleService for people operations.
func (ac *AccountClient) People() *PeopleService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.people == nil {
		ac.people = NewPeopleService(ac)
	}
	return ac.people
}

// Comments returns the CommentsService for comment operations.
func (ac *AccountClient) Comments() *CommentsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.comments == nil {
		ac.comments = NewCommentsService(ac)
	}
	return ac.comments
}

// Messages returns the MessagesService for message operations.
func (ac *AccountClient) Messages() *MessagesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.messages == nil {
		ac.messages = NewMessagesService(ac)
	}
	return ac.messages
}

// MessageBoards returns the MessageBoardsService for message board operations.
func (ac *AccountClient) MessageBoards() *MessageBoardsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.messageBoards == nil {
		ac.messageBoards = NewMessageBoardsService(ac)
	}
	return ac.messageBoards
}

// MessageTypes returns the MessageTypesService for message type operations.
func (ac *AccountClient) MessageTypes() *MessageTypesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.messageTypes == nil {
		ac.messageTypes = NewMessageTypesService(ac)
	}
	return ac.messageTypes
}

// Webhooks returns the WebhooksService for webhook operations.
func (ac *AccountClient) Webhooks() *WebhooksService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.webhooks == nil {
		ac.webhooks = NewWebhooksService(ac)
	}
	return ac.webhooks
}

// Events returns the EventsService for event operations.
func (ac *AccountClient) Events() *EventsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.events == nil {
		ac.events = NewEventsService(ac)
	}
	return ac.events
}

// Search returns the SearchService for search operations.
func (ac *AccountClient) Search() *SearchService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.search == nil {
		ac.search = NewSearchService(ac)
	}
	return ac.search
}

// Templates returns the TemplatesService for template operations.
func (ac *AccountClient) Templates() *TemplatesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.templates == nil {
		ac.templates = NewTemplatesService(ac)
	}
	return ac.templates
}

// Tools returns the ToolsService for dock tool operations.
func (ac *AccountClient) Tools() *ToolsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.tools == nil {
		ac.tools = NewToolsService(ac)
	}
	return ac.tools
}

// Lineup returns the LineupService for lineup marker operations.
func (ac *AccountClient) Lineup() *LineupService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.lineup == nil {
		ac.lineup = NewLineupService(ac)
	}
	return ac.lineup
}

// Subscriptions returns the SubscriptionsService for subscription operations.
func (ac *AccountClient) Subscriptions() *SubscriptionsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.subscriptions == nil {
		ac.subscriptions = NewSubscriptionsService(ac)
	}
	return ac.subscriptions
}

// Boosts returns the BoostsService for boost (emoji reaction) operations.
func (ac *AccountClient) Boosts() *BoostsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.boosts == nil {
		ac.boosts = NewBoostsService(ac)
	}
	return ac.boosts
}

// Campfires returns the CampfiresService for campfire chat operations.
func (ac *AccountClient) Campfires() *CampfiresService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.campfires == nil {
		ac.campfires = NewCampfiresService(ac)
	}
	return ac.campfires
}

// Timesheet returns the TimesheetService for timesheet report operations.
func (ac *AccountClient) Timesheet() *TimesheetService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.timesheet == nil {
		ac.timesheet = NewTimesheetService(ac)
	}
	return ac.timesheet
}

// Schedules returns the SchedulesService for schedule operations.
func (ac *AccountClient) Schedules() *SchedulesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.schedules == nil {
		ac.schedules = NewSchedulesService(ac)
	}
	return ac.schedules
}

// Forwards returns the ForwardsService for email forward operations.
func (ac *AccountClient) Forwards() *ForwardsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.forwards == nil {
		ac.forwards = NewForwardsService(ac)
	}
	return ac.forwards
}

// Recordings returns the RecordingsService for recording operations.
func (ac *AccountClient) Recordings() *RecordingsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.recordings == nil {
		ac.recordings = NewRecordingsService(ac)
	}
	return ac.recordings
}

// Checkins returns the CheckinsService for automatic check-in operations.
func (ac *AccountClient) Checkins() *CheckinsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.checkins == nil {
		ac.checkins = NewCheckinsService(ac)
	}
	return ac.checkins
}

// Vaults returns the VaultsService for vault (folder) operations.
func (ac *AccountClient) Vaults() *VaultsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.vaults == nil {
		ac.vaults = NewVaultsService(ac)
	}
	return ac.vaults
}

// Documents returns the DocumentsService for document operations.
func (ac *AccountClient) Documents() *DocumentsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.documents == nil {
		ac.documents = NewDocumentsService(ac)
	}
	return ac.documents
}

// Uploads returns the UploadsService for upload (file) operations.
func (ac *AccountClient) Uploads() *UploadsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.uploads == nil {
		ac.uploads = NewUploadsService(ac)
	}
	return ac.uploads
}

// CardTables returns the CardTablesService for card table operations.
func (ac *AccountClient) CardTables() *CardTablesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.cardTables == nil {
		ac.cardTables = NewCardTablesService(ac)
	}
	return ac.cardTables
}

// Cards returns the CardsService for card operations.
func (ac *AccountClient) Cards() *CardsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.cards == nil {
		ac.cards = NewCardsService(ac)
	}
	return ac.cards
}

// CardColumns returns the CardColumnsService for card column operations.
func (ac *AccountClient) CardColumns() *CardColumnsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.cardColumns == nil {
		ac.cardColumns = NewCardColumnsService(ac)
	}
	return ac.cardColumns
}

// CardSteps returns the CardStepsService for card step operations.
func (ac *AccountClient) CardSteps() *CardStepsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.cardSteps == nil {
		ac.cardSteps = NewCardStepsService(ac)
	}
	return ac.cardSteps
}

// Attachments returns the AttachmentsService for file upload operations.
func (ac *AccountClient) Attachments() *AttachmentsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.attachments == nil {
		ac.attachments = NewAttachmentsService(ac)
	}
	return ac.attachments
}

// ClientApprovals returns the ClientApprovalsService for client approval operations.
func (ac *AccountClient) ClientApprovals() *ClientApprovalsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.clientApprovals == nil {
		ac.clientApprovals = NewClientApprovalsService(ac)
	}
	return ac.clientApprovals
}

// ClientCorrespondences returns the ClientCorrespondencesService for client correspondence operations.
func (ac *AccountClient) ClientCorrespondences() *ClientCorrespondencesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.clientCorrespondences == nil {
		ac.clientCorrespondences = NewClientCorrespondencesService(ac)
	}
	return ac.clientCorrespondences
}

// ClientReplies returns the ClientRepliesService for client reply operations.
func (ac *AccountClient) ClientReplies() *ClientRepliesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.clientReplies == nil {
		ac.clientReplies = NewClientRepliesService(ac)
	}
	return ac.clientReplies
}

// Timeline returns the TimelineService for timeline and progress operations.
func (ac *AccountClient) Timeline() *TimelineService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.timeline == nil {
		ac.timeline = NewTimelineService(ac)
	}
	return ac.timeline
}

// Reports returns the ReportsService for reports operations.
func (ac *AccountClient) Reports() *ReportsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.reports == nil {
		ac.reports = NewReportsService(ac)
	}
	return ac.reports
}
