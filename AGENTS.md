# Basecamp SDK Agent Guidelines

## Current Status

| Component | Status | Details |
|-----------|--------|---------|
| **Smithy Spec** | 175 operations | Single source of truth for all APIs |
| **Go SDK** | Production-ready | Full generated client + service wrappers |
| **TypeScript SDK** | Production-ready | 37 generated services, openapi-fetch based |
| **Ruby SDK** | Production-ready | 37 generated services |
| **Swift SDK** | Production-ready | 38 generated services, URLSession-based |
| **Kotlin SDK** | Production-ready | 38 generated services, Ktor/KMP-based |
| **Python SDK** | Production-ready | 40 generated services, httpx-based |

All six SDKs share the same architecture: **Smithy spec -> OpenAPI -> Generated services**. No hand-written API methods exist in any SDK runtime.

---

## Architecture

```
Smithy Spec → OpenAPI → Generated Client → Service Layer → User
```

| SDK | Generated Client | Service Layer |
|-----|-----------------|---------------|
| **Go** | `pkg/generated/client.gen.go` | `pkg/basecamp/*.go` (wraps generated client) |
| **TypeScript** | `openapi-fetch` + `schema.d.ts` | `src/generated/services/*.ts` |
| **Ruby** | HTTP client | `lib/basecamp/generated/services/*.rb` |
| **Swift** | `URLSession` via `Transport` protocol | `Sources/Basecamp/Generated/Services/*.swift` |
| **Kotlin** | Ktor via `BaseService` | `sdk/src/commonMain/kotlin/.../generated/services/*.kt` |
| **Python** | httpx via `HttpClient` | `src/basecamp/generated/services/*.py` |

All 175 operations across 38+ services are generated. Hand-written code is limited to infrastructure:

| Purpose | TypeScript | Ruby | Swift | Kotlin | Python |
|---------|-----------|------|-------|--------|--------|
| HTTP helpers, pagination, hooks | `src/services/base.ts` | `lib/basecamp/services/base_service.rb` | `Sources/Basecamp/Services/BaseService.swift` | `sdk/.../services/BaseService.kt` | `src/basecamp/generated/services/_base.py` |
| OAuth flows (not in OpenAPI spec) | `src/services/authorization.ts` | `lib/basecamp/services/authorization_service.rb` | — | `sdk/.../oauth/*.kt` | `src/basecamp/services/authorization.py` |

Other hand-written service files in `src/services/` (TS) and `lib/basecamp/services/` (Ruby) are NOT loaded at runtime. They exist only as reference implementations.

### Smithy Spec vs Actual API Responses

Smithy wrapper structures are a spec convention, not the API shape. The spec uses wrapper structures for list responses:

```smithy
structure ListAssignablePeopleOutput {
  people: PersonList
}
```

But the actual API returns top-level arrays. The Go code generator unwraps these:

```go
ListAssignablePeopleResponseContent = []Person
```

When verifying API response shapes, check Go generated code in `go/pkg/generated/client.gen.go` — look for `*ResponseContent` type definitions. Don't assume Smithy wrapper structures match the wire format.

**Why the wrappers exist:** Smithy's AWS restJson1 protocol requires list outputs to be wrapped structures because `@httpPayload` only supports string, blob, structure, union, and document types — not arrays directly. See the ARCHITECTURAL NOTE in `spec/basecamp.smithy`.

---

## Hard Rules

### Never Do These

1. **NEVER edit files under `*/generated/`** — they get overwritten by generators
2. **NEVER add hand-written service methods for API operations** — all API ops come from generators
3. **NEVER skip running `make smithy-build` after Smithy changes** — keeps OpenAPI in sync
4. **NEVER construct API paths manually** — use the generated client methods
5. **NEVER bypass the SDK** — no raw `client.Get()`, string-concatenated URLs, or internal method calls

If you're writing `fmt.Sprintf` with an API path, you're doing it wrong. If the generated client lacks functionality, fix the spec and regenerate — don't work around it.

### Anti-patterns

```go
// WRONG - Manual path construction
path := fmt.Sprintf("/buckets/%d/todolists/%d/todos.json", bucketID, todolistID)

// WRONG - Query parameter hacks
path := generatedPath + "?status=active"

// WRONG - "Just this once" shortcuts
path := fmt.Sprintf("/projects/%d/people.json", projectID)
```

### Correct Patterns

```go
// Single-resource: use generated client directly
resp, err := client.gen.GetTodoWithResponse(ctx, accountID, bucketID, todoID)

// Paginated: generated client for first page, Link headers for subsequent
resp, err := client.gen.ListTodosWithResponse(ctx, accountID, bucketID, todolistID, params)
nextURL := parseNextLink(resp.HTTPResponse.Header.Get("Link"))
for nextURL != "" {
    resp, err := client.Get(ctx, nextURL)  // URL from API, not constructed
    nextURL = parseNextLink(resp.Headers.Get("Link"))
}
```

```python
# Python — single resource
todo = account.todos.get(todo_id=123)

# Python — paginated (automatic)
todos = account.todos.list(todolist_id=456, status="active")

# WRONG — manual path construction
url = f"/buckets/{project_id}/todolists/{todolist_id}/todos.json"

# WRONG — bypassing the SDK
response = account.http.get(f"/{account_id}/buckets/{project_id}/todos.json")
```

### Andon Cord — Stop and Fix Immediately

Pull the andon cord when you see:

- **Compilation errors referencing generated types/methods that don't exist** — regenerate from spec, update hand-written code to match. Do NOT patch around missing types.
- **Operation count mismatches** — generators report different counts or wrong service groupings
- **Test fixtures that don't match generated types** — the spec has drifted
- **`make generate` fails or produces unexpected diffs** — investigate before proceeding
- **Script failures in the generation pipeline** — fix tooling before continuing feature work

---

## Smithy-First Development

All new API coverage starts in `spec/basecamp.smithy`. Before writing SDK code, add operations and shapes to the spec.

### Smithy Patterns

```smithy
/// Operation documentation
@http(method: "GET", uri: "/buckets/{projectId}/resources/{resourceId}.json")
operation GetResource {
  input: GetResourceInput
  output: GetResourceOutput
}

structure GetResourceInput {
  @required
  @httpLabel
  projectId: ProjectId

  @required
  @httpLabel
  resourceId: ResourceId
}

structure GetResourceOutput {
  resource: Resource
}
```

### Naming Conventions

- Operations: `Verb` + `Noun` (e.g., `ListTodos`, `GetProject`, `CreateMessage`, `TrashComment`)
- Input structures: `{OperationName}Input`
- Output structures: `{OperationName}Output`
- IDs: `{Resource}Id` as `long` type (e.g., `MessageId`, `CommentId`)
- Status enums: Use `@documentation` string with valid values (e.g., `"active|archived|trashed"`)

### Common URL Patterns

| Pattern | Example |
|---------|---------|
| Bucket-scoped | `/buckets/{projectId}/{resources}/{resourceId}.json` |
| Recording ops | `/buckets/{projectId}/recordings/{recordingId}/status/trashed.json` |
| Nested resources | `/buckets/{projectId}/recordings/{recordingId}/comments.json` |
| Account-level | `/reports/{reportType}.json` |

### URI Constraints

Smithy's `@http` URI labels cannot have literal suffixes in the same segment. `.json` is only valid after literal path segments:
- OK: `/{accountId}/projects.json`
- OK: `/{accountId}/buckets/{projectId}/todos/{todoId}`
- WRONG: `/{accountId}/buckets/{projectId}/boosts/{boostId}.json`

### Shape Reuse

Reuse these common shapes: `ProjectId`, `PersonId`, `ISO8601Timestamp`, `ISO8601Date`, `Person`, `TodoParent`/`RecordingParent`, `TodoBucket`/`RecordingBucket`.

### Reference Sources

- **BC3 API docs** (`~/Work/basecamp/bc3-api/sections/*.md`) — authoritative HTTP endpoint documentation
- **Go SDK** (`go/pkg/basecamp/*.go`) — existing operation signatures
- **Existing Smithy** (`spec/basecamp.smithy`) — established patterns and reusable types

---

## Generation Pipeline

After any Smithy spec change, run the full pipeline:

```
make smithy-build && make -C go generate && make url-routes && \
  make ts-generate && make ts-generate-services && \
  make rb-generate && make rb-generate-services && \
  make swift-generate && make kt-generate-services && \
  make py-generate
```

Or `make generate` if it cascades. Never commit a Smithy change without regenerating all downstream artifacts. Never assume "I'll regenerate later" — regenerate now, or the drift compounds.

### Invariants

1. **`openapi.json` must always reflect the current Smithy spec.** Run `make smithy-build` after any change to `spec/basecamp.smithy` or `spec/overlays/*.smithy`.
2. **Service generator mappings must stay current.** `typescript/scripts/generate-services.ts`, `ruby/scripts/generate-services.rb`, `kotlin/generator/.../Config.kt`, and `python/scripts/generate_services.py` all have hardcoded `TAG_TO_SERVICE` mappings. Update them for new/renamed/removed operations. Treat unmapped-operation warnings as errors.
3. **Tags in `spec/overlays/tags.smithy` control service grouping.** Every new operation needs a tag or it won't appear in any generated service.
4. **Hand-written Go service methods must use generated client types.** Field names, method signatures, and request/response body types come from `go/pkg/generated/client.gen.go`.

### Verification

When reviewing a PR that touches `spec/basecamp.smithy`, verify that `openapi.json` and all generated files are included in the diff.

---

## Release Procedure

Two commands cut a release. `make release` handles pushing `main`, tagging, and triggering all 7 workflows (Go, TypeScript, Ruby, Swift, Kotlin, Python, GitHub Release).

```bash
make bump VERSION=x.y.z   # updates 10 version files + lockfiles
# commit the bump
make release VERSION=x.y.z  # pushes main, tags, pushes tag
```

### What `make release` does

1. Verifies all 10 version constants match the requested version
2. Verifies the working tree is clean
3. Verifies you're on the `main` branch
4. Pushes `main` to origin (release workflows guard that the tag commit is reachable from `origin/main`)
5. Creates and pushes the `v{VERSION}` tag

### Guards

- **Branch guard**: refuses to release from non-`main` branches
- **Version guard**: refuses if any of the 10 constants don't match
- **Clean tree guard**: refuses if there are uncommitted changes
- **CI guard**: each release workflow runs `git merge-base --is-ancestor "$GITHUB_SHA" origin/main` — rejects tags whose commit isn't on `main`

### Verification

After releasing, monitor all 7 workflows in GitHub Actions. The "Create GitHub Release" workflow waits for the 6 SDK workflows to succeed before creating the release.

```bash
gh run list --repo basecamp/basecamp-sdk --limit 7 --json name,status,conclusion
```

---

## Upstream API Sync Workflow

When syncing the SDK spec to match upstream API changes (bc3-api docs + bc3 Rails app):

### Provenance is Mandatory

Every sync MUST update `spec/api-provenance.json` with the upstream HEADs:
```bash
gh api repos/basecamp/bc3-api/commits/HEAD --jq '.sha'
gh api repos/basecamp/bc3/commits/HEAD --jq '.sha'
```

Update both `revision` and `date` fields, then `make provenance-sync`. This is not optional — provenance tracks what the SDK is conformant to.

### Pre-sync

Use `make sync-status` to see upstream diffs since last sync.

### Sync Checklist

1. Update `spec/basecamp.smithy` — new operations, structures, field additions, path fixes
2. Update `spec/overlays/tags.smithy` — tag new operations
3. Update service generator `TAG_TO_SERVICE` mappings if adding new service groups
4. Run full generation pipeline
5. Wire new services into clients (`typescript/src/client.ts`, `typescript/src/index.ts`, `ruby/lib/basecamp/client.rb`, `python/src/basecamp/client.py`, `python/src/basecamp/async_client.py`)
6. Write tests for ALL new operations (see Completeness Bar below)
7. Update tests for any changed paths/signatures
8. Update provenance, run `make provenance-sync`
9. `make` must pass clean

---

## SDK Change Completeness Bar

`make` passing is necessary but not sufficient. A change that compiles but ships new operations without tests is incomplete.

### Every New Operation Requires

1. **Smithy spec** — operation, input/output structures, error list
2. **Tag** — in `spec/overlays/tags.smithy` for service grouping
3. **Generator mapping** — if introducing a new service group
4. **Client wiring** — import, type declaration, `defineService`/`service()` call, re-export from `index.ts`
5. **TypeScript test** — in `typescript/tests/services/<service>.test.ts` (happy path + error case)
6. **Ruby test** — in `ruby/test/basecamp/services/<service>_service_test.rb` (same coverage)
7. **Python test** — in `python/tests/services/test_<service>_service.py` (same coverage)
8. **Regeneration** — all generated artifacts freshly regenerated, not stale

### Every Changed Field/Path Requires

1. **Existing tests updated** — every test stubbing a changed path must be updated
2. **New field tests** — at least one test fixture should include new fields to verify they flow through

### Pre-Merge Verification

Run `make go-check-drift`, `make kt-check-drift`, and `make py-check-drift` (all included in `make check`) and verify:
- No new UNWRAPPED operations unless intentionally deferred (document why in PR)
- No MISSING operations (service layer calling non-existent generated methods)

### New SDK Method Checklist

- [ ] Does the generated client have a method for this endpoint?
- [ ] If not, is the endpoint in the OpenAPI spec? (Add it if missing)
- [ ] Does my implementation use ONLY generated client methods for API calls?
- [ ] Is there ANY `fmt.Sprintf` with a URL path? (If yes, refactor)
- [ ] For pagination: am I using `FollowPagination()` with Link headers?
