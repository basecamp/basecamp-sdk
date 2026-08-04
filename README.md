# <img src="assets/basecamp-badge.svg" height="28" alt="Basecamp"> Basecamp SDK

Official [Basecamp](https://basecamp.com) [API](https://github.com/basecamp/bc3-api) clients, runtimes, and software development kits for Go, Ruby, TypeScript, Swift, Kotlin, and Python.

OpenAPI 3.1 spec included.

**Upgrading?** Read [MIGRATING.md](MIGRATING.md) *before* you bump the version. v0.13.0 breaks all six SDKs, and a large share of those breaks are silent — no compile error, no exception, no decoder failure.

## Languages

| Language | Path | Status | Package |
|----------|------|--------|---------|
| [Go](go/) | `go/` | Active | `github.com/basecamp/basecamp-sdk/go` |
| [Ruby](ruby/) | `ruby/` | Active | `basecamp-sdk` |
| [TypeScript](typescript/) | `typescript/` | Active | `@37signals/basecamp` |
| [Swift](swift/) | `swift/` | Active | `Basecamp` (SPM) |
| [Kotlin](kotlin/) | `kotlin/` | Active | `com.basecamp:basecamp-sdk` (GitHub Packages) |
| [Python](python/) | `python/` | Active | `basecamp-sdk` (PyPI) |

| Feature | Go | TypeScript | Ruby | Swift | Kotlin | Python |
|---------|:--:|:----------:|:----:|:-----:|:------:|:------:|
| OAuth 2.0 Authentication | ✓ | ✓ | ✓ | ✗ | ✓ | ✓ |
| Static Token Authentication | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| ETag HTTP Caching (opt-in) | ✓ | ✓ | via Faraday† | ✓ | ✓ | ✗ |
| Automatic Retry with Backoff | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Pagination Handling | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Observability Hooks | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Structured Errors | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Webhook Verification | ✓ | ✓ | ✓ | ✗ | ✓ | ✓ |

† Ruby SDK uses Faraday - add caching via [faraday-http-cache](https://github.com/sourcelevel/faraday-http-cache)

**Note:** HTTP caching is disabled by default. Enable explicitly via configuration:
- **Go:** `cfg.CacheEnabled = true`, or `BASECAMP_CACHE_ENABLED=true` plus a `cfg.LoadConfigFromEnv()` call
- **TypeScript:** `enableCache: true` in client options
- **Swift:** `BasecampConfig(enableCache: true)`
- **Kotlin:** `enableCache = true` in builder DSL

All SDKs are generated from a single [Smithy](https://smithy.io/) specification, ensuring consistent behavior and API coverage across languages.

## Getting a token

Every Basecamp API request carries an OAuth 2.0 access token. There is no API key and no personal access token, so even a throwaway script starts here:

1. Choose the grant that matches how your code runs:

| Your integration | Grant | Who refreshes the token |
|---|---|---|
| already holds a token you obtained elsewhere | **static token** | you do |
| can receive a browser redirect (web app, or a local callback server) | **authorization code + PKCE** | a refreshing token provider (built in for Go, Ruby, Python; wire it yourself in TypeScript and Kotlin) |
| has no browser, but a person can approve on another device (CLI, headless server, TV) | **device flow** ([RFC 8628](https://www.rfc-editor.org/rfc/rfc8628)) | Go's `AuthManager`; in Ruby and Python the standalone refresh helper, **not** their built-in token providers (see below); wire it yourself in TypeScript and Kotlin |

The one-line rule: **a redirect URI you control → authorization code; no browser but someone to approve → device flow; a token already in hand → static token.** An unattended daemon or CI job fits none of the three on its own — the device flow needs a person to enter the user code at the verification URI — so provision a token out of band and hand it to the process as a static or refresh token.

2. Get the client credentials that grant needs:

- **Authorization code + PKCE** — register your own integration at **<https://launchpad.37signals.com/integrations>**. You get a client ID, a client secret, and whatever redirect URI you nominated.
- **Device flow** — nothing to register. It runs as the pre-registered public `basecamp-cli` client, which sends no secret, against the device endpoint that discovery returns. Launchpad advertises no device endpoint, so a client you register there is not the one this flow uses.
- **Static token** — nothing to register; you already hold the token.

A device-flow token needs a matching refresh path. BC5 device logins mint multi-account refresh tokens carrying an RFC 8707 `resource` indicator, and a refresh that does not echo it is rejected with `400 invalid_request`. Go's `AuthManager` refreshes against the stored token endpoint and echoes `resource`, so it handles this. Ruby's `OauthTokenProvider` and Python's `OAuthTokenProvider` do **not** — both are pinned to Launchpad's legacy token URL, send no `resource`, and expect a client secret the public client does not have. In those two, refresh a device token with `Basecamp::Oauth.refresh_token` / `basecamp.oauth.exchange.refresh_token`, passing the stored `resource`.

A static token is the shortest path to a first successful call, and it is the one option the SDK will never refresh for you — once it expires, every request fails with `401` until you supply a new one. The Quick Start snippets below all use static tokens for brevity; move to one of the other two grants before you ship. OAuth is available in every SDK except Swift, and the device flow in Go, Ruby, TypeScript, Kotlin, and Python — see the per-language docs linked under [Documentation](#documentation).

## Finding your account ID

Every API path is scoped to an account — `https://3.basecampapi.com/{accountId}/…` — so you need that number before your first call, and the Quick Start snippets below assume you already have it. One token can reach several accounts, so ask the token which:

| Language | Call |
|---|---|
| Go | `client.Authorization().GetInfo(ctx, nil)` |
| Ruby | `client.authorization.get` |
| TypeScript | `await client.authorization.getInfo()` |
| Python | `client.authorization.get()` |

The response lists every account the token can reach; take `accounts[].id` for an entry whose `product` is `"bc3"` (that is Basecamp — the same response also carries `"hey"` and other 37signals products). The same response carries the token's expiry — `expires_at` on the wire, `expiresAt` on the TypeScript type — which is the quickest way to confirm a static token has not lapsed.

The document is account-independent, so call it on the *top-level* client, before `ForAccount`/`for_account`. TypeScript is the exception — `createBasecampClient` requires an `accountId` up front, so pass a placeholder for the bootstrap call and rebuild the client once you know the real one.

It lives on the authorization server that issued your token, not on the Basecamp API — which matters once you leave Launchpad behind. A Launchpad-issued token (authorization code, or a static token from there) reads it at Launchpad. A **device-flow** token is issued by the discovered BC5 server, and its document lives there too. Ruby follows the token: `Http#get_authorization_document` runs resource-first discovery and fetches from the *selected* issuer. The other three hardcode Launchpad, so a device-flow token needs the issuer supplied — Go takes `GetInfoOptions.Endpoint` and TypeScript an `endpoint` option, while Python's `authorization.get()` accepts no override at all, so fetch the document yourself as in the `curl` below.

Swift and Kotlin ship no `authorization` service. Fetch it once with any HTTP client:

```bash
# Launchpad-issued token. For a device-flow token, replace the host with the
# issuer discovery selected — that is where its authorization.json lives.
curl -s https://launchpad.37signals.com/authorization.json \
  -H "Authorization: Bearer $BASECAMP_TOKEN" \
  -H "User-Agent: my-app/1.0 (you@example.com)"
```

A `User-Agent` identifying your app is required on every Basecamp request, including this one.

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
    result, err := account.Projects().List(context.Background(), nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    for _, p := range result.Projects {
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
import { createBasecampClient } from "@37signals/basecamp";

const client = createBasecampClient({
  accountId: process.env.BASECAMP_ACCOUNT_ID!,
  accessToken: process.env.BASECAMP_TOKEN!,
});

const projects = await client.projects.list();
projects.forEach(p => console.log(`${p.id}: ${p.name}`));
```

### Swift

```swift
import Basecamp

let client = BasecampClient(
    accessToken: ProcessInfo.processInfo.environment["BASECAMP_TOKEN"]!,
    userAgent: "my-app/1.0 (you@example.com)"
)

let account = client.forAccount(ProcessInfo.processInfo.environment["BASECAMP_ACCOUNT_ID"]!)
let projects = try await account.projects.list()
for project in projects {
    print("\(project.id): \(project.name)")
}
```

### Kotlin

Service accessors are extension properties, so `account.projects` needs the `com.basecamp.sdk.generated` import as well as the client's. Every service method is `suspend`, so the calls need a coroutine.

```kotlin
import com.basecamp.sdk.BasecampClient
import com.basecamp.sdk.generated.projects

suspend fun main() {
    val client = BasecampClient {
        accessToken(System.getenv("BASECAMP_TOKEN"))
        userAgent = "my-app/1.0 (you@example.com)"
    }

    val account = client.forAccount(System.getenv("BASECAMP_ACCOUNT_ID"))
    account.projects.list().forEach { println("${it.id}: ${it.name}") }

    client.close()
}
```

### Python

```python
import os
from basecamp import Client

client = Client(access_token=os.environ["BASECAMP_TOKEN"])
account = client.for_account(os.environ["BASECAMP_ACCOUNT_ID"])

projects = account.projects.list()
for project in projects:
    print(f"{project['id']}: {project['name']}")
```

## Features

All SDKs provide:

- **Full API coverage** - 35+ services covering projects, todos, messages, schedules, campfires, card tables, and more
- **OAuth 2.0 authentication** - Token refresh, PKCE support (Go, TypeScript, Ruby, Kotlin, Python), and static token options
- **Automatic retry** - Exponential backoff with jitter, respects `Retry-After` headers
- **Pagination** - Link header–based pagination support (high-level handling may vary by SDK; see language docs)
- **ETag caching** - Opt-in HTTP caching for efficient API usage (Go, TypeScript, Ruby†, Swift, Kotlin); off by default everywhere
- **Structured errors** - Typed errors with helpful hints, CLI-friendly exit codes, and per-field validation detail you can bind straight to a form
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
| **Card Tables** | CardTables, Cards, CardColumns, CardSteps, Wormholes |
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
- [TypeScript SDK documentation](typescript/README.md) - npm package usage
- [Swift SDK documentation](swift/README.md) - SPM package with async/await
- [Kotlin SDK documentation](kotlin/README.md) - Gradle package with coroutines
- [Python SDK documentation](python/README.md) - PyPI package with sync and async support
- [Contributing guide](CONTRIBUTING.md) - Development setup and guidelines
- [Security policy](SECURITY.md) - Reporting vulnerabilities

## Environment Variables

There is no environment variable every SDK honours. The Quick Start snippets above call `getenv` themselves — that is the *caller* reading its own environment, not an SDK convention. What the SDKs read on their own, and only when you ask them to (the XDG directory variables aside: Go reads `XDG_CACHE_HOME` in `DefaultConfig` and `XDG_CONFIG_HOME` for its config directory, and Ruby reads `XDG_CONFIG_HOME` in `Config.global_config_dir`):

| Variable | Read by | Only when |
|----------|---------|-----------|
| `BASECAMP_BASE_URL` | Go, Ruby, Python | `cfg.LoadConfigFromEnv()` / `Config.from_env` |
| `BASECAMP_TIMEOUT` | Ruby, Python | `Config.from_env` |
| `BASECAMP_MAX_RETRIES` | Ruby, Python | `Config.from_env` |
| `BASECAMP_CACHE_ENABLED`, `BASECAMP_CACHE_DIR` | Go | `cfg.LoadConfigFromEnv()` |
| `BASECAMP_PROJECT_ID`, `BASECAMP_TODOLIST_ID` | Go | `cfg.LoadConfigFromEnv()` |
| `BASECAMP_TOKEN` | Go | you authenticate through `AuthManager`, which prefers it over the stored OAuth credentials |
| `BASECAMP_NO_KEYRING` | Go | you construct a `CredentialStore` via `NewCredentialStore` — which `NewAuthManager` does for you, but `NewAuthManagerWithStore` does not |

TypeScript, Swift, and Kotlin read no environment variables at all — configure them entirely through their options objects.

**`BASECAMP_ACCOUNT_ID` is not an SDK variable.** No SDK reads it; it is a convention shared by this repository's examples, `make conformance-*-live`, and the nightly canary. Pass the account ID explicitly to `ForAccount` / `for_account` / `forAccount` / `accountId`.

See individual SDK documentation for language-specific options.

## License

MIT
