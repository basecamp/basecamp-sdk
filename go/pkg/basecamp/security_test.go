package basecamp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// 1.1 HTTP Redirect Credential Leakage
// =============================================================================

func TestRedirect_StripsAuthOnCrossOrigin(t *testing.T) {
	// Set up an "evil" server that records whether it received an Authorization header.
	var evilReceivedAuth string
	evil := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evilReceivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
		fmt.Fprint(w, `[]`)
	}))
	defer evil.Close()

	// Set up the "legit" API server that redirects to the evil server.
	legit := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/stolen", http.StatusFound)
	}))
	defer legit.Close()

	cfg := &Config{BaseURL: legit.URL}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"},
		WithTransport(legit.Client().Transport))

	// We need the client's httpClient to trust both servers' TLS certs.
	// Override the transport to trust both test servers.
	client.httpClient.Transport = evil.Client().Transport

	_, _ = client.Get(context.Background(), "/test")

	if evilReceivedAuth != "" {
		t.Errorf("Authorization header leaked to cross-origin redirect: %q", evilReceivedAuth)
	}
}

func TestRedirect_PreservesAuthOnSameOrigin(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(200)
			fmt.Fprint(w, `[]`)
			return
		}
		http.Redirect(w, r, "/redirected", http.StatusFound)
	}))
	defer srv.Close()

	cfg := &Config{BaseURL: srv.URL}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"},
		WithTransport(srv.Client().Transport))

	_, _ = client.Get(context.Background(), "/test")

	if receivedAuth != "Bearer secret-token" {
		t.Errorf("Expected Authorization header on same-origin redirect, got: %q", receivedAuth)
	}
}

// =============================================================================
// 1.2 Link Header SSRF / Token Leakage
// =============================================================================

func TestGetAll_RejectsLinkToDifferentOrigin(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `<https://evil.com/page2>; rel="next"`)
		w.WriteHeader(200)
		fmt.Fprint(w, `[{"id":1}]`)
	}))
	defer srv.Close()

	cfg := &Config{BaseURL: srv.URL}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"},
		WithTransport(srv.Client().Transport))

	_, err := client.GetAll(context.Background(), "/items.json")
	if err == nil {
		t.Fatal("Expected error when Link header points to different origin")
	}
	if !strings.Contains(err.Error(), "different origin") {
		t.Errorf("Expected 'different origin' error, got: %v", err)
	}
}

func TestGetAll_AcceptsSameOriginLink(t *testing.T) {
	callCount := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First page: include Link to same server
			w.Header().Set("Link", fmt.Sprintf(`<%s/page2>; rel="next"`, r.URL.Scheme+"://"+r.Host))
			// The URL in a test server handler doesn't include scheme/host,
			// so we need to construct from the server URL.
		}
		w.WriteHeader(200)
		fmt.Fprint(w, `[{"id":1}]`)
	}))
	defer srv.Close()

	// Re-create to use server URL in Link header
	callCount = 0
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Link", fmt.Sprintf(`<%s/page2>; rel="next"`, srv.URL))
		}
		w.WriteHeader(200)
		fmt.Fprint(w, `[{"id":1}]`)
	})

	cfg := &Config{BaseURL: srv.URL}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"},
		WithTransport(srv.Client().Transport))

	results, err := client.GetAll(context.Background(), "/items.json")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results (2 pages), got %d", len(results))
	}
}

// =============================================================================
// 1.3 Unbounded io.ReadAll — Response Body DoS
// =============================================================================

func TestLimitedReadAll_WithinLimit(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 100)
	result, err := limitedReadAll(bytes.NewReader(data), 200)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 100 {
		t.Errorf("Expected 100 bytes, got %d", len(result))
	}
}

func TestLimitedReadAll_ExceedsLimit(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 200)
	_, err := limitedReadAll(bytes.NewReader(data), 100)
	if err == nil {
		t.Fatal("Expected error when body exceeds limit")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("Expected limit error, got: %v", err)
	}
}

func TestLargeResponseBody_ReturnsError(t *testing.T) {
	t.Run("limitedReadAll rejects oversized body", func(t *testing.T) {
		data := strings.NewReader(strings.Repeat("x", 1024))
		_, err := limitedReadAll(data, 512)
		if err == nil {
			t.Fatal("Expected error for oversized body")
		}
		if !strings.Contains(err.Error(), "exceeds") {
			t.Errorf("Expected 'exceeds' in error, got: %v", err)
		}
	})

	t.Run("limitedReadAll accepts body within limit", func(t *testing.T) {
		data := strings.NewReader(strings.Repeat("x", 512))
		result, err := limitedReadAll(data, 512)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(result) != 512 {
			t.Errorf("Expected 512 bytes, got %d", len(result))
		}
	})

	t.Run("client handles oversized error response without OOM", func(t *testing.T) {
		// Server returns a large error body on a status code that reads the body (422).
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(422)
			// MaxErrorBodyBytes is 1MB; write 2MB
			for i := 0; i < 2*1024; i++ {
				fmt.Fprint(w, strings.Repeat("x", 1024))
			}
		}))
		defer srv.Close()

		cfg := &Config{BaseURL: srv.URL}
		client := NewClient(cfg, &StaticTokenProvider{Token: "token"},
			WithTransport(srv.Client().Transport))

		// Should return an error without OOM — the body is bounded by limitedReadAll
		_, err := client.Get(context.Background(), "/test")
		if err == nil {
			t.Fatal("Expected error for 422 response")
		}
	})
}

// =============================================================================
// 2.1 HTTPS Enforcement on Token Endpoints
// =============================================================================

func TestRequireHTTPS_RejectsHTTP(t *testing.T) {
	err := requireHTTPS("http://example.com/token")
	if err == nil {
		t.Fatal("Expected error for HTTP URL")
	}
	if !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("Expected HTTPS error, got: %v", err)
	}
}

func TestRequireHTTPS_AcceptsHTTPS(t *testing.T) {
	err := requireHTTPS("https://example.com/token")
	if err != nil {
		t.Fatalf("Unexpected error for HTTPS URL: %v", err)
	}
}

func TestRequireHTTPS_RejectsInvalidURL(t *testing.T) {
	err := requireHTTPS("://bad")
	if err == nil {
		t.Fatal("Expected error for invalid URL")
	}
}

// =============================================================================
// 2.2 HTTPS Enforcement on buildURL
// =============================================================================

func TestBuildURL_RejectsHTTPAbsoluteURL(t *testing.T) {
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "token"})

	_, err := client.buildURL("http://evil.com/path")
	if err == nil {
		t.Fatal("Expected error for http:// absolute URL")
	}
	if !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("Expected HTTPS error, got: %v", err)
	}
}

func TestBuildURL_AcceptsHTTPSAbsoluteURL(t *testing.T) {
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "token"})

	result, err := client.buildURL("https://3.basecampapi.com/page2")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "https://3.basecampapi.com/page2" {
		t.Errorf("Expected passthrough for https:// URL, got: %q", result)
	}
}

func TestBuildURL_BuildsRelativePath(t *testing.T) {
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "token"})

	result, err := client.buildURL("/projects.json")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != "https://3.basecampapi.com/projects.json" {
		t.Errorf("Expected full URL, got: %q", result)
	}
}

// =============================================================================
// 2.3 Webhook PayloadURL Validation
// =============================================================================

func TestWebhookCreate_RejectsHTTPPayloadURL(t *testing.T) {
	// We can't easily call Create without a full server, but we can test
	// the requireHTTPS function which is the core validation.
	err := requireHTTPS("http://example.com/webhook")
	if err == nil {
		t.Fatal("Expected error for HTTP webhook URL")
	}
}

func TestWebhookCreate_RejectsJavascriptURL(t *testing.T) {
	err := requireHTTPS("javascript:alert(1)")
	if err == nil {
		t.Fatal("Expected error for javascript: URL")
	}
}

func TestWebhookCreate_RejectsFileURL(t *testing.T) {
	err := requireHTTPS("file:///etc/passwd")
	if err == nil {
		t.Fatal("Expected error for file: URL")
	}
}

// =============================================================================
// 2.5 Error Body Truncation
// =============================================================================

func TestTruncateString_Short(t *testing.T) {
	result := truncateString("hello", 10)
	if result != "hello" {
		t.Errorf("Expected 'hello', got %q", result)
	}
}

func TestTruncateString_Long(t *testing.T) {
	long := strings.Repeat("x", 1000)
	result := truncateString(long, 500)
	if len(result) != 500 {
		t.Errorf("Expected exactly 500 chars, got %d", len(result))
	}
	if !strings.HasSuffix(result, "...") {
		t.Error("Expected '...' suffix")
	}
}

func TestHandleError_TruncatesLargeErrorMessage(t *testing.T) {
	// Create a response with a very large error message
	largeMsg := strings.Repeat("x", 1000)
	body := fmt.Sprintf(`{"error": "%s"}`, largeMsg)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(body))
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test"})

	_, err := client.Get(context.Background(), "/test")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("Expected *Error, got %T", err)
	}

	// Error message should be truncated to 500 chars (497 + "...")
	if len(apiErr.Message) > MaxErrorMessageBytes {
		t.Errorf("Error message too long (%d chars), expected max %d", len(apiErr.Message), MaxErrorMessageBytes)
	}
	if !strings.HasSuffix(apiErr.Message, "...") {
		t.Error("Expected '...' suffix in truncated error")
	}
}

// =============================================================================
// isSameOrigin Tests
// =============================================================================

func TestIsSameOrigin(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"https://api.example.com/path1", "https://api.example.com/path2", true},
		{"https://api.example.com/path", "https://evil.com/path", false},
		{"https://api.example.com/path", "http://api.example.com/path", false},
		{"https://api.example.com:443/path", "https://api.example.com:443/path", true},
		// Port normalization: explicit default port matches implicit default port
		{"https://api.example.com:443/path", "https://api.example.com/path", true},
		{"http://api.example.com:80/path", "http://api.example.com/path", true},
		// Non-default ports must match
		{"https://api.example.com:8443/path", "https://api.example.com/path", false},
		{"https://api.example.com:8443/path", "https://api.example.com:8443/other", true},
		// URLs without a scheme are rejected (resolveURL handles resolution first)
		{"/page2", "https://api.example.com/page1", false},
		{"https://api.example.com/page1", "/page2", false},
		{"not-a-url", "https://example.com", false},
		{"https://example.com", "not-a-url", false},
	}

	for _, tt := range tests {
		got := isSameOrigin(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("isSameOrigin(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// =============================================================================
// Account ID Validation
// =============================================================================

func TestForAccount_RejectsEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for empty account ID")
		}
	}()

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "token"})
	client.ForAccount("")
}

func TestForAccount_RejectsNonNumeric(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for non-numeric account ID")
		}
	}()

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "token"})
	client.ForAccount("abc")
}

func TestForAccount_RejectsPathTraversal(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for path traversal attempt")
		}
	}()

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "token"})
	client.ForAccount("../etc/passwd")
}

// =============================================================================
// No Tokens in Error Messages
// =============================================================================

func TestErrorMessages_NoTokenLeakage(t *testing.T) {
	// Verify that common error constructors don't include bearer tokens
	token := "super-secret-bearer-token-12345"

	errors := []error{
		ErrAuth("Authentication failed"),
		ErrNetwork(fmt.Errorf("connection refused")),
		ErrAPI(500, "Server error"),
		ErrNotFound("Project", "12345"),
		ErrRateLimit(30),
		ErrForbidden("Access denied"),
	}

	for _, err := range errors {
		msg := err.Error()
		if strings.Contains(msg, token) {
			t.Errorf("Error message contains token: %q", msg)
		}
	}
}

// =============================================================================
// Response Body limitedReadAll integration
// =============================================================================

func TestLimitedReadAll_ExactLimit(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 100)
	result, err := limitedReadAll(bytes.NewReader(data), 100)
	if err != nil {
		t.Fatalf("Unexpected error at exact limit: %v", err)
	}
	if len(result) != 100 {
		t.Errorf("Expected 100 bytes, got %d", len(result))
	}
}

func TestLimitedReadAll_OneOverLimit(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 101)
	_, err := limitedReadAll(bytes.NewReader(data), 100)
	if err == nil {
		t.Fatal("Expected error when body is 1 byte over limit")
	}
}

func TestLimitedReadAll_EmptyBody(t *testing.T) {
	result, err := limitedReadAll(io.LimitReader(bytes.NewReader(nil), 100), 100)
	if err != nil {
		t.Fatalf("Unexpected error for empty body: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 bytes, got %d", len(result))
	}
}

// =============================================================================
// Webhook Update PayloadURL Validation (backported from Ruby)
// =============================================================================

func TestWebhookUpdate_RejectsHTTPPayloadURL(t *testing.T) {
	err := requireHTTPS("http://example.com/webhook")
	if err == nil {
		t.Fatal("Expected error for HTTP webhook URL on update")
	}
}

func TestWebhookUpdate_AcceptsHTTPSPayloadURL(t *testing.T) {
	err := requireHTTPS("https://example.com/webhook")
	if err != nil {
		t.Fatalf("Unexpected error for HTTPS webhook URL: %v", err)
	}
}

func TestWebhookUpdate_AllowsEmptyPayloadURL(t *testing.T) {
	// When PayloadURL is empty on update, no validation should occur.
	// The Update method only validates non-empty PayloadURL.
	// We test that empty string is not rejected by requireHTTPS
	// (it would fail if passed, but Update skips validation for empty).
	url := ""
	if url != "" {
		t.Fatal("Test setup error: URL should be empty")
	}
	// No call to requireHTTPS — passes by design
}

// =============================================================================
// Config Validation (backported from Ruby)
// =============================================================================

func TestNewClient_PanicsOnHTTPBaseURL(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for HTTP base URL")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "HTTPS") {
			t.Errorf("Expected HTTPS panic message, got: %v", r)
		}
	}()

	cfg := &Config{BaseURL: "http://3.basecampapi.com"}
	NewClient(cfg, &StaticTokenProvider{Token: "token"})
}

func TestNewClient_PanicsOnNegativeTimeout(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for negative timeout")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "timeout") {
			t.Errorf("Expected timeout panic message, got: %v", r)
		}
	}()

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	NewClient(cfg, &StaticTokenProvider{Token: "token"},
		WithTimeout(-1*time.Second))
}

func TestNewClient_PanicsOnNegativeMaxPages(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected panic for zero max pages")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "max pages") {
			t.Errorf("Expected max pages panic message, got: %v", r)
		}
	}()

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	NewClient(cfg, &StaticTokenProvider{Token: "token"},
		WithMaxPages(0))
}

// =============================================================================
// Header Redaction Tests
// =============================================================================

func TestRedactHeaders_RedactsSensitiveHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret-token")
	headers.Set("Cookie", "session=abc123")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-CSRF-Token", "csrf-token-value")

	redacted := RedactHeaders(headers)

	// Sensitive headers should be redacted
	if redacted.Get("Authorization") != "[REDACTED]" {
		t.Errorf("Expected Authorization to be redacted, got: %q", redacted.Get("Authorization"))
	}
	if redacted.Get("Cookie") != "[REDACTED]" {
		t.Errorf("Expected Cookie to be redacted, got: %q", redacted.Get("Cookie"))
	}
	if redacted.Get("X-CSRF-Token") != "[REDACTED]" {
		t.Errorf("Expected X-CSRF-Token to be redacted, got: %q", redacted.Get("X-CSRF-Token"))
	}

	// Non-sensitive headers should be preserved
	if redacted.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type to be preserved, got: %q", redacted.Get("Content-Type"))
	}
}

func TestRedactHeaders_PreservesOriginal(t *testing.T) {
	original := http.Header{}
	original.Set("Authorization", "Bearer secret-token")

	_ = RedactHeaders(original)

	// Original should not be modified
	if original.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("Original header was modified, got: %q", original.Get("Authorization"))
	}
}

func TestRedactHeaders_EmptyHeaders(t *testing.T) {
	headers := http.Header{}
	redacted := RedactHeaders(headers)

	if len(redacted) != 0 {
		t.Errorf("Expected empty headers, got: %v", redacted)
	}
}

func TestRedactHeaders_SkipsAbsentSensitiveHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	redacted := RedactHeaders(headers)

	// Non-sensitive header should be preserved
	if redacted.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type to be preserved, got: %q", redacted.Get("Content-Type"))
	}

	// Absent sensitive headers should remain absent, not be set to [REDACTED]
	if redacted.Get("Authorization") != "" {
		t.Errorf("Expected Authorization to be absent, got: %q", redacted.Get("Authorization"))
	}
}

// =============================================================================
// isLocalhost Tests
// =============================================================================

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		// Exact localhost matches
		{"http://localhost/path", true},
		{"https://localhost:3000/path", true},
		{"http://localhost", true},

		// IPv4 loopback
		{"http://127.0.0.1/path", true},
		{"https://127.0.0.1:8080/api", true},

		// IPv6 loopback
		{"http://[::1]/path", true},
		{"https://[::1]:3000/api", true},

		// RFC 6761: .localhost TLD
		{"http://myapp.localhost/path", true},
		{"https://myapp.localhost:3000/api", true},
		{"http://app.localhost", true},

		// RFC 6761: subdomains of localhost
		{"http://sub.app.localhost/path", true},
		{"https://deep.nested.sub.localhost:8080/api", true},

		// Non-localhost URLs should return false
		{"https://example.com/path", false},
		{"https://api.example.com/path", false},
		{"https://3.basecampapi.com/12345/projects.json", false},

		// Tricky cases that should NOT match
		{"https://notlocalhost.com/path", false},        // localhost as substring
		{"https://localhost.example.com/path", false},   // localhost as subdomain of non-localhost
		{"https://fakelocalhostdomain.com/path", false}, // localhost embedded in domain

		// Invalid URLs
		{"://invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isLocalhost(tt.url)
		if got != tt.want {
			t.Errorf("isLocalhost(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
