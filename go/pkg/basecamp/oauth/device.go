package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// DeviceCodeGrantType is the RFC 8628 URN grant type for the device
// authorization grant.
const DeviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// Device-flow polling defaults (RFC 8628 §3.2/§3.5).
const (
	// defaultDeviceInterval is the polling interval used when the server omits
	// interval (RFC 8628 §3.2).
	defaultDeviceInterval = 5
	// slowDownIncrementSeconds is the sustained interval bump applied on a
	// slow_down response (RFC 8628 §3.5).
	slowDownIncrementSeconds = 5
	// maxBackoffSeconds caps exponential backoff after connection timeouts.
	maxBackoffSeconds = 60
	// defaultDeviceRequestTimeout bounds each individual HTTP round-trip.
	defaultDeviceRequestTimeout = 30 * time.Second
	// maxDeviceSeconds caps expires_in/interval at 2147483 s (~24.8 days) — the
	// largest whole-second duration whose millisecond form fits a 32-bit signed
	// timer, shared across all five SDKs (SPEC.md). Far above any legitimate
	// device-code lifetime, and small enough that the float→int conversion and
	// time.Duration multiplication downstream can never overflow (an unbounded
	// value like 1e100 converts to int implementation-defined).
	maxDeviceSeconds = 2_147_483
	// maxTokenLifetimeSeconds caps an OAuth token's expires_in at 2147483647 s
	// (~68 years) — cross-runtime safe and vastly beyond any realistic token
	// lifetime. Unlike maxDeviceSeconds this bounds ExpiresAt arithmetic rather
	// than a timer: a large finite value (e.g. math.MaxInt64) would overflow
	// time.Duration(ExpiresIn) * time.Second and yield a garbage deadline, so a
	// value past this ceiling is a malformed response. Shared across all five SDKs.
	maxTokenLifetimeSeconds = 2_147_483_647
)

// DeviceAuthorization is an RFC 8628 §3.2 device authorization response.
type DeviceAuthorization struct {
	// DeviceCode is the device verification code (polled at the token endpoint).
	DeviceCode string
	// UserCode is the end-user code shown at the verification URI.
	UserCode string
	// VerificationURI is where the user enters the user code.
	VerificationURI string
	// VerificationURIComplete embeds the user code in the URI (optional).
	VerificationURIComplete string
	// ExpiresIn is the device/user code lifetime in seconds.
	ExpiresIn int
	// Interval is the minimum polling interval in seconds (default 5).
	Interval int
}

// deviceConfig holds the resolved options for a device-flow operation.
type deviceConfig struct {
	httpClient *http.Client
	scope      string
	hasScope   bool
	timeout    time.Duration
	clock      func() time.Time
	sleep      func(ctx context.Context, d time.Duration) error
}

// DeviceOption configures a device-flow operation.
type DeviceOption func(*deviceConfig)

// WithDeviceHTTPClient sets the HTTP client used for device-flow requests.
// Nil leaves http.DefaultClient.
func WithDeviceHTTPClient(c *http.Client) DeviceOption {
	return func(cfg *deviceConfig) {
		if c != nil {
			cfg.httpClient = c
		}
	}
}

// WithDeviceScope sets the requested scope. When omitted, scope is left out of
// the request entirely so the server applies its default (`read`).
func WithDeviceScope(scope string) DeviceOption {
	return func(cfg *deviceConfig) {
		cfg.scope = scope
		cfg.hasScope = true
	}
}

// WithDeviceTimeout bounds each individual HTTP round-trip. Zero or negative
// leaves the default (30s).
func WithDeviceTimeout(d time.Duration) DeviceOption {
	return func(cfg *deviceConfig) { cfg.timeout = d }
}

// WithDeviceClock injects a monotonic clock for the polling deadline. Defaults
// to time.Now (monotonic in Go). Tests inject a clock to advance time.
func WithDeviceClock(clock func() time.Time) DeviceOption {
	return func(cfg *deviceConfig) {
		if clock != nil {
			cfg.clock = clock
		}
	}
}

// WithDeviceSleep injects the wait seam between polls. It receives the poll
// context and the requested wait; returning a non-nil error (e.g. ctx.Err())
// ends the wait. Tests inject a sleep that records the schedule and returns
// immediately so there are no real delays.
func WithDeviceSleep(sleep func(ctx context.Context, d time.Duration) error) DeviceOption {
	return func(cfg *deviceConfig) {
		if sleep != nil {
			cfg.sleep = sleep
		}
	}
}

func newDeviceConfig(opts []DeviceOption) deviceConfig {
	cfg := deviceConfig{
		httpClient: http.DefaultClient,
		timeout:    defaultDeviceRequestTimeout,
		clock:      time.Now,
		sleep:      defaultDeviceSleep,
	}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.timeout <= 0 {
		cfg.timeout = defaultDeviceRequestTimeout
	}
	// Suppress redirects on every device-flow POST so a 3xx surfaces as a non-2xx
	// api_error rather than the client chasing an attacker-influenced Location.
	cfg.httpClient = suppressRedirects(cfg.httpClient)
	return cfg
}

// suppressRedirects returns a shallow copy of c that never follows a redirect.
// A 3xx response is returned as-is (via http.ErrUseLastResponse) so the caller
// classifies it as a non-success instead of dialing the redirect target.
func suppressRedirects(c *http.Client) *http.Client {
	clone := *c
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}

// defaultDeviceSleep waits d, returning early with ctx.Err() when the context
// is cancelled or its deadline passes.
func defaultDeviceSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// rawDeviceAuthorization mirrors an RFC 8628 §3.2 response. Numeric fields are
// pointers so absent is distinguishable from a present zero, and *float64 (not
// *int) so an integer-valued float like 900.0 decodes — encoding/json rejects a
// fractional-looking number into an int, but the cross-SDK contract accepts
// whole-second floats (900.0) and rejects fractional (2.5); whole-second
// enforcement happens in validation.
type rawDeviceAuthorization struct {
	DeviceCode              string   `json:"device_code"`
	UserCode                string   `json:"user_code"`
	VerificationURI         string   `json:"verification_uri"`
	VerificationURIComplete string   `json:"verification_uri_complete"`
	ExpiresIn               *float64 `json:"expires_in"`
	Interval                *float64 `json:"interval"`
}

// wholeSeconds coerces an RFC 8628 duration field to a positive integer number
// of seconds no greater than maxDeviceSeconds. It accepts a positive
// integer-valued float (900 or 900.0) and rejects absent, non-positive,
// fractional (2.5), or oversized (1e100) values — matching TS, Ruby, Python,
// and Kotlin.
func wholeSeconds(v *float64) (int, bool) {
	if v == nil || *v <= 0 || *v > maxDeviceSeconds || *v != math.Trunc(*v) {
		return 0, false
	}
	return int(*v), true
}

// RequestDeviceAuthorization obtains a device/user code pair (RFC 8628
// §3.1–3.2). The endpoint is TLS-guarded. scope is sent only when set via
// WithDeviceScope; otherwise it is omitted so the server applies its default
// (`read`). A network failure yields a DeviceFlowError(transport); a non-2xx,
// unparsable, or invalid response yields a coded *basecamp.Error.
func RequestDeviceAuthorization(ctx context.Context, deviceAuthEndpoint, clientID string, opts ...DeviceOption) (*DeviceAuthorization, error) {
	cfg := newDeviceConfig(opts)

	if err := basecamp.RequireSecureEndpoint(deviceAuthEndpoint); err != nil {
		return nil, &basecamp.Error{
			Code:    basecamp.CodeUsage,
			Message: fmt.Sprintf("device authorization endpoint is not secure: %s", deviceAuthEndpoint),
			Cause:   err,
		}
	}
	if clientID == "" {
		return nil, &basecamp.Error{Code: basecamp.CodeValidation, Message: "client ID is required for device authorization"}
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	// Omit scope entirely when unset so the server applies its default (`read`).
	if cfg.hasScope && cfg.scope != "" {
		form.Set("scope", cfg.scope)
	}

	reqCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, deviceAuthEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &basecamp.Error{Code: basecamp.CodeUsage, Message: fmt.Sprintf("creating device authorization request: %v", err), Cause: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := cfg.httpClient.Do(req) // #nosec G704 -- SDK HTTP client: caller-supplied discovery endpoint
	if err != nil {
		return nil, &DeviceFlowError{Reason: DeviceFlowTransport, Err: fmt.Errorf("device authorization request failed: %w", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBoundedBody(resp.Body, maxTokenResponseBytes)
	if err != nil {
		// An oversized body is a server/api fault (api_error, not retryable); any
		// other read failure is a transport failure, matching the Do() error above.
		if errors.Is(err, errBodyTooLarge) {
			return nil, &basecamp.Error{Code: basecamp.CodeAPI, Message: fmt.Sprintf("device authorization response too large: %v", err), Cause: err}
		}
		return nil, &DeviceFlowError{Reason: DeviceFlowTransport, Err: fmt.Errorf("reading device authorization response: %w", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, basecamp.ErrAPI(resp.StatusCode,
			fmt.Sprintf("device authorization failed with status %d: %s", resp.StatusCode, truncateBody(body)))
	}

	var raw rawDeviceAuthorization
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &basecamp.Error{Code: basecamp.CodeAPI, Message: "failed to parse device authorization response", Cause: err}
	}
	return validateDeviceAuthorization(raw)
}

func validateDeviceAuthorization(raw rawDeviceAuthorization) (*DeviceAuthorization, error) {
	apiErr := func(msg string) error { return &basecamp.Error{Code: basecamp.CodeAPI, Message: msg} }

	if raw.DeviceCode == "" || raw.UserCode == "" || raw.VerificationURI == "" {
		return nil, apiErr("invalid device authorization response: missing required fields")
	}
	expiresIn, ok := wholeSeconds(raw.ExpiresIn)
	if !ok {
		return nil, apiErr(fmt.Sprintf("invalid device authorization response: expires_in must be a positive integer no greater than %d", maxDeviceSeconds))
	}
	interval := defaultDeviceInterval
	if raw.Interval != nil {
		i, ok := wholeSeconds(raw.Interval)
		if !ok {
			return nil, apiErr(fmt.Sprintf("invalid device authorization response: interval must be a positive integer no greater than %d", maxDeviceSeconds))
		}
		interval = i
	}
	return &DeviceAuthorization{
		DeviceCode:              raw.DeviceCode,
		UserCode:                raw.UserCode,
		VerificationURI:         raw.VerificationURI,
		VerificationURIComplete: raw.VerificationURIComplete,
		ExpiresIn:               expiresIn,
		Interval:                interval,
	}, nil
}

// PollDeviceToken runs the RFC 8628 §3.4–3.5 polling loop against the token
// endpoint until the user approves, denies, or the code expires. It waits at
// least interval seconds between polls, enforces a monotonic expiry deadline via
// the injectable clock, sustains slow_down bumps (+5s), backs off exponentially
// on connection timeouts (resetting once a round-trip completes), and honors
// context cancellation.
//
// Terminal DeviceFlowError reasons: access_denied, expired, transport,
// cancelled. Other server errors surface as a coded *basecamp.Error.
func PollDeviceToken(ctx context.Context, tokenEndpoint, clientID, deviceCode string, interval, expiresIn int, opts ...DeviceOption) (*Token, error) {
	cfg := newDeviceConfig(opts)
	deadline := cfg.clock().Add(time.Duration(expiresIn) * time.Second)
	return pollDeviceTokenUntil(ctx, cfg, tokenEndpoint, clientID, deviceCode, interval, deadline)
}

// pollDeviceTokenUntil is the polling loop against an ABSOLUTE monotonic
// deadline. PerformDeviceLogin calls it with the exact issuance-anchored
// deadline so no whole-second rounding or re-anchoring at a later clock read
// can extend the code's lifetime.
func pollDeviceTokenUntil(ctx context.Context, cfg deviceConfig, tokenEndpoint, clientID, deviceCode string, interval int, deadline time.Time) (*Token, error) {
	if err := basecamp.RequireSecureEndpoint(tokenEndpoint); err != nil {
		return nil, &basecamp.Error{
			Code:    basecamp.CodeUsage,
			Message: fmt.Sprintf("token endpoint is not secure: %s", tokenEndpoint),
			Cause:   err,
		}
	}

	intervalSeconds := interval
	if intervalSeconds <= 0 {
		intervalSeconds = defaultDeviceInterval
	}
	backoffSeconds := intervalSeconds

	form := url.Values{}
	form.Set("grant_type", DeviceCodeGrantType)
	form.Set("device_code", deviceCode)
	form.Set("client_id", clientID)

	for {
		if err := ctx.Err(); err != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: err}
		}
		// Check-before-wait: if the monotonic deadline has already passed, the
		// codes are expired — return now rather than sleeping a negative
		// duration into the (possibly injected) sleep seam.
		remaining := deadline.Sub(cfg.clock())
		if remaining <= 0 {
			return nil, &DeviceFlowError{Reason: DeviceFlowExpired}
		}
		// Each wait is the server-driven interval or the transient timeout
		// backoff, whichever is larger, clamped to the time left before the
		// deadline so a long backoff never overshoots expiry; the deadline
		// check below then terminates the flow promptly.
		wait := time.Duration(max(intervalSeconds, backoffSeconds)) * time.Second
		if remaining < wait {
			wait = remaining
		}
		if err := cfg.sleep(ctx, wait); err != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: cancelCause(ctx, err)}
		}
		if err := ctx.Err(); err != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: err}
		}
		if !cfg.clock().Before(deadline) {
			return nil, &DeviceFlowError{Reason: DeviceFlowExpired}
		}

		result := postDeviceToken(ctx, cfg, tokenEndpoint, form)
		switch result.kind {
		case pollToken:
			return result.token, nil
		case pollCancelled:
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: result.err}
		case pollTimeout:
			// Connection timeout — back off and keep polling (RFC 8628 §3.5).
			// The server-driven interval is untouched so the backoff decays
			// fully once a round-trip completes.
			backoffSeconds = min(backoffSeconds*2, maxBackoffSeconds)
			continue
		case pollTransport:
			return nil, &DeviceFlowError{Reason: DeviceFlowTransport, Err: result.err}
		case pollInvalidResponse:
			// Malformed 2xx token response — api_error, not a retryable transport.
			return nil, basecamp.ErrAPI(result.status, result.err.Error())
		case pollOAuthError:
			// Any completed round-trip resets the timeout backoff to the
			// server-driven interval.
			backoffSeconds = intervalSeconds
			switch result.oauthError {
			case "authorization_pending":
				continue
			case "slow_down":
				intervalSeconds += slowDownIncrementSeconds
				continue
			case "access_denied":
				return nil, &DeviceFlowError{Reason: DeviceFlowAccessDenied}
			case "expired_token":
				return nil, &DeviceFlowError{Reason: DeviceFlowExpired}
			default:
				return nil, basecamp.ErrAPI(result.status,
					fmt.Sprintf("device token request failed: %s", result.oauthError))
			}
		default:
			return nil, basecamp.ErrAPI(result.status, "device token request failed")
		}
	}
}

// pollResultKind classifies a single token-endpoint poll.
type pollResultKind int

const (
	pollToken pollResultKind = iota
	pollOAuthError
	pollTimeout
	pollTransport
	pollCancelled
	// pollInvalidResponse is a server/api fault (api_error), NOT a retryable
	// transport: a 2xx whose body is unparseable or missing the access token, a
	// 3xx (redirects are suppressed, never a valid token response), or any
	// response whose body exceeds the size cap.
	pollInvalidResponse
)

type pollResult struct {
	kind       pollResultKind
	token      *Token
	oauthError string
	status     int
	err        error
}

// postDeviceToken performs one token-endpoint poll and classifies the outcome.
// A parent-context cancellation is pollCancelled; a per-request timeout (or any
// net timeout) is pollTimeout (→ backoff); any other transport failure is
// pollTransport. A 2xx yields a Token; a 3xx is pollInvalidResponse; any other
// non-2xx with an OAuth error body yields pollOAuthError.
func postDeviceToken(ctx context.Context, cfg deviceConfig, tokenEndpoint string, form url.Values) pollResult {
	reqCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return pollResult{kind: pollTransport, err: fmt.Errorf("creating device token request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := cfg.httpClient.Do(req) // #nosec G704 -- SDK HTTP client: caller-supplied token endpoint
	if err != nil {
		// Parent cancellation ends the flow; a per-request timeout backs off.
		if ctx.Err() != nil {
			return pollResult{kind: pollCancelled, err: ctx.Err()}
		}
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return pollResult{kind: pollTimeout, err: err}
		}
		return pollResult{kind: pollTransport, err: fmt.Errorf("device token poll failed: %w", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBoundedBody(resp.Body, maxTokenResponseBytes)
	if err != nil {
		switch {
		case errors.Is(err, errBodyTooLarge):
			// Oversized body — a server/api fault (api_error, not retryable), NOT a
			// retryable transport failure.
			return pollResult{kind: pollInvalidResponse, status: resp.StatusCode, err: fmt.Errorf("reading device token response: %w", err)}
		case ctx.Err() != nil:
			return pollResult{kind: pollCancelled, err: ctx.Err()}
		case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
			return pollResult{kind: pollTimeout, err: err}
		default:
			return pollResult{kind: pollTransport, err: fmt.Errorf("reading device token response: %w", err)}
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// expires_in decodes via *float64, not Token's plain int: a pointer keeps
		// an explicit "expires_in":0 distinguishable from an omitted field (a
		// plain int makes 0 look absent and skip validation), and float64 accepts
		// an integer-valued 3600.0 per the cross-SDK contract. Whole-second
		// enforcement happens below. token_type is *string for the same
		// absent-vs-explicit reason: an omitted field defaults to Bearer, but an
		// explicit "token_type":"" is malformed metadata (api_error), uniform
		// with the other SDKs. Non-string token_type/refresh_token/scope still
		// fail Unmarshal here as pollInvalidResponse.
		var raw struct {
			AccessToken  string   `json:"access_token"`
			RefreshToken string   `json:"refresh_token"`
			TokenType    *string  `json:"token_type"`
			ExpiresIn    *float64 `json:"expires_in"`
			Scope        string   `json:"scope"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return pollResult{kind: pollInvalidResponse, status: resp.StatusCode, err: fmt.Errorf("parsing device token response: %w", err)}
		}
		if raw.AccessToken == "" {
			return pollResult{kind: pollInvalidResponse, status: resp.StatusCode, err: errors.New("device token response missing access_token")}
		}
		if raw.TokenType != nil && *raw.TokenType == "" {
			return pollResult{kind: pollInvalidResponse, status: resp.StatusCode, err: errors.New("device token response token_type must be a non-empty string")}
		}
		token := Token{
			AccessToken:  raw.AccessToken,
			RefreshToken: raw.RefreshToken,
			TokenType:    "Bearer",
			Scope:        raw.Scope,
		}
		if raw.TokenType != nil {
			token.TokenType = *raw.TokenType
		}
		// When present, expires_in must be a positive WHOLE number of seconds no
		// greater than maxTokenLifetimeSeconds — an explicit 0, a fractional
		// 3600.5, or an oversized value is a malformed response (api_error),
		// while an absent field yields a token with no expiry. The ceiling keeps
		// the time.Duration multiplication below from wrapping ExpiresAt.
		if raw.ExpiresIn != nil {
			v := *raw.ExpiresIn
			if v <= 0 || v > maxTokenLifetimeSeconds || v != math.Trunc(v) {
				return pollResult{kind: pollInvalidResponse, status: resp.StatusCode,
					err: fmt.Errorf("device token response expires_in must be a positive whole number of seconds no greater than %d", maxTokenLifetimeSeconds)}
			}
			token.ExpiresIn = int(v)
			token.ExpiresAt = cfg.clock().Add(time.Duration(token.ExpiresIn) * time.Second)
		}
		return pollResult{kind: pollToken, token: &token}
	}

	// Redirects are suppressed (http.ErrUseLastResponse), so a 3xx lands here
	// as-is. A redirect is never a valid token response — classify it as an api
	// fault BEFORE the OAuth-error body parse so a crafted
	// authorization_pending body on a 3xx cannot keep the loop polling.
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return pollResult{kind: pollInvalidResponse, status: resp.StatusCode,
			err: fmt.Errorf("device token endpoint returned redirect status %d", resp.StatusCode)}
	}

	var errResp struct {
		Error string `json:"error"`
	}
	oauthError := fmt.Sprintf("http_%d", resp.StatusCode)
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		oauthError = errResp.Error
	}
	return pollResult{kind: pollOAuthError, oauthError: oauthError, status: resp.StatusCode}
}

// PerformDeviceLogin runs the full RFC 8628 device authorization grant against
// an ALREADY-SELECTED config. It first guards the device capability — requiring
// BOTH a device authorization endpoint AND the device_code grant advertised —
// then requests a device code, surfaces it through display, and polls for the
// token.
//
// A config that cannot do device flow yields a DeviceFlowError(unavailable) and
// no request is made.
func PerformDeviceLogin(ctx context.Context, config *Config, clientID string, display func(DeviceAuthorization), opts ...DeviceOption) (*Token, error) {
	if config == nil || config.DeviceAuthorizationEndpoint == nil || !supportsDeviceGrant(config.GrantTypesSupported) {
		return nil, &DeviceFlowError{Reason: DeviceFlowUnavailable}
	}

	cfg := newDeviceConfig(opts)

	auth, err := RequestDeviceAuthorization(ctx, *config.DeviceAuthorizationEndpoint, clientID, opts...)
	if err != nil {
		return nil, err
	}

	// Anchor the code's lifetime at issuance so a slow display hook cannot yield a
	// fresh full polling window.
	deadline := cfg.clock().Add(time.Duration(auth.ExpiresIn) * time.Second)

	if display != nil {
		display(*auth)
	}

	// Charge display time against the code's lifetime. If the hook consumed the
	// whole window, the code has expired and no token request is warranted.
	if deadline.Sub(cfg.clock()) <= 0 {
		return nil, &DeviceFlowError{Reason: DeviceFlowExpired}
	}

	// Pass the EXACT issuance-anchored deadline — never a whole-second remaining
	// count that a re-anchoring clock read could round upward — so the poll loop
	// terminates precisely when the code expires.
	return pollDeviceTokenUntil(ctx, cfg, config.TokenEndpoint, clientID, auth.DeviceCode, auth.Interval, deadline)
}

// supportsDeviceGrant reports whether the advertised grant types include the
// device_code grant.
func supportsDeviceGrant(grantTypes []string) bool {
	for _, g := range grantTypes {
		if g == DeviceCodeGrantType {
			return true
		}
	}
	return false
}

// cancelCause prefers the context's own error (native cancellation) over the
// sleep seam's error, so a cancelled flow carries ctx.Err().
func cancelCause(ctx context.Context, sleepErr error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return sleepErr
}

// isTimeout reports whether err is a network timeout.
func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
