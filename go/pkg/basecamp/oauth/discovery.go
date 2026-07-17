package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// LaunchpadBaseURL is the default Basecamp/Launchpad OAuth authorization server.
const LaunchpadBaseURL = "https://launchpad.37signals.com"

// Well-known discovery paths.
const (
	wellKnownAS       = "/.well-known/oauth-authorization-server"
	wellKnownResource = "/.well-known/oauth-protected-resource"
)

// Discovery limits.
const (
	// maxDiscoveryBodyBytes bounds a discovery response body (1 MiB); discovery
	// documents are tiny.
	maxDiscoveryBodyBytes int64 = 1 * 1024 * 1024
	// defaultDiscoveryTimeout bounds each discovery fetch.
	defaultDiscoveryTimeout = 10 * time.Second
)

// Discoverer fetches OAuth 2.0 server configuration from discovery endpoints.
//
// All fetches are SSRF-hardened: origins are validated with net/url before any
// socket opens, HTTPS is required (localhost exempt), redirects are suppressed,
// timeouts are bounded, and bodies are read under a genuine bounded cap that
// aborts before an oversized body is fully buffered.
type Discoverer struct {
	httpClient *http.Client
}

// NewDiscoverer creates a Discoverer with the given HTTP client.
// If httpClient is nil, http.DefaultClient is used.
func NewDiscoverer(httpClient *http.Client) *Discoverer {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Discoverer{httpClient: httpClient}
}

// DiscoverOption configures a discovery operation.
type DiscoverOption func(*discoverConfig)

type discoverConfig struct {
	expectedIssuer string
	hasExpected    bool
	timeout        time.Duration
	maxBodyBytes   int64
}

// WithExpectedIssuer sets an explicit, authoritative issuer for
// DiscoverFromResource. When provided, the advertised member equal by code-point
// is selected; if none matches, discovery raises ErrExpectedIssuerUnavailable
// (it never falls back). Omit to use the Basecamp-profile exclusion heuristic.
func WithExpectedIssuer(issuer string) DiscoverOption {
	return func(c *discoverConfig) {
		c.expectedIssuer = issuer
		c.hasExpected = true
	}
}

// WithTimeout bounds each discovery fetch. Zero or negative leaves the default (10s).
func WithTimeout(d time.Duration) DiscoverOption {
	return func(c *discoverConfig) { c.timeout = d }
}

// WithMaxBodyBytes caps the discovery response body read. Zero or negative
// leaves the default (1 MiB).
func WithMaxBodyBytes(n int64) DiscoverOption {
	return func(c *discoverConfig) { c.maxBodyBytes = n }
}

func newDiscoverConfig(opts []DiscoverOption) discoverConfig {
	c := discoverConfig{timeout: defaultDiscoveryTimeout, maxBodyBytes: maxDiscoveryBodyBytes}
	for _, o := range opts {
		o(&c)
	}
	if c.timeout <= 0 {
		c.timeout = defaultDiscoveryTimeout
	}
	if c.maxBodyBytes <= 0 {
		c.maxBodyBytes = maxDiscoveryBodyBytes
	}
	return c
}

// requireOriginRoot parses a caller- or metadata-supplied origin and enforces
// the origin-root profile: scheme https (or http on localhost), host present,
// valid/absent port, path empty or exactly "/", and no query/fragment/userinfo.
// It uses net/url (never a regex) so bracketed IPv6 and ports agree with the
// host the client actually dials.
//
// A violation is a usage error — a bad *caller* origin is a usage error; callers
// validating an *advertised* origin reclassify it. The returned origin is
// normalized (scheme://host[:port], default ports and trailing slash dropped).
func requireOriginRoot(raw, label string) (string, error) {
	usage := func(msg string) error {
		return &basecamp.Error{Code: basecamp.CodeUsage, Message: msg}
	}

	// Reject C0 controls, space, and backslash up front: URL parsers variously
	// strip tabs/newlines/surrounding spaces or convert backslashes to slashes,
	// so a malformed spelling ("https:\\host", "https://host\n") could be cleaned
	// and accepted. None of these code points is legitimate in an origin root.
	if strings.IndexFunc(raw, func(r rune) bool { return r <= 0x20 || r == '\\' }) >= 0 {
		return "", usage(fmt.Sprintf("%s contains invalid characters: %s", label, raw))
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", usage(fmt.Sprintf("invalid %s: not a valid absolute URL: %s", label, raw))
	}

	// Scheme profile: https anywhere, or http on localhost. RequireSecureEndpoint
	// encodes exactly that (localhost exempt from the HTTPS requirement).
	scheme := strings.ToLower(u.Scheme)
	if err := basecamp.RequireSecureEndpoint(raw); err != nil {
		return "", usage(fmt.Sprintf("%s must use HTTPS (or http on localhost): %s", label, raw))
	}
	if u.Hostname() == "" {
		return "", usage(fmt.Sprintf("%s has no host: %s", label, raw))
	}
	if u.User != nil {
		return "", usage(fmt.Sprintf("%s must not contain userinfo: %s", label, raw))
	}
	// net/url sets ForceQuery for a bare trailing "?" but has no equivalent for a
	// bare "#" (Fragment is "" for both absent and empty), so also scan the raw
	// input: a "#" only ever delimits a fragment here.
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") {
		return "", usage(fmt.Sprintf("%s must not contain a query or fragment: %s", label, raw))
	}
	if u.Path != "" && u.Path != "/" {
		return "", usage(fmt.Sprintf("%s must be an origin root (no path): %s", label, raw))
	}
	if u.Opaque != "" {
		return "", usage(fmt.Sprintf("%s must be an origin root: %s", label, raw))
	}

	// url.Parse rejects a non-numeric port but accepts a numeric-but-out-of-range
	// one (e.g. ":99999"), so range-check it explicitly against 1–65535.
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", usage(fmt.Sprintf("%s has an invalid port: %s", label, raw))
		}
	}

	// net/url reports an empty Port() for a dangling ":" ("https://host:"), so
	// scan the raw authority for a trailing ":" (an IPv6 authority ends with "]",
	// so only a trailing ":" is a dangling port).
	authority := raw
	if i := strings.Index(authority, "://"); i >= 0 {
		authority = authority[i+3:]
	}
	if j := strings.IndexAny(authority, "/?#"); j >= 0 {
		authority = authority[:j]
	}
	if strings.HasSuffix(authority, ":") {
		return "", usage(fmt.Sprintf("%s has an invalid port: %s", label, raw))
	}

	// Lowercase the host: DNS names and schemes are case-insensitive (RFC 3986
	// §3.1/§6.2.2.1), so a mixed-case advertised issuer like
	// https://Launchpad.37signals.com must normalize to the same origin as its
	// canonical form — otherwise the Launchpad exclusion misses it and it is
	// wrongly treated as a distinct BC5 issuer.
	host := strings.ToLower(u.Hostname())
	origin := scheme + "://"
	if strings.Contains(host, ":") {
		// IPv6 literal — re-bracket (Hostname strips the brackets).
		origin += "[" + host + "]"
	} else {
		origin += host
	}
	if port := u.Port(); port != "" && !isDefaultPort(scheme, port) {
		origin += ":" + port
	}
	return origin, nil
}

func isDefaultPort(scheme, port string) bool {
	return (scheme == "https" && port == "443") || (scheme == "http" && port == "80")
}

// isLaunchpadIssuer reports whether an advertised issuer denotes Launchpad.
func isLaunchpadIssuer(issuer string) bool {
	origin, err := requireOriginRoot(issuer, "issuer")
	if err != nil {
		return false
	}
	launchpad, err := requireOriginRoot(LaunchpadBaseURL, "issuer")
	if err != nil {
		return false
	}
	return origin == launchpad
}

// noRedirectClient returns a shallow copy of the configured client that
// suppresses redirects. A 3xx then surfaces as a non-2xx api_error rather than
// the client chasing an attacker-influenced Location.
func (d *Discoverer) noRedirectClient() *http.Client {
	c := *d.httpClient
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &c
}

// errBodyTooLarge is returned (wrapped) by readBoundedBody when a response body
// exceeds the byte cap. It is the package sentinel that lets callers tell an
// oversized body (an api_error, not retryable) apart from a genuine read/I-O
// failure (transport). Discovery and device flow both classify against it via
// errors.Is.
var errBodyTooLarge = errors.New("response body exceeds size cap")

// readBoundedBody reads at most maxBytes from r, aborting once the cap is
// exceeded so an oversized body is never fully buffered (io.LimitReader reads at
// most maxBytes+1 bytes and we detect the overflow byte).
//
// On overflow it returns errBodyTooLarge (wrapped); on any other failure it
// returns the underlying read error unwrapped, so callers can distinguish the
// two with errors.Is.
func readBoundedBody(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w (%d byte cap)", errBodyTooLarge, maxBytes)
	}
	return data, nil
}

func truncateBody(body []byte) string {
	s := string(body)
	if len(s) > maxErrorMessageLen {
		return s[:maxErrorMessageLen-3] + "..."
	}
	return s
}

// fetchDiscoveryDocument performs an SSRF-hardened GET of a discovery document.
// The origin must already be validated via requireOriginRoot; this re-checks TLS,
// suppresses redirects, bounds the timeout, reads the body under a bounded cap,
// and maps non-2xx to api_error.
func (d *Discoverer) fetchDiscoveryDocument(ctx context.Context, rawURL string, cfg discoverConfig) ([]byte, error) {
	if err := basecamp.RequireSecureEndpoint(rawURL); err != nil {
		return nil, &basecamp.Error{
			Code:    basecamp.CodeUsage,
			Message: fmt.Sprintf("OAuth discovery endpoint is not secure: %s", rawURL),
			Cause:   err,
		}
	}

	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &basecamp.Error{
			Code:    basecamp.CodeUsage,
			Message: fmt.Sprintf("creating OAuth discovery request: %v", err),
			Cause:   err,
		}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.noRedirectClient().Do(req) // #nosec G704 -- SDK HTTP client: URL is origin-root validated
	if err != nil {
		return nil, basecamp.ErrNetwork(fmt.Errorf("OAuth discovery request failed: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Drain-and-cap defensively; the body is only used for the message.
		body, _ := readBoundedBody(resp.Body, cfg.maxBodyBytes)
		return nil, basecamp.ErrAPI(resp.StatusCode,
			fmt.Sprintf("OAuth discovery failed with status %d: %s", resp.StatusCode, truncateBody(body)))
	}

	body, err := readBoundedBody(resp.Body, cfg.maxBodyBytes)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			return nil, &basecamp.Error{
				Code:    basecamp.CodeAPI,
				Message: fmt.Sprintf("OAuth discovery response too large: %v", err),
				Cause:   err,
			}
		}
		// A mid-stream read failure (peer reset, timeout) on a 2xx is a
		// transient transport fault, not malformed AS metadata — only the
		// size-cap overflow above is an api_error.
		return nil, &basecamp.Error{
			Code:      basecamp.CodeNetwork,
			Message:   fmt.Sprintf("reading OAuth discovery response: %v", err),
			Retryable: true,
			Cause:     err,
		}
	}
	return body, nil
}

// Discover fetches OAuth 2.0 Authorization Server Metadata (RFC 8414) from
// {baseURL}/.well-known/oauth-authorization-server and binds it: the returned
// issuer must equal the requested issuer by code-point. token_endpoint is
// required; authorization_endpoint is optional (device-only servers omit it).
//
// The baseURL should be the OAuth server's issuer origin
// (e.g., "https://launchpad.37signals.com").
func (d *Discoverer) Discover(ctx context.Context, baseURL string, opts ...DiscoverOption) (*Config, error) {
	origin, err := requireOriginRoot(baseURL, "OAuth discovery base URL")
	if err != nil {
		return nil, err
	}
	// Bind against the caller's raw baseURL (RFC 8414 §3.3, SPEC.md §16 "NO
	// normalization"); the normalized origin is only for the fetch URL.
	return d.fetchASMetadata(ctx, origin, baseURL, newDiscoverConfig(opts))
}

// rawDiscoveryResponse mirrors an RFC 8414 metadata document. Endpoint fields
// are pointers so present-but-empty ("") is distinguishable from absent.
type rawDiscoveryResponse struct {
	Issuer                        *string  `json:"issuer"`
	AuthorizationEndpoint         *string  `json:"authorization_endpoint"`
	TokenEndpoint                 *string  `json:"token_endpoint"`
	DeviceAuthorizationEndpoint   *string  `json:"device_authorization_endpoint"`
	RegistrationEndpoint          *string  `json:"registration_endpoint"`
	ScopesSupported               []string `json:"scopes_supported"`
	GrantTypesSupported           []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
}

// fetchASMetadata fetches the AS metadata from issuerOrigin's well-known URL but
// binds the returned issuer against bindIssuer by code-point. Routing and binding
// are distinct: the resource-first flow fetches from the normalized origin yet
// binds against the exact advertised issuer string (which may spell a trailing
// slash or explicit default port). Public Discover passes the same value for both.
func (d *Discoverer) fetchASMetadata(ctx context.Context, issuerOrigin, bindIssuer string, cfg discoverConfig) (*Config, error) {
	body, err := d.fetchDiscoveryDocument(ctx, issuerOrigin+wellKnownAS, cfg)
	if err != nil {
		return nil, err
	}
	return parseAndBindASMetadata(body, bindIssuer)
}

// parseAndBindASMetadata validates AS metadata and binds issuer to
// expectedIssuerOrigin by code-point. Universal validation only: issuer and
// token_endpoint present and non-empty; any present endpoint field non-empty.
// Per-grant endpoint checks are the consumer's responsibility.
func parseAndBindASMetadata(body []byte, expectedIssuerOrigin string) (*Config, error) {
	apiErr := func(msg string) error {
		return &basecamp.Error{Code: basecamp.CodeAPI, Message: msg}
	}

	var raw rawDiscoveryResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &basecamp.Error{Code: basecamp.CodeAPI, Message: "failed to parse OAuth discovery response", Cause: err}
	}

	if raw.Issuer == nil || *raw.Issuer == "" {
		return nil, apiErr("invalid OAuth discovery response: missing required field (issuer)")
	}
	// RFC 8414 §3.3/§4: issuer identical by code-point. No normalization.
	if *raw.Issuer != expectedIssuerOrigin {
		return nil, &basecamp.Error{
			Code:    basecamp.CodeAPI,
			Message: fmt.Sprintf("OAuth issuer mismatch: metadata issuer %q does not equal %q", *raw.Issuer, expectedIssuerOrigin),
			Cause:   errIssuerBindingMismatch,
		}
	}
	if raw.TokenEndpoint == nil || *raw.TokenEndpoint == "" {
		return nil, apiErr("invalid OAuth discovery response: missing required field (token_endpoint)")
	}
	if err := rejectEmptyEndpoints(body); err != nil {
		return nil, err
	}
	if err := rejectNullListFields(body, "grant_types_supported", "scopes_supported", "code_challenge_methods_supported"); err != nil {
		return nil, err
	}

	cfg := &Config{
		Issuer:                        *raw.Issuer,
		AuthorizationEndpoint:         raw.AuthorizationEndpoint,
		TokenEndpoint:                 *raw.TokenEndpoint,
		DeviceAuthorizationEndpoint:   raw.DeviceAuthorizationEndpoint,
		GrantTypesSupported:           raw.GrantTypesSupported,
		ScopesSupported:               raw.ScopesSupported,
		CodeChallengeMethodsSupported: raw.CodeChallengeMethodsSupported,
	}
	if raw.RegistrationEndpoint != nil {
		cfg.RegistrationEndpoint = *raw.RegistrationEndpoint
	}
	return cfg, nil
}

// rejectEmptyEndpoints rejects any present "*_endpoint" field that is not a
// non-empty string. A present endpoint must be a non-empty string: an empty
// string, or a non-string value (number, array, object, or JSON null), is
// malformed metadata — not silently treated as absent.
func rejectEmptyEndpoints(body []byte) error {
	var m map[string]json.RawMessage
	// The body already parsed as an object upstream; a decode failure here leaves
	// m empty and the loop below is a no-op.
	_ = json.Unmarshal(body, &m)
	for k, v := range m {
		if !strings.HasSuffix(k, "_endpoint") {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) != nil || s == "" {
			return &basecamp.Error{
				Code:    basecamp.CodeAPI,
				Message: fmt.Sprintf("invalid OAuth discovery response: %s must be a non-empty string", k),
			}
		}
	}
	return nil
}

// rejectNullListFields rejects any of the named optional list fields that is
// present with a JSON null value. A present list field must be an array (RFC
// 8414 / RFC 9728); a JSON null is malformed metadata, distinct from an absent
// key (which the typed decode legitimately treats as unset).
func rejectNullListFields(body []byte, keys ...string) error {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(body, &m)
	for _, k := range keys {
		v, present := m[k]
		if present && string(bytes.TrimSpace(v)) == "null" {
			return &basecamp.Error{
				Code:    basecamp.CodeAPI,
				Message: fmt.Sprintf("invalid OAuth discovery response: %s must be an array when present, not null", k),
			}
		}
	}
	return nil
}

// DiscoverProtectedResource fetches RFC 9728 protected-resource metadata from
// {resourceOrigin}/.well-known/oauth-protected-resource. resource is required and
// must equal the requested origin by code-point. authorization_servers is
// preserved distinctly as absent vs [].
func (d *Discoverer) DiscoverProtectedResource(ctx context.Context, resourceOrigin string, opts ...DiscoverOption) (*ProtectedResourceMetadata, error) {
	origin, err := requireOriginRoot(resourceOrigin, "resource origin")
	if err != nil {
		return nil, err
	}
	return d.fetchProtectedResource(ctx, origin, resourceOrigin, newDiscoverConfig(opts))
}

// fetchProtectedResource fetches from origin's well-known URL but binds the
// metadata resource against bindResource by code-point. Routing and binding are
// distinct (RFC 9728 §3.1/§3.3): the well-known URL is built from the normalized
// origin, but doc.resource must be identical to the resource identifier the
// caller supplied — with NO normalization (SPEC.md §16). DiscoverProtectedResource
// and DiscoverFromResource pass the raw resourceOrigin.
func (d *Discoverer) fetchProtectedResource(ctx context.Context, origin, bindResource string, cfg discoverConfig) (*ProtectedResourceMetadata, error) {
	body, err := d.fetchDiscoveryDocument(ctx, origin+wellKnownResource, cfg)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Resource             *string   `json:"resource"`
		AuthorizationServers *[]string `json:"authorization_servers"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &basecamp.Error{Code: basecamp.CodeAPI, Message: "failed to parse resource metadata response", Cause: err}
	}
	if raw.Resource == nil || *raw.Resource == "" {
		return nil, &basecamp.Error{Code: basecamp.CodeAPI, Message: "invalid resource metadata: missing required field (resource)"}
	}
	if err := rejectNullListFields(body, "authorization_servers"); err != nil {
		return nil, err
	}
	// Bind the resource identifier to the requested identifier (the raw caller
	// origin), code-point exact, NO normalization (RFC 9728 §3.3, SPEC.md §16).
	if *raw.Resource != bindResource {
		return nil, &basecamp.Error{
			Code:    basecamp.CodeAPI,
			Message: fmt.Sprintf("resource identifier mismatch: metadata resource %q does not equal %q", *raw.Resource, bindResource),
		}
	}

	return &ProtectedResourceMetadata{
		Resource:             *raw.Resource,
		AuthorizationServers: raw.AuthorizationServers,
	}, nil
}

// DiscoverFromResource is the resource-first discovery orchestrator (SPEC.md
// §16). It composes RFC 9728 + RFC 8414 and applies the stage-sensitive fallback
// state machine.
//
// It returns a DiscoveryResult that is either selected (Config set) or a soft
// fallback (FallbackReason set, one of FallbackResourceDiscoveryFailed or
// FallbackNoASAdvertised). Every hard failure is returned as a *SelectionError
// wrapping a sentinel — callers MUST NOT convert an error into a Launchpad
// request. A malformed caller origin is a usage error and propagates as-is.
func (d *Discoverer) DiscoverFromResource(ctx context.Context, resourceOrigin string, opts ...DiscoverOption) (*DiscoveryResult, error) {
	cfg := newDiscoverConfig(opts)

	// Origin-root validation of the caller's input is a usage error.
	origin, err := requireOriginRoot(resourceOrigin, "resource origin")
	if err != nil {
		return nil, err
	}

	// --- Hop 1: resource metadata. Failure here is soft (before selection). ---
	// Fetch from the normalized origin, bind against the raw caller identifier.
	resource, err := d.fetchProtectedResource(ctx, origin, resourceOrigin, cfg)
	if err != nil {
		var be *basecamp.Error
		if errors.As(err, &be) && be.Code == basecamp.CodeUsage {
			return nil, err
		}
		// A caller cancelling the context (or its deadline expiring) must see that
		// cancellation, never a soft fallback that silently proceeds to Launchpad.
		// fetchDiscoveryDocument derives its own per-fetch timeout as a CHILD
		// context, so a non-nil parent ctx.Err() means the CALLER aborted — not the
		// SDK's internal timeout, which leaves the parent's Err() nil and stays soft.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return &DiscoveryResult{FallbackReason: FallbackResourceDiscoveryFailed}, nil
	}

	var advertised []string
	if resource.AuthorizationServers != nil {
		advertised = *resource.AuthorizationServers
	}

	// --- Selection ---
	var selectedIssuer string
	if cfg.hasExpected {
		selectedIssuer = findAdvertised(advertised, cfg.expectedIssuer)
		if selectedIssuer == "" {
			// api_error (not validation) to match the other four SDKs: an issuer the
			// resource does not advertise is a metadata fault, not a caller-usage one.
			return nil, newSelectionError(ErrExpectedIssuerUnavailable,
				fmt.Sprintf("expected issuer %q is not advertised by the resource", cfg.expectedIssuer), nil)
		}
	} else {
		// Dedupe by code-point: the same non-Launchpad issuer advertised more than
		// once is ONE candidate, not an ambiguity.
		nonLaunchpad := make([]string, 0, len(advertised))
		seen := make(map[string]struct{}, len(advertised))
		for _, s := range advertised {
			if isLaunchpadIssuer(s) {
				continue
			}
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			nonLaunchpad = append(nonLaunchpad, s)
		}
		switch {
		case len(nonLaunchpad) >= 2:
			return nil, newSelectionError(ErrAmbiguousIssuers,
				fmt.Sprintf("multiple non-Launchpad issuers advertised; pass an expected issuer to disambiguate: %s", strings.Join(nonLaunchpad, ", ")), nil)
		case len(nonLaunchpad) == 0:
			// Valid resource metadata omits BC5 — soft fallback (before selection).
			return &DiscoveryResult{FallbackReason: FallbackNoASAdvertised}, nil
		default:
			selectedIssuer = nonLaunchpad[0]
		}
	}

	// --- BC5 is now committed: every subsequent failure is fatal (no Launchpad). ---
	issuerOrigin, err := requireOriginRoot(selectedIssuer, "advertised issuer")
	if err != nil {
		return nil, newSelectionError(ErrInvalidIssuerOrigin,
			fmt.Sprintf("advertised issuer %q is not a valid origin root", selectedIssuer), err)
	}

	// Fetch from the normalized origin, but bind the metadata issuer against the
	// exact advertised string (selectedIssuer), not the normalized origin.
	config, err := d.fetchASMetadata(ctx, issuerOrigin, selectedIssuer, cfg)
	if err != nil {
		if errors.Is(err, errIssuerBindingMismatch) {
			return nil, newSelectionError(ErrIssuerMismatch, err.Error(), err)
		}
		// A caller cancelling (or its deadline expiring) during the committed AS
		// fetch must surface as cancellation, not a misclassified as_fetch_failed
		// api_error. As on the hop-1 path, the parent ctx.Err() is non-nil only for
		// a caller abort — the SDK's own per-fetch timeout is a child context.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, newSelectionError(ErrASFetchFailed,
			fmt.Sprintf("authorization server metadata fetch failed for committed issuer %q: %v", issuerOrigin, err), err)
	}

	return &DiscoveryResult{Config: config, Issuer: config.Issuer}, nil
}

func findAdvertised(advertised []string, want string) string {
	for _, s := range advertised {
		if s == want {
			return s
		}
	}
	return ""
}

// DiscoverLaunchpad fetches OAuth configuration from Basecamp's Launchpad server.
func (d *Discoverer) DiscoverLaunchpad(ctx context.Context, opts ...DiscoverOption) (*Config, error) {
	return d.Discover(ctx, LaunchpadBaseURL, opts...)
}
