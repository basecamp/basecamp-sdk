package basecamp

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Default values for HTTP client configuration.
// These can be overridden using functional options.
const (
	DefaultMaxRetries = 3
	DefaultBaseDelay  = 1 * time.Second
	DefaultMaxJitter  = 100 * time.Millisecond
	DefaultTimeout    = 30 * time.Second
	DefaultMaxPages   = 10000
)

// MaxBackoffDelay is the ceiling on the exponential backoff term (SPEC §7,
// "Backoff Ceiling"). Jitter is added after the clamp, so the longest single
// backoff sleep is MaxBackoffDelay + MaxJitter.
//
// It matches the generated client's RetryConfig.MaxDelay, which has capped at
// 30s since that retry loop was first templated; #577 generalized the value to
// every backoff site in every SDK.
const MaxBackoffDelay = 30 * time.Second

// HTTPOptions configures the HTTP client behavior.
type HTTPOptions struct {
	// Timeout is the request timeout (default: 30s).
	Timeout time.Duration

	// MaxRetries is the total attempt count for retryable requests (default: 3;
	// 0 means no retry, exactly one attempt — NewClient panics only on a
	// negative value). It governs the raw GET and download loops here and the
	// generated client's retry loop behind every typed operation.
	// POST/PUT/DELETE on the raw path make one attempt plus one retry after a
	// successful token refresh.
	MaxRetries int

	// BaseDelay is the initial backoff delay (default: 1s).
	BaseDelay time.Duration

	// MaxJitter is the maximum random jitter to add to delays (default: 100ms).
	MaxJitter time.Duration

	// MaxPages is the maximum pages to fetch in GetAll (default: 10000).
	MaxPages int

	// Transport is the HTTP transport to use. If nil, a default transport
	// with sensible connection pooling is created.
	Transport http.RoundTripper
}

// DefaultHTTPOptions returns HTTPOptions with sensible defaults.
func DefaultHTTPOptions() HTTPOptions {
	return HTTPOptions{
		Timeout:    DefaultTimeout,
		MaxRetries: DefaultMaxRetries,
		BaseDelay:  DefaultBaseDelay,
		MaxJitter:  DefaultMaxJitter,
		MaxPages:   DefaultMaxPages,
	}
}

// WithTimeout sets the HTTP request timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpOpts.Timeout = d
	}
}

// WithMaxRetries sets the total attempt count for retryable requests: the raw
// Get/GetAll and download paths, and the generated client's own retry loop
// behind every typed operation.
//
// The count includes the initial request, so 3 means one attempt plus two
// retries. Zero is legal and means "no retries — exactly one attempt"; every
// loop floors the cap at one, so a request is always made. Only a negative
// value is a configuration error, and NewClient panics on it.
func WithMaxRetries(n int) ClientOption {
	return func(c *Client) {
		c.httpOpts.MaxRetries = n
	}
}

// WithBaseDelay sets the initial backoff delay.
func WithBaseDelay(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpOpts.BaseDelay = d
	}
}

// WithMaxJitter sets the maximum random jitter to add to delays.
func WithMaxJitter(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpOpts.MaxJitter = d
	}
}

// WithMaxPages sets the maximum pages to fetch in GetAll.
func WithMaxPages(n int) ClientOption {
	return func(c *Client) {
		c.httpOpts.MaxPages = n
	}
}

// WithTransport sets a custom HTTP transport.
func WithTransport(t http.RoundTripper) ClientOption {
	return func(c *Client) {
		c.httpOpts.Transport = t
	}
}

// newDefaultTransport creates an HTTP transport with sensible defaults.
// It clones http.DefaultTransport to preserve proxy settings, HTTP/2, TLS config.
func newDefaultTransport() http.RoundTripper {
	// Clone DefaultTransport to preserve proxy, HTTP/2, dial timeouts, TLS
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxIdleConnsPerHost = 10
	t.IdleConnTimeout = 90 * time.Second
	return t
}

// attemptKey is the context key for tracking request attempt number.
type attemptKey struct{}

// contextWithAttempt adds the request attempt number to the context.
func contextWithAttempt(ctx context.Context, attempt int) context.Context {
	return context.WithValue(ctx, attemptKey{}, attempt)
}

// attemptFromContext extracts the attempt number from context (defaults to 1).
func attemptFromContext(ctx context.Context) int {
	if v := ctx.Value(attemptKey{}); v != nil {
		if attempt, ok := v.(int); ok {
			return attempt
		}
	}
	return 1
}

// downloadRequestKey is the context key marking a download hop-1 request, so
// loggingTransport renders its URL as origin+path only (SPEC §9): the
// caller-supplied download URL can smuggle a signed query into that hop.
type downloadRequestKey struct{}

// markDownloadRequest marks the context as belonging to a download hop-1 request.
func markDownloadRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, downloadRequestKey{}, true)
}

// isDownloadRequest reports whether the context carries the download marker.
func isDownloadRequest(ctx context.Context) bool {
	v, _ := ctx.Value(downloadRequestKey{}).(bool)
	return v
}

// loggingTransport wraps an http.RoundTripper to log requests and responses,
// and calls observability hooks for all HTTP requests (including generated client).
// It holds a pointer to the client so it can access the current logger and hooks.
type loggingTransport struct {
	inner  http.RoundTripper
	client *Client
}

// RoundTrip implements http.RoundTripper with logging and hooks.
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// A download hop-1 request is projected for hooks and logs (SPEC §9): its
	// URL as origin+path only, and a transport failure as the fixed network
	// error, since *url.Error renders the URL it failed on. Every other
	// request URL carries no credential — the token rides in the
	// Authorization header — so hooks and logs get it whole.
	download := isDownloadRequest(req.Context())
	displayURL := req.URL.String()
	if download {
		displayURL = (&url.URL{Scheme: req.URL.Scheme, Host: req.URL.Host, Path: req.URL.Path}).String()
	}

	// Call hooks before request
	info := RequestInfo{
		Method:  req.Method,
		URL:     displayURL,
		Attempt: attemptFromContext(req.Context()),
	}
	hookCtx := t.client.hooks.OnRequestStart(req.Context(), info)
	startTime := time.Now()

	// Update request context with hook context for trace propagation
	req = req.WithContext(hookCtx)

	// Track result for hooks
	var result RequestResult
	defer func() {
		result.Duration = time.Since(startTime)
		t.client.hooks.OnRequestEnd(hookCtx, info, result)
	}()

	// Log request if logger is enabled
	if t.client.logger != nil {
		t.client.logger.Debug("http request",
			"method", req.Method,
			"url", displayURL)
	}

	resp, err := t.inner.RoundTrip(req)

	// Record result
	if err != nil {
		result.Error = err
		if download {
			result.Error = downloadNetworkError(err)
		}
	} else {
		result.StatusCode = resp.StatusCode
		// Parse Retry-After header for 429/503 responses
		if resp.StatusCode == 429 || resp.StatusCode == 503 {
			result.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
		}
		// Log response if logger is enabled
		if t.client.logger != nil {
			t.client.logger.Debug("http response",
				"status", resp.StatusCode)
		}
	}

	return resp, err
}
