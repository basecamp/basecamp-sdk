package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStaticTokenProvider(t *testing.T) {
	p := &StaticTokenProvider{Token: "my-token"}
	tok, err := p.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if tok != "my-token" {
		t.Errorf("AccessToken = %q, want %q", tok, "my-token")
	}
}

func TestCredentialStore_FileRoundTrip(t *testing.T) {
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := "https://3.basecampapi.com"

	creds := &Credentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Unix() + 3600,
		Scope:        "full",
		UserID:       "user-1",
	}

	if err := store.Save(origin, creds); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(origin)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.AccessToken != "access" {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, "access")
	}
	if loaded.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", loaded.UserID, "user-1")
	}
}

func TestCredentialStore_LoadNotFound(t *testing.T) {
	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	_, err := store.Load("https://unknown.example.com")
	if err == nil {
		t.Error("expected error loading missing credentials")
	}
}

func TestCredentialStore_Delete(t *testing.T) {
	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := "https://3.basecampapi.com"

	_ = store.Save(origin, &Credentials{AccessToken: "tok"})

	if err := store.Delete(origin); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.Load(origin)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestCredentialStore_MultipleOrigins(t *testing.T) {
	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}

	_ = store.Save("https://a.example.com", &Credentials{AccessToken: "tok-a"})
	_ = store.Save("https://b.example.com", &Credentials{AccessToken: "tok-b"})

	a, _ := store.Load("https://a.example.com")
	b, _ := store.Load("https://b.example.com")

	if a.AccessToken != "tok-a" {
		t.Errorf("origin A: %q, want %q", a.AccessToken, "tok-a")
	}
	if b.AccessToken != "tok-b" {
		t.Errorf("origin B: %q, want %q", b.AccessToken, "tok-b")
	}
}

func TestAuthManager_AccessToken_EnvVar(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "env-token")

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	m := NewAuthManagerWithStore(cfg, &http.Client{}, &CredentialStore{fallbackDir: t.TempDir()})

	tok, err := m.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if tok != "env-token" {
		t.Errorf("AccessToken = %q, want %q", tok, "env-token")
	}
}

func TestAuthManager_IsAuthenticated_EnvVar(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "env-token")

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	m := NewAuthManagerWithStore(cfg, &http.Client{}, &CredentialStore{fallbackDir: t.TempDir()})

	if !m.IsAuthenticated() {
		t.Error("expected IsAuthenticated=true with BASECAMP_TOKEN")
	}
}

func TestAuthManager_Refresh(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
		})
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)
	_ = store.Save(origin, &Credentials{
		AccessToken:   "old-access",
		RefreshToken:  "old-refresh",
		ExpiresAt:     1,
		TokenEndpoint: ts.URL + "/token",
	})

	cfg := &Config{BaseURL: ts.URL}
	m := NewAuthManagerWithStore(cfg, ts.Client(), store)

	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	creds, _ := store.Load(origin)
	if creds.AccessToken != "new-access" {
		t.Errorf("AccessToken = %q, want %q", creds.AccessToken, "new-access")
	}
	if creds.RefreshToken != "new-refresh" {
		t.Errorf("RefreshToken = %q, want %q", creds.RefreshToken, "new-refresh")
	}
}

func TestAuthManager_Refresh_OversizedResponse(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	// Respond with >1MB body
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"`))
		w.Write([]byte(strings.Repeat("x", 2*1024*1024)))
		w.Write([]byte(`"}`))
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)
	_ = store.Save(origin, &Credentials{
		AccessToken:   "old",
		RefreshToken:  "refresh",
		ExpiresAt:     1,
		TokenEndpoint: ts.URL + "/token",
	})

	cfg := &Config{BaseURL: ts.URL}
	m := NewAuthManagerWithStore(cfg, ts.Client(), store)

	err := m.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error for oversized response")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("error = %q, expected mention of size limit", err.Error())
	}
}

func TestAuthManager_TokenExpiryBuffer(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	refreshCalled := false
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "refreshed",
			"expires_in":   3600,
		})
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)

	// Token expires in 4 minutes (within the 5-minute buffer)
	_ = store.Save(origin, &Credentials{
		AccessToken:   "about-to-expire",
		RefreshToken:  "refresh",
		ExpiresAt:     time.Now().Unix() + 240,
		TokenEndpoint: ts.URL + "/token",
	})

	cfg := &Config{BaseURL: ts.URL}
	m := NewAuthManagerWithStore(cfg, ts.Client(), store)

	tok, err := m.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if !refreshCalled {
		t.Error("expected refresh to be called for token expiring within buffer")
	}
	if tok != "refreshed" {
		t.Errorf("AccessToken = %q, want %q", tok, "refreshed")
	}
}

func TestAuthManager_Refresh_NoRefreshToken(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := "https://3.basecampapi.com"
	_ = store.Save(origin, &Credentials{
		AccessToken: "tok",
		ExpiresAt:   time.Now().Unix() + 3600,
	})

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	m := NewAuthManagerWithStore(cfg, &http.Client{}, store)

	err := m.Refresh(context.Background())
	if err == nil {
		t.Error("expected error when no refresh token")
	}
}

func TestAuthManager_Refresh_NoTokenEndpoint(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := "https://3.basecampapi.com"
	_ = store.Save(origin, &Credentials{
		AccessToken:  "tok",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Unix() + 3600,
	})

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	m := NewAuthManagerWithStore(cfg, &http.Client{}, store)

	err := m.Refresh(context.Background())
	if err == nil {
		t.Error("expected error when no token endpoint")
	}
}

func TestAuthManager_GetSetUserID(t *testing.T) {
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := "https://3.basecampapi.com"
	_ = store.Save(origin, &Credentials{AccessToken: "tok"})

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	m := NewAuthManagerWithStore(cfg, &http.Client{}, store)

	if err := m.SetUserID("user-42"); err != nil {
		t.Fatalf("SetUserID: %v", err)
	}

	got := m.GetUserID()
	if got != "user-42" {
		t.Errorf("GetUserID = %q, want %q", got, "user-42")
	}
}

func TestAuthManager_Logout(t *testing.T) {
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := "https://3.basecampapi.com"
	_ = store.Save(origin, &Credentials{AccessToken: "tok"})

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	m := NewAuthManagerWithStore(cfg, &http.Client{}, store)

	if err := m.Logout(); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	if m.IsAuthenticated() {
		t.Error("expected not authenticated after logout")
	}
}

func TestCredentialStore_UsingKeyring(t *testing.T) {
	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	if store.UsingKeyring() {
		t.Error("expected UsingKeyring=false")
	}
}

func TestAuthManager_Refresh_EchoesClientIDAndResource(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	// The refresh response deliberately OMITS resource: the stored binding
	// must be echoed on the form and survive the rotation (SPEC §16).
	var gotClientID, gotResource string
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotClientID = r.PostFormValue("client_id")
		gotResource = r.PostFormValue("resource")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
		})
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)
	_ = store.Save(origin, &Credentials{
		AccessToken:   "old-access",
		RefreshToken:  "old-refresh",
		ExpiresAt:     1,
		TokenEndpoint: ts.URL + "/token",
		ClientID:      "basecamp-cli",
		Resource:      "urn:bc:account:42",
	})

	cfg := &Config{BaseURL: ts.URL}
	m := NewAuthManagerWithStore(cfg, ts.Client(), store)

	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if gotClientID != "basecamp-cli" {
		t.Errorf("client_id form value = %q, want basecamp-cli", gotClientID)
	}
	if gotResource != "urn:bc:account:42" {
		t.Errorf("resource form value = %q, want urn:bc:account:42", gotResource)
	}

	creds, _ := store.Load(origin)
	if creds.Resource != "urn:bc:account:42" {
		t.Errorf("stored Resource = %q, want preserved urn:bc:account:42", creds.Resource)
	}
	if creds.ClientID != "basecamp-cli" {
		t.Errorf("stored ClientID = %q, want basecamp-cli", creds.ClientID)
	}
}

func TestAuthManager_Refresh_ResponseResourceReplacesStored(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			"resource":     "urn:bc:account:7",
		})
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)
	_ = store.Save(origin, &Credentials{
		AccessToken:   "old-access",
		RefreshToken:  "old-refresh",
		ExpiresAt:     1,
		TokenEndpoint: ts.URL + "/token",
		Resource:      "urn:bc:account:42",
	})

	cfg := &Config{BaseURL: ts.URL}
	m := NewAuthManagerWithStore(cfg, ts.Client(), store)

	if err := m.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	creds, _ := store.Load(origin)
	if creds.Resource != "urn:bc:account:7" {
		t.Errorf("stored Resource = %q, want replaced urn:bc:account:7", creds.Resource)
	}
}

func TestAuthManager_Refresh_RejectsEmptyResource(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	// A present-but-empty resource is malformed (SPEC §16): the refresh must
	// FAIL — persisting the rotated tokens while silently keeping the old
	// binding would store credentials under a stale resource.
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			"resource":     "",
		})
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)
	_ = store.Save(origin, &Credentials{
		AccessToken:   "old-access",
		RefreshToken:  "old-refresh",
		ExpiresAt:     1,
		TokenEndpoint: ts.URL + "/token",
		Resource:      "urn:bc:account:42",
	})

	cfg := &Config{BaseURL: ts.URL}
	m := NewAuthManagerWithStore(cfg, ts.Client(), store)

	err := m.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error for present-but-empty resource")
	}
	if !strings.Contains(err.Error(), "resource") {
		t.Errorf("error = %q, expected mention of resource", err.Error())
	}

	// The stored credentials must be untouched — no rotation was persisted.
	creds, _ := store.Load(origin)
	if creds.AccessToken != "old-access" {
		t.Errorf("AccessToken = %q, want old-access untouched", creds.AccessToken)
	}
	if creds.Resource != "urn:bc:account:42" {
		t.Errorf("Resource = %q, want untouched binding", creds.Resource)
	}
}

func TestAuthManager_Refresh_MissingAccessTokenIsAPIError(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	// A 200 with a missing (or empty) access_token is a malformed response,
	// not a rotation: persisting it would overwrite working credentials with
	// an unusable empty token — an effective logout. The device and exchange
	// paths already classify this as an api fault; the refresh must too.
	for _, body := range []map[string]any{
		{"refresh_token": "rotated-refresh"},
		{"access_token": "", "refresh_token": "rotated-refresh"},
	} {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(body)
		}))

		store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
		origin := NormalizeBaseURL(ts.URL)
		_ = store.Save(origin, &Credentials{
			AccessToken:   "old-access",
			RefreshToken:  "old-refresh",
			ExpiresAt:     1,
			TokenEndpoint: ts.URL + "/token",
		})

		cfg := &Config{BaseURL: ts.URL}
		m := NewAuthManagerWithStore(cfg, ts.Client(), store)

		err := m.Refresh(context.Background())
		if err == nil {
			t.Fatalf("body %v: expected error for missing access_token", body)
		}
		if !strings.Contains(err.Error(), "access_token") {
			t.Errorf("body %v: error = %q, expected mention of access_token", body, err.Error())
		}

		// The stored credentials must be untouched — no rotation persisted.
		creds, _ := store.Load(origin)
		if creds.AccessToken != "old-access" {
			t.Errorf("body %v: AccessToken = %q, want old-access untouched", body, creds.AccessToken)
		}
		if creds.RefreshToken != "old-refresh" {
			t.Errorf("body %v: RefreshToken = %q, want old-refresh untouched", body, creds.RefreshToken)
		}
		ts.Close()
	}
}

func TestAuthManager_Refresh_NonStringResourceIsAPIError(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	// A non-string resource fails the *string decode; that malformed 200 body
	// must surface as a classifiable api_error, not a raw UnmarshalTypeError.
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			"resource":     7,
		})
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)
	_ = store.Save(origin, &Credentials{
		AccessToken:   "old-access",
		RefreshToken:  "old-refresh",
		ExpiresAt:     1,
		TokenEndpoint: ts.URL + "/token",
	})

	cfg := &Config{BaseURL: ts.URL}
	m := NewAuthManagerWithStore(cfg, ts.Client(), store)

	err := m.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error for non-string resource")
	}
	var be *Error
	if !errors.As(err, &be) || be.Code != CodeAPI {
		t.Errorf("want *basecamp.Error with code %q, got %v", CodeAPI, err)
	}
}

func TestAuthManager_AccessToken_NoExpiryIsNotRefreshed(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	// ExpiresAt <= 0 means no known expiry (a token response may omit
	// expires_in): the credential is used as-is — a forced refresh would
	// hard-fail a fresh, usable token that carries no refresh token.
	refreshed := false
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshed = true
		http.NotFound(w, r)
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)
	for _, expiresAt := range []int64{0, -62135596800} { // zero and time.Time{}.Unix()
		_ = store.Save(origin, &Credentials{
			AccessToken:   "fresh-access",
			TokenEndpoint: ts.URL + "/token",
			ExpiresAt:     expiresAt,
		})

		m := NewAuthManagerWithStore(&Config{BaseURL: ts.URL}, ts.Client(), store)
		tok, err := m.AccessToken(context.Background())
		if err != nil {
			t.Fatalf("ExpiresAt=%d: %v", expiresAt, err)
		}
		if tok != "fresh-access" {
			t.Errorf("ExpiresAt=%d: token = %q", expiresAt, tok)
		}
	}
	if refreshed {
		t.Error("no-expiry credentials must never be force-refreshed")
	}
}

func TestAuthManager_Refresh_OmittedExpiresInClearsStaleExpiry(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	// A refresh response that omits expires_in must CLEAR the old expiry:
	// keeping the stale past ExpiresAt would force a refresh on every
	// subsequent AccessToken call despite the fresh token.
	refreshes := 0
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshes++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
		})
	}))
	defer ts.Close()

	store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
	origin := NormalizeBaseURL(ts.URL)
	_ = store.Save(origin, &Credentials{
		AccessToken:   "old-access",
		RefreshToken:  "old-refresh",
		ExpiresAt:     1, // long past
		TokenEndpoint: ts.URL + "/token",
	})

	m := NewAuthManagerWithStore(&Config{BaseURL: ts.URL}, ts.Client(), store)

	// First call refreshes (stale expiry) and stores the new token with no
	// known expiry; the second call must use it as-is.
	for i := 0; i < 2; i++ {
		tok, err := m.AccessToken(context.Background())
		if err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
		if tok != "new-access" {
			t.Errorf("call %d: token = %q", i+1, tok)
		}
	}
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want exactly 1 (stale expiry cleared)", refreshes)
	}

	creds, _ := store.Load(origin)
	if creds.ExpiresAt != 0 {
		t.Errorf("ExpiresAt = %d, want 0 (no known expiry)", creds.ExpiresAt)
	}
}

func TestAuthManager_Refresh_ExplicitNonPositiveExpiresInIsAPIError(t *testing.T) {
	t.Setenv("BASECAMP_TOKEN", "")
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	// An explicit zero/negative expires_in is malformed — distinct from
	// omission (which clears to no-known-expiry): persisting it as
	// non-expiring would store an already-expired token that never refreshes.
	for _, expiresIn := range []int64{0, -30, 9223372036854775807} {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "new-access",
				"expires_in":   expiresIn,
			})
		}))

		store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
		origin := NormalizeBaseURL(ts.URL)
		_ = store.Save(origin, &Credentials{
			AccessToken:   "old-access",
			RefreshToken:  "old-refresh",
			ExpiresAt:     1,
			TokenEndpoint: ts.URL + "/token",
		})

		m := NewAuthManagerWithStore(&Config{BaseURL: ts.URL}, ts.Client(), store)
		err := m.Refresh(context.Background())
		if err == nil {
			t.Fatalf("expires_in=%d: expected api_error", expiresIn)
		}
		var be *Error
		if !errors.As(err, &be) || be.Code != CodeAPI {
			t.Errorf("expires_in=%d: want *basecamp.Error CodeAPI, got %v", expiresIn, err)
		}

		// Nothing was persisted — the old credentials stand.
		creds, _ := store.Load(origin)
		if creds.AccessToken != "old-access" {
			t.Errorf("expires_in=%d: AccessToken = %q, want untouched", expiresIn, creds.AccessToken)
		}
		ts.Close()
	}
}

// TestAuthManager_Refresh_RefusesTokenEndpointRedirects pins the stored-
// endpoint refresh to the same transport policy as the exchange path (SPEC
// §16 "Token-Endpoint Transport Policy"): every refused redirect is a typed
// api_error carrying its status, the Location host is never dialed — the
// operator's client would otherwise follow, 307/308 re-POSTing the refresh
// token — and the stored credentials survive untouched.
func TestAuthManager_Refresh_RefusesTokenEndpointRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprintf("%d", status), func(t *testing.T) {
			t.Setenv("BASECAMP_TOKEN", "")
			t.Setenv("BASECAMP_NO_KEYRING", "1")

			var hits atomic.Int64
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"stolen","expires_in":3600}`))
			}))
			defer target.Close()

			ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(status)
			}))
			defer ts.Close()

			store := &CredentialStore{useKeyring: false, fallbackDir: t.TempDir()}
			origin := NormalizeBaseURL(ts.URL)
			_ = store.Save(origin, &Credentials{
				AccessToken:   "old-access",
				RefreshToken:  "old-refresh",
				ExpiresAt:     1,
				TokenEndpoint: ts.URL + "/token",
			})

			m := NewAuthManagerWithStore(&Config{BaseURL: ts.URL}, ts.Client(), store)

			err := m.Refresh(context.Background())
			var be *Error
			if !errors.As(err, &be) {
				t.Fatalf("Refresh() error = %v, want *Error", err)
			}
			if be.Code != CodeAPI {
				t.Errorf("Code = %q, want %q", be.Code, CodeAPI)
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
			creds, _ := store.Load(origin)
			if creds.AccessToken != "old-access" {
				t.Errorf("AccessToken = %q, want the stored credentials untouched", creds.AccessToken)
			}
		})
	}
}
