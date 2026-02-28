# Basecamp Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/basecamp/basecamp-sdk/go.svg)](https://pkg.go.dev/github.com/basecamp/basecamp-sdk/go)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/basecamp/basecamp-sdk/go)](https://goreportcard.com/report/github.com/basecamp/basecamp-sdk/go)

Official Go SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

## Features

- Full coverage of 30+ Basecamp API services
- OAuth 2.0 authentication with automatic token refresh
- Static token authentication for simple integrations
- ETag-based HTTP caching for efficient API usage
- Automatic retry with exponential backoff
- Pagination handling with `GetAll()`
- Structured errors with CLI-friendly exit codes
- Secure credential storage (system keyring with file fallback)

## Installation

```bash
go get github.com/basecamp/basecamp-sdk/go
```

Requires Go 1.25 or later.

## Quick Start

### Using a Static Token

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func main() {
    // Configure the client
    cfg := basecamp.DefaultConfig()

    // Use a static token
    token := &basecamp.StaticTokenProvider{
        Token: os.Getenv("BASECAMP_TOKEN"),
    }

    client := basecamp.NewClient(cfg, token)

    // Get account ID from environment (ForAccount validates it's numeric)
    accountID := os.Getenv("BASECAMP_ACCOUNT_ID")
    if accountID == "" {
        log.Fatal("BASECAMP_ACCOUNT_ID environment variable is required")
    }
    account := client.ForAccount(accountID)

    // List all projects
    projects, err := account.Projects().List(context.Background(), nil)
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range projects {
        fmt.Printf("%d: %s\n", p.ID, p.Name)
    }
}
```

### Using OAuth 2.0

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"

    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func main() {
    cfg := basecamp.DefaultConfig()

    // AuthManager handles token storage and refresh
    authMgr := basecamp.NewAuthManager(cfg, http.DefaultClient)
    client := basecamp.NewClient(cfg, authMgr)

    // Discover available accounts (account-agnostic operation)
    info, err := client.Authorization().GetInfo(context.Background(), nil)
    if err != nil {
        log.Fatal(err)
    }

    // Create an account-scoped client
    account := client.ForAccount(fmt.Sprint(info.Accounts[0].ID))

    // List active projects
    projects, err := account.Projects().List(context.Background(), &basecamp.ProjectListOptions{
        Status: basecamp.ProjectStatusActive,
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range projects {
        fmt.Printf("%s (%d)\n", p.Name, p.ID)
    }
}
```

## Configuration

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `BASECAMP_TOKEN` | Static API token or OAuth access token | Yes (unless using OAuth flow) |
| `BASECAMP_PROJECT_ID` | Default project ID | No |
| `BASECAMP_TODOLIST_ID` | Default todolist ID | No |
| `BASECAMP_BASE_URL` | API base URL | No (default: `https://3.basecampapi.com`) |
| `BASECAMP_CACHE_DIR` | Cache directory path | No (default: `~/.cache/basecamp`) |
| `BASECAMP_CACHE_ENABLED` | Enable HTTP caching | No (default: `false`) |
| `BASECAMP_NO_KEYRING` | Disable system keyring | No |

Note: Account ID is specified via `client.ForAccount(accountID)` rather than configuration.

### Programmatic Configuration

```go
cfg := basecamp.DefaultConfig()
cfg.ProjectID = "67890"           // Optional default project
cfg.CacheEnabled = true           // Enable ETag caching
cfg.CacheDir = "/custom/cache"    // Custom cache location

// Or load from environment
cfg.LoadConfigFromEnv()

// Or load from JSON file
cfg, err := basecamp.LoadConfig("/path/to/config.json")
```

## API Coverage

### Projects & Organization

| Service | Methods |
|---------|---------|
| `Projects()` | List, Get, Create, Update, Trash |
| `Templates()` | List, Get, CreateProject |
| `Tools()` | Get, List, Update (enable/disable/reorder dock tools) |
| `People()` | List, Get, ListPingable, Me, ListProjectPeople |

### To-dos

| Service | Methods |
|---------|---------|
| `Todos()` | List, Get, Create, Update, Trash, Complete, Uncomplete, Reposition |
| `Todosets()` | Get |
| `Todolists()` | List, Get, Create, Update, Trash |
| `TodolistGroups()` | List, Get, Create, Reposition |

### Messages & Communication

| Service | Methods |
|---------|---------|
| `Messages()` | List, Get, Create, Update, Trash |
| `MessageBoards()` | Get |
| `MessageTypes()` | List, Get, Create, Update, Destroy |
| `Comments()` | List, Get, Create, Update, Trash |
| `Campfires()` | List, Get, ListLines, GetLine, CreateLine, DeleteLine, Chatbot CRUD |
| `Forwards()` | List, Get |

### Scheduling

| Service | Methods |
|---------|---------|
| `Schedules()` | Get, ListEntries, GetEntry, CreateEntry, UpdateEntry, TrashEntry, GetEntryOccurrence, UpdateSettings |
| `Lineup()` | List, Get, Create, Update, Delete |
| `Checkins()` | Get, List, ListQuestions, GetQuestion, ListAnswers, GetAnswer, UpdateAnswer |

### Files & Documents

| Service | Methods |
|---------|---------|
| `Vaults()` | Get, List, Create, Update |
| `Attachments()` | CreateUploadURL, Create |

### Card Tables (Kanban)

| Service | Methods |
|---------|---------|
| `CardTables()` | Get, ListColumns, GetColumn |
| `Cards()` | List, Get, Create, Update, Move |
| `CardColumns()` | List, Get, Create, Update, Watch, Unwatch |
| `CardSteps()` | List, Get |

### Reporting & Search

| Service | Methods |
|---------|---------|
| `Timeline()` | Progress, ProjectTimeline, PersonProgress |
| `Reports()` | AssignablePeople, AssignedTodos, OverdueTodos, UpcomingSchedule |
| `Timesheet()` | MyEntries, ProjectEntries |
| `Search()` | Search |
| `Events()` | List, ListForRecording |

### Integrations

| Service | Methods |
|---------|---------|
| `Webhooks()` | List, Get, Create, Update, Delete |
| `Subscriptions()` | List, Subscribe, Unsubscribe, Update |
| `Recordings()` | Archive, Unarchive, Trash |

### Client Portal

| Service | Methods |
|---------|---------|
| `ClientApprovals()` | Get, ListResponses, GetResponse |
| `ClientCorrespondences()` | List, Get, Create, Update, Trash |

## Working with Todos

```go
ctx := context.Background()

// List todos in a todolist
todos, err := account.Todos().List(ctx, todolistID, nil)

// Create a todo
todo, err := account.Todos().Create(ctx, todolistID, &basecamp.CreateTodoRequest{
    Content:     "Review pull request",
    Description: "Check the new authentication flow",
    DueOn:       "2026-02-01",
    AssigneeIDs: []int64{12345},
})

// Complete a todo
err = account.Todos().Complete(ctx, todoID)

// Reposition a todo
err = account.Todos().Reposition(ctx, todoID, 1) // Move to first position
```

## Working with Messages

```go
ctx := context.Background()

// Get the message board (boardID from project dock/tools)
var boardID int64 = 12345
board, err := account.MessageBoards().Get(ctx, boardID)

// List messages
messages, err := account.Messages().List(ctx, board.ID, nil)

// Create a message
msg, err := account.Messages().Create(ctx, board.ID, &basecamp.CreateMessageRequest{
    Subject: "Weekly Update",
    Content: "<p>Here's what we accomplished this week...</p>",
})
```

## Working with Campfire

```go
ctx := context.Background()

// List all campfires
campfires, err := account.Campfires().List(ctx)

// Send a message
line, err := account.Campfires().CreateLine(ctx, campfireID, "Hello, team!")

// List recent messages
lines, err := account.Campfires().ListLines(ctx, campfireID)
```

## Working with Webhooks

```go
ctx := context.Background()
var bucketID int64 = 12345 // project/bucket ID

// Create a webhook
webhook, err := account.Webhooks().Create(ctx, bucketID, &basecamp.CreateWebhookRequest{
    PayloadURL: "https://example.com/webhook",
    Types:      []string{"Todo", "Comment"},
})

// List webhooks
webhooks, err := account.Webhooks().List(ctx, bucketID)

// Delete a webhook
err = account.Webhooks().Delete(ctx, webhookID)
```

## Error Handling

The SDK provides structured errors with codes for programmatic handling:

```go
projects, err := account.Projects().List(ctx, nil)
if err != nil {
    if apiErr, ok := err.(*basecamp.Error); ok {
        switch apiErr.Code {
        case basecamp.CodeNotFound:
            // Handle not found
        case basecamp.CodeAuth:
            // Handle authentication error
        case basecamp.CodeRateLimit:
            // Handle rate limiting (SDK retries automatically)
        case basecamp.CodeForbidden:
            // Handle permission error
        default:
            // Handle other errors
        }

        // Errors include helpful hints
        fmt.Printf("Error: %s\nHint: %s\n", apiErr.Message, apiErr.Hint)

        // Use exit codes for CLI applications
        os.Exit(apiErr.ExitCode())
    }
}
```

### Error Codes

| Code | Meaning | Exit Code |
|------|---------|-----------|
| `usage` | Invalid arguments or configuration | 1 |
| `not_found` | Resource not found | 2 |
| `auth_required` | Authentication required | 3 |
| `forbidden` | Access denied | 4 |
| `rate_limit` | Rate limited (retryable) | 5 |
| `network` | Network error (retryable) | 6 |
| `api_error` | Server error | 7 |
| `ambiguous` | Multiple matches found | 8 |
| `validation` | Validation error (400, 422) | 9 |

## Caching

The SDK supports ETag-based caching for GET responses. **Caching is disabled by default** to avoid writing private data to disk unexpectedly.

To enable caching:

```go
cfg := basecamp.DefaultConfig()
cfg.CacheEnabled = true

// Or via environment variable:
// BASECAMP_CACHE_ENABLED=true
```

When enabled, the SDK caches GET responses using ETags:

```go
// First request fetches from API
projects, _ := account.Projects().List(ctx, nil)

// Second request uses cached data if unchanged (304 Not Modified)
projects, _ = account.Projects().List(ctx, nil)
```

## Custom HTTP Client

```go
httpClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns: 50,
    },
}

client := basecamp.NewClient(cfg, token, basecamp.WithHTTPClient(httpClient))
```

## Observability

The SDK provides a hooks interface for observability at two levels:

- **Operation-level**: Semantic SDK operations like `Todos.Complete`, `Projects.List`
- **Request-level**: HTTP requests including retries, caching, and timing

### Debug Logging with SlogHooks

For debugging or verbose CLI modes, use `SlogHooks` to log all SDK activity:

```go
import (
    "log/slog"
    "os"
    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// Create a debug logger
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

// Enable observability hooks
hooks := basecamp.NewSlogHooks(logger)
client := basecamp.NewClient(cfg, token, basecamp.WithHooks(hooks))
```

Output:
```
level=DEBUG msg="basecamp operation start" service=Todos operation=Complete resource_type=todo is_mutation=true
level=DEBUG msg="basecamp request start" method=POST url=https://3.basecampapi.com/123/todos/789/completion.json attempt=1
level=DEBUG msg="basecamp request complete" method=POST url=... duration=145ms status=204 from_cache=false
level=DEBUG msg="basecamp operation complete" service=Todos operation=Complete duration=147ms
```

### OpenTelemetry Integration

For distributed tracing and metrics with OTel:

```go
import (
    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
    basecampotel "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/otel"
)

// Uses global TracerProvider/MeterProvider by default
hooks := basecampotel.NewHooks()
client := basecamp.NewClient(cfg, token, basecamp.WithHooks(hooks))

// Or with custom providers
hooks := basecampotel.NewHooks(
    basecampotel.WithTracerProvider(tp),
    basecampotel.WithMeterProvider(mp),
)
```

Creates spans like:
- `Todos.Complete` (operation span)
  - `basecamp.request` (HTTP span, child of operation)

### Prometheus Metrics

For Prometheus-style metrics:

```go
import (
    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
    basecampprom "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/prometheus"
    "github.com/prometheus/client_golang/prometheus"
)

hooks := basecampprom.NewHooks(prometheus.DefaultRegisterer)
client := basecamp.NewClient(cfg, token, basecamp.WithHooks(hooks))
```

Exposes metrics:
| Metric | Type | Labels |
|--------|------|--------|
| `basecamp_operation_duration_seconds` | Histogram | `operation` |
| `basecamp_operations_total` | Counter | `operation`, `status` |
| `basecamp_http_requests_total` | Counter | `http_method`, `status_code` |
| `basecamp_retries_total` | Counter | `http_method` |
| `basecamp_cache_operations_total` | Counter | `result` |
| `basecamp_errors_total` | Counter | `http_method`, `type` |

### Combining Multiple Backends

Use `NewChainHooks` to send telemetry to multiple backends:

```go
import (
    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
    basecampotel "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/otel"
    basecampprom "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/prometheus"
)

otelHooks := basecampotel.NewHooks()
promHooks := basecampprom.NewHooks(prometheus.DefaultRegisterer)

client := basecamp.NewClient(cfg, token,
    basecamp.WithHooks(basecamp.NewChainHooks(otelHooks, promHooks)),
)
```

### Custom Hooks

Implement the `Hooks` interface for custom behavior. Embed `NoopHooks` to only override what you need:

```go
type AlertingHooks struct {
    basecamp.NoopHooks
}

func (h *AlertingHooks) OnRetry(ctx context.Context, info basecamp.RequestInfo, attempt int, err error) {
    if attempt >= 3 {
        alertOncall(fmt.Sprintf("Basecamp API struggling: %s %s attempt %d", info.Method, info.URL, attempt))
    }
}

hooks := &AlertingHooks{}
client := basecamp.NewClient(cfg, token, basecamp.WithHooks(hooks))
```

### Zero Overhead When Disabled

By default, the SDK uses `NoopHooks` which compiles to nothing—no overhead when observability isn't needed.

## Logging

Enable HTTP-level debug logging with a custom `slog` logger:

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

client := basecamp.NewClient(cfg, token, basecamp.WithLogger(logger))
```

For semantic operation logging (recommended), use `SlogHooks` instead—see [Observability](#observability) above.

## License

MIT
