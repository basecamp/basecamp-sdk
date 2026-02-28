# Basecamp Swift SDK

[![Swift 6.0](https://img.shields.io/badge/Swift-6.0+-orange.svg)](https://swift.org)
[![Platforms](https://img.shields.io/badge/Platforms-iOS%2016+%20|%20macOS%2012+-blue.svg)](https://developer.apple.com)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)

Official Swift SDK for the [Basecamp 3 API](https://github.com/basecamp/bc3-api).

## Features

- Full Swift 6 concurrency support (strict `Sendable` throughout)
- 38 services covering the complete Basecamp API
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

Add the package to your `Package.swift`:

```swift
dependencies: [
    .package(url: "https://github.com/basecamp/basecamp-sdk", from: "0.1.0"),
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

Or add it via Xcode: File > Add Package Dependencies and enter the repository URL.

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

```swift
// Fetches all pages automatically
let allProjects = try await account.projects.list()
print("Got \(allProjects.count) projects")

// Access pagination metadata
print("Total: \(allProjects.meta.totalCount)")
print("Truncated: \(allProjects.meta.truncated)")
```

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
    let todo = try await account.todos.get(projectId: 123, todoId: 456)
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
            try await Task.sleep(for: .seconds(seconds))
        }
    case .network(let message, _):
        print("Network error: \(message)")
    case .api(let message, let status, _, _):
        print("API error (\(status ?? 0)): \(message)")
    case .validation(let message, _, _, _):
        print("Validation: \(message)")
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
| `.validation` | 400, 422 | 1 | Invalid request data |
| `.network` | - | 6 | Network error (retryable) |
| `.api` | 5xx | 7 | Server error |
| `.usage` | - | 1 | Configuration or argument error |

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
