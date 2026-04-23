package basecamp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

// fetchSignedDownload fetches content from a signed download URL (e.g., S3).
// Uses the bare transport (no loggingTransport, no auth headers) and no
// client-level timeout so the caller owns the streaming lifecycle.
func (c *Client) fetchSignedDownload(ctx context.Context, downloadURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	transport := c.httpOpts.Transport
	if transport == nil {
		transport = newDefaultTransport()
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   0, // no client-level timeout — streaming owned by caller
	}

	resp, err := httpClient.Do(req) // #nosec G704 -- SDK HTTP client: URL is caller-configured
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, ErrAPI(resp.StatusCode, fmt.Sprintf("download failed with status %d", resp.StatusCode))
	}

	return resp, nil
}

// DownloadURL fetches file content from any API-routable download URL.
//
// Handles the full download flow: URL rewriting to the configured API host,
// authenticated first hop (which typically 302s to a signed download URL),
// and unauthenticated second hop to fetch the actual file content. Common
// inputs include storage blob URLs from <bc-attachment> elements and any
// other signed-download URL that routes through the API.
//
// The caller is responsible for closing the returned Body.
func (ac *AccountClient) DownloadURL(ctx context.Context, rawURL string) (result *DownloadResult, err error) {
	// Validation
	if rawURL == "" {
		return nil, ErrUsage("download URL is required")
	}
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil || !parsed.IsAbs() {
		return nil, ErrUsage("download URL must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrUsage("download URL must use HTTP or HTTPS scheme")
	}

	// Operation hooks
	op := OperationInfo{
		Service: "Account", Operation: "DownloadURL",
		ResourceType: "download", IsMutation: false,
	}
	if gater, ok := ac.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = ac.parent.hooks.OnOperationStart(ctx, op)
	defer func() { ac.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	return ac.parent.fetchAPIDownload(ctx, rawURL)
}

// fetchAPIDownload executes the authenticated-hop + optional 302-follow flow
// used by both AccountClient.DownloadURL and UploadsService.Download. It
// rewrites the URL's host to the configured API base, authenticates the first
// hop, and either returns the 2xx body directly or follows a 3xx Location
// through an unauthenticated second hop to a signed URL.
//
// The authenticated hop is wrapped in the SDK-standard GET retry loop:
// network errors and 429/502/503/504 responses are retried up to MaxRetries
// with exponential backoff, honoring Retry-After on 429. Other 5xx statuses
// (500 and up) are surfaced without retry, matching Client.singleRequest's
// @retryable markings. Retries stop once the response enters 2xx/3xx
// dispatch — the body then belongs to the caller (2xx direct) or has
// already been discarded in favor of the Location hop (3xx). Not sharing
// doWithRetry because that path is tightly coupled to the JSON-response
// generated client; this loop owns raw *http.Response.
//
// Callers own operation-hook lifecycle and are responsible for closing the
// returned Body. Filename is derived from rawURL; callers may override.
func (c *Client) fetchAPIDownload(ctx context.Context, rawURL string) (*DownloadResult, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrUsage("download URL must be a valid URL")
	}
	// Defense-in-depth: AccountClient.DownloadURL already validates user input,
	// but UploadsService.Download passes upload.download_url straight from the
	// API response. Reject relative or non-http(s) URLs here so a malformed
	// field can't silently collapse to requesting the API base root.
	if !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, ErrUsage("download URL must be an absolute http or https URL")
	}

	baseURL, baseErr := url.Parse(c.cfg.BaseURL)
	if baseErr != nil {
		return nil, fmt.Errorf("invalid base URL: %w", baseErr)
	}
	rewritten := &url.URL{
		Scheme:   baseURL.Scheme,
		Host:     baseURL.Host,
		Path:     parsed.Path,
		RawQuery: parsed.RawQuery,
		Fragment: parsed.Fragment,
	}
	rewrittenURL := rewritten.String()

	apiClient := &http.Client{
		Transport: c.httpClient.Transport, // loggingTransport — fires hooks
		Timeout:   0,                      // no client-level timeout — body may be streamed on direct 2xx
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// MaxRetries is the total attempt count for this loop, matching
	// Client.doRequestURL's iteration. MaxRetries<=0 skips the loop entirely
	// and is surfaced as ErrUsage by the fallback after the loop. On
	// exhaustion, the last per-attempt error is returned directly. Retry-
	// eligible statuses are aligned with the main GET loop's @retryable set:
	// 429 (rate limit) and 502/503/504 (gateway errors), plus transport
	// errors. 500 and other non-@retryable 5xx fall through to the dispatch
	// switch and surface as errors without retry.
	maxAttempts := c.httpOpts.MaxRetries

	var resp *http.Response
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptCtx := contextWithAttempt(ctx, attempt)

		req, reqErr := http.NewRequestWithContext(attemptCtx, "GET", rewrittenURL, nil)
		if reqErr != nil {
			return nil, fmt.Errorf("failed to create request: %w", reqErr)
		}
		if authErr := c.authStrategy.Authenticate(attemptCtx, req); authErr != nil {
			return nil, authErr
		}
		req.Header.Set("User-Agent", c.userAgent)

		r, doErr := apiClient.Do(req) // #nosec G704 -- SDK HTTP client: URL is caller-configured

		var retryAfter int
		switch {
		case doErr != nil:
			lastErr = ErrNetwork(doErr)
		case r.StatusCode == http.StatusTooManyRequests ||
			r.StatusCode == http.StatusBadGateway ||
			r.StatusCode == http.StatusServiceUnavailable ||
			r.StatusCode == http.StatusGatewayTimeout:
			// Drain up to MaxErrorBodyBytes so the connection can return to
			// the keep-alive pool before the next retry. checkResponse only
			// uses the first maxErrorMessageLen*2 bytes for the error message.
			drained, _ := io.ReadAll(io.LimitReader(r.Body, MaxErrorBodyBytes))
			_ = r.Body.Close()
			bodyForErr := drained
			if len(bodyForErr) > maxErrorMessageLen*2 {
				bodyForErr = bodyForErr[:maxErrorMessageLen*2]
			}
			lastErr = checkResponse(r, bodyForErr)
			if r.StatusCode == http.StatusTooManyRequests {
				retryAfter = parseRetryAfter(r.Header.Get("Retry-After"))
			}
		default:
			resp = r
		}

		if resp != nil {
			break
		}

		if attempt >= maxAttempts {
			return nil, lastErr
		}

		delay := c.backoffDelay(attempt)
		if retryAfter > 0 {
			delay = time.Duration(retryAfter) * time.Second
		}
		info := RequestInfo{Method: "GET", URL: rewrittenURL, Attempt: attempt}
		c.hooks.OnRetry(ctx, info, attempt+1, lastErr)
		c.logger.Debug("retrying download request", "attempt", attempt, "maxRetries", maxAttempts, "delay", delay, "error", lastErr)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if resp == nil {
		// Exhaustion after real attempts is handled inside the loop (`return
		// nil, lastErr`). The only path that reaches this fallback is a
		// misconfigured MaxRetries<=0 that skips the loop entirely.
		return nil, ErrUsage(fmt.Sprintf("download aborted: MaxRetries (%d) must be >= 1", maxAttempts))
	}

	switch {
	case resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 ||
		resp.StatusCode == 307 || resp.StatusCode == 308:
		location := resp.Header.Get("Location")
		// Drain a bounded prefix of the body before close so the underlying
		// connection can be returned to the keep-alive pool for hop 2.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if location == "" {
			return nil, ErrAPI(resp.StatusCode, fmt.Sprintf("redirect %d with no Location header", resp.StatusCode))
		}
		resolvedLocation := resolveURL(rewrittenURL, location)

		signedResp, signedErr := c.fetchSignedDownload(ctx, resolvedLocation) //nolint:bodyclose // body ownership transfers to caller via DownloadResult
		if signedErr != nil {
			return nil, signedErr
		}
		return &DownloadResult{
			Body:          signedResp.Body,
			ContentType:   signedResp.Header.Get("Content-Type"),
			ContentLength: signedResp.ContentLength,
			Filename:      filenameFromURL(rawURL),
		}, nil

	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return &DownloadResult{
			Body:          resp.Body,
			ContentType:   resp.Header.Get("Content-Type"),
			ContentLength: resp.ContentLength,
			Filename:      filenameFromURL(rawURL),
		}, nil

	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorMessageLen*2))
		_ = resp.Body.Close()
		return nil, checkResponse(resp, body)
	}
}

// filenameFromURL extracts a filename from the last path segment of a URL.
// Falls back to "download" if the URL is unparseable or has no path segments.
func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "download"
	}
	base := path.Base(u.Path)
	if base == "" || base == "." || base == "/" {
		return "download"
	}
	unescaped, err := url.PathUnescape(base)
	if err != nil {
		return base
	}
	return unescaped
}
