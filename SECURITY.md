# Security Guarantees

This document describes the security invariants maintained by the Basecamp SDK across all six implementations (Go, TypeScript, Ruby, Swift, Kotlin, Python).

## Transport Security

### HTTPS Enforcement

All SDK implementations enforce HTTPS for API communication with specific exceptions for local development:

| Context | HTTPS Required | Localhost Exception |
|---------|---------------|---------------------|
| Base URLs | Yes | Yes - for local dev/testing |
| OAuth endpoints | Yes | Yes - for local OAuth testing |
| Webhook payload URLs | Yes | **No** - webhooks are production-only |

Localhost is defined as: `localhost`, any `*.localhost` subdomain (RFC 6761), `127.0.0.1`, or the IPv6 loopback `::1` (also accepted in its bracketed URL form `[::1]`). Host matching is case-insensitive.

**Rationale**: Base URLs and OAuth endpoints may use localhost during development. Webhook payload URLs never allow localhost because webhooks are a server-to-server feature that only makes sense in production contexts.

### Credential Protection

#### Cross-Origin Redirect Handling
Authorization headers are automatically stripped when HTTP redirects cross origin boundaries. This prevents credential leakage to third-party hosts.

#### Same-Origin Credential Attachment
The bearer token is attached only to requests targeting the configured base-URL origin (with a localhost carve-out for development and testing). When a caller supplies an absolute URL as the request path, its origin must match the configured base URL or the request is rejected **before any network call is made** — so the credential can never be sent to a foreign host.

This is enforced at two layers:
- **URL-build chokepoint**: the URL builder rejects absolute URLs whose origin differs from the configured base URL.
- **Token-attach backstop**: immediately before the `Authorization` header is added, the request origin is re-checked, so the invariant holds even if a future code path bypasses the URL builder.

The one intentional exception is the authenticated request to the OAuth **authorization endpoint** — by default Launchpad's (`https://launchpad.37signals.com/authorization.json`). Some SDKs let callers override this endpoint, validated as HTTPS (or localhost) before the token is attached. This is the sole sanctioned cross-origin credentialed request, and it complements the redirect Authorization-stripping and same-origin pagination `Link` validation described above.

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

### Python
- `Client` and `AccountClient` are safe for concurrent use from threads
- `OAuthTokenProvider` uses `threading.Lock` for token refresh operations
- Service accessors are protected by per-AccountClient `threading.Lock`
- Configuration is immutable (frozen `dataclass`)

**Important**: Do not modify configuration after creating a client. Configuration is captured at construction time.

**Breaking Change (Go)**: `Client.Config()` now returns `Config` by value instead of `*Config` pointer. This prevents post-construction modification but may require code changes if callers expected pointer semantics.

## PKCE Support

Go, TypeScript, Ruby, Kotlin, and Python SDKs provide helper utilities for OAuth 2.0 PKCE (Proof Key for Code Exchange):

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

```python
# Python
from basecamp.oauth import generate_pkce, generate_state

pkce = generate_pkce()
# pkce.verifier, pkce.challenge

state = generate_state()
```

**Security properties**:
- Verifiers are 43 characters (32 random bytes, base64url-encoded)
- Challenges are SHA256 hashes of verifiers (use `code_challenge_method=S256`)
- State parameters are 22 characters (16 random bytes) in Go/TypeScript/Ruby/Kotlin, 43 characters (32 random bytes) in Python
- All use cryptographically secure random number generators

## Header Redaction

Go, TypeScript, Ruby, and Python SDKs provide utilities to safely log HTTP requests without exposing credentials:

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

```python
# Python (internal helper — not part of public API)
from basecamp._security import redact_headers

safe = redact_headers(headers)
print(f"Headers: {safe}")
```

**Redacted headers**: `Authorization`, `Cookie`, `Set-Cookie`, `X-CSRF-Token`

## Retry Behavior

Retry eligibility is decided per *operation*, not per HTTP method. `behavior-model.json` classifies
all `259` operations: the 128 GETs are retryable by method, and 88 mutations are flagged <!-- @operation-count -->
`idempotent: true` — all 52 PUTs, all 26 DELETEs, and 10 POSTs (`CompleteTodo`, `PauseQuestion`,
`SubscribeToCardColumn`, `Subscribe`, `EnableCardColumnOnHold`, `CreateBookmark`, `PrioritizeAssignment`,
`SpotlightRecording`, `RecordProjectVisit`, `CreateBubbleUp`). The other 43 POSTs are attempted exactly once. SPEC.md §7 specifies the
three-gate algorithm and the per-SDK divergences.

- **Reads (GET)**: retried with exponential backoff on 429/503 in every SDK. (HEAD is idempotent by method too, but Ruby's transport gates on `method == :get` specifically, so a HEAD would not retry there. The API surface has no HEAD operations today, so this is theoretical.)
- **Naturally-idempotent mutations (PUT/DELETE) and the 10 flagged POSTs**: *are* retried on 429/503
  by Go (generated operation path), Python, TypeScript, Kotlin, and Swift. Retrying these cannot
  duplicate a resource, which is why the gate is idempotency rather than "is it a mutation".
  **Ruby is the sole exception** — its transport retries GET only.
  Go's separate hand-written `pkg/basecamp` HTTP helper is also GET-only.
- **Non-idempotent POSTs**: never retried on 429/503 or network failure, in any SDK — a retry could create a duplicate resource. This does **not** mean such a POST is always attempted exactly once: a 401 that triggers a successful token refresh replays the request once regardless of idempotency, in Ruby and both Python transports (see the 401 table below). Ruby's **raw upload** path is the exception — `post_raw`/`put_raw` (attachments, campfire uploads) go through `single_request_raw`, which raises the mapped error directly and has no refresh-and-replay branch, so those POSTs really are attempted exactly once.
- **Retry-After headers**: respected for 429 responses.

### 401 handling is not uniform

Reactive token refresh — refresh on a 401, then retry the request once — exists in only three
transports. Everywhere else the 401 is surfaced to the caller:

| SDK | 401 behavior |
|-----|--------------|
| Go — hand-written `pkg/basecamp` | Refresh via `AuthManager`, then a single retry (`client.go`) |
| Go — generated `pkg/generated` operations | **No reactive refresh.** `AuthTransport` obtains a token proactively per request via `TokenProvider.AccessToken`. A 401 is **not** returned as an error by the `*WithResponse` variants: they populate `response.JSON401` and return `(response, nil)`, so a caller checking only `err` would read the request as successful. Check `JSON401` (or the status code) explicitly. The `ParseHTTPError` helper is what maps a 401 to `AuthError` |
| Ruby | Refresh, then a single retry (`http.rb`) — **except** the raw upload path: `single_request_raw` (used by `post_raw`/`put_raw` for attachments and campfire uploads) raises without replaying |
| Python (sync **and** async) | Refresh, then a single retry (`_http.py`, `_async_http.py`) |
| TypeScript | No refresh. Generated **service wrappers** convert a 401 into a thrown `BasecampError`; the **raw client** does not — `BasecampClient` extends `RawClient`, so `client.GET(...)` resolves with `{ data: undefined, error }` and never throws. A caller using the raw API with `try`/`catch` alone will miss authentication failures |
| Kotlin | No refresh; raised as `BasecampException.Auth` |
| Swift | No refresh; raised as `BasecampError.auth`. The transport re-runs its auth strategy before each *retry*, but 401 is not a retryable status, so that path is never reached for a 401 |

If you use TypeScript, Kotlin, or Swift, supply a token provider that refreshes proactively, or
handle the 401 and retry at the application layer.

## Reporting Security Issues

If you discover a security vulnerability, please report it through [Basecamp's security page](https://basecamp.com/about/policies/security) or email **security@basecamp.com** rather than opening a public issue. You can also use [GitHub Security Advisories](https://github.com/basecamp/basecamp-sdk/security/advisories) to report privately.
