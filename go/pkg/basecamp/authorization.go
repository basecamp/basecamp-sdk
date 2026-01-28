package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Identity represents the authenticated user's identity from the authorization endpoint.
type Identity struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	EmailAddress string `json:"email_address"`
}

// AuthorizedAccount represents a Basecamp account the user has access to.
type AuthorizedAccount struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Product  string `json:"product"`
	HREF     string `json:"href"`
	AppHREF  string `json:"app_href"`
	Hidden   bool   `json:"hidden,omitempty"`
	Expired  bool   `json:"expired,omitempty"`
	Featured bool   `json:"featured,omitempty"`
}

// AuthorizationInfo contains the complete authorization response.
type AuthorizationInfo struct {
	ExpiresAt time.Time           `json:"expires_at"`
	Identity  Identity            `json:"identity"`
	Accounts  []AuthorizedAccount `json:"accounts"`
}

// GetInfoOptions specifies options for fetching authorization info.
type GetInfoOptions struct {
	// Endpoint overrides the default authorization endpoint URL.
	// If empty, defaults to "https://launchpad.37signals.com/authorization.json".
	Endpoint string

	// FilterProduct filters accounts to only those matching this product.
	// Common values: "bc3" (Basecamp 4), "bcx" (Basecamp 2), "hey" (HEY).
	// If empty, all accounts are returned.
	FilterProduct string
}

// AuthorizationService handles authorization operations.
type AuthorizationService struct {
	client *Client
}

// NewAuthorizationService creates a new AuthorizationService.
func NewAuthorizationService(client *Client) *AuthorizationService {
	return &AuthorizationService{client: client}
}

// GetInfo fetches authorization information for the current access token.
// This includes the user's identity and list of authorized accounts.
func (s *AuthorizationService) GetInfo(ctx context.Context, opts *GetInfoOptions) (result *AuthorizationInfo, err error) {
	op := OperationInfo{
		Service: "Authorization", Operation: "GetInfo",
		ResourceType: "authorization", IsMutation: false,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	endpoint := "https://launchpad.37signals.com/authorization.json"
	if opts != nil && opts.Endpoint != "" {
		endpoint = opts.Endpoint
	}

	// Get access token
	token, err := s.client.tokenProvider.AccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating authorization request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", s.client.userAgent)
	req.Header.Set("Accept", "application/json")

	// Execute request using the client's HTTP client
	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return nil, ErrNetwork(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrAuth("Authorization failed: invalid or expired token")
		}
		return nil, ErrAPI(resp.StatusCode, fmt.Sprintf("authorization request failed: %s", string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading authorization response: %w", err)
	}

	var info AuthorizationInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing authorization response: %w", err)
	}

	// Filter accounts by product if requested
	if opts != nil && opts.FilterProduct != "" {
		filtered := make([]AuthorizedAccount, 0, len(info.Accounts))
		for _, acct := range info.Accounts {
			if acct.Product == opts.FilterProduct {
				filtered = append(filtered, acct)
			}
		}
		info.Accounts = filtered
	}

	return &info, nil
}
