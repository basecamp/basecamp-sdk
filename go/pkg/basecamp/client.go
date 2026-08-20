package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
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
const DefaultUserAgent = "basecamp-sdk-go/" + Version + " (api:" + APIVersion + ")"

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
	hillCharts            *HillChartsService
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
	bookmarks             *BookmarksService
	folders               *FoldersService
	drafts                *DraftsService
	myNotes               *MyNotesService
	calendars             *CalendarsService
	boosts                *BoostsService
	campfires             *CampfiresService
	timesheet             *TimesheetService
	schedules             *SchedulesService
	forwards              *ForwardsService
	recordings            *RecordingsService
	checkins              *CheckinsService
	vaults                *VaultsService
	documents             *DocumentsService
	cloudFiles            *CloudFilesService
	googleDocuments       *GoogleDocumentsService
	uploads               *UploadsService
	cardTables            *CardTablesService
	cards                 *CardsService
	cardColumns           *CardColumnsService
	cardSteps             *CardStepsService
	wormholes             *WormholesService
	attachments           *AttachmentsService
	clientApprovals       *ClientApprovalsService
	clientCorrespondences *ClientCorrespondencesService
	clientReplies         *ClientRepliesService
	timeline              *TimelineService
	reports               *ReportsService
	account               *AccountService
	gauges                *GaugesService
	myAssignments         *MyAssignmentsService
	myNotifications       *MyNotificationsService
	everything            *EverythingService
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
//   - WithMaxRetries(n)   - Total attempt count (default: 3; 0 means no retry)
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
		// MaxRetries names the total attempt count used by the retry loops in
		// doRequestURL and fetchAPIDownload. Zero is legal and means "no
		// retries — exactly one attempt" (SPEC §2 validation step 4): it is the
		// spelling callers reach for to turn retry off, and every other SDK
		// with a numeric cap already accepted it. Both loops floor the cap at
		// one attempt, so what they can assume is >= 0, not >= 1. A negative
		// cap is the genuine misconfiguration and is what this rejects.
		panic("basecamp: max retries must not be negative")
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
			// Backstop: refuse to attach credentials to a foreign origin.
			if err := c.assertCredentialOrigin(req); err != nil {
				return err
			}
			if err := c.authStrategy.Authenticate(ctx, req); err != nil {
				return err
			}
			req.Header.Set("User-Agent", c.userAgent)
			// Content-Type describes a request body, so set the JSON default only
			// when a body is present and the caller has not already set one.
			if requestHasBody(req) && req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Accept", "application/json")
			return nil
		}
		// The generated client runs its own retry loop, and until #718 it was
		// never told this client's settings — so a RETRY-ELIGIBLE typed
		// operation ran on DefaultRetryConfig{MaxRetries: 3} no matter what
		// WithMaxRetries said, and a caller who lowered the cap to fail fast
		// still got the generated default. Only the raw Get/GetAll and download
		// paths, which run doRequestURL's own loop, honored it.
		//
		// "Retry-eligible" is the whole qualification: doWithRetry forces
		// maxAttempts to 1 for a non-idempotent operation before consulting the
		// config at all, and then clamps to the per-operation ceiling. So this
		// changes nothing for CreateTodo, and nothing for an op declaring
		// max: 2 beyond lowering it further. Pass what the caller configured.
		//
		// BaseDelay carries over, CLAMPED to the same ceiling doRequestURL
		// applies: the generated loop uses BaseDelay verbatim for its first
		// sleep and only applies MaxDelay after multiplying for the next one, so
		// an unclamped WithBaseDelay(10*time.Minute) would stall a typed
		// operation for ten minutes. SPEC §7's backoff ceiling is explicit that
		// no single computed sleep exceeds it, "with no carve-out for the first
		// one". The clamp is not conditional on being non-zero: a caller who
		// sets 0 wants no backoff, and skipping the assignment would leave typed
		// operations on the generated 1s default while raw GETs waited only for
		// jitter — the exact split this change exists to remove.
		//
		// MaxDelay and Multiplier keep the generated defaults, HTTPOptions
		// having no counterpart for either (MaxDelay's default is already
		// MaxBackoffDelay). MaxJitter has no counterpart in the other direction
		// and stays with doRequestURL.
		retryCfg := generated.DefaultRetryConfig()
		retryCfg.MaxRetries = c.httpOpts.MaxRetries
		retryCfg.BaseDelay = min(c.httpOpts.BaseDelay, MaxBackoffDelay)
		if retryCfg.BaseDelay < 0 {
			retryCfg.BaseDelay = 0
		}

		gen, err := generated.NewClientWithResponses(serverURL,
			generated.WithHTTPClient(c.httpClient),
			generated.WithRetryConfig(retryCfg),
			generated.WithRequestEditorFn(authEditor))
		if err != nil {
			panic(fmt.Sprintf("basecamp: failed to create generated client: %v", err))
		}
		c.gen = gen
	})
}

func requestHasBody(req *http.Request) bool {
	return req != nil && req.Body != nil && req.Body != http.NoBody
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
// FollowPagination is the public API — returns items and error only.
func (c *Client) FollowPagination(ctx context.Context, httpResp *http.Response, firstPageCount, limit int) ([]json.RawMessage, error) {
	items, _, err := c.followPagination(ctx, httpResp, firstPageCount, limit)
	return items, err
}

// followPagination is the internal implementation that also reports truncation.
func (c *Client) followPagination(ctx context.Context, httpResp *http.Response, firstPageCount, limit int) (items []json.RawMessage, truncated bool, err error) {
	if httpResp == nil {
		return nil, false, nil
	}

	// Check if we already have enough items
	if limit > 0 && firstPageCount >= limit {
		return nil, false, nil
	}

	// Get next page URL from Link header
	nextLink := parseNextLink(httpResp.Header.Get("Link"))
	if nextLink == "" {
		return nil, false, nil
	}

	// Security: Require httpResp.Request.URL for same-origin validation.
	// Without the original request URL, we cannot verify Link headers are same-origin,
	// which could allow SSRF or token leakage to malicious servers.
	if httpResp.Request == nil || httpResp.Request.URL == nil {
		return nil, false, fmt.Errorf("cannot follow pagination: response has no request URL (required for same-origin validation)")
	}
	baseURL := httpResp.Request.URL.String()

	// Resolve relative Link URLs against the current page
	nextURL := resolveURL(baseURL, nextLink)

	// Guard against resolution failures (shouldn't happen with valid baseURL, but be safe)
	parsedURL, err := url.Parse(nextURL)
	if err != nil || !parsedURL.IsAbs() {
		return nil, false, fmt.Errorf("failed to resolve Link header URL %q against %q", nextLink, baseURL)
	}

	// Validate same-origin for the FIRST Link header before making any request.
	// This prevents the first Link header from redirecting to a malicious origin.
	if !isSameOrigin(baseURL, nextURL) {
		return nil, false, fmt.Errorf("pagination Link header points to different origin: %s", nextURL)
	}

	var allResults []json.RawMessage
	currentCount := firstPageCount
	hasMore := false
	var page int

	for page = 2; page <= c.httpOpts.MaxPages && nextURL != ""; page++ {
		// Track current page URL for relative URL resolution in this iteration
		currentPageURL := nextURL

		resp, err := c.doRequestURL(ctx, "GET", nextURL, nil)
		if err != nil {
			return nil, false, err
		}

		// Parse response as array
		var pageItems []json.RawMessage
		if err := json.Unmarshal(resp.Data, &pageItems); err != nil {
			return nil, false, fmt.Errorf("failed to parse response: %w", err)
		}
		allResults = append(allResults, pageItems...)
		currentCount += len(pageItems)

		// Check if we've reached the limit
		if limit > 0 && currentCount >= limit {
			// Trim to exactly the limit (accounting for first page)
			excess := currentCount - limit
			if excess > 0 && len(allResults) > excess {
				allResults = allResults[:len(allResults)-excess]
			}
			// Truncated if we dropped items OR more pages exist
			hasMore = excess > 0 || parseNextLink(resp.Headers.Get("Link")) != ""
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
			return nil, false, fmt.Errorf("pagination Link header points to different origin: %s", nextURL)
		}
	}

	// page exceeds MaxPages only when the for-loop post-increment fires,
	// which means the last iteration did NOT break (i.e., it found a Link
	// header pointing to another page we chose not to follow).
	if page > c.httpOpts.MaxPages {
		hasMore = true
		c.logger.Warn("pagination capped", "maxPages", c.httpOpts.MaxPages)
	}

	return allResults, hasMore, nil
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

	// MaxRetries is the total attempt count, floored at one: a cap of 0 means
	// "no retries", not "no request" (SPEC §2 validation step 4). The floor
	// belongs here rather than only at construction because this loop is
	// pre-check — `attempt <= 0` would send nothing at all — and because a
	// directly-struct-built Client never passes through NewClient's validation.
	// Mirrors generated Go's effectiveMaxAttempts, Ruby's [cap, 1].max and
	// Kotlin's computeMaxAttempts.
	maxAttempts := max(c.httpOpts.MaxRetries, 1)

	for attempt = 1; attempt <= maxAttempts; attempt++ {
		resp, err := c.singleRequest(ctx, method, url, body, attempt)
		if err == nil {
			return resp, nil
		}

		// Check for retryable error with server-specified delay
		var delay time.Duration
		if apiErr, ok := err.(*Error); ok {
			if !apiErr.Retryable {
				return nil, err
			}
			lastErr = err
			// A server-specified Retry-After replaces the backoff curve
			// outright — no jitter, no ceiling, same idiom as downloadURL.
			// Only the 429 arm of singleRequest sets it today; widening the
			// set of statuses that carry one is #775's call, not this loop's.
			if apiErr.RetryAfter > 0 {
				delay = time.Duration(apiErr.RetryAfter) * time.Second
			} else {
				delay = c.backoffDelay(attempt)
			}
		} else {
			return nil, err
		}

		// MaxRetries is the total attempt count. After the final attempt there is
		// no retry, so don't sleep the backoff delay or fire OnRetry for an
		// attempt that will never happen — return the last error immediately.
		if attempt >= maxAttempts {
			break
		}

		c.logger.Debug("retrying request", "attempt", attempt, "maxRetries", maxAttempts, "delay", delay, "error", lastErr)

		// Notify hooks about the retry
		info := RequestInfo{Method: method, URL: url, Attempt: attempt}
		c.hooks.OnRetry(ctx, info, attempt+1, lastErr)

		// Cancellation must win over the wait. A server-specified Retry-After
		// carries no ceiling by design, so an uninterruptible sleep would let a
		// server pin a request the caller already abandoned open for as long as
		// it liked.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			continue
		}
	}

	noun := "attempts"
	if maxAttempts == 1 {
		noun = "attempt"
	}
	return nil, fmt.Errorf("request failed after %d %s: %w", maxAttempts, noun, lastErr)
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

	// Backstop: never attach credentials to a foreign origin, even if a future
	// code path reaches here without going through buildURL.
	if err := c.assertCredentialOrigin(req); err != nil {
		return nil, err
	}
	if err := c.authStrategy.Authenticate(ctx, req); err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if requestHasBody(req) && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
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
	resp, err := c.httpClient.Do(req) // #nosec G704 -- SDK HTTP client: URL is caller-configured
	if err != nil {
		return nil, ErrNetwork(err)
	}
	defer func() { _ = resp.Body.Close() }()

	c.logger.Debug("http response", "status", resp.StatusCode)

	requestID := resp.Header.Get(requestIDHeader)

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
		return nil, ErrAPI(304, "304 received but no cached response available").withRequestID(requestID)

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
		return nil, ErrRateLimit(retryAfter).withRequestID(requestID)

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
		return nil, ErrAuth("Authentication failed").withRequestID(requestID)

	case http.StatusForbidden: // 403
		// Check if this might be a scope issue
		if method != "GET" {
			return nil, ErrForbiddenScope().withRequestID(requestID)
		}
		return nil, ErrForbidden("Access denied").withRequestID(requestID)

	case http.StatusNotFound: // 404
		return nil, ErrNotFound("Resource", url).withRequestID(requestID)

	case http.StatusBadRequest, http.StatusUnprocessableEntity: // 400, 422
		// The generated service layer maps these through checkResponse; the raw
		// escape hatch used to fall through to the default arm and report
		// api_error with the field-keyed detail dropped.
		respBody, _ := limitedReadAll(resp.Body, MaxErrorBodyBytes)
		serverMsg, serverHint, fieldErrors := parseErrorBody(respBody)
		return nil, validationError(serverMsg, serverHint, fieldErrors, resp.StatusCode, requestID)

	case http.StatusInsufficientStorage: // 507
		// Same reason the 400/422 arm above exists: the generated service layer
		// maps this through checkResponse, and the raw escape hatch would
		// otherwise fall through to the default arm and report an account limit
		// as a generic api_error. Decided before the 5xx arms — a limit is not a
		// transient failure, and no retry can satisfy it (SPEC §6, step 11).
		respBody, _ := limitedReadAll(resp.Body, MaxErrorBodyBytes)
		serverMsg, serverHint, _ := parseErrorBody(respBody)
		return nil, (&Error{
			Code:       CodeLimitExceeded,
			Message:    msgOrDefault(serverMsg, "account limit reached"),
			Hint:       serverHint,
			HTTPStatus: 507,
			Retryable:  false,
		}).withRequestID(requestID)

	case http.StatusInternalServerError: // 500
		return nil, ErrAPI(500, "Server error (500)").withRequestID(requestID)

	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout: // 502, 503, 504
		return nil, (&Error{
			Code:       CodeAPI,
			Message:    fmt.Sprintf("Gateway error (%d)", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Retryable:  true,
		}).withRequestID(requestID)

	default:
		// One SPEC §6 parser, shared with checkResponse: members decode
		// independently and messages are truncated, where the struct decode
		// this arm used before failed whole on any unexpected member type.
		respBody, _ := limitedReadAll(resp.Body, MaxErrorBodyBytes)
		serverMsg, serverHint, _ := parseErrorBody(respBody)
		// Retryability is left exactly as ErrAPI set it — untouched here so the
		// retry contract stays the one §7 pins.
		return nil, (&Error{
			Code:       CodeAPI,
			Message:    msgOrDefault(serverMsg, fmt.Sprintf("Request failed (HTTP %d)", resp.StatusCode)),
			Hint:       serverHint,
			HTTPStatus: resp.StatusCode,
		}).withRequestID(requestID)
	}
}

func (c *Client) buildURL(path string) (string, error) {
	// Schemes are case-insensitive (RFC 3986), so detect absolute URLs on a
	// lowercased copy — otherwise HTTPS://... would be mis-treated as a
	// relative path and concatenated onto the base URL.
	lowerPath := strings.ToLower(path)
	// Absolute URLs: enforce HTTPS and return as-is (e.g., pagination Link headers)
	if strings.HasPrefix(lowerPath, "https://") {
		// Absolute URLs must target the configured origin; otherwise we would
		// attach the bearer token to a foreign host (credential exfiltration).
		// Localhost targets are carved out for local development and httptest
		// servers, matching the same-origin guard already applied to pagination
		// Link headers and cross-origin redirects.
		if isLocalhost(path) || isSameOrigin(path, c.cfg.BaseURL) {
			return path, nil
		}
		return "", fmt.Errorf("absolute URL points to a different origin than base URL: %s", truncateString(path, MaxErrorMessageBytes))
	}
	if strings.HasPrefix(lowerPath, "http://") {
		// Localhost may use plain HTTP for local development and httptest
		// servers, matching the BaseURL check in NewClient and the localhost
		// carve-out in the other SDKs.
		if isLocalhost(path) {
			return path, nil
		}
		return "", fmt.Errorf("URL must use HTTPS, got: %s", truncateString(path, MaxErrorMessageBytes))
	}
	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Normalize BaseURL to avoid double slashes when concatenating
	base := strings.TrimSuffix(c.cfg.BaseURL, "/")
	return base + path, nil
}

// assertCredentialOrigin verifies the outgoing request targets the configured
// origin before any credential is attached. Localhost is carved out for local
// development and httptest servers. This attach-point backstop complements the
// buildURL chokepoint and fails closed if a future code path reaches the
// token-attach site without going through buildURL.
func (c *Client) assertCredentialOrigin(req *http.Request) error {
	target := req.URL.String()
	if isLocalhost(target) || isSameOrigin(target, c.cfg.BaseURL) {
		return nil
	}
	return fmt.Errorf("refusing to attach credentials to a different origin than base URL: %s", truncateString(target, MaxErrorMessageBytes))
}

func (c *Client) backoffDelay(attempt int) time.Duration {
	delay := saturatingBackoff(c.httpOpts.BaseDelay, attempt)

	// Add jitter
	jitter := time.Duration(rand.Int63n(int64(c.httpOpts.MaxJitter))) // #nosec G404 -- jitter doesn't need cryptographic randomness

	return delay + jitter
}

// saturatingBackoff computes base * 2^(attempt-1) for a 1-based attempt,
// saturating at MaxBackoffDelay (SPEC §7, "Backoff Ceiling").
//
// The clamp is load-bearing. `1 << (attempt-1)` is evaluated in int: at attempt
// 64 that is 1<<63, which is negative, and Go defines a shift at or past the
// operand width as 0. Either way the product stops being a delay — and both
// time.Sleep and time.After treat a non-positive duration as "no wait", so an
// unclamped formula turned a long failure streak into a retry loop with no
// backoff at all, against a server already answering 429/503.
//
// The multiplier is compared against MaxBackoffDelay/base before multiplying,
// so no intermediate can overflow int64.
func saturatingBackoff(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	// 62 is the largest shift that stays positive in int64; anything at or past
	// it is far above any ceiling worth computing.
	if shift >= 62 {
		return MaxBackoffDelay
	}
	// The multiplier is a plain count, not a duration — multiplying two
	// time.Durations would be a unit error (and golangci's durationcheck says
	// so), so the arithmetic happens in int64 nanoseconds.
	multiplier := int64(1) << uint(shift)
	if multiplier > int64(MaxBackoffDelay)/int64(base) {
		return MaxBackoffDelay
	}
	return time.Duration(int64(base) * multiplier)
}

// extractAngleBracketed returns the contents of the first non-empty <...> pair,
// or "" if there is none.
//
// Searching for ">" from after the "<" is what makes this correct: looking for
// both independently from position 0 means a ">" that precedes the "<" yields
// end < start, and the extraction silently fails for that part. It also matches
// the leftmost-match semantics of <([^>]+)>, which is what the TypeScript, Ruby
// and Python SDKs used to spell this with.
//
// An empty <> is skipped rather than returned, because [^>]+ requires at least
// one character. That scan was O(1), so the loop stays linear overall.
func extractAngleBracketed(part string) string {
	for cursor := 0; cursor < len(part); {
		start := strings.Index(part[cursor:], "<")
		if start < 0 {
			return ""
		}
		start += cursor

		end := strings.Index(part[start+1:], ">")
		if end < 0 {
			return ""
		}
		end += start + 1

		if end > start+1 {
			return part[start+1 : end]
		}
		cursor = start + 1
	}

	return ""
}

// parseNextLink extracts the next URL from a Link header.
func parseNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	for part := range strings.SplitSeq(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			if url := extractAngleBracketed(part); url != "" {
				return url
			}
		}
	}

	return ""
}

// maxRetryAfterSeconds is the largest delta-seconds value this SDK honours:
// 2147483647, ~68 years. It is a REPRESENTABILITY bound taken at the portable
// limit, not a policy ceiling — SPEC §7's "Retry-After is exempt" note already
// carves out exactly this ("implementations may still bound it against host
// limits"), and Kotlin saturates this same header at the same number while
// Swift clamps its own seconds→nanoseconds conversion for the same class of
// reason. Deciding a *policy* cap on server-directed waits is #793's, and this
// is not one: at ~68 years, nothing a server could sensibly ask for is
// affected.
//
// Two host limits sit above it and this is at or below both, which is why the
// answer does not depend on the word size: `seconds × time.Second` wraps past
// math.MaxInt64 above ~292 years, and RetryAfter is an `int`, which is 32 bits
// on a 32-bit target. Taking the smaller keeps one documented number for every
// platform — the same 2147483647 SPEC §16 already names as a shared cross-SDK
// ceiling — instead of an answer a reader has to compute from GOARCH.
const maxRetryAfterSeconds = math.MaxInt32

// clampRetryAfterSeconds normalizes a delta-seconds value to the range Error's
// RetryAfter field promises: non-negative, and small enough that the retry
// loops' `time.Duration(n) * time.Second` stays positive on every target.
//
// The failure this closes is not hypothetical arithmetic. `Retry-After:
// 9223372036854775807` parses cleanly on a 64-bit build, and the product wraps
// to -1s; `time.After` on a non-positive duration fires immediately, so a
// server-directed wait becomes a tight retry loop — the opposite of what the
// header asked for.
//
// Over-range clamps rather than falling back to "absent" because clamping is
// what honours the server: falling back would compute the millisecond backoff
// curve instead and hammer a peer that just asked for a long wait, which is the
// same tight loop by another route. The wait is a select on ctx.Done(), so an
// absurd clamped delay is abandonable rather than a hang.
//
// It takes an int64 because both callers have one — the date branch derives
// seconds from a time.Duration, and the delta-seconds branch parses into 64
// bits so the answer cannot depend on the word size. A single comparison
// against a bound below every target's int range is also what makes the
// narrowing legible to CodeQL's go/incorrect-integer-conversion, which reads
// the guard rather than the arithmetic and flagged the previous spelling.
func clampRetryAfterSeconds(seconds int64) int {
	if seconds <= 0 {
		return 0
	}
	if seconds > maxRetryAfterSeconds {
		return maxRetryAfterSeconds
	}
	return int(seconds)
}

// isDelaySeconds reports whether the value is RFC 9110's `1*DIGIT` and nothing
// else: no sign, no space, no separator, no decimal point.
func isDelaySeconds(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseRetryAfter parses the Retry-After header value.
// It handles both seconds (integer) and HTTP-date formats.
// Returns 0 if the header is empty or cannot be parsed, and clamps a parsed
// value to what a time.Duration can hold — every caller multiplies the result
// by time.Second.
func parseRetryAfter(header string) int {
	if header == "" {
		return 0
	}
	// Try parsing as seconds (integer). Parsed as an int64 rather than through
	// Atoi, whose range is int's: `Retry-After: 2147483648` would otherwise be
	// ErrRange, hence malformed, hence the millisecond backoff on a 32-bit
	// build while the same header is honoured on a 64-bit one. Deciding the
	// ceiling is the clamp's job, not the parse's.
	//
	// A POSITIVE range error saturates too, and deliberately. ParseInt reports
	// ErrRange with the value already clamped to math.MaxInt64, so
	// `9223372036854775808` — one past the largest int64, and still `1*DIGIT`,
	// which is all RFC 9110 requires of delay-seconds — lands on the ceiling
	// like every other over-range value instead of falling off a boundary that
	// exists only because of Go's word size. The alternative would honour
	// 9223372036854775807 and hammer the server for 9223372036854775808, one
	// digit apart, which no reader could predict and no server could intend.
	// Truly malformed input ("120junk", "-5", "") still falls through: ParseInt
	// returns 0 with ErrSyntax, and a negative range error clamps to
	// math.MinInt64, both caught by the `> 0` guard.
	//
	// TypeScript (`Number.isSafeInteger`) and Kotlin (`toIntOrNull`) fall back
	// to the backoff curve at their own parse limits rather than saturating;
	// that divergence is #799.
	//
	// A value too large for that int64 is MALFORMED and falls through to step
	// 3's backoff — SPEC §6's first tier — rather than saturating. The second
	// tier, saturation, is for a value the parser holds but the host cannot
	// schedule, and that is clampRetryAfterSeconds' job below. The tiers are
	// stated once in SPEC and deliberately not restated here.
	//
	// The digits are checked rather than left to ParseInt, which accepts a
	// leading `+` or `-`. RFC 9110 spells delay-seconds as `1*DIGIT` — no sign
	// — so `+5` is not a delay at all, and ParseInt would otherwise honour it
	// as 5. Same digits-only test SPEC §16's device parser makes, and the same
	// reading conformance's "partly numeric rejected (`1*DIGIT`)" case asserts.
	if isDelaySeconds(header) {
		if seconds, err := strconv.ParseInt(header, 10, 64); err == nil && seconds > 0 {
			return clampRetryAfterSeconds(seconds)
		}
	}
	// Try parsing as HTTP-date (e.g., "Wed, 21 Oct 2015 07:28:00 GMT")
	if t, err := http.ParseTime(header); err == nil {
		// Whole seconds by integer division of the Duration, NOT via
		// `int(d.Seconds())`. That spelling narrows a float64 to an int, and
		// Go leaves an out-of-range float→int conversion implementation-
		// defined: where int is 32 bits, a date more than ~68 years out (and
		// time.Until saturates at ~292 for anything further, including the
		// year-9999 dates a server can legally send) produced a garbage value
		// that read as non-positive, so the header was discarded and the loop
		// fell back to its millisecond backoff — the one outcome this clamp
		// exists to avoid. Integer division cannot leave the Duration's range,
		// and clampRetryAfterSeconds bounds the result by int's.
		//
		// Rounded UP, matching Kotlin's `(remainingMs + 999) / 1000`, Swift's
		// `.rounded(.up)` and TypeScript's `Math.ceil`, whose shared reason is
		// that truncating a sub-second remainder toward zero turns the
		// shortest honoured delay into "retry immediately": a date 400ms out
		// became 0, was read as "no delay", and fell onto the backoff curve.
		// Rounding up also never retries before the moment the server named.
		// Python and Ruby still truncate; that half of the divergence is #799.
		if remaining := time.Until(t); remaining > 0 {
			seconds := int64(remaining / time.Second)
			if remaining%time.Second != 0 {
				seconds++
			}
			return clampRetryAfterSeconds(seconds)
		}
	}
	return 0
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

// HillCharts returns the HillChartsService for hill chart operations.
func (ac *AccountClient) HillCharts() *HillChartsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.hillCharts == nil {
		ac.hillCharts = NewHillChartsService(ac)
	}
	return ac.hillCharts
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

// Calendars returns the CalendarsService for per-account calendars.
func (ac *AccountClient) Calendars() *CalendarsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.calendars == nil {
		ac.calendars = NewCalendarsService(ac)
	}
	return ac.calendars
}

// MyNotes returns the MyNotesService for the current user's scratchpad note.
func (ac *AccountClient) MyNotes() *MyNotesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.myNotes == nil {
		ac.myNotes = NewMyNotesService(ac)
	}
	return ac.myNotes
}

// Drafts returns the DraftsService for the current user's unpublished drafts.
func (ac *AccountClient) Drafts() *DraftsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.drafts == nil {
		ac.drafts = NewDraftsService(ac)
	}
	return ac.drafts
}

// Bookmarks returns the BookmarksService for the current user's bookmarks.
func (ac *AccountClient) Bookmarks() *BookmarksService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.bookmarks == nil {
		ac.bookmarks = NewBookmarksService(ac)
	}
	return ac.bookmarks
}

// Folders returns the FoldersService for the current user's home-screen folders.
func (ac *AccountClient) Folders() *FoldersService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.folders == nil {
		ac.folders = NewFoldersService(ac)
	}
	return ac.folders
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

// CloudFiles returns the CloudFilesService for cloud file operations.
func (ac *AccountClient) CloudFiles() *CloudFilesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.cloudFiles == nil {
		ac.cloudFiles = NewCloudFilesService(ac)
	}
	return ac.cloudFiles
}

// GoogleDocuments returns the GoogleDocumentsService for Google document operations.
func (ac *AccountClient) GoogleDocuments() *GoogleDocumentsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.googleDocuments == nil {
		ac.googleDocuments = NewGoogleDocumentsService(ac)
	}
	return ac.googleDocuments
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

// Wormholes returns the WormholesService for card-table wormhole operations.
func (ac *AccountClient) Wormholes() *WormholesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.wormholes == nil {
		ac.wormholes = NewWormholesService(ac)
	}
	return ac.wormholes
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

// Everything returns the EverythingService for account-wide aggregate listings.
func (ac *AccountClient) Everything() *EverythingService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.everything == nil {
		ac.everything = NewEverythingService(ac)
	}
	return ac.everything
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

// Account returns the AccountService for account operations.
func (ac *AccountClient) Account() *AccountService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.account == nil {
		ac.account = NewAccountService(ac)
	}
	return ac.account
}

// Gauges returns the GaugesService for gauge operations.
func (ac *AccountClient) Gauges() *GaugesService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.gauges == nil {
		ac.gauges = NewGaugesService(ac)
	}
	return ac.gauges
}

// MyAssignments returns the MyAssignmentsService for assignment operations.
func (ac *AccountClient) MyAssignments() *MyAssignmentsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.myAssignments == nil {
		ac.myAssignments = NewMyAssignmentsService(ac)
	}
	return ac.myAssignments
}

// MyNotifications returns the MyNotificationsService for notification operations.
func (ac *AccountClient) MyNotifications() *MyNotificationsService {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.myNotifications == nil {
		ac.myNotifications = NewMyNotificationsService(ac)
	}
	return ac.myNotifications
}
