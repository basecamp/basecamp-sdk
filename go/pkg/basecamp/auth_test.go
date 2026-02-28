package basecamp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
