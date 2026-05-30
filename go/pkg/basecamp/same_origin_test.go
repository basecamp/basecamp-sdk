package basecamp

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// recordingTransport records whether it was ever invoked and what Authorization
// header it observed. It never performs real network I/O, so a request that
// reaches it would prove the same-origin guard failed to fire.
type recordingTransport struct {
	mu    sync.Mutex
	calls int
	auth  string
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	rt.calls++
	rt.auth = req.Header.Get("Authorization")
	rt.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`[]`)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func (rt *recordingTransport) snapshot() (int, string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.calls, rt.auth
}

// TestBuildURL_RejectsForeignOriginAbsoluteURL verifies that an absolute URL on
// a different origin than the configured base URL is rejected by the chokepoint.
func TestBuildURL_RejectsForeignOriginAbsoluteURL(t *testing.T) {
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"})

	_, err := client.buildURL("https://evil.example/x.json")
	if err == nil {
		t.Fatal("expected error for foreign-origin absolute URL, got nil")
	}
	if !strings.Contains(err.Error(), "different origin") {
		t.Errorf("expected error mentioning 'different origin', got: %v", err)
	}
}

// TestBuildURL_AcceptsLocalhostAbsoluteURL verifies the localhost dev/test
// carve-out: an absolute localhost URL passes even when the base URL is a
// different (non-localhost) origin.
func TestBuildURL_AcceptsLocalhostAbsoluteURL(t *testing.T) {
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "token"})

	got, err := client.buildURL("https://localhost:8080/page2")
	if err != nil {
		t.Fatalf("unexpected error for localhost absolute URL: %v", err)
	}
	if got != "https://localhost:8080/page2" {
		t.Errorf("expected localhost URL passthrough, got: %q", got)
	}

	// Localhost base URL with a localhost absolute path (httptest-style).
	cfg2 := &Config{BaseURL: "https://127.0.0.1:9999"}
	client2 := NewClient(cfg2, &StaticTokenProvider{Token: "token"})
	got2, err := client2.buildURL("https://127.0.0.1:9999/items.json")
	if err != nil {
		t.Fatalf("unexpected error for localhost base + path: %v", err)
	}
	if got2 != "https://127.0.0.1:9999/items.json" {
		t.Errorf("expected localhost passthrough, got: %q", got2)
	}
}

// TestForeignAbsoluteURL_NoTokenEgress is the end-to-end regression test for the
// credential-exfiltration primitive: a request to a foreign-origin absolute URL
// must error before any network send, and the transport must never see the
// Authorization header.
func TestForeignAbsoluteURL_NoTokenEgress(t *testing.T) {
	rt := &recordingTransport{}
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"},
		WithTransport(rt))

	_, err := client.Get(context.Background(), "https://evil.example/steal")
	if err == nil {
		t.Fatal("expected error for foreign-origin absolute URL, got nil")
	}
	if !strings.Contains(err.Error(), "different origin") {
		t.Errorf("expected error mentioning 'different origin', got: %v", err)
	}

	calls, auth := rt.snapshot()
	if calls != 0 {
		t.Errorf("expected zero requests to foreign origin, got %d", calls)
	}
	if auth != "" {
		t.Errorf("Authorization header leaked to foreign origin: %q", auth)
	}
}

// TestGetAll_RejectsForeignFirstPageURL verifies pagination's first page is also
// guarded by the chokepoint.
func TestGetAll_RejectsForeignFirstPageURL(t *testing.T) {
	rt := &recordingTransport{}
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"},
		WithTransport(rt))

	_, err := client.GetAll(context.Background(), "https://evil.example/items.json")
	if err == nil {
		t.Fatal("expected error for foreign-origin first page URL, got nil")
	}
	if !strings.Contains(err.Error(), "different origin") {
		t.Errorf("expected error mentioning 'different origin', got: %v", err)
	}
	if calls, _ := rt.snapshot(); calls != 0 {
		t.Errorf("expected zero requests to foreign origin, got %d", calls)
	}
}

// TestAccountClient_RejectsForeignAbsoluteURL verifies account-scoped requests
// reject foreign absolute URLs too (accountPath passes them through; buildURL
// rejects them).
func TestAccountClient_RejectsForeignAbsoluteURL(t *testing.T) {
	rt := &recordingTransport{}
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"},
		WithTransport(rt))
	ac := client.ForAccount("12345")

	_, err := ac.Get(context.Background(), "https://evil.example/projects.json")
	if err == nil {
		t.Fatal("expected error for foreign-origin absolute URL, got nil")
	}
	if !strings.Contains(err.Error(), "different origin") {
		t.Errorf("expected error mentioning 'different origin', got: %v", err)
	}
	if calls, auth := rt.snapshot(); calls != 0 || auth != "" {
		t.Errorf("token egress to foreign origin: calls=%d auth=%q", calls, auth)
	}
}

// TestSameOriginAbsoluteURL_CarriesToken confirms a same-origin absolute URL
// still works and still carries the bearer token.
func TestSameOriginAbsoluteURL_CarriesToken(t *testing.T) {
	rt := &recordingTransport{}
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "secret-token"},
		WithTransport(rt))

	_, err := client.Get(context.Background(), "https://3.basecampapi.com/page2")
	if err != nil {
		t.Fatalf("unexpected error for same-origin absolute URL: %v", err)
	}
	calls, auth := rt.snapshot()
	if calls != 1 {
		t.Fatalf("expected one request to same origin, got %d", calls)
	}
	if auth != "Bearer secret-token" {
		t.Errorf("expected Authorization %q, got %q", "Bearer secret-token", auth)
	}
}
