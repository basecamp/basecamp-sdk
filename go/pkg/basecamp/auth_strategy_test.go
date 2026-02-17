package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerAuth_SetsAuthorizationHeader(t *testing.T) {
	strategy := &BearerAuth{TokenProvider: &StaticTokenProvider{Token: "test-token"}}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://example.com", nil)
	if err := strategy.Authenticate(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("expected Authorization %q, got %q", "Bearer test-token", got)
	}
}

func TestBearerAuth_PropagatesTokenProviderError(t *testing.T) {
	strategy := &BearerAuth{TokenProvider: &failingTokenProvider{}}

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://example.com", nil)
	err := strategy.Authenticate(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from failing token provider")
	}
}

// cookieAuth is a custom AuthStrategy for testing.
type cookieAuth struct {
	cookie string
}

func (c *cookieAuth) Authenticate(_ context.Context, req *http.Request) error {
	req.Header.Set("Cookie", c.cookie)
	return nil
}

func TestCustomAuthStrategy_CookieAuth(t *testing.T) {
	var receivedCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookie = r.Header.Get("Cookie")
		// Verify no Bearer token is set
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("expected no Authorization header with cookie auth, got %q", auth)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "unused"},
		WithAuthStrategy(&cookieAuth{cookie: "session=abc123"}))

	_, err := client.Get(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedCookie != "session=abc123" {
		t.Errorf("expected cookie %q, got %q", "session=abc123", receivedCookie)
	}
}

func TestWithAuthStrategy_OverridesDefault(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	// Use a custom strategy that sets a different auth header
	strategy := &BearerAuth{TokenProvider: &StaticTokenProvider{Token: "custom-token"}}
	client := NewClient(cfg, &StaticTokenProvider{Token: "default-token"},
		WithAuthStrategy(strategy))

	_, err := client.Get(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAuth != "Bearer custom-token" {
		t.Errorf("expected Authorization %q, got %q", "Bearer custom-token", receivedAuth)
	}
}

func TestBackwardCompat_WithAccessToken(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	client := NewClient(cfg, &StaticTokenProvider{Token: "my-token"})

	_, err := client.Get(context.Background(), "/test.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAuth != "Bearer my-token" {
		t.Errorf("expected Authorization %q, got %q", "Bearer my-token", receivedAuth)
	}
}

// failingTokenProvider always returns an error.
type failingTokenProvider struct{}

func (f *failingTokenProvider) AccessToken(_ context.Context) (string, error) {
	return "", ErrAuth("token provider failed")
}
