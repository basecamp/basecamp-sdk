# Contributing to Basecamp SDK

Thank you for your interest in contributing to the Basecamp SDK. This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

| SDK | Requirements |
|-----|-------------|
| Go | Go 1.26+, [golangci-lint](https://golangci-lint.run/welcome/install/) |
| TypeScript | Node.js 18+, npm |
| Ruby | Ruby 3.2+, Bundler |
| Swift | Swift 6.0+, Xcode 16+ |
| Kotlin | JDK 17+, Kotlin 2.0+ |
| Python | Python 3.11+, [uv](https://docs.astral.sh/uv/) |

Shared tooling: `jq`, and bash >= 4.4 on `PATH` for the pairwise-canary
scripts that `make check` runs (macOS ships bash 3.2 at `/bin/bash` —
`brew install bash`; the scripts fail fast with this exact hint).

A Basecamp account is optional (for integration testing only).

### Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/basecamp/basecamp-sdk.git
   cd basecamp-sdk
   ```

2. Install dependencies and run tests for each SDK:

   **Go:**
   ```bash
   cd go && go mod download
   make test
   make check   # formatting, linting, tests
   ```

   **TypeScript:**
   ```bash
   cd typescript && npm install
   npm test
   npm run typecheck
   npm run lint
   ```

   **Ruby:**
   ```bash
   cd ruby && bundle install
   bundle exec rake test
   bundle exec rubocop
   ```

   **Swift:**
   ```bash
   cd swift
   swift build
   swift test
   ```

   **Kotlin:**
   ```bash
   cd kotlin
   ./gradlew :sdk:jvmTest
   ```

   **Python:**
   ```bash
   cd python && uv sync && cd ..
   make py-test
   make py-check   # tests, types, lint, format, drift
   ```

3. Run all SDKs at once from the repo root:
   ```bash
   make check        # all 6 SDK test suites
   make conformance  # cross-SDK conformance tests
   ```

## Code Style

### Python Code

- Target Python 3.11+
- Use [ruff](https://docs.astral.sh/ruff/) for linting and formatting (line length: 120)
- All service method parameters are keyword-only (after `*`)
- Use type annotations for function signatures
- Generated code under `src/basecamp/generated/` is exempt from style rules

### Go Code

- Follow standard Go conventions and [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` for formatting (run `make fmt`)
- Keep functions focused and small
- Document all exported types, functions, and methods
- Use meaningful variable names

### Naming Conventions

- Service types: `*Service` (e.g., `ProjectsService`, `TodosService`)
- Request types: `Create*Request`, `Update*Request`
- Options types: `*Options` or `*ListOptions`
- Error constructors: `Err*` (e.g., `ErrNotFound`, `ErrAuth`)

### Error Handling

- Return structured `*Error` types with appropriate codes
- Include helpful hints for user-facing errors
- Use `ErrUsageHint()` for configuration/usage errors
- Wrap underlying errors with context

### Testing

- Write unit tests for all new functionality
- Use table-driven tests where appropriate
- Mock HTTP responses using `httptest`
- Test both success and error paths

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/) for clear, structured commit history.

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, semicolons, etc.)
- `refactor`: Code changes that neither fix bugs nor add features
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `build`: Build system or dependency changes
- `ci`: CI configuration changes
- `chore`: Other changes that don't modify src or test files

### Scope

Use the service or component name:
- `projects`, `todos`, `campfires`, `webhooks`, etc.
- `auth`, `client`, `config`, `errors`
- `docs`, `ci`, `deps`

### Examples

```
feat(schedules): add GetEntryOccurrence method

fix(timesheet): use bucket-scoped endpoints for reports

docs(readme): add error handling section

test(cards): add coverage for move operations
```

## Pull Request Process

### Before Submitting

1. **Run all checks locally:**
   ```bash
   make check  # runs all 6 SDK test suites from repo root
   ```

2. **Ensure conformance tests pass:**
   ```bash
   make conformance
   ```

3. **Update documentation** if adding new features

### Submitting a PR

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```

2. Make your changes with clear, focused commits

3. Push and open a pull request against `main`

4. Fill out the PR template with:
   - Summary of changes
   - Motivation and context
   - Testing performed
   - Breaking changes (if any)

### Review Process

- All PRs require at least one review
- CI must pass (tests, linting, security checks)
- Address review feedback promptly
- Squash commits if requested

## Adding New API Coverage

All SDKs are generated from a single Smithy specification. When adding support for new Basecamp API endpoints:

1. **Edit the Smithy model** (`spec/basecamp.smithy`)
   - Define the resource, operations, and shapes
   - Follow patterns from existing resources (e.g., `Project`, `Todo`)

2. **Regenerate everything** in one step:
   ```bash
   make generate
   ```

   This runs Smithy build, behavior model, URL routes, provenance sync, and per-language generators (TypeScript, Ruby, Python, Kotlin, Swift, Go) in dependency order.

3. **Run per-SDK generators individually** if you only need one:
   - **Go:** `make go-check-drift` — Go services are hand-written wrappers around the generated client; the drift check verifies all generated operations are covered
   - **TypeScript:** `make ts-generate-services`
   - **Ruby:** `make rb-generate-services`
   - **Swift:** `make swift-generate`
   - **Kotlin:** `make kt-generate-services`
   - **Python:** `make py-generate`

4. **Add tests** for each SDK

5. **Add conformance tests** (`conformance/tests/`) covering the new operations

6. **Update documentation**:
   - Add to the services table in each SDK's README
   - Add to CHANGELOG under `[Unreleased]`

## Spec-shape lints

The repo enforces a small set of structural invariants on the OpenAPI spec
beyond the language-specific drift checks. These run as part of `make check`:

- **Bucket↔flat parity** (`make check-bucket-flat-parity`): every
  `GET /{accountId}/buckets/{bucketId}/<resource>(/...).json` list operation
  must have a flat counterpart at `/{accountId}/<resource>.json`, or be
  justified in [`spec/bucket-scoped-allowlist.txt`](spec/bucket-scoped-allowlist.txt).
  The intent is that cross-project SDK consumers shouldn't have to walk every
  project to query account-wide resources.

  When adding a bucket-scoped list endpoint, either add the matching flat
  endpoint or append a one-line entry to the allowlist with a justification
  comment.

## Live canary

The TypeScript runner also drives a *live canary* against a real Basecamp
backend. It dispatches every operation in
[`conformance/tests/live-my-surface.json`](conformance/tests/live-my-surface.json)
through the SDK's typed surface, captures the raw wire response (bytes +
headers), and validates each response body against the OpenAPI response
schema. Forward-compat additions on the wire surface as "extras observed"
in the run summary — never as failures — so new BC5 fields don't break
the canary while still being visible.

The canary is **opt-in** and **does not run as part of `make check`**:

```bash
BASECAMP_LIVE=1 \
BASECAMP_TOKEN=<your-token> \
BASECAMP_ACCOUNT_ID=<your-account> \
make conformance-typescript-live
```

Optional env:

- `BASECAMP_HOST` — backend **origin** only (e.g. `https://3.basecampapi.com`);
  the runner appends `/{accountId}` to mirror `createBasecampClient`'s
  default URL composition.
- `BASECAMP_BACKEND=bc4|bc5` — namespaces persisted snapshots so BC4 and
  BC5 runs don't collide.
- `LIVE_RECORD_DIR=<path>` — persists wire snapshots to
  `<path>/<backend>/wire/<test>.json`. Consumed by the cross-language
  replay runners (`make conformance-*-replay`) and by the pairwise
  BC4↔BC5 comparison (`scripts/compare-canary-runs.sh`).
- `BASECAMP_BC4_PROJECT_ID` / `BASECAMP_BC5_PROJECT_ID` /
  `BASECAMP_PROJECT_ID` etc. — explicit fixture-IDs override the runner's
  discovery walk. Same pattern applies for `TODOSET_ID`, `TODOLIST_ID`,
  `TODO_ID`.

Tests skip with a clear `skipReason` when a fixture-ID can't be resolved
(no env override, no discovery match) — they don't fail.

Adding an operation to the live canary requires both a fixture entry in
`live-my-surface.json` and a dispatch case in
`conformance/runner/typescript/live-dispatch.ts`. The runner's startup
gate refuses to run if any fixture operation lacks a dispatch.

Because live canary fixtures live in the shared `conformance/tests/` directory,
offline conformance runners must treat `mode` as part of the shared schema and
execute only mock tests: omitted `mode` or `mode: "mock"`. `mode: "live"` entries
belong to the TypeScript live wire-capture runner and the cross-language
wire-replay runners described next.

### Wire replay (cross-language)

The TypeScript live runner is the single canonical wire-capturer. When invoked
with `LIVE_RECORD_DIR=<path>`, it persists every captured response to
`<path>/<backend>/wire/<test>.json` with the snapshot format
`{ operation, pages: [{status, headers, body, bodyText, url}], pages_count }`.

The Ruby, Python, Go, and Kotlin runners each have a *wire-replay mode* that
reads those snapshots and decodes each page through their SDK. No HTTP calls,
no mock servers — the input is the canonical wire bytes captured by the TS
runner. Decode results land at
`<path>/<backend>/decode/<language>/<test>.json` with the format
`{ schema_version, operation, pages: [{decoded, decode_error, missing_required, extras_seen}] }`.

Each runner enforces three coverage gates at startup before doing any decode
work:

1. **Decoder coverage** — every operation in `live-my-surface.json` has a
   decode case in this runner.
2. **Snapshot completeness** — every operation in `live-my-surface.json` has
   a corresponding snapshot file at `<path>/<backend>/wire/`.
3. **Snapshot recognition** — every snapshot's `operation` field is in
   `live-my-surface.json` (catches drift between TS dispatch and the shared
   fixture).

Each gate prints which operations triggered it so the operator can fix the
right side: TS dispatch, the fixture, or the replay runner.

Two-stage flow:

```bash
# Step 1: TS captures canonical wire snapshots (live HTTP, requires creds).
BASECAMP_LIVE=1 \
BASECAMP_TOKEN=<token> \
BASECAMP_ACCOUNT_ID=<account> \
BASECAMP_BACKEND=bc4 \
LIVE_RECORD_DIR=tmp/canary \
make conformance-typescript-live

# Step 2: each language replays those snapshots through its SDK (offline).
for lang in ruby python go kotlin; do
  WIRE_REPLAY_DIR=tmp/canary BASECAMP_BACKEND=bc4 \
    make conformance-$lang-replay
done
```

Step 2 needs no credentials and no network — it's pure decode + walk. The
extras-observed output across languages is a consistency check on the
hand-rolled schema walkers (which mirror the TS validator's required + extras
algorithm in each language). Any per-language divergence in `extras_seen`
points at a walker bug in the diverging language.

When the SDK gains a new operation in `live-my-surface.json`, it must be
added to:

- `conformance/runner/typescript/live-dispatch.ts` — TS dispatch case.
- `conformance/runner/ruby/replay-runner.rb` — Ruby decoder.
- `conformance/runner/python/replay_runner.py` — Python decoder.
- `conformance/runner/go/replay_runner.go` — Go decoder.
- `kotlin/conformance/src/main/kotlin/com/basecamp/sdk/conformance/ReplayRunner.kt` — Kotlin decoder.

Each runner's coverage gate refuses to start until all five are in place.

### Pairwise BC4↔BC5 comparison

Per-backend schema validation is necessary but not sufficient. With every
new BC5 field marked optional, a regression where BC4 emits `memories:
[item, item]` and BC5 emits `memories: []` would pass per-backend schema
checks — yet that's exactly the additive-only invariant the canary should
catch. The pairwise comparison closes that loop.

Each live test in `conformance/tests/live-my-surface.json` can carry
`pairwiseAssertions`, which apply to a matched pair of BC4 and BC5 wire
snapshots for the same operation. Four rule types:

- `pairwiseSupersetArray` — BC5 array length at each path must be ≥ BC4's.
  Catches "this array shrank between backends".
- `pairwiseSupersetKeys` — BC5 object's keys at each path must be a superset
  of BC4's keys. Catches "this field disappeared".
- `pairwiseEqual` — BC5 value at each path must equal BC4's. Use sparingly;
  most useful for type-discriminator fields.
- `pairwiseDeltaAllowed` — paths where divergence is explicitly accepted.
  The listed paths are skipped by the other three rules for this operation.
  `reason` is **required** — a public compatibility report indexes accepted
  divergences.

Path syntax (dotted identifiers, evaluated against each snapshot):

- `""` (empty string) addresses the body root.
- `foo.bar` is shorthand for `pages[0].body.foo.bar` — single-page bodies.
- `pages[N].body.X` addresses a specific page in multi-page snapshots.
- `pages[*].body.X` aggregates across pages into a list (each page's value
  becomes one element of the result). `pairwiseEqual` compares that list as
  a single value. `pairwiseSupersetArray` compares the **total item count
  across pages** (a page missing the field contributes 0; any non-null
  non-array page value is invalid), so a field dropped on every page fails
  even when page counts match, and redistribution across page boundaries
  with the total preserved passes. `pairwiseSupersetKeys` does **not**
  support `pages[*]` — the aggregate is a list, not an object, and the rule
  flags it as an invalid target; address a specific page
  (`pages[N].body.X`) for keys checks on paginated endpoints.

Example — the canonical `memories` canary on `GetMyNotifications`:

```json
"pairwiseAssertions": [
  {
    "type": "pairwiseSupersetArray",
    "paths": ["memories"],
    "reason": "Additive-only invariant: BC5 memories[] should remain a superset of BC4's. PERMANENTLY waived by the pairwiseDeltaAllowed entry below — bc3 documents memories as an always-empty placeholder on BC5 (doc/api/sections/my_notifications.md). See spec/api-gaps/memories-emptied-regression.md."
  },
  {
    "type": "pairwiseDeltaAllowed",
    "paths": ["memories"],
    "reason": "PERMANENT documented contract divergence: BC5 ships `json.memories []` by contract — doc/api/sections/my_notifications.md (bc3 #11628) codifies memories as an always-empty placeholder superseded by bubble_ups. The once-planned alias (`json.memories @bubble_ups`, bc3 #10947) never shipped; #10947 is closed unmerged. Not a pending regression — this entry is the machine-readable record of an accepted BC4→BC5 subtractive delta. Retire only if BC4 (the four branch) empties memories too (the superset rule then passes clean) or BC5's documented contract changes. Tracked in spec/api-gaps/memories-emptied-regression.md."
  }
]
```

BC5 ships `memories: []` permanently by documented contract —
`doc/api/sections/my_notifications.md` (bc3 #11628) codifies memories as an
always-empty placeholder superseded by `bubble_ups`. The superset rule stays
as the encoded additive-only invariant; the `pairwiseDeltaAllowed` entry is
the machine-readable record of the accepted delta, feeding the public
compatibility report. Retire the waiver only if BC4 (the four branch) empties
memories too — the superset rule then passes clean — or BC5's documented
contract changes.

### Orchestrator

`make check-bc5-compat` threads the two-backend capture plus pairwise
comparison together:

```bash
BASECAMP_TOKEN=<token> \
BASECAMP_ACCOUNT_ID=<account> \
BC5_HOST=https://5.basecampapi.com \
make check-bc5-compat
```

What it runs, in order:

1. `BASECAMP_BACKEND=bc4 LIVE_RECORD_DIR=tmp/live-canary make conformance-live`
   — TS captures wire snapshots from BC4 (defaulting `BASECAMP_HOST` to
   `https://3.basecampapi.com`), then Ruby/Python/Go/Kotlin replay-decode.
2. `BASECAMP_BACKEND=bc5 LIVE_RECORD_DIR=tmp/live-canary make conformance-live`
   — same against the BC5 origin set via `BC5_HOST`.
3. `./scripts/compare-canary-runs.sh tmp/live-canary/bc4/wire tmp/live-canary/bc5/wire`
   — applies pairwise rules. Reports all violations outside
   `pairwiseDeltaAllowed` and exits non-zero if any are found.

Override `LIVE_RECORD_DIR` (default `tmp/live-canary`) or `BASECAMP_HOST`
(default `https://3.basecampapi.com`) on the make line.

**Identical account state across both runs is mandatory.** The pairwise
rules assume the BC4 and BC5 backends see the same projects, the same
todosets, the same reading list, etc. Without that, `unreads`/`reads`
arrays drift naturally between captures and pairwise asserts will false-fail.
Use a dedicated test account with stable, equivalent fixtures (e.g. a
sandbox account snapshot replicated to each backend).

### Scheduled CI

`.github/workflows/live-canary.yml` runs `check-bc5-compat` nightly via
cron and on `workflow_dispatch`. It is **opt-in**: the workflow no-ops
with a clear log message if the required repo secrets aren't configured.

Required secrets:

- `BASECAMP_TOKEN` — OAuth token with read scope for the canary fixtures.
- `BASECAMP_ACCOUNT_ID` — the numeric account ID used on both backends.
- `BC5_HOST` — origin of the BC5 backend.

Optional:

- `BASECAMP_HOST` — origin of the BC4 backend; defaults to `https://3.basecampapi.com`.

Snapshots are uploaded as a workflow artifact (`live-canary-<run-id>`,
14-day retention) so failures can be inspected post-hoc without rerunning.

## API gap registry (`spec/api-gaps/`)

When BC ships a new user-visible feature without a JSON API (or with an
incomplete one), add an entry under [`spec/api-gaps/`](spec/api-gaps/).
The registry is the SDK side of the [BC3 API parity coordination](COORDINATION.md):
the BC3 plan owns server-side delivery; the registry tracks the gap from
detection through absorption, with status changes in git history.

To add a new entry:

1. Copy an existing entry in `spec/api-gaps/` as a template.
2. Set frontmatter status to `no-json-contract` (or `partial-coverage` /
   `ambiguous` as appropriate). See
   [`spec/api-gaps/schema.json`](spec/api-gaps/schema.json) for valid
   statuses.
3. Add a row to the table in
   [`spec/api-gaps/README.md`](spec/api-gaps/README.md).
4. Run `make validate-api-gaps` to confirm frontmatter and required body
   sections are well-formed. Wired into `make check`.

For routes that should *not* warrant an entry (transient nav state, internal
endpoints, duplicates of a route already covered elsewhere), add a record
to [`spec/api-gaps/allowlist.yml`](spec/api-gaps/allowlist.yml) with a
justification.

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include reproduction steps for bugs
- Provide Go version and OS information
- Include relevant error messages and logs

## Questions?

Open a GitHub Discussion for questions about contributing or using the SDK.
