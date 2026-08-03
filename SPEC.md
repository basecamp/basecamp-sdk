# Basecamp SDK — Natural Language Specification

## §0. Preamble

### Audience

This document is a complete, implementation-grade specification for building a Basecamp API SDK in any programming language. The primary audience is coding agents and developers who need to implement a new language SDK.

### Existing SDKs as Exemplars

Six shipping SDKs live alongside this spec in the same repository: Go, Ruby, Python, TypeScript, Kotlin, and Swift. Use them as reference implementations when the spec leaves room for interpretation. TypeScript (`typescript/src/client.ts`) is the most complete single-file reference for auth, retry, pagination, and caching. Ruby (`ruby/lib/basecamp/http.rb`) has the most explicit pagination variants. Go (`go/pkg/basecamp/`) demonstrates the hand-written service wrapper pattern. When in doubt, read the code — the spec prescribes the contract, the SDKs show how it's been realized.

### Input Artifacts

| Artifact | Path | Role |
|----------|------|------|
| `openapi.json` | repo root | API surface: operations, paths, parameters, response schemas, tags |
| `behavior-model.json` | repo root | Operation metadata: retry config, idempotency flags |
| `conformance/schema.json` | `conformance/` | Test assertion type definitions |
| `conformance/tests/*.json` | `conformance/tests/` | Behavioral truth — 9 test categories |
| `spec/` directory | `spec/` | Smithy model source (generates `openapi.json` and `behavior-model.json`) |

### Notation Conventions

- **RECORD** — a data structure with named fields and types. Language adaptation: struct, class, data class, record, etc.
- **INTERFACE** — a contract with method signatures. Language adaptation: interface, protocol, trait, abstract class, etc.
- **Algorithms** — numbered steps executed sequentially. Step references use `→` for return and `⊥` for abort/throw.
- **Verification tags** — every behavioral requirement is tagged:
  - `[conformance]` — verified by conformance test suite
  - `[static]` — verified by static analysis, build checks, or code generation
  - `[manual]` — requires human review

### Source-of-Truth Precedence

When artifacts conflict, this precedence governs:

1. **Conformance tests** — behavioral truth. If a test asserts a behavior, the spec matches it.
2. **Shipping SDK code** (consensus of Go, Ruby, Python, TypeScript, Kotlin, Swift) — implementation truth. When 4+ SDKs agree, that's the contract.
3. **`behavior-model.json`** — machine-readable metadata. Descriptive of retry/idempotency semantics, but the retry block alone does not activate retry for POST (see §7).
4. **`rubric-audit.json`** — audit snapshot. Known to drift (e.g., 3C.3 claims 1024 chars; all six SDKs use 500). Trust code over audit.
5. **RUBRIC.md** — evaluation framework (external governance reference in the `basecamp/sdk` repo, not this repo). Defines criteria, not implementations. Referenced by criteria IDs (e.g., 2A.3, 3C.1) but not as an input artifact — this spec is self-contained.

`[CONFLICT]` annotations appear inline where sources disagree, with resolution rationale.

---

## §1. Architecture Overview

### Component Responsibilities

| Component | Responsibility |
|-----------|---------------|
| **Config** | Holds validated configuration: base URL, timeouts, retry params, pagination caps. May support env-var override (see §2). |
| **Client** | Top-level entry point. Enforces exactly-one-of auth. Owns account-independent services (authorization). |
| **AccountClient** | Account-scoped facade. Prepends `/{accountId}` to paths. Owns all 46 account-scoped services. |
| **Services** | One class per API resource group. Generated from OpenAPI tags. Methods map to operations. |
| **BaseService** | Abstract base for generated services. Provides request execution, error mapping, pagination following, hooks integration. |
| **HTTP Transport** | Executes HTTP requests. Applies auth headers, User-Agent, Content-Type. Implements retry, caching. |
| **Errors** | Structured error hierarchy. Maps HTTP statuses to typed error codes with exit codes. |
| **Security** | HTTPS enforcement, body size limits, message truncation, header redaction, same-origin validation. |

### Two-Tier Topology

```
Client
├── authorization (service — no account context)
└── forAccount(accountId) → AccountClient
    ├── projects (service)
    ├── todos (service)
    ├── ... (43 more services)
    └── HTTP Transport
        ├── Auth Middleware
        ├── Retry Middleware
        ├── Cache Middleware (opt-in)
        └── Hooks Middleware (opt-in)
```

### Dependency Invariant `[static]`

Generated code depends only on `BaseService` + schema types. `BaseService` may wrap a raw HTTP client or an account-scoped facade (e.g., Swift and Ruby services are initialized with an `AccountClient` reference), but the generated service code itself does not import or depend on the top-level `Client` constructor.

---

## §2. Configuration

### Config RECORD

```
RECORD Config
  base_url        : String    = "https://3.basecampapi.com"
  timeout         : Duration  = 30s
  max_pages       : Integer   = 10000
  -- Retry/backoff fields below are optional. Exposure varies:
  -- Ruby and Go expose all three. Kotlin exposes max_retries and
  -- base_delay but hard-codes jitter (MAX_JITTER_MS = 100).
  -- TypeScript uses per-operation metadata from a generated
  -- metadata.json (derived from OpenAPI x-basecamp-* extensions).
  -- Swift uses per-operation metadata from behavior-model.json.
  -- New implementations may omit these from the public config
  -- and use the per-operation metadata defaults directly.
  max_retries     : Integer   = 3       -- optional config field
  base_delay      : Duration  = 1000ms  -- optional config field
  max_jitter      : Duration  = 100ms   -- optional config field (Kotlin hard-codes this)
END
```

**Go Config divergence:** Go splits this across two structs — `Config` (base URL, project/todolist IDs, cache settings) and `HTTPOptions` (timeout, retry params, redirect policy, TLS config). The spec's single `Config` RECORD is the canonical shape; Go's split is a language adaptation.

**Naming note:** `max_retries` means total attempts (including the initial request), not the number of retries after the first attempt. With `max_retries = 3`, the transport makes at most 3 attempts total (1 initial + 2 retries). This name is inherited from the shipping Ruby SDK; the behavior-model.json uses `retry.max` with identical semantics.

**Per-operation retry ceiling.** Each operation carries a per-op `retry.max` in behavior-model.json (200 ops at `3`, 43 at `2`). **TypeScript and Swift** drive their retry loops directly from this per-op value, which is unambiguous there because neither exposes a numeric client-wide cap — only an on/off (`enableRetry`). Generated Go, Python, Kotlin (`BasecampConfig.maxRetries`), and Ruby's governed GET path (`config.max_retries`) expose a numeric client cap *and* honor the per-op value as a **ceiling**: `effective_attempts = min(client_cap, op_max)`. The ceiling can only reduce attempts below the client cap, never raise them, so a client that lowered its cap (e.g. to `1` to disable retries) is still honored; governed paths coerce the cap to at least one attempt (`min(max(1, cap), op_max)`), so `0` yields a single attempt rather than none. Because every op's `max` is ≤ the default cap of `3`, a default or raised client makes exactly the per-op number of attempts in every capped SDK — matching TS/Swift. Observable changes from the former client-wide behavior, by client configuration:

- **Default client (`max_retries = 3`):** only the **11 idempotent `max:2` operations** (account/gauge/preference writes plus two subscription-style POSTs: `UpdateAccountName`, `UpdateAccountLogo`, `RemoveAccountLogo`, `UpdateMyPreferences`, `DisableOutOfOffice`, `MarkAsRead`, `ToggleGauge`, `UpdateGaugeNeedle`, `DestroyGaugeNeedle`, `Subscribe`, `EnableCardColumnOnHold`) change — they now retry at most twice instead of three times. The other 192 retry-eligible ops are unaffected (`min(3, 3) = 3`).
- **Client that raised its cap above 3:** **all 203 retry-eligible operations** are now clamped to their per-op `max` (192 to `3`, 11 to `2`) instead of retrying up to the raised cap. This is the intended meaning of a per-op ceiling and brings Go/Python into line with TS/Swift/Kotlin, which never retry beyond the per-op `max`. Go, Python, Kotlin, and Ruby's governed path all equally honor a caller who wants *fewer* attempts than the operation declares.
- **Client that lowered its cap to `1`:** unchanged — the cap still wins (`min(cap, op_max) = cap`). A cap of `0` is coerced to one attempt on governed paths (see the `max_retries = 0` divergence note in §2). Go, Python, and Ruby's governed GET path consume `max` **and** `retry_on` (the declared status gate); only the emitted `base_delay_ms`/`backoff` remain inert per-op metadata for them (retained for parity — see `scripts/check-retry-metadata-parity.py`). Ruby remains GET-only: mutations never retry there, so per-op metadata governs only its reads.

**Recommended default:** A connect timeout of 10 seconds is recommended but not a required config field. Only Ruby exposes this (Faraday `open_timeout = 10`); other SDKs use their HTTP library's default.

### Environment Variable Mapping (optional convention)

These environment variables are implemented in the Ruby SDK and recommended for new implementations. Go also loads environment overrides via `Config.LoadConfigFromEnv()` (supports `BASECAMP_BASE_URL`, `BASECAMP_PROJECT_ID`, `BASECAMP_TODOLIST_ID`, `BASECAMP_CACHE_DIR`, `BASECAMP_CACHE_ENABLED`). TypeScript and Kotlin do not currently load config from environment variables.

| Variable | Config field | Parse |
|----------|-------------|-------|
| `BASECAMP_BASE_URL` | `base_url` | string, strip trailing `/` |
| `BASECAMP_TIMEOUT` | `timeout` | integer seconds |
| `BASECAMP_MAX_RETRIES` | `max_retries` | integer |

### Validation Algorithm

All validation errors are `BasecampError(code: "usage")` (see §6 error taxonomy).

1. Parse `base_url`. → `⊥ BasecampError(code: "usage")` if malformed.
2. If `base_url` is not the default (`https://3.basecampapi.com`) and not localhost (§9), enforce HTTPS. → `⊥ BasecampError(code: "usage", message: "base URL must use HTTPS")` if scheme ≠ `https`.
3. Validate `timeout > 0`. → `⊥ BasecampError(code: "usage")` otherwise.
4. Validate `max_retries ≥ 1`. → `⊥ BasecampError(code: "usage")` otherwise. (`max_retries` is total attempts including the initial request; 0 would mean no request is made.) **Divergence:** the SDKs handle `max_retries = 0` in three distinct outcomes across four implementations, none of which is the spec's `⊥`:
   - **Generated Go** (low-level `pkg/generated` client) and **Python** (sync + async): accept `0` as a compatibility exception and make a single attempt with no retry. Both reject a *negative* value as a configuration error (generated Go: `WithRetryConfig`/`doWithRetry` return a plain `error`; Python: `Config` raises `ValueError` at construction).
   - **Kotlin:** the builder rejects a *negative* value (`require(maxRetries >= 0)`) and accepts `0`, which the transport coerces to a single attempt (`config.maxRetries.coerceAtLeast(1)`).
   - **Ruby:** `0` passes config validation. A **governed** GET (canonical operation ID present) coerces the cap to one attempt (`[config.max_retries, 1].max`) and makes a single request. An **ungoverned** GET keeps the old outcome: the retry loop's `break if attempt > max_retries` fires before the first request, so it makes **zero** requests and raises `Basecamp::ApiError("Request failed after 0 attempts")`.
   - **Hand-written Go** (`pkg/basecamp` client): rejects `0` — `NewClient` panics `"basecamp: max retries must be at least 1"` (its GET/download loops treat `MaxRetries` as the total attempt count with a minimum of 1).
5. Validate `max_pages > 0`. → `⊥ BasecampError(code: "usage")` otherwise.
6. Normalize `base_url`: strip trailing `/`.

---

## §3. Client Architecture

### Client Construction Algorithm

1. Accept auth options: exactly one of `access_token` (string or provider) or `auth` (AuthStrategy). **Go divergence:** Go takes a single `TokenProvider` interface directly rather than offering dual `access_token`/`auth` options; the exactly-one-of guard is a TS/Ruby/Kotlin/Swift pattern.
2. If both provided → `⊥ BasecampError(code: "usage", message: "Provide either auth or access_token, not both")`. `[static]`
3. If neither provided → `⊥ BasecampError(code: "usage", message: "Either auth or access_token is required")`. `[static]`
4. If `access_token` provided, wrap in `BearerAuth` strategy.
5. Validate config (§2 validation algorithm). **Go divergence:** Go's `NewClient` panics on validation failure rather than returning a `BasecampError`; all other SDKs return/throw a structured error.
6. Initialize HTTP transport with auth strategy, config, and optional hooks.
7. Expose `forAccount(accountId)` method that returns an `AccountClient`.

### AccountClient INTERFACE

```
INTERFACE AccountClient
  account_id  : String
  get(path, params)     → Response
  post(path, body)      → Response
  put(path, body)       → Response
  delete(path)          → Response
  paginate(path, params) → ListResult<Item> | Iterator<Item>  -- language adaptation (see §8)
  download_url(url)     → DownloadResult
END
```

### Service Placement Rule

- `authorization` → on Client (no account context; calls Launchpad endpoints)
- All other services → on AccountClient (account-scoped)

**TypeScript divergence:** TypeScript embeds `accountId` in the base URL (`https://3.basecampapi.com/{accountId}`) and exposes all services on a single flat `BasecampClient` — no separate `AccountClient`. The path construction still prepends `/{accountId}`, but it happens at client creation rather than per-request. This is a valid language adaptation.

### Account Path Construction `[conformance]`

Every account-scoped request prepends `/{accountId}` to the path:

```
FUNCTION buildURL(base_url, account_id, path) → String
  -- Internal to the HTTP transport layer. Callers (service methods) pass
  -- relative paths; only the transport passes absolute URLs (e.g., pagination
  -- follow-up URLs). This is not a public API surface.
  1. If path starts with "https://":
     a. If NOT isSameOrigin(path, base_url) → ⊥ BasecampError(code: "usage", message: "absolute URL must be same-origin as base_url").
     b. → return path unchanged.
  2. If path starts with "http://":
     a. If it is a localhost URL (see §9) AND isSameOrigin(path, base_url) → return path unchanged.
     b. Else → ⊥ BasecampError(code: "usage", message: "URL must use HTTPS or be same-origin localhost").
  3. If path does not start with "/" → prepend "/".
  4. → base_url + "/" + account_id + path
END
```

Conformance tests in `paths.json` verify correct path construction (e.g., `GetProjectTimeline` → `/999/projects/12345/timeline.json`).

### Service Initialization Pattern

Services are lazy-initialized, cached, and (where the language supports it) thread-safe. On first access, the service is constructed and stored; subsequent accesses return the cached instance.

---

## §4. Authentication

### AuthStrategy INTERFACE

```
INTERFACE AuthStrategy
  authenticate(headers: Headers) → void
    -- Mutates headers to apply authentication credentials.
    -- May be async (e.g., to fetch/refresh tokens).
END
```

### BearerAuth RECORD

The default strategy. Accepts a token as a static string or an async function that returns one:

```
RECORD BearerAuth implements AuthStrategy
  token : String | (() → async String)

  authenticate(headers) →
    1. resolved = (typeof token == function) ? await token() : token
    2. headers.set("Authorization", "Bearer " + resolved)
END
```

### Token Refresh (Go/Ruby extension)

Go and Ruby support automatic token refresh via a richer provider interface. TypeScript ships a `TokenManager` (`typescript/src/oauth/token-manager.ts`) that handles automatic refresh with deduplication, but it is an opt-in helper rather than built into the transport. Kotlin and Swift delegate refresh to the caller (the async function can internally handle refresh logic).

```
INTERFACE RefreshableTokenProvider
  access_token()  → String       -- returns current token
  refresh()       → Boolean      -- attempts refresh, returns success
  refreshable()   → Boolean      -- whether refresh is supported
END
```

**OAuthTokenProvider** (Go/Ruby only):
- Caches the access token and its expiry timestamp.
- Proactively refreshes when `expires_at - now() < TOKEN_REFRESH_BUFFER` (Go uses 300s; Ruby refreshes only on expiry).
- `refresh()` POSTs to the token URL with `grant_type=refresh_token`. Go's
  `AuthManager` additionally submits `client_id` when stored (BC5 public
  clients authenticate by id alone) and echoes/preserves the stored RFC 8707
  `resource` per §16's Token Response `resource` section. The Ruby and Python
  legacy token providers are Launchpad-only and out of the BC5 resource-echo
  scope.

### 401 Refresh-and-Retry Algorithm

1. Receive 401 response.
2. If the token provider supports refresh (`refreshable() == true`), refresh has not yet been attempted for this request, **and the attempt budget has another attempt left**:
   a. Call `refresh()`.
   b. If refresh succeeded, retry the request once with updated token.
   c. → response from retry.
3. → `⊥ BasecampError(code: "auth_required", http_status: 401)`.

Refresh is attempted at most once per request. Implementations track this with a boolean (e.g., `refresh_attempted`) rather than a counter.

**The replay spends an attempt.** It puts a request on the wire, so it draws
from the same total-attempt budget as a transient retry (§7): `max_retries`
counts requests, not failure kinds, and a cap of one means one request no
matter what would have caused the second. That is what makes §14's "disabling
retry yields exactly ONE hop-1 attempt" literally true — refresh included —
and it preserves the total-attempt semantics settled for observed attempts in
#461.

Step 2's budget gate is checked **before** `refresh()` is called, not after.
Refreshing a token the SDK has no budget left to use would burn a rotation for
nothing and still hand the caller the stale 401; declining to refresh at all is
both cheaper and easier to reason about. The consequence is worth stating
plainly: with a budget of one attempt, a refreshable 401 is NOT replayed and
surfaces as `auth_required`. Callers who want the refresh replay must leave an
attempt for it.

This gate applies wherever a total-attempt budget governs the path. Go's
hand-written mutation path has no such budget — mutations are deliberately
single-attempt for transient failures — and keeps its documented
mutation-specific single re-attempt after a successful refresh (see §7's
Cross-SDK Divergence).

---

## §5. Service Surface

### Client-Level Services (account-independent)

- **authorization** — identity lookup and account listing via Launchpad. Exposes `getInfo()` which GETs `https://launchpad.37signals.com/authorization.json` and returns `{expires_at, identity, accounts}`. Implemented in Go, Ruby, and TypeScript. Swift and Kotlin do not currently expose this service — a known gap. OAuth utility functions (PKCE, state generation, discovery, code exchange) are standalone helpers in §16, not service methods.

### AccountClient-Level Services (account-scoped) — 46 services

account, attachments, automation, boosts, campfires, cardColumns, cardSteps, cardTables, cards, checkins, clientApprovals, clientCorrespondences, clientReplies, clientVisibility, comments, documents, events, everything, forwards, gauges, hillCharts, lineup, messageBoards, messageTypes, messages, myAssignments, myNotifications, people, projects, recordings, reports, schedules, search, subscriptions, templates, timeline, timesheets, todolistGroups, todolists, todos, todosets, tools, uploads, vaults, webhooks, wormholes

**Total surface:** 1 client-level + 46 account-scoped = 47 services. The generated service index in each SDK (e.g. `typescript/src/generated/services/`) is the authoritative per-SDK surface; per-SDK counts vary slightly with split decisions — Go folds automation and client visibility onto other services, exposing 44 account-scoped accessors.

### Derivation Rule `[static]`

The OpenAPI spec uses 12 coarse tags (e.g., `Automation`, `Todos`, `Files`). The service generators split these into 46 fine-grained services using a two-table mapping: `TAG_TO_SERVICE` (tag → default service name) and `SERVICE_SPLITS` (tag → {service → [operationIds]}). For example, the `Todos` tag splits into `Todos`, `Todolists`, `Todosets`, `TodolistGroups`; the `Files` tag splits into `Attachments`, `Uploads`, `Vaults`, `Documents`. These mappings are defined in each language's generator script and produce identical service sets across SDKs.

### Merge-Safe Write Surface (Cards)

BC3 builds a card's update params as `{ due_on: nil }.merge(card_params)`
(`kanban/cards_controller.rb`), so **any** update whose body omits `due_on` erases the card's due
date. A sparse PUT — the natural thing to write, and what every generated SDK produced — is
therefore destructive.

- **`update`** — merge-safe. GETs the card and resends the existing `due_on` when the caller left it
  unaddressed, then PUTs. The extra GET is paid for only in that case; naming the due date
  explicitly skips it. `due_on` is tri-state: unaddressed preserves, an explicit empty clears, a
  date sets. Clearing is encoded by **omitting** `due_on` — never by sending null (§18).
- **`updateVerbatim`** — the raw single PUT, no read-before-write. Sharp by construction: omitting
  `due_on` clears it.

The composite deliberately does **not** resend everything. BC3 filters incoming assignee IDs through
`reachable_people`, so echoing assignees back would silently unassign anyone who has since lost board
access; only the caller's own `title`/`content`/`assignee_ids` go out, plus `due_on`.

Not atomic: a concurrent due-date change landing between the GET and the PUT is overwritten with the
value the call read. The window is one round-trip.

Presence detection is language-native: Go `*string` (nil preserves, pointer-to-empty clears),
TypeScript `dueOn?: string | null`, Ruby/Python `nil`/`None` kwarg defaults with `""` to clear,
Kotlin nullable parameters with `""` to clear, Swift a `DueDate` enum (`.preserve`/`.clear`/`.on`)
because an optional cannot carry three states.

### Merge-Safe Write Surface (Todos)

The `PUT /{accountId}/todos/{todoId}` endpoint is **full replace, omission clears** (spec operation `ReplaceTodo`, `content` required, declared via `x-basecamp-write-semantics: {mode: "replace", clearsOmitted: true}` and the `write` clause in `behavior-model.json`). Every SDK exposes a three-method, two-state surface over it:

- **`update`** — merge-safe. GET the current todo → overlay only *explicitly-set* request fields → PUT the full representation. An omitted field is untouched, guaranteed; an explicitly-passed empty collection is a set (clears). Set-detection is language-native: Go zero-value guards, TypeScript `!== undefined`, Python/Ruby `None`/`nil` kwarg defaults, Kotlin `?.let`, Swift `if let`.
- **`edit`** — read-modify-write closure over the full writable state (`TodoFields`: content, description, assignee_ids, completion_subscriber_ids, due_on, starts_on, notify). Clear = set empty (`""`/`[]`); a closure error/throw aborts before the PUT. Python's form is a context manager (`with`/`async with`) whose `.result` holds the updated todo after clean exit (RuntimeError before completion).
- **`replace`** — the generated wire method: verbatim sparse PUT, no GET, omission clears, content required.

Full-state serialization (update/edit): content, description, and both ID lists are always sent (empties included, so clears survive); dates only when non-empty (the server clears an omitted date, and `""` is a format error); `notify` only when true (a send directive, never populated from GET).

**Hook contract:** update/edit compose the public get + replace, so hooks observe the wire operations under each SDK's native identities (conceptually one GetTodo + one ReplaceTodo; one ReplaceTodo for replace) — never a synthetic composite. This keeps retry/idempotency policy keyed to the wire operation and the mutation always observable/gateable as a replace. Precedent: `uploads.download` surfaces its constituent operations the same way.

**Race:** update/edit are read-modify-write, not atomic. There is no conditional-update signal on this endpoint; a concurrent write between the GET and PUT is overwritten — last write wins for the whole representation, with a window of one round-trip. Use `replace` to overwrite deliberately.

Conformance: `conformance/tests/todos_write.json` (`update-merge`, `edit-clear`, `replace-omission-clears`).

### Merge-Safe Write Surface (Todolists)

The `PUT /{accountId}/todolists/{id}` endpoint is **full replace, omission clears** (spec operation `UpdateTodolistOrGroup`, `name` required, declared via `x-basecamp-write-semantics: {mode: "replace", clearsOmitted: true}` and the `write` clause in `behavior-model.json`). BC3's `TodolistsController#update` runs `@recording.update! recordable: Todolist.new(params.require(:todolist).permit(:name, :description, :track_todolist))` — it builds a *brand-new* `Todolist` from only the permitted params and swaps the recordable wholesale, so a field absent from the body is `nil` on the replacement. The public API docs say the same thing outright: "Pass all existing parameters in addition to those being updated. Omitting a parameter will clear its value." `name` is not merely conventionally required: `Todolist` declares `validates :name, presence: true` and `Recording` declares `validates_associated :recordable, on: :update`, so omitting it is a 422, not a preserve.

The writable set is exactly `{name, description}`. `track_todolist` is **not** part of this surface: `wrap_parameters Todolist` defaults its `include:` to `Todolist.attribute_names` (`id`, `name`, `description`), so the key never reaches `params[:todolist]` on the JSON path, and the controller's own `toggle_hill_chart_tracking` is key-guarded (`return unless params.key?(:track_todolist)`) — a hill-chart side-effect directive, not recordable state.

Every SDK exposes the same three-method, two-state surface over it:

- **`update`** — merge-safe. GET the current list → overlay only *explicitly-set* request fields → PUT the full representation. An omitted field is untouched, guaranteed. Set-detection is language-native: TypeScript `!== undefined`, Python/Ruby `None`/`nil` kwarg defaults, Kotlin `?.let`, Swift `if let`, Go zero-value guards.

  In the five SDKs whose unset marker is distinct from the empty string, an explicitly-passed `""` is a set and therefore clears.

  **Go is the exception, and this bites in practice.** Its request struct uses zero-value guards (`if req.Description != ""`), so `""` *is* the unset marker: `Update` with an empty description does **nothing to that field** rather than clearing it. **To clear a field in Go, use `Edit` or `Replace` — not `Update`.**

  ```go
  // Does NOT clear the description — "" reads as "unaddressed".
  svc.Update(ctx, id, &UpdateTodolistRequest{Description: ""})

  // Clears it: Edit hands back the full writable state, where "" is unambiguous.
  svc.Edit(ctx, id, func(f *TodolistFields) error { f.Description = ""; return nil })

  // Or clear it verbatim, accepting that every unnamed field is replaced too.
  svc.Replace(ctx, id, &ReplaceTodolistRequest{Name: "Hardware"})
  ```

  This is a language-adaptation consequence of Go's absent/empty conflation, not a behavioural divergence in the composite: every SDK preserves unaddressed fields identically, and only the spelling of "clear this one" differs.
- **`edit`** — read-modify-write closure over the full writable state (`TodolistFields`: name, description). Clear = set empty (`""`); a closure error/throw aborts before the PUT. Python's form is a context manager (`with`/`async with`) whose `.result` holds the updated list after clean exit (RuntimeError before completion).
- **`replace`** — the generated wire method: verbatim sparse PUT, no GET, omission clears, name required. Renamed from the plain `update` via `METHOD_NAME_OVERRIDES` in all five service generators (§18 rule 6), so the raw single-request path stays reachable under a name that says what it does.

Full-state serialization (update/edit): both `name` and `description` are always sent, empties included, so clears survive. `description` is cleared by sending `""` — never by sending null (§18).

The endpoint is polymorphic, and more literally so than the name suggests: there is no separate group model or group write route at all. A "group" is just a `Todolist` whose parent is another `Todolist` (`Todolist.group?`), there is no `Todolist::Group` class, and the API-canonical routes mount `resources :groups, only: %i[ index create ]` — so *every* group write in the API namespace lands on `TodolistsController#update` through this same URI. BC3 renders both variants through `todolists/_todolist.json.jbuilder`, so a group carries `description` and `description_attachments` too and reports `"type": "Todolist"`, and the only structural discriminator is `groups_url` (list) XOR `group_position_url` (group). The composite is therefore deliberately **variant-agnostic**: it preserves `{name, description}` for a group exactly as for a list, with no type-sniffing. The spec models the response as the `TodolistOrGroup` union; the wire carries the recordable's flat JSON, and the SDKs read through that flat shape (see AGENTS.md, "Smithy Spec vs Actual API Responses"). Consolidating the three declared shapes into one truthful flat structure is tracked separately in #544.

**Go asymmetry.** Group-ness is service-static in Go: `TodolistGroupsService` has its own write path over the same `UpdateTodolistOrGroup` wire operation, where the other five SDKs expose no group update at all (their `TodolistGroups` split is List/Create/Reposition only). Go's group surface gets the raw method **renamed to `Replace`** — so the destructive path is honestly named — but deliberately gets **no merge-safe `update`/`edit`**. The `TodolistGroup` schema does not model `description` (nor `description_attachments`, `boosts_*`, `groups_url`), so a composite reading through that projection would PUT back a zero-valued description and erase it on every call — reintroducing the exact data loss this surface removes. Merge-safe group writes go through `todolists.update`, which addresses the same route through the full `Todolist` projection. `ReplaceTodolistGroupRequest` does carry a `description` field even though the response projection cannot return one: the request body is shared with todolists and the server accepts it, and without it a group replace would be unconditionally destructive with no caller recourse. `Replace` is verbatim by definition, so this offers control rather than the illusion of a merge. Modelling `description` on the group *response* shape is #544; a reflection test fails the build if `TodolistGroup` ever gains the field while the composite is still withheld.

**Hook contract:** update/edit compose the public get + replace, so hooks observe the wire operations under each SDK's native identities (conceptually one `GetTodolistOrGroup` + one `UpdateTodolistOrGroup`; one `UpdateTodolistOrGroup` for replace) — never a synthetic composite.

**Race:** update/edit are read-modify-write, not atomic. There is no conditional-update signal on this endpoint; a concurrent write between the GET and PUT is overwritten — last write wins for the whole representation, with a window of one round-trip. Use `replace` to overwrite deliberately.

Conformance: `conformance/tests/todolists_write.json` (`update-merge`, `update-group`, `edit-clear`, `replace-omission-clears`).

### Merge-Safe Write Surface (Documents)

The `PUT /{accountId}/documents/{documentId}` endpoint is **full replace, omission clears** (spec operation `ReplaceDocument`, declared via `x-basecamp-write-semantics: {mode: "replace", clearsOmitted: true}` and the `write` clause in `behavior-model.json`). BC3's `DocumentsController#update` runs `@recording.update! recording_attributes.merge(recordable: new_document)`, where `new_document` is `Document.new(params.require(:document).permit(:title, :content))` — it builds a *brand-new* `Document` from only the permitted params and swaps the recordable wholesale, so a field absent from the body is `nil` on the replacement. The public API docs say the same thing outright: "omitting a field clears its value."

The writable set is exactly `{title, content}`, and **both are optional** — this is the one place Documents diverges from Todolists, and it is measured rather than assumed:

- Omitting `title` returns `200` and the title becomes `"Untitled"`. `Document#title` is `super.presence || "Untitled"` (`app/models/document.rb:7-9`) and the attribute carries **no presence validation** — the model declares none, and neither `Recordable` nor `Recording` validates the recordable's title.
- Omitting `content` returns `200` and clears it.
- Neither is a `422`, so neither earns `@required` the way `ReplaceTodo`'s `content` or `UpdateTodolistOrGroup`'s `name` did. Modelling either as required would make the SDK reject a request the server accepts.

What BC3 *does* require is the wrapping `document` object: `params.require(:document)` raises `ActionController::ParameterMissing` when absent, and Rails `wrap_parameters` synthesizes that wrapper from a flat body only when the body carries at least one `Document` attribute name. So a body naming **neither** field is a `400`, pinned upstream by `test "publishing a draft document requires the full payload and preserves it"`. Go's `Replace` refuses that body locally rather than spending a round-trip on it; the other five leave it to the server.

Every SDK exposes the same three-method, two-state surface over it:

- **`update`** — merge-safe. GET the current document → overlay only *explicitly-set* request fields → PUT the full representation. An omitted field is untouched, guaranteed. Set-detection is language-native: TypeScript `!== undefined`, Python/Ruby `None`/`nil` kwarg defaults, Kotlin `?.let`, Swift `if let`, Go zero-value guards.

  In the five SDKs whose unset marker is distinct from the empty string, an explicitly-passed `""` is a set and therefore clears. **Go is the exception**, as for Todolists: `""` *is* its unset marker on `UpdateDocumentRequest`, so `Update` with an empty title does nothing to that field. To clear a field in Go, use `Edit` or `Replace`.

  `ReplaceDocumentRequest` is the one Go request here that does **not** use zero-value guards. On a verbatim replace, absent and explicitly-empty are different requests and only one of them is legal alone: a body naming neither field is a `400`, while `{"title": "", "content": ""}` is a legal full replacement that clears both. Zero-value guards conflate those, so both fields are `*string` — nil omits, a pointer to `""` sends. Their server *effect* happens to coincide for `title` (omitted and empty both read back as `"Untitled"`), but the SDK must not collapse a distinction the wire makes.
- **`edit`** — read-modify-write closure over the full writable state (`DocumentFields`: title, content). Clear = set empty (`""`); a closure error/throw aborts before the PUT. Python's form is a context manager (`with`/`async with`) whose `.result` holds the updated document after clean exit (RuntimeError before completion).
- **`replace`** — the generated wire method: verbatim sparse PUT, no GET, omission clears. Renamed from nothing — the wire operation itself is `ReplaceDocument`, so the generated method is `replace` by the ordinary naming algorithm. This is the `ReplaceTodo` route (#375), not the `METHOD_NAME_OVERRIDES` route Todolists and Cards took, and it ships **without a deprecated alias**: `UpdateDocument` is gone.

Full-state serialization (update/edit): both `title` and `content` are always sent, empties included, so clears survive. A field is cleared by sending `""` — never by sending null (§18), and never by omission, which would hand the clear back to the server's own rebuild and read as an accident rather than an intent.

**Read-side, the two fields are not symmetric, and this is the inverse of the write side.** `Document.title` is `@required` on the *response* schema and BC3 can never render it blank (`Document#title` is `super.presence || "Untitled"`), so an absent or null `title` in a 2xx body is a **malformed response**, not an empty title — coalescing it to `""` and sending that in the full-replace PUT would blank the real title on a call that only touched `content`. All six SDKs refuse it: Kotlin and Swift get it from the decoder (`val title: String`, `public let title: String`), and Go, Python, Ruby and TypeScript check explicitly, because their reads would otherwise yield the string zero value. `content` is optional on the response schema, so absent or null there is genuinely empty and `""` is what the server already holds. Optionality on the request (both fields) and requiredness on the response (`title` only) are separate facts and are modelled separately.

**Subscribers are the one field this surface must not touch, and the reason it could not ship earlier.** A full-representation PUT names neither `subscriptions` nor `notify`. BC3's `notify_param` defaults to `"custom"`, so `find_subscribers` used to run `where(id: params[:subscriptions])` → `where(id: nil)` → empty, and every sparse update to a **drafted** recording reset its subscriber list to the creator plus the updater. The list is also unreadable over the API — only `subscription_url` is emitted — so the composite could not have preserved it by resending. bc3 #12494 (`344581a379`) and #12501 (`2c0dafba13`) introduced `Recording::DraftSubscribers`, whose `update_subscribers?` is `params.key?(:subscriptions) || params.key?(:notify)`: a request addressing neither keeps the list it found. That predicate is what makes a merge-safe composite safe on a draft, and it is why this surface is pinned to a bc3 provenance at or after `2c0dafba13`.

**Publishing a draft is not modeled.** Setting `status: "active"` is how a draft is published, and BC3 rejects a status-only update for the same reason it rejects an empty body — the recordable params are required alongside. `status` is on `CreateDocument` but not on `ReplaceDocument`; a caller who needs to publish sends the full payload plus `status` through the raw HTTP surface. Modelling it is deferred rather than declined.

**Hook contract:** update/edit compose the public get + replace, so hooks observe the wire operations under each SDK's native identities (conceptually one `GetDocument` + one `ReplaceDocument`; one `ReplaceDocument` for replace) — never a synthetic composite.

**Race:** update/edit are read-modify-write, not atomic. There is no conditional-update signal on this endpoint; a concurrent write between the GET and PUT is overwritten — last write wins for the whole representation, with a window of one round-trip. Use `replace` to overwrite deliberately.

Conformance: `conformance/tests/documents_write.json` (`update-merge`, `edit-clear`, `replace-omission-clears`).

### Known Gaps (informational, not prescriptive)

- Go is missing a standalone `automation` service; `clientVisibility` is implemented on `RecordingsService` (not a separate service); uses singular `Timesheet` vs `timesheets`
- TypeScript flattens both tiers onto a single client object (no separate AccountClient exposed to consumers) — a valid language adaptation
- Ruby returns a lazy `ListEnumerator` (an `Enumerator` subclass carrying `ListMeta`) for pagination rather than an eager `ListResult`; the first page is fetched eagerly so `meta.total_count` is available at call time, while `meta.truncated` is final only once enumeration completes

---

## §6. Error Taxonomy

*Rubric-critical: 2A.1, 2A.3*

### BasecampError RECORD `[static]`

```
RECORD BasecampError extends Error
  code        : ErrorCode     -- categorical error code
  message     : String        -- human-readable description (truncated to MAX_ERROR_MESSAGE_LENGTH)
  hint        : String?       -- optional user-friendly resolution guidance
  http_status : Integer?      -- HTTP status code that caused the error
  retryable   : Boolean       -- whether the operation can be retried
  retry_after : Integer?      -- seconds to wait before retrying (from Retry-After header)
  request_id  : String?       -- X-Request-Id from response headers
  exit_code   : Integer       -- CLI-friendly exit code (derived from code)
END
```

**Go divergence:** Go's `Error` struct omits `retry_after`; retry delay is tracked on `RequestResult` instead. Go also exposes a `Cause` field (the underlying error) not present in this canonical RECORD — a language-specific extension.

### Error Code Table

Status-mapped codes are verified per the Verification column and are `[conformance]`-verified. Client-side codes (`usage`, `network`, `ambiguous`) and exit codes are `[static]`.

| Code | Exit Code | HTTP Status | Retryable | Description | Verification |
|------|-----------|-------------|-----------|-------------|-------------|
| `usage` | 1 | — | false | Client misconfiguration (invalid args, bad URL) | `[static]` |
| `not_found` | 2 | 404 | false | Resource not found | `[conformance]` |
| `auth_required` | 3 | 401 | false | Authentication required or token expired | `[conformance]` |
| `forbidden` | 4 | 403 | false | Insufficient permissions | `[conformance]` |
| `rate_limit` | 5 | 429 | true | Rate limit exceeded | `[conformance]` |
| `network` | 6 | — | true | Connection failure, timeout, DNS | `[static]` |
| `api_error` | 7 | 500, 502, 503, 504 | true | Server-side error | `[conformance]` |
| `ambiguous` | 8 | — | false | Multiple matches found (CLI disambiguation) | `[static]` |
| `validation` | 9 | 422 | false | Request validation failed | `[conformance]` |
| `validation` | 9 | 400 | false | Request validation failed | `[conformance]` |

### HTTP Status Mapping Algorithm

Each explicitly enumerated status mapping below (steps 1–10) is `[conformance]`-verified. The two catch-all fallback steps (11: general 5xx; 12: any other non-mapped status) have no dedicated conformance case and are `[static]`.

Given an HTTP response with status code `status` and body `body`:

1. If `status == 401` → `BasecampError(code: "auth_required", http_status: 401, retryable: false)`.
2. If `status == 403` → `BasecampError(code: "forbidden", http_status: 403, retryable: false)`.
3. If `status == 404` → `BasecampError(code: "not_found", http_status: 404, retryable: false)`.
4. If `status == 429` → `BasecampError(code: "rate_limit", http_status: 429, retryable: true, retry_after: parseRetryAfter(headers))`.
5. If `status == 400` → `BasecampError(code: "validation", http_status: 400, retryable: false)`.
6. If `status == 422` → `BasecampError(code: "validation", http_status: 422, retryable: false)`.
7. If `status == 500` → `BasecampError(code: "api_error", http_status: 500, retryable: true)`.
8. If `status == 502` → `BasecampError(code: "api_error", http_status: 502, retryable: true)`.
9. If `status == 503` → `BasecampError(code: "api_error", http_status: 503, retryable: true)`.
10. If `status == 504` → `BasecampError(code: "api_error", http_status: 504, retryable: true)`.
11. If `status >= 500` → `BasecampError(code: "api_error", http_status: status, retryable: true)`. `[static]`
12. Otherwise → `BasecampError(code: "api_error", http_status: status, retryable: false)`. `[static]`

In all cases, extract `request_id` from `X-Request-Id` response header if present. `[conformance]`

### Statusless `api_error` for a malformed 2xx body `[manual]`

The mapping above is keyed on an HTTP status, because it maps *failed* responses. A composite (§18) can also fail on a **successful** one: the transport returned 2xx, and the body is malformed in a way that makes the composite's next step unsafe — a writable field of the wrong type, or a required field absent, on a read the composite is about to echo back into a full-replace write.

That error is `api_error` with **no `http_status`** and **`retryable: false`**. Statusless because no status describes it (the request succeeded), and non-retryable because re-requesting cannot repair a malformed body. It is deliberately *not* `usage`/`validation`: the value came off the wire, so nothing the caller passed is at fault. The mirror case — the *caller* supplying the offending value — stays a usage error. **Classification is by origin, not by value:** the same empty string is a caller error when the caller passed it and a malformed response when the server did, so each provenance is checked where it is unambiguous (the read step owns the response, the write step owns the caller).

Message is truncated to `MAX_ERROR_MESSAGE_LENGTH` like any other (§9) — the malformed value is embedded in it, so the cap is load-bearing rather than cosmetic.

### Error Body Parsing Algorithm

1. Attempt to parse `body` as JSON.
2. If JSON and has `"error"` key (string value) → use as `message`.
3. If JSON and has `"error_description"` key (string value) → use as `hint`.
4. Else if JSON and has `"message"` key (string value) and `message` not yet set → use as `message`.
5. If parsing fails or body is empty → use HTTP status text as `message`.
6. Truncate `message` to `MAX_ERROR_MESSAGE_LENGTH` (see §9).

Note: `"error"` takes precedence over `"message"` — step 4 is a fallback for APIs that use `"message"` instead of `"error"`.

### Field-Keyed Validation Bodies (400/422) `[conformance]`

BC5 controllers that render Rails `RecordInvalid` emit a field-keyed 422 body instead of the flat `{"error": ...}` shape (e.g. `UpdateCalendar` rejecting an unknown color):

```json
{"errors": {"color": ["is not a valid color"]}}
```

Other controllers render the same `ActiveModel::Errors` payload with no wrapper at all — `render json: @webhook.errors` — so the field map arrives as the whole body (webhooks and chat integrations at 400, message-type categories at 400, lineup markers at 422):

```json
{"payload_url": ["is not a valid URL"]}
```

For `status == 400` or `status == 422` only:

1. If the parsed JSON body has an `"errors"` key whose value is an object, build `field_errors`: for each entry whose value is an array, keep its string elements; skip entries whose value is not an array and entries with no usable messages. If no entries remain, treat the map as absent. The map is parsed independently of the scalar members: a non-string `"error"` or `"error_description"` value (ignored per the Error Body Parsing Algorithm's string-value requirement) must not prevent field-error extraction — `{"error": {}, "errors": {...}}` still yields the flattened message and the structured slot.
2. Otherwise, if the body is a non-empty object carrying no `"errors"` key, and **every** member's value is a non-empty array whose elements are all non-empty strings, the body itself is the field map: `field_errors` is the body. This gate is deliberately stricter than step 1's per-entry filtering, and the asymmetry is the point — an explicit `"errors"` key already declares the body's intent, so a partly malformed map is still unambiguously a field map, whereas an unwrapped body is recognizable by shape alone. One non-conforming member means it is some other JSON object and must not be reinterpreted as validation detail.

   `"errors"` is the only structurally reserved key, because it belongs to step 1. `"error"` and `"message"` are **not** excluded by name: a flat body carries them as strings, and the shape gate already rejects a string-valued member — so `{"error": "Webhook is invalid", "payload_url": ["is invalid"]}` stays flat without a name-based rule, while a record whose validated attribute happens to be called `message` still gets `{"message": ["can't be blank"]}` recognized.
3. Flatten `field_errors` into a single string: fields sorted lexicographically, each rendered as `{field}: {msg1}; {msg2}` (a field's messages joined with `"; "`), fields joined with `", "`. This shape is shared by all six SDKs — change it everywhere or nowhere.
4. Compose the error message: appended in parentheses after the top-level message when both are present (`{message} ({flattened})`), standing alone when only the field map is present. The top-level message comes from the Error Body Parsing Algorithm above — including its `"message"`-key fallback, so `{"message": "Validation failed", "errors": {...}}` composes just like the `"error"`-keyed shape. A bare field map (step 2) never has a top-level message by construction, so it always stands alone. Truncation to `MAX_ERROR_MESSAGE_LENGTH` (§9) applies to the composed result — after flattening — so the appended tail is capped too.
5. Expose the raw map as a structured slot on the validation error (idiomatic spelling per language: `FieldErrors` / `fieldErrors` / `field_errors`; Swift carries it as the fifth associated value of `.validation` plus a `fieldErrors` computed property on `BasecampError`), preserving the raw, untruncated per-field messages. The slot is `nil`/`null`/`None`/`undefined` for every other error shape, including non-validation statuses whose bodies happen to carry an `errors` key.

Field names are data, never structure. Once a field map is recognized, no name is privileged: `"base"` — Rails' record-level error key — renders as an ordinary field (`base: Can't be undocked`), and `"__proto__"` is an ordinary key rather than a prototype mutation. The one place a name carries meaning is step 2's `"errors"` check, which is shape recognition on an unwrapped body — deciding *whether* this JSON object is a field map at all — not a claim about what a field may be called.

Swift carries the slot as a fifth associated value on `.validation` plus a `fieldErrors` property on `BasecampError`; the earlier flatten-only deviation is closed.

### Retry-After Parsing Algorithm

Given header value `value`:

1. Attempt parse as integer. If valid and > 0 → return as seconds.
2. Attempt parse as HTTP-date (RFC 7231, e.g., `Wed, 09 Jun 2021 10:18:14 GMT`). If valid → compute `max(0, date - now())` in seconds; if > 0 → return.
3. → `undefined` (fall through to backoff formula).

---

## §7. Retry

*Rubric-critical: 2B.4*

### Three-Gate Precedence Algorithm `[conformance]`

Retry eligibility is determined by three sequential gates. All three must pass for a retry to occur.

**Gate 1 — HTTP method default:**

| Method | Default Retry | Rationale |
|--------|--------------|-----------|
| GET, HEAD | retryable | Read-only, naturally idempotent |
| PUT, DELETE | retryable | Naturally idempotent |
| POST | NOT retryable | May create duplicate resources |

**Gate 2 — Idempotency override (POST only):**

If `behavior-model.json` marks an operation with `idempotent: true`, the POST becomes retryable. The `retry` block present on non-idempotent POSTs is **inert metadata** — it describes what retry parameters WOULD apply if the operation were retryable, but does not activate retry. The `idempotent` flag is the sole gate for POST retry eligibility.

**Gate 3 — Error retryability:**

The error must be retryable. Two categories qualify:

- **HTTP status retry:** Response status is in the operation's **declared** retryable set. `behavior-model.json` specifies `retry_on: [429, 503]` for all operations. The declared set is **exhaustive**: a status outside it — including 500, 502, and 504 — is not retried and is surfaced to the caller on the first attempt. An implementation may still *classify* those statuses as retryable in its error taxonomy (§6); that is a caller-facing hint and must not widen the transport's gate.
- **Network error retry:** Connection failures, timeouts, and DNS errors (no HTTP response received) are retryable. These correspond to `BasecampError(code: "network", retryable: true)` in §6. **Divergence:** **Go, Python, Swift, TypeScript, and Kotlin** retry network errors for retry-eligible operations — including idempotent mutations — with Swift, Go, TypeScript, and Kotlin gating on operation idempotency, so a non-idempotent POST is attempted once. TypeScript additionally treats caller aborts and request timeouts as terminal, and Kotlin carves out the whole-request time budget (Ktor's `HttpRequestTimeoutException`): once the caller's configured request timeout has elapsed, the failure surfaces without retry. **Ruby** retries network errors too, but only on GET: its transport routes every non-GET to a single-attempt path, so no mutation ever sees a network retry. The spec prescribes network error retry as the target behavior.

**Non-retryable statuses (never retry regardless of method):** 401, 403, 404, 400, 422.

**Gate 3 consumption `[CONFLICT]`.** Gates 1 and 2 read `behavior-model.json`. Gate 3's parameters
are consumed unevenly:

| | Gates status retry on the declared `retryOn` | Honors a caller asking for *fewer* attempts than the operation declares |
|---|---|---|
| Go | yes | yes — `min(caller_cap, operation_max)` |
| Python | yes | yes — `min(caller_cap, operation_max)` |
| TypeScript | yes | n/a — exposes no numeric cap (only `enableRetry`), so the operation value is the only input |
| Swift | yes | n/a — exposes no numeric cap (only `enableRetry`) |
| Kotlin | yes | yes — `min(caller cap, operation max)`, with the cap coerced to at least one attempt |
| Ruby | yes for governed GETs — a status-bearing error retries exactly when the declared `retryOn` says so; the error taxonomy's 500/502/504 classification neither widens nor vetoes the declared set. Status-less network errors keep the taxonomy's judgment | yes — governed GETs are bounded by `min(caller cap, operation max)`; Ruby's transport is GET-only, so mutations never reach the retry loop |

TypeScript and Swift expose no numeric cap, so the operation value is the only attempts input there;
every SDK with a cap now resolves it as a ceiling (#485).

**Ungoverned traffic.** A transport may carry requests that are not Smithy operations — today the
Launchpad authorization request. Those carry no behavior-model metadata, and the declared policy
must **not** be applied to them: they keep whatever contract the SDK gives non-generated traffic.
Python enforces this structurally — `get_absolute()` passes no operation id, so both the status gate
and the attempt ceiling no-op — and pins it with sync and async regressions that fail if generated
policy ever reaches OAuth traffic.

**Enforcement.** `scripts/check-retry-metadata-parity.py` asserts every SDK's emitted per-operation
retry metadata equals `behavior-model.json`, and records which fields each SDK actually consumes at
runtime. It deliberately does not assert *effective behavior* parity, since Ruby's GET-only
transport is intentional. Effective Gate 3 behavior is pinned by tests in Go, Python, TypeScript,
Kotlin, and Ruby.

### Cross-SDK Divergence `[CONFLICT]`

- **TypeScript** implements the three-gate algorithm with the retry loop beneath the openapi-fetch middleware chain (the client's custom `fetch`), so attempts run to each operation's declared `retry.max` and network errors retry under the same idempotency gate; caller aborts and request timeouts are terminal. **Kotlin** implements the three-gate algorithm for both HTTP status and network-error retries (POST retries only when `idempotent: true`, full exponential backoff): one eligibility gate covers both failure shapes, with the whole-request timeout (Ktor's `HttpRequestTimeoutException`) deliberately not retried and auth headers re-attached per attempt.
- **Go** implements the three-gate on its generated operation path: it retries operations classified idempotent at generation time — GET/HEAD by method, plus any operation carrying `x-basecamp-idempotent` (the naturally-idempotent PUT/DELETE mutations like `UpdateProject`/`TrashProject`, and the flagged-idempotent POSTs like `CompleteTodo`) — with exponential backoff; non-idempotent operations (e.g. `CreateTodo`) are single-attempt. The separate hand-written `doRequestURL` helper remains GET-only for ordinary retries, with a mutation-specific single re-attempt after successful 401 token refresh.
- **Ruby** is stricter: only GET retries; all non-GET methods do not retry. Governed GETs (those carrying their canonical operation ID) are bounded by the per-op ceiling and status-gated on the declared `retryOn`; ungoverned GETs (`get_absolute`, OAuth discovery) keep the taxonomy-driven pre-metadata contract. Ruby is acceptably conservative.
- **Swift** implements the three-gate algorithm: the transport retries only when the method is naturally idempotent (GET/HEAD/PUT/DELETE) **or** the operation is marked `idempotent: true`, so non-idempotent POSTs like `CreateProject` are attempted exactly once while the seven idempotent POSTs (`CompleteTodo`, `CreateBookmark`, `EnableCardColumnOnHold`, `PauseQuestion`, `PrioritizeAssignment`, `Subscribe`, `SubscribeToCardColumn`) keep retrying. The gate covers both retry paths — HTTP status (`429`/`503`) and network errors — so Swift retries network errors but only for retry-eligible operations. A network error is classified by *meaning*, not by type: a `Transport` that reports connectivity failure as the SDK's own `BasecampError.network` reaches the retry branch exactly as a raw `URLError` does (#567). `Transport` is `public`, so that normalization is the natural implementation and must not be the one that disables retry. Any other `BasecampError` out of the transport (`.auth`, `.usage`, `.api`, …) stays terminal on sight, and the non-HTTP-response guard raises a distinct internal error so a deterministic programming fault is never mistaken for a transport blip. `BaseService` threads the per-operation flag from generated `Metadata` into the transport; the naturally-idempotent method set is allowlisted so PATCH/OPTIONS and future methods stay fail-closed.
- The spec prescribes the three-gate algorithm.

### Retry Algorithm

```
FUNCTION executeWithRetry(request, retry_config) → Response
  -- retry_config has fields: max_attempts, base_delay_ms, retry_on, backoff.
  -- These map to behavior-model.json fields: retry.max → max_attempts,
  -- retry.base_delay_ms → base_delay_ms, retry.retry_on → retry_on.

  1. Determine retry eligibility:
     a. method = request.method
     b. If method is POST:
        - Look up operation in behavior-model.json by operationId
          (the generated service passes the operationId directly as
          the behavior-model.json key)
        - If operation.idempotent ≠ true → retry_config = NO_RETRY_CONFIG (max_attempts=1)
     c. If method is GET, HEAD, PUT, DELETE → use retry_config as passed
        by the caller (the generated service provides per-operation
        retry_config from behavior-model metadata; DEFAULT_RETRY_CONFIG
        is the fallback when no per-operation config exists)

  2. last_error = null
     last_response = null

  3. For attempt = 0 to retry_config.max_attempts - 1:
     a. Invoke hooks.on_request_start(RequestInfo{method, url, attempt+1}).
     b. Execute request → (response, error).
        - On success: last_response = response, last_error = null.
        - On network error: last_response = null, last_error = error.
     c. Construct request_result: RequestResult from last_response (or from last_error for network errors).
     d. Invoke hooks.on_request_end(RequestInfo{method, url, attempt+1}, request_result).

     e. If last_error (network error):
        - If attempt == retry_config.max_attempts - 1 → raise last_error.
        - Else → go to step 3h (skip status check, no Retry-After header).
     f. If last_response.status NOT IN retry_config.retry_on → return last_response.
     g. If attempt == retry_config.max_attempts - 1 → return last_response.

     h. Calculate delay:
        - If last_response exists and has valid Retry-After header →
          delay = parsed value × 1000 (Retry-After is in seconds; delay is in ms).
        - Else → delay = backoff formula (see below).
     i. retry_error = if last_response, construct BasecampError from HTTP status;
        if network error, use last_error.
     j. Invoke hooks.on_retry(RequestInfo{method, url, attempt+1}, attempt+2, retry_error, delay).
        -- RequestInfo.attempt = attempt+1: the 1-based attempt that just failed
        --   (1 = initial request failed, 2 = first retry failed, etc.)
        -- Standalone attempt = attempt+2: the 1-based attempt about to happen
        --   (2 = about to do first retry, 3 = about to do second retry, etc.)
        -- This matches shipped SDKs: all six pass the failed attempt in
        -- RequestInfo and the next attempt number as the standalone parameter.
     k. Sleep delay ms.
     l. Refresh auth headers (token may have been refreshed during sleep).
END
```

The loop always terminates via step 3e (raise on network error), 3f (return non-retryable response), or 3g (return on exhaustion). `on_request_start`/`on_request_end` are invoked per attempt within the loop; `on_operation_start`/`on_operation_end` are invoked by the calling layer (generated service method), not by the retry transport.

### Backoff Formula

```
delay = min(base_delay_ms * 2^(retry_index), MAX_BACKOFF_DELAY_MS) + random(0, max_jitter)
```

Where `retry_index` is the 0-indexed retry count (first retry = 0, second retry = 1, etc.). In the `executeWithRetry` loop, `retry_index = attempt` — when the initial request (attempt=0) fails and reaches step 3h, it computes the delay for the first retry using `2^0 = 1×base_delay_ms`. Default constants (from `retry_config` or Config):
- `base_delay_ms` = 1000 (from `retry_config.base_delay_ms`)
- `max_jitter` = 100ms (from Config; not part of `retry_config` — sourced from the client's Config RECORD)
- `MAX_BACKOFF_DELAY_MS` = 30,000 (30s) — the ceiling on the backoff term, below

Retry-After header value takes precedence when present and valid.

### Backoff Ceiling `[CONFLICT]`

The backoff term is **saturating**: it grows exponentially up to `MAX_BACKOFF_DELAY_MS`
and then stops. This is a correctness requirement, not a politeness one — an unbounded
`2^n` is not merely a long sleep, it is a different failure in every host language, and
each of the six SDKs demonstrated one before #577:

| Overflow shape | SDKs | What actually happens |
|---|---|---|
| Signed 64-bit wrap | Kotlin (`1L shl`), Go (`1<<`) | The product goes negative and the sleep primitive treats a non-positive delay as "no delay" — the client tight-loops against a server that is already answering 429/503, which is the exact traffic pattern backoff exists to prevent |
| Trap on overflow | Swift (`UInt64` multiply) | The process **crashes**. Swift's `<<` is a smart shift, so an over-shift silently yields `0` (tight loop), but at `1 << 63` the multiply overflows `UInt64` and traps |
| Saturate to infinity | TypeScript (`Math.pow`) | `Infinity` reaches `setTimeout`, which clamps an out-of-range delay to **1ms** — a tight loop again |
| Unbounded integer | Python (`2 **`), Ruby (`2**`) | No wrap: the multiplier becomes an arbitrary-precision bignum. Python raises `OverflowError` converting it to a float; Ruby coerces to `Float::INFINITY` and `sleep` never returns |

Requirements:

1. **Saturate, never wrap, never diverge.** An implementation must not evaluate an
   unbounded `2^retry_index`. Compare against `MAX_BACKOFF_DELAY_MS / base_delay_ms`
   *before* multiplying — either the multiplier itself, or, where the power is the
   thing that overflows, the exponent against the point at which the term first
   reaches the ceiling. That keeps every intermediate inside the host's numeric range
   **and** lands the term on the ceiling.

   A **fixed** exponent cap followed by `min(base × 2^capped, MAX_BACKOFF_DELAY_MS)`
   is not an acceptable substitute, however generous the cap looks. It bounds the
   intermediate but not the *outcome*: for a base delay small enough that
   `base × 2^cap < MAX_BACKOFF_DELAY_MS`, the term plateaus below the ceiling for
   every subsequent attempt instead of saturating at it. At `base_delay_ms = 1e-30`
   a cap of 64 pins every attempt from the 65th onward at ~1.84e-11s — which is
   requirement 1's tight loop, not a fix for it.

   The bound must be derived from the base **without overflowing its own
   arithmetic**. `MAX_BACKOFF_DELAY_MS / base_delay_ms` is itself infinite once
   `base_delay_ms` drops below `MAX_BACKOFF_DELAY_MS / MAX_FLOAT`, and falling back
   to a fixed exponent there fails in the mirror direction: the term saturates
   **early**, returning the ceiling at an attempt whose specified value is still far
   below it. Compute the crossing in the log domain (`log2(ceiling) - log2(base)`)
   and scale the term directly — `ldexp`, or repeated bounded multiplication where
   the host has no `ldexp` — so no fixed exponent cap is needed at all. Saturating
   early is as much a deviation as plateauing: the term must track
   `base × 2^retry_index` at every attempt below the ceiling and equal the ceiling
   at every attempt at or above it.
2. **The ceiling bounds the backoff term, not the total sleep.** Jitter is added after
   clamping, so the longest single backoff sleep is `MAX_BACKOFF_DELAY_MS + max_jitter`.
   This matches Go's generated client, which has capped at `RetryConfig.MaxDelay = 30s`
   since it was first templated; 30s is adopted as the cross-SDK constant because it is
   the one value already shipping.
3. **The ceiling applies to `linear` and `constant` backoff too**, and therefore to
   `base_delay_ms` itself: a caller configuring a base delay above the ceiling gets the
   ceiling. The rule is "no single computed backoff sleep exceeds `MAX_BACKOFF_DELAY_MS`",
   with no carve-out for the first one. No shipped configuration approaches it —
   `behavior-model.json` tops out at `base_delay_ms: 2000`, and the default three
   attempts never compute past `2000 × 2 = 4000ms`, so the ceiling is unreachable on
   default paths and changes no shipped behavior.
4. **`Retry-After` is exempt.** It is server-directed and takes precedence per step 3h;
   the ceiling governs the locally-computed formula only. Implementations may still
   bound it against host limits — Swift clamps its seconds→nanoseconds conversion to
   86,400s because `UInt64(_:)` on an out-of-range `Double` is a trap.

**Reachability.** Every SDK exposes a path to a high attempt count: Kotlin's builder
validates `maxRetries >= 0` with no upper bound, Go's `WithMaxRetries` only rejects
`n < 1`, and Python/Ruby take a caller cap that is intersected with — never raised
above — the per-operation max, so a caller who *lowers* the cap is fine but the
operation ceiling itself is whatever `behavior-model.json` says. Reaching the overflow
needs a long genuine failure streak, so this is a robustness gap rather than a live
incident; the consequences (crash, tight loop, infinite sleep) are severe enough that
the gate belongs in the spec rather than in six independent judgment calls.

### Default and No-Retry Configs

```
RECORD DEFAULT_RETRY_CONFIG
  max_attempts : 3
  base_delay_ms : 1000
  backoff      : "exponential"
  retry_on     : [429, 503]
END

RECORD NO_RETRY_CONFIG
  max_attempts : 1
  base_delay_ms : 0
  backoff      : "constant"
  retry_on     : []
END
```

### behavior-model.json Retry Patterns

All 243 operations in `behavior-model.json` use `retry_on: [429, 503]`. Three `(max, base_delay_ms)` patterns exist:
- `(2, 1000)` — most create operations
- `(3, 1000)` — most read/update/delete operations
- `(3, 2000)` — `CreateAttachment`, `CreateCampfireUpload` (file uploads)

---

## §8. Pagination

*Rubric-critical: 2C.5*

### ListResult RECORD

```
RECORD ListResult<T>
  items : List<T>    -- the items (may extend Array, wrap List, or use language-appropriate collection)
  meta  : ListMeta
END

RECORD ListMeta
  total_count : Integer   -- from X-Total-Count header; 0 if absent
  truncated   : Boolean   -- true only when items beyond those returned were available: a next Link was present or excess items were discarded. Landing exactly on the final item is not truncation.
  next_url    : String?   -- URL of the next page when truncated; not populated by all SDKs (optional field)
END
```

### Link Header Parsing Algorithm `[conformance]`

```
FUNCTION parseNextLink(linkHeader: String?) → String?
  1. If linkHeader is null or empty → return null.
  2. Split linkHeader by ",". (Basecamp's API does not produce URLs with bare commas in Link headers, so naive comma splitting is safe. A general-purpose implementation could use RFC 8288-aware parsing.)
  3. For each part:
     a. Trim whitespace.
     b. If part contains 'rel="next"':
        - Extract URL between < and >.
        - Return URL.
  4. → null (no next link found).
END
```

### Auto-Pagination Algorithm `[conformance]`

```
FUNCTION paginate(initial_response, max_pages, max_items?) → ListResult<T>
  1. Parse first_page_items from initial_response body.
  2. total_count = parse X-Total-Count header (0 if absent).
  3. all_items = first_page_items.
  4. If max_items set and all_items.length ≥ max_items:
     a. has_more = parseNextLink(initial_response.headers["Link"]) ≠ null OR all_items.length > max_items.
     → ListResult(all_items[0:max_items], meta: {total_count, truncated: has_more}).

  5. response = initial_response.
  6. For page = 1 to max_pages - 1:
     a. raw_next_url = parseNextLink(response.headers["Link"]).
     b. If raw_next_url is null → break.
     c. next_url = resolveURL(response.url, raw_next_url).
     d. Validate same-origin (see below). If fails → ⊥ BasecampError.
     e. response = authenticatedFetch(next_url).
     f. Parse page items, append to all_items.
     g. If max_items set and all_items.length ≥ max_items:
        a. has_more = parseNextLink(response.headers["Link"]) ≠ null OR all_items.length > max_items.
        → ListResult(all_items[0:max_items], meta: {total_count, truncated: has_more}).

  7. truncated = parseNextLink(response.headers["Link"]) ≠ null.
  8. → ListResult(all_items, meta: {total_count, truncated}).
END
```

### Pagination Variants

Three response shapes exist across the API:

| Variant | Response shape | Extraction |
|---------|---------------|------------|
| **Bare array** | `[item, item, ...]` | Parse body as array |
| **Keyed array** | `{"events": [item, ...]}` | Extract items from named key |
| **Wrapped response** | `{"wrapper_field": ..., "events": [item, ...]}` | Return wrapper fields + paginated items from named key |

The variant is determined at code-generation time from the OpenAPI response schema and encoded in the generated service method (via `x-basecamp-pagination` extension or response schema analysis).

**Wrapped response pagination:** For endpoints that return a wrapper object with a paginated array inside (e.g., `personProgress` returns `{person, events: [...]}`), the generated service method paginates the embedded array while preserving the wrapper fields from the first page. The `paginate` algorithm above handles item extraction; the wrapping/unwrapping is a code-generation concern, not a transport concern. See `typescript/src/generated/services/reports.ts` and `go/pkg/basecamp/timeline.go` for reference implementations.

### The `page` Query Parameter

Operations whose Basecamp endpoint honors `?page=` accept a `page` query parameter. **Its meaning currently differs between Go and the five auto-paginating SDKs, and callers must know which they are using.**

Two carve-outs, so the rule above is not read as universal:

- `ListWebhooks`, `ListMessageTypes`, `ListChatbots`, `ListPingablePeople`, `ListQuestionAnswerers`, and `ListUploadVersions` carry the pagination trait but declare **no** `page` parameter: their Basecamp index actions return the whole collection rather than paginating, so there is no page to select.
- `GetMyNotifications` declares `page` but carries **no** pagination trait, so no SDK follows links for it and everything below is inapplicable — it returns the page you asked for, in all six.

| SDK | Behavior with `page = 3` | Requests issued |
|-----|--------------------------|-----------------|
| Go | Returns exactly page 3; auto-pagination is suppressed. | 1 |
| TypeScript, Python, Ruby, Kotlin, Swift | Fetches page 3, then follows `Link: rel="next"` to the end of the collection, returning pages 3..N concatenated. | N - 2 |

The divergence is structural: `page` rides in the query string of the *first* request only, while every subsequent request comes from the `Link` header. The auto-pagination algorithm above has no notion of a pinned page, so `page` acts as a starting offset rather than a selector. Go escapes this because its hand-written wrappers short-circuit before the follow loop when `Page > 0`.

`max_items` bounds the walk but is not a page selector: it caps *items*, so it collapses to a single request only when the cap does not exceed that page's item count, which requires the caller to know the server's page size.

This behavior predates the operations added in #561 — it has held since the first operations declared `@httpQuery("page")`. Converging all six SDKs on the Go semantics is tracked in issue #566; until then, the divergence is documented rather than silent, and SDK doc comments for `page` point here.

### Same-Origin Validation Algorithm `[conformance]`

```
FUNCTION isSameOrigin(a: String, b: String) → Boolean
  1. Parse a and b as URLs.
  2. If either parse fails → return false.
  3. If either has no scheme → return false.
  4. Compare: scheme (case-insensitive) AND normalizeHost (case-insensitive).
  5. → true if match, false otherwise.
END

FUNCTION normalizeHost(url: URL) → String
  1. host = url.hostname (lowercase).
  2. port = url.port.
  3. If port is empty → return host.
  4. If scheme is "https" and port is 443 → return host (strip default port).
  5. If scheme is "http" and port is 80 → return host (strip default port).
  6. → host + ":" + port.
END
```

Cross-origin pagination Link headers are rejected to prevent SSRF and token leakage. `[conformance]`

Protocol downgrade (HTTPS → HTTP) in Link headers is also rejected. `[conformance]`

---

## §9. Security

*Rubric-critical: 3C.1*

### HTTPS Enforcement `[conformance]`

All API requests must use HTTPS. Exception: localhost addresses are permitted for development and testing. Conformance tests verify the general rule (non-localhost HTTP rejected) and basic localhost exemption.

**Localhost carve-out** `[static]` — the following are recognized as localhost (only `localhost` is conformance-tested; the remaining forms are `[static]` contract):
- `localhost` (exact) `[conformance]` — all SDKs
- `127.0.0.1` — all SDKs
- `::1` — Go, Ruby, TypeScript (Swift and Kotlin require bracket-wrapped URL form `http://[::1]:...`; bare `http://::1` does not parse as a valid URL in either language)
- `[::1]` (bracket-wrapped IPv6) — Go, Ruby, TypeScript, Swift, Kotlin
- `*.localhost` (any subdomain, per RFC 6761) — Go, Ruby, TypeScript only (Swift and Kotlin do not recognize subdomain patterns)

Client construction with a non-HTTPS, non-localhost base URL must fail with `BasecampError(code: "usage")`. `[conformance]`

### Response Body Size Cap

```
MAX_RESPONSE_BODY_BYTES = 52,428,800  (50 MiB, i.e., 50 × 1024 × 1024)
MAX_ERROR_BODY_BYTES    = 1,048,576   (1 MiB)
```

Go and Ruby enforce this limit. TypeScript, Kotlin, and Swift do not currently enforce it — they rely on the HTTP library's native limits. New implementations should enforce it. `[static]`

### Error Message Truncation `[static]`

```
MAX_ERROR_MESSAGE_LENGTH = 500
```

`[CONFLICT: rubric-audit.json 3C.3 says 1024; all six SDKs use 500. Code wins.]`

Error messages extracted from response bodies are truncated to 500 units. If the string exceeds the limit, the last 3 units are replaced with `"..."`, so the result is at most 500 units long.

**Unit semantics:** The unit is language-defined: Go (`len()`), Ruby (`bytesize`), and Python (`len(s.encode())`) use bytes; TypeScript (`s.length`), Swift (`s.count`), and Kotlin (`s.length`) use character/code-unit length. For ASCII text (which conformance test fixtures use today), these coincide. Unicode truncation semantics are a per-language divergence documented in Appendix F. Note: byte-level truncation (Go/Ruby) can produce invalid UTF-8 mid-codepoint; this is accepted behavior. Python slices bytes too but decodes with `errors="ignore"`, so it drops the partial codepoint instead of emitting it.

### Sensitive Header Redaction `[static]`

The following headers must be redacted (replaced with `"[REDACTED]"`) before logging:

- `Authorization`
- `Cookie`
- `Set-Cookie`
- `X-CSRF-Token`

Comparison is case-insensitive.

---

## §10. Type Fidelity

### Integer Precision `[conformance]`

All integer IDs must use at least 64 bits of precision (e.g., Go `int64`, Kotlin `Long`, Swift `Int` on 64-bit platforms). Note: Kotlin `Int` is 32-bit and must not be used for IDs — use `Long`. IDs up to 2^53 + 1 (`9007199254740993`) must survive JSON round-trip without precision loss.

`[CONFLICT: JavaScript Number.MAX_SAFE_INTEGER is 2^53 - 1. On the supported Node >=22.12 floor, JSON.parse reviver source access makes lossless bigint decoding feasible, but returning bigint would break the TypeScript SDK's number-typed API surface. The spec prescribes 64-bit precision; TypeScript implementations must document the retained limitation. See waiver 1B.6 in rubric-audit.json.]`

### Nullable Numeric Dimensions (rich-text attachment width/height)

Rich-text `*_attachments` elements — the companion array the API pairs with
every rich-text attribute, spanning 18 modeled emitters (`Todo`, 14 concrete
resources, `GaugeNeedle`, and the polymorphic `SearchResult`/`Recording`
projections; three are optional — `Gauge`, `SearchResult`, `Recording` — the
rest `@required`), all sharing the `RichTextAttachment` schema — always emit
`width` and `height` keys, but the
**value is `null` for non-image blobs**, and the BC3 API may serialize a present
dimension **float-spelled** (`1024.0`). These two members are deliberately
optional/nullable in the schema (not `@required`, and marked `nullable: true` in
the canonical OpenAPI); the nine other attachment fields are `@required`. All
SDKs **decode both forms faithfully and type the nullable value statically**; the
only residual is a pre-existing encoder behavior in two SDKs, noted below:

| SDK | static type | decode `null` | decode `1024.0` |
|-----|-------------|--------------|-----------------|
| **Go** | `*types.FlexInt` → `*int32` | `nil` | `1024` (+ re-encodes as `"width": null`) |
| **Swift** | `Int32?` | `nil` | `1024` |
| **Ruby** | nilable (`parse_integer`) | `nil` | `1024` (`to_i`) |
| **TypeScript** | `width?: number \| null` | `null` | `1024` (JS number) |
| **Python** | `NotRequired[Optional[int \| float]]` | `None` | `1024.0` (float, no coercion) |
| **Kotlin** | `Int?` via `FlexibleIntSerializer` | `null` | `1024` |

- **Go** is fully faithful and round-trips: `*int32` without `omitempty`, so a
  `nil` dimension re-encodes as an explicit `"width": null`.
- **Kotlin** decodes faithfully: the generated `width`/`height` are nullable
  `Int?` decoded through `FlexibleIntSerializer` (mirroring `FlexibleLongSerializer`
  and Go's `types.FlexInt`), so a served `null` stays `null` — never the sentinel
  `0` that a bare `Int = 0` under `coerceInputValues = true` would produce — and a
  float-spelled `1024.0` decodes to `1024`. `Upload.width`/`height` share this
  representation.
- **TypeScript / Python** model the nullable value in their **static** types
  because the schema is `nullable: true`, so a present `null` is captured — not
  just an absent key. TypeScript is `width?: number | null` (JS has no int/float
  distinction, so a float-spelled `1024.0` is simply the number `1024`). Python
  is `NotRequired[Optional[int | float]]`: the raw `response.json()` performs no
  int coercion, so a float-spelled `1024.0` stays a Python `float`, and the type
  admits both `int` and `float` rather than lying with a bare `int`.
- **Ruby / Swift** decode faithfully but omit a `nil` dimension on **re-encode**
  (Ruby `to_h` `.compact`; Swift's synthesized encoder). This is a pre-existing
  SDK-wide *encoder* behavior, **out of scope** for this response field (`todos
  show --json` surfaces it via the Go SDK, which round-trips faithfully).

### Date/Time Fields `[static]`

Fields declared with `format: date-time` in the OpenAPI spec use ISO 8601 format. Implementations may use the language's native date/time type (Go `time.Time`, Ruby `Time`, Kotlin `Instant`) or keep them as ISO 8601 strings (TypeScript uses `string` from openapi-fetch schema types). The choice is a language adaptation.

### Optional Fields `[static]`

Fields not listed in the `required` array of the OpenAPI schema must be nullable or optional in the language's type system. Sentinel values (empty string, 0, etc.) are not acceptable substitutes for absence.

**Scope.** This rule constrains the **static type** of a member on a **generated type**. It says an optional member must be *representable* as absent — it does not, on its own, mandate that every SDK's runtime decode and re-encode round-trip absence. Where a language's encoder drops an explicit null (Ruby `compact`, Swift's synthesized encoder), that is a separate concern, documented per language in the Nullable Numeric Dimensions table above.

**A third wire state.** A member may be *required and nullable* (`type: [T, "null"]` on a required member — e.g. `SearchType.key`, `TimelineEventData.starts_at`/`ends_at`, `Wormhole.color`/`destination_url`). This is distinct from optional: the key must be **present**, and its value may be null. It must not be encoded as though it were optional, because that would silently accept a response that omits the key. Kotlin therefore emits `T?` **with no default** for this case, versus `T? = null` for a genuinely optional member.

Enforced for Kotlin arrays and primitive scalars by `make kt-check-optional-arrays-and-scalars` (object/`$ref`/enum-typed members are out of that checker's scope — reference types with no zero-value sentinel to guard against), and for Go by `make go-check-optional-pointers`.

**Go: absence-capability by type, no waivers.** Every optional (`omitempty`) field in the generated Go client must have a type that can represent absence:

- **Pointers** (`*T`) — the default: `go/oapi-codegen.yaml` does **not** set `prefer-skip-optional-pointer`, so optional value types (strings, booleans, numerics, `time.Time`, `types.Date`, nested structs) generate as pointers. `IsZero()` on a value-typed temporal field is a zero-value sentinel, not a representation of absence — there is no date/time carve-out.
- **Slices** (`[]T`) — optional non-nullable arrays on response-shaped schemas keep native `[]T` via a generic pass in `scripts/enhance-openapi-go-types.sh` (`x-go-type-skip-optional-pointer: true`): a nil slice already represents absence. Request-shaped arrays stay pointers so an explicit empty array is sendable (`omitempty` drops a len-0 slice — e.g. `Create*` `subscriptions`, where nil means server default and `[]` means subscribe nobody), and nullable arrays stay pointers to capture present-null.
- **Maps and interfaces** — nil-capable as-is (oapi-codegen never emits `*interface{}`; `WebhookEvent.Details` is the one such field today).

`make go-check-optional-pointers` enforces exactly this classification over `client.gen.go` with **no waiver list** — the type classifier is the policy. For a writable optional scalar this is what makes an explicit `false` / `0` / `""` sendable at all: with a value type plus `omitempty`, the zero value was unsendable and absence was unrepresentable on decode.

### 204 No Content

Responses with status 204 have no body. The SDK must handle this without attempting JSON parse (`[static]`). Return `void`/`nil`/`undefined`/`Unit` as appropriate. Conformance tests verify the 204 path completes without error (`[conformance]`).

---

## §11. Response Semantics

### Success Status Codes

Common patterns by HTTP verb:

| Method | Typical Status | Behavior | Verification |
|--------|---------------|----------|-------------|
| GET | 200 | Parse body as JSON, return typed result | `[conformance]` |
| PUT | 200 | Parse body as JSON, return typed result | `[conformance]` |
| POST (create) | 201 | Parse body as JSON, return typed result | `[conformance]` |
| POST (action) | 200 or 204 | Some POST operations (e.g., `Subscribe`, `MoveCard`, `PinMessage`) are state mutations, not creates, and may return 200 or 204 | `[static]` |
| DELETE | 204 | No body; return void | `[conformance]` |

The authoritative success status for each operation is defined in `openapi.json`. The table above covers common patterns; generated code should use the per-operation status from the OpenAPI spec.

### Error Surfacing

All 4xx and 5xx responses must produce typed `BasecampError` errors (not silently swallowed). The error must include the HTTP status code, error code, retryable flag, and request ID (`[conformance]`-verified). Message parsing from the response body is `[static]` (see §6 Error Body Parsing Algorithm).

### Non-Retryable Errors

Status codes 401, 403, 404, and 422 must NOT be retried. Conformance tests assert `requestCount == 1` for these statuses. `[conformance]`

Status code 400 must also NOT be retried. The error-mapping conformance suite asserts `retryable == false` for 400 (mapped to `validation`), so its non-retryable classification is `[conformance]`-verified; there is no dedicated `requestCount` fixture for 400. `[conformance]`

### Retry Exhaustion

When all retry attempts fail, surface the **last** error to the caller. Do not synthesize a new error — propagate the final response's error.

---

## §12. Hooks

### BasecampHooks INTERFACE

```
INTERFACE BasecampHooks
  on_operation_start(info: OperationInfo) → void
  on_operation_end(info: OperationInfo, result: OperationResult) → void   -- see OperationResult RECORD below
  on_request_start(info: RequestInfo) → void
  on_request_end(info: RequestInfo, result: RequestResult) → void
  on_retry(info: RequestInfo, attempt: Integer, error: Error, delay?: Number) → void
    -- delay is optional; Go's OnRetry omits it entirely
    -- delay unit is a language adaptation: ms in TS/Kotlin (delayMs), seconds in Ruby/Swift (delay/delaySeconds)
  on_paginate(url: String, page: Integer) → void       -- Ruby only; not in Go/TS/Kotlin/Swift
END
```

All methods are optional. A no-op default is valid. `on_paginate` is Ruby-only — new implementations may omit it.

### OperationInfo RECORD

```
RECORD OperationInfo
  service       : String     -- e.g., "Todos", "Projects"
  operation     : String     -- full operationId, e.g., "ListProjects", "GetTodo", "CreateProject"
  resource_type : String     -- e.g., "todo", "project"
  is_mutation   : Boolean    -- true for POST, PUT, DELETE
  project_id    : Integer?   -- if operation is project-scoped
  resource_id   : Integer?   -- if operation targets a specific resource
END
```

### RequestInfo RECORD

```
RECORD RequestInfo
  method  : String    -- HTTP method
  url     : String    -- full request URL
  attempt : Integer   -- 1-based attempt number
END
```

### RequestResult RECORD

```
RECORD RequestResult
  status_code : Integer?   -- HTTP status code; language adaptation: Ruby uses null for network errors, TS/Swift/Kotlin/Go use 0
  duration    : Duration   -- request duration; language adaptation: ms Integer in TS/Swift, Float seconds in Ruby, native Duration in Go/Kotlin
  from_cache  : Boolean    -- whether response was served from ETag cache
  error       : Error?     -- error if the request failed (Swift omits this field; network failures reported via status_code: 0)
  retry_after : Integer?   -- Retry-After value in seconds if present (Ruby and Go; other SDKs omit this field)
END
```

**Go-specific extension:** Go's `RequestResult` also includes a `retryable` field (Boolean) indicating whether the error was eligible for retry. This is not part of the canonical RECORD.

### OperationResult RECORD

```
RECORD OperationResult
  error       : Error?     -- error if the operation failed (after all retries exhausted)
  duration    : Duration   -- total operation duration including retries; same language adaptation as RequestResult.duration
END
```

### Hook Safety Invariant `[static]`

Hook failures must not propagate to the caller or break API operations. Implementations should log caught exceptions to stderr, but the logging mechanism is a language adaptation. Cross-SDK status: TypeScript, Ruby, and Kotlin wrap hook calls in try/catch (or equivalent). Go does not currently use `recover` for hooks (a known gap). Swift hook methods are non-throwing, so `do/catch` does not apply — however, Swift's `safeInvokeHooks` also does not guard against traps/fatalErrors from hook implementations.

### ChainHooks Combinator

```
FUNCTION chainHooks(hooks: BasecampHooks[]) → BasecampHooks
  Invokes start events (on_operation_start, on_request_start) in forward order.
  End events (on_operation_end, on_request_end): reverse order (LIFO) is
  recommended (mirrors middleware stacking), but forward order is acceptable.
  Ruby, Go, Swift, and Kotlin use LIFO; TypeScript uses forward order.
  In languages with exceptions, each invocation is wrapped in try/catch
  so a failing hook does not prevent subsequent hooks from running.
  Swift hooks are non-throwing; trap/fatalError protection is not provided.
END
```

---

## §13. HTTP Transport

### Required Headers

Every JSON API request must include all four headers below. Download requests (§14) differ: Hop 1 sends only `Authorization` + `User-Agent` (no `Accept` or `Content-Type` — it's a binary download, not a JSON API call). Hop 2 sends no SDK headers (unauthenticated signed URL fetch).

| Header | Value | Scope | Verification |
|--------|-------|-------|-------------|
| `Authorization` | `Bearer {token}` (from AuthStrategy) | All API requests + download Hop 1 | `[conformance]` |
| `User-Agent` | `basecamp-sdk-{lang}/{VERSION} (api:{API_VERSION})` | All API requests + download Hop 1 | `[conformance]` |
| `Accept` | `application/json` | JSON API requests only (not download Hop 1) | `[static]` |
| `Content-Type` | `application/json` (for requests with a body; preserve if already set, e.g., for binary uploads). TS sets if missing; Go sets unconditionally; Swift/Kotlin set only when a body is present. All approaches are acceptable. | JSON API requests only (not download Hop 1) | `[conformance]` |

Where:
- `{lang}` is the language identifier: `go`, `ts`, `ruby`, `kotlin`, `swift`
- `{VERSION}` is the SDK version (e.g., `0.6.0`)
- `{API_VERSION}` is the API version from `openapi.json` `info.version` (currently `2026-08-02`), derived from the shared date in `spec/api-provenance.json` <!-- @api-version -->

### Redirect Handling

`follow_redirects = false` for download flow (§14). Redirect responses are handled explicitly.

For cross-origin redirects, strip the `Authorization` header to prevent credential leakage.

---

## §14. Download

### Two-Hop Algorithm

Downloads use a two-hop pattern: an authenticated API request that returns a redirect to a signed storage URL.

```
FUNCTION downloadURL(raw_url: String) → DownloadResult
  1. Validate raw_url is an absolute URL with http(s) scheme.
  2. Rewrite URL: replace origin with base_url origin, preserve path+query+fragment.
  3. Hop 1 — Authenticated API GET, wrapped in the hop-1 retry loop (below):
     a. Set Authorization and User-Agent headers only (no Accept or Content-Type — this is a binary download, not a JSON API call). Every attempt is authenticated — re-run the auth strategy on retry so a rotated token is picked up.
     b. Fetch with redirect: manual (do not follow redirects automatically).
     c. If the attempt fails with a network error, or the response status is in DOWNLOAD_RETRY_ON = {429, 502, 503, 504}: retry with exponential backoff while attempts remain (honor Retry-After on 429), else surface the failure. 500 is DELIBERATELY outside the set — it is never retried.
     d. If response is redirect (301, 302, 303, 307, 308):
        - Extract Location header. ⊥ if absent.
        - Resolve Location against rewritten URL (handle relative redirects).
        - Proceed to Hop 2.
     e. If response is 2xx:
        - Direct download (no second hop needed).
        - → DownloadResult from response body.
     f. If response is any other error → ⊥ BasecampError from response, without retry.

  4. Hop 2 — Unauthenticated fetch (signed URL):
     a. Fetch Location URL with NO auth headers. Hop 2 is NEVER retried and NEVER authenticated — the signed URL is single-purpose and credentials must not leak to the storage host.
     b. If not 2xx → ⊥ BasecampError.
     c. → DownloadResult from response body.
END
```

### Hop-1 Retry `[conformance]`

The authenticated first hop retries on **network errors plus {429, 502, 503, 504}** — never 500. The set is declared here rather than inherited from anywhere else, and it matches neither of the two sets an SDK already has to hand: it is broader than the per-operation `retry_on` in `behavior-model.json` (`{429, 503}` for all 243 operations, which never governs `DownloadURL` because it has no entry there), and narrower than the error taxonomy's "all 5xx retryable" flag, which would sweep in the 500 this policy deliberately excludes. It is the gateway-error set Go's hand-written `singleRequest` already uses for GETs. Backoff is exponential from a 1-second base with jitter; `Retry-After` is honored on 429. The second hop is exempt: no retry, no auth.

"Network error" means a transport failure, with one carve-out that SDKs inherit from their main GET loop rather than restate: an attempt that exhausted the caller's entire per-attempt time budget (a request timeout) is not retried. The timeout is per attempt, so a retry spends another full budget on the same slowness rather than riding out a blip. Kotlin implements this explicitly; SDKs whose transports surface timeouts indistinguishably from other connection failures retry them.

Attempt budget per SDK — disabling retry (each SDK's spelling of `enable_retry=false` or a zero cap) yields exactly ONE hop-1 attempt:

| SDK | Budget |
|-----|--------|
| Go | `MaxRetries` as total attempts (hand-written client rejects < 1) |
| Python | `max_retries` as total attempts, floored at one (`max_retries: 0` still sends one attempt) |
| Ruby | `max_retries` as total attempts, floored at one for downloads (`max_retries: 0` still sends one attempt; the general ungoverned GET path's zero-attempt behavior is tracked separately) |
| Kotlin | `maxRetries` as total attempts, floored at one, gated on `enableRetry`; an accepted `maxRetries = 0` still sends exactly one attempt |
| TypeScript | Fixed three-attempt policy when `enableRetry` is true; one attempt when false. No public numeric knob. |
| Swift | Fixed three-attempt policy when `enableRetry` is true; one attempt when false. No public numeric knob. |

Python and Ruby carve downloads out of their ungoverned GET taxonomy (which retries 500): the download hop uses the declared `{429, 502, 503, 504}` set, in both directions — the taxonomy neither widens nor vetoes it. `DownloadURL` is deliberately absent from `behavior-model.json`; SDKs pass this policy to their retry primitive directly rather than looking it up by operation.

### DownloadResult RECORD

```
RECORD DownloadResult
  body           : Bytes          -- file content (language adaptation: TS uses ReadableStream, Swift uses Data, Go uses io.ReadCloser, Ruby uses String)
  content_type   : String         -- MIME type from Content-Type header
  content_length : Integer        -- size in bytes (-1 if unknown)
  filename       : String         -- extracted from last URL path segment
END
```

---

## §15. Webhooks

### HMAC-SHA256 Verification

```
FUNCTION verifyWebhookSignature(payload: Bytes, signature: String, secret: String) → Boolean
  1. If signature or secret is empty → return false.
  2. Compute HMAC-SHA256 of payload using secret as key.
  3. Hex-encode the digest.
  4. Compare with signature using constant-time comparison.
  5. → true if match, false otherwise.
END
```

Constant-time comparison prevents timing attacks. Never short-circuit on first mismatch.

### WebhookReceiver (optional component)

```
RECORD WebhookReceiver
  handlers : Map<GlobPattern, List<Handler>>  -- multiple handlers per pattern; on() appends
  dedup    : Set<String>            -- bounded window (~1000 entries), FIFO eviction, keyed by event ID
    -- Implementations may add a pending set for concurrent-safe dedup (e.g., Go
    -- tracks dedupSeen + dedupPending + dedupOrder). The key type is String
    -- (event IDs extracted as strings to avoid precision loss).
  secret   : String

  receive(payload, signature) →
    1. Verify signature. If invalid → reject.
    2. Extract event_id from the payload's `id` field as a string.
       -- In languages with limited integer precision (e.g., JavaScript/TypeScript),
       -- extract the ID via string matching BEFORE JSON.parse to avoid 64-bit
       -- precision loss. See typescript/src/webhooks/handler.ts extractIdString().
    3. If event_id in dedup → skip (already processed).
    4. Dispatch to matching handler(s) by event type glob.
    5. Add event_id to dedup only after successful handler execution.
       (If a handler throws, the event can be reprocessed on redelivery.)
END
```

---

## §16. OAuth Utilities

### PKCE S256

```
FUNCTION generatePKCE() → (verifier: String, challenge: String)
  1. Generate 32 random bytes.
  2. verifier = base64url_encode(random_bytes) (no padding).
  3. challenge = base64url_encode(SHA-256(verifier)) (no padding).
  4. → (verifier, challenge)
END
```

### State Generation

```
FUNCTION generateState() → String
  1. Generate 16 random bytes.
  2. → base64url_encode(random_bytes) (no padding).
END
```

### Resource-First Discovery `[conformance]`

*Rubric-critical: BC5 OAuth go-live (communique §2/§3).*

BC5's Authorization Server (AS) metadata lives **only** at the canonical issuer
(the web host — production canonical `https://app.basecamp.com`). Probing the
API host (`3.basecampapi.com/.well-known/oauth-authorization-server`) 404s permanently
because RFC 8414 §3.3 requires `issuer` to equal the URL the metadata was
derived from, and BC5's issuer is the web host. Discovery therefore starts from
the **resource** (RFC 9728) and composes with AS discovery (RFC 8414).

**Two composable operations, never merged:**

1. `discover(issuerURL)` — RFC 8414 AS metadata (unchanged public op).
2. `discoverProtectedResource(resourceOrigin)` — RFC 9728 resource metadata.

A third orchestrator, `discoverFromResource(resourceOrigin, expectedIssuer?)`,
composes them and encodes selection + the stage-sensitive fallback below.

#### Origin-root profile `[conformance]`

Both hops derive a well-known path from a caller- or metadata-supplied origin.
The origin MUST be parsed with the SDK's **own transport URL parser** (Go
`net/url`, JS `URL`, Python `urllib`, Ruby `URI`, Kotlin the extracted Ktor
`parseAbsoluteUrl`) — never a hand-rolled regex, because bracketed IPv6
(`http://[::1]:3000`) and ports break naive regexes and can disagree with the
host the client actually dials.

```
FUNCTION requireOriginRoot(raw: String) → Origin | raise usage
  1. parsed = transportParser.parse(raw)          # fail-closed on parse error
  2. REQUIRE scheme ∈ {https} OR (scheme == http AND isLocalhost(host))
  3. REQUIRE host present
  4. REQUIRE port absent OR a valid numeric port
  5. REQUIRE path is empty OR exactly "/"
  6. REQUIRE no query, no fragment, no userinfo
  7. → origin = scheme "://" host [":" port]
  # A bad caller-supplied origin is a usage error; a bad *advertised* issuer
  # origin is a discovery-classification failure (see fallback table).
END
```

Accept `http://[::1]:3000`; reject `http://[::1]:notaport`, any path beyond `/`,
and any query/fragment/userinfo.

#### Hop 1 — RFC 9728 resource metadata `[conformance]`

```
FUNCTION discoverProtectedResource(resourceOrigin: String) → ProtectedResourceMetadata
  1. origin = requireOriginRoot(resourceOrigin)
  2. doc = fetchJSON(origin + "/.well-known/oauth-protected-resource")   # SSRF-hardened
  3. REQUIRE doc.resource present and non-empty
  4. REQUIRE doc.resource IDENTICAL BY CODE-POINT to the resource identifier used
            (the requested resourceOrigin); NO normalization
  5. authorization_servers is OPTIONAL; preserve absent vs [] DISTINCTLY
  6. → ProtectedResourceMetadata{ resource, authorizationServers? }
END
```

`ProtectedResourceMetadata` models `authorization_servers` as a nullable list so
"key absent" (BC5 omits it while dark) and "present but empty `[]`" stay
distinguishable at the type level, even though both select Launchpad.

#### Hop 2 — RFC 8414 AS metadata with issuer binding `[conformance]`

```
FUNCTION discover(issuerURL: String) → OAuthConfig
  1. origin = requireOriginRoot(issuerURL)
  2. doc = fetchJSON(origin + "/.well-known/oauth-authorization-server")  # SSRF-hardened
  3. REQUIRE doc.issuer present, non-empty, and IDENTICAL BY CODE-POINT to
            issuerURL (the advertised issuer string); NO normalization  # RFC 8414 §3.3/§4
  4. Universal endpoint validation: token_endpoint present + non-empty;
     any endpoint field that IS present must be non-empty (reject "").
  5. authorization_endpoint is OPTIONAL (device-only AS omit it).
  6. → OAuthConfig{ issuer, tokenEndpoint, authorizationEndpoint?, ... }
END
```

**Per-grant endpoint validation is the consumer's, not `discover`'s.** `discover`
no longer requires `authorization_endpoint`. Each consumer asserts the endpoints
its grant needs:
- authorization-code: `authorization_endpoint` + `token_endpoint`.
- device flow: `device_authorization_endpoint` + `token_endpoint`.

#### `authorization_endpoint` is now OPTIONAL `[static]`

Previously required in every SDK model; now optional/nullable, preserving absent
vs present-empty (Go `*string`, TS `authorizationEndpoint?`, Python `str | None`,
Ruby kw default `nil`, Kotlin `String? = null`). `token_endpoint` stays required.
**Public-compatibility impact:** a previously-always-present field becomes
optional — authorization-code consumers MUST assert presence before use.

#### Selection — one name, one rule `[conformance]`

`expectedIssuer` is the single selection parameter (`preferredIssuer` is dropped).

```
FUNCTION selectIssuer(advertised: [String], expectedIssuer?: String)
  IF expectedIssuer provided:                       # explicit, authoritative
    IF ∃ m ∈ advertised with m == expectedIssuer (code-point): SELECT m
    ELSE raise expected_issuer_unavailable          # HARD
  ELSE:                                              # Basecamp-profile heuristic
    nonLaunchpad = advertised \ {Launchpad}          #   (identification by exclusion)
    IF |nonLaunchpad| == 1: SELECT that member       # documented heuristic
    IF |nonLaunchpad| ≥ 2: raise ambiguous_issuers   # HARD — never guess
    IF |nonLaunchpad| == 0: soft no_as_advertised → Launchpad
END
```

The SDK does not know BC5's canonical issuer a priori; during migration the
advertised set is {BC5 canonical, Launchpad}, so exactly-one-non-Launchpad
identifies BC5 by exclusion. Callers wanting no heuristic pass `expectedIssuer`.

#### Stage-sensitive fallback state machine `[conformance]`

Launchpad fallback is allowed **only before BC5 is committed**. Once valid
resource metadata advertises BC5 and it is selected, every later failure is
**fatal — no Launchpad request may be issued.**

| Stage / failure | Outcome |
|---|---|
| Hop-1 fetch/parse fails, or `resource` mismatch | **soft** `resource_discovery_failed` → Launchpad |
| Valid resource metadata omits BC5 (absent / `[]` / only-Launchpad) | **soft** `no_as_advertised` → select Launchpad |
| ≥2 non-Launchpad issuers advertised (no `expectedIssuer`) | **hard** `ambiguous_issuers` |
| `expectedIssuer` provided but not advertised | **hard** `expected_issuer_unavailable` |
| BC5 committed → invalid BC5 issuer origin | **hard** `invalid_issuer_origin` |
| BC5 committed → AS-metadata fetch fails (5xx / network) | **hard** `as_fetch_failed` |
| BC5 committed → issuer binding mismatch | **hard** `issuer_mismatch` |
| BC5 committed → missing per-grant endpoint/capability | **hard** `capability_unavailable` (consumer-asserted) |

`discoverFromResource` returns `Selected(config)` **or** `FallBack(reason)` where
`reason ∈ {resource_discovery_failed, no_as_advertised}` ONLY, and **raises** a
typed selection error for every hard case. **No consumer may convert a raise into
a Launchpad request.** ("BC5 committed" = valid resource metadata advertised a
BC5 issuer that was then selected.)

#### SSRF hardening — both hops, all five SDKs with OAuth discovery `[conformance]`

RFC 9728 §7.7 flags SSRF via attacker-influenced metadata; advertised AS URLs are
untrusted input. Swift is out of scope here — it ships no OAuth discovery
implementation, so it has no `fetchJSON` hop to harden. In the five that do,
every `fetchJSON` above MUST:

1. **Require HTTPS** (localhost exempt) — validated by `requireOriginRoot` before
   any socket is opened.
2. **Bound the timeout.**
3. **Suppress redirects** — fetch `redirect:"error"` (TS) / `CheckRedirect:
   ErrUseLastResponse` (Go) / `followRedirects=false` (Ktor) /
   `follow_redirects=False` (httpx) / no redirect middleware (Faraday) — or
   re-validate each target against the origin-root profile.
4. **Read the body under a genuine, bounded/streaming cap that aborts once the
   limit is exceeded** — NOT a post-hoc size check on an already-buffered body.
   Python streams via `httpx.stream()`; Ruby's default transport is the
   headers-first bounded `Fetcher.stream_http` primitive (Net::HTTP block form:
   status classified at header time, watchdog wall-clock deadline, streamed cap
   — injected Faraday connections keep a capped `on_data` read); Go/Kotlin/TS
   use bounded reads.

Non-2xx on either hop → `api_error` (not `network`).

#### Injected-client fidelity tier `[static]`

SDKs that accept a caller-supplied HTTP client (Ruby `http_client:`, and any
future equivalent) MUST hold the same security invariants on it — redirect
suppression, bounded/streaming body cap, and a wall-clock bound on the whole
request — but MAY deliver coarser timing and classification fidelity than the
default transport: status classification may occur only after the body read
completes (no headers-time seam), and deadline enforcement is wall-clock
around the call rather than woven through each read. A definitive completed
status still outranks a deadline race; a response completing past the
deadline without one is refused as a transport timeout. Callers needing exact
headers-time classification use the default transport.

### Launchpad Legacy Format

The Basecamp Launchpad OAuth endpoints use a mix of standard and legacy parameters:

- Authorization URL: standard `response_type=code`
- Token exchange: `type=web_server` (legacy) or `grant_type=authorization_code` (standard) — SDKs use one or the other based on a legacy-format flag
- Token refresh: `type=refresh` (legacy) or `grant_type=refresh_token` (standard) — same flag controls which is sent

### Authorization Code Exchange

```
FUNCTION exchangeCode(token_endpoint, code, redirect_uri, client_id, client_secret?, code_verifier?) → TokenResponse
  1. POST to token_endpoint with Content-Type: application/x-www-form-urlencoded.
  2. Body parameters:
     - type=web_server OR grant_type=authorization_code (Launchpad accepts either;
       shipped SDKs choose one based on a legacy-format flag, not both simultaneously)
     - code={code}
     - redirect_uri={redirect_uri}
     - client_id={client_id}
     - client_secret={client_secret} (if provided; confidential clients)
     - code_verifier={code_verifier} (if PKCE was used)
  3. Parse JSON response → {access_token, refresh_token, expires_in}.
END
```

### Token Response `resource` Indicator (RFC 8707) `[conformance]`

*BC5 go-live. Contract source: bc3 #9471 (`Oauth::RefreshToken#resolve_target_account!`,
`Oauth::ResourceIndicator`).*

Every token response — authorization-code exchange, device grant, AND refresh —
MAY carry a `resource` member: an RFC 8707 resource indicator naming the account
the token is bound to. BC5 emits the URN form `urn:bc:account:<queenbee_id>`
(the server also parses a URL form). Token models in all five SDKs carry it as
an **optional string, appended after all existing fields** (never repositioning
existing positional/keyword parameters):

- Omitted or JSON `null` → absent (same null-as-absent rule as
  `refresh_token`/`scope` in §16 device validation).
- Present → MUST be a non-empty string; a present-but-empty `""` or non-string
  value is a malformed response (`api_error`). No format validation beyond
  non-empty: the indicator is an opaque echo token from the client's viewpoint.

**Refresh contract.** Refresh requests accept an optional `resource` form
parameter, sent only when set and appended without repositioning existing
parameters. BC5's `trusted` clients (e.g. `basecamp-cli`) receive
**multi-account refresh tokens** (identity-wide grant, no bound account); for
these the BC5 refresh grant HARD-REQUIRES `resource` — a refresh without it is
rejected 400 `invalid_request` ("resource parameter required for multi-account
token"). The rule for every consumer:

> **Echo the stored token's `resource` when refreshing.**

**Lifecycle managers** (TS `TokenManager`, Go `AuthManager`, and any consumer
that owns the refresh loop) do this automatically: they submit the stored
`resource` on every refresh, and when a refresh response OMITS `resource` they
preserve the prior stored value on the rotated credentials (same
carry-forward rule as an omitted rotated `refresh_token`). A refresh response
that carries `resource` replaces the stored value.

### RFC 8628 Device Authorization Grant `[conformance]`

*BC5 go-live (communique §4). Public pre-registered client `basecamp-cli`:
`token_endpoint_auth_method: none`, grants `device_code`+`refresh_token`, no
redirect URIs, no secret. An omitted scope defaults to the registry's
least-privilege first entry (`read` for `basecamp-cli`) — consumers SHOULD pin
the scope explicitly rather than rely on the server default. While BC5 OAuth is
dark-launched (`issuance_enabled` off), the device authorization endpoint
answers **503** — surfaced as `api_error` with that status, meaning "not yet
enabled here", not a protocol failure.*

Three functions per SDK. All device-auth + token requests are TLS-guarded (§9).

```
FUNCTION requestDeviceAuthorization(deviceAuthEndpoint, clientId, scope?) → DeviceAuthorization
  1. requireSecureEndpoint(deviceAuthEndpoint)
  2. POST deviceAuthEndpoint (application/x-www-form-urlencoded):
       client_id={clientId}
       scope={scope}          # OMITTED entirely when unset → server default `read`
  3. Parse → { device_code, user_code, verification_uri,
               verification_uri_complete?, expires_in, interval? }
  4. Validate: device_code, user_code, verification_uri non-empty;
     expires_in and interval are positive WHOLE seconds ≤ 2147483 — an
     integer-valued float (900.0) is accepted, a fractional value (2.5) is
     malformed (api_error). interval defaults to 5 when absent; a JSON null
     duration is treated as absent (Go/Kotlin decoders cannot distinguish
     null from omission).
     The 2147483 s (~24.8 day) ceiling is the largest whole-second duration
     whose millisecond form fits a 32-bit signed timer (2147483 × 1000 ≤ 2³¹−1):
     beyond it JS setTimeout silently clamps to 1 ms (hot poll loop) and Go's
     float→int conversion is implementation-defined. An absurd value such as
     1e100 is a malformed response (api_error), not a schedulable deadline.
     The bound is shared across all five SDKs.
END
```

```
FUNCTION pollDeviceToken(tokenEndpoint, clientId, deviceCode, interval, expiresIn, clock) → Token
  1. requireSecureEndpoint(tokenEndpoint)
  2. deadline = clock.now() + expiresIn    # MONOTONIC clock, injectable
     backoff = interval                    # transient timeout backoff, SEPARATE
                                           # from the server-driven interval
     nextWaitOverride = 0                  # one-shot 429 Retry-After override
  3. LOOP (cancellation-aware):
       IF cancelled → raise DeviceFlowError(cancelled)
       IF clock.now() ≥ deadline → raise DeviceFlowError(expired)   # check BEFORE waiting,
              # so a long display hook, a stalled prior request, or a long backoff
              # cannot carry the loop past expiry undetected
       wait = max(interval, backoff, nextWaitOverride), clamped to the
              remaining lifetime (> 0 here)
       nextWaitOverride = 0   # one-shot: consumed by this wait, then gone
       SLEEP wait   # abortable; a cancel mid-wait → DeviceFlowError(cancelled)
       IF clock.now() ≥ deadline → raise DeviceFlowError(expired)
              # re-check AFTER the wait, before POSTing: the clamp above makes the
              # final sleep land exactly on expiry, so without this check the loop
              # would issue one POST for a code already known to be expired
       POST tokenEndpoint: grant_type=urn:ietf:params:oauth:grant-type:device_code,
                           device_code, client_id
            # the per-request timeout is min(request timeout, remaining
            # lifetime): near expiry a stalled POST must not hold the flow past
            # the monotonic deadline for the full request budget
       CASE response:
         # ONLY a 200 can produce a token, and OAuth protocol error codes
         # (authorization_pending / slow_down / access_denied / expired_token)
         # are recognized ONLY on a 4xx (RFC 8628 §3.5 error responses are
         # 400-class). Any other status — a nonstandard 2xx like 201/202, or a
         # 5xx — is terminal api_error (http_<status>) even if its body
         # carries a protocol error code.
         200 with a non-empty access_token → validate optional fields, return Token
         200 that is not a JSON object, or lacks a non-empty access_token
              → raise api_error (a malformed success body is NOT a usable Token,
                and is NOT a retryable transport error)
         200 whose optional token fields are malformed → raise api_error:
              expires_in, when present, MUST be a finite positive WHOLE number ≤
              2147483647 s — a non-numeric value, a non-finite one (1e400 parses
              to Infinity), a fractional one, a non-positive one, or one past the
              ceiling would make expires_at arithmetic overflow/NaN so the token
              would appear to never expire (an integer-valued float like 3600.0 is
              accepted and coerced to whole seconds, matching the device-duration
              rule; every SDK decodes the numeric value and validates it
              explicitly to reject a fractional lifetime);
              refresh_token/token_type/scope/resource, when present and non-null,
              MUST be strings — a JSON null is treated as absent (like an omitted
              field), consistent with the null-duration rule above. token_type is
              additionally non-empty when present: absent or JSON null defaults to
              Bearer, while an explicit "" is malformed (api_error). resource is
              likewise non-empty when present (RFC 8707 indicator, see the
              Token Response `resource` section above) and is captured onto the
              returned Token. A non-object
              or unparsable body, and every field/status error above, carries the
              HTTP status on the raised api_error.
              Absent expires_in is allowed (the token carries no expiry).
              The 2147483647 s (~68 year) token-lifetime ceiling is separate from
              the 2147483 s device-duration ceiling above: it bounds expires_at
              arithmetic (a Date/instant), not a schedulable timer, so it is the
              cross-runtime-safe int/Date maximum rather than the 32-bit-ms timer
              bound. Shared across all five SDKs.
         3xx (redirects are never followed)
              → raise api_error BEFORE OAuth-error body parsing (a redirect
                carrying an authorization_pending body must not keep polling)
         error authorization_pending → keep polling
         error slow_down → interval += 5   (this AND all subsequent polls)
         error access_denied → raise DeviceFlowError(access_denied)
         error expired_token → raise DeviceFlowError(expired)
         HTTP 429 AND error too_many_requests → keep polling with a ONE-SHOT
              next-wait override: nextWaitOverride = max(interval, retryAfter)
              when the response carries a positive integral Retry-After delta
              (seconds), else the current interval. The override applies to the
              NEXT wait only (still clamped to the remaining lifetime by the
              wait rule above) and then decays — it never permanently inflates
              the slow_down-driven interval. A missing, malformed, fractional,
              non-positive, or UNREPRESENTABLE (overflowing the parser's native
              integer range or digit bound) Retry-After falls back to the
              current interval. A representable delta beyond the shared device
              ceiling CLAMPS to the ceiling instead of falling back: the wait
              rule clamps to the remaining code lifetime anyway, so an
              over-ceiling throttle waits out the rest of the lifetime rather
              than resending before the server's throttle. Parsing trims ONLY
              ASCII SP and HTAB around the value (RFC 9110 optional
              whitespace): delta-seconds is 1*DIGIT, so a value wrapped in any
              other whitespace (NBSP, Unicode spaces) is malformed and falls
              back — never trimmed into validity. Cancellation stays live through the (possibly longer)
              wait. ONLY this exact combination is retryable: a 429 without
              error=too_many_requests, or too_many_requests on any other
              status, stays terminal (api_error) like any unrecognized error.
         connection timeout → backoff = min(backoff × 2, 60), keep polling
       ANY completed round-trip (2xx or OAuth error) → backoff = interval
       # The two timers never contaminate each other: slow_down inflates
       # interval permanently; timeouts inflate backoff transiently, and a
       # completed round-trip resets backoff to the current interval; a 429
       # Retry-After override outlives neither — it is consumed by the next
       # wait and gone.
END
```

```
FUNCTION performDeviceLogin(config: OAuthConfig, clientId, scope?, display, clock?) → Token
  1. Capability guard: REQUIRE config.deviceAuthorizationEndpoint present
     AND config.grantTypesSupported ∋ "urn:ietf:params:oauth:grant-type:device_code"
     ELSE raise DeviceFlowError(unavailable)      # accepts an ALREADY-SELECTED config
  2. auth = requestDeviceAuthorization(config.deviceAuthorizationEndpoint, clientId, scope)
  3. deadline = clock.now() + auth.expiresIn   # anchor at ISSUANCE, before the hook
     display(auth)         # hook: show user_code + verification_uri
  4. remaining = deadline − clock.now()        # deduct display-hook time
     IF remaining ≤ 0 → raise DeviceFlowError(expired)
     return pollDeviceToken(config.tokenEndpoint, clientId, auth.deviceCode,
                            auth.interval, remaining, clock)
     # A slow display hook consumes code lifetime; polling gets only what is
     # left of the server-issued expires_in, never a fresh full window.
END
```

**Terminal outcomes — `DeviceFlowError` carrying a `DeviceFlowReason`; the parent
error category is DERIVED from the reason:**

| reason | parent `code` |
|---|---|
| `access_denied` | `auth_required` |
| `expired` | `auth_required` |
| `transport` | `network` (retryable) |
| `unavailable` | `validation` |
| `cancelled` | native cancellation (Go `ctx.Err()`, Kotlin `CancellationException`) else `usage` |

Clocks are injectable for tests and SHOULD be monotonic: Go `time.Now`
(monotonic reading); TS `performance.now` when available, falling back to
`Date.now()` (wall-clock — a system clock adjustment can move the deadline;
environments without `performance` accept this); Python `time.monotonic`;
Ruby `CLOCK_MONOTONIC`; Kotlin a `TimeSource` (default `Monotonic`; tests
pass a `TestTimeSource` locked to virtual time). The public
`basecamp-cli` client sends no secret; Kotlin `refreshToken`'s `clientSecret`
becomes nullable.

---

## §17. ETag Caching

### Configuration

- **Default:** disabled (opt-in via `cache_enabled`; SDK-specific names: TS `enableCache`, Go `CacheEnabled`)
- **Scope:** GET requests only
- **Implementation status:** TypeScript, Go, and Swift implement ETag caching. Ruby and Kotlin do not. New implementations may omit this or defer it.

### Cache Key

The cache key must include the URL. For shared caches (caches that may serve multiple client instances or tokens), credential-scoped isolation is required to prevent one token from receiving another token's cached response. For per-client caches (each client instance has its own cache), URL-only keys are sufficient. The exact key format is a language adaptation:

- **TypeScript:** `SHA256(authorization_header)` first 8 bytes → 16 hex characters, then `+ ":" + url` (credential-scoped)
- **Go:** let `tokenHash = hex(SHA256(authorization_header))[0:16]` (first 8 bytes → 16 hex characters); cache key = `SHA256(url + ":" + accountId + ":" + tokenHash)` (credential-scoped)
- **Swift:** URL-only key (per-client isolation — each client has its own cache instance)

### Cache Algorithm

```
FUNCTION cacheMiddleware(request, cache) → Response
  ON REQUEST:
    1. If method ≠ GET → pass through.
    2. Compute cache key (see Cache Key above — format varies by SDK).
    3. If cache has entry for key → set If-None-Match: entry.etag on request.

  ON RESPONSE:
    1. If method ≠ GET → pass through.
    2. If status == 304 and cache has entry → return cached body as 200.
    3. If status is 2xx and response has ETag header:
       a. Clone response body.
       b. Store {etag, body} in cache at key.
       c. Evict oldest if cache.size ≥ MAX_CACHE_ENTRIES.
    4. → response.
END
```

### Constants

- `MAX_CACHE_ENTRIES` = 1000 (evict oldest-inserted entry when full; FIFO via insertion-order map, not true LRU)
- `MAX_TOKEN_HASH_ENTRIES` = 100 (for token hash map)

---

## §18. Code Generation

### Input Artifacts

| Artifact | Generates |
|----------|----------|
| `openapi.json` | Schema types, service methods, path mappings |
| `behavior-model.json` | Retry config per operation, idempotency flags |
| Smithy model (`spec/`) | `openapi.json` and `behavior-model.json` (upstream) |

### Generated File Marker `[static]`

Generated files should include an unambiguous generated-file marker comment. Examples: `// @generated from OpenAPI spec — do not edit directly` (TypeScript, Swift), `Code generated by oapi-codegen. DO NOT EDIT.` (Go). The specific format is a language adaptation. Not all shipping SDKs include markers today (Kotlin and Ruby generated services currently lack them); this is a recommended practice for new implementations, not a retroactive requirement.

### Service Generation Pattern `[static]`

- One class per fine-grained service (see §5 derivation rule), extending `BaseService`.
- Each method maps to one OpenAPI operation.
- Method naming algorithm:
  1. Check explicit override table (e.g., `ListEventBoosts` → `listForEvent`). If found, use it.
  2. Match a verb prefix (`Get`, `List`, `Create`, `Update`, `Delete`, `Trash`, etc.) and extract the remainder.
  3. If remainder is empty → return the bare verb (e.g., `List` → `list`).
  4. If remainder matches a "simple resource" (the service's own resource name) → return the bare verb (e.g., `GetProject` in ProjectsService → `get`).
  5. Otherwise, the remainder disambiguates: for `get` verbs, return the camelCased remainder (e.g., `GetProjectTimeline` → `projectTimeline`); for other verbs, return verb + remainder (e.g., `CreateScheduleEntry` → `createEntry`).

### Body Compaction

When serializing request bodies to JSON, strip keys with null/nil values. Do not send `{"field": null}` — omit the key entirely.

### Idempotency Wiring

The generated service method must pass its operation name to the HTTP transport layer so the retry middleware can look up the operation's idempotency flag in `behavior-model.json` for Gate 2 (§7).

### Hand-Written Composite Methods `[manual]`

All wire operations are generated (rubric 1A.6). One narrow exception is sanctioned here: a hand-written **composite convenience method** may be added on top of a generated service when the generator cannot express a safety-critical surface. (A second, non-HTTP carve-out lives in §23: the Event Feed connector's cable dial of the URL a generated `CreateStreamTicket` call returned — see §23 "Classification: Infrastructure, Not a Composite".) A composite is permitted only when all of the following hold:

1. **No hand-written wire I/O.** Every request flows through public generated wire methods (Go: through the shared generated-client transport). No manual path construction or verb selection. Bodies use the generated request types, with one Go-specific carve-out: where zero-value + `omitempty` request structs cannot express always-send-empty semantics, the composite's private transport MAY marshal an explicit body map and call the operation's generated `*WithBody` variant — the generated wrapper still owns path, verb, content type, and response decoding, and the operation identity still reaches hooks and retry. This is the only sanctioned use of hand-marshaled bodies; sparse public methods keep using the generated request types.
2. **Composition, not substitution.** It composes existing generated operations (e.g. GET → overlay → full PUT); it never introduces a wire operation the spec lacks — fix the spec and regenerate instead.
3. **Native hook identities.** Hooks observe the constituent wire operations under their normal per-language identities; composites never mint synthetic operation names.
4. **Conformance-covered.** The composite's behavior is encoded in `conformance/tests/` fixtures run by every runner. All six SDKs now have one, so a native test mirror is no longer a substitute for fixture coverage.
5. **Declared placement.** The composite lives in the language's designated hand-written extension point (Kotlin generator `EXTENSIBLE_SERVICES`/`HAND_WRITTEN_SERVICES`, TS `src/services/*-extensions.ts` wired in `client.ts`, Ruby zeitwerk `prepend` module, Python service subclass re-exported by the client, Swift same-module extension) so regeneration can never silently drop or fork it.
6. **The raw operation stays reachable.** When a composite takes over the plain method name, the generated single-request method is renamed (via `METHOD_NAME_OVERRIDES`) rather than hidden, and gets its own conformance case asserting it makes exactly one request with no read-before-write. Without that second case, later generator drift could silently turn both public methods into composite behavior and nothing would notice.

### Replace-Semantic Operation Naming `[static]`

A wire operation is named for what the server does with the body, not for what the caller usually intends:

- **`Replace*`** when the endpoint takes a complete representation and clears what the body omits — `ReplaceTodo`, `ReplaceDocument`. This holds even where the replacement carries *declared carve-outs*: `ReplaceDocument` does not touch a drafted document's subscribers, and that does not make the operation a merge. A carve-out is one named field the server excludes from the swap; a merge is the server preserving anything the body omits. The rule keys on the default, and the carve-out is documented on the operation.
- **`Update*`** when the endpoint merges — the server preserves fields the body omits (`Recordable#changing` and friends), as Messages does — or when it is genuinely hybrid, as Cards is: merge for `title`/`content`, key-guarded for `assignee_ids`, forced-replace for `due_on` (#467).

Two shipped operations are replace-semantic but still named `Update*`. `UpdateTodolistOrGroup` reached the honest *method* name through `METHOD_NAME_OVERRIDES` (rule 6) rather than a wire rename, so its SDK surface reads `replace` while the operationId does not. `UpdateScheduleEntry` has neither yet — its method is still `updateEntry` and its composite is unbuilt (#546/#547). Both are naming debt, not a second sanctioned pattern; the wave that closes them is #374. New replace-semantic operations take the wire rename.

A rename is breaking and ships **without a deprecated alias** (`ReplaceTodo`, #375; `ReplaceDocument`, #543). An alias would keep the destructive method reachable under the name that misdescribes it, which is the defect the rename exists to remove.

Current composites:
- **Todos** `update` (merge-safe) and `edit` (read-modify-write) — see §5 "Merge-Safe Write Surface (Todos)".
- **Todolists** `update` (merge-safe) and `edit` (read-modify-write) — see §5 "Merge-Safe Write Surface (Todolists)". The raw path is `replace`, renamed from `update` via `METHOD_NAME_OVERRIDES` (rule 6) rather than by renaming the wire operation.
- **Documents** `update` (merge-safe) and `edit` (read-modify-write) — see §5 "Merge-Safe Write Surface (Documents)". The raw path is `replace`, and it needs no override: the wire operation is `ReplaceDocument`, so the ordinary naming algorithm produces it.
- **Cards** `update` (merge-safe) — see §5 "Merge-Safe Write Surface (Cards)". The raw path is `updateVerbatim`.
- **Uploads** `download` — composes the generated `get` (GetUpload) with the client-level `downloadURL` primitive (§14), erroring when the upload carries no `download_url`; the result's filename prefers the upload metadata's `filename`.

**Body compaction is not relaxed for composites.** A composite never sends `{"field": null}` to express "clear" (§18 rule). Where the server treats an omitted key as a clear — as BC3 does for `due_on` — omission *is* the clear encoding, and it is the only one all six SDKs can express identically: five strip nulls structurally before the wire (Python `_compact`, Ruby `compact_params`, Kotlin `?.let`, TypeScript's `JSON.stringify` dropping `undefined`, Swift `encodeIfPresent`).

---

## §19. Conformance Testing

### Test Schema

Test cases conform to `conformance/schema.json`. Each test specifies:
- `operation` — OpenAPI operation ID
- `method` — HTTP method
- `path` — URL path pattern
- `mockResponses` — sequence of mock responses the test server returns
- `assertions` — behavioral assertions to verify

### Assertion Types

Enumerated from `conformance/schema.json` — the table below is gated against
that enum by `make doc-constants-check`, so a new assertion type cannot ship
undocumented:

<!-- @assertion-types:begin -->

| Type | Description |
|------|-------------|
| `requestCount` | Number of HTTP requests made (verifies retry behavior) |
| `delayBetweenRequests` | Minimum delay between requests in ms (verifies backoff). `index` names one inter-request gap; omitted, every gap must clear the minimum. Never passes vacuously: a named gap the run did not produce fails, an omitted index with no gaps fails, and a negative index is rejected. |
| `statusCode` | HTTP status code of the response |
| `responseStatus` | Response status category |
| `responseBody` | Specific value in response body (by path) |
| `headerPresent` | Named header exists on request |
| `headerAbsent` | Named header does **not** exist on the request. Absence is decided on the header's value list, not a `get` that returns the empty string for both "missing" and "present but empty" — a present-but-empty header fails this assertion. |
| `headerValue` | Named header has specific value |
| `errorType` | Error type classification |
| `noError` | Operation completed without error |
| `errorRaised` | Operation failed, without naming how — the code-agnostic inverse of `noError`. For cases the SDKs refuse by *different* mechanisms (a hand-written guard raises `api_error`; a model decoder raises its own language's decode failure), so no single `errorType` is assertable and what all six agree on is that the call fails at all. Declaring it also tells the decoder-backed runners that a decode failure is the behaviour under test rather than a stale fixture body, which switches the stop-on-mismatch policy off for that case — so every fixture declaring it must have a non-`errorRaised` control sibling whose decoded body differs in exactly one field, enforced by `conformance/check_kill_case_controls.py`. |
| `requestPath` | URL path of the outgoing request |
| `requestMethod` | HTTP method of the outgoing request |
| `requestBody` | Value at `path` (dot-notation key) inside the captured JSON request body. A request that sent no JSON body fails, as does a body that omits the key. |
| `requestBodyAbsent` | The `path` key is **not** present in the captured JSON request body — the assertion that pins omission-as-clear and body compaction (§18). A request with no JSON body at all satisfies it. |
| `errorCode` | Error code in structured error |
| `errorMessage` | Error message text |
| `errorField` | Specific field value on the error object |
| `headerInjected` | Header was injected with specific value |
| `requestScheme` | URL scheme (http/https) of request |
| `urlOrigin` | Origin validation result (accepted/rejected) |
| `responseMeta` | Metadata on paginated response (totalCount, truncated) |

<!-- @assertion-types:end -->

The per-request assertions — `headerPresent`, `headerAbsent`, `headerInjected`,
`requestPath`, `requestMethod`, `requestBody`, `requestBodyAbsent` — take an
optional `index` naming which recorded request to inspect (0-based; negative
counts from the end, so `-1` is the last request). It defaults to `0`, and an
index past the number of recorded requests fails rather than passing vacuously.

### Test Categories and Owning Sections

| Category | Files | Owning Spec Section(s) |
|----------|-------|----------------------|
| auth | `auth.json` | §4 Authentication, §13 HTTP Transport |
| cards-write | `cards_write.json` | §5 Merge-Safe Write Surface (Cards), §18 Hand-Written Composite Methods |
| downloads | `downloads.json` | §14 Download |
| error-mapping | `error-mapping.json` | §6 Error Taxonomy |
| idempotency | `idempotency.json` | §7 Retry (Gate 2) |
| integer-precision | `integer-precision.json` | §10 Type Fidelity |
| live-my-surface | `live-my-surface.json` | External governance (CONTRIBUTING.md, live canary — opt-in via `BASECAMP_LIVE`) |
| network-retry | `network-retry.json` | §7 Retry (network errors, Gate 2) |
| pagination | `pagination.json` | §8 Pagination |
| paths | `paths.json` | §3 Client Architecture (account path construction) |
| retry | `retry.json` | §7 Retry |
| schedule-entries-write | `schedule_entries_write.json` | §10 Type Fidelity (explicit-empty vs. omitted wire semantics) |
| security | `security.json` | §9 Security |
| status-codes | `status-codes.json` | §11 Response Semantics |
| todolists-write | `todolists_write.json` | §5 Merge-Safe Write Surface (Todolists), §18 Hand-Written Composite Methods |
| todos-write | `todos_write.json` | §5 Merge-Safe Write Surface (Todos), §18 Hand-Written Composite Methods |
| uploads-download | `uploads_download.json` | §14 Download, §18 Hand-Written Composite Methods |

### Runner Pattern

```
1. Start mock HTTP server.
2. Configure SDK client with mock server URL (localhost — bypasses HTTPS enforcement).
3. For each test case:
   a. Register mockResponses on the mock server.
   b. Execute the operation via SDK.
   c. Evaluate each assertion against the observed behavior.
4. Report pass/fail per test, per category.
```

### Zero-Skip Target `[manual]`

All conformance tests should pass. The roster below enumerates every skip a
default (mock-mode) conformance run reports, one line per runner × test,
verbatim from the runners' skip mechanisms. A skip is
either **waiver-backed** (a `rubric-audit.json` waiver ID), **architectural**
(runner mechanics, compensated by native tests), or **unwaivered** (a known gap
with no rubric record — tracked work, not an accepted divergence). A PR that
closes a gap deletes exactly its own lines.

One fixture, "List operation returns first page with Link header"
(`conformance/tests/pagination.json`, tagged `link-header`), is handled by a
tag branch rather than a named-skip entry, because the exclusion is
architectural: every SDK auto-paginates by design, so its first-page-only
`requestCount` assertion is inapplicable. What that branch excludes differs by
runner, and the difference is deliberate:

- **Go, Python, Ruby, TypeScript** suppress the `requestCount` ASSERTION only.
  The case still runs, and its `statusCode: 200` and `noError` assertions still
  fire. This is what lets `requestCount` be asserted as an exact count
  everywhere else (#573) without shedding the rest of the case.

- **Kotlin and Swift** skip the whole CASE, as they always have. Both derive a
  response's status from the last mock response the SDK consumed, and an
  auto-paginating SDK walks past the end of a one-response queue, so `statusCode`
  reports "no response" and the case cannot pass on those two runners. Narrowing
  them to the assertion was tried and reverted: `make conformance-kotlin` and
  `make conformance-swift` each then report
  `FAIL: List operation returns first page with Link header` /
  `Expected status code 200, but got no response` and exit 2. Widening their
  status model is separate work, not a skip to delete here.

Note the shape this avoids: #573 first narrowed nothing and instead added the
whole-case skip to all four remaining runners, which left the fixture skipped by
all six — present in `pagination.json`, passing `conformance-fixtures-check` and
`check-fixture-coverage`, and executed by nothing. That is #572's defect one
layer down. Nothing in the build detects a fixture no runner runs; that gap is
tracked as #602.

**Go** (`conformance/runner/go/main.go` `goSDKSkips`) — architectural; same-origin
logic is covered by `TestIsSameOrigin` unit tests:
- "Mixed-case host and explicit default port stay on the mocked origin" — Go runner dials `configOverrides.baseUrl` directly; its `httptest` mock owns its origin, so origin-interception normalization does not apply.
- "Bracketed IPv6 loopback origin stays on the mocked origin" — same as above.

**Python** (`conformance/runner/python/runner.py` `SKIPS`) — none. The
`link-header` fixture above runs; only its `requestCount` assertion is
suppressed.

**Ruby** (`conformance/runner/ruby/runner.rb` `RUBY_SKIPS`):
- "PUT operation is naturally idempotent" — GET-only retry (waiver 2B.3).
- "DELETE operation is naturally idempotent" — GET-only retry (waiver 2B.3).
- "POST operation retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "Subscribe POST retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "CreateBookmark POST retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "DeleteBookmark DELETE retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "UpdateMyNote PUT retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "UpdateCalendar PUT retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "PrioritizeAssignment POST retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "DeprioritizeAssignment DELETE retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "Network error on an idempotent POST is retried then succeeds" — GET-only network retry (waiver 2B.3).

**TypeScript** (`conformance/runner/typescript/runner.test.ts` `TS_SDK_SKIPS`):
- "Large integer IDs preserved without precision loss" — `Number` is 53-bit (waiver 1B.6).

**Kotlin** (`kotlin/conformance/.../Main.kt` — `KOTLIN_SKIPS` is empty) — none
beyond the whole-case `link-header` tag branch described above.

**Swift** (`conformance/runner/swift/.../Runner.swift` — `temporarySkips` is
empty) — none beyond the whole-case `link-header` tag branch described above.

Swift carries no capability skips. It is three-gate on retry (status, network,
idempotent POST) and, since #563, retries the authenticated download hop, so
`SWIFT_CONFORMANCE_NO_SKIPS=1` is a no-op today — the mechanism is kept live so
a future temporary skip must be proven genuine before it is added.

The TypeScript live canary additionally reports one placeholder skip when
`BASECAMP_LIVE` is unset (`live-runner.test.ts`) — that is the opt-in gate for
`live-my-surface.json` documented in the category table above, not a
conformance gap, and it is deliberately not rostered.

---

## §20. Critical Requirements

The following are must-pass criteria from the rubric. Each maps to a spec section and verification method.

| # | Rubric ID | Requirement | Spec Section | Verification |
|---|-----------|------------|--------------|-------------|
| 1 | 1A.1 | Smithy model validates | §18 | `[static]` |
| 2 | 1A.2 | OpenAPI derived from Smithy | §18 | `[static]` |
| 3 | 2A.1 | Structured error type with code, message, hint, http_status, retryable | §6 | `[static]` |
| 4 | 2A.3 | HTTP status → error code mapping | §6 | `[conformance]` |
| 5 | 2B.4 | POST not retried unless idempotent | §7 | `[conformance]` |
| 6 | 2C.5 | Cross-origin pagination Link header rejected | §8 | `[conformance]` |
| 7 | 3C.1 | HTTPS enforcement for non-localhost | §9 | `[conformance]` |
| 8 | 1C.3 | No manual path construction | §3, §18 | `[manual]` |
| 9 | 1A.6 | No hand-written wire methods (multi-language only; Go uses hand-written service wrappers around generated client — see Appendix F). Conformance-tested composites over generated operations are permitted per §18 "Hand-Written Composite Methods", and the §23 Event Feed connector's cable dial (verbatim, of a generated mint's URL) is the one sanctioned non-HTTP wire act | §18, §23 | `[manual]` |
| 10 | 4A.1 | Smithy → OpenAPI freshness check | §21 | `[static]` |

---

## §21. Verification Gates

### Enforced by `make check`

| Target | What it verifies |
|--------|-----------------|
| `smithy-check` | `openapi.json` matches Smithy rebuild |
| `behavior-model-check` | `behavior-model.json` matches regeneration |
| `provenance-check` | Embedded provenance matches `spec/api-provenance.json` |
| `sync-spec-version-check` | Smithy service version matches the shared date in `spec/api-provenance.json` |
| `sync-api-version-check` | `API_VERSION` constants match `openapi.json` `info.version` across all SDKs |
| `doc-constants-check` | Constants restated in prose match their sources: `@api-version`-marked spans vs. `openapi.json` `info.version`, `@bc3-pin`-marked spans vs. `spec/api-provenance.json`, and §19's `@assertion-types` table vs. `conformance/schema.json`. Only marked spans are checked — `spec/api-gaps/` cites historical bc3 SHAs on purpose — and `spec/doc-constants.json` commits the exact per-file marker count, so neither deleting a marker nor adding an unrecorded one can silence the gate. It also bounds, by count and with a reason, the files allowed to name today's pin unmarked. `make sync-api-version` rewrites the two scalar constants, except in files listed `.writerExcludes` |
| `url-routes-check` | `go/pkg/basecamp/url-routes.json` (embedded via `//go:embed`) matches regeneration from `openapi.json` |
| `go-check-drift` | Go generated services match current OpenAPI spec |
| `kt-check-drift` | Kotlin generated services match current OpenAPI spec (operation-level coverage) |
| `go-check` | Go: lint + test |
| `ts-check` | TypeScript: typecheck + test |
| `rb-check` | Ruby: test + rubocop |
| `kt-check` | Kotlin: build + test |
| `swift-check` | Swift: build + test |
| `conformance` | All conformance test categories pass with documented waivers (go, kotlin, python, ruby, swift, typescript runners) |

Representative dependency chain (see the Makefile `check:` line for the authoritative, complete list): `check: … sync-api-version-check url-routes-check go-check-drift … kt-check-drift … go-check ts-check rb-check kt-check swift-check py-check conformance …`

Regenerate-and-diff freshness gates now exist for all six SDKs' generated output. Five run inside `make check` — `check-go-generated-drift.sh`, `check-typescript-service-drift.sh`, `check-ruby-service-drift.sh`, `check-python-service-drift.sh`, and `check-swift-service-drift.sh` (via `swift-check-drift`). The sixth, Kotlin's `kt-check-generated-drift`, is a heavier Gradle/JVM run kept out of the default `make check` and exercised as its own target plus the `test-kotlin` CI job; the fast, coverage-only `kt-check-drift` remains in `make check`.

### Advisory (not in `make check` today)

| Target | Status |
|--------|--------|
| `kt-check-generated-drift` | Kotlin regenerate-and-diff freshness gate; runs as a standalone target and in the `test-kotlin` CI job (kept out of default `make check` to avoid JVM/Gradle startup latency) |
| `audit-check` | Defined in the Makefile convention (external governance reference in `basecamp/sdk` `MAKEFILE-CONVENTION.md`) but no target exists in this repo's Makefile |

---

## §22. Out of Scope

The following are explicitly NOT part of this specification:

- GraphQL or SSE transport. WebSocket as a general API transport remains out of scope; the §23 Event Feed connector's Action Cable lane is in scope
- CLI UI or interactive prompts
- Circuit breaker, bulkhead, or client-side rate limiter (rubric T2D criteria exist but are optional extras, not core contracts)
- Prometheus or OpenTelemetry hook implementations (the hook protocol is in scope; specific integrations are not)
- Package publishing or release automation
- Language-specific async/concurrency model (spec is synchronous-first; async is a language adaptation). The §23 Event Feed connector is the carve-out: its surface is inherently asynchronous — a long-lived, serially back-pressured event stream — and each language realizes it with its native streaming idiom (§23 "Consumer Surface"); the concurrency primitive is the adaptation, the behavioral contract is not
- Smithy model authoring
- File upload multipart encoding details
- Webhook receiver HTTP server implementation (the verification algorithm is in scope; how to run an HTTP server is not)

---

## §23. Event Feed Connector

### What This Is

BC3 exposes an account-wide **event feed** — a resumable notification feed, not an audit
log — over two lanes. The **push lane** is a native Action Cable WebSocket (`EventsChannel`),
authenticated by short-lived stream tickets minted at `POST /events/stream_ticket.json`;
its payloads are wake-up signals that may be delayed, duplicated, or dropped. The **poll
lane** is `GET /events.json`: stateless, client-held cursor, strict event-id order, behind a
best-effort ~30-second safety horizon. The poll lane repairs what push missed **for ids above
the held position whose transactions commit within the horizon** — the bound is
position-relative, never wall-clock, and it is best-effort rather than guaranteed delivery.
Completeness-critical workflows corroborate against canonical resource APIs.

The SDK ships **one blessed connector** in every language so that six integrations never
become six divergent reinventions of a subtle stateful protocol: mint → dial the mint's URL
verbatim → subscribe → await confirmation → catch-up poll → drain the live buffer → stream,
with the reconnect, backoff, staleness, dedupe, and checkpoint discipline specified below.
Connecting the live lane **before** entering the poll lane is load-bearing, not an
optimization: entry at the present places every already-visible event — and any in-flight
event that drew a lower id and commits after entry — permanently behind the entry position,
so the live buffer is the only carrier of an in-flight-at-entry straggler (see Entry
Boundary below).

The wire operations beneath the connector — `PollEvents` and `CreateStreamTicket` — are
ordinary generated operations, tracked in `spec/api-gaps/event-feed.md` until the BC3
contract merges. The connector performs **no wire I/O of its own** except dialing the mint's
URL verbatim through the transport seam; every HTTP exchange reaches the wire through a seam
backed by a generated operation. That is what lets this section fix the connector contract
ahead of the generated layer landing.

### Provenance `[manual]`

Everything bc3-derived in this section is verified at bc3 `8be5c67de5` (pre-merge; lineage
`ee19670c02`); re-verified at bc3's merge-time gate. Until that gate clears, the
bc3-derived material is **PROVISIONAL** — normative for SDK drafting, frozen only at the
gate — and it comes in two classes with different re-verification mechanics:

**Class 1 — wire literals**, frozen when bc3 regenerates transcripts against the rebased
head:

- disconnect reason strings `unauthorized`, `remote`, `invalid_event_stream_command`;
- the poll body envelope keys `events` / `position` / `next`;
- the 409 body's digest keys `position_digest` / `filters_digest`;
- the 410 body's keys `epoch_after_id` / `resume`;
- the 400-position and 400-filter error-body shapes (the discriminating matrix, as
  captured in the transcripts);
- the published srv1 digest vectors (Checkpoint Identity below);
- the mint response body `{ticket, expires_in, url}`;
- the subscribe identifier literals — channel `EventsChannel`, filter param spellings
  `types` / `buckets` / `creators`, comma-joined values.

One literal in that list carries a different verification basis: `remote` has **no
transcript capture** (none exists in the provisional delivery, and the connection-outcome
scenarios don't produce it) — it is source-verified against the pinned Rails and the
branch. Its freeze at the merge-time gate rides bc3's re-verification of the disconnect
matrix (plus the single requested capture frame), not transcript regeneration.

**Class 2 — semantic behavior**, equally provisional; transcript diffs cannot prove these,
so each is re-verified at the gate as its own row (against the rebased source and docs):

- present-entry semantics — `since=now` and the bare entry are equivalent, mint the cursor
  at the newest visible event id, and permanently exclude an in-flight lower id that
  commits after entry (the Entry Boundary section's premise);
- the best-effort, position-relative ~30-second safety-horizon bound;
- the frozen-head `next` predicate (when a walk continues vs. terminates);
- 409/410 re-entry semantics, including that a 410 `resume` URL re-enters at `since=now`
  with the canonical filter set preserved;
- the srv1 canonicalization algorithm itself (not just its vectors);
- ticket statelessness and replayability, and the ~120-second TTL;
- the 3-second server heartbeat cadence (the input to the SDK's staleness policy);
- the 400 split's recovery semantics — a malformed-position 400 is recoverable by `since=`
  re-entry, a malformed-filter 400 is not;
- the subscribe retransmit contract — identical resubscribes silently absorbed, different
  ones rejected;
- the push payload's shape, including the `visible_to_clients` presence asymmetry (push
  rows carry it, poll rows omit it);
- the disconnect matrix's completeness at the verified head, including that `unauthorized`
  arrives only pre-welcome.

**SDK-owned contract** — normative as written, not gated on bc3: the state inventory and
transitions, the ownership cut and the conjunctive save-ordering invariant (they *respond
to* class-2 semantics — the premises re-verify at the gate; the discipline stands), the
semantic-handler contract, the dedupe rule, timer kinds and the virtual-advance algorithm,
the checkpoint-identity structure with origin canonicalization and the `srv1-` namespace
prefix, continuation/resume-URL validation, the staleness detection policy (7500ms), the
options surface, and the security invariants.

### Classification: Infrastructure, Not a Composite `[manual]`

The connector is hand-written, spec-normative infrastructure in the same class as §14
(Download), §15 (Webhooks), and §16 (OAuth Utilities). It is **not** a §18 composite:
composites are stateless multi-call orchestrations over generated operations; this is a
long-lived stateful transport with its own liveness contract. §18's composite rules
(read-overlay-write, `METHOD_NAME_OVERRIDES`, raw-path reachability) do not apply here;
what does carry over is the spirit of §18 rule 1 — no hand-written wire I/O. Every mint and
poll flows through a generated operation behind a seam; the one non-HTTP wire act the
connector owns is `CableTransport.dial(mint.url)`, verbatim, as sanctioned. The connector
never assembles cable topology (scheme, host, path, account prefix) client-side — the
mint's `url` is connected to exactly as returned.

### Consumer Surface

The connector is consumed as a serial stream of deduplicated events plus lifecycle
observability. The delivery idiom is language-native (Go `iter.Seq2[Event, error]`
range-over-func is the reference; TypeScript/Python async iterators, Kotlin `Flow`, Swift
`AsyncSequence`, Ruby a blocking enumerator are the expected adaptations), but the contract
is shared:

- **Serial back-pressure is structural.** The consumer's processing *is* the loop body; the
  connector does not advance, monitor, or reconnect while the consumer is processing. Timers
  that fire mid-delivery are observed when control returns; a stale socket is then detected
  before any further delivery **from the live socket**. (Draining's replay of
  already-admitted buffered events is a bounded in-memory completion, not socket delivery —
  it finishes first; see the deferred-consumption note under the state machine.)
- **Events only.** The iteration element is the event. Gap, caught-up, and position-rejected
  notifications are Observer callbacks; semantic dispositions live exclusively in the signal
  handler (below). Neither is ever an in-band union element.
- **Typed terminal errors end iteration** as exactly one final error element carrying a
  `TerminalReason`. Cancellation, `close()`, and a consumer break end iteration with **no**
  error element — a clean stop; the feed is resumable by design.
- **Single-shot.** A second consumption of the same connector is the `usage` terminal —
  the one `usage` condition that surfaces as an iteration error element. Construction does
  no I/O (validation only); construction-time validation failures (a filter violation, a
  configured store with an empty consumer namespace) surface as **language-native
  construction errors** carrying the `usage` code, before any iteration exists — never as
  iteration elements, and with zero wire attempts. First I/O happens on first iteration,
  which keeps hosts and tests deterministic.
- **`close()`** is idempotent and callable from any context: it abandons, never drains.
  Undelivered buffered events are abandoned — the next run re-serves from the last **usable**
  checkpoint (see the exclusion under Entry Boundary).
- A consumer break takes the identical teardown path; the in-flight page's checkpoint is
  **not** saved.
- All Observer callbacks fire on the consumer's execution context, never concurrently with a
  delivery.

Feed rows are wake-up signals — enough to route, not enough to act. Consumers refetch the
referenced recording through canonical resource APIs before acting; feed payloads are never
current resource state.

```
RECORD Event
  id                 : Integer
  kind               : String
  event_type         : String
  action             : String
  created_at         : String       -- ISO 8601
  bucket_id          : Integer
  creator_id         : Integer
  recording_id       : Integer
  visible_to_clients : Boolean?     -- presence-bearing: push payloads carry it, poll rows
                                    -- omit it; absent ≠ false. Never a defaulted boolean.
END

RECORD Filters
  types    : List<String>?      -- cataloged event types only
  buckets  : List<Integer>?     -- ≤ 100 ids
  creators : List<Integer>?     -- ≤ 100 ids
END
```

All id fields carry §10's 64-bit integer contract; no new type spelling is introduced for
it.

Filter validation is client-side and fail-closed `[conformance]`: ids must be positive; type
strings non-empty with no commas, whitespace, or quotes; each id list capped at 100. A
violation is a `usage`-coded construction error (Consumer Surface above) — zero wire
attempts. Positions are filter-bound; changing filters starts a new checkpoint lineage (the
server enforces this with 409).

### State Machine `[conformance]`

The connector is **11 states and 26 labeled transitions**, plus one universal edge to
`Closed` from each of the 9 non-absorbing states. This inventory is published as the
cross-language contract; each SDK's state code encodes this table.

States: `Idle`, `Backoff`, `Minting`, `Connecting`, `AwaitingWelcome`,
`AwaitingConfirmation`, `CatchingUp`, `Draining`, `Streaming`, `Terminal` (absorbing; the
typed error element is emitted), `Closed` (absorbing; no error element).

| # | From | To | Trigger / action |
|---|------|----|------------------|
| 1 | Idle | Minting | first iteration (initial connect is immediate; no backoff) |
| 2 | Backoff | Minting | `backoff` timer fired (a fresh ticket is ALWAYS minted next) |
| 3 | Minting | Connecting | ticket minted; dial the mint's `url` verbatim |
| 4 | Minting | Backoff | mint transient/throttled, or an unauthorized mint (401/403) below the shared-counter threshold (Retry-After honored as the floor of the next delay) |
| 5 | Minting | Terminal(`authorization_failed`) | 3rd consecutive connection-level authorization failure (shared counter across unauthorized mints, `unauthorized` disconnects, and unauthorized polls; resets only on a successful poll page) |
| 6 | Connecting | AwaitingWelcome | dial ok; frame pump started (`handshake-deadline` was armed on entry to Connecting, before `dial` — a stalled dial expires it) |
| 7 | Connecting | Backoff | dial failed, or `handshake-deadline` expired mid-dial (the pending dial is cancelled). A dial refused by cable-URL policy is NOT this edge — it is Terminal(`invalid_cable_url`), below the table |
| 8 | AwaitingWelcome | AwaitingConfirmation | `welcome` → send subscribe; re-arm as `confirmation-deadline` (10s) |
| 9 | AwaitingWelcome | Backoff | socket error/close, staleness expiry, or handshake deadline lapsed (full teardown first) |
| 10 | AwaitingWelcome | Terminal(`authorization_failed`) | 3rd consecutive connection-level authorization failure (reason-string dispatch on the literal `unauthorized`; shared counter with unauthorized mints) |
| 11 | AwaitingConfirmation | CatchingUp | `confirm_subscription` → cancel deadline; reset the attempt counter (the authorization counter resets only on a successful poll page); select entry cursor |
| 12 | AwaitingConfirmation | Terminal(`subscription_rejected`) | `reject_subscription` → cancel deadline, **close the socket**, ZERO reconnects |
| 13 | AwaitingConfirmation | Terminal(`protocol_fatal`) | raw disconnect frame `reason=invalid_event_stream_command, reconnect=false` |
| 14 | AwaitingConfirmation | Backoff | confirmation deadline lapsed → dispose conn + pump + ALL of the attempt's timers, then jittered fresh-ticket retry |
| 15 | AwaitingConfirmation | Backoff | socket dropped, staleness expiry, or disconnect with `reconnect ≠ false` (includes `remote`) |
| 16 | CatchingUp | CatchingUp | page fetched → deliver events → save + announce checkpoint → follow `next` |
| 17 | CatchingUp | CatchingUp **or** Terminal(`feed_gap`) | 410 → dispatch `FeedGap` signal (Semantic Signals below): a registered handler returning Accept re-enters via the provided resume URL; Terminate — or no handler — is Terminal(`feed_gap`). `Observer.gap` fires as observability either way. A 410 never silently auto-continues. |
| 18 | CatchingUp | CatchingUp | 400-position → re-enter `since=<last poll-served id>` (or, with none, `since="now"` — a present-class entry, Entry Boundary below); `Observer.position_rejected(position_invalid)` |
| 19 | CatchingUp | CatchingUp | 409 → discard the held position → re-enter `since=<last poll-served id>` (or, with none, `since="now"` — a present-class entry); `Observer.position_rejected(filter_changed)` |
| 20 | CatchingUp | Terminal(`filter_invalid`) | 400-filter: configuration error; a position reset won't help |
| 21 | CatchingUp | Backoff | socket died mid-walk, or staleness expiry (per-page checkpoints already saved are kept) |
| 22 | CatchingUp | Draining | `next` absent — the walk reached its frozen head → deliver the final page's events → save + announce its checkpoint (position-resume entries) → enter Draining. **Present-class amendment (present-class entries only — Entry Boundary below; position-resume entries unchanged):** the entry poll's returned position is HELD, not saved; the ownership cut — defined under Entry Boundary — is taken after entry into Draining. |
| 23 | Draining | Streaming | buffer replayed through dedupe → `Observer.caught_up` → arm jittered `repair-poll`. **Present-class amendment:** the held entry position is saved only once the conjunctive save-ordering invariant holds; a `BufferOverflow` disposition of Terminate — or no handler — lands in Terminal(`buffer_overflow`) instead, with no Save. (A `FeedGap` cannot arise here: 410s arise from polls, which Draining never performs — a 410 on the entry walk was already dispatched at transition 17.) |
| 24 | Streaming | CatchingUp | `repair-poll` fired → one walk from the durable position (60s ± 20%) |
| 25 | Streaming | Backoff | `staleness` fired (7.5s without a frame) / socket error / disconnect `reconnect ≠ false` and not protocol-fatal (includes `remote`) |
| 26 | Streaming | Terminal(`protocol_fatal`) | raw disconnect `invalid_event_stream_command` |
| — | any non-absorbing | Closed | `close()` / cancellation / consumer break (universal edge) |

Semantic-signal dispositions are an **overlay** on this table, not additional numbered
rows: a `FeedGap` signal arises at transition 17; a `BufferOverflow` signal arises whenever
the live buffer drops events (any buffer-holding state) and is dispatched synchronously
before the next Save; a Terminate disposition — or an absent handler — lands in the
corresponding Terminal state from wherever the signal was dispatched. Seven further edges
sit outside the 26 numbered rows for the same reason: the Terminal(`checkpoint_load`) edge
(fires on the first iteration, before the first mint — zero wire attempts); the
Terminal(`usage`) edge (a re-consumed iterator); the Terminal(`invalid_continuation`) edge
(fires between URL validation and the poll call, wherever a `next` continuation or 410
`resume` URL is about to be followed — row 16's follow and row 17's Accept re-entry both
pass through it; zero requests to the failing URL); the Terminal(`poll_failed`) edge (an
`unrecoverable`-kind poll error — Seam Contracts below — from any polling state, carrying
the generated error); the Terminal(`mint_failed`) edge (an `unrecoverable`-kind mint
error, from Minting, carrying the generated error); the Terminal(`invalid_cable_url`)
edge (a policy-refused dial, from Connecting — recurring by construction, so never the
Backoff path); and the poll transient/throttle
**self-loop** inside CatchingUp (the `poll-retry` timer — a wait, not a state change).
An `unauthorized`-kind poll error rides the reconnect cycle — full teardown → Backoff with
a shared-counter increment (the fresh mint/token cycle is its recovery path) — and the
counter's threshold-3 terminal fires from wherever the third consecutive failure lands.
The published inventory stays 11/26.

Interpretation, pinned:

- **`reject_subscription` is always terminal** — first attempt or reconnect, zero reconnect
  attempts, and the connector must explicitly close the still-open socket (Action Cable
  leaves a rejected socket open; an unhandled one stays registered server-side, receiving
  heartbeats forever while delivering nothing).
- **Connection-level authorization failures retry, then surface — on ONE shared counter.**
  Unauthorized mints (401/403), `unauthorized`-reason disconnects at connect
  (pre-welcome — rows 9/10; Disconnect Dispatch pins the arrival point), and
  `unauthorized`-kind poll errors (401/403 after the seam's own refresh/retry budget)
  increment the same consecutive-failure counter; at
  `EVENT_FEED_AUTH_FAILURE_THRESHOLD = 3` the connector surfaces `authorization_failed`
  (a mixed sequence — mint 401, `unauthorized` disconnect, poll 401 — is terminal). A
  single blip must not kill a long-lived agent: a ticket that expired in the mint→dial
  race is indistinguishable from revocation on one sample. **The counter resets only on
  a successful poll page** — never on `confirm_subscription`: a confirmed subscription
  proves the ticket worked, not the bearer, and resetting there would let alternating
  poll-401 → reconnect → confirm cycles hold the counter below threshold forever. A
  connect-level blip still clears promptly — every confirmation is followed by the
  catch-up poll, whose first successful page resets the counter. Only these three
  failure shapes ever increment it.
- **Poll transients and throttles are never terminal.** They retry inside CatchingUp on the
  `poll-retry` timer, with one algorithm: a server-directed `Retry-After` is waited
  exactly, exempt from local caps per §7's rule (implementations may still bound it
  against host limits, as §7 allows); otherwise the wait is full-jitter
  `uniform(0, min(60s, 1s × 2^(k−1)))` over the **consecutive-poll-failure index k** — a
  counter separate from the reconnect-cycle count, starting at 1 on the first consecutive
  failed poll, incremented per consecutive transient/throttled failure, and reset by any
  successful poll page and by socket teardown (after a teardown, the Backoff cycle's own
  counter governs; the poll index starts fresh in the next walk).
- **Checkpoint discipline:** only poll-page acceptance ever calls `save` (transitions 16/22–23
  per their amendments; re-entry walks save through 16 like any other pages). Streaming never
  saves — live event ids never advance the durable position. `save` failure is
  `Observer.checkpoint_save_failed` and the feed continues. `load` happens exactly once,
  on the first iteration **before the first mint**; its failure is
  Terminal(`checkpoint_load`) with zero wire attempts, because silently starting at the
  present would skip history.
- **The reset cursor is poll-lane-only.** Rows 18/19's `since=<last poll-served id>` is
  the highest event id ever **served by the poll lane** on this lineage — a value tracked
  independently of delivery, dedupe, and the live lane. A live-delivered id is NEVER a
  reset cursor: a live id above the durable position would re-enter past the un-polled
  gap behind it and permanently skip everything in between. Empty pages serve no ids and
  do not advance this value; with no poll-served id at all, the re-entry is present-class.
- **Reconnect discipline:** one path through Backoff → Minting → Connecting; at most one
  in-flight reconnect attempt at any time; the attempt counter increments per failed cycle
  and resets on confirmation. A fresh ticket is minted on **every** pass — the connector
  never stores a mint URL across attempts.

Per-state invariants (asserted by the tier-2 harness as exact sets):

| State | Socket | Allowed timers (exact set) | Delivery? | Save? |
|---|---|---|---|---|
| Idle | none | {} | no | no |
| Backoff | closed | {`backoff`} | no | no |
| Minting | none | {} | no | no |
| Connecting | opening | {`handshake-deadline`} | no | no |
| AwaitingWelcome | open | {`handshake-deadline`, `staleness`} | no | no |
| AwaitingConfirmation | open | {`confirmation-deadline`, `staleness`} | **no** | no |
| CatchingUp | open | {`staleness`} (+ `poll-retry` while a poll wait is pending) | poll pages only | after each page's delivery (held on present-class entry) |
| Draining | open | {`staleness`} | buffered live, deduped | only per the save-ordering invariant |
| Streaming | open | {`staleness`, `repair-poll`} | live (deduped) + repair pages | poll positions only — never live ids |
| Terminal / Closed | closed | **{}** | no | no |

Draining is a bounded, in-memory replay — it performs no polls and takes no wire waits, so
it has no failure edge of its own: a staleness expiry or socket failure observed while
draining is consumed at the Streaming boundary (transition 23 completes the drain, then
transition 25 handles the failure). This is the one deliberate deferred-consumption case;
everywhere else a socket-open state's failure edge (9/15/21/25) fires directly, and
staleness expiry is among each of those edges' triggers. **The protocol-fatal disconnect
is carved out of this deferral**: a raw `invalid_event_stream_command` observed during
Draining is Terminal(`protocol_fatal`) immediately (the state-generic rule under
Disconnect Dispatch) — the drain is not completed, the held entry position is NOT saved,
and no `caught_up` is announced; only recoverable failures defer.

### Disconnect Dispatch `[conformance]`

Action Cable's `disconnect` is a **text frame**, not a WebSocket close frame, and stock
Action Cable discards the reason before subscription callbacks fire. The transport seam's
verbatim raw frames exist so the connector can read it. Dispatch is on the **reason
string** — never on the `reconnect:false` flag alone, since `unauthorized` and
`invalid_event_stream_command` share `reconnect:false` and demand opposite responses. The
matrix — complete over every reason the server emits at the verified head:

| Failure | Wire signal | Connector response |
|---|---|---|
| Handshake failure (bad `Origin`, missing ticket) | HTTP upgrade failure — **no frame** | Backoff → fresh mint (transitions 7/9) |
| Invalid/expired ticket at connect | `{"type":"disconnect","reason":"unauthorized","reconnect":false}`, then close | retriable **with a fresh ticket** (9 → Backoff); Terminal(`authorization_failed`) after 3 consecutive on the shared counter (10) |
| Mid-connection revocation / server-initiated disconnect | `{"type":"disconnect","reason":"remote","reconnect":true}` | maps into the existing `reconnect ≠ false` → Backoff transitions (15/25) — no new state, no new transition: re-mint and reconnect. A genuinely revoked user's next mint fails, and that mint-failure path into `authorization_failed` (5) **is** the designed revocation detection — revocation is not wire-distinguishable at disconnect time |
| Protocol violation (malformed/oversized/uncorrelatable command) | `{"type":"disconnect","reason":"invalid_event_stream_command","reconnect":false}` | Terminal(`protocol_fatal`) (13/26) — surface, never retry into it |

Reason strings are provisional wire literals (Provenance above). The reason-level dispatch
rule itself — and the threshold-3 `authorization_failed` — are SDK-owned and final.

Two dispatch clarifications, pinned:

- **`unauthorized` arrives only pre-welcome on the wire** — the server rejects the ticket
  at connect, before any `welcome` (the TTL+ε capture shows the disconnect with no
  preceding welcome), which is why the inventory routes it through transitions 9/10 and no
  other row admits it. An `unauthorized`-reason disconnect observed in any later
  socket-open state (wire-impossible at the verified head) is treated as a **socket drop**
  — the current state's "socket error/close" failure edge (15/21/25) — with **no** counter
  increment. Nothing is lost by that: the reconnect cycle re-mints, and a genuinely
  revoked user's mint then fails 401/403 and increments the shared counter — the same
  detection path the `remote` row relies on. No out-of-inventory edge exists.
- **An unrecognized reason string is treated as a socket drop** — the current state's
  "socket error/close" failure edge (9/15/21/25), whatever its `reconnect` flag says — and
  never increments the authorization counter; only the literal `unauthorized` does.
  Unknown reasons must not be guessed into either terminal class.
- **The protocol-fatal disconnect is terminal from EVERY socket-open state.** The
  inventory numbers it where the connector is actively commanding (rows 13 and 26); a
  raw `invalid_event_stream_command` frame read in AwaitingWelcome, CatchingUp, or
  Draining — the pump runs throughout — is the same reason-level dispatch applied
  state-generically: Terminal(`protocol_fatal`), never Backoff. An explicitly
  non-retryable protocol rejection must not reconnect from any state.

### Cable Protocol Details `[conformance]`

- The subscribe command is built once per connection as an exact byte string:
  `{"command":"subscribe","identifier":"<json-escaped identifier>"}`, where the identifier
  is the JSON-encoded string of an **ordered** object
  `{"channel":"EventsChannel"[,"types":"a,b"][,"buckets":"1,2"][,"creators":"3"]}` —
  comma-joined values, fixed key order, absent filters omitted, hand-built rather than
  map-marshaled so any retransmit is byte-identical. The server absorbs identical
  retransmits and rejects different ones.
- Subscribe is sent on each `welcome` received. Confirm/reject correlation is exact string
  equality against the connector's identifier; frames carrying other identifiers are
  ignored.
- Ping parsing accepts both `{"type":"ping"}` and `{"type":"ping","message":<epoch>}`.
  Unknown frame types — parseable JSON whose `type` the connector doesn't recognize —
  update liveness and are otherwise ignored.
- **An invalid inbound frame is a peer protocol violation, dispatched as a socket
  failure** `[conformance]` — three shapes, one disposition: a frame that fails to parse
  as JSON; a frame exceeding `EVENT_FEED_MAX_FRAME_BYTES`; and a correlated `message`
  frame whose payload fails to decode as an Event (a missing required key, a wrong-typed
  id). Each triggers full teardown through the current state's socket-failure edge
  (9/15/21/25 → Backoff; the reconnect cycle recovers — the fresh walk repairs anything
  the discarded stream carried), with `Observer.disconnected` carrying an invalid-frame
  indication; never an untyped decoder error escaping, never a silent skip. Never
  terminal — a garbled frame is transport-level corruption, unlike the server's own
  `invalid_event_stream_command` verdict. The size check binds inside the transport (the
  `max_frame_bytes` dial parameter — an over-limit message is rejected during the read,
  never materialized); the parse and decode checks are the connector's. **In Draining,
  this class defers like any other recoverable socket failure** (the deferred-consumption
  rule): the already-admitted retained set is intact and unimplicated, so the drain
  completes and transition 25 handles the failure — only the protocol-fatal disconnect,
  where the server itself declares the session dead, terminates mid-drain. Never terminal — a
  garbled frame is transport-level corruption, unlike the server's own
  `invalid_event_stream_command` verdict — and never silently ignored: continuing to read
  a stream that has stopped parsing invites silent divergence.
- **Every** inbound frame, of any kind, resets the `staleness` timer
  (`EVENT_FEED_STALE_AFTER = 7500ms`: two missed 3-second server heartbeats plus 25% grace —
  the server contract leaves detection policy to the SDK; the SDK pins and publishes this
  one). **The reset happens pump-side, at frame receipt** — not at state-machine dequeue —
  so frame-vs-deadline ordering is well-defined at the transport boundary regardless of
  queue depth or consumer latency: a fired staleness deadline observed on return from a
  slow delivery is authoritative, and frames still
  queued at that moment were received before the firing and already reset the timer then.
  **Staleness is suspended while the pump is blocked on a full hand-off queue** — a full
  queue proves the peer was sending faster than the connector consumed, the opposite of
  a dead peer, and a pump that isn't reading cannot observe resets, so absence of a
  reset is not evidence. Suspension is realized **at evaluation, not arming**: the timer
  stays armed throughout (the per-state exact-set invariants are unchanged — `staleness`
  remains in every socket-open state's set), and a firing whose window overlapped a
  pump-blocked interval is disregarded and re-armed rather than dispatched. A firing
  whose window the pump spent reading is authoritative.
- **The frame pump's hand-off queue is bounded and never drops.** The pump reads frames
  from the transport and hands them to the state machine over a queue of small fixed depth
  (implementation-chosen; the Go reference uses 256). At capacity the pump **blocks** —
  back-pressure propagates to the socket and TCP — rather than dropping: the
  state-machine-owned live buffer is the only place a frame can ever be dropped, and its
  overflow signal is the only drop signal. Worst-case connector memory is therefore
  bounded multiplicatively — every queued or buffered item is itself bounded by
  `EVENT_FEED_MAX_FRAME_BYTES`, so the ceiling is
  (pump depth + `EVENT_FEED_LIVE_BUFFER_CAPACITY`) × `EVENT_FEED_MAX_FRAME_BYTES`
  (≈ 10 GiB at the defaults' extreme, reached only if every slot holds a maximum-size
  frame) — even under a slow consumer. Implementations MAY additionally impose a total
  byte cap on the live buffer; if they do, eviction routes through the same overflow
  signal, never a silent drop.
- The transport negotiates subprotocol `actioncable-v1-json`, sends no `Origin` header
  (non-browser clients), and passes the mint URL through untouched, query string included.

### Entry Boundary and Save Ordering `[conformance]`

`since=now` — and the bare present entry, which the server treats identically — mints the
cursor at the newest **visible** event id. An in-flight transaction that drew a lower id N
before entry and commits after it falls permanently behind that cursor: the poll lane never
serves N, because the repair bound covers ids **above** the position, position-relative,
never wall-clock. The live buffer is therefore the only carrier of an in-flight-at-entry
straggler, which creates a client-side ordering obligation and a durability boundary this
section states honestly rather than papers over.

**Present-class entries, defined.** The amendment below applies to every entry whose
cursor resolves at the server's present head — the class, used by this name throughout
this section and the state table: the zero Cursor (bare present entry); `since="now"`; a
410 reset's resume URL (the server documents it as `since=now` with the canonical filter
set preserved); and a 400-position/409 re-entry that falls back to the present because no
poll-served id exists (the reset cursor is poll-lane-only — a live-delivered id never
positions a re-entry). Entries positioned in served history — `position=`,
`since=<id>`, `since=0` — are **position-resume class** and keep the unamended per-page
save discipline.

**Entry sequencing (present-class entries only):** hold the entry poll's returned
position → take the **ownership cut** → fix the snapshot → drain-and-accept → only then
`save`.

**The ownership cut, defined:** after accepting the entry-poll response, the state machine
performs **one bounded admission pass** — receiving from the frame pump's queue without
blocking until the queue is momentarily empty OR the pass has **dequeued**
`EVENT_FEED_LIVE_BUFFER_CAPACITY` frames of any kind, whichever comes first. The cut is
the completion of that pass. The bound counts dequeued frames, not admitted events: pings,
control frames, and unknown types are dequeued without being admitted, and an
event-counting bound would let a heartbeat-replenished queue spin the pass forever. The
pass is deliberately **not** a drain-until-empty barrier: unbounded
draining races a concurrent sender, and under sustained arrival at or above the dequeue
rate it never completes — the cut must be reachable in bounded time or the entry position
never saves. The frame bound keeps the pass finite without weakening the retained set:
admitted events never exceed dequeued frames, and any admission beyond capacity would
evict retained pre-acceptance events anyway.
"Observed" means **admitted into the state-machine-owned buffer at or before the cut**; a
frame the transport had read but the state machine had not yet admitted at the cut does
not count. **The snapshot** is the pre-cut contents of the state-machine-owned buffer,
fixed at the cut; "pre-cut events" and "post-snapshot stragglers" below are defined against
it.

**The published delivery promise is the CONJUNCTIVE save-ordering invariant, and nothing
stronger:**

> `save(P)` only after ALL retained pre-cut events have been accepted AND every pre-cut
> loss condition has been explicitly accepted. Terminate — or no handler — means no `save`.

A disjunctive form ("accepted, or its signal handled") would let an accepted overflow
bypass delivery of the other retained events; the conjunction is the invariant.

**Explicitly excluded: crash or cancellation before the first usable checkpoint.** On a
first present entry there is no older durable cursor, and on a 410 reset the old cursor is
unusable — so an event admitted pre-cut and lost to a crash before delivery and `save` is
unrecoverable, with no signal. Client-side ordering cannot manufacture durability the
server does not offer. No blanket loss-prevention or global delivery-completeness claim is
published anywhere in this section — such a claim would contradict this exclusion. What is
published is the save-ordering invariant above, scoped to the defined observation point.

**Overflow invalidates completeness before checkpoint.** A dropped lower-ID event behind
the present position is **not** poll-repairable — the repair bound covers ids above the
position only. Overflow during the entry window therefore invalidates completeness, and its
signal must be handled (or be terminal) before any `save`. This supersedes any reading of
overflow as safe-because-the-poll-lane-repairs.

**Post-snapshot lower-id stragglers** (N arrives on the live lane after the snapshot) are
the server's documented best-effort case: delivered live, deduplicated, never a position
regression. Completeness-critical work corroborates against canonical resource APIs.

### Semantic Signals `[conformance]`

Conditions that change what the feed can promise get a **separate synchronous handler with
an explicit disposition** — never a fire-and-forget callback, and never an overloaded
epoch-shaped callback:

```
Signal = BufferOverflow | FeedGap        -- two DISTINCT types, a closed union

RECORD BufferOverflow
  dropped_ids   : List<Integer>   -- exact ids dropped: "dropped" is unambiguous
  dropped_count : Integer
END

RECORD FeedGap
  epoch_after_id : Integer
  resume_url     : String
END

SignalHandler : (Signal) → Accept | Terminate
```

- **No handler registered ⇒ typed terminal:** `buffer_overflow` for an overflow, `feed_gap`
  for a 410 gap. An unhandled semantic signal cannot disappear, and **a 410 never silently
  auto-continues**.
- **Accept on `FeedGap`** resumes via the provided resume URL (it preserves the canonical
  filter set). **Accept on `BufferOverflow`** means the consumer owns the acknowledged
  incompleteness — and acceptance is not license to skip retained deliveries (the
  conjunctive invariant above still gates the `save`).
- **A registered handler is invoked exactly once per semantic signal, synchronously, on the
  consumer's execution context, before its disposition takes effect.** Skipping a
  registered handler and applying the default-terminal path is a conformance violation:
  the fixtures assert exact handler-invocation records `{kind, disposition}`, and the
  default-terminal fixtures assert **zero** invocations. (The visible outcome of
  handler-Terminate is otherwise identical to no-handler-terminal; the invocation record is
  what distinguishes them.)
- `Observer.gap` and `Observer.buffer_overflow` remain observability-only notifications.
  The semantic disposition lives exclusively in the handler.

**Only event-bearing frames are admitted to the live buffer** — pings, control frames, and
unknown frame types update liveness and are discarded, never buffered — so the buffer is
denominated in events and every dropped entry has an id. On overflow the connector drops
the oldest buffered events; the signal carries their exact ids and count. The live
buffer's capacity is its own option (`EVENT_FEED_LIVE_BUFFER_CAPACITY`, default 10,000
events), deliberately decoupled from the dedupe capacity.

### Dedupe `[conformance]`

The connector keeps a bounded LRU (default 10,000 entries) of **actually-delivered event
ids** — never position ordering. The rule is symmetric across lanes: **every delivery —
poll page, drain, or streaming — checks the LRU before delivering and records the
delivered id.** Poll-vs-push duplication is expected in both directions (pushes arrive
instantly and polls re-serve the same events once they clear the safety horizon; a repair
poll can equally re-serve what streaming already delivered), so duplicates are suppressed
by id regardless of which lane delivered first.

Two sharp edges, pinned:

- **A buffered live event with an id ≤ the current position is still delivered** — it was
  never served by poll. Discarding live ids at or below the position is the named mutant
  this rule exists to kill.
- A duplicate suppressed during poll delivery still counts toward the page's checkpoint:
  the position advances regardless — it is a poll position, not an event acknowledgment.

### Terminal and Continuable Outcomes `[conformance]`

Terminal reasons end iteration with exactly one typed error element:

| TerminalReason | Trigger |
|---|---|
| `subscription_rejected` | `reject_subscription` — always terminal, first attempt or reconnect; zero reconnect attempts |
| `protocol_fatal` | raw disconnect `reason=invalid_event_stream_command` |
| `filter_invalid` | 400-filter from a poll; the server's message naming the offending list is preserved |
| `authorization_failed` | 3rd consecutive connection-level authorization failure on the shared counter (unauthorized mint, `unauthorized` disconnect, or unauthorized poll) |
| `checkpoint_load` | `CheckpointStore.load` failed at start |
| `usage` | re-consumed iterator (the only `usage` condition that surfaces as an iteration element — construction-time validation failures raise language-native construction errors carrying the same code) |
| `buffer_overflow` | live-buffer overflow with no registered handler, or a handler returning Terminate |
| `feed_gap` | 410 with no registered handler, or a handler returning Terminate |
| `invalid_continuation` | a `next` or 410 `resume` URL failed same-origin/downgrade validation (Continuation and Resume URL Validation below); no request is issued to it |
| `poll_failed` | an `unrecoverable`-kind poll error — a generated-operation outcome outside the feed's 400/409/410 matrix and the retryable classes (e.g. 404, 405, an unexpected shape) — passed through with the generated error attached |
| `mint_failed` | an `unrecoverable`-kind mint error — a non-retryable `CreateStreamTicket` outcome other than 401/403 (e.g. 404, 422, a malformed success) — passed through with the generated error attached |
| `invalid_cable_url` | the mint's `url` violates cable-URL policy (non-`wss://` outside localhost, a redirect on dial, unparseable) — recurring by construction on every re-mint, so it is surfaced, never retried into |

`auth_revoked` is deliberately **reserved, not used**: the wire carries no distinct
revocation signal — revocation is one possible cause of repeated unauthorized failures, and
the error name must not claim certainty the wire cannot substantiate. It comes into
existence only if bc3 ships an observable revocation contract.

Continuable — none of these end iteration: mint transients and throttles (Retry-After
floors the next reconnect delay), poll transients and throttles (Retry-After waited
exactly) — server-directed waits exempt from local caps per §7 in both lanes — dial
failures, socket drops, `remote` disconnects, `unauthorized`-kind poll errors below the
shared-counter threshold (recovery rides the reconnect cycle),
`unauthorized` failures below the threshold, staleness, 400-position (re-enter `since=`),
409 (discard the held position, re-enter `since=`), and a 410 whose registered handler
returns Accept (resume via the provided URL).

### Seam Contracts

Five seams isolate the connector from wire I/O, time, and persistence. `CableTransport` and
`Clock` are **product surface, not test hooks** — they are the documented extension points
for custom WebSocket stacks and embedded runtimes, and the reason the conformance harness
is deterministic.

```
INTERFACE TicketMinter
  mint_stream_ticket(cancellation) → StreamTicket
  -- One fully-governed generated CreateStreamTicket call. `cancellation` is the same
  -- language-native channel the dial seam takes (Go context, TS AbortSignal, …): the
  -- connector triggers it on close(), caller cancellation, and any teardown of the
  -- attempt the call belongs to, and a triggered call MUST
  -- return promptly — the universal edge to Closed cannot wait out a stalled request.
END

RECORD StreamTicket
  ticket     : String    -- opaque bearer credential; never logged
  expires_in : Integer   -- seconds (~120); server-owned, NEVER used for client scheduling
  url        : String    -- connect verbatim; never assemble cable topology client-side
END
-- Mint errors carry a kind: transient | throttled(retry_after) | unauthorized |
-- unrecoverable(error). The adapter maps every §6/§7 outcome onto exactly one kind:
-- retryable outcomes exhausted inside the seam → transient/throttled; 401/403 →
-- unauthorized (shared counter); anything else non-retryable (404, 422, a malformed
-- success) → unrecoverable → Terminal(mint_failed), generated error attached.

INTERFACE PollSource
  poll(cursor: Cursor, filters: Filters, cancellation) → PollPage
  -- One fully-governed generated PollEvents call; `cancellation` as on TicketMinter —
  -- triggered on close(), caller cancellation, AND any teardown of the attempt the call
  -- belongs to (mid-walk socket failure, staleness, a terminal): a superseded poll must
  -- not stall reconnection or return into a disposed attempt. Prompt return required.
END

RECORD Cursor           -- exactly one field set; the zero Cursor is the bare present entry
  position : String?    -- durable resume/repair token
  since    : String?    -- "now", "0", or a decimal event id
  page_url : String?    -- absolute URL: a `next` continuation OR a 410 resume URL.
                        -- Same-origin + no-downgrade validated BEFORE any poll call
                        -- (Continuation and Resume URL Validation)
END

RECORD PollPage         -- the body envelope IS the contract; never bind to response headers
  events   : List<Event>
  position : String     -- the ONLY thing that ever advances the checkpoint
  next     : String?    -- continuation URL; absent = the walk reached its frozen head.
                        -- Bound to that walk; NEVER persisted.
END
-- Poll errors carry a kind: transient | throttled(retry_after) | position_invalid |
-- filter_invalid(server message) | filter_changed | gone(epoch_after_id, resume_url) |
-- unauthorized | unrecoverable(error).
-- The adapter maps every §6/§7 outcome of the generated call onto exactly one kind:
-- 429/503 and §7-retryable outcomes exhausted inside the seam → transient/throttled;
-- the feed's 400/409/410 matrix → its four kinds; 401/403 (after the seam's own token
-- refresh and retry budget) → unauthorized; anything else non-retryable (404, 405,
-- unexpected shapes) → unrecoverable, carrying the generated error verbatim.

INTERFACE CableTransport
  dial(ws_url, cancellation, max_frame_bytes) → CableConn
  -- Dials exactly one connection per call. MUST NOT auto-reconnect. MUST NOT interpret,
  -- filter, or swallow application text frames. Negotiates subprotocol
  -- "actioncable-v1-json". Refuses redirects (§23 Security Invariants).
  -- `cancellation` is the language-native cancellation channel (Go context, TS
  -- AbortSignal, Kotlin coroutine cancellation, Swift task cancellation, a Python/Ruby
  -- cancel token): the connector triggers it on handshake-deadline expiry and on
  -- close(), and a triggered dial MUST return promptly — a dial that cannot be
  -- interrupted violates this contract.
  -- `max_frame_bytes`: the transport MUST enforce this limit WHILE reading (e.g. a
  -- read-limit on the socket), rejecting an over-limit message without materializing it
  -- — the security cap has to bind inside the WebSocket stack, or the allocation happens
  -- before the connector can measure anything. The rejection surfaces from read_frame as
  -- an error and takes the frame-violation socket-failure dispatch (Cable Protocol
  -- Details).
  -- Dial errors carry a kind: transient | policy(reason). `policy` is a PERMANENT
  -- refusal the transport detected (a redirect encountered; a scheme the invariants
  -- forbid; an unparseable URL) → Terminal(`invalid_cable_url`), never Backoff — a
  -- fresh mint returns the same unusable URL. Everything else is `transient` →
  -- transition 7. The connector performs the scheme/parse checks it can before dialing;
  -- only the transport can see a redirect.
END

INTERFACE CableConn
  read_frame() → String  -- the next raw text frame VERBATIM — including
                         -- {"type":"disconnect",...}: the terminal/non-terminal
                         -- distinction lives only in this raw frame. Byte-level
                         -- representation is language-native (Go []byte); verbatim-ness
                         -- is the contract. Peer close surfaces as CloseError{code, reason}.
  write_frame(String)
  close(code, reason)    -- idempotent, safe from any context, unblocks read_frame
END

INTERFACE Clock
  now() → monotonic reading            -- used ONLY for deltas, never persisted
  new_timer(duration, kind) → Timer    -- kind-labelled, cancellable, enumerable
  outstanding() → List<kind>           -- kinds of live (unfired, unstopped) timers
END
-- Product seam, registry included: the system clock keeps the same enumerable registry
-- the test clocks rely on, which is what makes "no timer survives teardown" an exact-set
-- assertion (`outstanding()`) rather than a test-only artifact.

INTERFACE CheckpointStore
  load(key: CheckpointKey) → (position: String, ok: Boolean)
  save(key: CheckpointKey, position: String)
END

RECORD CheckpointKey
  origin             : String   -- canonicalized (Checkpoint Identity below)
  account_id         : String
  consumer_namespace : String   -- required whenever a store is configured
  filter_key         : String   -- "srv1-" + bare server digest
END
```

Frame-parsing ownership: **the connector parses every frame; the transport moves bytes.**
Application-level `{"type":"ping"}` frames are connector business (staleness);
WebSocket-level ping/pong control frames stay inside the transport implementation. This
split is what makes the protocol-fatal disconnect interceptable by contract rather than by
library-specific hacks.

### Seam-Call Semantics `[conformance]`

**One seam call (`mint_stream_ticket`, `poll`) is one fully-governed generated call.** The
generated operation keeps its full §7 contract *inside* the seam — internal retry budget,
declared `retry_on` gate, backoff, Retry-After. `CreateStreamTicket` is safe-to-retry (a
stateless mint — deliberately not a claim of identical responses), so §7 retry applies
inside a mint seam call. The connector **never adds a second per-request retry layer**: its
backoff and Retry-After logic governs **reconnect cycles and poll-walk resumption only**,
and treats seam errors as post-retry outcomes. Conformance mint/connect/poll counts count
seam calls, never wire attempts.

### Continuation and Resume URL Validation `[conformance]`

The two absolute URLs the poll lane follows — the envelope's `next` continuation and a 410
body's `resume` URL — carry the caller's `Authorization` bearer when followed. Before a
`poll(Cursor{page_url})` call is made, the connector validates the URL against the
configured base origin with §8's Same-Origin Validation Algorithm, and rejects a protocol
downgrade (HTTPS → HTTP) — the same rule, for the same reason, as §8's pagination `Link`
rejection: a cross-origin or downgraded URL in a response body must never redirect an
authenticated request (SSRF and token leakage). A URL that fails validation is
Terminal(`invalid_continuation`) — no request is issued to the failing URL, and the
rejected URL is carried redacted (origin only) in the error. There is no retry and no
handler for this condition: a hostile continuation is not an operable feed state.

The mint's cable `url` is deliberately **not** under this rule: it is server-directed
cable topology, cross-host by design, dialed verbatim with its own credential (the
short-lived ticket rides in the URL itself; no `Authorization` header is attached), and
governed by its own invariants — `wss://` outside localhost, redirects refused, never
logged (Security Invariants below).

Required tier-2 coverage: a hostile cross-origin `next` mid-walk and a hostile 410
`resume` URL each terminate with `invalid_continuation` and zero requests to the foreign
origin.

### Clock, Timers, and Virtual Time `[conformance]`

**Every delay the connector takes flows through the injected Clock** — no native timer or
sleep may bypass it. There are exactly six timer kinds, kebab-case:

| Kind | Armed | Duration |
|---|---|---|
| `handshake-deadline` | on entry to Connecting, BEFORE `dial` is invoked — it spans dial-to-`welcome`, so a stalled TCP connect or HTTP upgrade cannot hang the connector | `EVENT_FEED_HANDSHAKE_DEADLINE` (10s) |
| `confirmation-deadline` | on `welcome` → subscribe (transition 8) | `EVENT_FEED_CONFIRMATION_DEADLINE` (10s default, configurable) |
| `backoff` | on entry to Backoff | full-jitter: `uniform(0, min(60s, 1s × 2^(n−1)))` for failed-cycle count n; Retry-After floors it, and wins outright when it exceeds the cap (server-directed waits are exempt, §7) |
| `staleness` | at socket open; re-armed per inbound frame of any kind | 7500ms |
| `repair-poll` | on entry to Streaming; re-armed per cycle | 60s ± 20% jitter per cycle |
| `poll-retry` | on a transient/throttled poll inside CatchingUp | Retry-After when present (exact, cap-exempt per §7); else full-jitter `uniform(0, min(60s, 1s × 2^(k−1)))` on the consecutive-poll-failure index k (reset by a successful poll or socket teardown) |

The connector's reconnect backoff is deliberately **not** the §7 per-request formula: it is
full-jitter delay *selection* over the whole range (the §7 formula saturates its backoff
term at `MAX_BACKOFF_DELAY_MS` = 30s and adds jitter on top; this one draws uniformly from
`[0, min(60s, 1s × 2^(n−1)))` with no added term). The two govern different things — §7
governs attempts inside a seam call; this governs cycles between them. The same 60s cap
bounds `poll-retry`'s locally computed jitter draw; a server-directed `Retry-After` is
exempt from both caps, per §7.

**Saturate before exponentiating** — §7's overflow rule applies to both formulas: an
implementation MUST compare the failure index (n or k) against the cap-crossing exponent
before evaluating `2^(n−1)`/`2^(k−1)` (for base 1s and cap 60s, any index above 7 already
saturates the range at 60s). Evaluating the power first overflows fixed-width integers
after a long genuine outage (~64 consecutive failures), producing exactly the tight-loop
or crash failure §7's saturation rules exist to prevent.

**Virtual-advance algorithm (normative).** Every language's test clock must honor the same
semantics, so a tier-2 script means the same thing everywhere: *advancing virtual time
fires due timers in deadline order, re-evaluating after each fire; timers scheduled during
the advance whose deadlines land inside the window also fire; ties break by creation
order.* A harness may additionally fire a named timer without advancing the clock,
asserting its scheduled delay against a `{min, max}` envelope — that is how jitter is
asserted without a cross-language RNG seam. Each language's test clock passes a shared
semantics checklist (deadline order, reentrant scheduling within an advance, creation-order
tie-break) before its tier-2 results count.

Teardown discipline: disposing a connection attempt — deadline lapse, staleness, socket
death, terminal — cancels the frame pump, **cancels any in-flight seam call belonging to
the attempt** (a stalled poll must not delay the reconnect cycle or return into a
superseded attempt), closes the connection, and stops **all** of that
attempt's timers before the next state is entered. After a confirmation-deadline teardown,
the exact outstanding-timer set is `{backoff}`; after a terminal, it is `{}`. Exact-set
timer assertions are what make a leaked deadline timer, ghost watchdog, or duplicated
backoff a hard failure rather than a heuristic.

### Checkpoint Identity `[conformance]`

Checkpoint identity is `{origin, account_id, consumer_namespace, filter_key}` — all four,
always:

- Server positions are bound to `{account, filter set}` but carry **no consumer identity**;
  two independent consumers in one account would otherwise share a lineage and silently
  skip each other's work. `consumer_namespace` is therefore a required input whenever a
  store is configured (a configured store with an empty namespace fails construction with
  a `usage`-coded language-native error — Consumer Surface above).
- `origin` is included because the SDK supports configurable base URLs; a server-side
  cursor-domain key is not a safe client persistence key on its own. **Origin
  canonicalization:** lowercase scheme and host; omit the default port (`:443` for https,
  `:80` for http); no path, query, fragment, or trailing slash — canonical form exactly
  `scheme "://" host [":" nondefault-port]`. Hosts are used as configured after lowercasing
  (no IDN/punycode transformation).
- `filter_key = "srv1-" + <bare 16-lowercase-hex server digest>`. The **server wire format
  is the bare hex** — exactly what the 409 body's `position_digest`/`filters_digest` carry;
  the server never emits the `srv1-` prefix. It is the SDK-side checkpoint-lineage
  namespace only.

**srv1 canonicalization (the published server contract; the SDK implements it as
published, not as a mirror of server internals):**

```
digest = lowercase_hex(SHA-256(UTF-8(canonical_json)))[0:16]   -- 16 hex chars = first 8 bytes
canonical_json = "[" T "," B "," C "]"     -- compact: no whitespace anywhere
  T = null if no types,   else a JSON array of the type strings, deduped,
      sorted bytewise-ascending over their UTF-8 encodings
  B = null if no buckets, else a JSON array of integers: base-10 coerced, deduped AFTER
      coercion ("1" and "01" are one id), numerically ascending, canonical integer
      rendering (no sign for positives, no leading zeros, no fraction, no exponent)
  C = same as B, for creators
  absent list ⇒ null; empty filter set ⇒ the input is exactly [null,null,null]
  string escaping: RFC 8259 minimal — only ", \, and control characters U+0000–U+001F;
  NO HTML escaping; no \uXXXX for non-control characters
```

The canonical bytes are hand-built (string builder plus a minimal escape helper) — no
language's default JSON emitter is load-bearing, because several HTML-escape by default.
**The algorithm is total over every client-validated input, and the SDK computes it for
any filter set that passes construction validation** — catalog membership is deliberately
NOT client-validated (the catalog is server-owned and grows), and the checkpoint key must
form before the first poll can answer. A syntactically valid but uncataloged type
therefore gets a well-defined `filter_key` and its `load` runs normally; the first poll
then draws the server's filter 400 (Terminal(`filter_invalid`)) and that lineage simply
never advances — harmless. The *server-side* srv1 domain is the cataloged ASCII type
strings and integer ids: the server rejects unknown types with the filter 400 before
computing any digest, which is why bc3 publishes no quoted-string or non-ASCII vectors.

Published srv1 vectors (provisional until the merge-time gate; the conformance vector
source):

| Input | Canonical JSON | Digest |
|---|---|---|
| no filters | `[null,null,null]` | `fe44a8cccd89edae` |
| `types=message.created` | `[["message.created"],null,null]` | `9eae6bbae1414746` |
| `types=todo.completed,message.created&buckets=2,1` (unsorted multi-list) | `[["message.created","todo.completed"],[1,2],null]` | `00ed03e6196e77a2` |
| `buckets=01` (also `buckets=1,01` — post-coercion dedup) | `[null,[1],null]` | `fb19e601cd033cad` |
| `buckets=1,...,100` (the cap boundary) | `[null,[1,2,...,100],null]` | `832adbc56aa7c8f2` |

The one built-in store is a file store: a single JSON file keyed by the compact RFC 8259
JSON array of the four identity strings — e.g.
`["https://3.basecampapi.com","5951425","openclaw","srv1-9f2ab04e5c11d3a7"]` — written
atomically (temp + rename, 0600), documented as single-process (a server-side advisory
checkpoint API is deliberately deferred until a multi-host connector needs a shared
cursor). No `delete` method exists: after a 409 the connector re-enters via `since=` and
the next page's `save` overwrites under the same key, and after a filter change the key
itself changed, so the old lineage simply goes cold.

Checkpoints only move forward; an access grant does not replay history behind them.
Inspecting a newly granted bucket's past activity is an explicit caller choice: rewind to
an older stored position, reset via `since=`, or (preferred for agents) fetch canonical
resources.

### Options and Per-Language Naming `[static]`

Following the §16 device-flow precedent (injectable clock, per-language option idiom), the
connector's options map per language as follows. Go uses functional options; TypeScript an
options object with optional fields; Python and Ruby keyword arguments (Ruby with
`DEFAULT_…` constants); Kotlin constructor parameters; Swift initializer parameters.

| Concern (default) | Go option | TS field | Python / Ruby kwarg | Kotlin / Swift parameter |
|---|---|---|---|---|
| Filters (none) | `WithFilters` | `filters?` | `filters` | `filters` |
| Entry mode (resume: stored position if any, else present; also present / beginning / after(id) / at-position(token)) | `WithStart` | `start?` | `start` | `start` |
| Cable transport (default WebSocket impl) | `WithTransport` | `transport?` | `transport` | `transport` |
| Clock (system monotonic) | `WithClock` | `clock?` | `clock` | `clock` |
| Checkpoint store (none) | `WithCheckpointStore` | `checkpointStore?` | `checkpoint_store` | `checkpointStore` |
| Consumer namespace (required with a store) | `WithConsumerNamespace` | `consumerNamespace?` | `consumer_namespace` | `consumerNamespace` |
| Confirmation deadline (10s) | `WithConfirmationDeadline` | `confirmationDeadlineMs?` | `confirmation_deadline` | `confirmationDeadline` |
| Repair interval (60s ± 20%) | `WithRepairInterval` | `repairIntervalMs?` | `repair_interval` | `repairInterval` |
| Dedupe capacity (10,000) | `WithDedupeCapacity` | `dedupeCapacity?` | `dedupe_capacity` | `dedupeCapacity` |
| Live buffer capacity (10,000) | `WithLiveBufferCapacity` | `liveBufferCapacity?` | `live_buffer_capacity` | `liveBufferCapacity` |
| Signal handler (none ⇒ default-terminal) | `WithSignalHandler` | `signalHandler?` | `signal_handler` | `signalHandler` |
| Observer (none) | `WithObserver` | `observer?` | `observer` | `observer` |

The Observer is a struct of optional callbacks in the `httptrace.ClientTrace` style —
extensible without breaking implementers: `connecting(attempt, delay)`, `connected()`,
`confirmed()`, `disconnected(reason, error)`, `catch_up_started(cursor)`,
`page_delivered(count, position)`, `checkpoint(position)` (after that page's events were
accepted), `checkpoint_save_failed(error)`, `caught_up()`, `gap(epoch_after_id,
resume_url)`, `position_rejected(kind)`, `stale_connection(since_last_frame)`,
`buffer_overflow(dropped_count)`. All are observability-only; none carries a disposition.

### Security Invariants `[static]`

- **Never log the ticket or the mint URL's query string** — the ticket rides in it.
  Observer callbacks and error renderings carry redacted URLs.
- **Bound the inbound frame size** (`EVENT_FEED_MAX_FRAME_BYTES`, 1 MiB default) and
  bound/truncate any error rendering of frame contents (§9's `MAX_ERROR_MESSAGE_LENGTH`
  applies).
- **Require `wss://`** for the cable URL, with the §9 localhost/loopback carve-out.
- **Refuse mint-URL redirects** — a redirect on dial is a hard error, never followed.
- **Validate every continuation and resume URL** before following it — §8's same-origin
  algorithm plus downgrade rejection, terminal `invalid_continuation` on failure
  (Continuation and Resume URL Validation above). Authenticated poll requests never
  follow a cross-origin URL.

### Constants

The connector's constants live in Appendix A (the `EVENT_FEED_*` rows): handshake deadline
10s; confirmation deadline 10s; repair interval 60s ± 20% jitter per cycle; reconnect
backoff base 1s, ×2, cap 60s, full-jitter (the same cap bounds `poll-retry`'s jitter draw;
server-directed `Retry-After` is exempt from local caps per §7); server
heartbeat cadence 3s; staleness 7500ms; authorization-failure threshold 3; dedupe capacity
10,000 ids; live buffer capacity 10,000 events; ticket TTL ~120s (server-owned —
`expires_in` is never used for client-side scheduling; expiry is arbitrated by the server
and the connector always mints fresh); maximum inbound frame 1 MiB.

### Verification

Verification is three tiers with disjoint responsibilities:

| Tier | What it proves | Where it lives |
|---|---|---|
| 1 — poll-lane wire behavior | request/response contract of the generated `PollEvents` as ordinary operation-dispatch cases | `conformance/tests/` — deferred to the generated layer's landing |
| 2 — connector protocol scenarios | the cross-lane state machine, frame-level cable behavior, mint/connect/poll interleave, virtual time | `conformance/event-feed/` fixture family + per-SDK native scenario drivers |
| 3 — per-language internals | what data fixtures cannot express: LRU bounds, jitter formula, single-flight under real concurrency, real transport adapter contract, test-clock semantics | per-SDK unit/integration tests against the seams |

The tier-2 family has its own schema and README; scenarios are strictly-ordered interleaved
scripts (an unexpected mint, poll, connect, or outbound frame fails the scenario), with
time consumed through the Clock seam per the virtual-advance algorithm above. Four
acceptance behaviors are cross-language contracts: **(a)** fresh-ticket reconnect — after a
severed socket past the ticket TTL, the reconnect presents a newly minted ticket URL;
**(b)** confirmation gating — zero deliveries, zero polls, zero saves before
`confirm_subscription`; **(c)** deadline teardown — the disposed attempt leaves exactly
`{backoff}` outstanding; **(d)** terminal rejection — the connector closes the open socket,
zero reconnect attempts, no armed retry timer survives. Handler-invocation records and the
present-class-entry fixtures pin the Entry Boundary and Semantic Signals contracts;
mint/connect/poll counts in fixtures count seam calls per Seam-Call Semantics. The
fixture-by-fixture inventory lives in the family's own README — following the
oauth/oauth-token precedent, parallel fixture families are documented at their consuming
section and directory, not in §19's operation-dispatch category table or Appendix D.
Per-SDK scenario-lane divergences (Swift's fake-transport lane, Kotlin's jvmTest-only
consumption) are recorded in Appendix F with their compensating tier-3 tests.

---

## Appendix A: Constants Reference

All magic numbers in one place, derived from shipping SDK code (not `rubric-audit.json`).

Only `API_VERSION` is gated (`<!-- @api-version -->`, checked by `make doc-constants-check`). The other 14 pre-§23 rows are hand-maintained: 13 were read against their cited sources on 2026-08-03 — all 13 matched — and `MAX_BACKOFF_DELAY` joined with #592 under that PR's own six-SDK verification (the sentence previously said 13 rows while the table carried 14). The `EVENT_FEED_*` block below them is different in kind and marked so: those rows are contract-first — their source is §23's normative text, connector code ships in later PRs, and the two server-owned values are provisional until bc3's merge-time gate; when the connector lands, they join the read-against-source discipline. They are not gated because each is asserted of several SDKs at once in a different spelling per language (Go `1 * time.Second`, Python `1.0`, Ruby `1.0`, Kotlin `30.seconds`, Swift `1_000`), so a checker would need a per-row, per-language extraction rule rather than the one-value-one-source substitution the marker convention is built on. The name in the table is the concept, not a symbol to grep: `MAX_ERROR_MESSAGE_LENGTH` is `MaxErrorMessageBytes` in Go and `MAX_ERROR_MESSAGE_BYTES` in Ruby, and `TOKEN_REFRESH_BUFFER` is the literal `300` in `creds.ExpiresAt-300` (`go/pkg/basecamp/auth.go`) rather than a named constant at all. If one of these starts moving, gate that row rather than the appendix.

| Constant | Value | Unit | Source |
|----------|-------|------|--------|
| `MAX_RESPONSE_BODY_BYTES` | 52,428,800 (50 MiB) | bytes | `go/pkg/basecamp/security.go`, `ruby/lib/basecamp/security.rb`; Go/Ruby enforce; TS/Kotlin/Swift do not |
| `MAX_ERROR_BODY_BYTES` | 1,048,576 (1 MiB) | bytes | `go/pkg/basecamp/security.go`, `ruby/lib/basecamp/security.rb` |
| `MAX_ERROR_MESSAGE_LENGTH` | 500 | bytes (Go/Ruby/Python) or code units (TS/Swift/Kotlin) | All six SDKs |
| `DEFAULT_BASE_URL` | `https://3.basecampapi.com` | — | All six SDKs |
| `DEFAULT_TIMEOUT` | 30 | seconds | All six SDKs |
| `DEFAULT_CONNECT_TIMEOUT` | 10 | seconds | `ruby/lib/basecamp/http.rb` (Faraday open_timeout); recommended default, not a required config field |
| `DEFAULT_MAX_RETRIES` | 3 | — | All six SDKs |
| `DEFAULT_BASE_DELAY` | 1000 | milliseconds | All six SDKs |
| `DEFAULT_MAX_JITTER` | 100 | milliseconds | All six SDKs |
| `MAX_BACKOFF_DELAY` | 30,000 (30s) | milliseconds | All six SDKs; ceiling on the §7 backoff term, jitter added on top. Was Go's generated `RetryConfig.MaxDelay` before #577 generalized it |
| `DEFAULT_MAX_PAGES` | 10,000 | — | All six SDKs |
| `MAX_CACHE_ENTRIES` | 1000 | entries | `typescript/src/client.ts` |
| `MAX_TOKEN_HASH_ENTRIES` | 100 | entries | `typescript/src/client.ts` |
| `API_VERSION` | `2026-08-02` | — | `openapi.json` `info.version` <!-- @api-version --> |
| `TOKEN_REFRESH_BUFFER` | 300 | seconds | Go OAuth token refresh threshold (5-minute buffer); Ruby refreshes only on expiry (no buffer); TS/Kotlin/Swift delegate expiry to caller |
| `EVENT_FEED_HANDSHAKE_DEADLINE` | 10 | seconds | §23 timers — dial-to-`welcome` deadline |
| `EVENT_FEED_CONFIRMATION_DEADLINE` | 10 | seconds | §23 (configurable; default) |
| `EVENT_FEED_REPAIR_INTERVAL` | 60, ± 20% jitter per cycle | seconds | §23 (configurable; default) |
| `EVENT_FEED_BACKOFF_BASE` | 1 | seconds | §23 — full-jitter base for both the reconnect-cycle and `poll-retry` formulas |
| `EVENT_FEED_BACKOFF_CAP` | 60 | seconds | §23 — caps the locally computed reconnect-cycle and `poll-retry` delays (server-directed `Retry-After` is exempt, §7); deliberately distinct from `MAX_BACKOFF_DELAY` above (§7's `MAX_BACKOFF_DELAY_MS`, 30s), which governs attempts inside a seam call |
| `EVENT_FEED_PING_INTERVAL` | 3 | seconds | server heartbeat cadence (bc3; provisional until the merge-time gate); input to `EVENT_FEED_STALE_AFTER` |
| `EVENT_FEED_STALE_AFTER` | 7500 | milliseconds | §23 — two missed 3s heartbeats + 25% grace; SDK-pinned detection policy |
| `EVENT_FEED_AUTH_FAILURE_THRESHOLD` | 3 | consecutive failures | §23 disconnect dispatch (one shared counter) |
| `EVENT_FEED_DEDUPE_CAPACITY` | 10,000 | event ids | §23 (configurable; default) |
| `EVENT_FEED_LIVE_BUFFER_CAPACITY` | 10,000 | events | §23 (configurable; default; decoupled from the dedupe capacity; only event-bearing frames are buffered) |
| `EVENT_FEED_TICKET_TTL` | ~120 | seconds | server-owned (`expires_in`; provisional until the merge-time gate); never used for client scheduling |
| `EVENT_FEED_MAX_FRAME_BYTES` | 1,048,576 (1 MiB) | bytes | §23 security invariants |

---

## Appendix B: Canonical Service Surface

Repeated from §5 for quick reference.

**Client-level (1):** authorization

**AccountClient-level (46):**
account, attachments, automation, boosts, campfires, cardColumns, cardSteps, cardTables, cards, checkins, clientApprovals, clientCorrespondences, clientReplies, clientVisibility, comments, documents, events, everything, forwards, gauges, hillCharts, lineup, messageBoards, messageTypes, messages, myAssignments, myNotifications, people, projects, recordings, reports, schedules, search, subscriptions, templates, timeline, timesheets, todolistGroups, todolists, todos, todosets, tools, uploads, vaults, webhooks, wormholes

---

## Appendix C: Rubric Criteria Cross-Reference

| Rubric ID | Spec Section | Summary |
|-----------|-------------|---------|
| 1A.1 | §18, §21 | Smithy model validates |
| 1A.2 | §18, §21 | OpenAPI derived from Smithy |
| 1A.6 | §18, §23 | No hand-written wire methods; conformance-tested composites permitted; the §23 cable dial is the one sanctioned non-HTTP wire act |
| 1B.2 | §18 | Types generated from OpenAPI schema |
| 1B.4 | §10 | Optional fields use language optionals |
| 1B.5 | §10 | Date fields use ISO 8601 / native types |
| 1B.6 | §10 | 64-bit integer precision |
| 1C.1 | §3 | API paths verified against upstream |
| 1C.3 | §3, §18 | No manual path construction |
| 2A.1 | §6 | Structured error type |
| 2A.3 | §6 | HTTP status → error code mapping |
| 2A.5 | §6, §7 | Retry-After header parsed (integer + HTTP-date) |
| 2A.6 | §9 | Error body truncation |
| 2B.1 | §7 | Retry middleware exists |
| 2B.3 | §7 | Idempotent methods retried |
| 2B.4 | §7 | POST not retried unless idempotent |
| 2B.5 | §7 | 403 not retried |
| 2C.1 | §8 | Auto-pagination via Link headers |
| 2C.2 | §8 | X-Total-Count header exposed |
| 2C.3 | §8 | maxPages safety cap |
| 2C.4 | §8 | maxItems early-stop |
| 2C.5 | §8 | Cross-origin Link header rejected |
| 2C.6 | §8 | Truncation metadata exposed |
| 2D.5 | §7 | Per-operation retry config |
| 3A.3 | §4, §13 | Bearer token in Authorization header |
| 3A.4 | §16 | OAuth PKCE discovery |
| 3A.5 | §16 | OAuth PKCE code exchange |
| 3A.6 | §4 | Token auto-refresh with expiry buffer |
| 3C.1 | §9 | HTTPS enforcement |
| 3C.2 | §9 | Response body size limit |
| 3C.3 | §9 | Error message truncation |
| 3C.4 | §9 | Authorization header redacted |
| 3C.6 | §8 | Same-origin pagination validation |
| 4A.1 | §21 | Smithy → OpenAPI freshness check |
| 4B.5 | External governance (AGENTS.md) | Tests for every operation |
| 4C.4 | External governance (AGENTS.md) | Release workflows idempotent |

---

## Appendix D: Conformance Test → Spec Section Mapping

| Test file | Test name | Primary section |
|-----------|----------|----------------|
| `auth.json` | Bearer token injected | §4, §13 |
| `auth.json` | User-Agent header present | §13 |
| `auth.json` | Bearer token value matches | §4 |
| `auth.json` | Content-Type on POST | §13 |
| `error-mapping.json` | 401 → auth_required | §6 |
| `error-mapping.json` | 403 → forbidden | §6 |
| `error-mapping.json` | 404 → not_found | §6 |
| `error-mapping.json` | 400 → validation | §6 |
| `error-mapping.json` | 422 → validation | §6 |
| `error-mapping.json` | 422 field-keyed errors flatten into the message | §6 |
| `error-mapping.json` | 422 field-keyed errors sort and join multi-message fields | §6 |
| `error-mapping.json` | 422 field-keyed errors append to a top-level error message | §6 |
| `error-mapping.json` | 422 field-keyed errors survive a non-string top-level error | §6 |
| `error-mapping.json` | 422 field-keyed errors append after a message-key fallback | §6 |
| `error-mapping.json` | 422 field-keyed errors treat `__proto__` as an ordinary field name | §6 |
| `error-mapping.json` | 422 field-keyed errors keep valid entries beside malformed ones | §6 |
| `error-mapping.json` | 400 bare field-map body flattens into the message | §6 |
| `error-mapping.json` | 400 bare field-map body sorts and joins multi-message fields | §6 |
| `error-mapping.json` | 400 bare field-map body treats `__proto__` as an ordinary field name | §6 |
| `error-mapping.json` | 400 body with a string error key keeps the flat message | §6 |
| `error-mapping.json` | 429 → rate_limit | §6 |
| `error-mapping.json` | 500 → api_error | §6 |
| `error-mapping.json` | 502 → api_error (retryable) | §6 |
| `error-mapping.json` | 503 → api_error (retryable) | §6 |
| `error-mapping.json` | 504 → api_error (retryable) | §6 |
| `error-mapping.json` | X-Request-Id extracted | §6 |
| `idempotency.json` | PUT retries on 503 | §7 (Gate 1) |
| `idempotency.json` | DELETE retries on 503 | §7 (Gate 1) |
| `idempotency.json` | POST does NOT retry | §7 (Gate 2) |
| `retry.json` | GET retries on 503 | §7 |
| `retry.json` | GET retries on 429 with Retry-After | §7 |
| `retry.json` | POST does NOT retry (503) | §7 (Gate 2) |
| `retry.json` | POST does NOT retry (429) | §7 (Gate 2) |
| `retry.json` | 404 not retried | §7 (Gate 3) |
| `retry.json` | 403 not retried | §7 (Gate 3) |
| `retry.json` | Retry-After HTTP-date respected | §6, §7 |
| `security.json` | Cross-origin Link rejected | §8, §9 |
| `security.json` | HTTPS enforced (non-localhost) | §9 |
| `security.json` | HTTP allowed for localhost | §9 |
| `security.json` | Same-origin pagination | §8 |
| `security.json` | Protocol downgrade rejected | §8, §9 |
| `pagination.json` | First page with Link header | §8 |
| `pagination.json` | X-Total-Count accessible | §8 |
| `pagination.json` | Auto-pagination follows links | §8 |
| `pagination.json` | maxPages safety cap | §8 |
| `pagination.json` | Missing X-Total-Count → 0 | §8 |
| `pagination.json` | maxItems caps results | §8 |
| `pagination.json` | maxItems exact landing not truncated | §8 |
| `status-codes.json` | GET → 200 | §11 |
| `status-codes.json` | PUT → 200 | §11 |
| `status-codes.json` | POST create → 201 | §11 |
| `status-codes.json` | DELETE → 204 | §11 |
| `status-codes.json` | 4xx/5xx surfaced as errors | §11 |
| `status-codes.json` | Non-retryable not retried | §7, §11 |
| `integer-precision.json` | Large integer IDs preserved | §10 |
| `paths.json` | Path construction | §3 |
| `downloads.json` | DownloadURL auth'd first hop 302s to signed URL | §14 |
| `downloads.json` | DownloadURL direct 2xx body | §14 |
| `downloads.json` | DownloadURL retries on 503 at the auth'd first hop | §14, §7 |
| `downloads.json` | DownloadURL retries hop 1 on a network error | §14, §7 |
| `downloads.json` | DownloadURL does not retry hop 1 on 500 | §14, §7 |
| `downloads.json` | DownloadURL honors Retry-After on 429 at the auth'd first hop | §14, §7 |
| `downloads.json` | DownloadURL surfaces redirect with no Location | §14 |
| `network-retry.json` | Network error on a non-idempotent POST is not retried | §7 (Gate 2) |
| `network-retry.json` | Network error on an idempotent POST is retried then succeeds | §7 (Gate 2) |
| `uploads_download.json` | UploadsDownload delegates through DownloadURL primitive | §14, §18 |
| `uploads_download.json` | UploadsDownload errors when upload has no download_url | §14, §18 |
| `todos_write.json` | update-merge / edit-clear / replace-omission-clears | §5 (Todos), §18 |
| `todolists_write.json` | update-merge / update-group / edit-clear / replace-omission-clears | §5 (Todolists), §18 |
| `cards_write.json` | Merge-safe update composite (5 cases: due-on preservation, verbatim raw path, explicit clears/empties) | §5 (Cards), §18 |
| `schedule_entries_write.json` | update-omits-participant-ids / update-empty-participant-ids | §10 |
| `live-my-surface.json` | Live schema validation, 31 read-surface cases (opt-in via `BASECAMP_LIVE`) | External governance (CONTRIBUTING.md, live canary) |

---

## Appendix E: behavior-model.json Schema

### Structure

```
{
  "$schema": "https://basecamp.com/schemas/behavior-model.json",
  "version": "1.0.0",
  "generated": true,
  "operations": {
    "<OperationId>": {
      "idempotent": true,           ← optional; only present when true
      "retry": {
        "max": 3,                   ← total attempts (including first)
        "base_delay_ms": 1000,      ← initial delay before first retry
        "backoff": "exponential",   ← always "exponential" in practice
        "retry_on": [429, 503]      ← HTTP statuses that trigger retry
      }
    }
  },
  "redaction": { "<TypeName>": ["$.fieldPath", ...] },
  "sensitiveTypes": ["AvatarUrl", "EmailAddress", ...]
}
```

The `redaction` and `sensitiveTypes` sections are used for PII handling and are not part of the retry/idempotency contract. They appear in the schema snippet above for completeness but are out of scope for retry/idempotency semantics.

### Field Semantics

| Field | Meaning |
|-------|---------|
| `idempotent` | When `true`, the operation is safe to retry even if it's a POST. Absent (or `false`) means POST must not be retried. |
| `retry.max` | Total number of attempts. `max: 3` means 1 initial + 2 retries. |
| `retry.base_delay_ms` | Base delay for exponential backoff. First retry waits `base_delay_ms`, second waits `base_delay_ms * 2`, etc. |
| `retry.retry_on` | HTTP status codes that trigger retry. Always `[429, 503]` in the current model. |

### Inert Retry Block on Non-Idempotent POSTs

Every operation has a `retry` block, including non-idempotent POSTs. For non-idempotent POSTs, the `retry` block is **inert metadata** — it describes what parameters WOULD apply if the operation were retryable, but the absence of `idempotent: true` prevents retry activation. This is the Gate 2 mechanism from §7.

### Operation Counts

- Total operations: 243
- Idempotent: 79 (flagged with `idempotent: true`)
- Non-idempotent: 164 (no `idempotent` field, or not present)
- All operations use `retry_on: [429, 503]`

---

## Appendix F: Known Cross-SDK Divergences

### Retry Strategy (§7)

| SDK | Retry behavior |
|-----|---------------|
| TypeScript | Three-gate: POST retries only when `idempotent: true`. Retries on `retry_on` set from metadata to each operation's declared `max`, with the retry loop beneath the openapi-fetch middleware chain as the client's custom `fetch`. Network errors retry under the same idempotency gate; caller aborts and request timeouts are terminal. |
| Kotlin | Three-gate for both HTTP status and network-error retries: POST retries only when `idempotent: true`, full exponential backoff. Network errors retry through the same eligibility gate; the whole-request timeout (Ktor's `HttpRequestTimeoutException`) is deliberately not retried. |
| Go | Generated operation path retries operations classified idempotent at generation time — GET/HEAD by method, plus any operation carrying `x-basecamp-idempotent` (naturally-idempotent PUT/DELETE mutations like `UpdateProject`/`TrashProject`, and flagged-idempotent POSTs like `CompleteTodo`) — with exponential backoff; non-idempotent operations (e.g. `CreateTodo`) are single-attempt. The separate hand-written `doRequestURL` helper remains GET-only for ordinary retries, with a mutation-specific single re-attempt after successful 401 token refresh. |
| Ruby | Simplified: only GET retries. All non-GET methods never retry. Governed GETs gate status retries on the declared `retryOn` and bound attempts by `min(config.max_retries, operation max)`; ungoverned traffic (no operation ID: `get_absolute`, OAuth) keeps the taxonomy-driven contract. |
| Python | Three-gate, sync and async: `_mutation()` retries only when `behavior-model` metadata classifies the operation retryable, so non-idempotent POSTs are single-attempt; GETs always retry. Gate 3 uses the operation's declared `retry_on` and `max`. Non-Smithy traffic (`get_absolute()`, Launchpad authorization) passes no operation id and keeps the pre-Smithy contract. |
| Swift | Three-gate: retries when the method is naturally idempotent (GET/HEAD/PUT/DELETE) or the operation is marked `idempotent: true`; non-idempotent POSTs make a single attempt. Gate covers both HTTP status and network-error retries, so Swift *does* retry network errors, gated by idempotency. |

The table above describes **Gate 1 and Gate 2** — *whether* an operation retries. Gate 3's parameters
are tracked separately: all six SDKs gate status retry on the declared `retryOn` (Ruby for governed
GETs), and every SDK with a numeric caller cap — Go, Python, Kotlin, and Ruby — honors a caller
asking for *fewer* attempts than an operation declares; TypeScript and Swift expose no such cap.
See the Gate 3 consumption table in §7 above.

### Integer Precision (§10)

| SDK | Precision |
|-----|----------|
| Go | Full 64-bit (`int64`) |
| Ruby | Full arbitrary precision (Ruby Integer) |
| Kotlin | Full 64-bit (`Long`) |
| Swift | Platform-width `Int` (64-bit on all supported platforms). Generated models use `Int`, not `Int64`. |
| TypeScript | 53-bit (`Number`). Node >=22.12 can decode larger IDs losslessly as `bigint`, but that would break the number-typed API surface; waiver 1B.6 retains the limitation. |

### Pagination Metadata (§8)

| SDK | ListResult | total_count | truncated |
|-----|-----------|------------|-----------|
| TypeScript | `ListResult<T>` extends Array | yes | yes |
| Kotlin | `ListResult<T>` | yes | yes |
| Swift | `ListResult<T>` | yes | yes |
| Go | Typed `*XxxListResult` with `Meta ListMeta` | yes | yes |
| Python | `ListResult(list)` with `meta ListMeta` | yes | yes |
| Ruby | Lazy `ListEnumerator` (Enumerator subclass) with `meta ListMeta` | yes | yes (final after enumeration completes) |

### Error Message Truncation Unit (§9)

| SDK | Unit | Method |
|-----|------|--------|
| Go | bytes | `len(s)` |
| Ruby | bytes | `s.bytesize` |
| TypeScript | UTF-16 code units | `s.length` |
| Swift | Character count | `s.count` |
| Kotlin | UTF-16 code units | `s.length` |

For ASCII text (all conformance test fixtures today), these are equivalent.

### Client Topology (§3)

| SDK | Structure |
|-----|----------|
| Go | `Client` → `AccountClient` → Services (two-tier) |
| Ruby | `Client` → `AccountClient` → Services (two-tier) |
| Kotlin | `Client` → `AccountClient` → Services (two-tier) |
| Swift | `Client` → `AccountClient` → Services (two-tier) |
| TypeScript | Flat — all services on a single `BasecampClient` object (valid language adaptation) |

### Service Coverage (§5)

| SDK | Account-scoped services |
|-----|------------------------|
| Swift | 46 (full canonical set) |
| TypeScript | 46 (full canonical set) |
| Kotlin | 46 public accessors backed by 46 generated service classes — 45 exposed directly; `todos` exposes a handwritten composite that subclasses the generated `TodosService` |
| Ruby | 46 (full canonical set) |
| Go | 44 as standalone accessors (folds `automation`; `clientVisibility` ops exist on `RecordingsService` rather than as a separate service). Hand-written service wrappers around generated OpenAPI client — not fully generated. |
| Python | 46 (full canonical set; sync + async) |

### Event Feed Connector Scenario Lane (§23)

Tier-2 scenarios drive the real transport adapter over an in-process loopback cable server
wherever a lightweight one exists, with only the Clock faked; divergences are compensated
by named tier-3 tests.

| SDK | Tier-2 scenario lane | Compensation / note |
|-----|----------------------|---------------------|
| Go | Real default transport against an in-process loopback ws server; fixtures consumed as data | Full-jitter formula additionally pinned exactly via an injected deterministic rand source (tier 3) |
| TypeScript | Real transport under MSW `ws` interception | Default transport must use the global `WebSocket` (Node ≥ 22), never the `ws` package. The global API exposes no read limit, so the `max_frame_bytes` cap is enforced at message receipt — before any parse, decode, or queueing — rather than during the read: the allocation itself cannot be pre-bounded on this lane (accepted, documented divergence); an injected `transport` MAY provide true bounded reads |
| Python | Real transport over an in-process `websockets` loopback | Connector transport ships under an optional `stream` extra |
| Ruby | Real transport over a `websocket-driver` loopback | `websocket-driver` becomes a runtime dependency |
| Kotlin | jvmTest-only: real ktor ws client against a test-scoped server (`MockEngine` cannot mock WebSockets) | State machine mirrored in commonTest tier 3 for the four acceptance scenarios |
| Swift | Fake-transport-driven scenarios (no in-process ws server without adding SwiftNIO) | macOS-gated tier-3 `URLSessionWebSocketTask` adapter contract test proves the real adapter honors the transport contract (verbatim frames, close mapping) |

Outside Go, jitter is asserted only as a `{min, max}` envelope — a degenerate RNG
(always-0 is legal full-jitter output) is caught only by Go's formula pin. Documented
divergence, accepted.
