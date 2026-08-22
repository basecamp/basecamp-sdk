# Basecamp Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/basecamp/basecamp-sdk/go.svg)](https://pkg.go.dev/github.com/basecamp/basecamp-sdk/go)
[![Test](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml/badge.svg)](https://github.com/basecamp/basecamp-sdk/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/basecamp/basecamp-sdk/go)](https://goreportcard.com/report/github.com/basecamp/basecamp-sdk/go)

Official Go SDK for the [Basecamp API](https://github.com/basecamp/bc3-api).

**Upgrading to v0.14.0?** Read [MIGRATING.md](../MIGRATING.md) before you bump the version. The compiler catches the type changes — `ListVersions` elements became `UploadVersion`, and `UpdateUploadRequest.Description` became `*string` — but not the reroute: every 507 now reports `limit_exceeded` instead of a retryable `api_error`, so a `case` falling through to a default arm sends storage, project and webhook limits wherever that default goes. Coming from v0.12.0 or earlier, read v0.13.0's section too.

## Features

- Full coverage of 30+ Basecamp API services
- OAuth 2.0 authentication with automatic token refresh
- Static token authentication for simple integrations
- ETag-based HTTP caching for efficient API usage (opt-in)
- Automatic retry with exponential backoff
- Pagination handling with `GetAll()`
- Structured errors with CLI-friendly exit codes
- Secure credential storage (system keyring with file fallback)

## Installation

```bash
go get github.com/basecamp/basecamp-sdk/go
```

Requires Go 1.26 or later.

## Getting a token

Every Basecamp API request carries an OAuth 2.0 access token. There is no API key and no personal access token, so even a throwaway script starts here:

1. Choose the grant that matches how your code runs:

| Your integration | Grant | Who refreshes the token |
|---|---|---|
| already holds a token you obtained elsewhere | **static token** — [below](#using-a-static-token) | you do |
| can receive a browser redirect (web app, or a local callback server) | **authorization code + PKCE** — [below](#using-oauth-20) | `AuthManager` |
| has no browser, but a person can approve on another device (CLI, headless server, TV) | **device flow** — [below](#oauth-device-authorization-grant-rfc-8628) | `AuthManager` |

The one-line rule: **a redirect URI you control → authorization code; no browser but someone to approve → device flow; a token already in hand → static token.** An unattended daemon or CI job fits none of the three on its own — the device flow needs a person to enter the user code at the verification URI — so provision a token out of band and hand it to the process as a static or refresh token.

2. Get the client credentials that grant needs:

- **Authorization code + PKCE** — register your own integration at **<https://launchpad.37signals.com/integrations>**. You get a client ID, a client secret, and whatever redirect URI you nominated.
- **Device flow** — nothing to register. It runs as the pre-registered public `basecamp-cli` client, which sends no secret, against the device endpoint that discovery returns. Launchpad advertises no device endpoint, so a client you register there is not the one this flow uses.
- **Static token** — nothing to register; you already hold the token.

`StaticTokenProvider` hands back the string you gave it and nothing more — it never refreshes, so once the token expires every call fails with `401` until you supply a new one. Use it to get a first successful call, then move to `AuthManager` before you ship.

## Finding your account ID

Every API path is scoped to an account — `https://3.basecampapi.com/{accountId}/…` — so `ForAccount` needs that number before your first call. One token can reach several accounts, so ask the token which. `Authorization()` hangs off the *top-level* client because the endpoint lives on the authorization server rather than the Basecamp API, and so takes no account context. It defaults to Launchpad, which is right for a Launchpad-issued token; a **device-flow** token is issued by the discovered BC5 server and its `authorization.json` lives there, so pass that issuer as `GetInfoOptions.Endpoint`:

```go
info, err := client.Authorization().GetInfo(context.Background(), &basecamp.GetInfoOptions{
    FilterProduct: "bc3", // Basecamp; the same response also carries "hey" and other products
    // Endpoint defaults to Launchpad, which is right for a Launchpad-issued
    // token. For a device-flow token, set it to the discovered issuer:
    //   Endpoint: issuer + "/authorization.json",
})
if err != nil {
    log.Fatal(err)
}
for _, a := range info.Accounts {
    fmt.Printf("%d: %s\n", a.ID, a.Name)
}

account := client.ForAccount(fmt.Sprint(info.Accounts[0].ID))
```

`info.Expiry()` tells you how long the token has left, which is the quickest way to confirm a static token has not lapsed. `ok` is false when the document states no expiry — both production issuers always state one today, so that branch is robustness for whatever `GetInfoOptions.Endpoint` points at:

```go
if expiry, ok := info.Expiry(); ok && time.Until(expiry) < 0 {
    log.Fatal("token has lapsed")
}
```

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
    result, err := account.Projects().List(context.Background(), nil)
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range result.Projects {
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
    result, err := account.Projects().List(context.Background(), &basecamp.ProjectListOptions{
        Status: basecamp.ProjectStatusActive,
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range result.Projects {
        fmt.Printf("%s (%d)\n", p.Name, p.ID)
    }
}
```

### OAuth discovery (resource-first)

BC5's authorization-server (AS) metadata lives only at its canonical issuer (the
web host), so discovery starts from the **resource** (RFC 9728) and composes with
AS metadata discovery (RFC 8414). The `oauth` package exposes three composable
operations on `*oauth.Discoverer`:

```go
import (
    "errors"
    "net/http"

    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/oauth"
)

d := oauth.NewDiscoverer(http.DefaultClient)

// RFC 8414 — authorization-server metadata, with issuer binding.
cfg, err := d.Discover(ctx, "https://launchpad.37signals.com")

// RFC 9728 — protected-resource metadata (hop 1).
meta, err := d.DiscoverProtectedResource(ctx, "https://3.basecampapi.com")

// Orchestrator — resource-first selection + stage-sensitive fallback.
result, err := d.DiscoverFromResource(ctx, "https://3.basecampapi.com")
if err != nil {
    // Hard failure — never fall back to Launchpad. Match with errors.Is:
    switch {
    case errors.Is(err, oauth.ErrAmbiguousIssuers):          // ≥2 non-Launchpad issuers, no expected issuer
    case errors.Is(err, oauth.ErrExpectedIssuerUnavailable): // expected issuer not advertised
    case errors.Is(err, oauth.ErrInvalidIssuerOrigin):       // advertised issuer refused: bad origin root, or blocked address
    case errors.Is(err, oauth.ErrASFetchFailed):             // committed issuer's AS metadata unavailable
    case errors.Is(err, oauth.ErrIssuerMismatch):            // committed issuer's metadata fails issuer binding
    }
    return err
}
if result.IsFallback() {
    // Soft fallback: result.FallbackReason is resource_discovery_failed or
    // no_as_advertised. Fall back to Launchpad (oauth.LaunchpadBaseURL).
} else {
    use(result.Config, result.Issuer) // selected BC5 authorization server
}
```

Pass `oauth.WithExpectedIssuer(issuer)` to select an issuer authoritatively
instead of by the exclude-Launchpad heuristic; `oauth.WithTimeout` and
`oauth.WithMaxBodyBytes` tune each fetch.

Notes:

- `Config.AuthorizationEndpoint` is now `*string` (optional): device-only servers
  omit it, so authorization-code consumers must assert its presence before use.
  `Config` also carries `DeviceAuthorizationEndpoint *string` and
  `GrantTypesSupported []string`.
- Every fetch is SSRF-hardened: origins are validated with `net/url` before any
  socket opens, HTTPS is required (localhost exempt), redirects are suppressed,
  timeouts are bounded, and bodies are read under a bounded cap. Non-2xx on
  either hop surfaces as an `api_error`.
- **The advertised-issuer hop additionally enforces an address policy.** That
  hop is the one whose destination comes from a parsed response body, so URL
  syntax is not enough: `net/url` has no notion of what a host resolves to.
  `DiscoverFromResource` judges the literal address at connection time via
  `oauth.DefaultIssuerPolicy()`, so a syntactically valid issuer pointing at
  private, loopback, link-local, or other special-use space is refused before a
  socket opens — including legacy spellings like `https://2130706433/`, and
  including a name that resolves to blocked space only at dial time. (The
  `https` here is load-bearing: on the `http` spelling of that same host the
  origin-root profile refuses first, so it would demonstrate the pre-existing
  scheme gate rather than the address policy.) The
  refusal surfaces as `ErrInvalidIssuerOrigin`; it also matches
  `errors.Is(err, surfguard.ErrBlocked)` and wraps a `*surfguard.Violation` if
  you need to tell the two causes apart. Hop 1 and `Discover` are unaffected —
  their destinations are operator-configured.

  A deployment whose issuer legitimately sits off the public internet re-admits
  exactly the space it needs rather than switching the policy off. Mind
  surfguard's precedence when you do: `Allow` re-admits space the default deny
  tables refuse but **not** space the `IANASpecialUse` tables refuse, and those
  cover all of RFC 1918.

  ```go
  // On-premises issuer in private space — build without IANASpecialUse.
  oauth.NewDiscoverer(c, oauth.WithIssuerPolicy(
      surfguard.Policy{}.AllowAllPorts().Allow(netip.MustParsePrefix("10.4.0.0/16"))))

  // Local development — AllowLoopback does pierce those tables.
  oauth.NewDiscoverer(c, oauth.WithIssuerPolicy(
      oauth.DefaultIssuerPolicy().AllowLoopback()))
  ```

  `oauth.WithIssuerHTTPClient` carries the hop on your own client, which a
  consumer egressing through a proxy needs since surfguard's transport sets
  `Proxy: nil` by construction. `oauth.WithoutIssuerPolicy` restores the
  pre-policy behavior outright.

- **The same policy governs where the credentials go next.** The selected
  `Config` carries the `token_endpoint` and `device_authorization_endpoint`
  the issuer's metadata named, and nothing constrains those to the issuer's
  origin — so a public issuer that passes the policy could still steer the
  `client_id`, `device_code`, authorization code, client secret, or refresh
  token into private space. `PerformDeviceLogin`, `RequestDeviceAuthorization`,
  `PollDeviceToken`, and an `Exchanger` therefore judge the endpoint's address
  at dial time by default, on the same shared `oauth.DefaultIssuerPolicy()`
  client. A refusal is a non-retryable `*basecamp.Error` (`api_error`) that
  matches `errors.Is(err, surfguard.ErrBlocked)`; in the poll loop it ends the
  flow on the first attempt rather than backing off. See the device-flow
  section below for the overrides.

### OAuth device authorization grant (RFC 8628)

For CLIs and other input-constrained clients, the `oauth` package implements the
RFC 8628 device authorization grant. `PerformDeviceLogin` accepts an
already-selected `*oauth.Config` (from discovery), guards the device capability,
shows the user a code, and polls the token endpoint until they approve. Each
configured endpoint is required to use HTTPS (localhost exempt) and redirects are
suppressed.

```go
import (
    "errors"
    "fmt"
    "net/http"

    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/oauth"
)

// The device grant lives on BC5's authorization server, not Launchpad, so the
// config must be the selected first-party result of resource-first discovery. A
// Launchpad fallback advertises no device endpoint, and PerformDeviceLogin would
// return DeviceFlowError{Reason: DeviceFlowUnavailable}.
d := oauth.NewDiscoverer(http.DefaultClient)
result, err := d.DiscoverFromResource(ctx, "https://3.basecampapi.com")
if err != nil {
    return err // hard failure
}
if result.IsFallback() {
    // Launchpad fallback: no device endpoint. Surface it explicitly (or switch
    // to the authorization-code flow) — never return nil here, or the caller
    // treats the fallback as a successful device login with no token.
    return fmt.Errorf("device flow unavailable: fell back to Launchpad (%s)", result.FallbackReason)
}

token, err := oauth.PerformDeviceLogin(ctx, result.Config, "basecamp-cli",
    func(auth oauth.DeviceAuthorization) {
        fmt.Printf("Visit %s and enter code: %s\n", auth.VerificationURI, auth.UserCode)
    },
    // Optional: oauth.WithDeviceScope("read"), oauth.WithDeviceHTTPClient(hc).
)
if err != nil {
    var dfe *oauth.DeviceFlowError
    if errors.As(err, &dfe) {
        switch dfe.Reason {
        case oauth.DeviceFlowAccessDenied: // user declined  → auth_required
        case oauth.DeviceFlowExpired:      // code expired    → auth_required
        case oauth.DeviceFlowTransport:    // network failure → network (retryable)
        case oauth.DeviceFlowUnavailable:  // AS lacks device flow → validation
        case oauth.DeviceFlowCancelled:    // ctx cancelled   → wraps ctx.Err()
        }
    }
    return err
}
use(token) // *oauth.Token
```

The capability guard requires BOTH `cfg.DeviceAuthorizationEndpoint != nil` AND
`cfg.GrantTypesSupported` advertising `oauth.DeviceCodeGrantType`; otherwise it
returns `DeviceFlowError{Reason: DeviceFlowUnavailable}` before any request.

`DeviceFlowError` derives its parent error category from `Reason` (call `.Code()`
for the `basecamp` taxonomy code, `.Retryable()` for retryability). A cancelled
flow wraps the context error, so `errors.Is(err, context.Canceled)` matches.

The two lower-level steps are exported for callers that drive the flow directly:

```go
auth, err := oauth.RequestDeviceAuthorization(ctx, deviceAuthEndpoint, "basecamp-cli")
if err != nil {
    return err
}
token, err := oauth.PollDeviceToken(ctx, tokenEndpoint, "basecamp-cli",
    auth.DeviceCode, auth.Interval, auth.ExpiresIn)
```

`PollDeviceToken` runs the §3.5 loop: it waits at least `interval` seconds
between polls, enforces a monotonic expiry deadline, sustains `slow_down` bumps
(+5s), backs off exponentially on connection timeouts, and honors context
cancellation. The clock (`oauth.WithDeviceClock`) and inter-poll wait
(`oauth.WithDeviceSleep`) are injectable for deterministic tests; scope is
omitted from the authorization request unless set with `oauth.WithDeviceScope`,
so the server applies its default (`read`) — prefer pinning it explicitly.

**Both endpoints are address-policed by default**, and so is the token endpoint
an `oauth.Exchanger` posts to: the flow cannot tell a discovered endpoint from
a hand-configured one, so every request on the default client is judged by
`oauth.DefaultIssuerPolicy()` at dial time. The precedence is the same on every
surface of the package — a client you hand in is yours, enforcement included;
otherwise your policy; otherwise the default:

```go
// Local development against an AS on loopback — AllowLoopback pierces the
// IANASpecialUse tables; plain Allow does not (see the discovery notes above).
devPolicy := oauth.WithDevicePolicy(oauth.DefaultIssuerPolicy().AllowLoopback())
token, err := oauth.PerformDeviceLogin(ctx, cfg, "basecamp-cli", display, devPolicy)
ex := oauth.NewExchanger(nil, oauth.WithExchangerPolicy(oauth.DefaultIssuerPolicy().AllowLoopback()))

// Your own client carries the requests instead, policy and all. To keep the
// policy on a custom transport, build the transport from it:
hc := &http.Client{Transport: oauth.DefaultIssuerPolicy().RoundTripper()}
oauth.PerformDeviceLogin(ctx, cfg, "basecamp-cli", display, oauth.WithDeviceHTTPClient(hc))
oauth.NewExchanger(hc)

// http.DefaultClient restores the pre-policy behavior outright.
oauth.NewExchanger(http.DefaultClient)
```

`WithDevicePolicy` builds its transport once, when the option is constructed,
so build it once and reuse it; an `Exchanger` given `WithExchangerPolicy` owns
its transport the same way. Neither has a `Close`.

### Persisting device-login credentials (RFC 8707 resource echo)

BC5 device logins as `basecamp-cli` mint **multi-account** refresh tokens: the
token response carries an RFC 8707 `resource` indicator
(`urn:bc:account:<id>`, on `token.Resource`), and every refresh of a
multi-account token MUST send it back or the server rejects the refresh
(400 `invalid_request`). `AuthManager` echoes the stored `resource` (and
submits the stored `client_id` — BC5 public clients send no secret)
automatically, and preserves the stored value when a refresh response omits
it.

The oauth helpers return an `*oauth.Token`; they never write the root
package's `Credentials`. After a device login (or code exchange) the caller
bridges the two — saving the client id it used and the token's resource so
later refreshes can echo them:

```go
import (
    "net/http"

    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// sdkCfg is the SDK *basecamp.Config (the device-login example above binds
// its discovery result to result.Config, an *oauth.Config — a different type).
sdkCfg := &basecamp.Config{BaseURL: "https://3.basecampapi.com"}
authMgr := basecamp.NewAuthManager(sdkCfg, http.DefaultClient)
creds := &basecamp.Credentials{
    AccessToken:   token.AccessToken,
    RefreshToken:  token.RefreshToken,
    Scope:         token.Scope,
    TokenEndpoint: result.Config.TokenEndpoint,
    ClientID:      "basecamp-cli",       // the id the login used
    Resource:      token.Resource,       // the account binding to echo on refresh
}
// A token response may omit expires_in; a zero ExpiresAt means no known
// expiry (never force-refreshed) — storing zero-time.Unix()'s negative value
// would instead mark the fresh token expired.
if !token.ExpiresAt.IsZero() {
    creds.ExpiresAt = token.ExpiresAt.Unix()
}
err = authMgr.Store().Save(basecamp.NormalizeBaseURL(sdkCfg.BaseURL), creds)
```

Pass the discovered base origin without a trailing slash everywhere a base URL
or issuer is expected: binding is code-point exact — a trailing-slash
`WithExpectedIssuer` value fails the advertised-member lookup as a **hard**
`ErrExpectedIssuerUnavailable`, while a trailing-slash resource origin breaks
the hop-1 binding and silently soft-falls back to Launchpad.

## Configuration

### Environment Variables

Nothing here is read automatically. The five `Config` variables apply only when you call `cfg.LoadConfigFromEnv()`. `BASECAMP_TOKEN` is read by `AuthManager.AccessToken` and `AuthManager.IsAuthenticated`; `BASECAMP_NO_KEYRING` is read one level down, by `NewCredentialStore`. (Separately, `DefaultConfig` consults `XDG_CACHE_HOME` to site the cache directory, and `globalConfigDir` consults `XDG_CONFIG_HOME` to site the config directory.)

| Variable | Read by | Description |
|----------|---------|-------------|
| `BASECAMP_BASE_URL` | `cfg.LoadConfigFromEnv()` | API base URL (default: `https://3.basecampapi.com`) |
| `BASECAMP_PROJECT_ID` | `cfg.LoadConfigFromEnv()` | Default project ID |
| `BASECAMP_TODOLIST_ID` | `cfg.LoadConfigFromEnv()` | Default todolist ID |
| `BASECAMP_CACHE_DIR` | `cfg.LoadConfigFromEnv()` | Cache directory path (default: `~/.cache/basecamp`) |
| `BASECAMP_CACHE_ENABLED` | `cfg.LoadConfigFromEnv()` | Enable HTTP caching (default: `false`) |
| `BASECAMP_TOKEN` | `AuthManager` | Access token. Consulted **first**, ahead of any stored OAuth credentials, so setting it short-circuits the OAuth flow — handy for scripts and CI |
| `BASECAMP_NO_KEYRING` | `NewCredentialStore` | Store credentials in a file instead of the system keyring. Read when the store is constructed, so `NewAuthManager` picks it up but `NewAuthManagerWithStore` does not — pass a store you built yourself and this variable has already been applied, or not, by that call |

`StaticTokenProvider` does **not** read `BASECAMP_TOKEN`; it uses whatever string you put in its `Token` field. The Quick Start above reads the variable itself, at the call site.

Note: account ID is specified via `client.ForAccount(accountID)` rather than configuration. `BASECAMP_ACCOUNT_ID` is a convention used by this repository's examples and tooling — no SDK reads it.

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

## Optional Fields

Optional fields are pointers, so that "not addressed" stays distinguishable from
a value. Nil omits the field; a non-nil pointer sends the value verbatim,
including the zero value. `basecamp.Ptr` builds one for any type:

```go
entry, err := account.Schedules().UpdateEntry(ctx, entryID, &basecamp.UpdateScheduleEntryRequest{
    Summary:        basecamp.Ptr("Kickoff, moved"),
    AllDay:         basecamp.Ptr(false),     // an explicit false, not "unset"
    ParticipantIDs: basecamp.Ptr([]int64{}), // an explicit empty list: remove everyone
    // Description stays nil, so the entry's description is left alone.
})
```

Reading one is the half that fails quietly: Go auto-dereferences a value-receiver
method call, so `hc.UpdatedAt.IsZero()` compiles against a `*time.Time` and
panics at run time on a chart that has never moved. Nil-check it, or let
`basecamp.Deref` return the zero value for you:

```go
hc, err := account.HillCharts().Get(ctx, todosetID)
if updated := basecamp.Deref(hc.UpdatedAt); !updated.IsZero() {
    fmt.Println("last moved", updated)
}
```

Collapsing absence to the zero value is only safe where the caller cannot tell
the two apart. Where the difference carries meaning — a string the server really
sent as empty versus a field it omitted — compare against nil instead.

## API Coverage

### Projects & Organization

| Service | Methods |
|---------|---------|
| `Projects()` | List, Get, Create, Update, Trash, Archive, Unarchive |
| `Templates()` | List, Get, CreateProject |
| `Tools()` | Get, Create, Update, Delete, Enable, Disable, Reposition (dock tools) |
| `People()` | List, Get, ListPingable, Me, ListProjectPeople |

### To-dos

| Service | Methods |
|---------|---------|
| `Todos()` | List, Get, Create, Update, Edit, Replace, Complete, Uncomplete, Reposition |
| `Todosets()` | Get |
| `Todolists()` | List, Get, Create, Update, Edit, Replace, Trash, Reposition |
| `TodolistGroups()` | List, Get, Create, Replace, Reposition |

`PUT` on these resources is a full replace: BC3 rebuilds the record from the
params it receives, so a field you omit is cleared. `Update` (overlay the
fields you set) and `Edit` (read-modify-write closure) are merge-safe
composites over that route — they GET first and resend the whole
representation. `Replace` is the raw verbatim PUT. `TodolistGroups()` offers
only `Replace`; use `Todolists().Update`/`Edit` for merge-safe group writes.
That is not a workaround — BC3 has no group model, so `TodolistGroup` is a Go
alias for `Todolist` (#544) and both surfaces address one polymorphic route
through one projection. Tell the variants apart structurally: `GroupsURL` on a
list, `GroupPositionURL` on a group. Never on `Type`, which reads `"Todolist"`
for both.

### Messages & Communication

| Service | Methods |
|---------|---------|
| `Messages()` | List, Get, Create, Update, Trash |
| `MessageBoards()` | Get |
| `MessageTypes()` | List, Get, Create, Update, Destroy |
| `Comments()` | List, Get, Create, Update, Trash |
| `Campfires()` | List, Get, ListLines, GetLine, CreateLine, UpdateLine, DeleteLine, Chatbot CRUD |
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
| `Wormholes()` | Create, Update, Delete |

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
err = account.Todos().Reposition(ctx, todoID, 1, nil) // Move to first position

// Move a todo to a different todolist
targetListID := int64(12345)
err = account.Todos().Reposition(ctx, todoID, 1, &targetListID)
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
campfires, err := account.Campfires().List(ctx, nil)

// Send a message
line, err := account.Campfires().CreateLine(ctx, campfireID, "Hello, team!")

// List recent messages
lines, err := account.Campfires().ListLines(ctx, campfireID, nil)
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
webhooks, err := account.Webhooks().List(ctx, bucketID, nil)

// Delete a webhook
err = account.Webhooks().Delete(ctx, webhookID)
```

## Event Feed (experimental)

The `eventfeed` package is the account-wide event feed connector: an Action Cable
subscription for live push, plus a poll lane that catches up on entry, repairs on a
timer, and resumes after a disconnect. You consume it as one serial, deduplicated
stream of events; the connector owns reconnection, backoff, staleness detection, and
the durable position.

**Experimental: the Layer-1 seam adapters have not landed yet.** The connector performs
no HTTP API I/O of its own: every HTTP exchange reaches the wire through a seam backed by
a generated operation. Its one direct wire act is the Action Cable dial above — the
connector connects verbatim to the URL a generated `CreateStreamTicket` call returned,
which is the sanctioned non-HTTP wire act. The adapters that build those seams over the
generated `CreateStreamTicket` and `PollEvents` operations are still to come. Until they
do, a consumer must supply the `TicketMinter` and `PollSource` implementations itself, and
the exported surface may still change as they land.

```go
import (
    "context"
    "errors"
    "fmt"
    "log"

    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// The two seams the host supplies until the Layer-1 adapters land. Each call is
// exactly one generated operation:
//
//	MintStreamTicket(ctx) (eventfeed.StreamTicket, error)   // CreateStreamTicket
//	Poll(ctx, cursor, filters) (eventfeed.PollPage, error)  // PollEvents
var minter eventfeed.TicketMinter
var polls eventfeed.PollSource

ctx := context.Background()

feed, err := eventfeed.New("https://3.basecampapi.com", "5951425", minter, polls,
    eventfeed.WithFilters(eventfeed.Filters{Types: []string{"message.created"}}),
    eventfeed.WithCheckpointStore(eventfeed.NewFileCheckpointStore("/var/lib/myapp/feed.json")),
    eventfeed.WithConsumerNamespace("myapp"),
    eventfeed.WithSignalHandler(func(sig eventfeed.Signal) eventfeed.Disposition {
        switch s := sig.(type) {
        case eventfeed.FeedGap:
            // History before this id is gone. Accept resumes at the server's
            // resume URL, having acknowledged what it skips.
            log.Printf("feed gap: history before %d is gone", s.EpochAfterID)
            return eventfeed.Accept
        case eventfeed.BufferOverflow:
            log.Printf("live buffer dropped %d events: %v", s.DroppedCount, s.DroppedIDs)
            return eventfeed.Terminate
        }
        return eventfeed.Terminate
    }),
)
if err != nil {
    return err // *eventfeed.TerminalError: a usage-coded construction error, zero wire attempts
}
defer feed.Close()

for ev, err := range feed.Events(ctx) {
    if err != nil {
        var te *eventfeed.TerminalError
        if errors.As(err, &te) {
            return fmt.Errorf("event feed terminated (%s): %w", te.Reason, te)
        }
        return err
    }
    // A feed row is a wake-up signal — enough to route, not enough to act on.
    // Refetch the recording through the canonical resource API before acting.
    handle(ev.BucketID, ev.RecordingID, ev.EventType)
}
```

Construction validates and does no I/O. The base origin must be `https://` — cleartext
`http://` is accepted only for localhost/loopback, the same carve-out the client's base
URL and the connector's cable URL make — because that origin is the trust anchor every
continuation and resume URL is validated against before an authenticated poll follows
it. The checkpoint identity's text inputs (origin, account id, consumer namespace,
filter types) must be valid UTF-8, since the identity encoding is one-to-one only over
valid UTF-8. Either violation is a `ReasonUsage` construction error with zero wire
attempts.

`Events` is single-shot: consuming it twice yields one `ReasonUsage` error element.
`Close` stops the feed without draining, and cancelling the context, calling `Close`, or
breaking out of the loop all end iteration with **no** error element — a clean stop, and
the feed is resumable by design.

### Checkpointing

`FileCheckpointStore` is the built-in `CheckpointStore`: one JSON file holding every
lineage's position, keyed by the four-part checkpoint identity (origin, account,
consumer namespace, filter key). It writes temp-file-plus-rename at 0600, and it is safe
for concurrent use within one process but deliberately not across processes. A store
requires `WithConsumerNamespace` — two independent consumers in one account must not
share a lineage — and changing filters starts a new lineage, because positions are
filter-bound.

Only poll pages ever advance the durable position; live event ids never do. What the
connector publishes is SPEC.md §23's conjunctive save-ordering invariant, and nothing
stronger: a position is saved only after the retained events it covers have been
delivered **and** every loss condition in that window has been explicitly accepted.
Terminate — or no handler — means no save. A load failure is terminal
(`ReasonCheckpointLoad`, before any wire attempt: silently starting at the present would
skip history), while a save failure is reported through `Observer.CheckpointSaveFailed`
and the feed continues.

### Semantic signals

A semantic signal is a condition that changes what the feed can promise, and there are
exactly two: `BufferOverflow` (the live buffer dropped events, naming the exact ids) and
`FeedGap` (a 410 — history before `EpochAfterID` is gone). The handler registered with
`WithSignalHandler` is invoked exactly once per signal, synchronously, on your own
execution context, and returns `Accept` or `Terminate`.

**With no handler registered, every semantic signal is terminal** — `ReasonBufferOverflow`
or `ReasonFeedGap` — so an unhandled signal cannot disappear, and a 410 never silently
auto-continues. `Accept` on a `FeedGap` resumes via the server's resume URL; `Accept` on a
`BufferOverflow` means you own the acknowledged incompleteness. The `Observer.Gap` and
`Observer.BufferOverflow` callbacks see the same conditions but are observability only:
the disposition lives exclusively in the handler.

### Terminal versus continuable

Continuable failures — a dropped socket, a staleness expiry, a throttled or transient
mint or poll, a rejected position (400-position or 409) — never reach the consumer as
errors. They ride the reconnect cycle (full-jitter backoff) or the poll-retry timer, and
you see them through `Observer` callbacks if you register any.

A terminal condition ends the iteration with exactly one final `*eventfeed.TerminalError`
element carrying a `TerminalReason`: `subscription_rejected`, `protocol_fatal`,
`filter_invalid`, `authorization_failed`, `checkpoint_load`, `usage`, `buffer_overflow`,
`feed_gap`, `invalid_continuation`, `poll_failed`, `mint_failed`, or `invalid_cable_url`.
Switch on `te.Reason` rather than on message text; `errors.Unwrap` reaches the generated
error behind `mint_failed` and `poll_failed`.

## Pagination

List methods auto-paginate: they follow the API's `Link: rel="next"` headers and
collect every item across all pages, bounded by `Limit` when you set one.

`Page` opts out of that. A positive `Page` fetches exactly that page and disables
auto-pagination:

```go
// Page 3 only — one request, no link-following.
result, err := account.Projects().List(ctx, &basecamp.ProjectListOptions{Page: 3})
```

A positive `Limit` still trims that page, and dropping items from it counts as
truncation. The per-operation *default* limits (100 todos, and so on) do not
apply to a pinned page — asking for page 3 asks for page 3, not its first 100
items.

`Meta.Truncated` still answers "was there more": on a pinned page it is true when
a `rel="next"` Link went deliberately unfollowed, or when `Limit` discarded items.

A handful of endpoints are not paginated server-side (`Webhooks().List()`,
`MessageTypes().List()`, and the other cases each options struct calls out); they
return the whole list, and `Page` there only short-circuits auto-pagination.

All six SDKs share these semantics — one request, that page only, no
link-following. See SPEC section 8.

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
| `limit_exceeded` | Account limit reached (507) — never retryable | 10 |

### Validation Errors

Basecamp rejects invalid writes with a body keyed by field. The SDK folds those
messages into `Message` and keeps the raw map in `FieldErrors`, so you can drive
a form without re-parsing the message:

```go
_, err := account.Calendars().Update(ctx, calendarID, "chartreuse")
if apiErr, ok := err.(*basecamp.Error); ok && apiErr.Code == basecamp.CodeValidation {
    // Message: "color: is not a valid color"
    fmt.Println(apiErr.Message)

    for field, messages := range apiErr.FieldErrors {
        for _, message := range messages {
            fmt.Printf("  %s %s\n", field, message)
        }
    }
}
```

`FieldErrors` is `nil` for every other error shape, and its messages are the raw
ones — `Message` is capped at 500 bytes, the map is not.

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

// Or with custom providers
hooks = basecampotel.NewHooks(
    basecampotel.WithTracerProvider(tp),
    basecampotel.WithMeterProvider(mp),
)

client := basecamp.NewClient(cfg, token, basecamp.WithHooks(hooks))
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
    "github.com/prometheus/client_golang/prometheus"
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
