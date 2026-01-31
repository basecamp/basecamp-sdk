# Basecamp SDK

Official SDKs for the [Basecamp 3 API](https://github.com/basecamp/bc3-api).

## Languages

| Language | Path | Status | Package |
|----------|------|--------|---------|
| [Go](go/) | `go/` | Active | `github.com/basecamp/basecamp-sdk/go` |
| [Ruby](ruby/) | `ruby/` | Active | `basecamp-sdk` |
| [TypeScript](typescript/) | `typescript/` | Active | `@basecamp/sdk` |

All SDKs are generated from a single [Smithy](https://smithy.io/) specification, ensuring consistent behavior and API coverage across languages.

## Quick Start

### Go

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func main() {
    cfg := basecamp.DefaultConfig()
    token := &basecamp.StaticTokenProvider{Token: os.Getenv("BASECAMP_TOKEN")}
    client := basecamp.NewClient(cfg, token)

    account := client.ForAccount(os.Getenv("BASECAMP_ACCOUNT_ID"))
    projects, _ := account.Projects().List(context.Background(), nil)

    for _, p := range projects {
        fmt.Printf("%d: %s\n", p.ID, p.Name)
    }
}
```

### Ruby

```ruby
require "basecamp"

client = Basecamp.client(access_token: ENV["BASECAMP_TOKEN"])
account = client.for_account(ENV["BASECAMP_ACCOUNT_ID"])

account.projects.list.each do |project|
  puts "#{project['id']}: #{project['name']}"
end
```

### TypeScript

```typescript
import { createBasecampClient } from "@basecamp/sdk";

const client = createBasecampClient({
  accountId: process.env.BASECAMP_ACCOUNT_ID!,
  accessToken: process.env.BASECAMP_TOKEN!,
});

const projects = await client.projects.list();
projects.forEach(p => console.log(`${p.id}: ${p.name}`));
```

## Features

All SDKs provide:

- **Full API coverage** - 35+ services covering projects, todos, messages, schedules, campfires, card tables, and more
- **OAuth 2.0 authentication** - Token refresh, PKCE support, and static token options
- **Automatic retry** - Exponential backoff with jitter, respects `Retry-After` headers
- **Pagination** - Automatic handling via Link headers
- **ETag caching** - Built-in HTTP caching for efficient API usage
- **Structured errors** - Typed errors with helpful hints and CLI-friendly exit codes
- **Observability hooks** - Integration points for logging, metrics, and tracing

## API Coverage

| Category | Services |
|----------|----------|
| **Projects** | Projects, Templates, Tools, People |
| **To-dos** | Todos, Todolists, Todosets, TodolistGroups |
| **Messages** | Messages, MessageBoards, MessageTypes, Comments |
| **Chat** | Campfires (lines, chatbots) |
| **Scheduling** | Schedules, Timeline, Lineup, Checkins |
| **Files** | Vaults, Documents, Uploads, Attachments |
| **Card Tables** | CardTables, Cards, CardColumns, CardSteps |
| **Client Portal** | ClientApprovals, ClientCorrespondences, ClientReplies |
| **Automation** | Webhooks, Subscriptions, Events |
| **Reporting** | Search, Reports, Timesheets, Recordings |

## Specification

The [`spec/`](spec/) directory contains the API specification in [Smithy IDL](https://smithy.io/) format. This specification drives:

- OpenAPI generation for client codegen
- Type definitions across all SDKs
- Consistent behavior modeling (pagination, retries, idempotency)

See the [spec README](spec/README.md) for details on the model structure.

## Documentation

- [Go SDK documentation](go/README.md) - Full API reference with examples
- [Ruby SDK documentation](ruby/README.md) - Gem usage and configuration
- [TypeScript SDK documentation](typescript/) - npm package usage
- [Contributing guide](CONTRIBUTING.md) - Development setup and guidelines
- [Security policy](SECURITY.md) - Reporting vulnerabilities

## Environment Variables

All SDKs support common environment variables:

| Variable | Description |
|----------|-------------|
| `BASECAMP_TOKEN` | OAuth access token |
| `BASECAMP_ACCOUNT_ID` | Basecamp account ID |
| `BASECAMP_BASE_URL` | API base URL (default: `https://3.basecampapi.com`) |

See individual SDK documentation for language-specific options.

## License

MIT
