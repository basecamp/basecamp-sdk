package basecamp

import (
	"context"
	"net/http"
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

// HTTPOptions configures the HTTP client behavior.
type HTTPOptions struct {
	// Timeout is the request timeout (default: 30s).
	Timeout time.Duration

	// MaxRetries is the maximum retry attempts for GET requests (default: 3).
	// POST/PUT/DELETE requests only get 1 retry after successful token refresh.
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

// WithMaxRetries sets the maximum number of retry attempts for GET requests.
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

// retryableError wraps an error with retry metadata.
// This allows respecting Retry-After headers from 429 responses.
type retryableError struct {
	err        error
	retryAfter time.Duration
}

func (r *retryableError) Error() string {
	return r.err.Error()
}

func (r *retryableError) Unwrap() error {
	return r.err
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

// loggingTransport wraps an http.RoundTripper to log requests and responses,
// and calls observability hooks for all HTTP requests (including generated client).
// It holds a pointer to the client so it can access the current logger and hooks.
type loggingTransport struct {
	inner  http.RoundTripper
	client *Client
}

// RoundTrip implements http.RoundTripper with logging and hooks.
func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Call hooks before request
	info := RequestInfo{
		Method:  req.Method,
		URL:     req.URL.String(),
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
			"url", req.URL.String())
	}

	resp, err := t.inner.RoundTrip(req)

	// Record result
	if err != nil {
		result.Error = err
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
