package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

const serviceName = "basecamp-sdk"

// maxRefreshTokenLifetimeSeconds caps a refresh response's expires_in at the
// shared cross-SDK token-lifetime ceiling (2147483647 s ≈ 68 years, SPEC §16
// — the same bound the oauth package's device/exchange parsers enforce). It
// bounds the Unix-time addition in refreshLocked: an unbounded value like
// math.MaxInt64 would overflow into a negative ExpiresAt that the
// no-known-expiry rule reads as never-expiring.
const maxRefreshTokenLifetimeSeconds = 2_147_483_647

// Credentials holds OAuth tokens and metadata.
type Credentials struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	ExpiresAt     int64  `json:"expires_at"`
	Scope         string `json:"scope"`
	TokenEndpoint string `json:"token_endpoint"`
	UserID        string `json:"user_id,omitempty"`

	// ClientID is the OAuth client the tokens were issued to. BC5 public
	// clients (token_endpoint_auth_method: none) authenticate refreshes by
	// client_id alone, so refresh submits it when present.
	ClientID string `json:"client_id,omitempty"`

	// Resource is the RFC 8707 resource indicator the tokens are bound to
	// (BC5: urn:bc:account:<id>). Refresh echoes it and preserves it when a
	// refresh response omits it — BC5 multi-account refresh tokens reject a
	// refresh without it (SPEC §16).
	//
	// The oauth helpers only return an oauth.Token; after a device login or
	// code exchange the CALLER saves Credentials carrying the ClientID it
	// used and the token's Resource (see the README OAuth section) — there is
	// no automatic bridge from oauth.Token to Credentials.
	Resource string `json:"resource,omitempty"`
}

// TokenProvider is the interface for obtaining access tokens.
type TokenProvider interface {
	// AccessToken returns a valid access token, refreshing if needed.
	AccessToken(ctx context.Context) (string, error)
}

// StaticTokenProvider provides a fixed token (e.g., from BASECAMP_TOKEN env var).
type StaticTokenProvider struct {
	Token string
}

// AccessToken returns the static token.
func (p *StaticTokenProvider) AccessToken(ctx context.Context) (string, error) {
	return p.Token, nil
}

// CredentialStore handles secure credential storage.
type CredentialStore struct {
	useKeyring  bool
	fallbackDir string
}

// NewCredentialStore creates a credential store.
// It prefers the system keyring if available, falling back to file storage.
func NewCredentialStore(fallbackDir string) *CredentialStore {
	// Skip keyring for tests or when explicitly disabled
	if os.Getenv("BASECAMP_NO_KEYRING") != "" {
		return &CredentialStore{useKeyring: false, fallbackDir: fallbackDir}
	}

	// Test if keyring is available
	testKey := "basecamp-sdk::test"
	err := keyring.Set(serviceName, testKey, "test")
	if err == nil {
		_ = keyring.Delete(serviceName, testKey) // Cleanup test key
		return &CredentialStore{useKeyring: true, fallbackDir: fallbackDir}
	}
	return &CredentialStore{useKeyring: false, fallbackDir: fallbackDir}
}

// keyFor returns the storage key for an origin.
func keyFor(origin string) string {
	return fmt.Sprintf("basecamp-sdk::%s", origin)
}

// Load retrieves credentials for the given origin.
func (s *CredentialStore) Load(origin string) (*Credentials, error) {
	if s.useKeyring {
		return s.loadFromKeyring(origin)
	}
	return s.loadFromFile(origin)
}

// Save stores credentials for the given origin.
func (s *CredentialStore) Save(origin string, creds *Credentials) error {
	if s.useKeyring {
		return s.saveToKeyring(origin, creds)
	}
	return s.saveToFile(origin, creds)
}

// Delete removes credentials for the given origin.
func (s *CredentialStore) Delete(origin string) error {
	if s.useKeyring {
		return keyring.Delete(serviceName, keyFor(origin))
	}
	return s.deleteFile(origin)
}

// UsingKeyring returns true if the store is using the system keyring.
func (s *CredentialStore) UsingKeyring() bool {
	return s.useKeyring
}

func (s *CredentialStore) loadFromKeyring(origin string) (*Credentials, error) {
	data, err := keyring.Get(serviceName, keyFor(origin))
	if err != nil {
		return nil, fmt.Errorf("credentials not found: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}
	return &creds, nil
}

func (s *CredentialStore) saveToKeyring(origin string, creds *Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return keyring.Set(serviceName, keyFor(origin), string(data))
}

func (s *CredentialStore) credentialsPath() string {
	return s.fallbackDir + "/credentials.json"
}

func (s *CredentialStore) loadAllFromFile() (map[string]*Credentials, error) {
	data, err := os.ReadFile(s.credentialsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*Credentials), nil
		}
		return nil, err
	}

	var all map[string]*Credentials
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	return all, nil
}

func (s *CredentialStore) saveAllToFile(all map[string]*Credentials) error {
	if err := os.MkdirAll(s.fallbackDir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write using unique temp file to avoid collisions
	tmpFile, err := os.CreateTemp(s.fallbackDir, "credentials-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath) // #nosec G703 -- path derived from caller-configured credentials directory
		return err
	}
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath) // #nosec G703 -- path derived from caller-configured credentials directory
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath) // #nosec G703 -- path derived from caller-configured credentials directory
		return err
	}
	if err := os.Rename(tmpPath, s.credentialsPath()); err != nil { // #nosec G703 -- path derived from caller-configured credentials directory
		_ = os.Remove(tmpPath) // #nosec G703 -- path derived from caller-configured credentials directory
		return err
	}
	return nil
}

func (s *CredentialStore) loadFromFile(origin string) (*Credentials, error) {
	all, err := s.loadAllFromFile()
	if err != nil {
		return nil, err
	}

	creds, ok := all[origin]
	if !ok {
		return nil, fmt.Errorf("credentials not found for %s", origin)
	}
	return creds, nil
}

func (s *CredentialStore) saveToFile(origin string, creds *Credentials) error {
	all, err := s.loadAllFromFile()
	if err != nil {
		return err
	}

	all[origin] = creds
	return s.saveAllToFile(all)
}

func (s *CredentialStore) deleteFile(origin string) error {
	all, err := s.loadAllFromFile()
	if err != nil {
		return err
	}

	delete(all, origin)
	return s.saveAllToFile(all)
}

// AuthManager handles OAuth token management.
type AuthManager struct {
	cfg        *Config
	store      *CredentialStore
	httpClient *http.Client
	mu         sync.Mutex
}

// NewAuthManager creates a new auth manager.
func NewAuthManager(cfg *Config, httpClient *http.Client) *AuthManager {
	return &AuthManager{
		cfg:        cfg,
		store:      NewCredentialStore(globalConfigDir()),
		httpClient: httpClient,
	}
}

// NewAuthManagerWithStore creates an auth manager with a custom credential store.
func NewAuthManagerWithStore(cfg *Config, httpClient *http.Client, store *CredentialStore) *AuthManager {
	return &AuthManager{
		cfg:        cfg,
		store:      store,
		httpClient: httpClient,
	}
}

// AccessToken returns a valid access token, refreshing if needed.
// If BASECAMP_TOKEN env var is set, it's used directly without OAuth.
func (m *AuthManager) AccessToken(ctx context.Context) (string, error) {
	// Check for BASECAMP_TOKEN environment variable first
	if token := os.Getenv("BASECAMP_TOKEN"); token != "" {
		return token, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	origin := NormalizeBaseURL(m.cfg.BaseURL)
	creds, err := m.store.Load(origin)
	if err != nil {
		return "", ErrAuth("Not authenticated")
	}

	// Check if token is expired (with 5 minute buffer). ExpiresAt <= 0 means
	// NO KNOWN EXPIRY (a token response may legally omit expires_in — the
	// device/exchange parsers leave ExpiresAt zero, and refreshLocked stores
	// no expiry when the server sends none): such credentials are used as-is,
	// never force-refreshed — a fresh token without a refresh token would
	// otherwise hard-fail here despite being perfectly usable.
	if creds.ExpiresAt > 0 && time.Now().Unix() >= creds.ExpiresAt-300 {
		if err := m.refreshLocked(ctx, origin, creds); err != nil {
			return "", err
		}
		// Reload refreshed credentials
		creds, err = m.store.Load(origin)
		if err != nil {
			return "", err
		}
	}

	return creds.AccessToken, nil
}

// IsAuthenticated checks if there are valid credentials.
func (m *AuthManager) IsAuthenticated() bool {
	// Check for BASECAMP_TOKEN environment variable first
	if os.Getenv("BASECAMP_TOKEN") != "" {
		return true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	origin := NormalizeBaseURL(m.cfg.BaseURL)
	creds, err := m.store.Load(origin)
	if err != nil {
		return false
	}
	return creds.AccessToken != ""
}

// Refresh forces a token refresh.
func (m *AuthManager) Refresh(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	origin := NormalizeBaseURL(m.cfg.BaseURL)
	creds, err := m.store.Load(origin)
	if err != nil {
		return ErrAuth("Not authenticated")
	}

	return m.refreshLocked(ctx, origin, creds)
}

func (m *AuthManager) refreshLocked(ctx context.Context, origin string, creds *Credentials) error {
	if creds.RefreshToken == "" {
		return ErrAuth("No refresh token available")
	}

	tokenEndpoint := creds.TokenEndpoint
	if tokenEndpoint == "" {
		return ErrAuth("No token endpoint stored")
	}
	if err := RequireSecureEndpoint(tokenEndpoint); err != nil {
		return ErrAuth(fmt.Sprintf("Token endpoint must use HTTPS: %s", tokenEndpoint))
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", creds.RefreshToken)
	if creds.ClientID != "" {
		data.Set("client_id", creds.ClientID)
	}
	if creds.Resource != "" {
		data.Set("resource", creds.Resource)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// The stored endpoint's response steers where a followed redirect would
	// re-POST the refresh token, so this hop never follows one (SPEC §16
	// "Token-Endpoint Transport Policy") — same as the signed download hop and
	// the oauth package's exchange. A shallow copy: the operator-configured
	// client is never mutated, and keeps every other property it was built with.
	client := *m.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Do(req) // #nosec G704 -- SDK HTTP client: URL is caller-configured
	if err != nil {
		return ErrNetwork(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Classified by status BEFORE the body read, so a redirect with a stalled
	// body fails here rather than hanging in limitedReadAll.
	if isRedirectStatus(resp.StatusCode) {
		return ErrAPI(resp.StatusCode, fmt.Sprintf("redirect %d on the token endpoint is not followed", resp.StatusCode))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body, MaxErrorBodyBytes)
		return ErrAPI(resp.StatusCode, fmt.Sprintf("token refresh failed: %s", truncateString(string(body), MaxErrorMessageBytes)))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		// *int64 so an omitted (or null) expires_in is distinguishable from an
		// explicit value: omission clears the stale expiry (no known expiry),
		// while an explicit non-positive value is a malformed response.
		ExpiresIn *int64 `json:"expires_in"`
		// *string so an omitted (or null) resource is distinguishable from a
		// present one: omission preserves the stored value, presence replaces
		// it (SPEC §16 lifecycle rule).
		Resource *string `json:"resource"`
	}
	const maxTokenResponseSize int64 = 1 << 20 // 1 MB
	body, err := limitedReadAll(resp.Body, maxTokenResponseSize)
	if err != nil {
		return fmt.Errorf("reading token response: %w", err)
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		// A malformed 200 body — including a non-string resource failing the
		// *string decode — is an api fault (SPEC §16), not a raw
		// json.UnmarshalTypeError callers cannot classify.
		return ErrAPI(resp.StatusCode, fmt.Sprintf("parsing token refresh response: %v", err))
	}

	// A 200 with a missing or empty access_token is a malformed response
	// (SPEC §16), not a rotation: persisting it would overwrite working
	// credentials with an unusable empty token — an effective logout. Fail
	// before mutating anything, matching the device/exchange paths.
	if tokenResp.AccessToken == "" {
		return ErrAPI(resp.StatusCode, "token refresh response missing access_token")
	}

	// Validate EVERYTHING before mutating creds — an error return below must
	// not leave a partially-updated in-memory Credentials behind (the nearby
	// access_token check states the same fail-before-mutating intent).
	var expiresAt int64
	switch {
	case tokenResp.ExpiresIn == nil:
		// A refresh response may legally omit expires_in. Leaving the OLD
		// (already-passed) ExpiresAt would mark the fresh token expired and
		// force a refresh on EVERY subsequent call — clear to 0, the
		// no-known-expiry state AccessToken never force-refreshes.
		expiresAt = 0
	case *tokenResp.ExpiresIn <= 0:
		// An EXPLICIT zero/negative lifetime is a malformed response, not an
		// omission (SPEC §16's positive-lifetime rule): treating it as
		// no-expiry would persist an already-expired token that never
		// refreshes again. Fail without persisting anything.
		return ErrAPI(resp.StatusCode, "token refresh response expires_in must be positive when present")
	case *tokenResp.ExpiresIn > maxRefreshTokenLifetimeSeconds:
		// An absurd lifetime (math.MaxInt64 above all) would overflow the
		// Unix-time addition below into a NEGATIVE ExpiresAt — which the
		// no-known-expiry rule then treats as never-expiring, so an
		// effectively-expired token would never refresh again. The shared
		// token-lifetime ceiling (SPEC §16) makes it a malformed response.
		return ErrAPI(resp.StatusCode, fmt.Sprintf("token refresh response expires_in must be no greater than %d seconds", maxRefreshTokenLifetimeSeconds))
	default:
		expiresAt = time.Now().Unix() + *tokenResp.ExpiresIn
	}
	// An omitted (or null) resource preserves the stored binding
	// (carry-forward, like an omitted rotated refresh_token); a present one
	// replaces it. A present-but-EMPTY resource is a malformed response
	// (SPEC §16: present ⇒ non-empty) — fail the refresh rather than
	// persisting rotated credentials under a stale binding.
	if tokenResp.Resource != nil && *tokenResp.Resource == "" {
		return ErrAPI(resp.StatusCode, "token refresh response resource must be a non-empty string when present")
	}

	creds.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		creds.RefreshToken = tokenResp.RefreshToken
	}
	creds.ExpiresAt = expiresAt
	if tokenResp.Resource != nil {
		creds.Resource = *tokenResp.Resource
	}

	return m.store.Save(origin, creds)
}

// Logout removes stored credentials.
func (m *AuthManager) Logout() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	origin := NormalizeBaseURL(m.cfg.BaseURL)
	return m.store.Delete(origin)
}

// GetUserID returns the stored user ID.
func (m *AuthManager) GetUserID() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	origin := NormalizeBaseURL(m.cfg.BaseURL)
	creds, err := m.store.Load(origin)
	if err != nil {
		return ""
	}
	return creds.UserID
}

// SetUserID stores the user ID.
func (m *AuthManager) SetUserID(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	origin := NormalizeBaseURL(m.cfg.BaseURL)
	creds, err := m.store.Load(origin)
	if err != nil {
		return err
	}
	creds.UserID = userID
	return m.store.Save(origin, creds)
}

// Store returns the credential store.
func (m *AuthManager) Store() *CredentialStore {
	return m.store
}
