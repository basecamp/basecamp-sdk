// Package oauth provides OAuth 2.0 discovery, token exchange, and refresh functionality.
package oauth

import (
	"errors"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// Config represents an OAuth 2.0 authorization server configuration (RFC 8414).
type Config struct {
	// Issuer is the authorization server's issuer identifier. It equals the URL
	// the metadata was retrieved from, code-point exact (RFC 8414 §3.3/§4).
	Issuer string `json:"issuer"`

	// AuthorizationEndpoint is the URL of the authorization endpoint.
	//
	// Optional as of BC5 resource-first discovery: device-only authorization
	// servers omit it, so absent (nil) and present-empty are preserved
	// distinctly. Authorization-code consumers MUST assert its presence before
	// use.
	AuthorizationEndpoint *string `json:"authorization_endpoint,omitempty"`

	// TokenEndpoint is the URL of the token endpoint (required).
	TokenEndpoint string `json:"token_endpoint"`

	// DeviceAuthorizationEndpoint is the URL of the RFC 8628 device
	// authorization endpoint (optional; nil when absent).
	DeviceAuthorizationEndpoint *string `json:"device_authorization_endpoint,omitempty"`

	// RegistrationEndpoint is the URL of the dynamic client registration
	// endpoint (optional).
	RegistrationEndpoint string `json:"registration_endpoint,omitempty"`

	// GrantTypesSupported lists the OAuth 2.0 grant types the server supports.
	GrantTypesSupported []string `json:"grant_types_supported,omitempty"`

	// ScopesSupported lists the OAuth 2.0 scopes the server supports.
	ScopesSupported []string `json:"scopes_supported,omitempty"`

	// CodeChallengeMethodsSupported lists the PKCE code challenge methods the
	// server supports.
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`
}

// ProtectedResourceMetadata is RFC 9728 protected-resource metadata (hop 1 of
// resource-first discovery).
type ProtectedResourceMetadata struct {
	// Resource is the resource identifier; it equals the requested resource
	// origin, code-point exact.
	Resource string `json:"resource"`

	// AuthorizationServers lists the authorization servers advertised for this
	// resource.
	//
	// It is a pointer slice so absent (nil) and present-but-empty (non-nil,
	// len 0) stay distinct: BC5 omits the key while dark (RFC 9728 §3.2). Both
	// nonetheless select Launchpad, but the distinction is meaningful to callers
	// inspecting metadata.
	AuthorizationServers *[]string `json:"authorization_servers,omitempty"`
}

// FallbackReason is a soft resource-first fallback outcome — the ONLY two
// outcomes under which DiscoverFromResource yields a fallback (Launchpad) rather
// than a selected config. Every other failure is a hard, sentinel-wrapped error.
type FallbackReason string

const (
	// FallbackResourceDiscoveryFailed indicates hop-1 resource metadata could
	// not be fetched, parsed, or bound before any BC5 issuer was committed.
	FallbackResourceDiscoveryFailed FallbackReason = "resource_discovery_failed"
	// FallbackNoASAdvertised indicates valid resource metadata advertised no
	// non-Launchpad issuer (absent / empty / only-Launchpad).
	FallbackNoASAdvertised FallbackReason = "no_as_advertised"
)

// DiscoveryResult is the outcome of DiscoverFromResource: either a selected
// authorization-server config, or a soft fallback to Launchpad. Hard failures
// are returned as sentinel-wrapped errors, never represented here.
type DiscoveryResult struct {
	// Config is the selected authorization-server config; nil on fallback.
	Config *Config
	// Issuer is the selected issuer identifier; empty on fallback.
	Issuer string
	// FallbackReason is non-empty when discovery fell back to Launchpad.
	FallbackReason FallbackReason
}

// IsFallback reports whether the result is a soft fallback to Launchpad rather
// than a selected authorization-server config.
func (r *DiscoveryResult) IsFallback() bool {
	return r.FallbackReason != ""
}

// Sentinel errors for the hard resource-first selection/validation failures.
// These are returned wrapped in a *SelectionError; match them with errors.Is.
// A hard failure MUST NOT be converted by any consumer into a Launchpad request.
var (
	// ErrAmbiguousIssuers is returned when two or more non-Launchpad issuers are
	// advertised and no expected issuer was provided to disambiguate.
	ErrAmbiguousIssuers = errors.New("ambiguous issuers advertised")
	// ErrExpectedIssuerUnavailable is returned when an expected issuer was
	// provided but is not advertised by the resource.
	ErrExpectedIssuerUnavailable = errors.New("expected issuer not advertised")
	// ErrInvalidIssuerOrigin is returned when a selected advertised issuer is
	// not a valid origin root.
	ErrInvalidIssuerOrigin = errors.New("advertised issuer is not a valid origin root")
	// ErrASFetchFailed is returned when the authorization-server metadata fetch
	// fails (5xx / network) for a committed BC5 issuer.
	ErrASFetchFailed = errors.New("authorization server metadata fetch failed")
	// ErrIssuerMismatch is returned when a committed BC5 issuer's metadata does
	// not bind to the advertised issuer (code-point mismatch).
	ErrIssuerMismatch = errors.New("issuer binding mismatch")
	// ErrCapabilityUnavailable is returned when a committed BC5 issuer lacks a
	// per-grant endpoint/capability the consumer requires.
	ErrCapabilityUnavailable = errors.New("required capability unavailable")
)

// errIssuerBindingMismatch is the internal marker attached to an AS-metadata
// binding failure so DiscoverFromResource can distinguish an issuer mismatch
// from a generic fetch failure without matching on message text.
var errIssuerBindingMismatch = errors.New("oauth: issuer binding mismatch")

// SelectionError is a hard resource-first selection/validation failure. It wraps
// one of the sentinel errors above (match with errors.Is) and carries a SDK
// error code so it maps to the standard error taxonomy (errors.As a
// *basecamp.Error). It is returned — never yielded as a fallback — so no
// consumer can convert it into a Launchpad request.
type SelectionError struct {
	// Reason is the sentinel error identifying the failure class.
	Reason error
	// Code is the SDK error taxonomy code, derived from Reason: every hard
	// discovery failure is basecamp.CodeAPI except ErrCapabilityUnavailable
	// (consumer-asserted), which is basecamp.CodeValidation — matching the
	// other four SDKs.
	Code string
	// Message is the human-readable description.
	Message string
	// Cause is the underlying error, if any.
	Cause error
}

// Error implements the error interface.
func (e *SelectionError) Error() string {
	return e.Message
}

// Unwrap exposes the sentinel reason, a taxonomy-coded *basecamp.Error, and the
// underlying cause for errors.Is / errors.As traversal.
//
// The taxonomy-coded view keeps e.Code (the SDK's classification of the failure)
// but inherits the cause's HTTP status and retryability when the cause carries a
// *basecamp.Error — e.g. an AS 5xx fetch (ErrASFetchFailed) or a network failure
// on the committed-issuer hop. Without this, an errors.As(&basecamp.Error)
// consumer would match a stripped {Code, Message} that hid the underlying status
// and retryable flag. The sentinel reason stays first so errors.Is keeps working.
func (e *SelectionError) Unwrap() []error {
	coded := &basecamp.Error{Code: e.Code, Message: e.Message}
	var cause *basecamp.Error
	if errors.As(e.Cause, &cause) {
		coded.HTTPStatus = cause.HTTPStatus
		coded.Retryable = cause.Retryable
		coded.RequestID = cause.RequestID
	}

	errs := make([]error, 0, 3)
	errs = append(errs, e.Reason, coded)
	if e.Cause != nil {
		errs = append(errs, e.Cause)
	}
	return errs
}

// selectionErrorCode derives the taxonomy code from the sentinel reason. All hard
// discovery failures are api_error except capability_unavailable (validation).
func selectionErrorCode(reason error) string {
	if errors.Is(reason, ErrCapabilityUnavailable) {
		return basecamp.CodeValidation
	}
	return basecamp.CodeAPI
}

func newSelectionError(reason error, message string, cause error) *SelectionError {
	return &SelectionError{Reason: reason, Code: selectionErrorCode(reason), Message: message, Cause: cause}
}

// Token represents an OAuth 2.0 access token response.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	ExpiresAt    time.Time `json:"-"`
	Scope        string    `json:"scope,omitempty"`
}

// ExchangeRequest contains parameters for exchanging an authorization code for tokens.
type ExchangeRequest struct {
	TokenEndpoint string
	Code          string
	RedirectURI   string
	ClientID      string
	ClientSecret  string
	CodeVerifier  string

	// UseLegacyFormat uses Launchpad's non-standard token format:
	// type=web_server instead of grant_type=authorization_code
	UseLegacyFormat bool
}

// RefreshRequest contains parameters for refreshing an access token.
type RefreshRequest struct {
	TokenEndpoint string
	RefreshToken  string
	ClientID      string
	ClientSecret  string

	// UseLegacyFormat uses Launchpad's non-standard token format:
	// type=refresh instead of grant_type=refresh_token
	UseLegacyFormat bool
}
