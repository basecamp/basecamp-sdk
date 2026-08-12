package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FlexTime is a time.Time that can unmarshal from either a Unix timestamp (integer)
// or an RFC 3339 string. This supports both BC3 OAuth 2.1 (integer) and Launchpad (string).
//
// The zero value means "no expiry known": an absent field, an explicit null,
// and the integer 0 all decode to it, and it marshals back as null. Check
// IsZero() before treating the instant as real.
type FlexTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler for FlexTime.
func (ft *FlexTime) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		ft.Time = time.Time{}
		return nil
	}

	// Try as integer (Unix timestamp) first
	var unix int64
	if err := json.Unmarshal(data, &unix); err == nil {
		if unix == 0 {
			// bc3 renders `expires_at.to_i`, so a wire 0 would be its spelling
			// of an unstated expiry, and RFC 7591 gives 0 the meaning "never
			// expires" (bc3's own client_secret_expires_at). Either way,
			// "expired at the 1970 epoch" — a *valid* time.Unix(0, 0) instant —
			// is the one wrong reading. Treat it as "no expiry known",
			// matching TypeScript's parseExpiresAt.
			ft.Time = time.Time{}
			return nil
		}
		ft.Time = time.Unix(unix, 0)
		return nil
	}

	// Try as string (RFC 3339)
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		t, err := time.Parse(time.RFC3339, str)
		if err != nil {
			return fmt.Errorf("invalid time string %q: %w", str, err)
		}
		ft.Time = t
		return nil
	}

	return fmt.Errorf("expires_at must be a Unix timestamp or RFC 3339 string, got: %s", string(data))
}

// MarshalJSON implements json.Marshaler for FlexTime.
// Zero times marshal as null; non-zero times use time.Time's JSON encoding.
// Without this, a zero ExpiresAt would re-marshal as the fabricated instant
// 0001-01-01T00:00:00Z, indistinguishable from a timestamp the server sent.
func (ft FlexTime) MarshalJSON() ([]byte, error) {
	if ft.IsZero() {
		return []byte("null"), nil
	}
	return ft.Time.MarshalJSON()
}

// Identity represents the authenticated user's identity from the authorization endpoint.
//
// Only ID is emitted by both issuers. A BC5 issuer's own document carries nothing
// but the identity id — it drops the PII the API docs already say not to use for
// identifying users — so FirstName, LastName and EmailAddress are "" there.
type Identity struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	EmailAddress string `json:"email_address"`
}

// AuthorizedAccount represents a Basecamp account the user has access to.
//
// Product and AppHREF are Launchpad's and are "" against a BC5 issuer; Resource
// is a BC5 issuer's and is "" against Launchpad.
type AuthorizedAccount struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Product string `json:"product"`
	HREF    string `json:"href"`
	AppHREF string `json:"app_href"`
	// Resource is the RFC 8707 resource indicator for this account
	// (urn:bc:account:<id>), emitted by BC5 issuers only. Pass it as the
	// "resource" parameter when requesting a token scoped to this account.
	Resource string `json:"resource,omitempty"`
	Hidden   bool   `json:"hidden,omitempty"`
	Expired  bool   `json:"expired,omitempty"`
	Featured bool   `json:"featured,omitempty"`
}

// AuthorizationInfo contains the complete authorization response.
type AuthorizationInfo struct {
	// ExpiresAt is the token's expiry. Its zero value means the document did
	// not state one; prefer Expiry(), which makes that case explicit.
	ExpiresAt FlexTime            `json:"expires_at"`
	Identity  Identity            `json:"identity"`
	Accounts  []AuthorizedAccount `json:"accounts"`
	// Scope is the token's granted scope. BC5 issuers only, and only for
	// BC3-issued tokens — legacy Signal tokens predate scopes, so an empty
	// Scope is not an error.
	Scope string `json:"scope,omitempty"`

	// ProductFilterApplied reports whether a requested GetInfoOptions.FilterProduct
	// was actually applied. It is meaningful only when FilterProduct was set: false
	// then means the document carried at least one account and no product on any of
	// them — a BC5 document — so the filter was inapplicable and Accounts is
	// unfiltered rather than empty. An empty account list reports true: it is no
	// evidence about the issuer either way. Not a wire field.
	ProductFilterApplied bool `json:"-"`
}

// Expiry returns the token's expiry instant. ok is false when the
// authorization document did not state one — the zero ExpiresAt covers an
// absent field, an explicit null, and a wire `0` alike. No production
// issuer emits any of those today; the branch is robustness for the
// endpoints GetInfoOptions.Endpoint can reach.
func (a *AuthorizationInfo) Expiry() (t time.Time, ok bool) {
	return a.ExpiresAt.Time, !a.ExpiresAt.IsZero()
}

// GetInfoOptions specifies options for fetching authorization info.
type GetInfoOptions struct {
	// Endpoint overrides the default authorization endpoint URL.
	// If empty, defaults to "https://launchpad.37signals.com/authorization.json".
	Endpoint string

	// FilterProduct filters accounts to only those matching this product.
	// Common values: "bc3" (Basecamp), "bcx" (Basecamp 2), "hey" (HEY).
	// If empty, all accounts are returned.
	//
	// A BC5 issuer's document carries no product on any account, so the filter
	// cannot apply there. In that case all accounts are returned and
	// AuthorizationInfo.ProductFilterApplied is false, rather than the empty
	// list a literal filter would produce.
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
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	endpoint := "https://launchpad.37signals.com/authorization.json"
	if opts != nil && opts.Endpoint != "" {
		endpoint = opts.Endpoint
		// Validate custom endpoint uses HTTPS (allow localhost for testing)
		if err := RequireSecureEndpoint(endpoint); err != nil {
			return nil, fmt.Errorf("authorization endpoint validation failed: %w", err)
		}
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
	resp, err := s.client.httpClient.Do(req) // #nosec G704 -- SDK HTTP client: URL is caller-configured
	if err != nil {
		return nil, ErrNetwork(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := limitedReadAll(resp.Body, MaxErrorBodyBytes)
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrAuth("Authorization failed: invalid or expired token")
		}
		return nil, ErrAPI(resp.StatusCode, fmt.Sprintf("authorization request failed: %s", truncateString(string(body), MaxErrorMessageBytes)))
	}

	body, err := limitedReadAll(resp.Body, MaxResponseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("reading authorization response: %w", err)
	}

	var info AuthorizationInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing authorization response: %w", err)
	}

	// Filter accounts by product if requested. A document whose accounts carry no
	// product at all cannot be filtered by product: matching nothing would empty a
	// list the caller is about to pick an HREF out of, which is silently wrong
	// rather than merely unhelpful. Report the filter inapplicable instead, and
	// leave the accounts alone.
	//
	// An EMPTY account list is filterable, not inapplicable. "No account carries a
	// product" is vacuously true of an empty slice, but Launchpad returns an empty
	// list for an identity with no currently accessible accounts, and that document
	// filters fine. The returned list is empty either way; what would differ is the
	// claim — reporting the filter inapplicable asserts "this issuer cannot filter
	// by product" on no evidence.
	if opts != nil && opts.FilterProduct != "" {
		filterable := len(info.Accounts) == 0
		for _, acct := range info.Accounts {
			if acct.Product != "" {
				filterable = true
				break
			}
		}
		info.ProductFilterApplied = filterable
		if filterable {
			filtered := make([]AuthorizedAccount, 0, len(info.Accounts))
			for _, acct := range info.Accounts {
				if acct.Product == opts.FilterProduct {
					filtered = append(filtered, acct)
				}
			}
			info.Accounts = filtered
		}
	}

	return &info, nil
}
