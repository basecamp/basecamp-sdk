// Package oauth provides OAuth 2.0 discovery, token exchange, and refresh functionality.
package oauth

import "time"

// Config represents an OAuth 2.0 server configuration from discovery.
type Config struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint,omitempty"`
	ScopesSupported       []string `json:"scopes_supported,omitempty"`
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
