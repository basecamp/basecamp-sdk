package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

const serviceName = "basecamp-sdk"

// Credentials holds OAuth tokens and metadata.
type Credentials struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	ExpiresAt     int64  `json:"expires_at"`
	Scope         string `json:"scope"`
	TokenEndpoint string `json:"token_endpoint"`
	UserID        string `json:"user_id,omitempty"`
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
		keyring.Delete(serviceName, testKey)
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

	// Atomic write
	tmpPath := s.credentialsPath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.credentialsPath())
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

	// Check if token is expired (with 5 minute buffer)
	if time.Now().Unix() >= creds.ExpiresAt-300 {
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

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", creds.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return ErrNetwork(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ErrAPI(resp.StatusCode, fmt.Sprintf("token refresh failed: %s", string(body)))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	creds.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		creds.RefreshToken = tokenResp.RefreshToken
	}
	if tokenResp.ExpiresIn > 0 {
		creds.ExpiresAt = time.Now().Unix() + tokenResp.ExpiresIn
	}

	return m.store.Save(origin, creds)
}

// Logout removes stored credentials.
func (m *AuthManager) Logout() error {
	origin := NormalizeBaseURL(m.cfg.BaseURL)
	return m.store.Delete(origin)
}

// GetUserID returns the stored user ID.
func (m *AuthManager) GetUserID() string {
	origin := NormalizeBaseURL(m.cfg.BaseURL)
	creds, err := m.store.Load(origin)
	if err != nil {
		return ""
	}
	return creds.UserID
}

// SetUserID stores the user ID.
func (m *AuthManager) SetUserID(userID string) error {
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
