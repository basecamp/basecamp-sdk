package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	surfguard "github.com/basecamp/surfguard/go"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// Exchanger handles OAuth 2.0 token exchange and refresh operations.
//
// Both post credentials — the authorization code and client secret, or the
// refresh token — to a token endpoint the caller names in the request, which
// may be one that DiscoverFromResource's metadata chose. By default the
// Exchanger therefore carries those POSTs on a client that judges the
// endpoint's literal address at dial time against [DefaultIssuerPolicy]; see
// [NewExchanger] for the overrides. It never follows a redirect from the
// token endpoint — a 301, 302, 303, 307 or 308 surfaces as a typed
// api_error carrying that status, and any other 3xx as the generic non-200
// failure — and bounds each request at [WithExchangerTimeout]'s deadline
// (30 s by default).
type Exchanger struct {
	httpClient *http.Client
	timeout    time.Duration
}

// ExchangerOption configures an Exchanger at construction.
type ExchangerOption func(*exchangerConfig)

type exchangerConfig struct {
	policy    surfguard.Policy
	policySet bool
	timeout   time.Duration
}

// WithExchangerPolicy replaces [DefaultIssuerPolicy] for the token endpoint
// POSTs, so a deployment whose authorization server is not on the public
// internet re-admits exactly the space it needs rather than switching the
// policy off. The derivations and the precedence trap are the ones documented
// on [WithIssuerPolicy]: AllowLoopback for a local server,
// surfguard.Policy{}.AllowAllPorts().Allow(...) for private space.
//
// It has no effect when NewExchanger is given a non-nil client, which supplies
// the transport the policy would otherwise be installed in.
func WithExchangerPolicy(p surfguard.Policy) ExchangerOption {
	return func(c *exchangerConfig) { c.policy, c.policySet = p, true }
}

// WithExchangerTimeout bounds each token-endpoint request at d instead of the
// 30-second default — the same per-request budget as the device flow's
// WithDeviceTimeout, with the same shared 3600 s ceiling and the same
// normalize-at-entry rule: a non-positive or beyond-ceiling value falls back
// to the default. The bound is a child context deadline, not a client
// mutation, so it holds on an injected client's requests too; a caller
// context with a sooner deadline still wins.
func WithExchangerTimeout(d time.Duration) ExchangerOption {
	return func(c *exchangerConfig) { c.timeout = d }
}

// NewExchanger creates an Exchanger.
//
// A nil httpClient selects the policy-enforced default: the shared
// [DefaultIssuerPolicy] client, or a client built from [WithExchangerPolicy]
// when that option is given. A non-nil httpClient is the caller's, enforcement
// included — no address policy is applied on top of it, and
// WithExchangerPolicy is ignored. A consumer that must egress through a proxy
// has no other way to keep both, since surfguard's transport sets Proxy: nil
// by construction; compose the client's transport from
// DefaultIssuerPolicy().RoundTripper() where that is possible. Passing
// http.DefaultClient switches off the address policy outright — but not the
// redirect refusal or the request timeout, which no client choice disables.
//
// Redirect suppression is not the address policy and rides every lane: the
// token endpoint's redirects are refused on an injected client too, via a
// per-request shallow copy that never mutates the caller's client — the same
// contract as the device flow's POSTs.
//
// An Exchanger given WithExchangerPolicy owns a transport, and has no Close, so
// build it once and reuse it rather than constructing one per exchange.
func NewExchanger(httpClient *http.Client, opts ...ExchangerOption) *Exchanger {
	cfg := exchangerConfig{timeout: defaultTokenRequestTimeout}
	for _, o := range opts {
		o(&cfg)
	}
	// Non-positive AND oversized values both fall back to the default: the
	// same normalize-at-entry discipline as newDeviceConfig.
	if cfg.timeout <= 0 || cfg.timeout > maxDeviceRequestTimeout {
		cfg.timeout = defaultTokenRequestTimeout
	}
	switch {
	case httpClient != nil:
	case cfg.policySet:
		httpClient = newPolicyClient(cfg.policy)
	default:
		httpClient = sharedPolicyClient()
	}
	return &Exchanger{httpClient: httpClient, timeout: cfg.timeout}
}

// Exchange exchanges an authorization code for access and refresh tokens.
func (e *Exchanger) Exchange(ctx context.Context, req ExchangeRequest) (*Token, error) {
	if req.TokenEndpoint == "" {
		return nil, fmt.Errorf("token endpoint is required")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("authorization code is required")
	}
	if req.RedirectURI == "" {
		return nil, fmt.Errorf("redirect URI is required")
	}
	if req.ClientID == "" {
		return nil, fmt.Errorf("client ID is required")
	}

	data := url.Values{}
	if req.UseLegacyFormat {
		// Launchpad uses non-standard "type" parameter
		data.Set("type", "web_server")
	} else {
		// Standard OAuth 2.0
		data.Set("grant_type", "authorization_code")
	}
	data.Set("code", req.Code)
	data.Set("redirect_uri", req.RedirectURI)
	data.Set("client_id", req.ClientID)
	if req.ClientSecret != "" {
		data.Set("client_secret", req.ClientSecret)
	}
	if req.CodeVerifier != "" {
		data.Set("code_verifier", req.CodeVerifier)
	}

	return e.doTokenRequest(ctx, req.TokenEndpoint, data)
}

// Refresh exchanges a refresh token for a new access token.
func (e *Exchanger) Refresh(ctx context.Context, req RefreshRequest) (*Token, error) {
	if req.TokenEndpoint == "" {
		return nil, fmt.Errorf("token endpoint is required")
	}
	if req.RefreshToken == "" {
		return nil, fmt.Errorf("refresh token is required")
	}

	data := url.Values{}
	if req.UseLegacyFormat {
		// Launchpad uses non-standard "type" parameter
		data.Set("type", "refresh")
	} else {
		// Standard OAuth 2.0
		data.Set("grant_type", "refresh_token")
	}
	data.Set("refresh_token", req.RefreshToken)
	if req.ClientID != "" {
		data.Set("client_id", req.ClientID)
	}
	if req.ClientSecret != "" {
		data.Set("client_secret", req.ClientSecret)
	}
	if req.Resource != "" {
		data.Set("resource", req.Resource)
	}

	return e.doTokenRequest(ctx, req.TokenEndpoint, data)
}

// maxTokenResponseBytes is the maximum size for token endpoint response bodies (1 MB).
const maxTokenResponseBytes int64 = 1 * 1024 * 1024

// maxErrorMessageLen is the maximum length for error messages included in errors.
const maxErrorMessageLen = 500

// defaultTokenRequestTimeout bounds each token exchange/refresh round-trip —
// the 30 s every other credential POST already converged on (the device flow
// here, TS/Python/Ruby's exchange). The ceiling is the device flow's shared
// maxDeviceRequestTimeout.
const defaultTokenRequestTimeout = 30 * time.Second

// isRedirectStatus reports whether status is one of the redirects the token
// endpoint refuses to follow (SPEC §16 "Token-Endpoint Transport Policy" —
// the same set as SPEC §14's download hop). 304 is not among them and stays
// on the generic non-200 path. The basecamp package keeps its own unexported
// copy for the download flow.
func isRedirectStatus(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func (e *Exchanger) doTokenRequest(ctx context.Context, tokenEndpoint string, data url.Values) (*Token, error) {
	// Validate HTTPS to prevent sending tokens/credentials over plaintext
	// Allow localhost for testing against local mock OAuth servers
	if err := basecamp.RequireSecureEndpoint(tokenEndpoint); err != nil {
		return nil, fmt.Errorf("token endpoint validation failed for %q: %w", tokenEndpoint, err)
	}

	// A child deadline, not a client mutation, so it bounds the request on the
	// injected-client lane too; a caller context with a sooner deadline wins.
	reqCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")

	// The suppression is about gosec's taint rule, not a claim that this URL is
	// trusted. TokenEndpoint may be caller-configured, but it may equally come
	// from DiscoverFromResource's metadata, in which case a remote peer chose it
	// — so by default the client judges the address it resolves to at dial
	// time (DefaultIssuerPolicy, #806). This request carries the authorization
	// code, the client secret, or a refresh token, so it is the highest-value
	// of the three such call sites.
	//
	// noRedirectClient rides every lane, injected clients included: redirect
	// suppression is a transport invariant (SPEC §16 "Token-Endpoint Transport
	// Policy"), not part of the address policy a caller's client opts out of.
	// The shallow copy keeps the caller's (and the shared policy) client
	// unmutated.
	resp, err := noRedirectClient(e.httpClient).Do(httpReq) // #nosec G704 -- see the note above: address-policed by default
	if err != nil {
		// A policy refusal is a typed, permanent verdict on the endpoint; every
		// other failure keeps the untyped wrap callers already match on.
		if errors.Is(err, surfguard.ErrBlocked) {
			return nil, blockedEndpointError("token endpoint", tokenEndpoint, err)
		}
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// A refused redirect is a typed api fault classified by status BEFORE the
	// body read: a 3xx that streams its body slowly (or never) must surface as
	// this error now, not as a mid-read timeout. Every other non-200 keeps the
	// body-informed handling below.
	if isRedirectStatus(resp.StatusCode) {
		return nil, basecamp.ErrAPI(resp.StatusCode, fmt.Sprintf("redirect %d on the token endpoint is not followed", resp.StatusCode))
	}

	// Bounded read to prevent OOM from malicious/corrupted responses
	lr := io.LimitReader(resp.Body, maxTokenResponseBytes+1)
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}
	if int64(len(body)) > maxTokenResponseBytes {
		return nil, fmt.Errorf("token response body exceeds %d byte limit", maxTokenResponseBytes)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			desc := errResp.ErrorDescription
			if len(desc) > maxErrorMessageLen {
				desc = desc[:maxErrorMessageLen-3] + "..."
			}
			if desc != "" {
				return nil, fmt.Errorf("token error: %s - %s", errResp.Error, desc)
			}
			return nil, fmt.Errorf("token error: %s", errResp.Error)
		}
		bodyStr := string(body)
		if len(bodyStr) > maxErrorMessageLen {
			bodyStr = bodyStr[:maxErrorMessageLen-3] + "..."
		}
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, bodyStr)
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		// A malformed 200 body — including a non-string resource failing the
		// string decode — is a typed api fault (SPEC §16) so callers can
		// classify it via errors.As(*basecamp.Error) with the HTTP status.
		return nil, basecamp.ErrAPI(resp.StatusCode, fmt.Sprintf("parsing token response: %v", err))
	}
	// A 2xx without a usable access_token is malformed, not a success — the
	// device-flow and AuthManager paths already enforce this.
	if token.AccessToken == "" {
		return nil, basecamp.ErrAPI(resp.StatusCode, "token response missing access_token")
	}

	// resource re-decodes through a *string because Token's plain string field
	// cannot distinguish an absent field from an explicit "": absent and JSON
	// null map to unset (nil), while a present-but-empty resource is a
	// malformed response (SPEC §16) — an empty binding is not a binding. A
	// non-string resource already failed the Token unmarshal above.
	var rawResource struct {
		Resource  *string `json:"resource"`
		TokenType *string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &rawResource); err != nil {
		return nil, basecamp.ErrAPI(resp.StatusCode, fmt.Sprintf("parsing token response: %v", err))
	}
	if rawResource.Resource != nil && *rawResource.Resource == "" {
		// A typed api fault (SPEC §16), not a bare error: callers classify
		// malformed server responses via errors.As(*basecamp.Error) and need
		// the HTTP status — matching the device-token and AuthManager paths.
		return nil, basecamp.ErrAPI(resp.StatusCode, "token response resource must be a non-empty string when present")
	}
	// token_type: absent/JSON-null defaults to Bearer; a present-but-empty
	// value is malformed (SPEC §16) — matching the device-flow parser.
	if rawResource.TokenType != nil && *rawResource.TokenType == "" {
		return nil, basecamp.ErrAPI(resp.StatusCode, "token response token_type must be a non-empty string when present")
	}
	if rawResource.TokenType == nil {
		token.TokenType = "Bearer"
	}

	// Calculate ExpiresAt from ExpiresIn
	if token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	return &token, nil
}
