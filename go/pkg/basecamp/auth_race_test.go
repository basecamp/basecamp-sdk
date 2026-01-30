package basecamp

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// TestAuthManager_ConcurrentAccess verifies that AuthManager methods are safe
// for concurrent use. Run with -race flag to detect data races:
//
//	go test -race -run TestAuthManager_ConcurrentAccess ./...
func TestAuthManager_ConcurrentAccess(t *testing.T) {
	// Create a test credential store with in-memory backend
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	tmpDir := t.TempDir()
	store := &CredentialStore{useKeyring: false, fallbackDir: tmpDir}

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	m := NewAuthManagerWithStore(cfg, &http.Client{}, store)

	// Pre-populate credentials to avoid "not authenticated" errors
	origin := NormalizeBaseURL(cfg.BaseURL)
	_ = store.Save(origin, &Credentials{
		AccessToken:  "initial-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    9999999999,
	})

	ctx := context.Background()
	var wg sync.WaitGroup

	// Hammer all AuthManager methods concurrently
	for i := 0; i < 100; i++ {
		wg.Add(4)

		go func() {
			defer wg.Done()
			_, _ = m.AccessToken(ctx)
		}()

		go func() {
			defer wg.Done()
			_ = m.IsAuthenticated()
		}()

		go func() {
			defer wg.Done()
			_ = m.GetUserID()
		}()

		go func(userID string) {
			defer wg.Done()
			_ = m.SetUserID(userID)
		}("user-" + string(rune('A'+i%26)))
	}

	wg.Wait()
}

// TestAuthManager_LogoutDuringRefresh verifies that Logout doesn't race
// with token refresh operations. This test sets expired credentials to
// trigger the refresh path in AccessToken().
func TestAuthManager_LogoutDuringRefresh(t *testing.T) {
	t.Setenv("BASECAMP_NO_KEYRING", "1")

	tmpDir := t.TempDir()
	store := &CredentialStore{useKeyring: false, fallbackDir: tmpDir}

	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	m := NewAuthManagerWithStore(cfg, &http.Client{}, store)

	origin := NormalizeBaseURL(cfg.BaseURL)
	ctx := context.Background()

	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		// Re-populate credentials with EXPIRED token to trigger refresh path
		_ = store.Save(origin, &Credentials{
			AccessToken:  "expired-token",
			RefreshToken: "refresh-token",
			ExpiresAt:    1, // Expired (Unix timestamp 1 = 1970)
		})

		wg.Add(2)

		go func() {
			defer wg.Done()
			_ = m.Logout()
		}()

		go func() {
			defer wg.Done()
			// AccessToken with expired creds triggers refresh path
			// (refresh will fail due to mock server, but the race is exercised)
			_, _ = m.AccessToken(ctx)
		}()
	}

	wg.Wait()
}

// TestAccountClient_ConcurrentServiceAccess verifies that service accessors
// are safe for concurrent use.
func TestAccountClient_ConcurrentServiceAccess(t *testing.T) {
	cfg := &Config{BaseURL: "https://3.basecampapi.com"}
	client := NewClient(cfg, &StaticTokenProvider{Token: "token"})
	ac := client.ForAccount("12345")

	var wg sync.WaitGroup

	// Hammer service accessors concurrently
	for i := 0; i < 100; i++ {
		wg.Add(6)

		go func() {
			defer wg.Done()
			_ = ac.Projects()
		}()

		go func() {
			defer wg.Done()
			_ = ac.Todos()
		}()

		go func() {
			defer wg.Done()
			_ = ac.People()
		}()

		go func() {
			defer wg.Done()
			_ = ac.Comments()
		}()

		go func() {
			defer wg.Done()
			_ = ac.Webhooks()
		}()

		go func() {
			defer wg.Done()
			_ = ac.Search()
		}()
	}

	wg.Wait()

	// Verify all accessors return the same instance (singleton pattern)
	p1 := ac.Projects()
	p2 := ac.Projects()
	if p1 != p2 {
		t.Error("Expected Projects() to return the same instance")
	}
}
