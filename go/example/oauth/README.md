# OAuth Example

This example demonstrates the complete OAuth 2.0 authorization flow with PKCE for authenticating with the Basecamp API.

## What it demonstrates

- Generating PKCE code verifier and challenge
- Building an authorization URL with state parameter
- Exchanging authorization codes for tokens
- Secure credential storage (system keyring or encrypted file)
- Using AuthManager for automatic token refresh
- Making authenticated API calls

## Prerequisites

1. A Basecamp account
2. A registered OAuth application

## Registering your OAuth application

1. Go to [Basecamp Integrations](https://launchpad.37signals.com/integrations)
2. Click "Register another application"
3. Fill in the application details:
   - **Name**: Your application name
   - **Company**: Your company name
   - **Website URL**: Your website
   - **Redirect URI**: `urn:ietf:wg:oauth:2.0:oob` (for CLI apps)
4. Save your **Client ID** and **Client Secret**

## Running the example

Set the required environment variables:

```bash
export BASECAMP_CLIENT_ID="your-client-id"
export BASECAMP_CLIENT_SECRET="your-client-secret"
export BASECAMP_ACCOUNT_ID="12345"
```

Then run the example:

```bash
go run main.go
```

## OAuth flow walkthrough

### 1. Generate PKCE

```go
pkce, err := oauth.GeneratePKCE()
```

PKCE (Proof Key for Code Exchange) prevents authorization code interception attacks. It generates:
- **Code Verifier**: A cryptographically random string kept secret
- **Code Challenge**: SHA-256 hash of the verifier, sent with the auth request

### 2. Generate State

```go
state, err := oauth.GenerateState()
```

The state parameter prevents CSRF attacks. Store it before redirecting and verify it matches in the callback.

### 3. Authorization URL

The user visits this URL to grant access:

```
https://launchpad.37signals.com/authorization/new?
  type=web_server&
  client_id=YOUR_CLIENT_ID&
  redirect_uri=urn:ietf:wg:oauth:2.0:oob&
  state=RANDOM_STATE&
  code_challenge=PKCE_CHALLENGE&
  code_challenge_method=S256
```

### 4. Token Exchange

After the user authorizes, exchange the code for tokens:

```go
token, err := exchanger.Exchange(ctx, oauth.ExchangeRequest{
    TokenEndpoint:   tokenEndpoint,
    Code:            code,
    RedirectURI:     redirectURI,
    ClientID:        clientID,
    ClientSecret:    clientSecret,
    CodeVerifier:    pkce.Verifier,  // Proves we started the flow
    UseLegacyFormat: true,           // Basecamp-specific
})
```

### 5. Credential Storage

The SDK stores credentials securely:

```go
authManager.Store().Save(origin, creds)
```

Storage locations:
- **macOS**: Keychain
- **Linux**: Secret Service (GNOME Keyring, KWallet)
- **Windows**: Credential Manager
- **Fallback**: Encrypted file in `~/.config/basecamp/`

### 6. Automatic Refresh

AuthManager automatically refreshes tokens before they expire:

```go
client := basecamp.NewClient(cfg, authManager)
// Tokens are refreshed transparently on each API call
```

## Security best practices

1. **Always use PKCE** - Prevents code interception attacks
2. **Validate state** - Prevents CSRF attacks
3. **Store credentials securely** - Use the built-in credential store
4. **Use HTTPS only** - The SDK enforces HTTPS for all OAuth endpoints
5. **Limit scopes** - Request only the permissions you need

## Web application flow

For web applications, modify the redirect URI and callback handling:

```go
// Use your callback URL instead of OOB
redirectURI = "https://your-app.com/oauth/callback"

// In your callback handler:
func handleCallback(w http.ResponseWriter, r *http.Request) {
    // Verify state matches what you stored in the session
    if r.URL.Query().Get("state") != session.Get("oauth_state") {
        http.Error(w, "Invalid state", http.StatusBadRequest)
        return
    }

    // Exchange the code
    code := r.URL.Query().Get("code")
    token, err := exchanger.Exchange(ctx, oauth.ExchangeRequest{
        Code:         code,
        CodeVerifier: session.Get("oauth_pkce_verifier"),
        // ... other params
    })
}
```

## Troubleshooting

### "Invalid grant" error

The authorization code may have expired or been used already. Codes are single-use and expire quickly.

### "Invalid redirect_uri" error

The redirect URI must exactly match what you registered with your OAuth application.

### Credentials not persisting

Check if the system keyring is available. Set `BASECAMP_NO_KEYRING=1` to force file storage for debugging.
