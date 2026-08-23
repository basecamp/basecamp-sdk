package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func TestDiscoverer_Discover(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		// bindIssuer serves metadata whose issuer equals the server origin so
		// the RFC 8414 issuer binding passes.
		bindIssuer bool
		wantErr    bool
	}{
		{
			name:       "successful discovery",
			statusCode: http.StatusOK,
			bindIssuer: true,
			wantErr:    false,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var origin string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/.well-known/oauth-authorization-server" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if tt.bindIssuer {
					_, _ = fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q}`,
						origin, origin+"/authorize", origin+"/token")
				} else {
					_, _ = w.Write([]byte("error body"))
				}
			}))
			defer server.Close()
			origin = server.URL

			d := NewDiscoverer(server.Client())
			cfg, err := d.Discover(context.Background(), server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("Discover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if cfg == nil {
					t.Fatal("Discover() returned nil config")
				}
				if cfg.Issuer != origin {
					t.Errorf("Discover() issuer = %q, want %q", cfg.Issuer, origin)
				}
			}
		})
	}
}

// TestDiscoverer_Discover_MidStreamReadFailureIsNetwork verifies that a 2xx
// whose body dies mid-stream (peer reset, truncation) is classified as network
// (retryable), not as the size-cap api_error.
func TestDiscoverer_Discover_MidStreamReadFailureIsNetwork(t *testing.T) {
	// A 2xx whose body dies mid-read (peer reset, truncation) is a transient
	// transport fault — network, retryable — never misclassified as the
	// size-cap api_error, which is reserved for errBodyTooLarge.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("server does not support hijacking")
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack: %v", err)
		}
		// Declare more bytes than are sent, then close the connection: the
		// client's body read fails mid-stream with an unexpected EOF.
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 4096\r\n\r\n{\"issuer\":")
		_ = buf.Flush()
		_ = conn.Close()
	}))
	defer srv.Close()

	d := NewDiscoverer(srv.Client())
	_, err := d.Discover(context.Background(), srv.URL)

	var bcErr *basecamp.Error
	if !errors.As(err, &bcErr) {
		t.Fatalf("want *basecamp.Error, got %v", err)
	}
	if bcErr.Code != basecamp.CodeNetwork {
		t.Errorf("Code = %q, want %q (mid-stream read failure is transport, not malformed metadata)",
			bcErr.Code, basecamp.CodeNetwork)
	}
	if !bcErr.Retryable {
		t.Error("mid-stream read failure must be retryable")
	}
}

func TestDiscoverer_Discover_TrailingSlash(t *testing.T) {
	// A trailing slash is normalized away for the fetch URL (routing), but issuer
	// binding is code-point-exact against the caller's RAW baseURL (RFC 8414 §3.3,
	// SPEC.md §16), so the AS must echo the trailing-slash issuer to bind.
	var caller string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/oauth-authorization-server" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprintf(w, `{"issuer":%q,"token_endpoint":%q}`, caller, caller+"token")
	}))
	defer server.Close()
	caller = server.URL + "/"

	d := NewDiscoverer(server.Client())

	cfg, err := d.Discover(context.Background(), caller)
	if err != nil {
		t.Fatalf("Discover() with trailing slash failed: %v", err)
	}
	if cfg.Issuer != caller {
		t.Errorf("Discover() issuer = %q, want %q", cfg.Issuer, caller)
	}
}

func TestExchanger_Exchange(t *testing.T) {
	tests := []struct {
		name            string
		req             ExchangeRequest
		response        any
		statusCode      int
		wantErr         bool
		wantLegacyParam bool
	}{
		{
			name: "successful exchange",
			req: ExchangeRequest{
				TokenEndpoint: "will be replaced",
				Code:          "auth_code",
				RedirectURI:   "http://localhost/callback",
				ClientID:      "client123",
			},
			response: map[string]any{
				"access_token":  "access123",
				"refresh_token": "refresh123",
				"token_type":    "Bearer",
				"expires_in":    3600,
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "legacy format exchange",
			req: ExchangeRequest{
				TokenEndpoint:   "will be replaced",
				Code:            "auth_code",
				RedirectURI:     "http://localhost/callback",
				ClientID:        "client123",
				UseLegacyFormat: true,
			},
			response: map[string]any{
				"access_token":  "access123",
				"refresh_token": "refresh123",
			},
			statusCode:      http.StatusOK,
			wantErr:         false,
			wantLegacyParam: true,
		},
		{
			name: "error response",
			req: ExchangeRequest{
				TokenEndpoint: "will be replaced",
				Code:          "bad_code",
				RedirectURI:   "http://localhost/callback",
				ClientID:      "client123",
			},
			response: map[string]any{
				"error":             "invalid_grant",
				"error_description": "The authorization code has expired",
			},
			statusCode: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "missing token endpoint",
			req: ExchangeRequest{
				Code:        "auth_code",
				RedirectURI: "http://localhost/callback",
				ClientID:    "client123",
			},
			wantErr: true,
		},
		{
			name: "missing code",
			req: ExchangeRequest{
				TokenEndpoint: "https://example.com/token",
				RedirectURI:   "http://localhost/callback",
				ClientID:      "client123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedType string
			var receivedGrantType string

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				receivedType = r.FormValue("type")
				receivedGrantType = r.FormValue("grant_type")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			req := tt.req
			if req.TokenEndpoint == "will be replaced" {
				req.TokenEndpoint = server.URL
			}

			e := NewExchanger(server.Client())
			token, err := e.Exchange(context.Background(), req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Exchange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if token == nil {
					t.Error("Exchange() returned nil token")
					return
				}
				if token.AccessToken == "" {
					t.Error("Exchange() returned empty access token")
				}
			}

			if tt.wantLegacyParam {
				if receivedType != "web_server" {
					t.Errorf("Expected type=web_server, got type=%s", receivedType)
				}
				if receivedGrantType != "" {
					t.Errorf("Expected no grant_type for legacy, got grant_type=%s", receivedGrantType)
				}
			} else if tt.statusCode == http.StatusOK {
				if receivedGrantType != "authorization_code" {
					t.Errorf("Expected grant_type=authorization_code, got grant_type=%s", receivedGrantType)
				}
			}
		})
	}
}

func TestExchanger_Refresh(t *testing.T) {
	tests := []struct {
		name            string
		req             RefreshRequest
		response        any
		statusCode      int
		wantErr         bool
		wantLegacyParam bool
	}{
		{
			name: "successful refresh",
			req: RefreshRequest{
				TokenEndpoint: "will be replaced",
				RefreshToken:  "refresh123",
			},
			response: map[string]any{
				"access_token":  "new_access123",
				"refresh_token": "new_refresh123",
				"expires_in":    3600,
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "legacy format refresh",
			req: RefreshRequest{
				TokenEndpoint:   "will be replaced",
				RefreshToken:    "refresh123",
				UseLegacyFormat: true,
			},
			response: map[string]any{
				"access_token": "new_access123",
			},
			statusCode:      http.StatusOK,
			wantErr:         false,
			wantLegacyParam: true,
		},
		{
			name: "missing refresh token",
			req: RefreshRequest{
				TokenEndpoint: "https://example.com/token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedType string
			var receivedGrantType string

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				receivedType = r.FormValue("type")
				receivedGrantType = r.FormValue("grant_type")

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			req := tt.req
			if req.TokenEndpoint == "will be replaced" {
				req.TokenEndpoint = server.URL
			}

			e := NewExchanger(server.Client())
			token, err := e.Refresh(context.Background(), req)

			if (err != nil) != tt.wantErr {
				t.Errorf("Refresh() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && token == nil {
				t.Error("Refresh() returned nil token")
			}

			if tt.wantLegacyParam {
				if receivedType != "refresh" {
					t.Errorf("Expected type=refresh, got type=%s", receivedType)
				}
			} else if tt.statusCode == http.StatusOK {
				if receivedGrantType != "refresh_token" {
					t.Errorf("Expected grant_type=refresh_token, got grant_type=%s", receivedGrantType)
				}
			}
		})
	}
}

func TestToken_ExpiresAt(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access123",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	e := NewExchanger(server.Client())
	before := time.Now()

	token, err := e.Exchange(context.Background(), ExchangeRequest{
		TokenEndpoint: server.URL,
		Code:          "code",
		RedirectURI:   "http://localhost/callback",
		ClientID:      "client",
	})
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}

	after := time.Now()

	// ExpiresAt should be approximately 1 hour from now
	expectedMin := before.Add(3600 * time.Second)
	expectedMax := after.Add(3600 * time.Second)

	if token.ExpiresAt.Before(expectedMin) || token.ExpiresAt.After(expectedMax) {
		t.Errorf("ExpiresAt = %v, expected between %v and %v", token.ExpiresAt, expectedMin, expectedMax)
	}
}

// =============================================================================
// Security Tests
// =============================================================================

func TestExchanger_Exchange_RejectsHTTPEndpoint(t *testing.T) {
	e := NewExchanger(http.DefaultClient)
	_, err := e.Exchange(context.Background(), ExchangeRequest{
		TokenEndpoint: "http://example.com/token",
		Code:          "code",
		RedirectURI:   "http://localhost/callback",
		ClientID:      "client",
	})
	if err == nil {
		t.Fatal("Expected error for HTTP token endpoint")
	}
	if !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("Expected HTTPS error, got: %v", err)
	}
}

func TestExchanger_Refresh_RejectsHTTPEndpoint(t *testing.T) {
	e := NewExchanger(http.DefaultClient)
	_, err := e.Refresh(context.Background(), RefreshRequest{
		TokenEndpoint: "http://example.com/token",
		RefreshToken:  "refresh123",
	})
	if err == nil {
		t.Fatal("Expected error for HTTP token endpoint")
	}
	if !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("Expected HTTPS error, got: %v", err)
	}
}

func TestExchanger_Exchange_TruncatesLargeErrorBody(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		// Write a large error body (not valid JSON, falls through to raw body path)
		largeBody := strings.Repeat("x", 10000)
		fmt.Fprint(w, largeBody)
	}))
	defer server.Close()

	e := NewExchanger(server.Client())
	_, err := e.Exchange(context.Background(), ExchangeRequest{
		TokenEndpoint: server.URL,
		Code:          "bad_code",
		RedirectURI:   "http://localhost/callback",
		ClientID:      "client123",
	})
	if err == nil {
		t.Fatal("Expected error")
	}
	errMsg := err.Error()
	// The truncated body portion must be at most maxErrorMessageLen (500).
	// Full message includes prefix "token request failed with status 400: " (38 chars) + body (<=500).
	if len(errMsg) > 600 {
		t.Errorf("Error message too long (%d chars), truncated body should be at most %d", len(errMsg), maxErrorMessageLen)
	}
	if !strings.Contains(errMsg, "...") {
		t.Error("Expected '...' suffix in truncated error")
	}
}

func TestExchanger_Exchange_TruncatesLargeErrorDescription(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		largeDesc := strings.Repeat("y", 10000)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": largeDesc,
		})
	}))
	defer server.Close()

	e := NewExchanger(server.Client())
	_, err := e.Exchange(context.Background(), ExchangeRequest{
		TokenEndpoint: server.URL,
		Code:          "bad_code",
		RedirectURI:   "http://localhost/callback",
		ClientID:      "client123",
	})
	if err == nil {
		t.Fatal("Expected error")
	}
	errMsg := err.Error()
	// The truncated description portion must be at most maxErrorMessageLen (500).
	// Full message: "token error: invalid_grant - " (29 chars) + desc (<=500).
	if len(errMsg) > 600 {
		t.Errorf("Error message too long (%d chars), truncated description should be at most %d", len(errMsg), maxErrorMessageLen)
	}
	if !strings.Contains(errMsg, "...") {
		t.Error("Expected '...' suffix in truncated error description")
	}
}

// TestNewDiscoverConfig_TimeoutClamp guards finding A: WithTimeout(0) (and any
// non-positive duration) must NOT drop the fetch timeout — it clamps to the
// default rather than leaving an unbounded fetch.
func TestNewDiscoverConfig_TimeoutClamp(t *testing.T) {
	cases := []struct {
		name string
		opts []DiscoverOption
		want time.Duration
	}{
		{"default when unset", nil, defaultDiscoveryTimeout},
		{"zero clamps to default", []DiscoverOption{WithTimeout(0)}, defaultDiscoveryTimeout},
		{"negative clamps to default", []DiscoverOption{WithTimeout(-5 * time.Second)}, defaultDiscoveryTimeout},
		{"positive preserved", []DiscoverOption{WithTimeout(3 * time.Second)}, 3 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newDiscoverConfig(tc.opts).timeout; got != tc.want {
				t.Errorf("timeout = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestNewDeviceConfig_TimeoutClamp is the device-flow analogue of finding A: the
// per-round-trip timeout clamps to the default on a non-positive value.
func TestNewDeviceConfig_TimeoutClamp(t *testing.T) {
	cases := []struct {
		name string
		opts []DeviceOption
		want time.Duration
	}{
		{"default when unset", nil, defaultDeviceRequestTimeout},
		{"zero clamps to default", []DeviceOption{WithDeviceTimeout(0)}, defaultDeviceRequestTimeout},
		{"negative clamps to default", []DeviceOption{WithDeviceTimeout(-time.Second)}, defaultDeviceRequestTimeout},
		{"positive preserved", []DeviceOption{WithDeviceTimeout(7 * time.Second)}, 7 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := newDeviceConfig(tc.opts).timeout; got != tc.want {
				t.Errorf("timeout = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExchanger_Refresh_ResourceEcho(t *testing.T) {
	tests := []struct {
		name         string
		resource     string
		wantSent     bool
		wantResource string
	}{
		{name: "resource sent when set", resource: "urn:bc:account:123", wantSent: true, wantResource: "urn:bc:account:123"},
		{name: "resource omitted when unset", resource: "", wantSent: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sawResourceKey bool
			var receivedResource string

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				_, sawResourceKey = r.PostForm["resource"]
				receivedResource = r.PostFormValue("resource")
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new_access"})
			}))
			defer server.Close()

			e := NewExchanger(server.Client())
			_, err := e.Refresh(context.Background(), RefreshRequest{
				TokenEndpoint: server.URL,
				RefreshToken:  "refresh123",
				ClientID:      "basecamp-cli",
				Resource:      tt.resource,
			})
			if err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}
			if sawResourceKey != tt.wantSent {
				t.Errorf("resource form key present = %v, want %v", sawResourceKey, tt.wantSent)
			}
			if receivedResource != tt.wantResource {
				t.Errorf("resource form value = %q, want %q", receivedResource, tt.wantResource)
			}
		})
	}
}

func TestExchanger_TokenTypeContract(t *testing.T) {
	// SPEC §16: token_type defaults to Bearer only when absent/JSON-null; a
	// present-but-empty value is a malformed response — matching the
	// device-flow parser.
	tests := []struct {
		name     string
		response string
		wantErr  bool
		wantType string
	}{
		{name: "absent token_type defaults to Bearer", response: `{"access_token":"a"}`, wantType: "Bearer"},
		{name: "null token_type defaults to Bearer", response: `{"access_token":"a","token_type":null}`, wantType: "Bearer"},
		{name: "present token_type round-trips", response: `{"access_token":"a","token_type":"Bearer"}`, wantType: "Bearer"},
		{name: "empty token_type rejected", response: `{"access_token":"a","token_type":""}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			e := NewExchanger(server.Client())
			token, err := e.Refresh(context.Background(), RefreshRequest{
				TokenEndpoint: server.URL,
				RefreshToken:  "refresh123",
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("Refresh() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var apiErr *basecamp.Error
				if !errors.As(err, &apiErr) || apiErr.Code != basecamp.CodeAPI {
					t.Fatalf("error = %T %v, want *basecamp.Error with CodeAPI", err, err)
				}
				return
			}
			if token.TokenType != tt.wantType {
				t.Errorf("TokenType = %q, want %q", token.TokenType, tt.wantType)
			}
		})
	}
}

func TestExchanger_TokenResponseResource(t *testing.T) {
	tests := []struct {
		name         string
		response     string
		wantErr      bool
		wantResource string
	}{
		{name: "resource round-trips", response: `{"access_token":"a","resource":"urn:bc:account:42"}`, wantResource: "urn:bc:account:42"},
		{name: "absent resource is unset", response: `{"access_token":"a"}`, wantResource: ""},
		{name: "null resource is unset", response: `{"access_token":"a","resource":null}`, wantResource: ""},
		{name: "empty resource rejected", response: `{"access_token":"a","resource":""}`, wantErr: true},
		{name: "non-string resource rejected", response: `{"access_token":"a","resource":7}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			e := NewExchanger(server.Client())
			token, err := e.Refresh(context.Background(), RefreshRequest{
				TokenEndpoint: server.URL,
				RefreshToken:  "refresh123",
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("Refresh() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.name == "empty resource rejected" || tt.name == "non-string resource rejected" {
				// A typed api fault (SPEC §16), not a bare error: callers
				// classify malformed responses via errors.As and need the
				// HTTP status, matching the device-token/AuthManager paths.
				var apiErr *basecamp.Error
				if !errors.As(err, &apiErr) || apiErr.Code != basecamp.CodeAPI {
					t.Fatalf("%s error = %T %v, want *basecamp.Error with CodeAPI", tt.name, err, err)
				}
				if apiErr.HTTPStatus != http.StatusOK {
					t.Errorf("HTTPStatus = %d, want 200", apiErr.HTTPStatus)
				}
			}
			if !tt.wantErr && token.Resource != tt.wantResource {
				t.Errorf("token.Resource = %q, want %q", token.Resource, tt.wantResource)
			}
		})
	}
}

// tokenRedirectServers starts a token endpoint answering every POST with the
// given redirect status and a Location naming a second server, and returns the
// endpoint URL plus the Location target's hit counter. As in
// endpoint_policy_test.go, the counter is the load-bearing assertion — and the
// target serves a USABLE token, so suppression that silently broke would
// surface as a successful exchange against the wrong host, not a
// differently-worded failure.
func tokenRedirectServers(t *testing.T, status int) (string, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(target.Close)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(status)
	}))
	t.Cleanup(endpoint.Close)
	return endpoint.URL, &hits
}

// assertTokenRedirectRefused checks the refusal's shape: typed api_error
// carrying the real status, the "not followed" message contract (SPEC §16
// "Token-Endpoint Transport Policy"), and a Location host that was never
// dialed.
func assertTokenRedirectRefused(t *testing.T, err error, status int, hits *atomic.Int64) {
	t.Helper()
	var be *basecamp.Error
	if !errors.As(err, &be) {
		t.Fatalf("error = %v, want *basecamp.Error", err)
	}
	if be.Code != basecamp.CodeAPI {
		t.Errorf("Code = %q, want %q", be.Code, basecamp.CodeAPI)
	}
	if be.HTTPStatus != status {
		t.Errorf("HTTPStatus = %d, want %d", be.HTTPStatus, status)
	}
	if !strings.Contains(be.Message, "not followed") {
		t.Errorf("Message = %q, want it to contain %q", be.Message, "not followed")
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("Location target hits = %d, want 0", got)
	}
}

// TestExchanger_RefusesTokenEndpointRedirects pins the full refused set on
// both operations and both client lanes: the SDK-built policy client and an
// injected client (a plain *http.Client would otherwise follow — 301/302/303
// as a GET, 307/308 re-POSTing the credentials).
func TestExchanger_RefusesTokenEndpointRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprintf("policy client %d", status), func(t *testing.T) {
			endpoint, hits := tokenRedirectServers(t, status)
			e := NewExchanger(nil, WithExchangerPolicy(DefaultIssuerPolicy().AllowLoopback()))

			_, err := e.Exchange(context.Background(), ExchangeRequest{
				TokenEndpoint: endpoint, Code: "code", RedirectURI: "http://localhost/cb", ClientID: "id",
			})
			assertTokenRedirectRefused(t, err, status, hits)

			_, err = e.Refresh(context.Background(), RefreshRequest{TokenEndpoint: endpoint, RefreshToken: "refresh"})
			assertTokenRedirectRefused(t, err, status, hits)
		})
		t.Run(fmt.Sprintf("injected client %d", status), func(t *testing.T) {
			endpoint, hits := tokenRedirectServers(t, status)
			e := NewExchanger(&http.Client{})

			_, err := e.Exchange(context.Background(), ExchangeRequest{
				TokenEndpoint: endpoint, Code: "code", RedirectURI: "http://localhost/cb", ClientID: "id",
			})
			assertTokenRedirectRefused(t, err, status, hits)

			_, err = e.Refresh(context.Background(), RefreshRequest{TokenEndpoint: endpoint, RefreshToken: "refresh"})
			assertTokenRedirectRefused(t, err, status, hits)
		})
	}
}

// TestExchanger_304StaysGenericNon200 pins the boundary of the refused set: a
// 304 is a cache validator, not a followable redirect, and keeps the untyped
// non-200 wrap.
func TestExchanger_304StaysGenericNon200(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(endpoint.Close)

	_, err := NewExchanger(&http.Client{}).Refresh(context.Background(),
		RefreshRequest{TokenEndpoint: endpoint.URL, RefreshToken: "refresh"})
	if err == nil {
		t.Fatal("Refresh() error = nil, want the generic non-200 failure")
	}
	if strings.Contains(err.Error(), "not followed") {
		t.Errorf("error = %v; 304 must not classify as a refused redirect", err)
	}
	if !strings.Contains(err.Error(), "status 304") {
		t.Errorf("error = %v, want the generic status-304 wrap", err)
	}
}

// TestExchanger_StalledRedirectBodyClassifiedBeforeRead proves the refusal is
// status-first: a 302 whose body never completes classifies immediately
// instead of timing out mid-read.
func TestExchanger_StalledRedirectBodyClassifiedBeforeRead(t *testing.T) {
	var hits atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer target.Close()

	release := make(chan struct{})
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusFound)
		w.(http.Flusher).Flush()
		<-release // the body never completes
	}))
	defer endpoint.Close()
	defer close(release) // unblock the handler before Close waits on it

	start := time.Now()
	_, err := NewExchanger(&http.Client{}).Refresh(context.Background(),
		RefreshRequest{TokenEndpoint: endpoint.URL, RefreshToken: "refresh"})
	assertTokenRedirectRefused(t, err, http.StatusFound, &hits)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("classification took %v, want status-first (before any body read)", elapsed)
	}
}

// TestNewExchanger_TimeoutClamp pins the normalize-at-entry rule shared with
// the device flow: non-positive and beyond-ceiling values fall back to the
// 30 s default; the ceiling itself and ordinary values are honored.
func TestNewExchanger_TimeoutClamp(t *testing.T) {
	tests := []struct {
		name string
		opts []ExchangerOption
		want time.Duration
	}{
		{"default when unset", nil, defaultTokenRequestTimeout},
		{"zero clamps to default", []ExchangerOption{WithExchangerTimeout(0)}, defaultTokenRequestTimeout},
		{"negative clamps to default", []ExchangerOption{WithExchangerTimeout(-time.Second)}, defaultTokenRequestTimeout},
		{"beyond ceiling clamps to default", []ExchangerOption{WithExchangerTimeout(maxDeviceRequestTimeout + time.Second)}, defaultTokenRequestTimeout},
		{"MaxInt64 clamps to default", []ExchangerOption{WithExchangerTimeout(time.Duration(math.MaxInt64))}, defaultTokenRequestTimeout},
		{"ceiling accepted", []ExchangerOption{WithExchangerTimeout(maxDeviceRequestTimeout)}, maxDeviceRequestTimeout},
		{"ordinary value accepted", []ExchangerOption{WithExchangerTimeout(5 * time.Second)}, 5 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewExchanger(&http.Client{}, tt.opts...).timeout; got != tt.want {
				t.Errorf("timeout = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExchanger_RequestTimeout proves the bound is live on an injected client
// with a context carrying no deadline of its own — the lane that was
// previously unbounded.
func TestExchanger_RequestTimeout(t *testing.T) {
	release := make(chan struct{})
	endpoint := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release: // the response never arrives
		}
	}))
	defer endpoint.Close()
	defer close(release) // unblock the handler before Close waits on it

	e := NewExchanger(&http.Client{}, WithExchangerTimeout(100*time.Millisecond))
	start := time.Now()
	_, err := e.Refresh(context.Background(), RefreshRequest{TokenEndpoint: endpoint.URL, RefreshToken: "refresh"})
	if err == nil {
		t.Fatal("Refresh() error = nil, want the timeout failure")
	}
	if !strings.Contains(err.Error(), "token request failed") {
		t.Errorf("error = %v, want the untyped transport wrap", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("request ran %v, want it bounded near the 100ms deadline", elapsed)
	}
}
