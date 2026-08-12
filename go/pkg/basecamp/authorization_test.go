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
				// Every success fixture here omits expires_at — a synthetic
				// shape (production issuers always state one) that exercises
				// the absence sentinel: it must read as "no expiry known",
				// not as a fabricated instant.
				if !info.ExpiresAt.IsZero() {
					t.Errorf("ExpiresAt = %v for a document without expires_at, want zero", info.ExpiresAt.Time)
				}
				if expiry, ok := info.Expiry(); ok {
					t.Errorf("Expiry() = (%v, true) for a document without expires_at, want ok=false", expiry)
				}
			}
		})
	}
}

// bc5AuthorizationDocument is what a BC5 (bc3) issuer serves from
// app/views/api/authorizations/show.json.jbuilder: identity id only, no product
// or app_href on accounts, an RFC 8707 resource indicator instead, a top-level
// scope, and expires_at as an ISO 8601 string (integer epoch seconds before
// bc3 #12646 converged it; TestAuthorizationInfo_UnmarshalWithIntExpiresAt
// keeps the integer spelling covered).
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
		"scope":      "read write",
		"expires_at": "2036-01-29T09:55:56Z",
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

// Launchpad returns an empty account list for an identity with no currently
// accessible accounts. "No account carries a product" is vacuously true there,
// but the document is perfectly filterable — reporting the filter inapplicable
// would assert "this issuer cannot filter by product" on no evidence at all.
func TestAuthorizationService_GetInfo_EmptyAccountListIsFilterable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"identity": map[string]any{"id": 123},
			"accounts": []map[string]any{},
		})
	}))
	defer server.Close()

	client := newBC5AuthorizationClient(t, server)

	info, err := client.Authorization().GetInfo(t.Context(), &GetInfoOptions{
		Endpoint:      server.URL + "/authorization.json",
		FilterProduct: "bc3",
	})
	if err != nil {
		t.Fatalf("GetInfo() unexpected error: %v", err)
	}

	if len(info.Accounts) != 0 {
		t.Errorf("GetInfo() returned %d accounts, want 0", len(info.Accounts))
	}
	if !info.ProductFilterApplied {
		t.Error("GetInfo() reported ProductFilterApplied = false on an empty list, want true — " +
			"an empty list is no evidence that the issuer omits product")
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
	if expiry, ok := info.Expiry(); !ok {
		t.Error("Expiry() ok = false for a document with a real expires_at, want true")
	} else if !expiry.Equal(time.Unix(2085213356, 0)) {
		t.Errorf("Expiry() = %v, want %v", expiry, time.Unix(2085213356, 0))
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
		wantSec int64 // expected Unix timestamp, when wantZero is false
		// wantZero asserts the zero time — "no expiry known". null and integer 0
		// both land here: bc3 renders `expires_at.to_i`, so a wire 0 would be its
		// spelling of an unstated expiry, and RFC 7591 already gives 0 the meaning
		// "never expires" (bc3's own client_secret_expires_at) — the one reading
		// that must not survive is "expired at the 1970 epoch".
		wantZero bool
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
			name:     "zero timestamp - treated as zero time, not the 1970 epoch",
			input:    `0`,
			wantErr:  false,
			wantZero: true,
		},
		{
			name:    "invalid string format",
			input:   `"not-a-date"`,
			wantErr: true,
		},
		{
			name:     "null value - treated as zero time",
			input:    `null`,
			wantErr:  false,
			wantZero: true,
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
				if tt.wantZero {
					if !ft.IsZero() {
						t.Errorf("FlexTime.IsZero() = false for input %s, want true — got %v", tt.input, ft.Time)
					}
				} else if ft.Unix() != tt.wantSec {
					t.Errorf("FlexTime.Unix() = %d, want %d", ft.Unix(), tt.wantSec)
				}
			}
		})
	}
}

// TestAuthorizationInfo_NoExpiryKnown pins the sentinel contract for #662: an
// absent expires_at, an explicit null, and a wire `0` all
// decode to the zero ExpiresAt, report Expiry() ok=false, and re-marshal as
// null — never as 0001-01-01T00:00:00Z, and never as a valid 1970 date.
//
// AuthorizationInfo is deliberately NOT in timestampCarriers(): that table
// asserts key *omission* on re-marshal, and expires_at is a value field whose
// honest re-marshal of "no expiry known" is an explicit null.
func TestAuthorizationInfo_NoExpiryKnown(t *testing.T) {
	cases := []struct {
		name string
		doc  string
	}{
		{"absent", `{"identity":{"id":1},"accounts":[]}`},
		{"null", `{"expires_at":null,"identity":{"id":1},"accounts":[]}`},
		{"integer zero", `{"expires_at":0,"identity":{"id":1},"accounts":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var info AuthorizationInfo
			if err := json.Unmarshal([]byte(tc.doc), &info); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if !info.ExpiresAt.IsZero() {
				t.Errorf("ExpiresAt = %v, want zero — %s must read as \"no expiry known\"", info.ExpiresAt.Time, tc.name)
			}
			if expiry, ok := info.Expiry(); ok {
				t.Errorf("Expiry() = (%v, true), want ok=false for %s expires_at", expiry, tc.name)
			}
			b, err := json.Marshal(info)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			var out map[string]json.RawMessage
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatalf("re-unmarshal %s: %v", b, err)
			}
			if got := string(out["expires_at"]); got != "null" {
				t.Errorf("expires_at re-marshaled as %s, want null", got)
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
	// bc3 rendered expires_at as integer epoch seconds before bc3 #12646
	// converged it on ISO 8601. The integer spelling stays accepted — recorded
	// documents and older deploys still carry it — and this test is what keeps
	// that compatibility covered now that the BC5 fixture speaks ISO 8601.
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
