# Basecamp Swift SDK

[![Swift 6.0](https://img.shields.io/badge/Swift-6.0+-orange.svg)](https://swift.org)
[![Platforms](https://img.shields.io/badge/Platforms-iOS%2016+%20|%20macOS%2012+-blue.svg)](https://developer.apple.com)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)

Official Swift SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

**Upgrading to v0.14.0?** Read [MIGRATING.md](../MIGRATING.md) before you bump the version. The compiler catches the `listUploadVersions` retype and any `switch` over `BasecampError` without a `default` (it gains `case limitExceeded`); what it will not catch is a `switch` **with** a `default`, where storage, project and webhook limit failures — previously a retryable `apiError` — now land silently. Coming from v0.12.0 or earlier, read v0.13.0's section too.

## Features

- Full Swift 6 concurrency support (strict `Sendable` throughout)
- 46 services covering the complete Basecamp API
- Async/await API with structured concurrency
- ETag-based HTTP caching (opt-in)
- Automatic retry with exponential backoff
- Automatic pagination via Link headers
- Structured error enum with exhaustive `switch` matching
- Observability hooks for logging, metrics, and tracing
- Extensible service architecture via `AccountClient` extensions

## Requirements

- Swift 6.0+
- iOS 16+ / macOS 12+

## Installation

In Xcode: **File > Add Package Dependencies**, enter `https://github.com/basecamp/basecamp-sdk`, and choose the version you want.

Or add the package to your `Package.swift`. Replace `MAJOR.MINOR.PATCH` with the version you want — `Package.swift` requires a literal, so this snippet does not compile until you do:

```swift
dependencies: [
    // Replace with the latest release:
    // https://github.com/basecamp/basecamp-sdk/releases/latest
    .package(url: "https://github.com/basecamp/basecamp-sdk", from: "MAJOR.MINOR.PATCH"),
],
targets: [
    .target(
        name: "YourApp",
        dependencies: [
            .product(name: "Basecamp", package: "basecamp-sdk"),
        ]
    ),
]
```

## Getting a token

Every Basecamp API request carries an OAuth 2.0 access token. There is no API key and no personal access token, so even a throwaway script starts here:

1. Choose the grant that matches how your code runs:

| Your integration | Grant | Who refreshes the token |
|---|---|---|
| already holds a token you obtained elsewhere | **static token** — `BasecampClient(accessToken:)` | you do |
| can receive a browser redirect (app with `ASWebAuthenticationSession`, or a local callback server) | **authorization code + PKCE** | your code |
| has no browser, but a person can approve on another device (CLI, headless server, TV) | **device flow** ([RFC 8628](https://www.rfc-editor.org/rfc/rfc8628)) | your code |

The one-line rule: **a redirect URI you control → authorization code; no browser but someone to approve → device flow; a token already in hand → static token.** An unattended daemon or CI job fits none of the three on its own — the device flow needs a person to enter the user code at the verification URI — so provision a token out of band and hand it to the process as a static or refresh token.

2. Get the client credentials that grant needs:

- **Authorization code + PKCE** — register your own integration at **<https://launchpad.37signals.com/integrations>**. You get a client ID, a client secret, and whatever redirect URI you nominated.
- **Device flow** — nothing to register. It runs as the pre-registered public `basecamp-cli` client, which sends no secret, against the device endpoint that discovery returns. Launchpad advertises no device endpoint, so a client you register there is not the one this flow uses.
- **Static token** — nothing to register; you already hold the token.

**This SDK ships no OAuth client.** It consumes tokens; it does not obtain them. `accessToken:` is never refreshed, so once it expires every call fails with `401`. For anything longer-lived, run the flow yourself (or in a companion service) and supply a [`TokenProvider`](#token-providers) that hands back a fresh token on each call — that is the extension point the SDK gives you in place of a built-in flow.

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

```swift
import Basecamp

let client = BasecampClient(
    accessToken: "your-token",
    userAgent: "MyApp/1.0 (you@example.com)"
)

let account = client.forAccount("12345")

// List all projects
let projects = try await account.projects.list()
for project in projects {
    print("\(project.id): \(project.name)")
}
```

## Configuration

```swift
let config = BasecampConfig(
    baseURL: "https://3.basecampapi.com",  // default
    enableRetry: true,                      // default
    enableCache: false,                     // default
    maxPages: 10_000,                       // default
    timeoutInterval: 30                     // default (seconds)
)

let client = BasecampClient(
    accessToken: "your-token",
    userAgent: "MyApp/1.0 (you@example.com)",
    config: config
)
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `baseURL` | `https://3.basecampapi.com` | Basecamp API base URL |
| `enableRetry` | `true` | Automatic retry on 429/503 |
| `enableCache` | `false` | ETag-based HTTP caching |
| `maxPages` | `10_000` | Maximum pages to follow during pagination |
| `timeoutInterval` | `30` | Request timeout in seconds |

### Token Providers

For static tokens, pass a string directly:

```swift
let client = BasecampClient(
    accessToken: "your-token",
    userAgent: "MyApp/1.0 (you@example.com)"
)
```

For token refresh scenarios, use a custom `TokenProvider`:

```swift
let client = BasecampClient(
    tokenProvider: myTokenProvider,
    userAgent: "MyApp/1.0 (you@example.com)"
)
```

For non-Bearer authentication (API keys, cookies, mTLS), use a custom `AuthStrategy`:

```swift
let client = BasecampClient(
    auth: myAuthStrategy,
    userAgent: "MyApp/1.0 (you@example.com)"
)
```

## Services

The SDK exposes 46 account-scoped services. The tables below group the common ones; see `Sources/Basecamp/Generated/Services/` for the full set.

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

## Downloading Files

Fetch an upload's file content in one call. The SDK fetches the upload
metadata, then follows the authenticated-hop + 302 flow against the signed
storage URL.

```swift
let account = client.forAccount("999999999")
let result = try await account.uploads.download(uploadId: 1069479400)
try result.body.write(to: URL(fileURLWithPath: "uploaded.bin"))
// result.contentType, result.contentLength, result.filename are also available
```

For any authenticated download URL (e.g. a `downloadUrl` you already have
in hand), use `AccountClient.downloadURL(_:)`:

```swift
let result = try await account.downloadURL(url)
```

## Pagination

List methods automatically follow Link headers and return all pages:

```swift
// Fetches all pages automatically
let allProjects = try await account.projects.list()
print("Got \(allProjects.count) projects")

// Access pagination metadata
print("Total: \(allProjects.meta.totalCount)")
print("Truncated: \(allProjects.meta.truncated)")
```

### The `page` option

A positive `page` selects exactly that page: one request, that page's items,
no link-following.

```swift
let pageThree = try await account.projects.list(options: .init(page: 3))
print(pageThree.meta.truncated) // true when a further page existed
```

Omit `page` (or pass `0`) to auto-paginate the whole collection. `maxItems`
still trims a pinned page.

All six SDKs share these semantics — one request, that page only, no
link-following. See SPEC section 8.

## Retry Behavior

The SDK automatically retries requests on transient failures:

- **Retryable errors**: 429 (rate limit) and 503 (service unavailable)
- **Backoff**: Exponential with jitter
- **Rate limits**: Respects `Retry-After` header
- **Per-operation config**: Each operation has its own retry settings from the behavior model

Disable retry globally:

```swift
let config = BasecampConfig(enableRetry: false)
```

## Caching

The SDK supports ETag-based HTTP caching. **Caching is disabled by default** to avoid storing private data unexpectedly.

```swift
let config = BasecampConfig(enableCache: true)
let client = BasecampClient(
    accessToken: "your-token",
    userAgent: "MyApp/1.0 (you@example.com)",
    config: config
)

// First request fetches from API
let projects = try await account.projects.list()

// Second request returns cached data if unchanged (304 Not Modified)
let projects2 = try await account.projects.list()
```

## Error Handling

The SDK uses a `BasecampError` enum with associated values for exhaustive `switch` matching:

```swift
do {
    let todo = try await account.todos.get(todoId: 456)
} catch let error as BasecampError {
    switch error {
    case .auth(let message, let hint, _):
        print("Auth failed: \(message)")
    case .forbidden(let message, _, _):
        print("Access denied: \(message)")
    case .notFound(let message, _, _):
        print("Not found: \(message)")
    case .rateLimit(_, let retryAfter, _, _):
        if let seconds = retryAfter {
            try await Task.sleep(nanoseconds: UInt64(seconds) * 1_000_000_000)
        }
    case .network(let message, _):
        print("Network error: \(message)")
    case .api(let message, let status, _, _):
        print("API error (\(status ?? 0)): \(message)")
    case .validation(let message, _, _, _, let fieldErrors):
        print("Validation: \(message)")
        fieldErrors?.forEach { field, messages in
            messages.forEach { print("  \(field) \($0)") }
        }
    case .limitExceeded(let message, _, _):
        // 507. An account limit, not a transient failure — do not retry.
        print("Limit reached: \(message)")
    case .ambiguous(let resource, _, _):
        print("Ambiguous \(resource)")
    case .usage(let message, _):
        print("Usage error: \(message)")
    }

    // Common properties available on all cases
    print("Hint: \(error.hint ?? "none")")
    print("Retryable: \(error.isRetryable)")

    // CLI exit codes (matches Go/TS/Ruby SDKs)
    Foundation.exit(Int32(error.exitCode))
}
```

### Error Cases

| Case | HTTP Status | Exit Code | Description |
|------|-------------|-----------|-------------|
| `.auth` | 401 | 3 | Authentication required |
| `.forbidden` | 403 | 4 | Access denied |
| `.notFound` | 404 | 2 | Resource not found |
| `.rateLimit` | 429 | 5 | Rate limit exceeded (retryable) |
| `.network` | - | 6 | Network error (retryable) |
| `.api` | 500, 502, 503, 504, other 5xx | 7 | Server error |
| `.ambiguous` | - | 8 | Multiple matches found |
| `.validation` | 400, 422 | 9 | Invalid request data |
| `.limitExceeded` | 507 | 10 | Account limit reached (file storage, projects, webhooks) — never retryable |
| `.usage` | - | 1 | Configuration or argument error |

### Validation Errors

Basecamp rejects invalid writes with a body keyed by field. The SDK folds those
messages into `message` and keeps the raw map in the `.validation` case's
`fieldErrors` associated value, also reachable as a property on any
`BasecampError`:

```swift
do {
    try await account.calendars.updateCalendar(
        calendarId: calendarID,
        req: UpdateCalendarRequest(calendar: CalendarAttributes(color: "chartreuse")))
} catch let error as BasecampError {
    print(error.message) // "color: is not a valid color"

    for (field, messages) in error.fieldErrors ?? [:] {
        print("  \(field): \(messages.joined(separator: ", "))")
    }
}
```

`fieldErrors` is `nil` for every other error shape, and its messages are the raw
ones — `message` is capped at 500 characters, the map is not.

## Observability

### Custom Hooks

Implement the `BasecampHooks` protocol. All methods have default no-op implementations, so override only what you need:

```swift
struct LoggingHooks: BasecampHooks {
    func onOperationStart(_ info: OperationInfo) {
        print("\(info.service).\(info.operation) starting")
    }

    func onOperationEnd(_ info: OperationInfo, result: OperationResult) {
        if let error = result.error {
            print("\(info.service).\(info.operation) failed (\(result.durationMs)ms): \(error)")
        } else {
            print("\(info.service).\(info.operation) completed (\(result.durationMs)ms)")
        }
    }

    func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval) {
        print("Retrying \(info.method) \(info.url) (attempt \(attempt), delay \(delaySeconds)s)")
    }
}

let client = BasecampClient(
    accessToken: "your-token",
    userAgent: "MyApp/1.0 (you@example.com)",
    hooks: LoggingHooks()
)
```

### Combining Multiple Hooks

Use `ChainHooks` to compose multiple hooks. Start events fire in order; end events fire in reverse order:

```swift
let client = BasecampClient(
    accessToken: "your-token",
    userAgent: "MyApp/1.0 (you@example.com)",
    hooks: ChainHooks(LoggingHooks(), MetricsHooks())
)
```

### Zero Overhead When Disabled

By default, the SDK uses `NoopHooks` which compiles to empty method bodies — no overhead when observability isn't needed.

## Not Yet Available

- OAuth helpers (discovery, PKCE, token exchange)
- Webhook signature verification

## License

MIT
