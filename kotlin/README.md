# Basecamp Kotlin SDK

[![Kotlin 2.0+](https://img.shields.io/badge/Kotlin-2.0+-blue.svg)](https://kotlinlang.org)
[![GitHub Packages](https://img.shields.io/badge/GitHub%20Packages-com.basecamp%3Abasecamp--sdk-blue)](https://github.com/basecamp/basecamp-sdk/packages)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)

Official Kotlin SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

## Features

- Kotlin Multiplatform (JVM target)
- Builder DSL for client configuration
- 38 services covering the complete Basecamp API
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

## Installation

The SDK is published to GitHub Packages. Add the repository and dependency to your `build.gradle.kts`:

```kotlin
repositories {
    maven {
        url = uri("https://maven.pkg.github.com/basecamp/basecamp-sdk")
        credentials {
            username = System.getenv("GITHUB_USER") ?: "x-access-token"
            password = System.getenv("GITHUB_ACCESS_TOKEN") ?: ""
        }
    }
}

dependencies {
    implementation("com.basecamp:basecamp-sdk:0.2.1")
}
```

## Quick Start

```kotlin
import com.basecamp.sdk.BasecampClient
import com.basecamp.sdk.generated.projects

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
| `maxRetries` | `3` | Maximum retry attempts |
| `maxPages` | `10_000` | Maximum pages to follow during pagination |
| `baseRetryDelay` | `1s` | Base delay for exponential backoff |

## OAuth 2.0

The SDK includes full OAuth 2.0 support with PKCE for Basecamp's Launchpad identity provider.

### Authorization Flow

```kotlin
import com.basecamp.sdk.oauth.*

// 1. Discover OAuth endpoints
val config = discoverLaunchpad()

// 2. Generate PKCE challenge and state
val pkce = generatePkce()
val state = generateState()
// Store pkce.verifier and state in session

// 3. Build authorization URL
val authUrl = buildString {
    append(config.authorizationEndpoint)
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

## Retry Behavior

The SDK automatically retries requests on transient failures:

- **Retryable errors**: 429 (rate limit) and 503 (service unavailable)
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
    val todo = account.todos.get(projectId = 123, todoId = 456)
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
[Basecamp] Projects.List
[Basecamp] Projects.List completed (147ms)
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
