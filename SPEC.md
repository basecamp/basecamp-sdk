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
| **AccountClient** | Account-scoped facade. Prepends `/{accountId}` to paths. Owns all `53` account-scoped services. <!-- @service-count --> |
| **Services** | One class per API resource group. Generated from OpenAPI tags. Methods map to operations. |
| **BaseService** | Abstract base for generated services. Provides request execution, error mapping, pagination following, hooks integration. |
| **HTTP Transport** | Executes HTTP requests. Applies auth headers, User-Agent, Content-Type. Implements retry, caching. |
| **Errors** | Structured error hierarchy. Maps HTTP statuses to typed error codes with exit codes. |
| **Security** | HTTPS enforcement, body size limits, message truncation, header redaction, same-origin validation, credential-bearing values never rendered. |

### Two-Tier Topology

```
Client
├── authorization (service — no account context)
└── forAccount(accountId) → AccountClient
    ├── projects (service)
    ├── todos (service)
    ├── ... (every other account-scoped service)
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

**Per-operation retry ceiling.** Each operation carries a per-op `retry.max` in behavior-model.json (205 ops at `3`, 45 at `2`). **TypeScript and Swift** drive their retry loops directly from this per-op value, which is unambiguous there because neither exposes a numeric client-wide cap — only an on/off (`enableRetry`). Generated Go, Python, Kotlin (`BasecampConfig.maxRetries`), and Ruby's governed GET path (`config.max_retries`) expose a numeric client cap *and* honor the per-op value as a **ceiling**: `effective_attempts = min(client_cap, op_max)`. The ceiling can only reduce attempts below the client cap, never raise them, so a client that lowered its cap (e.g. to `1` to disable retries) is still honored. In every SDK exposing a numeric cap the cap is floored at one attempt before the ceiling applies (`min(max(1, cap), op_max)`, an absent `op_max` meaning no ceiling), so a cap of `0` yields a single attempt rather than none whether or not the operation declares a retry block — Ruby's ungoverned path and hand-written Go's GET and download loops included. Because every op's `max` is ≤ the default cap of `3`, a default or raised client makes exactly the per-op number of attempts in every capped SDK — matching TS/Swift. Observable changes from the former client-wide behavior, by client configuration:

- **Default client (`max_retries = 3`):** only the **11 idempotent `max:2` operations** (account/gauge/preference writes plus two subscription-style POSTs: `UpdateAccountName`, `UpdateAccountLogo`, `RemoveAccountLogo`, `UpdateMyPreferences`, `DisableOutOfOffice`, `MarkAsRead`, `ToggleGauge`, `UpdateGaugeNeedle`, `DestroyGaugeNeedle`, `Subscribe`, `EnableCardColumnOnHold`) change — they now retry at most twice instead of three times. The other 197 retry-eligible ops are unaffected (`min(3, 3) = 3`).
- **Client that raised its cap above 3:** **all 208 retry-eligible operations** are now clamped to their per-op `max` (197 to `3`, 11 to `2`) instead of retrying up to the raised cap. This is the intended meaning of a per-op ceiling and brings Go/Python into line with TS/Swift/Kotlin, which never retry beyond the per-op `max`. Go, Python, Kotlin, and Ruby's governed path all equally honor a caller who wants *fewer* attempts than the operation declares.
- **Client that lowered its cap to `1`:** unchanged — the cap still wins (`min(cap, op_max) = cap`). A cap of `0` is coerced to one attempt on every path, governed or not (§2 validation algorithm step 4). Go, Python, and Ruby's governed GET path consume `max` **and** `retry_on` (the declared status gate); only the emitted `base_delay_ms`/`backoff` remain inert per-op metadata for them (retained for parity — see `scripts/check-retry-metadata-parity.py`). Ruby remains GET-only: mutations never retry there, so per-op metadata governs only its reads.

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
4. Validate `max_retries ≥ 0`. → `⊥ BasecampError(code: "usage")` otherwise. `max_retries` is total attempts including the initial request, so a **negative** cap is the configuration error; **`0` is legal and means "no retries — exactly one attempt"** `[conformance]`. Every SDK exposing a numeric cap floors it at one attempt on **every** path, governed or ungoverned: whether a request reaches the wire does not depend on whether the operation carries a declared retry block. A declared operation ceiling still clamps the floored cap downward (§2 per-operation retry ceiling).

   Read literally, "total attempts" makes `0` a request to send nothing, which is why this step said `⊥` until #718. That reading lost: `0` is the spelling users reach for to mean "don't retry", four of the five numeric implementations already accepted and floored it, and §14's attempt-budget table already blessed "a zero cap" as yielding exactly one hop-1 attempt — so the `⊥` contradicted this document rather than any SDK but one. The name is the thing that is wrong (it is inherited from the shipping Ruby SDK; see the naming note in §2), and the remedy is to document the name, not to reject the value.

   All five numeric implementations already agree on the boundary; only how they spell the rejection is idiomatic, and none of that changes. Generated Go returns a plain `error` from `WithRetryConfig`/`doWithRetry`; Python's `Config` raises `ValueError("max_retries must be non-negative")`; Ruby's `Config#validate!` raises `ArgumentError("max_retries must be non-negative")`, its `is_a?(Integer)` test also excluding `true`/`false`; Kotlin's builder uses `require(maxRetries >= 0)`; hand-written Go panics, as it does for every config failure (§3 step 5). That Ruby and Python had both settled on the words "must be non-negative" is the clearest evidence that `≥ 0` was the de-facto contract this step now states.

   **TypeScript and Swift expose no numeric cap at all**, only `enable_retry`, and that is deliberate rather than an omission. Their loops are driven by the per-operation `retry.max` ceiling (§2), so a client-wide number could only ever *lower* the budget, and the only lowering callers actually want is "off" — which `enable_retry: false` already spells, yielding the same one attempt this step licenses `0` to mean. Adding a numeric knob to two SDKs to express something they can already express would be new public API for no new capability. `max_retries: 0` and `enable_retry: false` are therefore the same contract in different spellings, and a conformance case pinning a zero cap maps to the latter in those two runners.
5. Validate `max_pages > 0`. → `⊥ BasecampError(code: "usage")` otherwise. Each SDK raises its own idiomatic configuration error rather than a literal `BasecampError`: Ruby `ArgumentError`, Python `ValueError`, Kotlin `IllegalArgumentException` (via `require`), TypeScript `BasecampError("usage")`. **Divergence:** Go and Swift are the two that are *not* recoverable, each for the same structural reason — the constructor that receives the cap has no way to report a failure. Go's `NewClient` has no error return, and panics `"basecamp: max pages must be positive"`; that is not special to this check, being how it reports every config failure (§3 step 5). Swift's `BasecampConfig.init` is public and non-throwing, so it uses `precondition`, which traps. The low-level generated Go client carries no `MaxPages` at all, so the hand-written `pkg/basecamp` client is the only Go path that validates a cap. Neither is a new failure mode, only an earlier and more legible one: Swift's `BaseService` pagination loops are `for _ in 1..<maxPages`, and Swift already trapped forming that range when `maxPages <= 0`, but only after page 1 had been fetched, and reporting a `Range` violation rather than a configuration mistake; Go's loops are bounded by `page <= MaxPages`, which a non-positive cap makes vacuous — the all-pages walk returned an empty result and the continuation walk returned only the page it had already been handed, each flagged by nothing louder than a `pagination capped` log line. **TypeScript is the only one that must also reject non-integers and unsafe integers**, because its `maxPages` is a `number` rather than an integer type. Its predicate is `Number.isSafeInteger(n) && n > 0`. `Infinity` makes the bound unreachable so pagination never stops; a fractional cap overruns by one page; and `Number.isInteger` is *not* sufficient, because it returns `true` for `Number.MAX_VALUE` and everything else above `2 ** 53`, where the page counter stops advancing — `page++` on `2 ** 53` yields `2 ** 53` again — so a bound like `2 ** 53 + 2` is never reached. `2 ** 53` itself does terminate, being arrived at from `2 ** 53 - 1`; rejecting it too is deliberately conservative, `MAX_SAFE_INTEGER` being the edge of the guarantee that the counter can reach the bound at all. **Python needs the same check for a different reason**: its `int` annotation is not enforced at runtime, so `Config(max_pages=float("inf"))` and `max_pages=2.5` are both accepted by a bare `<= 0` test, and `page < max_pages` then never terminates. It validates with `isinstance(self.max_pages, int)` alongside the sign, and excludes `bool` explicitly: `bool` subclasses `int`, so `max_pages=True` otherwise passes and yields a cap of `True`, which both `range(1, cap + 1)` and `page < cap` read as 1 — silently returning the first page as the whole collection. (`False` is refused either way, by being `0`.) Its `max_retries` check excludes `bool` for the same reason. Only Go, Kotlin and Swift get the integer guarantee from their compilers; Ruby buys it back with `is_a?(Integer)`, which also excludes `true`/`false`. In all six the check belongs at **every** construction path that accepts a cap, not only the client factory: TypeScript exports `BaseService` and its generated subclasses, so a directly-constructed service is a second door to the same loop. And construction-path validation holds only while the stored cap cannot be *replaced* afterward: Go, Kotlin, Swift and Python store it immutably (an unexported options copy, `val`, `let`, and a frozen dataclass respectively), TypeScript holds it in a native `#`-private field the pagination loops read directly — its compile-time `readonly` was assignable through a cast — and Ruby, whose `Config` stays deliberately mutable, validates in the `max_pages=` writer that `Http` re-reads at every page boundary. Those doors must further agree on what *absence* means, or they disagree about exactly one value. An absent cap is `undefined` **or** `null`, and both fall through to the default: TypeScript's client factory and `BaseService` each end in `maxPages ?? DEFAULT_MAX_PAGES`, so each guards `!= null` rather than `!== undefined`. A guard stricter than the `??` beside it would reject a value that the very same constructor goes on to treat as absent. The two standalone helpers, `fetchAllPages` and `paginateAll`, are the deliberate exception and validate unconditionally: their `maxPages: number = DEFAULT_MAX_PAGES` default *parameter* fires only on `undefined`, so an explicit `null` cannot reach a default there and must be rejected rather than silently become one.
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

### AccountClient-Level Services (account-scoped) — `53` services <!-- @service-count -->

<!-- @account-scoped-services:begin -->
account, attachments, automation, bookmarks, boosts, calendars, campfires, cardColumns, cardSteps, cardTables, cards, checkins, clientApprovals, clientCorrespondences, clientReplies, clientVisibility, cloudFiles, comments, documents, drafts, events, everything, folders, forwards, gauges, googleDocuments, hillCharts, lineup, messageBoards, messageTypes, messages, myAssignments, myNotes, myNotifications, people, projects, recordings, reports, schedules, search, subscriptions, templates, timeline, timesheets, todolistGroups, todolists, todos, todosets, tools, uploads, vaults, webhooks, wormholes
<!-- @account-scoped-services:end -->

**Total surface:** one client-level service (authorization) alongside the `53` account-scoped ones above. <!-- @service-count -->

That roster is the canonical surface, not a per-SDK inventory. Accessor counts vary by SDK with split and wiring decisions, and Appendix F tabulates each — that table, not this section, is where a given SDK's surface is stated.

### Derivation Rule `[static]`

The OpenAPI spec groups operations under coarse tags (e.g., `Automation`, `Todos`, `Files`). The service generators split those tags into the `53` fine-grained services above <!-- @service-count --> using a two-table mapping: `TAG_TO_SERVICE` (tag → default service name) and `SERVICE_SPLITS` (tag → {service → [operationIds]}). For example, the `Todos` tag splits into `Todos`, `Todolists`, `Todosets`, `TodolistGroups`, `HillCharts`; the `Files` tag splits into `Attachments`, `Uploads`, `Vaults`, `Documents`, `CloudFiles`, `GoogleDocuments`. Both examples are exhaustive on purpose: an abridged one is how `cloudFiles` and `googleDocuments` stayed invisible to this section for so long — a service that arrives through a split rather than a tag of its own is named nowhere a reader would look. These mappings are defined in each language's generator script. They are five hand-maintained copies of one table, and `make check-service-inventory-parity` compares what those copies **emitted** — the TypeScript, Ruby, Kotlin and Swift generated service directories, Python's generated `__init__.py` barrel, the two generated accessor files this section's roster is derived from, and Go's hand-written accessors — so identical service sets are enforced rather than merely expected. It reads what each generator already emitted rather than reimplementing the mappings, which is what keeps it from being a sixth copy — with Go the one exception, having no generated per-service files, so its hand-written accessors are compared against the others' generated output and carry the carve-outs noted below. (Python is read from its barrel rather than its directory. That began as a workaround: its generator, alone among the five, did not delete outputs a mapping stopped producing, so a directory listing counted the corpse as still emitted. The generator sweeps now (#757), which fixes it at the source. That sweep reads this same barrel — it is the generator's own record of what it last emitted, and each run deletes `that record minus its own output`, inspecting no file's contents; the two readers share a source and remain independent, since a sweep that stops working leaves the barrel correct and the corpse invisible to a barrel reader exactly as before. The barrel reading is kept for that reason and because it names exactly the modules the mapping produced, excluding the two hand-written base files without a drop-list.) Each per-SDK `check-*-service-drift` script remains the freshness gate for its own SDK; none of them can see another SDK, which is the axis this one adds. Go's three divergences (it folds `automation` and `clientVisibility` into other services and spells `timesheets` singular) are stated as data in that gate and fail it if they ever stop applying; Appendix F records them.

### Merge-Safe Write Surface (Cards)

BC3's JSON card update is **presence-aware** (basecamp/bc3#12521): an omitted key is left unchanged,
an explicit `""` or `null` clears. `card_update_params` forks on representation — JSON gets
`card_params` untouched, while the HTML/turbo_stream leg keeps `card_params.with_defaults(due_on: nil)`
because the web form omits `due_on` entirely when "No due date" is picked. API callers only ever see
the JSON leg.

This reversed a destructive default. BC3 used to build the params as `{ due_on: nil }.merge(card_params)`
for every representation, so **any** update whose body omitted `due_on` erased the card's due date, and
the composite had to defend against it with a read-modify-write. That defence is gone.

- **`update`** — a single PUT sending exactly the fields the caller addressed. No read-before-write, no
  race. `due_on` is tri-state: unaddressed omits the key (BC3 leaves the date alone), an explicit empty
  sends `"due_on": ""` (BC3 blank-casts to nil and clears), a date sets it.
- **`updateVerbatim`** — the raw single PUT. Since the server became presence-aware this is behaviourally
  identical to `update`; both names are retained because they are load-bearing in the generated surface.

Clearing is encoded as `"due_on": ""` — never as null (§18). The empty string is the only clear spelling
all six SDKs can express identically, since five of them strip nulls structurally before the wire, and
it is pinned by a BC3 server test so it cannot regress.

The composite deliberately does **not** resend anything the caller did not set. BC3 filters incoming
assignee IDs through `reachable_people`, so echoing assignees back would silently unassign anyone who
has since lost board access.

Presence detection is language-native: Go `*string` (nil omits, pointer-to-empty clears),
TypeScript `dueOn?: string | null`, Ruby/Python `nil`/`None` kwarg defaults with `""` to clear,
Kotlin nullable parameters with `""` to clear, Swift a `DueDate` enum (`.preserve`/`.clear`/`.on`)
because an optional cannot carry three states.

Card **steps** share the contract: `title` is optional on update, an omitted key is unchanged,
`"assignee_ids": []` removes everyone, and an assignee-only body is a valid partial update where it
used to 400. `UpdateStepRequest.DueOn` is presence-bearing for the same reason as the card's.

**Uploads** share it too, on both write paths. `Uploads::VersionsController#create` reads
`description` with `key?`, so an omitted key carries the previous version's description forward and
`""` clears; `UploadsController#update` reaches the same `serialize(:description, coder:
ActionText::Content)` attribute through `@upload.changing`, with no blank-cast in between. Both are
pinned by BC3 server tests (basecamp/bc3#12565), so `""` cannot regress to a no-op on either.
`CreateUploadVersionRequest.Description` and `UpdateUploadRequest.Description` are therefore
presence-bearing in every SDK — including Go, which uses `*string` here rather than the zero-value
guard described under Todolists below.

`base_name` is deliberately **not** presence-bearing on either: `Upload#base_name=` guards on
`new_base_name.present?`, so `""` and absent are the same server write and there is no third state
to model. Stating that is what keeps the asymmetry legible as a verified server fact rather than an
oversight.

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

  This still holds for `UpdateTodolistRequest` and `UpdateDocumentRequest`. It no
  longer holds for uploads: `UpdateUploadRequest.Description` and
  `CreateUploadVersionRequest.Description` are `*string`, following the
  gauge-needle precedent, so `Ptr("")` clears and `nil` leaves the field alone.
  `BaseName` stays a plain `string` on both — `Upload#base_name=` guards on
  `new_base_name.present?`, so `""` and absent are the same write server-side and
  there is no third state a pointer could express.

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

The endpoint is polymorphic, and more literally so than the name suggests: there is no separate group model or group write route at all. A "group" is just a `Todolist` whose parent is another `Todolist` (`Todolist.group?`), there is no `Todolist::Group` class, and the API-canonical routes mount `resources :groups, only: %i[ index create ]` — so *every* group write in the API namespace lands on `TodolistsController#update` through this same URI. BC3 renders both variants through `todolists/_todolist.json.jbuilder`, so a group carries `description` and `description_attachments` too and reports `"type": "Todolist"`, and the only structural discriminator is `groups_url` (list) XOR `group_position_url` (group). The composite is therefore deliberately **variant-agnostic**: it preserves `{name, description}` for a group exactly as for a list, with no type-sniffing.

**The spec now says the same thing (#544).** `Todolist`, `TodolistGroup` and the `TodolistOrGroup` union were three declared shapes for one wire body; they are one `Todolist` structure, returned by every operation that used to carry any of them — the polymorphic GET/PUT, the todoset-scoped list, and group-list/group-create/group-get. `description` and `description_attachments` are `@required` and never null (`format_api_content` funnels a blank rich text through `call_pipeline`, which returns `""`, and `rich_text&.downloadable_attachments.to_a` is `[]`), and the structural discriminator is modelled: `groups_url` XOR `group_position_url`, both optional, with `color` and `comments_app_url` alongside them. Those last two are **required** (#630) — `_todolist.json.jbuilder` calls `json.color` in both branches of its `todolist_group?` conditional and emits `comments_app_url` from a route helper, so neither key is ever absent. `color` is required-AND-nullable: `recordings.color` is a nullable column, so an uncolored list or group sends an explicit `null`. Smithy cannot express that natively on this shape — the member carries `@examples`, and an example cannot hold a `null` for a `String` — so both halves are layered onto the OpenAPI projection by `jsonAdd` (the `["string","null"]` union and an append to `Todolist.required`), the `SearchType.key` treatment. `comments_app_url` is never null and takes native `@required`. Read `groups_url`/`group_position_url` to tell the variants apart — never `type`, which reads `"Todolist"` for both. Conformance: `conformance/tests/todolists_read.json`.

**Go asymmetry.** Group-ness is service-static in Go: `TodolistGroupsService` has its own write path over the same `UpdateTodolistOrGroup` wire operation, where the other five SDKs expose no group update at all (their `TodolistGroups` split is List/Create/Reposition only). Go's group surface gets the raw method **renamed to `Replace`** — so the destructive path is honestly named — but deliberately gets **no merge-safe `update`/`edit`**. The original reason expired with #544: `TodolistGroup` used to model no `description`, so a composite reading through that projection would have PUT back a zero-valued description and erased it on every call. It is now a Go type alias for `Todolist` and carries the field, so that hazard is gone — and the composite still is not built, for a different and smaller reason: the other five SDKs ship no group write of any kind, and `todolists.Update` already addresses this exact route through the variant-agnostic projection, so a sixth spelling of the same composite would widen a cross-SDK asymmetry rather than close a gap. `TestTodolistGroupsService_ShipsNoMergeSafeComposite` pins that reason rather than the expired one, and `ReplaceTodolistGroupRequest` keeps its `description` field — now round-tripped, because the response projection carries it.

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

### Merge-Safe Write Surface (Schedule Entries)

`PUT /{accountId}/schedule_entries/{entryId}` is **full replace with declared carve-outs** (spec operation `ReplaceScheduleEntry`, `x-basecamp-write-semantics: {mode: "replace", clearsOmitted: true, preservedOnOmission: ["participant_ids", "url", "highlighted"]}`, mirrored into the `write` clause of `behavior-model.json`). BC3's `Schedules::EntriesController#update` runs `@recording.update! recording_attributes.merge(recordable: new_schedule_entry_for_update)`, building a *brand-new* `Schedule::Entry` from the permitted params and swapping the recordable wholesale.

This is the only composite whose server contract is not uniform across its writable fields, so the writable set splits in two:

| class | fields | on omission |
|---|---|---|
| **replaced** | `summary`, `description`, `all_day`, `starts_at`, `ends_at` | **cleared** |
| **carved out** | `participant_ids`, `url`, `highlighted` | **preserved** |

The carve-out is BC3-side: `PRESERVED_ON_OMISSION = %i[ url highlighted ]` seeds those two from the existing recordable when `params[:schedule_entry]` does not `key?` them, and `update_participants?` guards `participant_ids` separately. §18 states the bound that keeps this a `Replace*` rather than a merge, and the per-field justification.

**The composites must not resend the carve-outs.** This is the inverse of the Documents rule and the point of the surface:

- `url` is **identity-colliding**. On write it is the entry's join link; on read, `url` is the entry's own Basecamp API URL, because `recordings/_recording.json.jbuilder` writes that key before the entry partial renders. BC3 emits the join link as **`join_url`**. Echoing the response's `url` into the request's `url` writes the API URL into the join link.
- `highlighted` was accepted on write but never emitted before basecamp/bc3#12502, so no caller had a value to resend.
- `participant_ids` reads back as `participants` (objects, not IDs) and BC3 re-screens a submitted list through the bucket's reachable people, so a resent projection can silently drop a participant who has since become unreachable.

Resending any of the three would be redundant at best — BC3 already preserves them — and wrong if the read raced a concurrent change. So a carve-out reaches the wire **only when the caller addressed it**, and then it applies normally: `participant_ids: []` clears the participants, `url: ""` clears the join link, `highlighted: false` removes the highlight.

The three-method surface:

- **`replaceEntry`** — the generated wire method: verbatim single PUT, no GET. Renamed from `updateEntry` by renaming the wire operation (`UpdateScheduleEntry` → `ReplaceScheduleEntry`), the `ReplaceTodo`/`ReplaceDocument` route, and it ships **without a deprecated alias**.
- **`updateEntry`** — merge-safe. GET → resend the five replaced fields from the read-back, overlay the caller's explicitly-set values, add any caller-addressed carve-out → PUT. Set-detection is language-native, and a passed `false`/`""`/`[]` on a carve-out **is** a set.
- **`editEntry`** — read-modify-write closure over the same two hops. The five replaced fields are always sent. The carve-outs are seeded for *reading* (`url` from the response's `join_url`, `participant_ids` projected from `participants`, `highlighted` from `highlighted`) but reach the wire only when the setter was invoked — **setter-invocation dirty tracking, not value comparison**. Assigning the value the read already returned still sends it; snapshot/diff is explicitly rejected, because value comparison cannot express intent.

**Read-side requiredness is a separate fact from request optionality**, as for Documents:

- `summary` is optional on the request but `@required` on the response — `Schedule::Entry#summary` is `super.presence || "Untitled"`, so a healthy server can never render it blank, and an absent/null/blank `summary` in a 2xx body is a malformed response. Coalescing it to `""` and PUTting that back would blank a real summary on a call that only moved the entry.
- `all_day` is `@required` on the response (`schedule_entries.all_day` is NOT NULL, default `false`) and it is **not** carved out, so it must be resent. Its guard is the one that cannot be written as a truthiness test: the value it most needs to admit is `false`. Defaulting a missing `all_day` to `false` converts an all-day entry into a midnight-to-midnight timed one.
- `starts_at`/`ends_at` are `@required` on both sides — `Schedule::Entry` presence-validates both and `Recording` validates the associated recordable on update, so omitting either is a `422` rather than a clear. Their wire value is a bare **date** (`2016-06-01`) for an all-day entry and a full timestamp otherwise, because BC3 renders `starts_at_date_or_time`. `ISO8601Timestamp` is a plain string in this model so both shapes decode; the composites round-trip the value verbatim rather than parsing and re-rendering it. **`CreateScheduleEntry` takes the same two forms** — `base_schedule_entry_params` is the one permit list behind both `new_schedule_entry_params` and `update_schedule_entry_params` — so an SDK that narrows the create bound to a timestamp makes an all-day entry reachable by replace and unreachable by create (#634). Modelling it as a date-time type is wrong in both directions: it cannot parse the bare date, and it re-renders whatever it did parse.
- `join_url` and `highlighted` are optional on the response, and as of the `UpcomingScheduleEntry` carve-out they are **under**-modelled rather than correctly modelled. Both are emitted unconditionally by `api/schedules/entries/_entry.json.jbuilder`, which every operation still returning `ScheduleEntry` renders. They were optional because the reduced `api/schedules/calendar/_entry.json.jbuilder` reached the same shape through `GetUpcomingSchedule`; that report now has its own projection, so the reason is retired. Tightening them (`join_url` required-and-nullable, `highlighted` plain required) is a separate contract change across five operations and every inline stub, deliberately not ridden along.

**Recurring entries are unreachable on this route.** `ensure_non_recurring_event` 302-redirects both `show` and `update` to the entry's occurrence, so `ReplaceScheduleEntry` and its composites serve non-recurring entries only; read a recurring entry through `GetScheduleEntryOccurrence`. `time_zone_name`, `recurs_until` and `recurrence_schedule` are deliberately **not modeled** — BC3 forces `time_zone_name` to nil for any entry that is not both recurring and timed, and recurrence absorption is tracked separately.

**Hook contract:** update/edit compose the public `getEntry` + `replaceEntry`, so hooks observe the wire operations under each SDK's native identities — never a synthetic composite.

**Race:** update/edit are read-modify-write, not atomic. A concurrent write between the GET and PUT is overwritten — last write wins for the whole representation, window of one round-trip. Use `replaceEntry` to overwrite deliberately.

Conformance: `conformance/tests/schedule_entries_write.json`, 11 cases. Nine cover this surface: `replace-omission-clears`, `replace-clears-carve-outs`, `replace-single-request`, `update-merge`, `update-addresses-carve-outs`, `update-clears-carve-outs`, `edit-clear`, `edit-untouched-carve-outs`, `edit-touched-carve-outs`. The other two, `create-join-link` and `create-omits-unset`, do not: `CreateScheduleEntry` is a plain wire write with no read-back and nothing to preserve, and the pair pins that `url`, `highlighted` and `status` (#641) reach the wire when the caller sets them and stay off it when they are unset — the same absent-versus-zero-value distinction as the carve-outs, one operation earlier. They share the file because they share the fixture's entry shapes, not because create is merge-safe.

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

**Go divergence:** Go exposes a `Cause` field (the underlying error) not present in this canonical RECORD — a language-specific extension. `retry_after` is no longer a divergence: Go's `Error` carries it, populated at both 429 construction sites, and the raw GET retry loop sleeps it in place of the backoff curve. `RequestResult.retry_after` is unchanged and remains the hook-facing copy rather than the only one.

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
| `limit_exceeded` | 10 | 507 | false | An account limit blocks the request (file storage, webhooks) | `[conformance]` |

### HTTP Status Mapping Algorithm

Each explicitly enumerated status mapping below (steps 1–11) is `[conformance]`-verified. The two catch-all fallback steps (12: general 5xx; 13: any other non-mapped status) have no dedicated conformance case and are `[static]`.

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
11. If `status == 507` → `BasecampError(code: "limit_exceeded", http_status: 507, retryable: false)`.
12. If `status >= 500` → `BasecampError(code: "api_error", http_status: status, retryable: true)`. `[static]`
13. Otherwise → `BasecampError(code: "api_error", http_status: status, retryable: false)`. `[static]`

Step 11 must precede the 5xx catch-all. A 507 is a *server* status carrying a *client* fact: the account is out of storage, or at its webhook ceiling. Retrying cannot satisfy it, so classifying it by its 5xx range alone would report a plan limit as a transient server error — indistinguishable, to a caller deciding whether to back off, from a 500. Ordering is what makes the distinction, since both steps match.

In all cases, extract `request_id` from `X-Request-Id` response header if present. `[conformance]`

### Statusless `api_error` for a malformed 2xx body `[manual]`

The mapping above is keyed on an HTTP status, because it maps *failed* responses. A composite (§18) can also fail on a **successful** one: the transport returned 2xx, and the body is malformed in a way that makes the composite's next step unsafe — a writable field of the wrong type, or a required field absent, on a read the composite is about to echo back into a full-replace write.

That error is `api_error` with **no `http_status`** and **`retryable: false`**. Statusless because no status describes it (the request succeeded), and non-retryable because re-requesting cannot repair a malformed body. It is deliberately *not* `usage`/`validation`: the value came off the wire, so nothing the caller passed is at fault. The mirror case — the *caller* supplying the offending value — stays a usage error. **Classification is by origin, not by value:** the same empty string is a caller error when the caller passed it and a malformed response when the server did, so each provenance is checked where it is unambiguous (the read step owns the response, the write step owns the caller).

Message is truncated to `MAX_ERROR_MESSAGE_LENGTH` like any other (§9) — the malformed value is embedded in it, so the cap is load-bearing rather than cosmetic.

The composites are where this shape is *required*, not where it is *bounded*. Kotlin and Swift decode into typed models, so their decoder refuses a malformed body, and each **request primitive** maps that failure to this same shape rather than leaking `SerializationException`/`DecodingError` (#604). The mapping is scoped to the decode expression alone: an auth-phase throw, a transport failure and a *request-body* encoding failure are not malformed responses and keep their own classification — in Kotlin the request body is serialized inside the same `try` and raises the identical exception type, so the distinction is positional, not type-based.

A **wrapped-pagination** response is decoded in two halves — the items array on every page, and the first page's remaining members — and both are the primitive's decode, so an absent or wrong-typed member of the envelope is a malformed body and not an empty result. Absence is malformed because BC3 writes these envelopes unconditionally; the only such operation is `GetPersonProgress`, whose two members are two bare lines of `app/views/api/users/timelines/show.json.jbuilder` (#728).

### Error Body Parsing Algorithm

1. Attempt to parse `body` as JSON.
2. If JSON and has `"error"` key (string value) → use as `message`.
3. If JSON and has `"error_description"` key (string value) → use as `hint`.
4. Else if JSON and has `"message"` key (string value) and `message` not yet set → use as `message`.
5. If parsing fails or body is empty → `message` is the fixed phrase carrying the status code: `Request failed (HTTP 500)`.
6. Truncate `message` and `hint` to `MAX_ERROR_MESSAGE_LENGTH` (see §9) — `hint` is default-rendered too.

Note: `"error"` takes precedence over `"message"` — step 4 is a fallback for APIs that use `"message"` instead of `"error"`.

Step 5 used to say "HTTP status text", which is not one thing: the wire reason phrase is whatever the server sent under HTTP/1.1 and does not exist under HTTP/2, and a platform's status-code table is empty for an unregistered code (Go's `http.StatusText(599)` is `""`) and localized on Apple platforms. The fixed phrase is the only portable spelling. Ruby, Python and Go's raw client path always rendered it; #837 brought TypeScript (`response.statusText`), Kotlin (`status.description`), Go's `checkResponse` (`resp.Status`) and Swift (`localizedString(forStatusCode:)`) to it.

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
2. Attempt parse as HTTP-date (RFC 7231, e.g., `Wed, 09 Jun 2021 10:18:14 GMT`). If valid → compute `max(0, date - now())` in seconds, **rounding a sub-second remainder up**; if > 0 → return.
3. → `undefined` (fall through to backoff formula).

Step 2's rounding is up, not truncating, for two reasons: a positive remainder must never round to
zero, because zero is read as "no usable value" and drops the request onto the local backoff curve —
the opposite of what the header said; and rounding down retries up to a second *before* the moment
the server named, which is the one thing a date is unambiguous about. TypeScript, Kotlin, Swift and
Go round up. Python and Ruby still truncate, tracked in #799.

This algorithm defines **parsing** only — how a header value becomes a number of seconds. Which
statuses honour the result, and what bounds the sleep it buys, is the next section; do not read a
status set into the steps above.

### Retry-After Honouring `[CONFLICT]`

**A parsed `Retry-After` is honoured at any status a retry is already going to happen at.** There is
no status gate of its own, and this is a property of retrying rather than of one loop: §7's three
gates decide whether *this* response is retried on the generated-operation path, and §14's hop-1
policy decides it for `DownloadURL`. Wherever the deciding rule says yes, the parsed value governs the
wait — at 429, at 503, and at every status a declared retry set carries today or grows to carry later.
Where it says no, the value may still be parsed and surfaced on the error for the caller to read, but
nothing sleeps on it.

That is a rule about **which statuses**, and it is the only thing this section decides for every loop
at once. *How* the value composes with whatever delay the loop would otherwise have computed is a
separate question with a separate answer per loop, settled under "Composition is per-loop" below.

#### What "retry" means here

The rule above turns entirely on that word, and leaving it to intuition cost three review rounds — each
one finding another repeating loop the rule appeared to reach and the code deliberately did not. The
answer is not a longer list of loops. It is that **a loop which repeats a request is not thereby
retrying one.**

> **A retry is the re-issue of a request whose previous attempt produced no answer** — the transport
> failed, or the origin declined to serve it with a status the loop **declares retryable**. Re-issuing
> a request because the answer was *"not yet"* is a **poll**, and this section does not reach it.

Both halves carry weight, and the second is not decoration. §4's 401 refresh-and-retry re-issues after
the origin declined to serve — but 401 is on §7's explicit never-retry list, so no loop declares it
retryable, and §4 is outside. §23's below-threshold authorization recovery is the same shape one level
up, and is outside on the same clause: an unauthorized mint, disconnect or poll rides the reconnect
cycle to a fresh mint, but the seam classifies 401/403 as `unauthorized` — a kind that carries no
`retry_after` by construction, distinct from the `transient`/`throttled` kinds the `backoff` row
declares — and the cycle is bounded by the shared authorization counter, not a retry budget. Nothing
for the header to reach, and no declared retryable status. §7's `retry_on`, §14's hop-1
`{429, 502, 503, 504}`, §23's transient/throttled error kinds and §16's `429`-plus-`too_many_requests`
pair are all declared sets, so all four are inside.

For §23 the *kind* is what carries the header across the seam, so the adapter's mapping is fixed
here rather than left to each implementation: a retryable outcome exhausted inside the seam whose
last response carried a parsed `Retry-After` maps to `throttled(retry_after)` **whatever its
status**, and one without maps to `transient`. A mapping keyed on status instead — 429 to
`throttled`, 503 to `transient` — would honour the header at one retryable status and drop it at
another, which is exactly the gate this section removes.

There is a second, independent reason the boundary falls here, and it is worth stating because it
shows the definition is not merely stipulated. This section governs the relationship between a
server-directed delay and a **locally computed backoff** — what may replace it, floor it, cap it. A
completion poll has no backoff to relate to. It waits a cadence the *same server* already prescribed
(§16's `interval`, raised by each `slow_down`), so there is nothing for `Retry-After` to displace and
no question for this section to answer. Where the rule has no subject, it does not apply.

**The definition replaces the enumeration, so a loop is in or out by the criterion** rather than by
someone noticing it. Every delay-bearing branch in this document, walked back through it:

| Loop / branch | Repeat is driven by | Verdict |
|---|---|---|
| §7 generated-operation retry | declared `retry_on` `{429, 503}`, or a network error | **in** |
| §14 `DownloadURL` hop 1 | declared `{429, 502, 503, 504}`, or a network error | **in** |
| §16 poll — `authorization_pending`, `slow_down` | a 4xx protocol answer meaning "not yet" | **out** |
| §16 poll — `429` + `too_many_requests` | the one pair §16 declares retryable; the origin refused to serve | **in** |
| §16 poll — connection timeout | a network error | **in** |
| §16 poll — `429` alone, or `too_many_requests` off 429 | nothing; terminal `api_error` | **out** |
| §23 reconnect `backoff` | mint/connect outcomes classified transient or throttled | **in** |
| §23 `poll-retry` | poll outcomes classified transient or throttled | **in** |
| §23 `backoff` after an unauthorized mint, disconnect or poll (below threshold) | 401/403, classified `unauthorized` — a kind carrying no `retry_after`; authorization recovery bounded by the shared counter | **out** |
| §23 `repair-poll` | a schedule — 60s ± 20% per cycle, no failure involved | **out** |
| §23 `staleness`, `handshake-deadline`, `confirmation-deadline` | elapsed time; deadlines, not repeats | **out** |
| §4 401 refresh-and-retry | a status no loop declares retryable, gated on a token refresh | **out** |

Those verdicts are the contract — what the criterion decides, independent of what any SDK does today.
Nothing above is a carve-out: `repair-poll` and the §16 pending branch are outside for the same reason
as each other — they repeat without a failed attempt — and §4 and §23's authorization recovery are
outside for the second clause alone, which is what shows that clause is load-bearing rather than
restating the first.

*(As-of check, not a contract: the §7, §14, §16 and §4 rows were verified against shipped code —
`wt/lane-spec` @ `fc5645dfe`, §16 read in four SDKs — and every one agreed with its verdict. The §23
rows could only be checked against §23's written text, because no event-feed connector ships in any
SDK yet; they are the weaker evidence and should be re-checked, not assumed, once it does.)*

§8's auto-pagination is the case that makes the distinction obvious, which is why it is worth naming
even though it takes no delay and so has no row: it issues request after request, and not one of them
is a retry — each asks for a *different* page and each previous one answered. It is also the cleanest
illustration of how the two layers nest, because an individual request inside that loop **is** under
§7, and is retried, and honours `Retry-After` accordingly. "Repeats requests" and "retries a request"
are properties of different things, and conflating them is the specific mistake this definition exists
to prevent.

Verified across four SDKs rather than read off the spec: Go, Kotlin, TypeScript and Python all
structure §16's poll the same way, and Go's own field comment states the boundary outright — the raw
header is *"consumed by the loop's 429 `too_many_requests` handling only"*.

RFC 9110 §10.2.3 defines `Retry-After` as a general response header field restricted to no status
set, and attaches explicit semantics to two cases: **503**, where the value is how long the service
expects to be unavailable, and **3xx**, where it is the minimum wait before issuing the redirected
request. Of the statuses this SDK's retry sets carry, 503 is therefore the only one the RFC speaks to
directly — a redirect is followed or refused (§8, §14 hop 1) and never retried, so the 3xx case never
reaches this rule. RFC 6585 §4 says a **429** *MAY* carry it. So among retried statuses 429 is the
permitted use and 503 the canonical one, which makes a 429-only rule narrowest exactly where the RFCs
are most specific. Deriving the answer from retry eligibility rather than from a status list is also
the only position that needs no amendment when a retry set grows: a status worth retrying is a status
whose `Retry-After` was worth reading.

It is also what §7's Retry Algorithm has always spelled. Step 3h reads "if `last_response` exists and
has a valid `Retry-After` header" with no status test of its own, and it is reachable only for a
response that already passed step 3f's `status IN retry_config.retry_on`. The status gates the five
SDKs grew are narrower than the algorithm they implement; what was missing was this section saying so.

**The value is honoured as given.** It is exempt from §7's backoff ceiling (§7 "Backoff Ceiling",
requirement 4): the ceiling governs the locally computed formula, and a client that silently caps a
server's instruction retries sooner than the origin asked for. **Nothing is added to it either** — a
jitter term is part of the locally computed formula, where it exists to decorrelate clients choosing
the same delay independently; a delay the origin named is already the origin's choice, so adding to
it makes the client wait longer than it was told for no benefit. An implementation MAY have a **host
limit** — a timer that cannot schedule the value, a conversion that would trap or wrap — and where a
parsed value meets one it MUST bound the value there: saturate, per the second representability tier
below, never let the conversion trap or wrap, and never fall back to the local backoff. A bound of
that kind belongs at the sleep, so wherever the error carries `retry_after` the caller reads what the
server said, never the clamped copy. TypeScript's `Math.min(seconds × 1000,
MAX_TIMEOUT_MS)`, applied in both of its retry loops as the delay is computed, is the worked example:
the parser's result stays the public `retryAfter`, and only what reaches the timer is clamped.

That is a guarantee about the field's *integrity*, not its *presence*. The Status Mapping Algorithm
above populates `retry_after` in its 429 arm only, so today an exhausted 503 that was slept on for
the value the origin named surfaces no `retry_after` to the caller, and neither does the error §7 step
3i hands to `on_retry`. Whether the mapping grows the field at every status a declared retry set
carries is part of the status convergence in #775, not decided here — for the SDKs whose delay loop
reads the value off the constructed error, it is the same change as the status gate, because one
parse feeds both.

Read that paragraph narrowly: it says what may not be done *to* the value — not capped, not summed
with a jitter term. It does not say what the value is combined *with*, which is the next paragraph's
subject and is not the same question. `max(interval, retryAfter)` neither caps the value nor adds to
it.

**Composition is per-loop, and every loop states its own.** Honouring settles that the parsed value
governs the wait. Whether it *replaces* the delay the loop would otherwise have computed, *floors*
it, or is combined with it some other way is not one answer, because the delay loops in this document
are not one mechanism: two schedule a retry of the same request, one paces a poll the server is
throttling against a fixed code lifetime, and one selects a gap between whole reconnect cycles. A
single composition asserted here would have silently overridden three of the five rows below.

So the obligation runs the other way. **Each loop the definition above puts *inside* — one whose wait a
`Retry-After` can reach — states its own composition in its own section, and such a loop added to this
document later MUST state its own rather than inherit one from here.** There is deliberately no
default to fall back on: an in-scope loop that says nothing is under-specified, not governed by §7's
answer. A loop the definition puts outside — `repair-poll`, the §16 pending branch, the deadline
timers — has no composition to state, because no header reaches it, and is not made under-specified
by this rule. The rows that exist today:

| Loop | Composition with the locally computed delay | Stated in |
|---|---|---|
| §7 generated-operation retry | **replaces** it — step 3h computes `parsed × 1000` *instead of* the backoff formula, never a sum | §7 "Retry Algorithm" step 3h, "Backoff Formula" |
| §14 `DownloadURL` hop 1 | **replaces** it, at every status in that hop's own `{429, 502, 503, 504}` set | §14 "Hop-1 Retry" |
| §16 device-flow poll | **`max` with the current `interval`** — a one-shot `nextWaitOverride = max(interval, retryAfter)`, still clamped to the remaining code lifetime by the wait rule, then decayed. Reads the **delta-seconds form only** — a declared exception to the Parsing Algorithm, reasoned below | §16 "RFC 8628 Device Authorization Grant" |
| §23 reconnect `backoff` timer | **floors** the full-jitter draw, and wins outright when it exceeds the cap | §23 "Clock, Timers, and Virtual Time" |
| §23 `poll-retry` timer | **replaces** the full-jitter draw — waited exactly | §23, same table |

One row also departs from the Parsing Algorithm, and says so rather than leaving two prescriptions for
one response. **§16's branch accepts delta-seconds and nothing else; an HTTP-date there is malformed
and falls back to the current `interval`.** That is an exception, declared here, for a reason that is a
property of that loop rather than of the header: the poll measures every wait on an injectable
**monotonic** clock against a deadline fixed at code issuance (§16 `pollDeviceToken` step 2), and an
HTTP-date is resolvable only against wall-clock `now()`, which that loop deliberately never reads — a
wall-clock comparison is the one thing a monotonic deadline exists to exclude. The cost of the
exception is bounded by the loop's own shape: the fallback is the cadence the same authorization
server prescribed, not a locally computed backoff, and the wait rule clamps everything to the
remaining code lifetime regardless. §16's block already pins the shape — digits-only, a shared
significant-digit bound, clamped to the device ceiling; this paragraph is what makes it an exception
instead of a contradiction.

§23 is two rows because its two timers genuinely differ, and that is the sharpest evidence that one
rule would not have fitted. A 1-second header against a 50-second selected reconnect delay waits 50
seconds: that timer's job is to space whole cycles apart after repeated failure, and a server naming
one second has said nothing about whether the condition that failed the last four cycles has cleared.
The same header on a `poll-retry` waits exactly one second, because there the server is pacing
precisely the request that is about to be re-sent. Both are `Retry-After` honoured at a status that
was going to be retried anyway; only the composition differs, which is the whole point.

The five rows do not conflict with each other — they are five loops, not five answers to one
question — and this paragraph converges nothing: each section keeps the behaviour it already has, and
what changes is that it now states it on purpose rather than by omission. One *implementation*
divergence sits on this axis and is already recorded below: the generated Go client sleeps
`retryDelay + rand(0, 100ms)` on the §7 row, which is the added jitter the paragraph above forbids,
and it is tracked with the rest in #775.

**Representability is not a policy cap, and it is exempt from the "never in the parser" rule.** A
policy cap answers "how long *should* a caller wait"; representability answers "can this host express
the value at all", and where the answer is no there is nothing to preserve. Two tiers, in the shape
§16's device-flow parser already settled — its second tier bounds against a domain ceiling (the
remaining device-code lifetime) rather than a host one, but the fall-back-versus-clamp split is the
same and is adopted here:

- **Unrepresentable in the parser's own numeric type → malformed.** It falls out at step 3 of the
  Parsing Algorithm to the local backoff, exactly like a fractional or non-positive value. Nothing is
  surfaced on the error, because nothing was parsed.
- **Representable by the parser but beyond what the host can schedule → saturate**, never fall back.
  Falling back would replace a server's request for a long wait with the millisecond backoff curve
  and hammer a peer that just asked to be left alone — the same tight loop by another route. Where
  the overflowing conversion is the one *feeding* the parser's own output type, the saturation may
  sit in the parser, and the caller then reads the saturated value; that is accepted, and it is the
  one carve-out from the paragraph above.

The host whose limit binds is the one the build runs on, and an SDK that ships to more than one
build target MAY pin the second-tier ceiling at the smallest value every supported target can
represent and schedule, so the delay honoured does not vary by build. That is still a
representability bound, not a policy cap: it is derived from a host limit — the narrowest one the
SDK ships to — and answers "can every host express this value" rather than "how long should a caller
wait". It clamps nothing the narrowest build could have honoured, and what it costs the wider builds
is only the waits the narrowest one could never have scheduled.

The width itself is deliberately not fixed here, because it is a property of the host. *(As-of
observation, verified against `wt/lane-spec` @ `fc5645dfe` — kept because the decision not to fix a
width is unreadable without it, not as a live claim: TypeScript rejects above
`Number.MAX_SAFE_INTEGER`, Kotlin above `Int.MAX_VALUE` on the delta-seconds form — its date form
computes in `Long` and saturates to `Int.MAX_VALUE` instead, which is the parser-output carve-out
above rather than a rejection — Swift above its 64-bit `Int`, and Go above native `int`: both its
hand-written and its generated parser are `strconv.Atoi` at that revision, so the width is 32 bits on
the 32-bit targets this repository keeps viable and 64 elsewhere. #796 has since moved the
hand-written parser to `ParseInt` into `int64`; the generated one stays `Atoi` until #798.)*
Four hosts reject cleanly at thresholds differing by nine orders of magnitude without any of them
misbehaving, which is the evidence that the width does not need fixing — a `Retry-After` naming a wait
longer than the host can count is not a delay any caller is worse off for missing.

`[CONFLICT: Ruby and Python have no such width, and that is a defect rather than a third position —
but it is the **second** tier they owe, not the first. `Integer` and `int` are arbitrary-precision, so
the parse cannot fail on magnitude and the first tier has nothing to fire on; the failure is one layer
down, at the scheduler, where the sleep raises rather than saturating. Reading it as the first tier
would oblige an implementer to invent a parser limit this section otherwise forbids. Both owe
saturation at whatever their own sleep can schedule, applied at the sleep — which for Python is two
different ceilings for the sync and async clients, since the binding host limit differs. Exact
ceilings, exception types and call sites in #775.]`

Go is the worked example of the two tiers meeting in one parser, and the two numbers govern different
questions: a digit string too large for the parser's own `int64` is **malformed** and falls through to
the backoff (first tier), while a value it holds but the host cannot schedule **saturates** (second
tier) at the pinned portable ceiling the tier permits — `math.MaxInt32` seconds, the smallest value
every supported `GOARCH` can represent, and the same 2,147,483,647 §16 already names as a shared
cross-SDK ceiling. Pinning matters because two host limits sit above a Go `Retry-After` — native
`int`, which the public `Error.RetryAfter` field is and which is 32 bits wide on the 32-bit targets
this repository keeps viable, and `time.Duration` — and a ceiling derived from only the larger would
change with `GOARCH`. That is what #796 ships: `ParseInt` into `int64`, over-range malformed, and a
clamp at `math.MaxInt32` inside the shared hand-written `parseRetryAfter` that the raw retry loop,
the download path and the hook result all read.

The identical unclamped conversion in `go/pkg/generated/client.gen.go`, and an `Atoi` there whose
range error is discarded into a rate-limit hint, are **not yet fixed anywhere**. Their fix *belongs*
in `go/templates/client.tmpl` — the generated file is emitted from it and must never be edited
directly — and #798 is where the work is tracked, not where it has landed.

**The exemption is conditioned on the escape, not on a number.** There is deliberately no policy cap;
in its place, **an honoured `Retry-After` delay MUST be awaited through the platform's cancellation
primitive, with the caller's cancellation handle threaded into the sleep.** A cancellation primitive
is the better control precisely because it is not a cap: it bounds the caller's *total* wait,
including the network time no ceiling on a header value can reach, and it lets the caller choose that
bound instead of the SDK guessing one on their behalf. Where such a primitive is threaded through, a
policy cap is redundant. Where none is, the caller holds nothing, and a server-directed sleep that is
merely long becomes a sleep that cannot be abandoned.

**What the requirement rules out, so a remedy is not chosen wrongly.** An interruptible incremental
sleep polling a cancellation flag satisfies it. A bound against a caller-supplied **total-time budget
does not** — a numeric deadline fixed before the call cannot be acted on during it, so a caller who
changes their mind still waits it out, and bounding the worst case that way is a policy cap under
another name. Which satisfying shape each language picks carries a caller-facing API dimension and is
not settled here.

`[CONFLICT: the cost of this position is not uniform — four of the SDK sleep paths give the caller no
handle at all today, and all four already carry the exposure independently of this decision: each
honours Retry-After on its 429 path now, so the un-abandonable server-directed sleep predates the
status rule, and widening the status set widens it rather than introducing it. Per-path inventory
and remedies in #775.]`

Strictly, none of those four is *uninterruptible*: a signal on the main thread, or `Thread#raise`
from another, will break any of them. What they lack is a cancellation handle the caller can **hold**,
and the requirement above is about the handle, not about whether the platform can ever intervene.

`[CONFLICT: five of the six SDKs gate honouring on a narrower status set than this section
prescribes, in three different shapes, and two of this section's other clauses are also divergent —
one policy cap and one added jitter term. Converging is a behaviour change across five SDKs. The
per-SDK inventory is deliberately NOT restated here: it states current behaviour, the convergence
work below changes the very rows it would state, and no gate can catch it going stale. It lives in
#775, verified as of that issue's dated comment.]`

Which **date forms** step 2 accepts diverges on a second axis, and the inventory is over *parsers*
rather than one per SDK — Go has two and they do not agree, the generated one
accepting no date form at all, so **every** HTTP-date falls through to local backoff on **every**
generated Go wire operation. That matters to the contract in one way only, and it is the reason this
sentence stays: convergence scoped by SDK name would fix the hand-written parser and leave the
generated surface untouched, so the fix site is `go/templates/client.tmpl`, never a file under
`go/pkg/generated/`. `[CONFLICT: per-parser inventory in #775; the template change rides with #798,
which owns the other two `Retry-After` defects at those same two lines.]`

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
- **Ruby** is stricter: only GET retries; all non-GET methods do not retry. Governed GETs (those carrying their canonical operation ID) are bounded by the per-op ceiling and status-gated on the declared `retryOn`; ungoverned GETs (`get_absolute`, OAuth discovery) keep the taxonomy-driven pre-metadata status contract, under the same floored caller cap. Ruby is acceptably conservative.
- **Swift** implements the three-gate algorithm: the transport retries only when the method is naturally idempotent (GET/HEAD/PUT/DELETE) **or** the operation is marked `idempotent: true`, so non-idempotent POSTs like `CreateProject` are attempted exactly once while the eight idempotent POSTs (`CompleteTodo`, `CreateBookmark`, `EnableCardColumnOnHold`, `PauseQuestion`, `PrioritizeAssignment`, `SpotlightRecording`, `Subscribe`, `SubscribeToCardColumn`) keep retrying. The gate covers both retry paths — HTTP status (`429`/`503`) and network errors — so Swift retries network errors but only for retry-eligible operations. A network error is classified by *meaning*, not by type: a `Transport` that reports connectivity failure as the SDK's own `BasecampError.network` reaches the retry branch exactly as a raw `URLError` does (#567). `Transport` is `public`, so that normalization is the natural implementation and must not be the one that disables retry. Any other `BasecampError` out of the transport (`.auth`, `.usage`, `.api`, …) stays terminal on sight, and the non-HTTP-response guard raises a distinct internal error so a deterministic programming fault is never mistaken for a transport blip. `BaseService` threads the per-operation flag from generated `Metadata` into the transport; the naturally-idempotent method set is allowlisted so PATCH/OPTIONS and future methods stay fail-closed.
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

Retry-After header value takes precedence when present and valid, at every status this loop reaches
(§6 "Retry-After Honouring").

**Composition (§6 "Composition is per-loop"): a valid `Retry-After` REPLACES this formula.** Step 3h
computes one branch or the other and never a sum, so neither the `2^(retry_index)` term nor the
`random(0, max_jitter)` term is present in a server-directed delay. This loop's answer is stated here
because §6 deliberately supplies no default: a delay loop that does not state its composition is
under-specified rather than governed by this one.

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
4. **`Retry-After` is exempt.** It is server-directed and takes precedence per step 3h,
   at every status that step reaches (§6 "Retry-After Honouring"); the ceiling governs
   the locally-computed formula only, and step 2's jitter is part of that formula rather
   than an addend on a server-directed delay. Implementations may still bound it against
   **host limits** — a timer that cannot schedule the value, such as TypeScript's clamp to
   the 2,147,483,647ms `setTimeout` accepts, or a conversion that would trap or wrap, such
   as the seconds→`time.Duration` saturation Go takes (#796) — and may reject outright a
   value the parser's own numeric type cannot hold. §6 "Retry-After Honouring" governs
   which of those belongs at the sleep and which may sit in the parser. A **policy** cap is a
   different thing and is not permitted: Swift's 86,400s clamp is one (the `UInt64`
   nanosecond trap it cites sits five orders of magnitude higher), and §6 records it as a
   conflict alongside the status divergence. The exemption is not unconditional: §6
   requires the honoured delay to be awaited through the caller's cancellation primitive,
   and that requirement — not a number — is what stands in for a policy cap here.

**Reachability.** Every SDK exposes a path to a high attempt count: Kotlin's builder
validates `maxRetries >= 0` with no upper bound, Go's `WithMaxRetries` only rejects
`n < 0`, and Python/Ruby take a caller cap that is intersected with — never raised
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

All `254` operations in `behavior-model.json` use `retry_on: [429, 503]`. <!-- @operation-count --> Three `(max, base_delay_ms)` patterns exist:
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
        - url ← extractAngleBracketed(part)
        - If url is not null → return url.
        - Otherwise CONTINUE to the next part. A part that says rel="next" but
          carries no extractable URL must NOT suppress a well-formed part after
          it: the return value feeds `truncated`/`hasMore`, so short-circuiting
          to null there reports "no further pages" for a list that has them.
  4. → null (no next link found).
END

FUNCTION extractAngleBracketed(part: String) → String?
  1. cursor ← 0.
  2. Loop:
     a. start ← offset of "<" at or after cursor. If none → return null.
     b. end ← offset of ">" at or after start + 1. If none → return null.
     c. If end > start + 1 → return the substring strictly between them.
     d. Otherwise the pair is an empty "<>" → cursor ← start + 1, continue.
END
```

Two properties this shape is required to have, both of which an implementation
can lose without failing any well-formed-input test:

- **Leftmost-match parity with `/<([^>]+)>/`.** Step 2d skips an empty `<>`
  rather than returning `""`, because `[^>]+` requires at least one character
  and so the regex moves on to the next `<`. Verified exhaustively over
  `{<,>,a,b}^≤8`.
- **Linear time.** Step 2b must search for `>` from *after* the `<`, never from
  the start — searching from 0 is both quadratic and, when a `>` precedes the
  `<`, silently wrong. The offsets must also be O(1) to compute: bytes, UTF-16
  code units, or flat code points. A *character* offset into a variable-width
  string is O(offset) in some runtimes, which makes step 2d quadratic on any
  header carrying a non-ASCII byte.

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

### The `page` Query Parameter `[conformance]`

Operations whose Basecamp endpoint honors `?page=` accept a `page` query
parameter. **A positive `page` selects exactly that page. It is not a starting
offset.** In every SDK:

- the operation issues **exactly one** request;
- it returns **only** that page's items;
- the `Link: rel="next"` header is **not** followed;
- `ListMeta.truncated` is true when that page carried a next link (or when
  `max_items` dropped items from it), because items beyond those returned were
  available;
- `ListMeta.total_count` comes from `X-Total-Count` as usual.

Absent, `0`, and negative all mean the same thing: auto-paginate the whole
collection per the algorithm above. `page` and `max_items` compose — the cap
still trims the selected page, and dropping items from it is itself truncation
— but `max_items` is not itself a page selector: it caps *items*, so it
collapses to a single request only when the cap does not exceed that page's
item count, which requires the caller to know the server's page size.

One qualification for Go, whose `max_items` analog is a per-operation `Limit`
with a **nonzero default** on several services (`DefaultTodoLimit` and
friends): only an explicitly-set positive `Limit` trims a pinned page. The
default must not, because a caller who asked for page 3 asked for page 3, not
for its first 100 items. The other five SDKs have no such default — an absent
`max_items` is uncapped — so the rule reads identically in all six: whatever
cap the caller set applies to the page they pinned.

```
FUNCTION paginate(initial_response, max_pages, max_items?, page?) → ListResult<T>
  0. If page is set and page > 0:
     a. items = parse first_page_items from initial_response body.
     b. dropped = max_items set (explicitly, not a per-operation default)
        AND items.length > max_items.
     c. truncated = dropped OR parseNextLink(initial_response.headers["Link"]) ≠ null.
     d. → ListResult(dropped ? items[0:max_items] : items,
                     meta: {total_count, truncated}).
  ... otherwise continue with step 1 of the algorithm above.
END
```

Two carve-outs, so the rule is not read as universal:

- `ListWebhooks`, `ListMessageTypes`, `ListChatbots`, `ListPingablePeople`, `ListQuestionAnswerers`, and `ListUploadVersions` carry the pagination trait but declare **no** `page` parameter: their Basecamp index actions return the whole collection rather than paginating, so there is no page to select. Where an SDK's shared pagination options type still admits a `page` (TypeScript, Kotlin, Swift), passing one on those operations changes nothing — the responses carry no next link to suppress.
- `GetMyNotifications` declares `page` but carries **no** pagination trait, so no SDK follows links for it and this section is inapplicable — it returns the page you asked for, in all six.

**How each SDK learns the pinned page.** The paginator reads it from whichever
representation it already holds, so no SDK carries a second copy that can drift
from the query string actually sent:

| SDK | Source of the pinned page |
|-----|---------------------------|
| TypeScript | `PaginationOptions.page` (generated options interfaces extend it) |
| Kotlin | `PaginationOptions.page`, via the generated `toPaginationOptions()` |
| Swift | `PaginationOptions.page`, passed by the generated service method |
| Python | the outgoing `params` dict (`selects_single_page`) |
| Ruby | the outgoing `params` hash (`single_page_selected?`) |
| Go | the hand-written wrapper's `opts.Page` (or `page` argument) |

Before #566 this held only in Go: `page` rode in the query string of the *first*
request while every subsequent request came from the `Link` header, so in the
other five SDKs `page: 3` of a 100-page collection issued 98 requests and
returned pages 3..N concatenated. Go escaped it because its hand-written
wrappers short-circuited before the follow loop when `Page > 0`. #566 converged
the five on Go's semantics; #570 closed the last Go hole (`GaugesService.List`
and `ListNeedles` took no options struct, so `page` could not reach the wire).

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

The cap is a resource bound and a hygiene measure — it limits how much server text lands in a message. It is not a secrecy control, and it is not asked to be one: the next section says which values must never be rendered at all, and none of them is bounded by a cap.

### Credential-Bearing Values Are Never Rendered `[manual]`

**A value that carries a credential the SDK holds or requested is never rendered — not in an error message, a cause chain, an observability hook's argument (§12), or a log line. A credential-bearing URL is rendered as its origin only, projected from a parse before any library can render the whole; a credential-bearing body is not rendered at all.**

Handing a credential to its owner is not rendering it. `on_refresh(access_token, refresh_token, expires_at)` and an auth strategy returning the token it holds are the credential's own lifecycle, and sit outside this rule; what the rule governs is text that leaves the SDK for someone who did not ask for the secret.

**The trust model, stated once.** The SDK's API peer is Basecamp: §8 and §23 refuse any cross-origin or downgraded URL before a request is issued. It contacts two other hosts, and both are precisely the credential-bearing exchanges — the storage host of §14's hop 2, and the OAuth token and device-authorization endpoints (§16, discovered under issuer binding and free to sit on any `https` origin). Their error bodies can echo what was sent to them (a storage host's `SignatureDoesNotMatch` quotes the signed query it rejected), so neither body is rendered: a hop-2 failure carries the status alone — `download failed with status 403`, which every SDK already does — and an OAuth endpoint failure carries RFC 6749's `error` and `error_description`, truncated, and no other part of the body. `error_description` is rendered deliberately: it is the caller's only diagnostic for an OAuth failure, and the endpoint that could echo a submitted secret into it is the endpoint the SDK handed that secret to — a broken issuer rather than a hostile peer, and under §16's issuer binding, Basecamp's own. Within the API peer, server-chosen text — an error body, a close reason, a reason phrase, a decoder's quotation of a malformed body — reaching a Basecamp customer's log is not a leak, and this document does not treat it as one; the truncation cap above bounds it and that is enough. What must not echo is the SDK's *own* secrets: the material it was issued, or asked for, in order to make requests. That is the whole class, and it is the class #788 was actually about — the §23 stream ticket rides in the mint URL's query string, and Go's `*url.Error` renders the URL it failed on, so a failed dial put a live credential in the logs.

**The secrets are a closed set, and enumerating them is sound.** Four rounds on #788 established that enumerating *destinations* — every log, hook, return path, chain and serializer a value can reach — is an open set that grows with every integration anyone writes. Enumerating *secrets* is the opposite: the SDK creates or requests every one, so the list is complete by construction and grows only when this document adds a credential.

| Secret | Where it travels | What renders it if unguarded |
|---|---|---|
| Bearer token (§4) | `Authorization` header | header logging — already covered by Sensitive Header Redaction below |
| Stream ticket (§23) | query string of the mint URL the cable dials | a transport's rendering of the dial URL: Go's `*url.Error`, a WebSocket library's connect error |
| Signed download URL (§14 hop 2) | the URL itself | a transport's rendering of the hop-2 URL: `httpx`, `URLError`, Ktor, Faraday |
| OAuth token and device-authorization responses (§4, §16) | response bodies — `access_token`, `refresh_token`, `device_code` | a JSON parser quoting the body it could not parse — or retaining it: CPython's `JSONDecodeError.doc` holds the whole document, so chaining that exception chains the body |
| OAuth request bodies (§16) | form bodies — `client_secret`, `code`, `code_verifier`, `device_code`, `refresh_token` | request-body logging. Nothing in this document logs a request body and a transport error renders a URL, never a body; that is the invariant to keep |

The SDK also **holds** credentials at rest that never transit a failure path: the bearer token inside its auth strategy, the OAuth `client_secret` in configuration, and §15's webhook signing secret on the receiver. A held value has nothing to project — it appears in no URL, response body, or transport error — and owes only that no rendering is ever built from configuration, which none is: not an error message, a log line, or a hook argument. (A caller who serializes their own configured client or receiver is publishing their own secret; no library rule reaches that act.) Beyond the table and those held values, nothing the SDK handles carries a credential — an inventory audited by sweep, not assembled from review findings, after three rounds each surfaced a missing row. Request URLs carry none — the token is in the header — which is why §12's `RequestInfo.url` and the pagination hooks may carry the full URL, and why hop 2 fires no request hooks (§14). One input can smuggle a credential into hop 1: `downloadURL` accepts any absolute URL and §14 step 2 preserves its query through the origin rewrite, so a caller who passes a signed storage URL puts its signature on hop 1's request. Hop 1's hook arguments are therefore projected, in every SDK (#837): its URL is rendered as origin and path — no userinfo, query or fragment, which is all an observer needs from a download request — and a transport failure reaches `on_request_end`, `on_retry` and the caller as the fixed network error with its cause projected as below, because a transport's rendering of that failure is the same URL. The wire request keeps the whole URL. Ordinary API requests are untouched: their URLs carry no credential, and the projection is gated on the download flow.

**The rule binds at the sites where the table's values can enter an error, and only there.** Each is a construction site the SDK owns: the cable dial failure (§23), the hop-2 failure (§14), and the OAuth exchange and refresh paths (network failure or unparseable token response) — and the failures *before* a dial as much as the dial itself: a signed `Location` that fails validation or URL construction is the same value, and is rendered the same way. At each, the error is constructed from a *projection* of the value before any library rendering can occur — the parsed URL's origin, or nothing — and the transport's own error is not chained where a caller or a runtime would render it:

- **Projection, not redaction.** Take the origin out of a successful parse and discard the rest; the query the credential rides in is gone before anything can be searched for. Searching the rendered text for the credential is the thing that failed three times in a row on #788 (a value below a length threshold, a percent-encoded form, a query with no `=`), and it cannot work in principle for the ticket, which the contract makes opaque. Where the URL yields no complete origin — it does not parse, or parses without a scheme or a host — the origin component is the fixed token `unparsable`.
- **The cause chain is an egress too.** Chaining the raw transport error so callers can unwrap it hands the URL straight back. Two runtimes attach it without being asked: CPython sets `__context__` at raise time, after the constructor returns, so construct inside the `except` and raise after it, or clear `__context__` at a boundary that runs after the raise. `from None` is **not** sufficient for these values: it suppresses the default rendering but leaves `__context__` populated, and for an unparseable token response that slot holds the `JSONDecodeError` whose `.doc` retains the entire credential-bearing body — a credential in a serializable slot, which the bold sentence forbids. `oauth/exchange.py` and `device.py` rely on `from None` today and owe the stronger form (#788); MRI sets the built-in cause at raise time past any `cause:` keyword the class stores, so pass `cause: nil` at the raise site — `ruby/lib/basecamp/oauth/exchange.rb` already does, for exactly this reason. Where a classifier legitimately reads through the chain (Go's `shouldTripCircuit` looks for `context.Canceled` and `context.DeadlineExceeded`; Swift's `isCancellation` looks for `CancellationError` or `URLError(.cancelled)`), chain the peer-free sentinel or a fresh instance of that type in place of the transport error, so classification survives and the URL does not.
- **Retention is fine.** Keeping the origin, or a typed failure kind, on the error is encouraged. Keeping the credential-bearing value itself on the error is not, in any field, because any field can be serialized.

**What this deliberately does not do.** It does not prescribe how a close reason, a decoder's message, a reason phrase, or a rejected pagination `Link` is rendered — those are Basecamp's own text, bounded by the cap, and free to be as diagnostic as an SDK likes. It does not restrict what hooks receive beyond the table's values — the download flow's hop-1 projection above is the one place a hook argument is shaped by this rule. It does not ask for a closed vocabulary or a type allowlist. Every one of those was tried on #788 as a way to make a general theory of "peer-derived text" hold across six languages, and each failed because text flow through six languages is an open set. This rule holds because its subject is the short list of values the SDK itself can name — the table above, and nothing outside it. (The list is deliberately not counted in prose: a count restated outside the table is one more constant to drift, and it did, twice, before this sentence learned that.)

**The pre-conformance state, for #788 — resolved by #837.** Before that change, the hop-2 sites in Python (`download.py`), TypeScript (`download.ts`), Ruby (`client.rb`) and Kotlin (`Download.kt`) constructed their network error from the transport's rendering (`f"Download failed: {e}"`, `err.message`, `e.message`, `cause.message`) and chained the raw cause; Go's `ErrNetwork` appended `cause.Error()` as `Hint` and returned it from `Unwrap`. Two pre-dial sites rendered the signed URL before any transport was involved: Ruby's `client.rb` (`redirect to undialable download URL: #{Security.truncate(url)}` — the cap kept a short query intact) and Swift's `HTTPClient.swift` (`Invalid URL: \(url)`); TypeScript's `new URL(location, …)` threw Node's `ERR_INVALID_URL`, which retains its input. Python's `auth.py` refresh path chained its `JSONDecodeError` with `from e`; `oauth/exchange.py` and `device.py` used `from None`, which silences the traceback but still leaves the body-retaining exception in `__context__` (above) — and every Python OAuth transport failure chained the `httpx` error, which retains the request whose form body is the table's last row. Hop-2 non-2xx errors were already status-only in every SDK, and Swift's transport messages were fixed strings that owed only the chain. Ruby's OAuth exchange (`raise ..., cause: nil`, with a comment saying why) was already conformant and was the pattern copied. The §23 dial-failure site is owed by each connector as it is written — none exists yet.

### Sensitive Header Redaction `[static]`

The following headers must be redacted (replaced with `"[REDACTED]"`) before logging:

- `Authorization`
- `Cookie`
- `Set-Cookie`
- `X-CSRF-Token`

Comparison is case-insensitive.

---

## §10. Type Fidelity

### One Renderer, One Schema `[conformance]`

**Two operations may share a schema only when BC3 renders them through the same
jbuilder partial.** When one of them renders a *reduced* partial instead, it gets
its own schema — even when the two shapes overlap almost entirely.

The failure mode is not cosmetic. A `@required` member that the reduced renderer
omits is a decode error in the strict tiers and a silent zero-fill in the lenient
ones, so the same response is a thrown `DecodingError` in Swift, a
`MissingFieldException` in Kotlin, `""`/`false` in Go, and `undefined` behind a
non-optional type in TypeScript. Relaxing the shared schema to optional is the
tempting fix and the wrong one: it weakens the contract for every operation that
really does always send the member, and it hides the next instance.

The worked example is `GetUpcomingSchedule` (#635). It renders
`app/views/api/schedules/calendar/_entry.json.jbuilder` and `_assignable.json.jbuilder`
— purpose-built calendar partials that do not render `recordings/_recording` at
all — while the five other schedule-entry operations render
`schedules/entries/_entry.json.jbuilder`. Modeling both with `ScheduleEntry` made
any populated window undecodable in Swift; an empty window, and a call with no
window (a bodiless BC3 400), were the only shapes that ever worked. The fix is
the `UpcomingScheduleEntry` / `UpcomingAssignable` / `UpcomingScheduleBucket` /
`UpcomingSchedulePerson` / `UpcomingAssignableParent` /
`UpcomingAssignableCompletion` family, named after their owning report the way
`Draft*`, `MyAssignment*` and `Timeline*` already are.

Two corollaries worth stating, because both were violated here:

- **A reduced projection is entitled to members the full one lacks.** The
  calendar entry partial emits `recurring`, which no other schedule-entry
  projection carries, and the calendar assignable partial emits the item text as
  `content` where the retired schema declared `title`. A reduced schema is not a
  subset — it is a different schema.
- **Fixtures must be built from the renderer, not invented.** Every test that
  covered this endpoint before #635 used a fabricated body — one whose top-level
  key was `entries`, which BC3 has never sent — so six SDKs shipped the mismatch
  with tests passing. Conformance cases and `spec/fixtures/` bodies are read out
  of the partial.

A second instance, absorbed the same way: `ListUploadVersions`. BC3 renders it
through `app/views/api/uploads/versions/_version.json.jbuilder` over the shared
`recordings/events/_event.json.jbuilder`, not through `uploads/_upload`. The
output declared `uploads: UploadList` anyway, which was a typed lie of the
sharpest kind — **11 of `Upload`'s 14 `@required` members are absent from every
response**, so the CLI's versions command and the MCP server's
`list_upload_versions` rendered blank fields (basecamp-sdk#649). It is now
`UploadVersion` / `UploadVersionFile`.

The reason not to widen `Event` instead is stated by bc3's own commit for the
partial: doing so "would leak upload fields onto todo, message, and card
events". `UploadVersion` also demonstrates the first corollary above from the
other direction — it is an Event *plus* a member no other event projection
carries (`upload`), which is exactly why plain `EventList` would have needed
growing again the moment anyone wanted the filename, the only reason the
endpoint is called at all.

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
  on_paginate(url: String, page: Integer) → void       -- Ruby and Python only; not in Go/TS/Kotlin/Swift
END
```

All methods are optional. A no-op default is valid. `on_paginate` is Ruby- and Python-only — new implementations may omit it.

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
- `{API_VERSION}` is the API version from `openapi.json` `info.version` (currently `2026-08-31`), derived from the shared date in `spec/api-provenance.json` <!-- @api-version -->

### Redirect Handling

`follow_redirects = false` on **both** hops of the download flow (§14). Hop 1's redirect is the
flow's own dispatch — the SDK reads `Location` itself and decides what to do with it — and a
redirect on hop 2 is refused outright (§14 "Hop-2 Redirect Policy"). Redirect responses are
handled explicitly, never by the HTTP stack's default policy.

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
     a. Set Authorization and User-Agent headers only (no Accept or Content-Type — this is a binary download, not a JSON API call). Every attempt is authenticated — re-run the auth strategy on retry so a rotated token is picked up. Request hooks receive the rewritten URL projected to origin and path — no query, no fragment: the input may have been a signed storage URL whose signature step 2 preserved (§9 "Credential-Bearing Values Are Never Rendered").
     b. Fetch with redirect: manual (do not follow redirects automatically).
     c. If the attempt fails with a network error, or the response status is in DOWNLOAD_RETRY_ON = {429, 502, 503, 504}: retry with exponential backoff while attempts remain (honor Retry-After at every status in the set, per §6), else surface the failure. 500 is DELIBERATELY outside the set — it is never retried.
     d. If response is redirect (301, 302, 303, 307, 308):
        - Extract Location header. ⊥ if absent.
        - Resolve Location against rewritten URL (handle relative redirects).
        - Proceed to Hop 2.
     e. If response is 2xx:
        - Direct download (no second hop needed).
        - → DownloadResult from response body.
     f. If response is any other error → ⊥ BasecampError from response, without retry.

  4. Hop 2 — Unauthenticated fetch (signed URL):
     a. Fetch Location URL with NO auth headers and redirect: manual. Hop 2 is NEVER retried, NEVER authenticated and NEVER redirected — the signed URL is single-purpose, credentials must not leak to the storage host, and the storage host does not get to choose a further destination (Hop-2 Redirect Policy below). It fires NO request hooks, and a hop-2 failure renders at most the signed URL's origin — a transport failure as a fixed message with the transport error unchained, a Location that fails validation or URL construction as its origin or the fixed token `unparsable`: the URL is itself a credential (§9 "Credential-Bearing Values Are Never Rendered").
     b. If response is a redirect (301, 302, 303, 307, 308) → ⊥ BasecampError api_error carrying that status; its Location is never dialled.
     c. If not 2xx → ⊥ BasecampError.
     d. → DownloadResult from response body.
END
```

### Hop-1 Retry `[conformance]`

The authenticated first hop retries on **network errors plus {429, 502, 503, 504}** — never 500. The set is declared here rather than inherited from anywhere else, and it matches neither of the two sets an SDK already has to hand: it is broader than the per-operation `retry_on` in `behavior-model.json` (`{429, 503}` for all `254` operations, which never governs `DownloadURL` because it has no entry there), and narrower than the error taxonomy's "all 5xx retryable" flag, which would sweep in the 500 this policy deliberately excludes. It is the gateway-error set Go's hand-written `singleRequest` already uses for GETs. <!-- @operation-count --> Backoff is exponential from a 1-second base with jitter; `Retry-After` is honoured at **every status in that set**, not at 429 alone. The second hop is exempt: no retry, no auth.

That last clause changed with §6's "Retry-After Honouring", and the reason it changed is the reason this set is declared here at all: honouring is derived from retry eligibility, so a loop that declares its own eligibility set inherits the honouring rule over that set rather than over §7's. A 502, 503 or 504 on hop 1 carrying `Retry-After` therefore waits what the origin named, exactly as a 429 does. `[CONFLICT: most download loops honour it on 429 alone today and owe convergence; one SDK already conforms. Per-SDK state and call sites in #775 — not restated here, because this is exactly the row that convergence changes. For conformance: the existing downloads.json case covering the 429 path stays valid; the other three statuses need cases of their own.]` The honoured value is subject to §6's other two clauses on this path as well: nothing is added to it, and it must be awaited through a cancellation handle the caller holds, which not every download path yet gives them (#775).

**Composition (§6 "Composition is per-loop"): a valid `Retry-After` REPLACES this hop's exponential-plus-jitter delay**, the same answer §7's loop gives and for the same reason — the wait is pacing a retry of exactly the request the origin just answered. It is stated here rather than inherited: §6 supplies no default, so a loop that declares its own retry set (as this one does) declares its own composition too.

"Network error" means a transport failure, with one carve-out that SDKs inherit from their main GET loop rather than restate: an attempt that exhausted the caller's entire per-attempt time budget (a request timeout) is not retried. The timeout is per attempt, so a retry spends another full budget on the same slowness rather than riding out a blip. Kotlin implements this explicitly; SDKs whose transports surface timeouts indistinguishably from other connection failures retry them.

Attempt budget per SDK — disabling retry (each SDK's spelling of `enable_retry=false` or a zero cap) yields exactly ONE hop-1 attempt:

| SDK | Budget |
|-----|--------|
| Go | `MaxRetries` as total attempts, floored at one (`MaxRetries: 0` still sends one attempt); the hand-written client rejects a negative cap |
| Python | `max_retries` as total attempts, floored at one (`max_retries: 0` still sends one attempt) |
| Ruby | `max_retries` as total attempts, floored at one on every path — downloads, governed GETs and ungoverned GETs alike (`max_retries: 0` still sends one attempt) |
| Kotlin | `maxRetries` as total attempts, floored at one, gated on `enableRetry`; an accepted `maxRetries = 0` still sends exactly one attempt |
| TypeScript | Fixed three-attempt policy when `enableRetry` is true; one attempt when false. No public numeric knob. |
| Swift | Fixed three-attempt policy when `enableRetry` is true; one attempt when false. No public numeric knob. |

Python and Ruby carve downloads out of their ungoverned GET taxonomy (which retries 500): the download hop uses the declared `{429, 502, 503, 504}` set, in both directions — the taxonomy neither widens nor vetoes it. `DownloadURL` is deliberately absent from `behavior-model.json`; SDKs pass this policy to their retry primitive directly rather than looking it up by operation.

### Hop-2 Redirect Policy `[conformance]`

The signed hop follows no redirect. A redirect (301, 302, 303, 307 or 308) from the storage host surfaces as `api_error` carrying
that status, with a message saying the redirect is **not followed** — the substring the conformance
case asserts — and the `Location` it carries is never dialled. The refusal is a property of hop 2's
own HTTP client, not of the dispatch around it: `CheckRedirect: ErrUseLastResponse` (Go),
`redirect: "manual"` (TS), `follow_redirects=False` (httpx), `dataNoRedirect` (Swift's
`Transport`), `followRedirects = false` (Ktor), `Net::HTTP#request` (Ruby, which never follows).
Every other hop in the SDK that a response could steer already refuses redirects or validates
each target — hop 1 here, §16's discovery fetches, §23's polls — and until #805 this hop was the
exception in four SDKs, by four different stack defaults, none of them argued.

**Why refuse rather than cap or validate.** Hop 2's target is the one URL the API host named, and
that host is operator-configured; what a followed redirect adds is a destination chosen by whoever
answers *that* URL. A hop cap bounds loops and resource use, not destination — one redirect to an
internal address is under every cap. Per-hop validation has nothing to validate against: a signed
URL is legitimately cross-origin to the API, and the SDK holds no roster of storage hosts, so the
only policy it can state is "the host the API named, and nothing that host names in turn". Refusal
states exactly that.

**Why it is safe to refuse.** Upstream, hop 2 is a presigned GET against a single-endpoint
S3-compatible object store (bc3 `config/storage.yml`; the redirect is minted by
`Downloading#respond_with_download_redirect` from the blob's service URL, with no
`direct_download_endpoint` configured). A presigned GET on a path-style single endpoint is answered
by that endpoint — the region and virtual-host redirects that make "S3 redirects" a real phenomenon
are artefacts of AWS's multi-region addressing, which this store does not have — and nothing
redirecting sits in front of it. Local and test environments never reach hop 2 at all:
`respond_with_download_on_disk` sends the body on hop 1. The strongest evidence is empirical,
though: Kotlin (by design, reusing hop 1's `followRedirects = false` client) and Ruby (by
`Net::HTTP`'s default) have refused hop-2 redirects since #178 introduced the download path, with
no download reported broken.

**What happens if that changes.** Should the storage tier ever start redirecting — a CDN in front of
it, a region move — every SDK fails loudly with the redirect's status and the "not followed" message rather than
quietly following somewhere. That is deliberate: the remedy is then a spec change argued from the new
evidence, with a destination policy attached, not a default that happened to work.

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

##### 5. Judge the advertised issuer's ADDRESS, not only its spelling `[Go-first]`

Requirements 1–4 apply to both hops, and hop 1 needs nothing further: its origin
is the resource identifier the *caller* supplied. Hop 2 is different in kind.
`discoverFromResource` lifts `authorization_servers[]` out of a parsed response
body, and with no `expectedIssuer` the "single non-Launchpad entry wins"
heuristic makes a remote peer's string the socket destination. `requireOriginRoot`
is a syntax gate with no notion of what a host *resolves to*, and issuer binding
runs only after the response comes back — so a refusal there is already too late
to stop the connection from reporting whether an internal host and port are live.
The exposure is bounded — fixed path, GET, no credentials, near-blind — but it is
a working internal host/port oracle, and the discovery fetch is not where the
selected issuer stops being used. The `Config` it returns carries
`token_endpoint` (required) and `device_authorization_endpoint`, and those become
the destinations of the grant itself: `performDeviceLogin` takes exactly this
already-selected config and posts the `client_id` to one and the `device_code` to
the other, and the token exchange and refresh post the authorization code,
`client_secret`, or refresh token to `token_endpoint`. Those are form-body
credentials, not an `Authorization: Bearer` header — no Bearer header is sent to
a discovered issuer.

Hop 2 therefore SHOULD refuse an advertised issuer whose **address** is in
private, loopback, link-local, CGNAT, or IANA special-purpose space, judged at
the moment of connection rather than by parsing the URL — which is also what
catches a legacy-numeric spelling (`https://2130706433/` is `127.0.0.1`) and a
name that resolves into that space. The refusal is the existing hard
`invalid_issuer_origin`, not `as_fetch_failed`: it is a permanent verdict on the
origin, and must not be marked retryable. It applies on **both** selection paths,
`expectedIssuer` included — an SDK-level exemption for a caller-named issuer is
silently wrong when the consumer computed that value from untrusted input.

Because an SDK is a shared dependency, an implementation MUST expose an override
for the policy, for the client that carries the hop, and to disable it — an
internal deployment must be able to admit its own range without abandoning the
rest of the deny tables.

**Go is the only implementation today**, via
`github.com/basecamp/surfguard/go`'s dial-time enforcement
(`oauth.DefaultIssuerPolicy`, `WithIssuerPolicy` / `WithIssuerHTTPClient` /
`WithoutIssuerPolicy`). It is written `SHOULD` and marked `[Go-first]` rather
than folded into the `[conformance]` list above because the corpus cannot express
it: the assertion is "no connection was attempted", which is not an observable
of the mock-HTTP runner, and the remaining four SDKs have no equivalent
enforcement layer to point at yet. See Appendix F.

##### 6. Judge the ADDRESS of the endpoints the selected metadata names `[Go-first]`

Requirement 5 closes the metadata GET and leaves an indirect route open
(#806). An attacker-controlled issuer on *public* space passes the policy, is
selected, and returns correctly issuer-bound metadata whose `token_endpoint`
and `device_authorization_endpoint` point wherever it likes — the metadata
parser checks only that `token_endpoint` is non-empty and copies it verbatim.
`requireSecureEndpoint` then admits any `https` host, private space included.
So the cost of reaching private space is one public host, and what arrives
there is the `client_id`, the `device_code`, the authorization code, the
`client_secret`, or the refresh token — real credentials, where requirement 5's
exposure was a blind GET. That is strictly worse than the one it closes.

Two remedies that look sufficient are not, and the reasons are worth keeping:

- **Same-origin is not the control.** RFC 8414 §2 requires the endpoint fields
  to be URLs and says nothing about their origin, and RFC 8705 §5's
  `mtls_endpoint_aliases` exists precisely so a server can publish endpoints
  *off* the issuer origin. A same-origin rule is a departure from the standard,
  not a tightening of it. It also does not survive DNS rebinding: the rule
  compares hostname strings, and the hostname is resolved twice — once during
  discovery, once at the later credential POST — so a name that resolves
  publicly at discovery can resolve privately when the credentials go out.
  Same-origin MAY be added as BC5 profile policy once fleet compatibility is
  confirmed — BC5's metadata controller mints every endpoint from its own
  route helpers next to `issuer: canonical_issuer_url`, but whether those share
  an origin in every deployment is the check to run first — and even then it is
  defence in depth, not the closure.
- **Scheme alone is not the control.** `https` is satisfied by any private host.

The control is the same one as requirement 5: **dial-time address enforcement
on every device-authorization and token endpoint request the device flow and
the token exchanger make**, judging the literal address at the moment each
socket opens, so there is no check-to-use gap for a rebind to exploit and no
assumption about what the AS chose to publish.

The device and exchange functions cannot tell a discovered endpoint from a
hand-configured one — `performDeviceLogin` takes a `Config`, `exchangeCode` and
`refreshToken` take a string — so the policy applies to every request those
functions make on their default client. That is the same uniformity decision as
requirement 5's `expectedIssuer` rule, for the same reason: a provenance flag on
the config is a marker that a consumer round-tripping the config through
storage silently drops. The overrides are therefore the consumer's, and an
implementation MUST expose the same three as requirement 5 — a replacement
policy, a replacement client for the request, and a way to disable it (handing
over a plain client counts). A refusal is coded `api_error`, is NOT retryable,
and in the poll loop terminates the flow on the first attempt rather than
backing off — surfguard's `unresolvable` (retry later) and `blocked` (stop)
are distinct verdicts and must stay distinct here.

The boundary stops at those functions, and one SDK path sits outside it by
construction: a refresh driven by **stored credentials** — Go's `AuthManager`
posting to `Credentials.TokenEndpoint` on the client its constructor was
handed — is caller-configured territory even when the stored endpoint was
originally discovered. The documented device-login bridge copies
`result.Config.TokenEndpoint` into the credential store, and that round trip
through storage is exactly the provenance loss described above: the endpoint
re-enters the SDK as caller configuration, on a caller-owned client, and the
enforcement is that client's. Consumers persisting discovered endpoints MUST
NOT infer that later automatic refreshes receive this requirement's default
policy; they compose the policy into the client they hand `AuthManager`, the
same as any caller-supplied client here. See Appendix F.

**Go is the only implementation today** (`oauth.DefaultIssuerPolicy` on the
device flow's and `Exchanger`'s default client; `WithDevicePolicy`,
`WithExchangerPolicy`, `WithDeviceHTTPClient`, a non-nil `NewExchanger`
client). Written `SHOULD` and `[Go-first]` for requirement 5's reasons. A
caller-supplied client is the caller's, enforcement included: the policy is not
layered on top of it, because the enforcement seam is the client's own dialer.
That contract is what makes a consumer that passes its general-purpose client
into these functions (as `basecamp-cli` does) responsible for composing the
policy into that client's transport. See Appendix F.

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

#### Token-Endpoint Transport Policy `[static]`

Every token-exchange and refresh POST — `exchangeCode`/`refreshToken` and
their per-SDK spellings, Ruby's legacy `OauthTokenProvider`, and Go's
`AuthManager` refresh — carries the transport contract the device flow and
the signed download hop (§14 "Hop-2 Redirect Policy") already hold. The
device flow's own POSTs already refuse and classify redirects status-first
under their own messages; the message contract below binds the
exchange/refresh paths it names:

- **No redirect is followed.** A 301, 302, 303, 307 or 308 from the token
  endpoint surfaces as the SDK's typed API error (`api_error`) carrying that
  status, with a message saying the redirect is **not followed** — the same
  substring contract as §14 — and the `Location` it names is never dialled.
  A followed 307/308 would re-POST the form — the authorization code, the
  client secret, or the refresh token — to a destination the response chose.
  Any other 3xx (304 above all) is the generic non-2xx failure, not this
  refusal. One carrier cannot deliver the status: a browser Fetch answers
  `redirect: "manual"` with an `opaqueredirect` (type set, status 0, headers
  hidden), so TypeScript running in a browser refuses it by type — same
  typed `api_error`, same **not followed** substring, but no `httpStatus`,
  because the browser withholds the real one. Browser consumers MUST NOT
  key redirect handling on the status field; the substring is the portable
  contract.
- **Classification precedes the body read on the default transport.** The
  refusal is read off the status line, so a redirect whose body stalls
  forever is the typed refusal above, never a timeout. The one permitted
  exception is an injected buffered client (Ruby's `http_client:` Faraday
  lane), which has no headers-time seam: it keeps the injected-client
  fidelity tier's coarser contract stated above — status-first
  classification on the completed response, wall-clock deadline otherwise.
- **The request is timeout-bounded**: 30 s default, the shared 3600 s
  ceiling, and an invalid caller value normalizes to the default — the
  numbers every other credential POST already converged on. One exclusion:
  Go's `AuthManager` refresh runs on the operator-configured API client and
  adds no SDK timeout of its own; it still refuses redirects, because a
  response-steered hop is response-steered whichever client carries it.
- **Suppression applies to injected clients too** (the device-flow
  precedent). Handing these functions your own client keeps your transport,
  dialer, and address policy — requirement 6's enforcement is deliberately
  NOT layered on top — but never re-enables redirect following. The
  distinction: redirect refusal is a property of the token-request
  functions themselves; the address policy is a property of the default
  client they post on. Suppression is applied at the SDK's own layer, and
  an injected transport must not undo it beneath that layer: Kotlin's
  engine re-wrap sets `followRedirects = false` on the Ktor client and
  cannot reach an engine that follows redirects on its own before Ktor sees
  the response. No Ktor engine does so by default (OkHttp and Java pin it
  off even over a preconfigured client; Apache, Android, Darwin, WinHTTP,
  curl, and JS default it off), so engine-level following is an explicit
  operator opt-in inside the engine's own config — and, like Ruby's
  adapter-only Faraday requirement, an injected engine must not carry it.

Marked `[static]` (per-SDK unit tests), not `[conformance]`: the oauth-token
corpus schema is scoped to resource semantics — a response is a status and a
body — and expressing even one redirect case would mean adding headers,
redirect, and never-dialled vocabulary to that schema, stretching the
instrument past what it observes.

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

Three functions per SDK. All device-auth + token requests are TLS-guarded (§9),
and — Go only, today — address-policed at dial time on the default client (§16
SSRF hardening, requirement 6), since the endpoints they POST credentials to may
be the ones a discovered issuer's metadata named.

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

**Scope first (§6 "What 'retry' means here"): of the branches that received a response, only the
`429` + `too_many_requests` branch is a retry.** The connection-timeout branch is the loop's other
in-scope path — a transport failure re-issued with backoff, §6's first clause, and §6's table already
lists it — but no response means no header, so it has nothing to honour and nothing to compose.
`authorization_pending` and `slow_down` are 4xx protocol *answers* — the completion poll
asked whether the user had finished and was told "not yet" — so re-issuing the POST is polling, not
re-attempting a failed request, and §6's honouring rule does not reach it. That is why this loop reads
the header on one branch and not the others, and it is a boundary rather than an omission: there is
also nothing for a `Retry-After` to displace on the pending branch, which waits the `interval` this
same authorization server prescribed and then raised with each `slow_down`, not a locally computed
backoff. A `429` without `too_many_requests`, or `too_many_requests` off any other status, is terminal
and never repeats, so it is outside for the plainer reason that no wait follows it.

**Composition (§6 "Composition is per-loop"): on the branch that *is* a retry, this loop takes
`max(interval, retryAfter)`, not the value alone.** §6 settles that a `Retry-After` on a status about to be retried is honoured, and
forbids capping it or adding jitter to it; `max` does neither — it selects between two waits rather
than shortening or padding either. The `max` is what makes this loop's answer differ from §7's, and
it is deliberate: `interval` here is not a backoff term the server is better informed about, it is
the polling cadence the authorization server itself handed out in the device-code response and then
raised with each `slow_down`. A 429 naming a *shorter* wait than the cadence the same server just
prescribed is not permission to poll faster than that cadence, so the larger of the two wins. Two
further bounds, already stated in the block above, are properties of this flow rather than of
`Retry-After`: the override applies to the next wait only and then decays, and the wait rule still
clamps it to the remaining code lifetime — a *domain* ceiling on how long polling can usefully
continue, not the locally computed backoff ceiling §6 exempts the value from.

**Parsing (§6 "Composition is per-loop", the paragraph under its table): this branch reads the
delta-seconds form only, and that is a declared exception to §6's Parsing Algorithm, not a fourth
parser.** An HTTP-date here is malformed and falls back to the current `interval`. The reason is
this loop's clock: every wait is measured on the injectable monotonic `clock` against a deadline
fixed at issuance, and a date can only be resolved against wall-clock `now()`, which this loop never
reads. §6 records the exception and its cost; the block above is the contract.

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

1. **No hand-written wire I/O.** Every request flows through public generated wire methods (Go: through the shared generated-client transport). No manual path construction or verb selection. Bodies use the generated request types, with one Go-specific carve-out: where the wire contract is inexpressible through the generated request type, the method MAY marshal an explicit body map and call the operation's generated `*WithBody` variant, with keys matching the generated request schema — the generated wrapper still owns path, verb, content type, and response decoding, and the operation identity still reaches hooks and retry. The known instance is the `""` date clear, which cannot pass through a `*types.Date` member — its three spellings are absent (nil pointer), `null` (zero value), and a real date. An empty string or empty list behind a pointer member is NOT such an instance: a non-nil pointer to an empty value survives `omitempty` and reaches the wire. This is the only sanctioned use of hand-marshaled bodies; a body the generated request type can express keeps using it.
2. **Composition, not substitution.** It composes existing generated operations (e.g. GET → overlay → full PUT); it never introduces a wire operation the spec lacks — fix the spec and regenerate instead.
3. **Native hook identities.** Hooks observe the constituent wire operations under their normal per-language identities; composites never mint synthetic operation names.
4. **Conformance-covered.** The composite's behavior is encoded in `conformance/tests/` fixtures run by every runner. All six SDKs now have one, so a native test mirror is no longer a substitute for fixture coverage.
5. **Declared placement.** The composite lives in the language's designated hand-written extension point (Kotlin generator `EXTENSIBLE_SERVICES`/`HAND_WRITTEN_SERVICES`, TS `src/services/*-extensions.ts` wired in `client.ts`, Ruby zeitwerk `prepend` module, Python service subclass re-exported by the client, Swift same-module extension) so regeneration can never silently drop or fork it.
6. **The raw operation stays reachable.** When a composite takes over the plain method name, the generated single-request method is renamed (via `METHOD_NAME_OVERRIDES`) rather than hidden, and gets its own conformance case asserting it makes exactly one request with no read-before-write. Without that second case, later generator drift could silently turn both public methods into composite behavior and nothing would notice.

### Replace-Semantic Operation Naming `[static]`

A wire operation is named for what the server does with the body, not for what the caller usually intends:

- **`Replace*`** when the endpoint takes a complete representation and clears what the body omits — `ReplaceTodo`, `ReplaceDocument`, `ReplaceScheduleEntry`. This holds even where the replacement carries *declared carve-outs*: `ReplaceDocument` does not touch a drafted document's subscribers, and that does not make the operation a merge. A carve-out is one named field the server excludes from the swap; a merge is the server preserving anything the body omits. The rule keys on the default, and the carve-out is documented on the operation.

  **The carve-out set is bounded, and the bound is what keeps the distinction honest.** `Replace*` survives declared carve-outs only while the preserved set is limited to fields a client *could not safely resend from a read-back* — write-only, system-managed, or identity-colliding. Every field that is both readable and writable still clears on omission. Widen the set past that and the operation has become a merge wearing a replace's name.

  `ReplaceScheduleEntry` is the worked example. It declares `preservedOnOmission: ["participant_ids", "url", "highlighted"]`, and each of the three earns its place:

  | field | why a client cannot resend it |
  |---|---|
  | `url` | **identity-colliding.** On write it is the entry's join link; on read, `url` is the entry's own Basecamp API URL — `recordings/_recording.json.jbuilder` writes that key first and the entry partial renders after it, so BC3 emits the join link as `join_url`. Echoing the response's `url` back into this member writes the API URL into the join link. |
  | `highlighted` | **was write-only.** Accepted on write but never emitted until basecamp/bc3#12502, so no caller had a value to resend. |
  | `participant_ids` | **system-managed on read-back.** The response carries `participants` (objects, not IDs), and BC3 re-screens a submitted list through the bucket's reachable people, so resending a projection can silently drop a participant who has since become unreachable. |

  `summary`, `description`, `all_day`, `starts_at` and `ends_at` are readable and writable, so they are *not* carved out and a merge-safe composite must resend them. `all_day` is the sharp edge: the column is NOT NULL with a `false` default, so omitting it converts an all-day entry into a midnight-to-midnight timed one.

  The trait field is `preservedOnOmission` on `@basecampWriteSemantics`, and it is carried into `behavior-model.json` as well as the OpenAPI extension. `make check-write-semantics-parity` compares the two artifacts in both directions, because they are produced by different tools and the behavior-model generator builds its clause key by key — a trait field nobody taught it about is dropped silently, which is exactly how `preservedOnOmission` would otherwise have shipped as a no-op.
- **`Update*`** when the endpoint merges — the server preserves fields the body omits (`Recordable#changing` and friends), as Messages does. Cards was the one hybrid — merge for `title`/`content`, key-guarded for `assignee_ids`, forced-replace for `due_on` (#467) — until basecamp/bc3#12521 made its JSON representation uniformly merge-semantic; it is now an ordinary `Update*`.

One shipped operation is replace-semantic but still named `Update*`: `UpdateTodolistOrGroup` reached the honest *method* name through `METHOD_NAME_OVERRIDES` (rule 6) rather than a wire rename, so its SDK surface reads `replace` while the operationId does not. That is naming debt, not a second sanctioned pattern; the wave that closes it is #374. New replace-semantic operations take the wire rename.

A rename is breaking and ships **without a deprecated alias** (`ReplaceTodo`, #375; `ReplaceDocument`, #543). An alias would keep the destructive method reachable under the name that misdescribes it, which is the defect the rename exists to remove.

Current composites:
- **Todos** `update` (merge-safe) and `edit` (read-modify-write) — see §5 "Merge-Safe Write Surface (Todos)".
- **Todolists** `update` (merge-safe) and `edit` (read-modify-write) — see §5 "Merge-Safe Write Surface (Todolists)". The raw path is `replace`, renamed from `update` via `METHOD_NAME_OVERRIDES` (rule 6) rather than by renaming the wire operation.
- **Documents** `update` (merge-safe) and `edit` (read-modify-write) — see §5 "Merge-Safe Write Surface (Documents)". The raw path is `replace`, and it needs no override: the wire operation is `ReplaceDocument`, so the ordinary naming algorithm produces it.
- **Schedule entries** `updateEntry` (merge-safe) and `editEntry` (read-modify-write) — see §5 "Merge-Safe Write Surface (Schedule Entries)". The raw path is `replaceEntry`. Alone among the composites it is **carve-out-aware**: it resends the five readable-and-writable fields from the read-back but leaves `participant_ids`, `url` and `highlighted` off the wire unless the caller addressed them, because BC3 preserves those server-side and resending them is redundant at best and wrong if the read raced a change.
- **Cards** `update` (merge-safe) — see §5 "Merge-Safe Write Surface (Cards)". The raw path is `updateVerbatim`.
- **Uploads** `download` — composes the generated `get` (GetUpload) with the client-level `downloadURL` primitive (§14), erroring when the upload carries no `download_url`; the result's filename prefers the upload metadata's `filename`.

**Body compaction is not relaxed for composites.** A composite never sends `{"field": null}` to express "clear" (§18 rule). Where a server accepts a blank-cast — as BC3 does for `due_on`, which it casts to nil and which a server test pins — the **empty string** is the clear encoding, and it is the only one all six SDKs can express identically: five strip nulls structurally before the wire (Python `_compact`, Ruby `compact_params`, Kotlin `?.let`, TypeScript's `JSON.stringify` dropping `undefined`, Swift `encodeIfPresent`), but none of them strip `""`.

Omission is **not** a clear encoding. It once was for `due_on`, when BC3 merged card params over `{ due_on: nil }`; basecamp/bc3#12521 removed that default, so an absent key now means "leave unchanged" and an omission-encoded clear silently no-ops.

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
| `requestBodyAbsent` | The `path` key is **not** present in the captured JSON request body — the assertion that pins body compaction (§18) and, for a merge-semantic endpoint, that an unaddressed field is left off the wire rather than echoed back. A request with no JSON body at all satisfies it. |
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

Every tracked fixture under `conformance/tests/` has exactly one row here, and
the category slug is the filename (basename, `_` written as `-`).
`make doc-constants-check` asserts the bijection.

<!-- @fixture-categories:begin -->
| Category | Files | Owning Spec Section(s) |
|----------|-------|----------------------|
| auth | `auth.json` | §4 Authentication, §13 HTTP Transport |
| cards-write | `cards_write.json` | §5 Merge-Safe Write Surface (Cards), §18 Hand-Written Composite Methods |
| documents-write | `documents_write.json` | §5 Merge-Safe Write Surface (Documents), §18 Hand-Written Composite Methods |
| downloads | `downloads.json` | §14 Download |
| error-mapping | `error-mapping.json` | §6 Error Taxonomy |
| idempotency | `idempotency.json` | §7 Retry (Gate 2) |
| integer-precision | `integer-precision.json` | §10 Type Fidelity |
| live-my-surface | `live-my-surface.json` | External governance (CONTRIBUTING.md, live canary — opt-in via `BASECAMP_LIVE`) |
| network-retry | `network-retry.json` | §7 Retry (network errors, Gate 2) |
| pagination | `pagination.json` | §8 Pagination |
| paths | `paths.json` | §3 Client Architecture (account path construction) |
| retry | `retry.json` | §7 Retry |
| schedule-entries-write | `schedule_entries_write.json` | §5 Merge-Safe Write Surface (Schedule Entries), §18 Hand-Written Composite Methods, §10 Type Fidelity (explicit-empty vs. omitted wire semantics) |
| search | `search.json` | §10 Type Fidelity — the polymorphic search projection, whose file-attachment branch is recognized by the ABSENCE of the recording envelope's `id`/`title`/`type`/`url`/`app_url` |
| security | `security.json` | §9 Security |
| status-codes | `status-codes.json` | §11 Response Semantics |
| todolists-read | `todolists_read.json` | §5 Merge-Safe Write Surface (Todolists) — the flat read shape the composites read through |
| todolists-write | `todolists_write.json` | §5 Merge-Safe Write Surface (Todolists), §18 Hand-Written Composite Methods |
| todos-write | `todos_write.json` | §5 Merge-Safe Write Surface (Todos), §18 Hand-Written Composite Methods |
| upcoming-schedule | `upcoming_schedule.json` | §10 Type Fidelity — the reduced calendar projection `GetUpcomingSchedule` renders, distinct from the shared `ScheduleEntry` shape |
| uploads-download | `uploads_download.json` | §14 Download, §18 Hand-Written Composite Methods |
| uploads-write | `uploads_write.json` | §5 Merge-Safe Write Surface (Cards, Uploads), §18 Hand-Written Composite Methods, §10 Type Fidelity, §6 Error Taxonomy (507 → limit_exceeded) |
<!-- @fixture-categories:end -->

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
closes a gap deletes exactly its own entry from `spec/zero-skip-roster.yml`,
which is where the roster is written; the block below is rendered from it.

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
layer down.

Two checks cover that state, and the split between them is the point.

Each runner takes a **case census** (#742): at the end of its own run it asserts
that `passed + failed + skipped` equals the number of cases under
`conformance/tests` whose `mode` is not `live`, counted by a walk independent of
its own load path. That catches a case executed by no runner for a MECHANICAL
reason — an unrecognized `mode`, a fixture that failed to parse or was never
globbed, one nested where no runner looks, a case dropped between load and
dispatch. It cannot catch the case this section describes: where every runner
excludes the same fixture deliberately, each census counts its own skip and
stays green.

`make check-fixture-execution` (#602) is what catches that one. Every runner
writes the cases it did not execute to `conformance/manifests/<runner>.json`,
and the gate fails when a case appears in all six. Manifests rather than parsed
output because TypeScript prints no `SKIP:` line — a skip there is `it.skip` —
so a gate scraping stdout would be blind to exactly one runner, in the silent
direction.

Its absence rule is the whole design. FULL mode requires all six manifests and
fails if any is missing, because a missing manifest must never read as "that
runner executed everything" — that assumption is precisely what makes an all-six
case invisible. Swift's runner is macOS-only, so a Linux run produces five and
runs in PARTIAL mode instead: an exclusion shared by every VISIBLE runner is a
warning, never a failure, since five-of-six is not the all-six claim and a
warning cannot false-fail. CI resolves it properly — the six language jobs each
upload their manifest and the fan-in job runs FULL mode over all six.

Maximum overlap today is 2 of 6 (narrowed by #596), so the gate is green on
arrival and a live run only proves it can say yes;
`scripts/test-check-fixture-execution.rb` crafts the all-six state and every
absence case, and is what proves it can say no.

The roster below is GENERATED. Its source is `spec/zero-skip-roster.yml`; `make
doc-constants-check` renders that file and requires the block between the
markers to match byte for byte, and `make sync-api-version` rewrites it. A hand
edit to the block is REJECTED rather than repaired: `make` runs `check`, which
reports the drift and stops; `make sync-api-version` is the only thing that
rewrites the block, and it rewrites it from the YAML. It used to be the other
way round — the roster lived here and a parser read it back out — until that
reader's "a misreading always surfaces as a mismatch" invariant had been
breached five times, each fix a new selector for a new spelling. Prose is an
output now, so there is nothing left to misread.

The roster is still HALF checked, and the split is the point. Its ENUMERATION —
which runner skips which case — is compared for set equality against the
execution manifests by `make check-fixture-execution` (#736), so a skip added to
a runner without an entry in the YAML, or an entry left behind after a gap
closes, fails the build. Its CLASSIFICATION and reasoning are judgement, and
nothing asserts them; that is why the section keeps its `[manual]` tag.

The check found this roster already wrong on arrival: Kotlin and Swift each
exclude the `link-header` case wholesale through their tag branch, and the
roster described that in prose instead of enumerating it — two of six runners
misstated, in a roster whose own text promises one line per runner × test.

Checking the enumeration against what the runners REPORTED is stronger than
parsing their source: a source parser cannot see a tag branch, a derived table,
or a case the loader dropped, which is why the enumeration waited for the
manifests rather than being checked on its own.

<!-- @zero-skip-roster:begin -->
**Go** (`conformance/runner/go/main.go` `goSDKSkips`) — architectural; same-origin logic is covered by `TestIsSameOrigin` unit tests:
- "Mixed-case host and explicit default port stay on the mocked origin" — Go runner dials `configOverrides.baseUrl` directly; its `httptest` mock owns its origin, so origin-interception normalization does not apply.
- "Bracketed IPv6 loopback origin stays on the mocked origin" — same as above.

**Python** (`conformance/runner/python/runner.py` `SKIPS`) — none; the `link-header` fixture above runs; only its `requestCount` assertion is suppressed.

**Ruby** (`conformance/runner/ruby/runner.rb` `RUBY_SKIPS`):
- "PUT operation is naturally idempotent" — GET-only retry (waiver 2B.3).
- "DELETE operation is naturally idempotent" — GET-only retry (waiver 2B.3).
- "POST operation retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "Subscribe POST retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "CreateBookmark POST retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "DeleteBookmark DELETE retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "SpotlightRecording POST retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "UnspotlightRecording DELETE retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "RecordProjectVisit POST retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "UpdateMyNote PUT retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "UpdateCalendar PUT retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "PrioritizeAssignment POST retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "DeprioritizeAssignment DELETE retries when marked idempotent" — GET-only retry (waiver 2B.3).
- "Network error on an idempotent POST is retried then succeeds" — GET-only network retry (waiver 2B.3).

**TypeScript** (`conformance/runner/typescript/runner.test.ts` `TS_SDK_SKIPS`):
- "Large integer IDs preserved without precision loss" — `Number` is 53-bit (waiver 1B.6).

**Kotlin** (`kotlin/conformance/.../Main.kt` — `KOTLIN_SKIPS` is empty; the entry below comes from the `link-header` tag branch) — architectural:
- "List operation returns first page with Link header" — the SDK auto-paginates, and its status model reports the last consumed response, so a one-response queue yields "no response" (see the tag-branch discussion above).

**Swift** (`conformance/runner/swift/.../Runner.swift` — `temporarySkips` is empty; the entry below comes from the `link-header` tag branch) — architectural:
- "List operation returns first page with Link header" — same as Kotlin: auto-pagination plus a last-consumed-response status model.

<!-- @zero-skip-roster:end -->

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
strings non-empty with no commas, whitespace, or quotes; each id list capped at 100. The
capacity options are validated the same way: `dedupeCapacity` and `liveBufferCapacity`
must be positive (there is no dedupe-disabled mode — a zero capacity would silently break
the deduplicated-surface promise). A
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
| 4 | Minting | Backoff | mint transient/throttled (Retry-After honored as the floor of the next delay), or an unauthorized mint (401/403) below the shared-counter threshold (`unauthorized` carries no `retry_after`, so the `backoff` draw alone governs — §6 "What 'retry' means here") |
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
| 24 | Streaming | CatchingUp | `repair-poll` fired → one walk from the connector's current position (in-memory authoritative — Checkpoint discipline below; 60s ± 20%) |
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
pass through it, as does a redirect `Location` failing per-hop validation inside a poll
seam call (Continuation and Resume URL Validation); zero requests to the failing URL); the Terminal(`poll_failed`) edge (an
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
  ("Retry" in the connector's sense of re-entering the reconnect cycle; under §6's definition this
  is authorization recovery, not a retry — the `unauthorized` kind carries no `retry_after`, so no
  `Retry-After` reaches the `backoff` draw on this path.)
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
  `Observer.checkpoint_save_failed` and the feed continues. The connector's position
  tracking is in-memory and authoritative for resume and repair within a run; the store
  is write-through durability only — a failed `save` never regresses or blanks the live
  cursor. `load` happens exactly once,
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
- **Dispatch timing:** a semantic signal is dispatched at the first consumer-context
  opportunity after its condition arises, with "before the next `save`" as the outer
  bound. Drop-time dispatch is therefore the normative expectation fixtures may rely
  on — an implementation must not defer the signal to a later cut that may never come.

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
| `invalid_continuation` | a `next` or 410 `resume` URL failed same-origin/downgrade validation (Continuation and Resume URL Validation below), or a redirect `Location` that fails the same validation; no request is issued to the failing URL |
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
-- retryable outcomes exhausted inside the seam → throttled(retry_after) when the last
-- response carried a parsed Retry-After, at ANY status, else transient (§6 "What
-- 'retry' means here"); 401/403 → unauthorized (shared counter); anything else
-- non-retryable (404, 422, a malformed success) → unrecoverable → Terminal(mint_failed),
-- generated error attached.

INTERFACE PollSource
  poll(cursor: Cursor, filters: Filters, cancellation) → PollPage
  -- One fully-governed generated PollEvents call; `cancellation` as on TicketMinter —
  -- triggered on close(), caller cancellation, AND any teardown of the attempt the call
  -- belongs to (mid-walk socket failure, staleness, a terminal): a superseded poll must
  -- not stall reconnection or return into a disposed attempt. Prompt return required.
END

RECORD Cursor           -- exactly one field set; the zero Cursor is the bare present entry
  position : String?    -- resume/repair token (in-memory authoritative within a run;
                        -- durable via write-through when saves succeed)
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
-- unauthorized | redirect_refused(location_origin) | unrecoverable(error).
-- The adapter maps every §6/§7 outcome of the generated call onto exactly one kind:
-- 429/503 and §7-retryable outcomes exhausted inside the seam → throttled(retry_after)
-- when the last response carried a parsed Retry-After, at ANY status, else transient;
-- the feed's 400/409/410 matrix → its four kinds; 401/403 (after the seam's own token
-- refresh and retry budget) → unauthorized; a 3xx whose Location fails the per-hop
-- same-origin/no-downgrade validation (auto-follow is disabled — Continuation and
-- Resume URL Validation) → redirect_refused, carrying the refused Location redacted to
-- its origin → Terminal(`invalid_continuation`), NEVER unrecoverable; anything else
-- non-retryable (404, 405, unexpected shapes) → unrecoverable, carrying the generated
-- error verbatim. A same-origin Location may be followed inside the seam under the same
-- per-hop rule (no error surfaces).

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
  write_frame(String)    -- close()/cancellation MUST unblock an in-progress write; a
                         -- write failure takes the current state's socket-failure path
  close(code, reason)    -- idempotent, safe from any context, unblocks read_frame AND
                         -- write_frame
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
  load(key: CheckpointKey) → Loaded(position) | Missing | Failed(error)
  save(key: CheckpointKey, position: String) → Saved | Failed(error)
  -- Tri-state by contract — a boolean/void shape cannot express the failures this
  -- section dispatches on. Missing proceeds to a present-class entry (no stored cursor
  -- is not an error). Failed on load is Terminal(`checkpoint_load`) with zero wire
  -- attempts; collapsing it to Missing would silently start at the present and skip
  -- history. Failed on save is `Observer.checkpoint_save_failed` with the feed
  -- continuing — subsequent saves are still attempted (no save circuit-breaker).
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
rejected URL is carried redacted (origin only) in the error; a URL that yields no complete
origin renders the fixed token `unparsable` (§9 "Credential-Bearing Values Are Never
Rendered"). There is no retry and no handler for this condition: a hostile continuation is
not an operable feed state.

**Prevalidation does not cover redirects, so the poll seam must.** The underlying HTTP
stacks auto-follow redirects (Go strips `Authorization` on a cross-origin hop but still
egresses), which would falsify the zero-foreign-egress guarantee the moment a validated
same-origin URL answers 3xx with a foreign `Location`. The Layer-1 adapter therefore
**disables automatic redirect-following for `PollEvents`** (or per-hop validates every
resolved `Location` under §8's hop-anchored rule): a 3xx from a validated URL yields its
`Location` to the same same-origin + no-downgrade validation — cross-origin or downgraded
→ Terminal(`invalid_continuation`) with zero egress to the foreign origin; same-origin →
it may be followed, each hop under the same rule.

The mint's cable `url` is deliberately **not** under this rule: it is server-directed
cable topology, cross-host by design, dialed verbatim with its own credential (the
short-lived ticket rides in the URL itself; no `Authorization` header is attached), and
governed by its own invariants — `wss://` outside localhost, redirects refused, never
logged (Security Invariants below).

Required tier-2 coverage: a hostile cross-origin `next` mid-walk, a hostile 410
`resume` URL, and a validated same-origin `next` answering 302 with a cross-origin
`Location` each terminate with `invalid_continuation` and zero requests to the foreign
origin; store-failure coverage proves Failed(load) terminates with zero wire attempts and
Failed(save) continues with the observer signal and a subsequent save attempt.

### Clock, Timers, and Virtual Time `[conformance]`

**Every delay the connector itself takes flows through the injected Clock** — no native
timer or sleep may bypass it. Delays *inside* a seam call are outside this rule: a
generated operation's §7 retry backoff is the operation's own machinery (native, as
shipped), which is exactly why conformance counts seam calls rather than wire attempts —
scenarios never depend on advancing seam-internal time. There are exactly six timer kinds, kebab-case:

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

**Composition (§6 "Composition is per-loop"): these two timers answer differently, and both
answers are in the table above.** `backoff` takes `Retry-After` as a **floor** —
`max(draw, retryAfter)` — so it wins only where it is the longer wait, and wins outright
above the 60s cap. `poll-retry` takes it as an **exact** wait, replacing the draw. Nothing
here is new behaviour; §6 supplies no default composition, so the connector states its own,
and it needs two rows because the two timers are not doing the same job. `backoff` spaces
whole reconnect cycles apart after repeated failure: a 1s header against a 50s draw waits
50s, because a server naming one second has said nothing about whether the condition that
failed the last several cycles has cleared, and the connector has no cheaper way to find
out than to keep the gap it selected. `poll-retry` paces the re-send of one throttled poll
against the same origin that just answered it, which is the §7 situation exactly, so it
gets the §7 answer. Both are still `Retry-After` honoured at a status that was going to be
retried anyway — only the composition differs.

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

- **Never log the ticket or the mint URL's query string** — the ticket rides in it, which
  makes the mint URL one of the credential-bearing values §9 "Credential-Bearing
  Values Are Never Rendered" names. A dial failure renders that URL as its origin only,
  projected from a parse (`unparsable` where there is none), and never chains the
  transport's own error where a caller or runtime would render it. Poll and resume URLs
  are not credentials — polls authenticate with the bearer header — so `gap(resume_url)`
  and `catch_up_started(cursor)` carry them whole.
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
| `API_VERSION` | `2026-08-31` | — | `openapi.json` `info.version` <!-- @api-version --> |
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

**AccountClient-level (`53`):** <!-- @service-count -->
<!-- @account-scoped-services:begin -->
account, attachments, automation, bookmarks, boosts, calendars, campfires, cardColumns, cardSteps, cardTables, cards, checkins, clientApprovals, clientCorrespondences, clientReplies, clientVisibility, cloudFiles, comments, documents, drafts, events, everything, folders, forwards, gauges, googleDocuments, hillCharts, lineup, messageBoards, messageTypes, messages, myAssignments, myNotes, myNotifications, people, projects, recordings, reports, schedules, search, subscriptions, templates, timeline, timesheets, todolistGroups, todolists, todos, todosets, tools, uploads, vaults, webhooks, wormholes
<!-- @account-scoped-services:end -->

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

Every tracked fixture under `conformance/tests/` appears on at least one row.
Rows are curated summaries and may bundle several cases, so the coverage is
what `make doc-constants-check` asserts — not a case-by-case index.

<!-- @fixture-section-map:begin -->
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
| `retry.json` | Retry-After HTTP-date in the past falls through to backoff | §6, §7 |
| `retry.json` | Retry-After of 0, and a negative value, rejected | §6, §7 |
| `retry.json` | Partly numeric Retry-After rejected (`1*DIGIT`) | §6, §7 |
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
| `downloads.json` | DownloadURL refuses a redirect on the signed second hop | §14 |
| `network-retry.json` | Network error on a non-idempotent POST is not retried | §7 (Gate 2) |
| `network-retry.json` | Network error on an idempotent POST is retried then succeeds | §7 (Gate 2) |
| `uploads_download.json` | UploadsDownload delegates through DownloadURL primitive | §14, §18 |
| `uploads_download.json` | UploadsDownload errors when upload has no download_url | §14, §18 |
| `uploads_write.json` | create-version presence states (unaddressed / clear / set) | §5 (Cards, Uploads), §18 |
| `uploads_write.json` | update presence states (unaddressed / clear) | §5 (Cards, Uploads), §18 |
| `uploads_write.json` | list-versions decodes the version payload | §10 (One Renderer, One Schema) |
| `uploads_write.json` | 507 → limit_exceeded, not retried | §6 |
| `todos_write.json` | update-merge / edit-clear / replace-omission-clears | §5 (Todos), §18 |
| `documents_write.json` | update-merge / edit-clear / replace-omission-clears | §5 (Documents), §18 |
| `todolists_write.json` | update-merge / update-group / edit-clear / replace-omission-clears | §5 (Todolists), §18 |
| `todolists_read.json` | list-read / group-read / group-list-read (one flat shape decodes for both variants) | §5 (Todolists) |
| `cards_write.json` | Presence-aware update composite (5 cases: unaddressed fields stay off the wire, verbatim raw path, explicit `due_on` clear as `""`, explicit empty content/assignees) | §5 (Cards), §18 |
| `schedule_entries_write.json` | Carve-out-aware replace/update/edit triad, plus the create-side #641 fields (11 cases: omission-preserves and explicit-clear pairs for `participant_ids`/`url`/`highlighted`, edit-touched vs edit-untouched, and `url`/`highlighted`/`status` present-when-set vs absent-when-unset on `CreateScheduleEntry`) | §5 (Schedule Entries), §18 |
| `upcoming_schedule.json` | The reduced calendar projection: entry, recurring occurrence, assignable, empty envelope (4 cases) | §10 (Type Fidelity) |
| `search.json` | The polymorphic search projection: the generic recording envelope plus all four special branches, and the file-attachment branch in isolation (2 cases) | §10 (Type Fidelity) |
| `live-my-surface.json` | Live schema validation, 31 read-surface cases (opt-in via `BASECAMP_LIVE`) | External governance (CONTRIBUTING.md, live canary) |
<!-- @fixture-section-map:end -->

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

- Total operations: `254` <!-- @operation-count -->
- Idempotent: 86 (flagged with `idempotent: true`)
- Non-idempotent: 168 (no `idempotent` field, or not present)
- All operations use `retry_on: [429, 503]`

---

## Appendix F: Known Cross-SDK Divergences

### Advertised-Issuer Address Policy (§16)

§16's SSRF requirement 5 — judging the *address* an advertised
`authorization_servers[]` entry resolves to, at connection time — is implemented
in Go only. This is a deliberate Go-first move, not an oversight in the other
five: the enforcement seam it needs (a dial-time `Control` hook, plus a shared
classification table) exists cheaply in Go and does not in the others.
Extending enforcement to the remaining SDKs is tracked in #818 (umbrella;
per-SDK #814/#815/#816/#817, upstream `surfguard` #24/#25).

| SDK | Advertised-issuer hop |
|-----|----------------------|
| Go | `oauth.DefaultIssuerPolicy()` — `surfguard.Policy{}.IANASpecialUse().AllowAllPorts()` — installed on a separate client that carries only that hop. Refused as hard `invalid_issuer_origin`, non-retryable, on both selection paths. Overrides: `WithIssuerPolicy`, `WithIssuerHTTPClient`, `WithoutIssuerPolicy` |
| TypeScript, Ruby, Python, Kotlin | Requirements 1–4 only: origin-root syntax gate, HTTPS, bounded timeout, suppressed redirects, bounded body. An advertised issuer naming a private address is still dialed |
| Swift | Not applicable — ships no OAuth discovery implementation |

Two consequences are worth stating rather than discovering. First, this is a
behavioral tightening, not a pure addition: a Go consumer whose BC5 issuer is
advertised on loopback or in RFC 1918 space, and which worked before, now needs
`WithIssuerPolicy`. Second, `Allow(prefix)` does **not** re-admit RFC 1918 under
`IANASpecialUse()` — those tables outrank `Allow`, and `AllowLoopback()` is the
only derivation that pierces them — so an on-premises policy is built as
`surfguard.Policy{}.AllowAllPorts().Allow(...)` instead.

### Device and Token Endpoint Address Policy (§16)

§16's SSRF requirement 6 — judging the address of the `token_endpoint` and
`device_authorization_endpoint` the selected metadata names, at connection
time, on every credential-bearing POST — is likewise implemented in Go only,
with the same policy and the same override shape as the issuer hop. The
per-SDK state, and the seam each SDK would need, so the follow-ups are
specified rather than rediscovered (tracked in #818; per-SDK
#814/#815/#816/#817):

| SDK | Device-authorization and token endpoint POSTs |
|-----|-----------------------------------------------|
| Go | `oauth.DefaultIssuerPolicy()` on the device flow's and `Exchanger`'s default client, shared with the issuer hop. Refused as `api_error`, non-retryable; the poll loop terminates on the first attempt. Overrides: `WithDevicePolicy` / `WithDeviceHTTPClient`; `WithExchangerPolicy` / a non-nil `NewExchanger` client. A caller-supplied client is the caller's, enforcement included |
| Ruby | Scheme gate, bounded timeout, suppressed redirects, bounded body. The `surfguard` gem (the shared classification tables, resolve-only by design) exists; enforcement would mean pinning a resolved address into `Net::HTTP#ipaddr=` under the default `Fetcher` transport, which the injected-Faraday lane cannot do |
| TypeScript | Scheme gate, bounded timeout, `redirect: "manual"`, bounded body. No classification tables in the ecosystem; the seam is an undici `Agent` with a `connect.lookup` hook, which the SDK's global-`fetch` contract does not reach |
| Python | Scheme gate, bounded timeout, `follow_redirects=False` (httpx default), bounded body. No classification tables; the seam is a custom `httpx` transport over a resolving `httpcore` backend |
| Kotlin | Scheme gate, bounded timeout, `followRedirects = false`, bounded body. No classification tables; the JVM seam is OkHttp's `Dns` interface (OkHttp connects to exactly the addresses it returns, so filtering there is connect-time judgement), with no multiplatform equivalent |
| Swift | Not applicable — ships no OAuth device flow or discovery |

Four things now hold in every SDK with an exchange path, policy or not, and
are not what this divergence is about: the scheme gate, the bounded body, the
bounded timeout, and redirect suppression. The last two were the exchange
path's own divergence — the same cross-SDK shape #805 had on the download's
signed hop before #809 closed it there — until §16 "Token-Endpoint Transport
Policy" closed it here: every token-exchange/refresh POST refuses the five
redirect statuses with a typed `api_error` carrying the real status, and
every default lane runs under the shared 30 s default / 3600 s ceiling
(Go's `AuthManager` refresh keeps its operator-configured client's timeout
and refuses redirects like every other credential POST).

One earlier claim in this section deserves a retraction rather than a silent
edit: previous revisions said Kotlin's `exchangeCode`/`refreshToken` followed
redirects, reasoned from the absence of `followRedirects = false` in the
source. They never followed — Ktor's `HttpRedirect` plugin defaults
`checkHttpMethod = true` (only GET/HEAD follow) and the CIO engine does not
follow at engine level. Kotlin's real defects were adjacent: a 3xx was thrown
through the generic error branch as `BasecampException.Auth` with the real
status lost, the default exchange client carried no `HttpTimeout`, and an
injected client was used verbatim, leaving redirect behavior to the engine
the caller happened to pick. All three are closed by the same policy above.

The behavioral tightening is the same as the issuer hop's, and it reaches one
more consumer shape: a Go caller that hand-configures a loopback or RFC 1918
authorization server and relied on `NewExchanger(nil)` or a device-flow call
with no `WithDeviceHTTPClient` now needs `WithExchangerPolicy` /
`WithDevicePolicy` (with `AllowLoopback()` for local development), or passes
`http.DefaultClient` to restore the old behavior outright. A caller that
already passes its own client — `basecamp-cli` passes its general-purpose
client to all three entry points — sees no change, and is also not protected
by this: the policy lives in the transport, and that consumer owns its
transport.

The same boundary holds one function further out, and is worth recording so
nobody infers otherwise: Go's `AuthManager.refreshLocked` posts a stored
refresh token to `Credentials.TokenEndpoint` on the client `NewAuthManager`
was handed, and the documented device-login bridge stores
`result.Config.TokenEndpoint` — a discovered endpoint — into those
credentials. Later automatic refreshes therefore do NOT receive the new
default policy; the endpoint re-enters the SDK as caller configuration on a
caller-owned client, and the enforcement is that client's to compose (§16
requirement 6).

### Retry Strategy (§7)

| SDK | Retry behavior |
|-----|---------------|
| TypeScript | Three-gate: POST retries only when `idempotent: true`. Retries on `retry_on` set from metadata to each operation's declared `max`, with the retry loop beneath the openapi-fetch middleware chain as the client's custom `fetch`. Network errors retry under the same idempotency gate; caller aborts and request timeouts are terminal. |
| Kotlin | Three-gate for both HTTP status and network-error retries: POST retries only when `idempotent: true`, full exponential backoff. Network errors retry through the same eligibility gate; the whole-request timeout (Ktor's `HttpRequestTimeoutException`) is deliberately not retried. |
| Go | Generated operation path retries operations classified idempotent at generation time — GET/HEAD by method, plus any operation carrying `x-basecamp-idempotent` (naturally-idempotent PUT/DELETE mutations like `UpdateProject`/`TrashProject`, and flagged-idempotent POSTs like `CompleteTodo`) — with exponential backoff; non-idempotent operations (e.g. `CreateTodo`) are single-attempt. The separate hand-written `doRequestURL` helper remains GET-only for ordinary retries, with a mutation-specific single re-attempt after successful 401 token refresh. Both paths run on the same caller cap: `pkg/basecamp` passes `WithMaxRetries` through to the generated client as a `RetryConfig`, with `BaseDelay` clamped at the §7 backoff ceiling (#718). Until it did, the sentence below this table was false for Go's **retry-eligible** typed operations — they used `min(3, op_max)`, the generated default, whatever the caller asked for. Non-idempotent operations were never affected: `doWithRetry` forces one attempt for them before it reads the config at all. |
| Ruby | Simplified: only GET retries. All non-GET methods never retry. Governed GETs gate status retries on the declared `retryOn` and bound attempts by `min(config.max_retries, operation max)`; ungoverned traffic (no operation ID: `get_absolute`, OAuth) keeps the taxonomy-driven status contract, under the same floored caller cap. |
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

Counts are of accessors actually wired onto the client, against §5's canonical
roster. The Kotlin and Swift rows are marked, because §5's roster is derived from
exactly those two files and a restatement of a gated value has to be gated too.
Of the other four, Python's, Ruby's and TypeScript's are each held by that SDK's
own accessor-roster test, which derives the roster from its generated services
directory and fails both when an accessor is missing and when one outlives its
service; Go's is read by `make check-service-inventory-parity` — including the
three carve-outs its row states, which that gate fails if they stop applying.
The parity gate reads the generated service directories, which is what each
generator emitted rather than what the client exposes: reachability is the axis
the per-SDK tests add, and it is per-SDK by nature, the thing checked being that
SDK's own hand-written file.

| SDK | Account-scoped services |
|-----|------------------------|
| Swift | `53` — full canonical set (`AccountClient+Services.swift`, generated; one of §5's two sources) <!-- @service-count --> |
| Kotlin | `53` — full canonical set (`ServiceAccessors.kt`, generated; §5's other source). Six accessors expose handwritten composites that subclass their generated service — `cards`, `documents`, `schedules`, `todolists`, `todos`, `uploads`, per the generator's `HAND_WRITTEN_SERVICES` — and the rest are the generated classes directly. The accessor set is identical either way, which is why §5 derives from this file regardless <!-- @service-count --> |
| Ruby | 53 — full canonical set. Held by its own accessor-roster test (`ruby/test/basecamp/accessor_inventory_test.rb`, added in #755) deriving the roster from `lib/basecamp/generated/services/`, so the next unwired service fails rather than going unnoticed. The five hand-written composites are `prepend`ed onto their generated classes rather than subclassing them, so every accessor's class is the generated constant exactly. |
| TypeScript | 53 — full canonical set, on the flat client alongside `authorization` (no `AccountClient` tier; see Client Topology above). Held by its own accessor-roster tests (`typescript/tests/accessor-inventory.test.ts` and `tests/types/accessor-inventory.test-d.ts`, added in #755) deriving the roster from `src/generated/services/`. Four hand-maintained renderings, so two instruments: the imports and `defineService` calls are resolved on a constructed client, the `index.ts` export blocks get their own assertion (a missing export is invisible at runtime to an in-repo importer and only bites a consumer), and the `BasecampClient` interface is asserted type-level, the factory returning `client as BasecampClient` so no runtime check can see it. Six accessors expose hand-written composites that subclass their generated service, which the class assertions allow for. |
| Go | 51 accessors. Two services are folded rather than missing: `automation`'s sole operation is `LineupService.ListMarkers`, and `clientVisibility`'s is `RecordingsService.SetClientVisibility`. `timesheets` is spelled `Timesheet` (singular). Capability is 53/53; the surface is not. Hand-written service wrappers around the generated OpenAPI client — not fully generated. |
| Python | 53 — full canonical set. `gauges` and `my_notifications` were a wiring gap rather than a fold, and were wired in #732; the same change added an accessor-inventory test (`python/tests/test_client.py`) deriving its roster from `generated/services/`, so the next unwired service fails rather than going unnoticed. Sync and async agree exactly. |

No row rests on a dated hand-verification any more. The 2026-08-13 sweep of the
accessor declarations (Go `AccountClient` methods, Ruby `Client#for_account`
accessors, TS `defineService` calls) was the backstop for Ruby and TypeScript,
and #755 retired it: both rosters are now re-derived from their generated
directories on every `make rb-check` and `make ts-check`, as #732 did for Python
on `make py-check`. A dated number is a constant that rots, and each of these
six counts is now restated by something that recomputes it.

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
