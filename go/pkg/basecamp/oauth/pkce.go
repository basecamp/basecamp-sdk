package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCE holds a code verifier and its corresponding challenge for OAuth 2.0 PKCE flow.
type PKCE struct {
	// Verifier is the cryptographically random code_verifier to send during token exchange.
	Verifier string
	// Challenge is the SHA256 hash of the verifier, base64url-encoded, to send during authorization.
	Challenge string
}

// GeneratePKCE returns a cryptographically secure PKCE code verifier and its SHA256 code challenge.
// The verifier is 43 characters (32 random bytes, base64url-encoded without padding).
// The challenge is the base64url-encoded SHA256 hash of the verifier.
//
// Use the Challenge with code_challenge_method=S256 in the authorization request,
// and the Verifier in the token exchange request.
//
// Example:
//
//	pkce, err := oauth.GeneratePKCE()
//	if err != nil {
//	    return err
//	}
//	authURL := fmt.Sprintf("%s?code_challenge=%s&code_challenge_method=S256", baseURL, pkce.Challenge)
//	// Later, during token exchange:
//	token, err := oauth.Exchange(ctx, code, pkce.Verifier, ...)
func GeneratePKCE() (*PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}

	verifier := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	return &PKCE{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}

// GenerateState returns a cryptographically secure OAuth state parameter.
// The state is 22 characters (16 random bytes, base64url-encoded without padding).
//
// Use this to prevent CSRF attacks on the OAuth flow. Store the state before
// redirecting to the authorization endpoint, and verify it matches when
// handling the callback.
//
// Example:
//
//	state, err := oauth.GenerateState()
//	if err != nil {
//	    return err
//	}
//	session.Set("oauth_state", state)
//	authURL := fmt.Sprintf("%s?state=%s", baseURL, state)
//	// Later, in the callback handler:
//	if r.URL.Query().Get("state") != session.Get("oauth_state") {
//	    return errors.New("state mismatch")
//	}
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
