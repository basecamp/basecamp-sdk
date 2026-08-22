# Basecamp SDK Agent Guidelines

All six SDKs (Go, TypeScript, Ruby, Swift, Kotlin, Python) share one architecture:
**Smithy spec -> OpenAPI -> generated services.** Every wire operation is generated. The
only hand-written runtime API methods are sanctioned composites calling generated wire
methods exclusively -- see SPEC.md §18 "Hand-Written Composite Methods" -- plus one
sanctioned non-HTTP wire act: the SPEC.md §23 Event Feed connector's cable dial of the
URL a generated `CreateStreamTicket` call returned (Hard Rule 2 below).

---

## Architecture

```
Smithy Spec → OpenAPI → Generated Client → Service Layer → User
```

Paths are from the repository root, since that is where you will be working.

| SDK | Generated Client | Service Layer |
|-----|-----------------|---------------|
| **Go** | `go/pkg/generated/client.gen.go` | `go/pkg/basecamp/*.go` (wraps generated client) |
| **TypeScript** | `openapi-fetch` + `schema.d.ts` | `typescript/src/generated/services/*.ts` |
| **Ruby** | HTTP client | `ruby/lib/basecamp/generated/services/*.rb` |
| **Swift** | `URLSession` via `Transport` protocol | `swift/Sources/Basecamp/Generated/Services/*.swift` |
| **Kotlin** | Ktor via `BaseService` | `kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/services/*.kt` |
| **Python** | httpx via `HttpClient` | `python/src/basecamp/generated/services/*.py` |

All `250` operations across the ~50-service per-SDK layer are generated. Hand-written code is limited to infrastructure: <!-- @operation-count -->

| Purpose | Location |
|---------|----------|
| HTTP helpers, pagination, hooks | `typescript/src/services/base.ts`, `ruby/lib/basecamp/generated/services/base_service.rb`, `swift/Sources/Basecamp/Services/BaseService.swift`, `kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/services/BaseService.kt`, `python/src/basecamp/generated/services/_base.py` |
| OAuth flows (not in OpenAPI spec) | `typescript/src/services/authorization.ts`, `ruby/lib/basecamp/services/authorization_service.rb`, `kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/oauth/`, `python/src/basecamp/services/authorization.py` (no Swift equivalent) |
| Event Feed connector: the long-lived push+poll transport, hand-written behind its TicketMinter/PollSource seams (SPEC.md §23) | `go/pkg/basecamp/eventfeed/` (Go reference — foundations, the run loop, and the tier-2 conformance driver; the Layer-1 seam adapters over the generated operations and the other SDKs are still pending) |
| Merge-safe Todos composites (update/edit over generated get+replace; SPEC.md §18) | `typescript/src/services/todos-extensions.ts`, `ruby/lib/basecamp/services/todos_extensions.rb`, `swift/Sources/Basecamp/TodosServiceExtensions.swift`, `kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/services/TodosService.kt`, `python/src/basecamp/services/todos.py` |
| Merge-safe Cards composite (update over generated get+updateVerbatim; SPEC.md §18) | `typescript/src/services/cards-extensions.ts`, `ruby/lib/basecamp/services/cards_extensions.rb`, `swift/Sources/Basecamp/CardsServiceExtensions.swift`, `kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/services/CardsService.kt`, `python/src/basecamp/services/cards.py` |
| Merge-safe Todolists composites (update/edit over generated get+replace; SPEC.md §18) | `typescript/src/services/todolists-extensions.ts`, `ruby/lib/basecamp/services/todolists_extensions.rb`, `swift/Sources/Basecamp/TodolistsServiceExtensions.swift`, `kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/services/TodolistsService.kt`, `python/src/basecamp/services/todolists.py` |
| Merge-safe Documents composites (update/edit over generated get+replace; SPEC.md §18) | `typescript/src/services/documents-extensions.ts`, `ruby/lib/basecamp/services/documents_extensions.rb`, `swift/Sources/Basecamp/DocumentsServiceExtensions.swift`, `kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/services/DocumentsService.kt`, `python/src/basecamp/services/documents.py`, `go/pkg/basecamp/documents.go` |
| Carve-out-aware Schedule entry composites (updateEntry/editEntry over generated getEntry+replaceEntry; SPEC.md §5, §18) | `typescript/src/services/schedules-extensions.ts`, `ruby/lib/basecamp/services/schedules_extensions.rb`, `swift/Sources/Basecamp/SchedulesServiceExtensions.swift`, `kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/services/SchedulesService.kt`, `python/src/basecamp/services/schedules.py`, `go/pkg/basecamp/schedules.go` |
| Merge-safe response guards shared by those composites (#576) | `typescript/src/services/merge-safe.ts`, `ruby/lib/basecamp/services/merge_safe.rb`, `python/src/basecamp/services/_merge_safe.py` |

Hand-written service files in `typescript/src/services/` and `ruby/lib/basecamp/services/` beyond the table above are NOT loaded at runtime. They exist only as reference implementations.

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

1. **NEVER edit files under `*/generated/`** — they get overwritten by generators. Four files under `generated/` are not generator-emitted and are the ONLY exceptions: three hand-written base files, edited like any other infrastructure file — `python/src/basecamp/generated/services/_base.py`, `python/src/basecamp/generated/services/_async_base.py`, `ruby/lib/basecamp/generated/services/base_service.rb` — plus the empty package marker `python/src/basecamp/generated/__init__.py`. The `generated/services/__init__.py` next to the Python base files IS generated.
2. **NEVER add hand-written service methods that touch the wire** — all wire operations come from generators. Two sanctioned exceptions: a conformance-tested composite that only calls generated wire methods and satisfies SPEC.md §18 "Hand-Written Composite Methods"; and the SPEC.md §23 Event Feed connector's cable dial — `CableTransport.dial(mint.url)`, connecting verbatim to the URL a generated `CreateStreamTicket` call returned. That dial is the connector's one non-HTTP wire act; every HTTP exchange it makes still flows through generated operations behind its `TicketMinter`/`PollSource` seams (§23 "Classification: Infrastructure, Not a Composite")
3. **NEVER skip running `make smithy-build` after Smithy changes** — keeps OpenAPI in sync
4. **NEVER construct API paths manually** — use the generated client methods
5. **NEVER bypass the SDK** — no raw `client.Get()`, string-concatenated URLs, or internal method calls

If you're writing `fmt.Sprintf` with an API path, you're doing it wrong. If the generated client lacks functionality, fix the spec and regenerate — don't work around it.

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

`spec/basecamp.smithy` holds `250` worked operations. <!-- @operation-count --> Copy the nearest one rather than
working from a skeleton here: it shows the live conventions for naming, `@http` URIs,
pagination traits and shape reuse, and it cannot drift from itself.

Two constraints the examples will not tell you outright:

- Every new operation needs a tag in `spec/overlays/tags.smithy`, or it is silently
  absent from every generated service.
- Smithy wrapper structures are a spec convention, not the wire shape (see above).

### Reference Sources

- **BC3 API docs** (`~/Work/basecamp/bc3/doc/api/sections/*.md`) — authoritative HTTP endpoint documentation. The public `bc3-api` repo is a synced mirror, not the source of truth.
- **Go SDK** (`go/pkg/basecamp/*.go`) — existing operation signatures
- **Existing Smithy** (`spec/basecamp.smithy`) — established patterns and reusable types

---

## Generation Pipeline

After any Smithy spec change, run the full pipeline:

```
make generate
```

That target is the pipeline's only definition, and this section deliberately no
longer spells out the sequence it runs. It used to, and the copy had already
drifted: it omitted `behavior-model`, `provenance-sync` and `sync-api-version`,
so a contributor following it verbatim regenerated the accessors and never
re-derived the constants that come from them — leaving every `@service-count`
span stating the previous run's number and failing the next `make check` for a
reason the transcript never mentioned. Ordering matters here (`sync-api-version`
must run *after* the Kotlin and Swift accessor generators, which is why it
appears twice in the target), and an ordering restated by hand in prose is one
more thing to keep in step. Read the `generate` target if you need the phases.

Never commit a Smithy change without regenerating all downstream artifacts.
Never assume "I'll regenerate later" — regenerate now, or the drift compounds.

### Invariants

1. **`openapi.json` must always reflect the current Smithy spec.** Run `make smithy-build` after any change to `spec/basecamp.smithy` or `spec/overlays/*.smithy`.
2. **Service generator mappings must stay current.** `typescript/scripts/generate-services.ts`, `ruby/scripts/generate-services.rb`, `kotlin/generator/src/main/kotlin/com/basecamp/sdk/generator/Config.kt`, and `python/scripts/generate_services.py` all have hardcoded `TAG_TO_SERVICE` mappings. Update them for new/renamed/removed operations. Treat unmapped-operation warnings as errors.
3. **Tags in `spec/overlays/tags.smithy` control service grouping.** Every new operation needs a tag or it won't appear in any generated service.
4. **Hand-written Go service methods must use generated client types.** Field names, method signatures, and request/response body types come from `go/pkg/generated/client.gen.go`. One carve-out (SPEC.md §18 "Hand-Written Composite Methods"): where the wire contract is inexpressible through the generated request type — concretely the `""` date clear, which `*types.Date` cannot spell; an empty string or list behind a pointer member is expressible and does not qualify — a method may marshal an explicit body map through the operation's generated `*WithBody` variant, with keys matching the generated request schema.

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

1. Verifies all version constants match the requested version
2. Verifies the working tree is clean
3. Verifies you're on the `main` branch
4. Pushes `main` to origin (release workflows guard that the tag commit is reachable from `origin/main`)
5. Creates and pushes the `v{VERSION}` tag

### Guards

- **Branch guard**: refuses to release from non-`main` branches
- **Version guard**: refuses if any version constant doesn't match
- **Clean tree guard**: refuses if there are uncommitted changes
- **CI guard**: each release workflow runs `git merge-base --is-ancestor "$GITHUB_SHA" origin/main` — rejects tags whose commit isn't on `main`

### Verification

After releasing, monitor all 7 workflows in GitHub Actions. The "Create GitHub Release" workflow waits for the 6 SDK workflows to succeed before creating the release.

```bash
gh run list --repo basecamp/basecamp-sdk --limit 7 --json name,status,conclusion
```

---

## Upstream API Sync Workflow

When syncing the SDK spec to match upstream API changes in `basecamp/bc3` (`doc/api/` docs + Rails app):

### Provenance is Mandatory

Every sync MUST update `spec/api-provenance.json` with the upstream `bc3` HEAD:
```bash
gh api repos/basecamp/bc3/commits/HEAD --jq '.sha'
```

Update the `revision` and `date` fields, then `make provenance-sync`. This is not optional — provenance tracks what the SDK is conformant to.

The Smithy service version is derived from the shared provenance date. Run `make sync-spec-version` (or `make smithy-build`, which does this automatically) after updating provenance.

**Prose restates the pin, and prose drifts.** `COORDINATION.md` and
`spec/api-gaps/README.md` each name the current pin in narrative, and both sat
two repins stale before `make doc-constants-check` existed. Every bc3 revision
you write into prose is one of exactly two things, and which one decides
everything else:

- **A current-value claim** — "the provenance pin is `X`". True only right now.
  It carries an `<!-- @bc3-pin -->` marker at the end of its line and names
  both the revision and the sync date. `make sync-api-version` rewrites every
  marked span from `spec/api-provenance.json` (SHA abbreviation length
  preserved), and the gate fails on any that drifted — or on a span that
  dropped its SHA or its date, since the writer can only rewrite what it can
  see.
- **An as-of fact** — "verified against `X`", "shipped in BC3 #12380 (`X`)",
  "the `A..B` range contains…". True forever, because the revision is bound to
  a fixed observation. It stays unmarked and is never rewritten:
  `spec/api-gaps/` cites ~30 historical SHAs on purpose, and rewriting one
  would convert settled triage into a claim nobody made.

There is a third form, and it is usually the right answer: **name the pin by
reference instead of by value** — "the revision `spec/api-provenance.json`
currently pins", "already inside the current pin". It means today's pin, stays
true across every repin, and restates no constant, so nothing can drift and
nothing needs marking. Reach for a literal SHA only when the sentence is
genuinely about *which* revision.

The failure to avoid is writing the second in the grammar of the first — "at
the pinned revision", "the provenance pin (`X`)" — which is true the day it is
written and silently false at the next repin. Name what binds the revision:
the PR that shipped it, or the verification it backs. `make
doc-constants-check` enforces the boundary at the one moment it can see it
without guessing at tense: an unmarked sentence naming **today's** pin fails,
because that is the day such a sentence is born. If it is genuinely an as-of
fact that happens to name the current pin — a repin cites the commit it moved
to, so `spec/api-gaps/README.md` hits this on every sync — record the file in
`spec/doc-constants.json` `.unmarkedPinCitations` with a reason and an exact
count. The count is what keeps the grant honest: the next unmarked citation in
that file still fails until someone reads it and restates the number. A SHA
that is not the current pin is not checked at all — sorting those needs the
pin's whole history, which CI's shallow clones do not have.

Expect the range form to trip this, because a range written at a repin ends
*at* the pin that repin set. An `` `A..B` `` whose `B` is today's revision
names it as surely as a bare mention does, and the check reads inside compound
code spans precisely so it cannot be smuggled past that way. A range is still
an as-of fact — grant it and move on. `A` is not the current pin and is never
flagged.

(This paragraph deliberately says `A..B` rather than quoting the live range:
documentation of the convention must not itself restate the constant, or it
becomes the very class-A claim it is warning about — as this one did, and the
gate caught it.)

The same marker convention covers `<!-- @api-version -->` (from `openapi.json`
`info.version`) and SPEC §19's `<!-- @assertion-types:begin/end -->` table
(from `conformance/schema.json`). `spec/doc-constants.json` commits the exact
number of markers each file carries, so deleting one fails the gate instead of
silencing it — and adding one fails too, until you record it, which is what
makes the next deletion fail. It also carries `.writerExcludes`, the files the
writer must never touch: `spec/api-gaps/README.md` is on it, because its pin
sentence heads the range triage and cannot advance without that triage
advancing too.
`scripts/test-doc-constants.rb` (run by `make doc-constants-check`) asserts the
gate rejects each of these failure modes.

Two marked spans are not claims to be checked but blocks to be GENERATED, and
neither should ever be edited by hand. SPEC §19's Zero-Skip roster,
`<!-- @zero-skip-roster:begin/end -->`, is rendered from
`spec/zero-skip-roster.yml` and required to match byte for byte — edit the YAML,
never the block. SPEC §5's and Appendix B's service rosters,
`<!-- @account-scoped-services:begin/end -->`, are rendered from the generated
Kotlin and Swift accessors — regenerate the SDKs, never the block. Both are
writable where the table kinds are not for the reason `spec/doc-constants.json`
states: no column of either is hand-written, so the writer can author the whole
span. It replaced a
parser that read the roster back out of SPEC's prose — five bypasses in, each
fix a new selector, which is what "reassess the instrument" looks like when the
instrument is a reader.

**Pin semantics.** The pin is the conformance baseline as of the last sync — it asserts that all upstream drift up to that revision has been *triaged*, not that every contract in it has been *absorbed*. Upstream contracts shipped past the SDK's modeled surface are tracked in `spec/api-gaps/` (status `addressed-in-bc3-pr-N`) until an absorption PR lands. A repin is valid exactly when every drift item in `pin..HEAD` is either absorbed into the spec or registered in `spec/api-gaps/`; it is not blocked on absorption itself. The pin never moves backward. The `compatibility.*` pins mark the last **verified API-surface state** of their branch, not a last-glanced timestamp — refresh one only when re-verifying that branch's API surface, and record verification dates in the PR that did the checking.

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
9. Run `make sync-spec-version` (or `make smithy-build`)
10. Run `make sync-api-version` — it rewrites the SDK constants *and* the
    `@bc3-pin` / `@api-version` marked spans in prose; record the new range's
    triage in `spec/api-gaps/README.md` (unmarked, it is history)
11. `make` must pass clean

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

1. **Existing tests updated** — every test stubbing a changed path must be updated.
   Inline stubs may omit unrelated response fields, but every response stub for a
   changed path must include any newly required or behaviorally changed field.
2. **New field tests** — at least one test fixture should include new fields to verify they flow through

> **Automated backstop.** The shared JSON fixtures under `spec/fixtures/` are
> guarded by `make check-fixture-coverage` (`spec/fixtures/manifest.yaml`): every
> manifest'd fixture is validated against its schema for both required-field
> presence and type/nullability (a required-null-against-non-nullable or a
> wrong-typed value fails), every `covered_schemas` entry must keep a concrete
> representative, and every rich-text emitter schema must be covered or explicitly
> excluded (so the inventory can't silently shrink). A new required field on a
> covered schema is therefore forced into a fixture. This guard covers only the
> manifest'd shared fixtures — one-off inline stubs remain the reviewer's
> responsibility under the rule above.

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
