// Copyright 2025 Basecamp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The oauth command demonstrates OAuth 2.0 authentication with PKCE.
// It shows how to:
//   - Generate PKCE code verifier and challenge
//   - Build an authorization URL
//   - Exchange an authorization code for tokens
//   - Use AuthManager for automatic token refresh
//
// This is the recommended authentication flow for production applications.
package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/oauth"
)

// OAuth configuration - replace with your own values.
// Register your application at https://launchpad.37signals.com/integrations
const (
	// The authorization endpoint for Basecamp OAuth.
	authorizationEndpoint = "https://launchpad.37signals.com/authorization/new"

	// The token endpoint for exchanging codes and refreshing tokens.
	tokenEndpoint = "https://launchpad.37signals.com/authorization/token" //nolint:gosec // Not a credential, just an endpoint URL

	// Redirect URI for CLI applications (out-of-band).
	// For web apps, use your callback URL instead.
	redirectURI = "urn:ietf:wg:oauth:2.0:oob"
)

func main() {
	// Get OAuth credentials from environment.
	clientID := os.Getenv("BASECAMP_CLIENT_ID")
	clientSecret := os.Getenv("BASECAMP_CLIENT_SECRET")
	accountID := os.Getenv("BASECAMP_ACCOUNT_ID")

	if clientID == "" || clientSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: OAuth credentials required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Set these environment variables:")
		fmt.Fprintln(os.Stderr, "  BASECAMP_CLIENT_ID     - Your OAuth client ID")
		fmt.Fprintln(os.Stderr, "  BASECAMP_CLIENT_SECRET - Your OAuth client secret")
		fmt.Fprintln(os.Stderr, "  BASECAMP_ACCOUNT_ID    - Your Basecamp account ID")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Register an application at https://launchpad.37signals.com/integrations")
		os.Exit(1)
	}

	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Error: BASECAMP_ACCOUNT_ID environment variable is required")
		os.Exit(1)
	}

	// Check if we already have a stored token.
	cfg := basecamp.DefaultConfig()
	httpClient := &http.Client{Timeout: 30 * time.Second}
	authManager := basecamp.NewAuthManager(cfg, httpClient)

	if authManager.IsAuthenticated() {
		fmt.Println("Already authenticated! Using stored credentials.")
		fmt.Println()
		useStoredCredentials(cfg, authManager, accountID)
		return
	}

	// Start the OAuth flow with PKCE.
	fmt.Println("Starting OAuth 2.0 authorization flow with PKCE...")
	fmt.Println()

	// Step 1: Generate PKCE code verifier and challenge.
	// PKCE (Proof Key for Code Exchange) prevents authorization code interception attacks.
	pkce, err := oauth.GeneratePKCE()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating PKCE: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Generate a random state parameter to prevent CSRF attacks.
	state, err := oauth.GenerateState()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating state: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Build the authorization URL.
	// The user will visit this URL to grant access to your application.
	authURL := buildAuthorizationURL(clientID, state, pkce.Challenge)

	fmt.Println("Please visit this URL to authorize the application:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Println("After authorizing, you'll receive an authorization code.")
	fmt.Print("Enter the authorization code: ")

	// Read the authorization code from user input.
	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
	code = strings.TrimSpace(code)

	if code == "" {
		fmt.Fprintln(os.Stderr, "Error: Authorization code is required")
		os.Exit(1)
	}

	// Step 4: Exchange the authorization code for tokens.
	// The PKCE verifier proves we're the same client that started the flow.
	fmt.Println()
	fmt.Println("Exchanging authorization code for tokens...")

	exchanger := oauth.NewExchanger(httpClient)
	token, err := exchanger.Exchange(context.Background(), oauth.ExchangeRequest{
		TokenEndpoint:   tokenEndpoint,
		Code:            code,
		RedirectURI:     redirectURI,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		CodeVerifier:    pkce.Verifier,
		UseLegacyFormat: true, // Basecamp uses non-standard token format
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error exchanging code: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully obtained tokens!")
	fmt.Printf("  Access Token:  %s\n", truncateToken(token.AccessToken))
	fmt.Printf("  Refresh Token: %s\n", truncateToken(token.RefreshToken))
	if !token.ExpiresAt.IsZero() {
		fmt.Printf("  Expires At:    %s\n", token.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Println()

	// Step 5: Store the credentials securely.
	// The SDK stores credentials in the system keyring (macOS Keychain, etc.)
	// or falls back to an encrypted file.
	creds := &basecamp.Credentials{
		AccessToken:   token.AccessToken,
		RefreshToken:  token.RefreshToken,
		ExpiresAt:     token.ExpiresAt.Unix(),
		TokenEndpoint: tokenEndpoint,
	}

	origin := basecamp.NormalizeBaseURL(cfg.BaseURL)
	if err := authManager.Store().Save(origin, creds); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not store credentials: %v\n", err)
		fmt.Fprintln(os.Stderr, "You'll need to re-authenticate next time.")
	} else {
		fmt.Println("Credentials stored securely.")
		if authManager.Store().UsingKeyring() {
			fmt.Println("  Storage: System keyring")
		} else {
			fmt.Println("  Storage: Encrypted file")
		}
	}
	fmt.Println()

	// Step 6: Use the authenticated client.
	useStoredCredentials(cfg, authManager, accountID)
}

// truncateToken safely truncates a token for display, showing only the first
// and last few characters. Returns the full token if it's too short to truncate.
func truncateToken(token string) string {
	const minLen = 14 // 10 prefix + 4 suffix
	if len(token) < minLen {
		return token
	}
	return token[:10] + "..." + token[len(token)-4:]
}

// buildAuthorizationURL constructs the OAuth authorization URL.
func buildAuthorizationURL(clientID, state, codeChallenge string) string {
	params := url.Values{}
	params.Set("type", "web_server")            // Basecamp uses "type" instead of "response_type"
	params.Set("client_id", clientID)           // Your application's client ID
	params.Set("redirect_uri", redirectURI)     // Where to redirect after authorization
	params.Set("state", state)                  // CSRF protection
	params.Set("code_challenge", codeChallenge) // PKCE challenge
	params.Set("code_challenge_method", "S256") // SHA-256 challenge method

	return authorizationEndpoint + "?" + params.Encode()
}

// useStoredCredentials demonstrates using AuthManager for API calls.
func useStoredCredentials(cfg *basecamp.Config, authManager *basecamp.AuthManager, accountID string) {
	// Create a client using AuthManager as the token provider.
	// AuthManager automatically handles token refresh when tokens expire.
	client := basecamp.NewClient(cfg, authManager)
	account := client.ForAccount(accountID)

	// Demonstrate that the authentication works.
	fmt.Println("Fetching projects to verify authentication...")
	fmt.Println()

	ctx := context.Background()
	result, err := account.Projects().List(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing projects: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d project(s):\n", len(result.Projects))
	for i, project := range result.Projects {
		fmt.Printf("  %d. %s\n", i+1, project.Name)
	}

	if len(result.Projects) == 0 {
		fmt.Println("  (No projects found)")
	}

	fmt.Println()
	fmt.Println("Authentication successful!")

	// Demonstrate token refresh (if the token is close to expiring).
	fmt.Println()
	fmt.Println("Note: The SDK automatically refreshes tokens when they expire.")
	fmt.Println("You don't need to handle token refresh manually.")
}
