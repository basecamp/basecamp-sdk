# Security Guarantees

This document describes the security invariants maintained by the Basecamp SDK across all five implementations (Go, TypeScript, Ruby, Swift, Kotlin).

## Transport Security

### HTTPS Enforcement

All SDK implementations enforce HTTPS for API communication with specific exceptions for local development:

| Context | HTTPS Required | Localhost Exception |
|---------|---------------|---------------------|
| Base URLs | Yes | Yes - for local dev/testing |
| OAuth endpoints | Yes | Yes - for local OAuth testing |
| Webhook payload URLs | Yes | **No** - webhooks are production-only |

Localhost is defined as: `localhost`, `127.0.0.1`, or `::1`.

**Rationale**: Base URLs and OAuth endpoints may use localhost during development. Webhook payload URLs never allow localhost because webhooks are a server-to-server feature that only makes sense in production contexts.

### Credential Protection

#### Cross-Origin Redirect Handling
Authorization headers are automatically stripped when HTTP redirects cross origin boundaries. This prevents credential leakage to third-party hosts.

#### Pagination Security
Link headers from paginated responses are validated for same-origin before following. This prevents:
- SSRF attacks via poisoned Link headers
- Token leakage to attacker-controlled servers

#### Cache Isolation
Cache keys include a hash of the authorization token to isolate cached responses per-credential. This prevents:
- Cross-user cache poisoning
- Stale responses after token refresh

## Response Handling

### Size Limits

| Context | Limit | Purpose |
|---------|-------|---------|
| General responses | 50 MB | Prevent memory exhaustion from large payloads |
| Error bodies | 1 MB | Limit parsing overhead for error responses |
| OAuth token responses | 1 MB | Prevent DoS during authentication |
| Error messages | 500 chars | Prevent information leakage in logs/errors |

### Error Message Truncation

Error messages extracted from API responses are truncated to 500 characters before being included in exceptions. This prevents:
- Sensitive data in error messages from being logged
- Unbounded memory growth from malformed error responses

## Concurrency Safety

All SDK clients are safe for concurrent use after construction. Thread/goroutine safety guarantees:

### Go
- `Client` and `AccountClient` are safe for concurrent use
- `AuthManager` uses mutex protection for all credential operations
- Service accessors are protected by per-AccountClient mutex

### TypeScript
- Service accessors use nullish coalescing for atomic initialization
- Token hash computation uses promise coalescing to prevent duplicate crypto operations
- ETag cache uses Map for thread-safe (single-threaded JS) access

### Ruby
- `OauthTokenProvider` uses mutex for token refresh operations
- The `refresh` method holds mutex during the entire check-and-refresh operation

### Swift
- `BasecampClient` and `AccountClient` are marked `Sendable` for Swift 6 strict concurrency
- All service properties are safe for concurrent access via actor isolation
- Configuration is immutable (`let` properties on `BasecampConfig`)

### Kotlin
- `BasecampClient` is safe for concurrent use from coroutines
- Ktor's `HttpClient` handles connection pooling and thread safety internally
- Configuration is immutable (`val` properties on `BasecampConfig` data class)

**Important**: Do not modify configuration after creating a client. Configuration is captured at construction time.

**Breaking Change (Go)**: `Client.Config()` now returns `Config` by value instead of `*Config` pointer. This prevents post-construction modification but may require code changes if callers expected pointer semantics.

## PKCE Support

Go, TypeScript, Ruby, and Kotlin SDKs provide helper utilities for OAuth 2.0 PKCE (Proof Key for Code Exchange):

```go
// Go
pkce, err := oauth.GeneratePKCE()
// pkce.Verifier, pkce.Challenge

state, err := oauth.GenerateState()
```

```typescript
// TypeScript
const pkce = await generatePKCE();
// pkce.verifier, pkce.challenge

const state = generateState();
```

```ruby
# Ruby
pkce = Basecamp::Oauth::Pkce.generate
# pkce[:verifier], pkce[:challenge]

state = Basecamp::Oauth::Pkce.generate_state
```

```kotlin
// Kotlin
val pkce = Pkce.generate()
// pkce.verifier, pkce.challenge

val state = Pkce.generateState()
```

**Security properties**:
- Verifiers are 43 characters (32 random bytes, base64url-encoded)
- Challenges are SHA256 hashes of verifiers (use `code_challenge_method=S256`)
- State parameters are 22 characters (16 random bytes, base64url-encoded)
- All use cryptographically secure random number generators

## Header Redaction

Go, TypeScript, and Ruby SDKs provide utilities to safely log HTTP requests without exposing credentials:

```go
// Go
safeHeaders := basecamp.RedactHeaders(req.Header)
logger.Info("request", "headers", safeHeaders)
```

```typescript
// TypeScript
const safeHeaders = redactHeaders(response.headers);
console.log("Response headers:", safeHeaders);
```

```ruby
# Ruby
safe = Basecamp::Security.redact_headers(headers)
logger.info("Headers: #{safe}")
```

**Redacted headers**: `Authorization`, `Cookie`, `Set-Cookie`, `X-CSRF-Token`

## Retry Behavior

The SDKs implement safe retry behavior:

- **GET requests**: Automatically retried with exponential backoff on 429/503
- **Mutations (POST/PUT/DELETE)**: NOT automatically retried on 429/503 to prevent duplicate operations
- **401 responses**: Token refresh attempted, then single retry for all methods
- **Retry-After headers**: Respected for 429 responses

## Reporting Security Issues

If you discover a security vulnerability, please report it through [Basecamp's security page](https://basecamp.com/about/policies/security) or email **security@basecamp.com** rather than opening a public issue. You can also use [GitHub Security Advisories](https://github.com/basecamp/basecamp-sdk/security/advisories) to report privately.
