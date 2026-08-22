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
	"strconv"
	"strings"
	"time"

	surfguard "github.com/basecamp/surfguard/go"

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

	// maxDeviceRequestTimeout caps a caller-supplied per-request timeout (the
	// shared 3600 s ceiling — Python's _MAX_DEVICE_REQUEST_TIMEOUT, TS's
	// resolveDeviceTimeoutMs bound, Ruby's Fetcher::MAX_REQUEST_TIMEOUT). A
	// huge value like math.MaxInt64 would hold a stalled request open
	// effectively forever, defeating the flow's bounded-request guarantee.
	maxDeviceRequestTimeout = time.Hour
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
	// VerificationURIComplete embeds the user code in the URI (optional; nil when
	// absent, matching the other SDKs' nullable/optional modeling of this field).
	VerificationURIComplete *string
	// ExpiresIn is the device/user code lifetime in seconds.
	ExpiresIn int
	// Interval is the minimum polling interval in seconds (default 5).
	Interval int
}

// deviceConfig holds the resolved options for a device-flow operation.
type deviceConfig struct {
	// httpClient carries every device-flow POST. Resolved by newDeviceConfig:
	// a caller-supplied client, else a caller-supplied policy's client, else
	// the shared DefaultIssuerPolicy client.
	httpClient *http.Client
	// policyClient is WithDevicePolicy's client, used only when no client was
	// supplied directly.
	policyClient *http.Client
	scope        string
	hasScope     bool
	timeout      time.Duration
	clock        func() time.Time
	sleep        func(ctx context.Context, d time.Duration) error
}

// DeviceOption configures a device-flow operation.
type DeviceOption func(*deviceConfig)

// WithDeviceHTTPClient carries every device-flow request on the given client
// instead of the policy-enforced default, and takes precedence over
// WithDevicePolicy. Nil leaves the default.
//
// The client is the caller's, enforcement included: no address policy is
// applied on top of it. A consumer that must egress through a proxy has no
// other way to keep both, since surfguard's transport sets Proxy: nil by
// construction — compose the client's transport from
// DefaultIssuerPolicy().RoundTripper() where that is possible. Passing
// http.DefaultClient restores the pre-policy behavior outright.
func WithDeviceHTTPClient(c *http.Client) DeviceOption {
	return func(cfg *deviceConfig) {
		if c != nil {
			cfg.httpClient = c
		}
	}
}

// WithDevicePolicy replaces [DefaultIssuerPolicy] for the device-authorization
// and token endpoint POSTs, so a deployment whose authorization server is not
// on the public internet re-admits exactly the space it needs rather than
// switching the policy off. The derivations and the precedence trap are the
// ones documented on [WithIssuerPolicy]: AllowLoopback for a local server,
// surfguard.Policy{}.AllowAllPorts().Allow(...) for private space.
//
// It has no effect alongside WithDeviceHTTPClient, which supplies the transport
// the policy would otherwise be installed in.
//
// The option builds its transport once, when it is constructed, and every
// device-flow call it is passed to reuses it — so construct it once and keep
// it, rather than rebuilding it per call, or each call leaks a connection
// pool nothing can close.
func WithDevicePolicy(p surfguard.Policy) DeviceOption {
	client := newPolicyClient(p)
	return func(cfg *deviceConfig) { cfg.policyClient = client }
}

// WithDeviceScope sets the requested scope. When omitted, scope is left out of
// the request entirely so the server applies its default (`read`).
func WithDeviceScope(scope string) DeviceOption {
	return func(cfg *deviceConfig) {
		cfg.scope = scope
		cfg.hasScope = true
	}
}

// WithDeviceTimeout bounds each individual HTTP round-trip. Zero, negative,
// or beyond the shared 3600 s ceiling leaves the default (30s).
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
		timeout: defaultDeviceRequestTimeout,
		clock:   time.Now,
		sleep:   defaultDeviceSleep,
	}
	for _, o := range opts {
		o(&cfg)
	}
	// Non-positive AND oversized values both fall back to the default: the
	// same normalize-at-entry discipline as the other SDKs' timeout ceilings.
	if cfg.timeout <= 0 || cfg.timeout > maxDeviceRequestTimeout {
		cfg.timeout = defaultDeviceRequestTimeout
	}
	// Client precedence, the same on every surface of this package: a client
	// the caller handed us is theirs, enforcement included; else a caller's
	// policy; else the shared DefaultIssuerPolicy client. The default is
	// policed because the endpoints these POSTs target may come from
	// DiscoverFromResource's metadata, and nothing here can tell a discovered
	// endpoint from a hand-configured one — "the policy applies sometimes" is
	// the branch a bypass grows out of.
	switch {
	case cfg.httpClient != nil:
	case cfg.policyClient != nil:
		cfg.httpClient = cfg.policyClient
	default:
		cfg.httpClient = sharedPolicyClient()
	}
	// Suppress redirects on every device-flow POST so a 3xx surfaces as a non-2xx
	// api_error rather than the client chasing an attacker-influenced Location.
	// This copies, which is what keeps sharing the default client legal.
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
	VerificationURIComplete *string  `json:"verification_uri_complete"`
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
	// An already-cancelled ctx makes no request at all — a context-ignoring
	// injected RoundTripper would otherwise still send the POST (the standard
	// transport rejects pre-cancelled contexts itself). Mirrors TS's entry
	// throwIfAborted.
	if err := ctx.Err(); err != nil {
		return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: err}
	}

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

	// The suppression is about gosec's taint rule, not a claim that this URL is
	// trusted. DeviceAuthorizationEndpoint may be caller-configured, but it may
	// equally come from DiscoverFromResource's metadata, in which case a remote
	// peer chose it — so by default the client judges the address it resolves
	// to at dial time (DefaultIssuerPolicy, #806). This request carries
	// client_id.
	resp, err := cfg.httpClient.Do(req) // #nosec G704 -- see the note above: address-policed by default
	if err != nil {
		// A caller cancelling (or its deadline expiring) during the POST must
		// surface as DeviceFlowCancelled, not a retryable transport failure. The
		// parent ctx.Err() is non-nil only for a caller abort — the SDK's own
		// per-request timeout is the child reqCtx, whose expiry stays transport.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: ctxErr}
		}
		// A policy refusal is a permanent verdict on the endpoint, not a
		// transport fault: classify it before the retryable transport case, or
		// the consumer is told to retry a target it must stop talking to.
		if errors.Is(err, surfguard.ErrBlocked) {
			return nil, blockedEndpointError("device authorization endpoint", deviceAuthEndpoint, err)
		}
		return nil, &DeviceFlowError{Reason: DeviceFlowTransport, Err: fmt.Errorf("device authorization request failed: %w", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	// Re-check cancellation BEFORE classifying the completed response: a
	// context-ignoring RoundTripper can cancel the parent and still complete a
	// non-2xx or malformed response, and cancellation must win over every
	// completed outcome — matching the token poll's pre-classification
	// re-check.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: ctxErr}
	}

	// A non-2xx is a hard failure whose body is unused — surface it by status
	// BEFORE reading the body. Otherwise a slow/never-ending error body could hit
	// the request timeout mid-read and be misclassified as a retryable transport
	// failure instead of the api_error it is.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, basecamp.ErrAPI(resp.StatusCode,
			fmt.Sprintf("device authorization failed with status %d", resp.StatusCode))
	}

	body, err := readBoundedBody(resp.Body, maxTokenResponseBytes)
	if err != nil {
		// An oversized body is a server/api fault (api_error, not retryable); any
		// other read failure is a transport failure, matching the Do() error above.
		if errors.Is(err, errBodyTooLarge) {
			return nil, &basecamp.Error{Code: basecamp.CodeAPI, Message: fmt.Sprintf("device authorization response too large: %v", err), Cause: err}
		}
		// A caller abort while the body is still streaming must surface as
		// cancellation too, not transport — same parent-ctx.Err() guard as the Do()
		// error above and the token poll's body read.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: ctxErr}
		}
		return nil, &DeviceFlowError{Reason: DeviceFlowTransport, Err: fmt.Errorf("reading device authorization response: %w", err)}
	}

	// Re-check cancellation the moment the body is in hand, BEFORE parsing:
	// a context-ignoring RoundTripper whose Body.Read cancels the parent while
	// yielding malformed JSON would otherwise classify as api_error first —
	// cancellation wins over every completed outcome (same contract as the
	// token poll). This also covers the valid-response-after-cancel case for
	// direct callers.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: ctxErr}
	}

	var raw rawDeviceAuthorization
	if err := json.Unmarshal(body, &raw); err != nil {
		// Cancellation wins even over a malformed body: another goroutine can
		// cancel while Unmarshal chews a large invalid payload.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: ctxErr}
		}
		// Carry the HTTP status like the sibling non-2xx raise above (and Python)
		// so a failed 2xx-body parse still reports which response it came from.
		return nil, &basecamp.Error{Code: basecamp.CodeAPI, Message: "failed to parse device authorization response", HTTPStatus: resp.StatusCode, Cause: err}
	}
	// And once more AFTER decoding: a cancellation landing while Unmarshal
	// chewed a large body must not hand back a usable device code.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: ctxErr}
	}
	return validateDeviceAuthorization(raw, resp.StatusCode)
}

func validateDeviceAuthorization(raw rawDeviceAuthorization, status int) (*DeviceAuthorization, error) {
	// Carry the (2xx) status on every validation error so a malformed success
	// body is diagnosable as such, uniform with the token-poll raises and the
	// other SDKs.
	apiErr := func(msg string) error {
		return &basecamp.Error{Code: basecamp.CodeAPI, Message: msg, HTTPStatus: status}
	}

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
// cancelled. Other server errors surface as a coded *basecamp.Error; an
// out-of-range interval/expiresIn is rejected up front as a usage error.
func PollDeviceToken(ctx context.Context, tokenEndpoint, clientID, deviceCode string, interval, expiresIn int, opts ...DeviceOption) (*Token, error) {
	// Caller-input sanity for this exported entry point (usage, not RFC response
	// validation): a non-positive duration builds a deadline in the past, and an
	// oversized one overflows the time.Duration(seconds) * time.Second math below
	// into a garbage (possibly negative) deadline. Reject both, mirroring the TS
	// pollDeviceToken guard so a direct caller gets the same rejection across SDKs.
	// PerformDeviceLogin bypasses this via pollDeviceTokenUntil with already-
	// validated, clamped values, so this only bounds direct callers.
	for _, field := range []struct {
		name  string
		value int
	}{{"expiresIn", expiresIn}, {"interval", interval}} {
		if field.value <= 0 || field.value > maxDeviceSeconds {
			return nil, &basecamp.Error{
				Code:    basecamp.CodeUsage,
				Message: fmt.Sprintf("PollDeviceToken: %s must be a positive number of seconds no greater than %d", field.name, maxDeviceSeconds),
			}
		}
	}
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
	// One-shot next-wait override from a 429 too_many_requests Retry-After
	// (SPEC §16): consumed by the next wait, never inflating the slow_down
	// interval. 0 = none.
	overrideSeconds := 0

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
		wait := time.Duration(max(intervalSeconds, backoffSeconds, overrideSeconds)) * time.Second
		overrideSeconds = 0 // one-shot: consumed by this wait, then gone
		if remaining < wait {
			wait = remaining
		}
		if err := cfg.sleep(ctx, wait); err != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: cancelCause(ctx, err)}
		}
		if err := ctx.Err(); err != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: err}
		}
		postRemaining := deadline.Sub(cfg.clock())
		if postRemaining <= 0 {
			return nil, &DeviceFlowError{Reason: DeviceFlowExpired}
		}

		// Bound the request by the REMAINING code lifetime as well as the
		// per-request timeout: near expiry, a stalled token POST must not hold
		// the flow past the monotonic deadline for the full request budget.
		result := postDeviceToken(ctx, cfg, tokenEndpoint, form, min(cfg.timeout, postRemaining))
		// Re-check cancellation before classifying ANY completed round trip:
		// an injected RoundTripper that ignores the request context can
		// complete a 200, a terminal 4xx (access_denied, expired_token), or a
		// malformed response after ctx is cancelled — the caller asked to
		// stop, so cancelled wins over every completed outcome. Mirrors the
		// post-authorization re-check and the Py/Rb post-round-trip probes.
		if err := ctx.Err(); err != nil {
			return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: err}
		}
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
		case pollBlocked:
			return nil, blockedEndpointError("token endpoint", tokenEndpoint, result.err)
		case pollOAuthError:
			// Any completed round-trip resets the timeout backoff to the
			// server-driven interval.
			backoffSeconds = intervalSeconds
			switch result.oauthError {
			case "authorization_pending":
				continue
			case "too_many_requests":
				// Retryable ONLY as the exact 429 + too_many_requests pair
				// (SPEC §16). The next wait honors a positive integral
				// Retry-After delta via a one-shot max(interval, Retry-After)
				// override — a missing/malformed header falls back to the
				// current interval, and the override decays after one wait.
				if result.status != http.StatusTooManyRequests {
					return nil, basecamp.ErrAPI(result.status,
						fmt.Sprintf("device token request failed: %s", result.oauthError))
				}
				overrideSeconds = parseRetryAfterSeconds(result.retryAfter)
				continue
			case "slow_down":
				intervalSeconds += slowDownIncrementSeconds
				// Re-sync the backoff to the GROWN interval: the reset above used
				// the pre-increment value, so a subsequent timeout must double from
				// the new interval, not the stale one (else the client polls too
				// aggressively under combined throttling + network timeouts).
				backoffSeconds = intervalSeconds
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
	// pollBlocked is the address policy refusing the token endpoint before any
	// connection opened (api_error, NOT retryable, and never backed off).
	pollBlocked
)

type pollResult struct {
	kind       pollResultKind
	token      *Token
	oauthError string
	status     int
	err        error
	// retryAfter is the raw Retry-After header on an OAuth-error response,
	// consumed by the loop's 429 too_many_requests handling only.
	retryAfter string
}

// parseRetryAfterSeconds validates a Retry-After delta for the 429 poll
// contract (SPEC §16): ASCII digits only (HTTP delta-seconds permits no
// sign), positive. A representable delta beyond maxDeviceSeconds (the shared
// 32-bit-ms timer bound) CLAMPS to the ceiling — the wait rule clips to the
// remaining code lifetime, honoring the throttle. Anything else — missing,
// an HTTP-date, signed ("+30"), fractional, non-positive, or unrepresentable
// (ErrRange) — returns 0 so the caller falls back to the current interval. Trimming is ASCII SP/HTAB only (RFC 9110 OWS) — NOT
// strings.TrimSpace, whose Unicode whitespace (NBSP above all) would trim a
// malformed value into validity.
func parseRetryAfterSeconds(header string) int {
	trimmed := strings.Trim(header, " \t")
	if trimmed == "" {
		return 0
	}
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return 0
		}
	}
	// The shared 10-significant-digit bound (SPEC §16): strip leading zeros
	// first so a padded in-range delta is honored, then treat longer strings
	// as unrepresentable → interval fallback — matching TS/Python/Ruby, where
	// an 11-digit delta must not clamp in one SDK and fall back in another.
	significant := strings.TrimLeft(trimmed, "0")
	if significant == "" {
		significant = "0"
	}
	if len(significant) > 10 {
		return 0
	}
	// A digit string too long for int returns ErrRange → malformed → 0.
	v, err := strconv.Atoi(significant)
	if err != nil || v <= 0 {
		return 0
	}
	// A REPRESENTABLE delta beyond the shared device ceiling clamps rather
	// than falling back: the wait rule clamps to the remaining code lifetime
	// anyway, so an over-ceiling throttle waits out the rest of the lifetime
	// instead of resending before the server's throttle. Only unrepresentable
	// strings (ErrRange above) are malformed → interval fallback.
	return min(v, maxDeviceSeconds)
}

// postDeviceToken performs one token-endpoint poll and classifies the outcome.
// A parent-context cancellation is pollCancelled; a per-request timeout (or any
// net timeout) is pollTimeout (→ backoff); any other transport failure is
// pollTransport. A 2xx yields a Token; a 3xx is pollInvalidResponse; any other
// non-2xx with an OAuth error body yields pollOAuthError.
func postDeviceToken(ctx context.Context, cfg deviceConfig, tokenEndpoint string, form url.Values, timeout time.Duration) pollResult {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return pollResult{kind: pollTransport, err: fmt.Errorf("creating device token request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// As above: TokenEndpoint may come from discovered metadata rather than from
	// the caller, so by default its address is judged at dial time
	// (DefaultIssuerPolicy, #806). This request carries the device_code.
	resp, err := cfg.httpClient.Do(req) // #nosec G704 -- see the note above: address-policed by default
	if err != nil {
		// Parent cancellation ends the flow; a per-request timeout backs off.
		if ctx.Err() != nil {
			return pollResult{kind: pollCancelled, err: ctx.Err()}
		}
		// A policy refusal terminates the flow. It is classified ahead of the
		// timeout and transport cases so the loop can neither back off and
		// re-dial a target it must stop talking to, nor report it as a
		// retryable transport failure.
		if errors.Is(err, surfguard.ErrBlocked) {
			return pollResult{kind: pollBlocked, err: err}
		}
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return pollResult{kind: pollTimeout, err: err}
		}
		return pollResult{kind: pollTransport, err: fmt.Errorf("device token poll failed: %w", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	// A suppressed 3xx is an api fault whose body is unused — classify it by status
	// BEFORE reading the body. Otherwise a redirect that slowly streams its body
	// could time out mid-read (→ pollTimeout → the loop backs off and retries until
	// the device code expires) instead of failing as api_error now.
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return pollResult{kind: pollInvalidResponse, status: resp.StatusCode,
			err: fmt.Errorf("device token endpoint returned redirect status %d", resp.StatusCode)}
	}

	// Every remaining status outside 200 and 4xx is terminal WITHOUT its body
	// (only a 200 carries the token and only a 4xx the OAuth error code) —
	// classify it before the read, like the 3xx above, so a 201/500 that
	// stalls while streaming its body cannot time out mid-read and be retried
	// as a transient failure until the code expires.
	if resp.StatusCode != http.StatusOK && (resp.StatusCode < 400 || resp.StatusCode >= 500) {
		return pollResult{kind: pollInvalidResponse, status: resp.StatusCode,
			err: fmt.Errorf("device token request failed with status %d", resp.StatusCode)}
	}

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

	// Exactly HTTP 200, not any 2xx: RFC 8628/6749 token responses are 200, and
	// SPEC §16 pins the contract. A nonstandard 201/202 never reaches here — the
	// status-first gate above already terminated every non-200/non-4xx as
	// api_error (http_<status>) without reading its body.
	if resp.StatusCode == http.StatusOK {
		// expires_in decodes via *float64, not Token's plain int: a pointer keeps
		// an explicit "expires_in":0 distinguishable from an omitted field (a
		// plain int makes 0 look absent and skip validation), and float64 accepts
		// an integer-valued 3600.0 per the cross-SDK contract. Whole-second
		// enforcement happens below. token_type is *string for the same
		// absent-vs-explicit reason: an omitted field defaults to Bearer, but an
		// explicit "token_type":"" is malformed metadata (api_error), uniform
		// with the other SDKs. Non-string token_type/refresh_token/scope still
		// fail Unmarshal here as pollInvalidResponse.
		// RefreshToken/Scope decode via *string to make null-tolerance
		// EXPLICIT: encoding/json also no-ops JSON null into a plain string
		// (leaving the zero value), but the pointer form documents the
		// absent/null-are-absent contract instead of relying on that quirk.
		var raw struct {
			AccessToken  string   `json:"access_token"`
			RefreshToken *string  `json:"refresh_token"`
			TokenType    *string  `json:"token_type"`
			ExpiresIn    *float64 `json:"expires_in"`
			Scope        *string  `json:"scope"`
			// *string like token_type: absent and JSON null map to nil, while
			// a present-but-empty "" is malformed (SPEC §16 resource rule) —
			// a plain string could not tell those apart.
			Resource *string `json:"resource"`
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
		if raw.Resource != nil && *raw.Resource == "" {
			return pollResult{kind: pollInvalidResponse, status: resp.StatusCode, err: errors.New("device token response resource must be a non-empty string when present")}
		}
		token := Token{
			AccessToken: raw.AccessToken,
			TokenType:   "Bearer",
		}
		if raw.RefreshToken != nil {
			token.RefreshToken = *raw.RefreshToken
		}
		if raw.Scope != nil {
			token.Scope = *raw.Scope
		}
		if raw.TokenType != nil {
			token.TokenType = *raw.TokenType
		}
		if raw.Resource != nil {
			token.Resource = *raw.Resource
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
			// Wall time, NOT cfg.clock(): the injected clock is a monotonic
			// polling-deadline seam (tests feed it artificial instants), while
			// ExpiresAt is a public wall-clock field consumed outside the poll
			// loop — matching exchange.go's time.Now() anchoring.
			token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
		}
		return pollResult{kind: pollToken, token: &token}
	}

	// Recognize OAuth protocol error codes ONLY on a 4xx (RFC 8628 §3.5 error
	// responses are 400-class): a nonstandard 2xx (201/202) or a 5xx carrying a
	// crafted {"error":"authorization_pending"} body must not keep the loop
	// polling — only a 200 can produce a token and only a 4xx can produce a
	// protocol state. Everything else is forced to http_<status>, which the
	// loop terminates as api_error. (3xx never reaches here — classified above
	// before the body read.)
	oauthError := fmt.Sprintf("http_%d", resp.StatusCode)
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			// Truncate at extraction (SPEC §9's 500-unit message cap): the
			// server controls this string and an unrecognized value is
			// interpolated into the api_error message. Real protocol codes are
			// short, so classification is unaffected.
			oauthError = errResp.Error
			if len(oauthError) > maxErrorMessageLen {
				oauthError = oauthError[:maxErrorMessageLen-3] + "..."
			}
		}
		// A 429 recognizes ONLY too_many_requests (the exact retryable pair):
		// a throttling endpoint whose body parrots authorization_pending or
		// slow_down must not keep the loop polling until code expiry — any
		// other code on a 429 is forced to http_429 and terminates as
		// api_error. Conversely too_many_requests off a 429 is already
		// terminal in the loop.
		if resp.StatusCode == http.StatusTooManyRequests && oauthError != "too_many_requests" {
			oauthError = fmt.Sprintf("http_%d", resp.StatusCode)
		}
	}
	// Exactly one Retry-After field line: duplicates make the combined field
	// ambiguous (Header.Get silently takes the first), so anything but a
	// single value falls back to the current interval via the empty string.
	retryAfter := ""
	if vals := resp.Header.Values("Retry-After"); len(vals) == 1 {
		retryAfter = vals[0]
	}
	return pollResult{kind: pollOAuthError, oauthError: oauthError, status: resp.StatusCode,
		retryAfter: retryAfter}
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
	// Present-but-empty is as unavailable as absent (the other SDK guards
	// treat "" as no endpoint): without the dereference check an empty
	// endpoint would fall through to RequestDeviceAuthorization and surface a
	// usage/security error instead of the documented unavailable — after
	// bypassing the make-no-request contract.
	if config == nil || config.DeviceAuthorizationEndpoint == nil ||
		*config.DeviceAuthorizationEndpoint == "" || !supportsDeviceGrant(config.GrantTypesSupported) {
		return nil, &DeviceFlowError{Reason: DeviceFlowUnavailable}
	}

	// A nil display is a usage error, not a skippable step: it is the ONLY
	// mechanism that surfaces the verification URI and user code, so skipping
	// it would mint a code nobody can approve and poll until it expires.
	// Reject before any request is issued.
	if display == nil {
		return nil, &basecamp.Error{Code: basecamp.CodeUsage, Message: "PerformDeviceLogin: display callback is required"}
	}

	cfg := newDeviceConfig(opts)

	auth, err := RequestDeviceAuthorization(ctx, *config.DeviceAuthorizationEndpoint, clientID, opts...)
	if err != nil {
		return nil, err
	}

	// Re-check cancellation before surfacing the code: a ctx cancelled after
	// the request completed (or under an injected RoundTripper that ignores
	// request cancellation) must never reach the display hook — matching the
	// Py/Rb/TS orchestrators' post-request re-check. The in-flight case is
	// covered by the request's own context threading.
	if err := ctx.Err(); err != nil {
		return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: err}
	}

	// Anchor the code's lifetime at ISSUANCE — the response's arrival, per
	// SPEC §16 — before the display hook, so a slow display eats into the
	// deadline instead of resetting it. Expiry past this point is arbitrated
	// by the server (expired_token), so receipt-anchoring fails safe.
	deadline := cfg.clock().Add(time.Duration(auth.ExpiresIn) * time.Second)

	display(*auth)

	// Cancellation raised INSIDE the display hook (a prompt closing in
	// response to cancellation) wins over expiry: a hook that both cancels
	// and consumes the lifetime must surface cancelled, not expired.
	if err := ctx.Err(); err != nil {
		return nil, &DeviceFlowError{Reason: DeviceFlowCancelled, Err: err}
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
