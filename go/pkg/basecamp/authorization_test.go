package basecamp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthorizationService_GetInfo(t *testing.T) {
	tests := []struct {
		name       string
		response   any
		statusCode int
		opts       *GetInfoOptions
		wantErr    bool
		wantCount  int // expected number of accounts after filtering
	}{
		{
			name: "successful response",
			response: map[string]any{
				"identity": map[string]any{
					"id":            123,
					"first_name":    "Test",
					"last_name":     "User",
					"email_address": "test@example.com",
				},
				"accounts": []map[string]any{
					{"id": 1, "name": "Account 1", "product": "bc3"},
					{"id": 2, "name": "Account 2", "product": "hey"},
				},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
			wantCount:  2,
		},
		{
			name: "filter by product",
			response: map[string]any{
				"identity": map[string]any{
					"id":         123,
					"first_name": "Test",
				},
				"accounts": []map[string]any{
					{"id": 1, "name": "Basecamp Account", "product": "bc3"},
					{"id": 2, "name": "HEY Account", "product": "hey"},
					{"id": 3, "name": "Another BC", "product": "bc3"},
				},
			},
			statusCode: http.StatusOK,
			opts:       &GetInfoOptions{FilterProduct: "bc3"},
			wantErr:    false,
			wantCount:  2, // Only bc3 accounts
		},
		{
			name:       "unauthorized",
			response:   map[string]any{"error": "invalid token"},
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "server error",
			response:   "Internal Server Error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify authorization header
				if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
					t.Errorf("unexpected Authorization header: %s", auth)
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.response)
				} else if s, ok := tt.response.(string); ok {
					_, _ = w.Write([]byte(s))
				} else {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			cfg := DefaultConfig()
			cfg.BaseURL = server.URL
			token := &StaticTokenProvider{Token: "test-token"}
			client := NewClient(cfg, token, WithHTTPClient(server.Client()))

			opts := tt.opts
			if opts == nil {
				opts = &GetInfoOptions{}
			}
			opts.Endpoint = server.URL + "/authorization.json"

			info, err := client.Authorization().GetInfo(t.Context(), opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if info == nil {
					t.Error("GetInfo() returned nil info")
					return
				}
				if len(info.Accounts) != tt.wantCount {
					t.Errorf("GetInfo() returned %d accounts, want %d", len(info.Accounts), tt.wantCount)
				}
			}
		})
	}
}

// bc5AuthorizationDocument is what a BC5 (bc3) issuer serves from
// app/views/api/authorizations/show.json.jbuilder: identity id only, no product
// or app_href on accounts, an RFC 8707 resource indicator instead, a top-level
// scope, and expires_at as integer epoch seconds.
//
// Go reaches this document exactly the way TypeScript does — by passing an
// Endpoint override, which is the documented way to point at a BC5 issuer.
func bc5AuthorizationDocument() map[string]any {
	return map[string]any{
		"identity": map[string]any{"id": 123},
		"accounts": []map[string]any{
			{"id": 1, "name": "Acme Corp", "href": "https://bc5.example.com/1", "resource": "urn:bc:account:1"},
			{"id": 2, "name": "Second Account", "href": "https://bc5.example.com/2", "resource": "urn:bc:account:2"},
		},
		"scope": "read write",
		// 2036-01-29T09:55:56Z as epoch seconds.
		"expires_at": 2085213356,
	}
}

func newBC5AuthorizationServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(bc5AuthorizationDocument())
	}))
	t.Cleanup(server.Close)
	return server
}

func newBC5AuthorizationClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	return NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithHTTPClient(server.Client()))
}

// A BC5 document carries no product on any account. Filtering it by product
// matches nothing, so a literal filter empties a list the caller is about to pick
// an HREF out of. Report the filter inapplicable and return the accounts instead.
func TestAuthorizationService_GetInfo_ProductFilterInapplicableOnBC5Document(t *testing.T) {
	server := newBC5AuthorizationServer(t)
	client := newBC5AuthorizationClient(t, server)

	info, err := client.Authorization().GetInfo(t.Context(), &GetInfoOptions{
		Endpoint:      server.URL + "/authorization.json",
		FilterProduct: "bc3",
	})
	if err != nil {
		t.Fatalf("GetInfo() unexpected error: %v", err)
	}

	if len(info.Accounts) != 2 {
		t.Errorf("GetInfo() returned %d accounts, want 2 — the filter is inapplicable, not unmatched", len(info.Accounts))
	}
	if info.ProductFilterApplied {
		t.Error("GetInfo() reported ProductFilterApplied = true, want false on a document with no product")
	}
	if info.Accounts[0].HREF != "https://bc5.example.com/1" {
		t.Errorf("GetInfo() account HREF = %q, want https://bc5.example.com/1", info.Accounts[0].HREF)
	}
}

// The converse: a Launchpad document is filterable, so a filter that matches
// nothing genuinely means nothing matched, and must still return empty.
func TestAuthorizationService_GetInfo_ProductFilterAppliedWhenFilterable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{"id": 123},
			"accounts": []map[string]any{
				{"id": 1, "name": "Basecamp Account", "product": "bc3"},
				{"id": 2, "name": "HEY Account", "product": "hey"},
			},
		})
	}))
	defer server.Close()

	client := newBC5AuthorizationClient(t, server)

	info, err := client.Authorization().GetInfo(t.Context(), &GetInfoOptions{
		Endpoint:      server.URL + "/authorization.json",
		FilterProduct: "nope",
	})
	if err != nil {
		t.Fatalf("GetInfo() unexpected error: %v", err)
	}

	if len(info.Accounts) != 0 {
		t.Errorf("GetInfo() returned %d accounts, want 0 — the filter applies and matched nothing", len(info.Accounts))
	}
	if !info.ProductFilterApplied {
		t.Error("GetInfo() reported ProductFilterApplied = false, want true on a document carrying products")
	}
}

// The rest of the BC5 shape: the resource indicator, the scope, the epoch-seconds
// timestamp Go's FlexTime already handled, and the Launchpad-only fields that
// degrade to "" rather than erroring.
func TestAuthorizationService_GetInfo_BC5DocumentShape(t *testing.T) {
	server := newBC5AuthorizationServer(t)
	client := newBC5AuthorizationClient(t, server)

	info, err := client.Authorization().GetInfo(t.Context(), &GetInfoOptions{
		Endpoint: server.URL + "/authorization.json",
	})
	if err != nil {
		t.Fatalf("GetInfo() unexpected error: %v", err)
	}

	if got := info.Accounts[0].Resource; got != "urn:bc:account:1" {
		t.Errorf("Resource = %q, want urn:bc:account:1", got)
	}
	if info.Scope != "read write" {
		t.Errorf("Scope = %q, want %q", info.Scope, "read write")
	}
	if got := info.ExpiresAt.UTC().Format(time.RFC3339); got != "2036-01-29T09:55:56Z" {
		t.Errorf("ExpiresAt = %s, want 2036-01-29T09:55:56Z", got)
	}
	if info.Identity.EmailAddress != "" {
		t.Errorf("Identity.EmailAddress = %q, want \"\" — a BC5 document omits it", info.Identity.EmailAddress)
	}
	if info.Accounts[0].Product != "" || info.Accounts[0].AppHREF != "" {
		t.Errorf("Product/AppHREF = %q/%q, want empty — Launchpad-only fields",
			info.Accounts[0].Product, info.Accounts[0].AppHREF)
	}
	// ProductFilterApplied is only meaningful when a filter was requested; with
	// no filter it stays at its zero value.
	if info.ProductFilterApplied {
		t.Error("ProductFilterApplied = true with no filter requested, want false")
	}
}

func TestAuthorizationService_GetInfo_RejectsHTTPEndpoint(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.basecamp.com"
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	// HTTP endpoint to a non-localhost host should be rejected
	_, err := client.Authorization().GetInfo(t.Context(), &GetInfoOptions{
		Endpoint: "http://evil.com/authorization.json",
	})

	if err == nil {
		t.Fatal("Expected error for HTTP authorization endpoint, got nil")
	}

	// Check that it's specifically an HTTPS validation error
	if !containsStr(err.Error(), "HTTPS") && !containsStr(err.Error(), "https") {
		t.Errorf("Expected HTTPS-related error, got: %v", err)
	}
}

func TestAuthorizationService_GetInfo_AllowsLocalhostHTTP(t *testing.T) {
	// Start a test server (which runs on localhost/127.0.0.1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{"id": 123},
			"accounts": []map[string]any{},
		})
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithHTTPClient(server.Client()))

	// HTTP localhost endpoint should be allowed
	_, err := client.Authorization().GetInfo(t.Context(), &GetInfoOptions{
		Endpoint: server.URL + "/authorization.json",
	})

	if err != nil {
		t.Errorf("HTTP localhost endpoint should be allowed, got error: %v", err)
	}
}

// containsStr checks if s contains substr (case-insensitive)
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStrHelper(s, substr))
}

func containsStrHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func TestFlexTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantSec int64 // expected Unix timestamp
	}{
		{
			name:    "unix timestamp integer",
			input:   `1705314600`,
			wantErr: false,
			wantSec: 1705314600,
		},
		{
			name:    "RFC 3339 string",
			input:   `"2024-01-15T10:30:00Z"`,
			wantErr: false,
			wantSec: 1705314600,
		},
		{
			name:    "RFC 3339 with timezone offset",
			input:   `"2024-01-15T05:30:00-05:00"`,
			wantErr: false,
			wantSec: 1705314600,
		},
		{
			name:    "zero timestamp",
			input:   `0`,
			wantErr: false,
			wantSec: 0,
		},
		{
			name:    "invalid string format",
			input:   `"not-a-date"`,
			wantErr: true,
		},
		{
			name:    "null value - treated as zero time",
			input:   `null`,
			wantErr: false,
			wantSec: 0,
		},
		{
			name:    "boolean value",
			input:   `true`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ft FlexTime
			err := json.Unmarshal([]byte(tt.input), &ft)

			if (err != nil) != tt.wantErr {
				t.Errorf("FlexTime.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// For zero time (null), check IsZero() instead of Unix()
				if tt.wantSec == 0 && tt.input == `null` {
					if !ft.IsZero() {
						t.Errorf("FlexTime.IsZero() = false, want true for null input")
					}
				} else if ft.Unix() != tt.wantSec {
					t.Errorf("FlexTime.Unix() = %d, want %d", ft.Unix(), tt.wantSec)
				}
			}
		})
	}
}

func TestAuthorizationInfo_UnmarshalWithStringExpiresAt(t *testing.T) {
	jsonData := `{
		"expires_at": "2024-01-15T10:30:00Z",
		"identity": {
			"id": 12345,
			"first_name": "John",
			"last_name": "Doe",
			"email_address": "john@example.com"
		},
		"accounts": [
			{
				"id": 1001,
				"name": "My Company",
				"product": "bc3",
				"href": "https://3.basecampapi.com/1001",
				"app_href": "https://3.basecamp.com/1001",
				"hidden": false,
				"expired": false,
				"featured": true
			},
			{
				"id": 1002,
				"name": "Side Project",
				"product": "bc3",
				"href": "https://3.basecampapi.com/1002",
				"app_href": "https://3.basecamp.com/1002"
			}
		]
	}`

	var info AuthorizationInfo
	if err := json.Unmarshal([]byte(jsonData), &info); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if info.Identity.ID != 12345 {
		t.Errorf("Identity.ID = %d, want 12345", info.Identity.ID)
	}
	if info.Identity.FirstName != "John" {
		t.Errorf("Identity.FirstName = %q, want %q", info.Identity.FirstName, "John")
	}
	if info.Identity.EmailAddress != "john@example.com" {
		t.Errorf("Identity.EmailAddress = %q, want %q", info.Identity.EmailAddress, "john@example.com")
	}
	if len(info.Accounts) != 2 {
		t.Errorf("len(Accounts) = %d, want 2", len(info.Accounts))
	}
	if info.Accounts[0].Name != "My Company" {
		t.Errorf("Accounts[0].Name = %q, want %q", info.Accounts[0].Name, "My Company")
	}
	if !info.Accounts[0].Featured {
		t.Error("Accounts[0].Featured = false, want true")
	}
	// Verify expires_at was parsed correctly
	if info.ExpiresAt.Unix() != 1705314600 {
		t.Errorf("ExpiresAt.Unix() = %d, want 1705314600", info.ExpiresAt.Unix())
	}
}

func TestAuthorizationInfo_UnmarshalWithIntExpiresAt(t *testing.T) {
	// BC3 OAuth 2.1 returns expires_at as Unix timestamp integer
	jsonData := `{
		"expires_at": 2085213356,
		"identity": {
			"id": 149087659,
			"first_name": "Jason",
			"last_name": "Fried"
		},
		"accounts": [
			{
				"id": 181900405,
				"name": "Basecamp's Basecamp",
				"href": "http://3.basecamp.localhost/181900405"
			}
		]
	}`

	var info AuthorizationInfo
	if err := json.Unmarshal([]byte(jsonData), &info); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if info.Identity.ID != 149087659 {
		t.Errorf("Identity.ID = %d, want 149087659", info.Identity.ID)
	}
	if info.ExpiresAt.Unix() != 2085213356 {
		t.Errorf("ExpiresAt.Unix() = %d, want 2085213356", info.ExpiresAt.Unix())
	}
	if len(info.Accounts) != 1 {
		t.Errorf("len(Accounts) = %d, want 1", len(info.Accounts))
	}
}
