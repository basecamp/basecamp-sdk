package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDiscoverer_Discover(t *testing.T) {
	tests := []struct {
		name       string
		response   any
		statusCode int
		wantErr    bool
	}{
		{
			name: "successful discovery",
			response: Config{
				Issuer:                "https://example.com",
				AuthorizationEndpoint: "https://example.com/authorize",
				TokenEndpoint:         "https://example.com/token",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "server error",
			response:   "Internal Server Error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "not found",
			response:   "Not Found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/.well-known/oauth-authorization-server" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.response)
				} else {
					_, _ = w.Write([]byte(tt.response.(string)))
				}
			}))
			defer server.Close()

			d := NewDiscoverer(server.Client())
			cfg, err := d.Discover(context.Background(), server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("Discover() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && cfg == nil {
				t.Error("Discover() returned nil config")
			}
		})
	}
}

func TestDiscoverer_Discover_URLNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/oauth-authorization-server" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(Config{
			TokenEndpoint: "https://example.com/token",
		})
	}))
	defer server.Close()

	d := NewDiscoverer(server.Client())

	// Test with trailing slash
	_, err := d.Discover(context.Background(), server.URL+"/")
	if err != nil {
		t.Errorf("Discover() with trailing slash failed: %v", err)
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
