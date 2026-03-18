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

	resp, err := httpClient.Do(req)
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

	// URL rewriting: replace scheme+host with cfg.BaseURL origin, preserve path+query
	baseURL, baseErr := url.Parse(ac.parent.cfg.BaseURL)
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

	// Hop 1: Authenticated API request (capture redirect)
	req, err := http.NewRequestWithContext(ctx, "GET", rewrittenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if authErr := ac.parent.authStrategy.Authenticate(ctx, req); authErr != nil {
		return nil, authErr
	}
	req.Header.Set("User-Agent", ac.parent.userAgent)

	apiClient := &http.Client{
		Transport: ac.parent.httpClient.Transport, // loggingTransport — fires hooks
		Timeout:   0,                              // no client-level timeout — body may be streamed on direct 2xx
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ctx = contextWithAttempt(ctx, 1)
	req = req.WithContext(ctx)

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, ErrNetwork(err)
	}

	// Dispatch on response status
	switch {
	case resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 ||
		resp.StatusCode == 307 || resp.StatusCode == 308:
		// Redirect — extract Location, close body, proceed to hop 2
		location := resp.Header.Get("Location")
		_ = resp.Body.Close()
		if location == "" {
			return nil, ErrAPI(resp.StatusCode, fmt.Sprintf("redirect %d with no Location header", resp.StatusCode))
		}
		// Resolve relative Location against the rewritten API URL
		resolvedLocation := resolveURL(rewrittenURL, location)

		// Hop 2: fetch from signed URL
		signedResp, signedErr := ac.parent.fetchSignedDownload(ctx, resolvedLocation) //nolint:bodyclose // body ownership transfers to caller via DownloadResult
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
		// Direct download — no second hop
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
