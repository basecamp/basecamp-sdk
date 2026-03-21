# Basecamp SDK — Natural Language Specification

## §0. Preamble

### Audience

This document is a complete, implementation-grade specification for building a Basecamp API SDK in any programming language. The primary audience is coding agents and developers who need to implement a new language SDK.

### Existing SDKs as Exemplars

Five shipping SDKs live alongside this spec in the same repository: Go, Ruby, TypeScript, Kotlin, and Swift. Use them as reference implementations when the spec leaves room for interpretation. TypeScript (`typescript/src/client.ts`) is the most complete single-file reference for auth, retry, pagination, and caching. Ruby (`ruby/lib/basecamp/http.rb`) has the most explicit pagination variants. Go (`go/pkg/basecamp/`) demonstrates the hand-written service wrapper pattern. When in doubt, read the code — the spec prescribes the contract, the SDKs show how it's been realized.

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
2. **Shipping SDK code** (consensus of Go, Ruby, TypeScript, Kotlin, Swift) — implementation truth. When 4+ SDKs agree, that's the contract.
3. **`behavior-model.json`** — machine-readable metadata. Descriptive of retry/idempotency semantics, but the retry block alone does not activate retry for POST (see §7).
4. **`rubric-audit.json`** — audit snapshot. Known to drift (e.g., 3C.3 claims 1024 chars; all 5 SDKs use 500). Trust code over audit.
5. **RUBRIC.md** — evaluation framework (external governance reference in the `basecamp/sdk` repo, not this repo). Defines criteria, not implementations. Referenced by criteria IDs (e.g., 2A.3, 3C.1) but not as an input artifact — this spec is self-contained.

`[CONFLICT]` annotations appear inline where sources disagree, with resolution rationale.

---

## §1. Architecture Overview

### Component Responsibilities

| Component | Responsibility |
|-----------|---------------|
| **Config** | Holds validated configuration: base URL, timeouts, retry params, pagination caps. May support env-var override (see §2). |
| **Client** | Top-level entry point. Enforces exactly-one-of auth. Owns account-independent services (authorization). |
| **AccountClient** | Account-scoped facade. Prepends `/{accountId}` to paths. Owns all 40 account-scoped services. |
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
    ├── ... (38 more services)
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
4. Validate `max_retries ≥ 1`. → `⊥ BasecampError(code: "usage")` otherwise. (`max_retries` is total attempts including the initial request; 0 would mean no request is made.) **Divergence:** Ruby and Go currently accept `max_retries = 0`; the spec prescribes `≥ 1` as the intended contract.
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
  1. If path starts with "https://" → return path unchanged.
  2. If path starts with "http://":
     a. If it is a localhost URL (see §9) → return path unchanged. (Absolute localhost URLs from pagination follow-ups are preserved as-is.)
     b. Else → ⊥ BasecampError(code: "usage", message: "URL must use HTTPS").
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

Go and Ruby support automatic token refresh via a richer provider interface. TypeScript, Kotlin, and Swift delegate refresh to the caller (the async function can internally handle refresh logic).

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
- `refresh()` POSTs to the token URL with `grant_type=refresh_token`.

### 401 Refresh-and-Retry Algorithm

1. Receive 401 response.
2. If the token provider supports refresh (`refreshable() == true`) and refresh has not yet been attempted for this request:
   a. Call `refresh()`.
   b. If refresh succeeded, retry the request once with updated token.
   c. → response from retry.
3. → `⊥ BasecampError(code: "auth_required", http_status: 401)`.

Refresh is attempted at most once per request. Implementations track this with a boolean (e.g., `refresh_attempted`) rather than a counter.

---

## §5. Service Surface

### Client-Level Services (account-independent)

- **authorization** — identity lookup and account listing via Launchpad (OAuth utility functions like PKCE, state generation, and discovery are standalone helpers in §16, not service methods)

### AccountClient-Level Services (account-scoped) — 40 services

attachments, automation, boosts, campfires, cardColumns, cardSteps, cardTables, cards, checkins, clientApprovals, clientCorrespondences, clientReplies, clientVisibility, comments, documents, events, forwards, hillCharts, lineup, messageBoards, messageTypes, messages, people, projects, recordings, reports, schedules, search, subscriptions, templates, timeline, timesheets, todolistGroups, todolists, todos, todosets, tools, uploads, vaults, webhooks

**Total surface:** 1 client-level + 40 account-scoped = 41 services.

### Derivation Rule `[static]`

The OpenAPI spec uses 12 coarse tags (e.g., `Automation`, `Todos`, `Files`). The service generators split these into 40 fine-grained services using a two-table mapping: `TAG_TO_SERVICE` (tag → default service name) and `SERVICE_SPLITS` (tag → {service → [operationIds]}). For example, the `Todos` tag splits into `Todos`, `Todolists`, `Todosets`, `TodolistGroups`; the `Files` tag splits into `Attachments`, `Uploads`, `Vaults`, `Documents`. These mappings are defined in each language's generator script and produce identical service sets across SDKs.

### Known Gaps (informational, not prescriptive)

- Go is missing a standalone `automation` service; `clientVisibility` is implemented on `RecordingsService` (not a separate service); uses singular `Timesheet` vs `timesheets`
- TypeScript flattens both tiers onto a single client object (no separate AccountClient exposed to consumers) — a valid language adaptation
- Ruby returns lazy `Enumerator` for pagination rather than `ListResult`

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

Status-mapped codes are verified per the Verification column. Most are `[conformance]`-verified; 400→`validation` is `[static]` (no conformance test). Client-side codes (`usage`, `network`, `ambiguous`) and exit codes are `[static]`.

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
| `validation` | 9 | 400 | false | Request validation failed | `[static]` |

### HTTP Status Mapping Algorithm

Each mapping below is `[conformance]`-verified except step 5 (400 → `validation`) which is `[static]`.

Given an HTTP response with status code `status` and body `body`:

1. If `status == 401` → `BasecampError(code: "auth_required", http_status: 401, retryable: false)`.
2. If `status == 403` → `BasecampError(code: "forbidden", http_status: 403, retryable: false)`.
3. If `status == 404` → `BasecampError(code: "not_found", http_status: 404, retryable: false)`.
4. If `status == 429` → `BasecampError(code: "rate_limit", http_status: 429, retryable: true, retry_after: parseRetryAfter(headers))`.
5. If `status == 400` → `BasecampError(code: "validation", http_status: 400, retryable: false)`. `[CONFLICT: Go currently maps 400 to "api_error" (falls through to default case). The spec prescribes "validation" to match other SDKs. No conformance test exists for 400 specifically.]` `[static]`
6. If `status == 422` → `BasecampError(code: "validation", http_status: 422, retryable: false)`.
7. If `status == 500` → `BasecampError(code: "api_error", http_status: 500, retryable: true)`.
8. If `status == 502` → `BasecampError(code: "api_error", http_status: 502, retryable: true)`.
9. If `status == 503` → `BasecampError(code: "api_error", http_status: 503, retryable: true)`.
10. If `status == 504` → `BasecampError(code: "api_error", http_status: 504, retryable: true)`.
11. If `status >= 500` → `BasecampError(code: "api_error", http_status: status, retryable: true)`.
12. Otherwise → `BasecampError(code: "api_error", http_status: status, retryable: false)`.

In all cases, extract `request_id` from `X-Request-Id` response header if present. `[conformance]`

### Error Body Parsing Algorithm

1. Attempt to parse `body` as JSON.
2. If JSON and has `"error"` key (string value) → use as `message`.
3. If JSON and has `"error_description"` key (string value) → use as `hint`.
4. Else if JSON and has `"message"` key (string value) and `message` not yet set → use as `message`.
5. If parsing fails or body is empty → use HTTP status text as `message`.
6. Truncate `message` to `MAX_ERROR_MESSAGE_LENGTH` (see §9).

Note: `"error"` takes precedence over `"message"` — step 4 is a fallback for APIs that use `"message"` instead of `"error"`.

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

- **HTTP status retry:** Response status is in the transport's retryable set. The `behavior-model.json` specifies `retry_on: [429, 503]` for all operations. Implementations may expand this set to include other 5xx statuses (500, 502, 504).
- **Network error retry:** Connection failures, timeouts, and DNS errors (no HTTP response received) are retryable. These correspond to `BasecampError(code: "network", retryable: true)` in §6.

**Non-retryable statuses (never retry regardless of method):** 401, 403, 404, 400, 422.

### Cross-SDK Divergence `[CONFLICT]`

- **TypeScript** implements the three-gate algorithm but chains at most 1 retry — on a retryable status, TS returns `fetch(retryRequest)` which bypasses middleware after the first retry (waiver 2B.1 in `rubric-audit.json`). **Kotlin** implements the three-gate algorithm for HTTP status retries (POST retries only when `idempotent: true`, full exponential backoff) but does not retry on network errors — transport exceptions are returned immediately as `BasecampException.Network`.
- **Go** is stricter: only GET retries with exponential backoff; all non-GET methods make a single attempt (plus one re-attempt after successful 401 token refresh). No idempotency gate.
- **Ruby** is stricter: only GET retries; all non-GET methods do not retry. Go and Ruby are acceptably conservative.
- **Swift** currently over-retries: generated create methods pass retry config directly, and the transport retries any request whose status matches `retry_on` — no idempotency gate. Non-idempotent POSTs like `CreateProject` are retried. This is a known bug.
- The spec prescribes the three-gate algorithm. **TS note:** TS retry returns `fetch(retryRequest)` which bypasses middleware after the first retry, so TS effectively caps at 1 retry per request regardless of `max_attempts`. This is a known limitation (waiver 2B.1).

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
     j. Invoke hooks.on_retry(RequestInfo{method, url, attempt+1}, attempt+1, retry_error, delay).
        -- The standalone `attempt` parameter is always attempt+1 (1-based: 1 = first
        -- retry, 2 = second retry). RequestInfo.attempt varies by SDK: Go/Ruby/Kotlin
        -- set it to the failed attempt (attempt+1 minus 1); TS sets it to attempt+1.
        -- Consumers should use the standalone parameter for retry counting.
     k. Sleep delay ms.
     l. Refresh auth headers (token may have been refreshed during sleep).
END
```

The loop always terminates via step 3e (raise on network error), 3f (return non-retryable response), or 3g (return on exhaustion). `on_request_start`/`on_request_end` are invoked per attempt within the loop; `on_operation_start`/`on_operation_end` are invoked by the calling layer (generated service method), not by the retry transport.

### Backoff Formula

```
delay = base_delay_ms * 2^(retry_index) + random(0, max_jitter)
```

Where `retry_index` is the 0-indexed retry count (first retry = 0, second retry = 1, etc.). In the `executeWithRetry` loop, `retry_index = attempt` — when the initial request (attempt=0) fails and reaches step 3h, it computes the delay for the first retry using `2^0 = 1×base_delay_ms`. Default constants (from `retry_config` or Config):
- `base_delay_ms` = 1000 (from `retry_config.base_delay_ms`)
- `max_jitter` = 100ms (from Config; not part of `retry_config` — sourced from the client's Config RECORD)

Retry-After header value takes precedence when present and valid.

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

All 181 operations in `behavior-model.json` use `retry_on: [429, 503]`. Three `(max, base_delay_ms)` patterns exist:
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
  truncated   : Boolean   -- true if results were capped by max_pages or max_items
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
- `::1` — Go, Ruby, TypeScript (Swift requires bracket-wrapped URL form `[::1]`; Kotlin does not recognize either IPv6 form)
- `[::1]` (bracket-wrapped IPv6) — Go, Ruby, TypeScript, Swift
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

`[CONFLICT: rubric-audit.json 3C.3 says 1024; all 5 SDKs use 500. Code wins.]`

Error messages extracted from response bodies are truncated to 500 units. If the string exceeds the limit, the last 3 units are replaced with `"..."`, so the result is at most 500 units long.

**Unit semantics:** The spec prescribes 500 **bytes**. Go (`len()`) and Ruby (`bytesize`) use bytes. TypeScript (`s.length`), Swift (`s.count`), and Kotlin (`s.length`) use character/code-unit length, which coincides with bytes for ASCII. Conformance test fixtures use ASCII error bodies today; Unicode truncation semantics are a per-language divergence documented in Appendix F.

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

`[CONFLICT: JavaScript Number.MAX_SAFE_INTEGER is 2^53 - 1. The TypeScript SDK has a documented known gap — JSON.parse truncates integers beyond this value. The spec prescribes 64-bit precision; TypeScript implementations must document the limitation. See waiver 1B.6 in rubric-audit.json.]`

### Date/Time Fields `[static]`

Fields declared with `format: date-time` in the OpenAPI spec use ISO 8601 format. Implementations may use the language's native date/time type (Go `time.Time`, Ruby `Time`, Kotlin `Instant`) or keep them as ISO 8601 strings (TypeScript uses `string` from openapi-fetch schema types). The choice is a language adaptation.

### Optional Fields `[static]`

Fields not listed in the `required` array of the OpenAPI schema must be nullable or optional in the language's type system. Sentinel values (empty string, 0, etc.) are not acceptable substitutes for absence.

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

Status code 400 must also NOT be retried. This is a `[static]` contract (no dedicated conformance test exists for 400).

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
  operation     : String     -- e.g., "List", "Get", "Create"
  resource_type : String     -- e.g., "todo", "project"
  is_mutation   : Boolean    -- true for POST, PUT, DELETE
  project_id    : Integer?   -- if operation is project-scoped (Go omits this field)
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
| `Content-Type` | `application/json` (for requests with a body; preserve if already set for binary uploads). TypeScript and Go set it unconditionally; Swift and Kotlin set it only when a body is present. Either approach is acceptable. | JSON API requests only (not download Hop 1) | `[conformance]` |

Where:
- `{lang}` is the language identifier: `go`, `ts`, `ruby`, `kotlin`, `swift`
- `{VERSION}` is the SDK version (e.g., `0.6.0`)
- `{API_VERSION}` is the API version from `openapi.json` `info.version` (currently `2026-01-26`)

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
  3. Hop 1 — Authenticated API GET:
     a. Set Authorization and User-Agent headers only (no Accept or Content-Type — this is a binary download, not a JSON API call).
     b. Fetch with redirect: manual (do not follow redirects automatically).
     c. If response is redirect (301, 302, 303, 307, 308):
        - Extract Location header. ⊥ if absent.
        - Resolve Location against rewritten URL (handle relative redirects).
        - Proceed to Hop 2.
     d. If response is 2xx:
        - Direct download (no second hop needed).
        - → DownloadResult from response body.
     e. If response is error → ⊥ BasecampError from response.

  4. Hop 2 — Unauthenticated fetch (signed URL):
     a. Fetch Location URL with NO auth headers.
     b. If not 2xx → ⊥ BasecampError.
     c. → DownloadResult from response body.
END
```

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
  handlers : Map<GlobPattern, Handler>
  dedup    : LRU<String, Boolean>   -- window of 1000 entries, keyed by event ID (payload `id` field)
    -- Matches shipped Go, Ruby, and TS implementations.
  secret   : String

  receive(payload, signature) →
    1. Verify signature. If invalid → reject.
    2. Parse payload, extract event_id from the event's `id` field.
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

### RFC 8414 Discovery

```
FUNCTION discoverOAuthEndpoints(issuer: String) → OAuthEndpoints
  1. Fetch issuer + "/.well-known/oauth-authorization-server". (Basecamp's Launchpad issuer is at the origin root; RFC 8414 path-segment rules do not apply.)
  2. Parse JSON response.
  3. Extract authorization_endpoint, token_endpoint.
  4. → OAuthEndpoints
END
```

### Launchpad Legacy Format

The Basecamp Launchpad OAuth endpoints use a mix of standard and legacy parameters:

- Authorization URL: standard `response_type=code`
- Token exchange: `type=web_server` parameter (in addition to or instead of `grant_type=authorization_code`)
- Token refresh: `type=refresh` parameter (in addition to or instead of `grant_type=refresh_token`)

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

Enumerated from `conformance/schema.json`:

| Type | Description |
|------|-------------|
| `requestCount` | Number of HTTP requests made (verifies retry behavior) |
| `delayBetweenRequests` | Minimum delay between requests in ms (verifies backoff) |
| `statusCode` | HTTP status code of the response |
| `responseStatus` | Response status category |
| `responseBody` | Specific value in response body (by path) |
| `headerPresent` | Named header exists on request |
| `headerValue` | Named header has specific value |
| `errorType` | Error type classification |
| `noError` | Operation completed without error |
| `requestPath` | URL path of the outgoing request |
| `errorCode` | Error code in structured error |
| `errorMessage` | Error message text |
| `errorField` | Specific field value on the error object |
| `headerInjected` | Header was injected with specific value |
| `requestScheme` | URL scheme (http/https) of request |
| `urlOrigin` | Origin validation result (accepted/rejected) |
| `responseMeta` | Metadata on paginated response (totalCount, truncated) |

### Test Categories and Owning Sections

| Category | Files | Owning Spec Section(s) |
|----------|-------|----------------------|
| auth | `auth.json` | §4 Authentication, §13 HTTP Transport |
| error-mapping | `error-mapping.json` | §6 Error Taxonomy |
| idempotency | `idempotency.json` | §7 Retry (Gate 2) |
| integer-precision | `integer-precision.json` | §10 Type Fidelity |
| pagination | `pagination.json` | §8 Pagination |
| paths | `paths.json` | §3 Client Architecture (account path construction) |
| retry | `retry.json` | §7 Retry |
| security | `security.json` | §9 Security |
| status-codes | `status-codes.json` | §11 Response Semantics |

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

All conformance tests should pass. Runners currently pass with documented waivers covering: retry depth (TS single-chained retry, waiver 2B.1), integer precision (TS Number, waiver 1B.6), retry scope (Ruby only-GET retry), and pagination metadata (Ruby Enumerator lacks totalCount/truncated/maxItems, waivers 2C.2/2C.4/2C.6). Waivers are documented in each runner's skip list and in `rubric-audit.json` with language-specific rationale.

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
| 9 | 1A.6 | No hand-written API methods (multi-language only; Go uses hand-written service wrappers around generated client — see Appendix F) | §18 | `[manual]` |
| 10 | 4A.1 | Smithy → OpenAPI freshness check | §21 | `[static]` |

---

## §21. Verification Gates

### Enforced by `make check`

| Target | What it verifies |
|--------|-----------------|
| `smithy-check` | `openapi.json` matches Smithy rebuild |
| `behavior-model-check` | `behavior-model.json` matches regeneration |
| `provenance-check` | Embedded provenance matches `spec/api-provenance.json` |
| `sync-api-version-check` | `API_VERSION` constants match `openapi.json` `info.version` across all SDKs |
| `go-check-drift` | Go generated services match current OpenAPI spec |
| `kt-check-drift` | Kotlin generated services match current OpenAPI spec |
| `go-check` | Go: lint + test |
| `ts-check` | TypeScript: typecheck + test |
| `rb-check` | Ruby: test + rubocop |
| `kt-check` | Kotlin: build + test |
| `swift-check` | Swift: build + test |
| `conformance` | All conformance test categories pass with documented waivers (go, kotlin, typescript, ruby runners) |

Full dependency chain: `check: smithy-check behavior-model-check provenance-check sync-api-version-check go-check-drift kt-check-drift go-check ts-check rb-check kt-check swift-check conformance`

### Advisory (not in `make check` today)

| Target | Status |
|--------|--------|
| `url-routes-check` | Exists as Makefile target but not wired into `check` |
| TS/Ruby/Swift drift checks | Not yet implemented (only Go and Kotlin have them) |
| `audit-check` | Defined in the Makefile convention (external governance reference in `basecamp/sdk` `MAKEFILE-CONVENTION.md`) but no target exists in this repo's Makefile |

---

## §22. Out of Scope

The following are explicitly NOT part of this specification:

- GraphQL, WebSocket, or SSE transport
- CLI UI or interactive prompts
- Circuit breaker, bulkhead, or client-side rate limiter (rubric T2D criteria exist but are optional extras, not core contracts)
- Prometheus or OpenTelemetry hook implementations (the hook protocol is in scope; specific integrations are not)
- Package publishing or release automation
- Language-specific async/concurrency model (spec is synchronous-first; async is a language adaptation)
- Smithy model authoring
- File upload multipart encoding details
- Webhook receiver HTTP server implementation (the verification algorithm is in scope; how to run an HTTP server is not)

---

## Appendix A: Constants Reference

All magic numbers in one place, derived from shipping SDK code (not `rubric-audit.json`).

| Constant | Value | Unit | Source |
|----------|-------|------|--------|
| `MAX_RESPONSE_BODY_BYTES` | 52,428,800 (50 MiB) | bytes | `go/pkg/basecamp/security.go`, `ruby/lib/basecamp/security.rb`; Go/Ruby enforce; TS/Kotlin/Swift do not |
| `MAX_ERROR_BODY_BYTES` | 1,048,576 (1 MiB) | bytes | `go/pkg/basecamp/security.go`, `ruby/lib/basecamp/security.rb` |
| `MAX_ERROR_MESSAGE_LENGTH` | 500 | bytes (Go/Ruby) or code units (TS/Swift/Kotlin) | All 5 SDKs |
| `DEFAULT_BASE_URL` | `https://3.basecampapi.com` | — | All 5 SDKs |
| `DEFAULT_TIMEOUT` | 30 | seconds | All 5 SDKs |
| `DEFAULT_CONNECT_TIMEOUT` | 10 | seconds | `ruby/lib/basecamp/http.rb` (Faraday open_timeout); recommended default, not a required config field |
| `DEFAULT_MAX_RETRIES` | 3 | — | All 5 SDKs |
| `DEFAULT_BASE_DELAY` | 1000 | milliseconds | All 5 SDKs |
| `DEFAULT_MAX_JITTER` | 100 | milliseconds | All 5 SDKs |
| `DEFAULT_MAX_PAGES` | 10,000 | — | All 5 SDKs |
| `MAX_CACHE_ENTRIES` | 1000 | entries | `typescript/src/client.ts` |
| `MAX_TOKEN_HASH_ENTRIES` | 100 | entries | `typescript/src/client.ts` |
| `API_VERSION` | `2026-01-26` | — | `openapi.json` `info.version` |
| `TOKEN_REFRESH_BUFFER` | 300 | seconds | Go OAuth token refresh threshold (5-minute buffer); Ruby refreshes only on expiry (no buffer); TS/Kotlin/Swift delegate expiry to caller |

---

## Appendix B: Canonical Service Surface

Repeated from §5 for quick reference.

**Client-level (1):** authorization

**AccountClient-level (40):**
attachments, automation, boosts, campfires, cardColumns, cardSteps, cardTables, cards, checkins, clientApprovals, clientCorrespondences, clientReplies, clientVisibility, comments, documents, events, forwards, hillCharts, lineup, messageBoards, messageTypes, messages, people, projects, recordings, reports, schedules, search, subscriptions, templates, timeline, timesheets, todolistGroups, todolists, todos, todosets, tools, uploads, vaults, webhooks

---

## Appendix C: Rubric Criteria Cross-Reference

| Rubric ID | Spec Section | Summary |
|-----------|-------------|---------|
| 1A.1 | §18, §21 | Smithy model validates |
| 1A.2 | §18, §21 | OpenAPI derived from Smithy |
| 1A.6 | §18 | No hand-written API methods |
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
| `error-mapping.json` | 422 → validation | §6 |
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
| `status-codes.json` | GET → 200 | §11 |
| `status-codes.json` | PUT → 200 | §11 |
| `status-codes.json` | POST create → 201 | §11 |
| `status-codes.json` | DELETE → 204 | §11 |
| `status-codes.json` | 4xx/5xx surfaced as errors | §11 |
| `status-codes.json` | Non-retryable not retried | §7, §11 |
| `integer-precision.json` | Large integer IDs preserved | §10 |
| `paths.json` | Path construction | §3 |

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

The `redaction` and `sensitiveTypes` sections are used for PII handling and are not part of the retry/idempotency contract. They are omitted from the detailed schema above for brevity.

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

- Total operations: 181
- Idempotent: 55 (flagged with `idempotent: true`)
- Non-idempotent: 126 (no `idempotent` field, or not present)
- All operations use `retry_on: [429, 503]`

---

## Appendix F: Known Cross-SDK Divergences

### Retry Strategy (§7)

| SDK | Retry behavior |
|-----|---------------|
| TypeScript | Three-gate: POST retries only when `idempotent: true`. Retries on `retry_on` set from metadata. Chains at most 1 retry via `fetch(retryRequest)` which bypasses middleware (waiver 2B.1). |
| Kotlin | Three-gate for HTTP status retries: POST retries only when `idempotent: true`, full exponential backoff. Does not retry network errors (transport exceptions returned immediately). |
| Go | Simplified: only GET retries with exponential backoff. All non-GET methods do not retry (single attempt, plus one re-attempt after successful 401 token refresh). |
| Ruby | Simplified: only GET retries. All non-GET methods never retry. Ruby retries on any error with `retryable? == true`. |
| Swift | Over-retries: generated create methods pass retry config directly. No idempotency gate. Known bug. |

### Integer Precision (§10)

| SDK | Precision |
|-----|----------|
| Go | Full 64-bit (`int64`) |
| Ruby | Full arbitrary precision (Ruby Integer) |
| Kotlin | Full 64-bit (`Long`) |
| Swift | Platform-width `Int` (64-bit on all supported platforms). Generated models use `Int`, not `Int64`. |
| TypeScript | 53-bit (`Number`). IDs > 2^53 - 1 lose precision. Documented known gap with waiver 1B.6. |

### Pagination Metadata (§8)

| SDK | ListResult | total_count | truncated |
|-----|-----------|------------|-----------|
| TypeScript | `ListResult<T>` extends Array | yes | yes |
| Kotlin | `ListResult<T>` | yes | yes |
| Swift | `ListResult<T>` | yes | yes |
| Go | Typed `*XxxListResult` with `Meta ListMeta` | yes | yes |
| Ruby | Lazy `Enumerator` yielding items | no (waiver 2C.2) | no (waiver 2C.6) |

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
| Swift | 40 (full canonical set) |
| TypeScript | 40 (full canonical set) |
| Kotlin | 40 (full canonical set) |
| Ruby | 40 (full canonical set) |
| Go | 38 as standalone services (missing standalone `automation`; `clientVisibility` ops exist on `RecordingsService` rather than as a separate service). Hand-written service wrappers around generated OpenAPI client — not fully generated. |
