# Basecamp Kotlin SDK

[![Kotlin 2.0+](https://img.shields.io/badge/Kotlin-2.0+-blue.svg)](https://kotlinlang.org)
[![GitHub Packages](https://img.shields.io/badge/GitHub%20Packages-com.basecamp%3Abasecamp--sdk-blue)](https://github.com/basecamp/basecamp-sdk/packages)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)

Official Kotlin SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

**Upgrading to v0.13.0?** Read [MIGRATING.md](../MIGRATING.md#kotlin) before you bump the version — three of this release's breaking changes give you no signal at all: no compile error, no exception, no decoder failure.

## Features

- Kotlin Multiplatform (JVM target)
- Builder DSL for client configuration
- 46 services covering the complete Basecamp API
- OAuth 2.0 with PKCE support
- Webhook signature verification (HMAC-SHA256)
- ETag-based HTTP caching (opt-in)
- Automatic retry with exponential backoff
- Automatic pagination via Link headers
- Sealed class error hierarchy with exhaustive `when` matching
- Observability hooks for logging, metrics, and tracing
- Built on Ktor and kotlinx.serialization

## Requirements

- JDK 17+
- Kotlin 2.0+

**Compatibility policy (pre-1.0).** Releases in the 0.x series guarantee
*source* compatibility only: public APIs evolve append-only (new optional
parameters are added after existing ones), so code compiles unchanged across
minor versions, but recompile against each release — Kotlin default-argument
and data-class synthetics make JVM *binary* compatibility infeasible to
promise, and we don't. One exception: when Basecamp withdraws an endpoint,
the SDK removes the corresponding operation rather than keeping a stub whose
only possible response is an error. Such removals track the server, ship in
a minor version bump, and are called out in the release notes.

Generated options classes are data classes with defaults, so their constructor
positions are part of that promise. The generator pins the shipped order per
class in `sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/options-param-order.json`
and appends new parameters after it, so a parameter added to an operation can
never displace one you already pass positionally.
## Installation

The SDK is published to [GitHub Packages](https://github.com/basecamp/basecamp-sdk/packages). GitHub Packages requires an access token for every download — including for public packages like this one — so there are three steps rather than one.

### 1. Create an access token

Create a [**classic** personal access token](https://github.com/settings/tokens) with the `read:packages` scope. Fine-grained personal access tokens do not work with GitHub Packages.

### 2. Store the credentials

Put them in `~/.gradle/gradle.properties`, so they stay out of your repository:

```properties
gpr.user=YOUR_GITHUB_USERNAME
gpr.key=YOUR_CLASSIC_TOKEN
```

The repository block below also reads the `GITHUB_USER` and `GITHUB_ACCESS_TOKEN` environment variables. Those names are this project's own convention, not GitHub Actions defaults — Actions gives you `github.actor` and `secrets.GITHUB_TOKEN` — so a workflow has to map them:

```yaml
env:
  GITHUB_USER: x-access-token
  GITHUB_ACCESS_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

`secrets.GITHUB_TOKEN` is scoped to the repository running the workflow. To consume the package from a *different* repository, grant that repository access under the package's Actions access settings and give the job `permissions: packages: read` — that avoids a long-lived PAT. Failing that, store a classic PAT as a secret and use it as `GITHUB_ACCESS_TOKEN`.

The username is not load-bearing; GitHub Packages authenticates on the token. `${{ github.actor }}` works just as well as `x-access-token`.

### 3. Declare the repository and dependency

In your `build.gradle.kts`:

```kotlin
repositories {
    mavenCentral()
    maven {
        url = uri("https://maven.pkg.github.com/basecamp/basecamp-sdk")
        credentials {
            username = project.findProperty("gpr.user") as String? ?: System.getenv("GITHUB_USER")
            password = project.findProperty("gpr.key") as String? ?: System.getenv("GITHUB_ACCESS_TOKEN")
        }
    }
}

dependencies {
    // Replace VERSION with the latest release:
    // https://github.com/basecamp/basecamp-sdk/releases/latest
    implementation("com.basecamp:basecamp-sdk:VERSION")
}
```

`mavenCentral()` is needed for the SDK's own dependencies — Ktor, kotlinx.serialization, and the Kotlin stdlib all resolve from there.

### Maven

Maven needs a different artifact, plus its own repository declaration and credentials.

Depend on **`basecamp-sdk-jvm`**, not `basecamp-sdk`. Gradle reads the Gradle Module Metadata that ships alongside the root `com.basecamp:basecamp-sdk` artifact and transparently redirects to the JVM variant. Maven does not read that metadata, so it resolves the root jar directly — which *succeeds*, and puts a Kotlin Multiplatform metadata jar containing no classes on your classpath. Nothing fails until compile time.

```xml
<dependency>
  <groupId>com.basecamp</groupId>
  <artifactId>basecamp-sdk-jvm</artifactId>
  <!-- Replace with the latest release: https://github.com/basecamp/basecamp-sdk/releases/latest -->
  <version>VERSION</version>
</dependency>
```

Declare the repository in the same `pom.xml`:

```xml
<repositories>
  <repository>
    <id>github</id>
    <url>https://maven.pkg.github.com/basecamp/basecamp-sdk</url>
  </repository>
</repositories>
```

And put the credentials in `~/.m2/settings.xml`, where `<id>` must match the repository's:

```xml
<settings>
  <servers>
    <server>
      <id>github</id>
      <username>YOUR_GITHUB_USERNAME</username>
      <password>YOUR_CLASSIC_TOKEN</password>
    </server>
  </servers>
</settings>
```

### Troubleshooting

| Error | Cause |
|---|---|
| `Could not find com.basecamp:basecamp-sdk` | The `maven { }` repository block is missing. |
| `Username must not be null!` | Neither `gpr.user` nor `GITHUB_USER` is set. |
| `Received status code 401 from server: Unauthorized` | The token is wrong, expired, fine-grained rather than classic, or missing the `read:packages` scope. |

## Getting a token

The access token in the snippets below is a **Basecamp API** token. It is unrelated to the GitHub token above, which only downloads the artifact.

Every Basecamp API request carries an OAuth 2.0 access token. There is no API key and no personal access token, so even a throwaway script starts here:

1. Choose the grant that matches how your code runs:

| Your integration | Grant | Who refreshes the token |
|---|---|---|
| already holds a token you obtained elsewhere | **static token** — `accessToken("…")` | you do |
| can receive a browser redirect (web app, or a local callback server) | **authorization code + PKCE** — [`Authorization Flow`](#authorization-flow) | you, calling the SDK's `refreshToken()` from `accessToken { … }` |
| has no browser, but a person can approve on another device (CLI, headless server, TV) | **device flow** — [`Device Authorization Flow`](#device-authorization-flow-rfc-8628) | you, calling the SDK's `refreshToken()` from `accessToken { … }` |

The one-line rule: **a redirect URI you control → authorization code; no browser but someone to approve → device flow; a token already in hand → static token.** An unattended daemon or CI job fits none of the three on its own — the device flow needs a person to enter the user code at the verification URI — so provision a token out of band and hand it to the process as a static or refresh token.

2. Get the client credentials that grant needs:

- **Authorization code + PKCE** — register your own integration at **<https://launchpad.37signals.com/integrations>**. You get a client ID, a client secret, and whatever redirect URI you nominated.
- **Device flow** — nothing to register. It runs as the pre-registered public `basecamp-cli` client, which sends no secret, against the device endpoint that discovery returns. Launchpad advertises no device endpoint, so a client you register there is not the one this flow uses.
- **Static token** — nothing to register; you already hold the token.

`accessToken("…")` hands back that exact string forever and never refreshes, so once the token expires every call fails with `401`. The lambda form, `accessToken { fetchFreshToken() }`, is re-invoked per request — that is where a refresh belongs.

## Finding your account ID

Every API path is scoped to an account — `https://3.basecampapi.com/{accountId}/…` — so `forAccount` needs that number before your first call. One token can reach several accounts.

This SDK has no `authorization` service, because that endpoint lives on the authorization server rather than on the Basecamp API. Fetch it once yourself, from the server that issued your token:

```bash
# Launchpad-issued token. For a device-flow token, replace the host with the
# issuer discovery selected — that is where its authorization.json lives.
curl -s https://launchpad.37signals.com/authorization.json \
  -H "Authorization: Bearer $BASECAMP_TOKEN" \
  -H "User-Agent: MyApp/1.0 (you@example.com)"
```

Take `accounts[].id` for an entry whose `product` is `"bc3"` — that is Basecamp; the same response also carries `"hey"` and other 37signals products. `expires_at` tells you how long the token has left. A `User-Agent` identifying your app is required on every Basecamp request, including this one.

## Quick Start

Service accessors are extension properties, so `account.projects` needs the `com.basecamp.sdk.generated` import as well as the client's. Every service method is `suspend`, so the calls need a coroutine.

```kotlin
import com.basecamp.sdk.BasecampClient
import com.basecamp.sdk.generated.projects

suspend fun main() {
    val client = BasecampClient {
        accessToken("your-token")
        userAgent = "MyApp/1.0 (you@example.com)"
    }

    val account = client.forAccount("12345")

    // List all projects
    val projects = account.projects.list()
    for (project in projects) {
        println("${project.id}: ${project.name}")
    }

    // Clean up when done
    client.close()
}
```

## Configuration

```kotlin
val client = BasecampClient {
    // Authentication (required — pick one)
    accessToken("your-token")             // static token
    accessToken { fetchFreshToken() }     // dynamic token provider
    auth(myCustomAuthStrategy)            // custom auth strategy

    // Options (all optional)
    baseUrl = "https://3.basecampapi.com" // default
    userAgent = "MyApp/1.0"              // default: basecamp-sdk-kotlin/VERSION
    enableRetry = true                    // default
    enableCache = false                   // default
    hooks = consoleHooks()                // default: NoopHooks

    // Advanced
    engine = MockEngine { ... }           // custom Ktor engine (testing)
    httpClient = myKtorClient             // pre-configured Ktor HttpClient
}
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `baseUrl` | `https://3.basecampapi.com` | Basecamp API base URL |
| `userAgent` | `BasecampConfig.DEFAULT_USER_AGENT` | User-Agent header |
| `enableRetry` | `true` | Automatic retry on 429/503 |
| `enableCache` | `false` | ETag-based HTTP caching |
| `timeout` | `30s` | Request timeout |
| `maxRetries` | `3` | Maximum retry attempts (fixed; not settable via the builder) |
| `maxPages` | `10_000` | Maximum pages to follow during pagination |
| `baseRetryDelay` | `1s` | Base delay for exponential backoff (fixed; not settable via the builder) |

## OAuth 2.0

The SDK includes full OAuth 2.0 support with PKCE for Basecamp's Launchpad identity provider.

### Discovery

Three composable operations follow the resource-first model (RFC 9728 + RFC 8414):

```kotlin
import com.basecamp.sdk.oauth.*

// Direct RFC 8414 Authorization Server metadata (issuer bound by code-point).
val config = discover("https://launchpad.37signals.com")

// Resource-first discovery: start from the API/resource host, select the
// advertised authorization server, and fall back to Launchpad when appropriate.
when (val result = discoverFromResource("https://3.basecampapi.com")) {
    is DiscoveryResult.Selected -> useConfig(result.config)          // a BC5 AS was committed
    is DiscoveryResult.FallBack -> useConfig(discoverLaunchpad())    // reason: resource_discovery_failed | no_as_advertised
}
```

`discoverFromResource` returns a `DiscoveryResult` for the two soft outcomes and
**throws** `BasecampException.DiscoverySelection` for every hard failure (e.g.
`ambiguous_issuers`, `issuer_mismatch`, `as_fetch_failed`) once a BC5 issuer is
committed — a hard failure is never converted into a Launchpad request. Pass an
`expectedIssuer` to select authoritatively instead of by exclusion.

All discovery fetches are SSRF-hardened: HTTPS-only origins (localhost exempt),
origin validated with the transport URL parser before any socket opens, redirects
suppressed, timeouts bounded, and response bodies read under a bounded cap.

`OAuthConfig.authorizationEndpoint` is **nullable** (`String?`): device-only
authorization servers omit it, so authorization-code consumers must assert its
presence before use (`token_endpoint` is always present).

### Authorization Flow

```kotlin
import com.basecamp.sdk.oauth.*

// 1. Discover OAuth endpoints
val config = discoverLaunchpad()

// 2. Generate PKCE challenge and state
val pkce = generatePkce()
val state = generateState()
// Store pkce.verifier and state in session

// 3. Build authorization URL (authorizationEndpoint is nullable; assert presence)
val authorizationEndpoint = requireNotNull(config.authorizationEndpoint) {
    "Authorization server does not support the authorization-code flow"
}
val authUrl = buildString {
    append(authorizationEndpoint)
    append("?type=web_server")
    append("&client_id=$CLIENT_ID")
    append("&redirect_uri=$REDIRECT_URI")
    append("&state=$state")
    append("&code_challenge=${pkce.challenge}")
    append("&code_challenge_method=S256")
}
// Redirect user to authUrl

// 4. Exchange code for tokens (in callback handler)
val token = exchangeCode(
    tokenEndpoint = config.tokenEndpoint,
    code = callbackCode,
    redirectUri = REDIRECT_URI,
    clientId = CLIENT_ID,
    clientSecret = CLIENT_SECRET,
    codeVerifier = pkce.verifier,
    useLegacyFormat = true,  // required for Launchpad
)

// 5. Create client with the token
val client = BasecampClient {
    accessToken(token.accessToken)
    userAgent = "MyApp/1.0"
}

// 6. Refresh when expired
if (isTokenExpired(token)) {
    val newToken = refreshToken(
        tokenEndpoint = config.tokenEndpoint,
        refreshToken = token.refreshToken!!,
        clientId = CLIENT_ID,
        clientSecret = CLIENT_SECRET,
        useLegacyFormat = true,
    )
}
```

### Device Authorization Flow (RFC 8628)

For input-constrained clients (CLIs, TVs) the SDK implements the OAuth 2.0 device
authorization grant. The public `basecamp-cli` client is pre-registered with
`token_endpoint_auth_method: none` — it sends no client secret, and an omitted
scope defaults to `read` (prefer pinning it explicitly with `scope = "read"`).
Pass bare origins everywhere (no trailing slash): binding is code-point exact
— a trailing-slash `expectedIssuer` fails the advertised-member lookup as a
**hard** `expected_issuer_unavailable`, while a trailing-slash resource origin
breaks the hop-1 binding and silently soft-falls back to Launchpad.

```kotlin
import com.basecamp.sdk.BasecampClient
import com.basecamp.sdk.oauth.*

// 1. Resource-first discovery selects the authorization server. Device flow needs
//    an AS that advertises the device_code grant AND a
//    device_authorization_endpoint — Launchpad advertises neither, so obtain the
//    config from discoverFromResource rather than discoverLaunchpad().
val config = when (val result = discoverFromResource("https://3.basecampapi.com")) {
    is DiscoveryResult.Selected -> result.config
    is DiscoveryResult.FallBack ->
        error("This authorization server does not offer device flow (${result.reason.code})")
}

// 2. Run the full flow: request a code, show it to the user, then poll for the
//    token. performDeviceLogin guards capability, so it throws
//    BasecampException.DeviceFlow(reason = "unavailable") when the server can't
//    do device flow.
val token = performDeviceLogin(
    config = config,
    clientId = "basecamp-cli",
    display = { auth ->
        println("Visit ${auth.verificationUri} and enter code ${auth.userCode}")
        // Or open auth.verificationUriComplete directly when present.
    },
)

// Use the token to build a client — never print or log its value.
val client = BasecampClient {
    accessToken(token.accessToken)
}

// BC5 device logins as basecamp-cli mint MULTI-ACCOUNT refresh tokens: the
// token carries an RFC 8707 resource indicator (token.resource,
// "urn:bc:account:<id>"), and refreshing without echoing it is rejected
// (400 invalid_request). Persist token.resource alongside the tokens and
// echo it on refresh:
// A device-token response MAY omit refresh_token — GUARD it rather than
// force-unwrapping: without one, refreshing is impossible and the user must
// re-run the device login when the access token expires.
val storedRefresh = token.refreshToken
if (storedRefresh != null) {
    val fresh = refreshToken(
        tokenEndpoint = config.tokenEndpoint,
        refreshToken = storedRefresh,
        clientId = "basecamp-cli",  // public client — no secret
        resource = token.resource,
    )
    // A refresh response MAY omit refresh_token and resource — persist
    // `fresh.refreshToken ?: storedRefresh` and `fresh.resource ?: token.resource`
    // so the next refresh still works and still echoes the binding.
}
```

The two building blocks are also public if you need finer control:

```kotlin
import kotlin.time.Duration.Companion.seconds
import kotlin.time.TimeSource

// deviceAuthorizationEndpoint is optional (device-only servers advertise it,
// others omit it), so assert its presence rather than dereferencing with `!!`.
val deviceEndpoint = config.deviceAuthorizationEndpoint
    ?: error("Selected authorization server does not offer device flow")

val auth = requestDeviceAuthorization(deviceEndpoint, "basecamp-cli")

// Anchor the expiry deadline at code issuance, BEFORE showing the code, so time
// the user spends reading it counts against the code's lifetime rather than
// resetting the clock. (performDeviceLogin does this for you.)
val issuedAt = TimeSource.Monotonic.markNow()
println("Enter ${auth.userCode} at ${auth.verificationUri}")

// Polls until approval, denial, or expiry. The wait honours the server interval,
// a sustained slow_down (+5s), and the monotonic expiry deadline.
val token = pollDeviceToken(
    tokenEndpoint = config.tokenEndpoint,
    clientId = "basecamp-cli",
    deviceCode = auth.deviceCode,
    interval = auth.interval,
    expiresIn = auth.expiresIn,
    deadline = issuedAt + auth.expiresIn.seconds,
)
```

Polling is cancellation-aware: cancel the enclosing coroutine (job/scope) to stop
it — the `CancellationException` propagates untouched. Terminal *flow* outcomes
(the user denied, the code expired, a transport failure, or the server can't do
device flow) surface as `BasecampException.DeviceFlow`, whose `reason`
(`access_denied`, `expired`, `transport`, `unavailable`) derives the parent error
`code` (`auth_required`, `network`, `validation`). Protocol faults are *not*
`DeviceFlow`: a malformed 2xx token response (unparseable body or an empty
`access_token`) and an unrecognized RFC 8628 error code both surface as
`BasecampException.Api`. `refreshToken`'s `clientSecret` is optional so public
clients like `basecamp-cli` can refresh without a secret.

## Webhook Verification

Verify incoming webhook signatures using HMAC-SHA256:

```kotlin
import com.basecamp.sdk.webhooks.verifyWebhookSignature

// In your webhook handler
val isValid = verifyWebhookSignature(
    payload = requestBody,
    signature = request.headers["X-Basecamp-Signature"]!!,
    secret = webhookSecret,
)

if (!isValid) {
    return respond(HttpStatusCode.Unauthorized)
}
```

## Services

The SDK exposes 46 account-scoped services. The tables below group the common ones; see `com/basecamp/sdk/generated/services/` for the full set.

### Projects & Organization

| Service | Description |
|---------|-------------|
| `projects` | Project management |
| `templates` | Project templates |
| `tools` | Project dock tools |
| `people` | People and users |

### To-dos

| Service | Description |
|---------|-------------|
| `todos` | Todo items |
| `todolists` | Todo lists |
| `todosets` | Todo set containers |
| `todolistGroups` | Todolist grouping/folders |

### Messages & Communication

| Service | Description |
|---------|-------------|
| `messages` | Message posts |
| `messageBoards` | Message boards |
| `messageTypes` | Message categories |
| `comments` | Comments on recordings |
| `campfires` | Chat rooms |
| `forwards` | Email forwards |

### Card Tables (Kanban)

| Service | Description |
|---------|-------------|
| `cardTables` | Card tables |
| `cards` | Card table cards |
| `cardColumns` | Card table columns |
| `cardSteps` | Card workflow steps |
| `wormholes` | Card table wormholes (cross-project moves) |

### Scheduling

| Service | Description |
|---------|-------------|
| `schedules` | Calendar schedules |
| `lineup` | Card lineup view |
| `checkins` | Automatic check-ins |

### Files & Documents

| Service | Description |
|---------|-------------|
| `vaults` | File folders |
| `documents` | Documents |
| `uploads` | File uploads |
| `attachments` | Binary attachments |

### Integrations & Events

| Service | Description |
|---------|-------------|
| `webhooks` | Webhook subscriptions |
| `subscriptions` | Notification subscriptions |
| `events` | Activity events |
| `recordings` | Generic recordings |
| `boosts` | Boosts / reactions |

### Search & Reports

| Service | Description |
|---------|-------------|
| `search` | Full-text search |
| `reports` | Activity reports |
| `timeline` | Activity timeline |
| `timesheets` | Time tracking reports |

### Client Portal

| Service | Description |
|---------|-------------|
| `clientApprovals` | Client approval workflows |
| `clientCorrespondences` | Client communications |
| `clientReplies` | Client replies |
| `clientVisibility` | Client visibility settings |

## Pagination

List methods automatically follow Link headers and return all pages:

```kotlin
// Fetches all pages automatically
val allProjects = account.projects.list()
println("Got ${allProjects.size} projects")

// Access pagination metadata
println("Total: ${allProjects.meta.totalCount}")
println("Truncated: ${allProjects.meta.truncated}")

// ListResult implements List<T>, so all collection operations work
allProjects.forEach { println(it.name) }
```

`meta.truncated` is `true` only when items beyond those returned were available — items were dropped by `maxItems`, or the last-fetched page still advertised a next page when collection stopped; when it is `false`, the result is definitely complete.

### The `page` option

A positive `page` selects exactly that page: one request, that page's items,
no link-following.

```kotlin
val pageThree = account.projects.list(ListProjectsOptions(page = 3))
println(pageThree.meta.truncated) // true when a further page existed
```

Omit `page` (or pass `0`) to auto-paginate the whole collection. `maxItems`
still trims a pinned page.

All six SDKs share these semantics — one request, that page only, no
link-following. See SPEC section 8.

## Retry Behavior

The SDK automatically retries requests on transient failures:

- **Retryable errors**: 429 (rate limit) and 503 (service unavailable)
- **Network errors**: transport-level failures (connection refused/reset, DNS, connect/socket timeouts) retry for retry-eligible operations — GET/PUT/DELETE/HEAD, plus POSTs marked idempotent; non-idempotent POSTs are attempted exactly once. The request timeout is not retried: an attempt that consumed its whole time budget is slowness a retry tends to repeat, and each retry would burn another full budget.
- **Backoff**: Exponential with jitter
- **Rate limits**: Respects `Retry-After` header
- **Max retries**: 3 attempts by default

Disable retry:

```kotlin
val client = BasecampClient {
    accessToken("your-token")
    enableRetry = false
}
```

## Caching

The SDK supports ETag-based HTTP caching. **Caching is disabled by default** to avoid storing private data unexpectedly.

```kotlin
val client = BasecampClient {
    accessToken("your-token")
    enableCache = true
}

// First request fetches from API
val projects = account.projects.list()

// Second request returns cached data if unchanged (304 Not Modified)
val projects2 = account.projects.list()
```

## Error Handling

The SDK uses a `BasecampException` sealed class for exhaustive `when` matching:

```kotlin
import com.basecamp.sdk.BasecampException

try {
    val todo = account.todos.get(todoId = 456)
} catch (e: BasecampException) {
    when (e) {
        is BasecampException.Auth -> println("Token expired: ${e.message}")
        is BasecampException.Forbidden -> println("Access denied: ${e.message}")
        is BasecampException.NotFound -> println("Not found: ${e.message}")
        is BasecampException.RateLimit -> println("Retry in ${e.retryAfterSeconds}s")
        is BasecampException.Validation -> println("Invalid input: ${e.message}")
        is BasecampException.Ambiguous -> println("Ambiguous: ${e.message}")
        is BasecampException.Network -> println("Network error: ${e.message}")
        is BasecampException.Api -> println("Server error (${e.httpStatus}): ${e.message}")
        is BasecampException.Usage -> println("Bad arguments: ${e.message}")
        is BasecampException.DiscoverySelection -> println("OAuth discovery: ${e.reason}")
        is BasecampException.DeviceFlow -> println("Device flow: ${e.reason}")
    }

    // Common properties available on all subclasses
    println("Hint: ${e.hint}")
    println("Retryable: ${e.retryable}")

    // CLI exit codes (matches Go/TS/Ruby/Swift SDKs)
    kotlin.system.exitProcess(e.exitCode)
}
```

### Error Types

| Type | HTTP Status | Exit Code | Description |
|------|-------------|-----------|-------------|
| `Auth` | 401 | 3 | Authentication required |
| `Forbidden` | 403 | 4 | Access denied |
| `NotFound` | 404 | 2 | Resource not found |
| `RateLimit` | 429 | 5 | Rate limit exceeded (retryable) |
| `Network` | - | 6 | Network error (retryable) |
| `Api` | 5xx | 7 | Server error |
| `Ambiguous` | - | 8 | Multiple matches found |
| `Validation` | 400, 422 | 9 | Invalid request data |
| `Usage` | - | 1 | Configuration or argument error |
| `DiscoverySelection` | - | 7 or 9 | OAuth discovery selection failed (code derived from `reason`) |
| `DeviceFlow` | - | 1, 3, 6, or 9 | Device authorization grant failed (code derived from `reason`) |

### Validation Errors

Basecamp rejects invalid writes with a body keyed by field. The SDK folds those
messages into `message` and keeps the raw map in `fieldErrors`, so you can drive
a form without re-parsing the message:

```kotlin
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

try {
    account.calendars.updateCalendar(
        calendarId,
        UpdateCalendarBody(calendar = buildJsonObject { put("color", "chartreuse") }),
    )
} catch (e: BasecampException.Validation) {
    println(e.message) // "color: is not a valid color"

    e.fieldErrors?.forEach { (field, messages) ->
        messages.forEach { println("  $field $it") }
    }
}
```

`fieldErrors` is `null` for every other error shape, and its messages are the
raw ones — `message` is capped at 500 characters, the map is not.

## Observability

### Console Logging

For debugging or development:

```kotlin
val client = BasecampClient {
    accessToken("your-token")
    hooks = consoleHooks(
        logOperations = true,   // default
        logRequests = false,    // more verbose
        logRetries = true,      // default
    )
}
```

Output:
```
[Basecamp] Projects.ListProjects
[Basecamp] Projects.ListProjects completed (147ms)
```

### Custom Hooks

Implement the `BasecampHooks` interface. All methods have default no-op implementations:

```kotlin
val metricsHooks = object : BasecampHooks {
    override fun onOperationEnd(info: OperationInfo, result: OperationResult) {
        metrics.record("${info.service}.${info.operation}", result.duration)
        if (result.error != null) {
            metrics.incrementError("${info.service}.${info.operation}")
        }
    }

    override fun onRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
        logger.warn("Retrying ${info.method} ${info.url} (attempt $attempt)")
    }
}

val client = BasecampClient {
    accessToken("your-token")
    hooks = metricsHooks
}
```

### Combining Multiple Hooks

Use `chainHooks` to compose multiple hooks. Start events fire in order; end events fire in reverse order:

```kotlin
val client = BasecampClient {
    accessToken("your-token")
    hooks = chainHooks(
        consoleHooks(),
        metricsHooks,
        tracingHooks,
    )
}
```

### Zero Overhead When Disabled

By default, the SDK uses `NoopHooks` (a singleton object) — no overhead when observability isn't needed.

## License

MIT
