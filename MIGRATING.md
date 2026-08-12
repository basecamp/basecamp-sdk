# Migrating

Upgrade notes for consumers of the Basecamp SDKs. One section per release that
breaks something, newest first. Read the section for your version range before
you upgrade, not after.

This file exists because the repo generates its release notes from PR labels
(see [CONTRIBUTING.md](CONTRIBUTING.md)). That produces an accurate list of what
merged; it cannot tell you which of those changes your code has to react to, or
what wrong behaviour you get if you ignore one. This file is that half.

---

# v0.14.0

Breaking in Go and in the shape every SDK decodes from
`GET /uploads/{id}/versions.json`.

**Operation inventory: 249 → 250** — `CreateUploadVersion`. Derive both ends
rather than trusting either number:

```bash
git show v0.13.0:openapi.json | jq '[.paths[]|keys[]]|length'
jq '[.paths[]|keys[]]|length' openapi.json
```

### `ListUploadVersions` returns versions, not uploads (#649)

The endpoint has always returned **events**. The spec declared
`uploads: UploadList` anyway, and **11 of `Upload`'s 14 required members are
absent from every response** — which is why the CLI's versions command and the
MCP server's `list_upload_versions` printed blank fields rather than failing.
The output is now `versions: UploadVersionList`.

| SDK | was | now |
|---|---|---|
| Go | `UploadVersionListResult.Versions []Upload` | `[]UploadVersion` |
| TypeScript | `ListResult<Upload>` | `ListResult<UploadVersion>` |
| Swift | `ListResult<Upload>` | `ListResult<UploadVersion>` |
| Kotlin | `ListResult<Upload>` | `ListResult<UploadVersion>` |
| Ruby / Python | parsed body, unchanged at runtime | fields differ, see below |

The four typed SDKs keep their `ListResult` wrapper, so `.meta.totalCount` and
the pagination surface are untouched; only the element type changes. Member
access moves with it — `version.filename` becomes `version.upload?.filename`,
and the event's own `action`, `createdAt` and `creator` sit alongside.

**In Ruby and Python the compiler will not catch this.** Nothing changes in the
type; what changes is which keys are actually there. Code reading
`version["filename"]` was reading a key the server never sent and getting nil —
it now reads `version["upload"]["filename"]`, and the event's own metadata
(`action`, `created_at`, `creator`) is available where it previously looked like
a partly-empty upload.

A version carries `upload` only when its recordable still resolves; a deleted
file leaves the event behind with no `upload` at all. Check before dereferencing.
`action` is `created`, `active` (the publication) or `blob_changed` (a file
replacement). To list the file's **past** versions, take the entries that carry
an `upload` with `current == false` — not the ones with `action == "blob_changed"`,
which drops the original (it arrives as `created` or `active`) and keeps the
current file. The per-version `download_url` serves **that** version's bytes; the
upload's own always serves the latest.

### Go: `UpdateUploadRequest.Description` became `*string`

Tri-state, following `UpdateGaugeNeedleRequest.Description` (#560): nil leaves
it untouched, `basecamp.Ptr("")` clears it, `basecamp.Ptr(v)` sets it.
Previously a plain `string` behind a zero-value guard, so `""` read as *unset*
and clearing a description through `Update` was unreachable — the divergence
SPEC §5 documented.

```go
// Before — compiled, and silently did nothing to the description.
svc.Update(ctx, id, &UpdateUploadRequest{Description: ""})

// After — clears it.
svc.Update(ctx, id, &UpdateUploadRequest{Description: basecamp.Ptr("")})

// After — leaves it alone.
svc.Update(ctx, id, &UpdateUploadRequest{BaseName: "renamed"})
```

The compiler catches this one: `Description: "text"` no longer type-checks. Wrap
it in `basecamp.Ptr`.

`BaseName` is deliberately still a plain `string` on both this and
`CreateUploadVersionRequest`. `Upload#base_name=` guards on
`new_base_name.present?`, so `""` and absent are the same write server-side —
there is no third state for a pointer to express.

### New: `UploadsService.CreateVersion` and a 507 error code

Not breaking, but the reason for the above. `POST /uploads/{id}/versions.json`
replaces an upload's file in place, keeping the recording's id, URL and
comments, so a published link keeps working — which `CreateUpload` cannot do.

A `507 Insufficient Storage` now maps to the new `limit_exceeded` code (exit
code 10) instead of `api_error`. **If you branch on `api_error` to decide
whether to back off, a limit failure no longer lands in that branch** — which is
the point: it was reported as retryable, and no retry can satisfy a plan limit.

The mapping is by **status**, not by operation, so it reaches every 507 the spec
declares — all eight, across three different limits:

| Operations | Limit | Error shape |
|---|---|---|
| `CreateUpload`, `CreateUploadVersion`, `CreateAttachment`, `CreateCampfireUpload` | file storage | `StorageLimitError` (new) |
| `CreateProject`, `UnarchiveProject` | project count | `ProjectLimitError` (v0.13.0) |
| `CreateWebhook`, `UpdateWebhook` | webhook count | `WebhookLimitError` (pre-existing) |

Only the first row is new surface. The other four operations already returned
507 and already reported it as a retryable `api_error`; they are reclassified
here too, so **webhook and project callers need the same new branch even though
nothing about those endpoints changed**. Derive the list rather than trusting
it:

```bash
jq -r '.paths[]|to_entries[]|select(.value.responses."507")|.value.operationId' openapi.json
```

### The new error code is source-breaking in four SDKs

Adding a member to a closed type breaks exhaustive handling, so this is not
merely behavioural:

| SDK | what changed | how it breaks |
|---|---|---|
| TypeScript | `ErrorCode` union gains `"limit_exceeded"` | a `Record<ErrorCode, T>` map, or a `switch` the compiler checks for exhaustiveness, stops compiling until it has a branch |
| Swift | `BasecampError` gains `case limitExceeded` | a `switch` over the enum without a `default` stops compiling |
| Kotlin | `BasecampException` gains `LimitExceeded` | a `when` over the sealed class used as an expression stops compiling |
| Python | `ErrorCode` (a `StrEnum`) gains `LIMIT_EXCEEDED` | a `match` over it ending in `typing.assert_never` stops type-checking — mypy reports the new member as unhandled |

Python's break needs a type-checker to surface, not an interpreter: the module
imports and runs either way. If your CI runs mypy — this package does — it fails
there rather than at import, which makes it easier to miss in review and no less
of a break.

Go and Ruby take a new constant rather than a new variant, so neither breaks a
build — which is exactly why they need reading for: a `case` or `when` falling
through to a default arm now routes storage and project limits wherever that
default goes.

Add a `limit_exceeded` branch that surfaces the limit to the user and does not
retry. This SDK's own Kotlin test suite hit the compile error, which is what the
exhaustive `when` in `ErrorTest` exists to produce.

### Go: an absent expiry reads as absent, not as an instant (#662)

Two silent behavior changes on `AuthorizationInfo.ExpiresAt` (`FlexTime`), and
neither gives you a compile error:

- **A wire `expires_at: 0` now decodes to the zero time.** Previously it
  decoded to `time.Unix(0, 0)` — a *valid* 1970 date with `IsZero() == false`,
  so "no expiry" read as "expired 56 years ago". No production issuer has ever
  sent `0` (BC3 tokens validate presence; legacy Signal tokens self-default an
  expiry), so this is hardening against the RFC 7591 collision — `0` means
  "never expires" in bc3's own `client_secret_expires_at` — not a live-bug fix.
  Code that deliberately round-tripped `0` through `FlexTime` gets the zero
  time back instead.
- **A zero `FlexTime` marshals as `null`.** Previously it marshaled as the
  fabricated instant `"0001-01-01T00:00:00Z"`, indistinguishable from data the
  server sent. If you re-serialize `AuthorizationInfo` and consume `expires_at`
  downstream, expect `null` where that sentinel used to be.

New, not breaking: `info.Expiry() (time.Time, bool)` is the documented front
door — `ok` is false when the document stated no expiry (absent field, explicit
`null`, or a wire `0` alike; all defensive, per the above — no production
issuer emits any of them). Prefer it over reading `ExpiresAt` directly.

### Go: `TimelineEventData.StartsAt`/`EndsAt` became `*types.FlexibleTime`

The same class as v0.13.0's [four Go pointer entries](#go): the generated
counterpart is `*types.FlexibleTime` (the bounds are required-and-nullable —
`schedule_entry_*` events always carry them, but the value may be `null`), and
the hand-written struct flattened them to value types, fabricating
`0001-01-01T00:00:00Z` for a null bound on re-marshal.

**This compiles unchanged and panics at runtime on the wrong payload.** Go
promotes value-receiver methods through the pointer, so
`ev.Data.StartsAt.IsZero()` still builds — and nil-panics when the API sent
`null`. Nil-check first:

```go
// Before
if !ev.Data.StartsAt.IsZero() { start := ev.Data.StartsAt.Time; ... }

// After
if ev.Data.StartsAt != nil { start := ev.Data.StartsAt.Time; ... }
```

A nil bound re-marshals as `null` (the key stays, matching the wire contract);
a null bound previously decoded to the zero time, so `IsZero()`-based absence
checks translate to nil checks.

---

# v0.13.0

Breaking across all six SDKs — Go, TypeScript, Python, Ruby, Kotlin, Swift.

**Read [Breaks your compiler will not catch](#breaks-your-compiler-will-not-catch)
first.** A large share of this release survives a clean build. Most of that
share gives you no signal at all — the call keeps working against a live server
and does something different. A smaller part compiles and then panics or raises,
but only on a payload of one particular shape — and the shape is not the same
one in every SDK. The four Go entries need a field to be **absent**; Ruby's and
Kotlin's need theirs to be **present**, and both stay quiet when it is nil. A
fixture built for one direction proves nothing about the other, and either way
it passes your tests and fails in production.

| SDK | no signal at all | fails at runtime |
|---|---:|---:|
| [Go](#go) | **12** | **4** |
| [Swift](#swift) | **10** | 0 |
| [TypeScript](#typescript) | 9 | 0 |
| [Python](#python) | 8 | 0 |
| [Ruby](#ruby) | 10 | 1 |
| [Kotlin](#kotlin) | 6 | 1 |

61 breaks the compiler will not catch, across the six: 55 with no signal at all
and 6 that fail at runtime. These are counts at `9a819e44d` — the last commit of
release content, and the baseline every count here was measured against, not the
commit the tag is cut from. v0.13.0 is tagged from `main` after this guide
merges, so the tagged tree contains this file; the counts carry over unchanged
because nothing described here is still in flight.

#637 does **not** add a row to either column, despite landing after the first
draft of this table. It made `Todolist.color` and `.comments_app_url` required —
but neither member existed on `Todolist` at v0.12.0 in any SDK. Both arrived
earlier in this same release with #628, so from a v0.12.0 consumer's position
there is no member that changed from optional to required; there are two new
members that happen to be required from the start. It reshapes the #628 break
rather than adding one, and that is where this guide documents it.

#681 **does** break TypeScript, and it is the one change here measured past
the baseline this document states. It adds nothing to either column — it is
five required-to-optional field relaxations, which `tsc` catches under
`strictNullChecks` — so the 61/55/6 totals below are unmoved. See
[Five `Identity` and `AuthorizedAccount` fields became optional](#five-identity-and-authorizedaccount-fields-became-optional-681).

Every claim below was read out of `git diff v0.12.0..main`, not out of a PR
body.

> **As of `9a819e44d`.** Base `v0.12.0` = `7e2925d25`. Every count in this
> document is a measurement at that commit, not a constant. If you are reading
> this from a later tag, re-run the derivations below — they are cheap, and a
> hand-incremented count is how these go wrong. The release spans 64 merged
> pull requests, 16 of them labelled `breaking`:
>
> ```bash
> # Ask which PRs are IN the range, rather than counting commits in it. Two
> # traps this avoids, both of which produced a wrong number here first:
> #
> #   1. A merge-TIMESTAMP filter is off by one at the boundary. #556's squash
> #      commit IS `7e2925d25`, the commit v0.12.0 tags, and its `mergedAt`
> #      lands a moment after that commit's own timestamp — so
> #      `mergedAt > tag_time` credits this release with a PR that shipped in
> #      the last one.
> #   2. `git log v0.12.0..HEAD | wc -l` counts COMMITS, which equals PRs only
> #      while every commit is a squash merge. The release-prep commit is
> #      pushed directly to main and is not a PR, so that count runs one high
> #      from the moment the version is bumped.
> #
> #   3. Too small a `--limit`. `gh pr list` orders by CREATED, not merged, so
> #      a long-open PR that merged late sits far down the list. At `--limit
> #      300` an old-but-recently-merged PR drops off the end and is silently
> #      uncounted. Ask for more than you need; the filtering is done below.
> #
> # Reachable from origin/main and not from v0.12.0 is the definition; apply it
> # to merge commits of PRs and none of the three traps applies. Read from
> # origin/main rather than HEAD — a stale checkout reported an inventory two
> # operations behind what was actually on main.
> gh pr list --repo basecamp/basecamp-sdk --state merged --limit 1000 \
>   --json mergeCommit --jq '.[].mergeCommit.oid' |
>   while read sha; do
>     git merge-base --is-ancestor "$sha" origin/main 2>/dev/null &&
>     ! git merge-base --is-ancestor "$sha" v0.12.0 2>/dev/null && echo "$sha"
>   done | wc -l
>
> # Same rule for the breaking subset.
> gh pr list --repo basecamp/basecamp-sdk --state merged --label breaking \
>   --limit 1000 --json mergeCommit --jq '.[].mergeCommit.oid' |
>   while read sha; do
>     git merge-base --is-ancestor "$sha" origin/main 2>/dev/null &&
>     ! git merge-base --is-ancestor "$sha" v0.12.0 2>/dev/null && echo "$sha"
>   done | wc -l
> ```
>
> A labelled PR is not the same unit as an entry below: one PR can break four
> SDKs and one SDK can carry two entries from the same PR, which is why the
> per-SDK columns are larger than 16.

## What shipped

**Operation inventory: 238 → 249.** Derive both ends rather than trusting
either number:

```bash
# at the tip
python3 -c "import json;d=json.load(open('openapi.json'));\
print(sum(1 for p,v in d['paths'].items() for m in v if m in ('get','post','put','patch','delete')))"

# at the previous release, without checking it out
git show v0.12.0:openapi.json | python3 -c "import json,sys;d=json.load(sys.stdin);\
print(sum(1 for p,v in d['paths'].items() for m in v if m in ('get','post','put','patch','delete')))"
```

Sixteen operation IDs added, five removed. At the level of *capability* that is
fourteen additions, three removals and two renames:

| | |
|---|---|
| **Added (14)** | Folders — `ListFolders`, `GetFolder`, `CreateFolder`, `UpdateFolder`, `DeleteFolder` (#593); `DestroyTimesheetEntry` (#626); cloud files — `GetCloudFile`, `CreateCloudFile`, `UpdateCloudFile` (#629); Google documents — `GetGoogleDocument`, `CreateGoogleDocument`, `UpdateGoogleDocument` (#629); project status — `ArchiveProject`, `UnarchiveProject` (#679) |
| **Renamed (2)** | `UpdateDocument` → `ReplaceDocument` (#601); `UpdateScheduleEntry` → `ReplaceScheduleEntry` (#632). Same route, same verb, honest name. |
| **Removed (3)** | `GetRecording`, `TrashTodo`, `CreateForwardReply` (#619) |

The Folders operations are worth a second look before you assume the name maps
to the route: they are drawn at `/{accountId}/stacks(.json)`, not `/folders`.

**Eleven operations kept their ID and moved route.** Nine gained a
`/buckets/{bucketId}` prefix (#619); `ListForwards` and
`RepositionTodolistGroup` had their spelling corrected (#586). The inventory
count is unchanged and every caller breaks. See
[Route corrections](#route-corrections).

Three `update` methods stopped being sparse PUTs and became read-modify-write
composites — the largest behavioural change in the release. See
[Merge-safe composites](#merge-safe-composites).

No surviving operation's retry, idempotency or pagination *configuration*
changed. `page` changed meaning; the metadata behind it did not.

**The pagination cap is now validated at construction (#678, #680).** This is
**not** one of the 61, and it adds nothing to any per-SDK column above. Those
count breaks the compiler will not catch: class A is silent, and class B needs a
particular server response before it fails. This is neither. It fails at
construction, deterministically, before any request is made, on a value you
wrote literally, with a named configuration error that says what was wrong. You
find out on the first line, every time, in development.

What newly raises depends on the SDK, because they did not start from the same
place:

| SDK | newly rejected at construction |
|---|---|
| TypeScript | `0`, negatives, `NaN`, `Infinity`, fractional caps, `Number.MAX_VALUE` and anything else `≥ 2 ** 53` |
| Kotlin | `0` and negatives (`IllegalArgumentException` via `require`) |
| Swift | `0` and negatives — `precondition`, which **traps**; `BasecampConfig.init` is public and non-throwing, so it has no way to return an error |
| Python | non-integers such as `float("inf")` and `2.5`, and `True` |
| Go, Ruby | nothing — both already rejected a non-positive cap at v0.12.0 |

Python's `0` and negatives already raised at v0.12.0; only the type check is
new. Go already panicked and Ruby already raised `ArgumentError`, so neither
moves.

The one that will actually bite is **`maxPages: Infinity` in TypeScript**, and
it is why #678 carries the `breaking` label. At v0.12.0 there was no validation
at all, so writing `Infinity` to mean "no cap" *ran* — and did exactly what it
said, following `rel="next"` without a bound. It now throws. If that was your
idiom, pass a real number.

An **absent** cap is unchanged, and that includes an explicit `null`: both
`undefined` and `null` fall through to the default 10,000, in the client factory
and in a directly-constructed service alike. #680 fixed a disagreement between
those two doors that would otherwise have shipped — the factory rejected `null`
where the service defaulted it.

**Coverage was corrected and re-scoped, not completed.** Five of the six routes
tracked as gaps turned out to be phantoms — already modelled at their flat
spelling — and both dock-door creation and schedule-recurrence writes were
deferred. Net coverage moved by one operation. See
[Coverage: corrected and re-scoped](#coverage-corrected-and-re-scoped).

## Before you upgrade — operator checklist

Four items here are invisible to a compiler and to a test suite that mocks the
network. Two of them are production incidents if you skip them.

### 1. Operation allowlists and denylists will start denying — or start passing

The merge-safe composites changed the operation identity your hooks observe. A
gating hook keyed on the old identity **denies the call** after upgrade; a
denylist keyed on it **stops blocking**. Nothing in your build catches this.

Every SDK reports operation identity differently. Use your own row.

**Go** and **Ruby** expose a short verb:

| SDK | v0.12.0 emitted | v0.13.0 emits |
|---|---|---|
| Go | `{Todolists, Update}` | `{Todolists, Get}` then `{Todolists, Replace}` |
| Go | `{Documents, Update}` | `{Documents, Get}` then `{Documents, Replace}` |
| Go | `{Schedules, UpdateEntry}` | `{Schedules, GetEntry}` then `{Schedules, ReplaceEntry}` |
| Ruby | `{todolists, update}` | `{todolists, get}` then `{todolists, replace}` |
| Ruby | `{documents, update}` | `{documents, get}` then `{documents, replace}` |
| Ruby | `{schedules, update_entry}` | `{schedules, get_entry}` then `{schedules, replace_entry}` |

**TypeScript, Python, Kotlin and Swift** put the wire operation ID in the same
field, so their strings differ:

| v0.12.0 emitted | v0.13.0 emits |
|---|---|
| `UpdateTodolistOrGroup` | `GetTodolistOrGroup` then `UpdateTodolistOrGroup` |
| `UpdateDocument` | `GetDocument` then `ReplaceDocument` |
| `UpdateScheduleEntry` | `GetScheduleEntry` then `ReplaceScheduleEntry` |

Two traps in that second table. The todolist pair kept its *names* — the
operation ID `UpdateTodolistOrGroup` did not change — so an allowlist
containing it keeps working for the write and **denies the new read**, which
fails the call just as completely. And `UpdateDocument` / `UpdateScheduleEntry`
no longer exist anywhere: an allowlist entry for either is now dead text.

Three operation IDs were also removed outright and will never match again:
`TrashTodo`, `GetRecording`, `CreateForwardReply`.

**Cards move the opposite way, and an allowlist is not automatically safe.**
[The cards fix](#cards-the-due-date-fix) is in this release: `cards.update` stops
issuing its `GetCard` and collapses to a single `UpdateCard`. Nothing starts being
denied — but if your allowlist names the write and deliberately omits the read,
that omission used to reject `cards.update` at its first request and now does
not. The same applies to a denylist on `GetCard`. Audit for gates that were
stopping a write **by way of** the read it used to make; those stop holding, and
no denial appears in your logs to tell you.

### 2. Request counts and rate-limit budget move

Each merge-safe `update` is now two HTTP requests. `downloadURL`'s first hop
retries up to three times (#563). Recount any per-call budget, request-count
assertion, or single-shot mock.

### 3. Path-keyed infrastructure needs repointing

Eleven operations emit a different URL. Proxy and WAF allowlists, log and APM
dashboards keyed on path, VCR/WebMock/MSW/nock/respx/`URLProtocol` handlers,
and recorded cassettes all match on the old spelling and will not match the new
one. Depending on the tool that is a passthrough, a wrong-fixture pass, or an
unrelated-looking failure.

### 4. `max_retries` of 0 or 1 breaks token refresh (#571)

If you run a **refreshable** token provider with `max_retries` set to 0 or 1,
an expired token on a read now raises an auth error instead of refreshing.
Raise it to at least 2. The default of 3 is unaffected, and a static token
provider is unaffected. Full detail in [Retry and transport](#retry-and-transport).

### 5. Clearing a card's due date is already broken in production

**This one is not caused by upgrading. It is true of the version you are
running right now**, and it is the reason to upgrade rather than a hazard of
doing so.

Every released SDK encodes "clear this card's due date" by **omitting**
`due_on` from the update body. That worked because bc3 built its card update
params as `{ due_on: nil }.merge(card_params)`, so an omitted key erased the
date. bc3 changed that: on the JSON representation an omitted key is now left
**unchanged**. The change deployed before any SDK release could match it.

So today, against production:

- `cards.update(...)` asking to clear a due date is a **silent no-op**. The
  request succeeds, returns 200, and the due date is still there.
- The same call has stopped being destructive in the other direction, which is
  the good half: a sparse update that never mentioned `due_on` used to erase it
  and no longer does.

**v0.13.0 is the release that fixes it.** `cards.update` now encodes a clear as
`"due_on": ""`, which bc3 blank-casts to nil — see
[Cards: the due-date fix](#cards-the-due-date-fix) for the SDK-side shape and
what it costs you in hook events. There is nothing to configure. Until you are on
this release, treat "clear a card due date" as unavailable and verify by reading
the card back.

---

# Breaks your compiler will not catch

Everything in this section survives a clean build. That is the only property
all of it shares, and it is why the section exists: the rest of the guide is
work your toolchain will find for you, and this is not.

Within it there are two classes, and they fail differently enough that mixing
them would be misleading:

**Class A — no signal at all (55).** Running your existing code against a live
Basecamp server, nothing tells you: it does not fail to compile, does not
raise, does not fail to decode, and does not change the shape of what you get
back. The call keeps working and does something different. These are the
dangerous ones, because there is no moment at which you find out.

**Class B — fails at runtime, on the wrong payload (6).** Compiles, then panics
or raises. You do get a signal; you get it late, from a stack trace, and only
sometimes.

**These six are classified for the payload that fails.** That is a real
limitation of the scheme and worth stating rather than hiding: class B is not a
property of the call, it is a property of the call *plus a response*. The same
method, against a response of the other shape, does not break at all — it
behaves exactly as it did at v0.12.0. So "is this class B?" has no answer until
you say which response you mean, and every entry below names its trigger.

That is also why the two classes need **opposite** test fixtures, and why "we
tested it against realistic data" is not evidence:

- **Absent-field triggers** (all four Go entries). A pointer that is nil
  because the server omitted the key. A fixture that populates every field never
  trips these — and against a fully-populated response they are not breaks.
- **Populated-field triggers** (the Ruby entry and the Kotlin one). Ruby's is a
  value that is now a `Time` rather than a `String`, so the failure needs the
  field to be *present*. Kotlin's is narrower still: the field must be present
  **and carry a JSON number or boolean where the model declares a string**. A
  fixture that leaves either nil never trips them — and against an absent field
  the behaviour is unchanged, because both versions did the same thing there.

Class A has no dependency on a *field's* presence — but that is not the same as
"every response", and the distinction matters when you go looking for one. Most
class-A entries do fire on every call of the affected method: the URL
corrections (#586), the `page` selector (#617), the merge-safe composites (#574,
#601, #632), the cards composite collapsing the other way (#647), the
empty-slice marshalling change (#560). Three groups do not, and their
precondition is a property of the *response* or of your *configuration*, not of
a field you can populate:

- **The error-message and validation entries (#541, #549)** need an error status
  to reach the code at all — a 2xx never composes an error message — and the
  field-map half additionally needs the body to be of a particular shape
  (recognition is all-or-nothing; see the Python entry). A suite that only
  exercises happy paths sees none of it.
- **`downloadURL`'s hop-1 retry (#563)** changes nothing until a network error
  or one of `{429, 502, 503, 504}` actually occurs. Against a healthy server it
  is indistinguishable from v0.12.0.
- **Ruby's floored attempt cap (#656)** is reachable only when you have set
  `max_retries` to 0 *and* the GET carries no operation ID. Every other
  configuration is bit-identical to v0.12.0 — see
  [the Ruby entry](#max_retries-0-now-sends-one-request-where-it-sent-none-656)
  before you go auditing.

| SDK | class A | class B |
|---|---:|---:|
| Go | 12 | 4 |
| Swift | 10 | 0 |
| TypeScript | 9 | 0 |
| Python | 8 | 0 |
| Ruby | 10 | 1 |
| Kotlin | 6 | 1 |

**How these are counted**, so the number can be checked against a rule rather
than an impression:

- One entry per distinct change, per SDK, counted in the SDK where it bites.
  A change that is a compile error in one SDK and silent in another appears
  only under the second.
- A change counts as class A if **any ordinary call-site shape** stays silent,
  even when another shape is compile-caught — annotated with which is which.
  Go's `Documents().Update` is the type case: silent for `pkg/basecamp`
  consumers, compile-caught only for direct `pkg/generated` importers.
- Where one change has a second face — a marshalling difference, a reformatted
  string — that face is annotated in place as *class-A residue* and counted
  once, against its parent change, not separately.
- Entries that raise only on a malformed or unusual **server** response are
  class B, not class A.

That rule-first stance is the contract for every number here: a count appears
in this document only with a stated derivation rule.

Neither class is a property of your **test suite**. Several class-A breaks
*will* fail loudly in a suite that pins request paths — a URL correction stops
matching a `WebMock`/`MSW`/`respx` stub, and a strict double raises on the
unregistered request. That is the good case, and it is called out where it
applies. It is not a contradiction: the break is silent in production, and your
mocks are the one thing that might catch it first. A suite that stubs loosely,
or matches on method and host only, catches nothing.

## Hits every SDK

### 1. `page` now selects one page instead of starting a walk (#617)

**Applies to TypeScript, Python, Ruby, Kotlin and Swift. Go changed
differently — [see below](#go-is-the-exception).**

At v0.12.0 in those five, seventeen already-paginated operations treated `page`
as a **starting offset**: the SDK put `page=N` on the first request and then
followed `Link: rel="next"` to the end of the collection. Now a positive `page`
means one request and no link-following, and `meta.truncated` reports whether a
further page existed.

**Wrong behaviour you get:** a job that read `page: 3` and processed
"everything from page 3 onward" now processes one page and reports success. If
that job drives a sync, the sync silently stops covering most of the
collection.

The seventeen: `ListMyBookmarks`, `ListMyDrafts`, `GetBubbleUps`, and the
fourteen auto-paginated `Everything*` readers (completed / no-due-date /
not-now / open / unassigned cards, checkins, comments, files, forwards,
messages, completed / no-due-date / open / unassigned todos).
`GetMyNotifications` also took `page` but never auto-paginated, so its meaning
is unchanged.

Fix: drop `page` entirely to get the old walk (cap it with
`maxItems`/`max_items`), or drive the page loop yourself and stop when
`meta.truncated` is false. Absent, `0` and negative all still walk the
collection.

#### Go is the exception

**Do not apply the fix above to Go.** At v0.12.0 a positive `Page` in Go
already meant *one request* — the wrapper returned before `followPagination`
whenever `Page > 0`. Dropping `Page` in Go does not restore old behaviour; it
converts a bounded, single-request call into a full traversal of the
collection.

What changed for Go is narrower, and splits in two:

- **Where the page number was already honoured** — `Bookmarks().List`,
  `Drafts().List`, the `Everything*` readers — v0.12.0 sent `page=N` and
  returned that page. **Nothing about the request changed.** `Meta.Truncated`
  is now populated where it used to be left false.
- **Where the page number was silently ignored** — the options structs whose
  v0.12.0 doc read *"Page, if non-zero, disables pagination and returns only
  the first page. NOTE: The page number itself is not yet honored due to
  OpenAPI client limitations."* Those built their params without `Page` at
  all, so they sent no `page` and returned **page 1's rows** under any
  positive `Page`. They now send `page=N` and return page N. Fourteen services
  carried that doc at v0.12.0: `cards`, `checkins`, `comments`, `events`,
  `forwards`, `messages`, `people`, `projects`, `recordings`, `schedules`,
  `timeline`, `todolists`, `todos`, `vaults`.

`Gauges().List` and `ListNeedles` are in neither group — they took no options
and no `page` at all at v0.12.0, so gaining both is purely additive.

So Go's break here is **wrong page returned**, not **walk collapsed**. A Go
job that passed `Page: 3` and quietly processed page 1 for months now processes
page 3 — which is what it always asked for, and a different set of rows than it
has been handling. Audit for code that compensated for the old behaviour.

#561 brought `page` to the rest of the list surface (18 → 56 parameter
structs). That half is purely additive.

### 2. Two operations emit a different URL under an unchanged signature (#586)

```
PUT  /{account}/todolists/{groupId}/position.json   →  /{account}/todolists/groups/{groupId}/position.json
GET  /{account}/inboxes/{inboxId}/forwards.json     →  /{account}/inboxes/{inboxId}/inbox_forwards.json
```

Same method name, same arguments, same return type, in all six SDKs. Both old
paths 404'd against bc3, so no working call is being taken away — but a test
double registered on the old path stops matching. Repoint mocks, cassettes,
proxy allowlists and path-keyed dashboards.

The nine #619 bucket-scoping rewrites also change URLs, but they additionally
change the call signature, so you will at least be looking at the code. These
two give you nothing.

### 3. Error message text changed, on more error shapes than you would expect (#541, #549)

400 and 422 responses now fold field detail into the message: `"color: is not a
valid color"`, or `"Invalid record (color: is not a valid color)"` when the body
also carries a top-level message. Fields sort lexicographically, a field's
messages join with `"; "`, fields join with `", "`. A bare unwrapped field map
(`{"content": ["can't be blank"]}`, no `errors` wrapper) is recognised too,
all-or-nothing by shape.

Separately, a top-level `"message"` key is now honoured as a general fallback at
**every** status, not just validation — so not-found, forbidden, auth and
generic API errors can carry different text than before.

**Wrong behaviour you get:** `if (e.message === "…")`, a regex over the message,
or an error-grouping key derived from it silently stops matching. The branch it
guarded stops running, and your error dashboard grows a new bucket.

Fix: branch on the error case or the HTTP status, and read the new structured
field map — `error.FieldErrors` (Go), `error.fieldErrors` (TypeScript, Kotlin,
Swift), `e.field_errors` (Python, Ruby). It is raw and untruncated; the message
is capped at 500 characters.

TypeScript users: this is bigger for you than for the others. At v0.12.0 *every*
error from a generated service call carried the HTTP status text, never the
server's message. See the [TypeScript](#typescript) section.

### 4. `update` on todolists, documents and schedule entries is a read-modify-write (#574, #601, #632)

Same method name, same request shape in most SDKs, same return type. What
changed:

- **Two HTTP requests, not one.**
- **Not atomic.** A concurrent write inside the GET→PUT window is overwritten.
  Last write wins on the whole representation.
- **Omission no longer clears.** Passing nothing for `description`/`content`
  used to erase it, because bc3 rebuilds the recordable from the permitted
  params it receives. Now it is preserved. To clear, pass `""` explicitly, or
  use the new `replace`.
- **Hook operation identity changed** — see the
  [operator checklist](#1-operation-allowlists-and-denylists-will-start-denying--or-start-passing).
- **Malformed read-backs are refused, not written through.** A GET body that is
  not an object, or a writable field that is not the type the spec claims,
  aborts before the PUT with a statusless API error.

## Go — 12 class A, 4 class B

Go carries every *panic*-shaped class-B break in the release; Ruby's and
Kotlin's raise instead. All four Go entries come from #560/#615/#658's
pointerization, and all four share one shape: Go auto-dereferences a pointer
for a field selector and for a value-receiver method call, so the old code
compiles untouched and panics **only when the server omits that field**.

### Class B — panics at runtime, on the wrong payload

1. **Five optional timestamps became `*time.Time` (#615).**
   `hc.UpdatedAt.IsZero()` still compiles and panics on nil. Fields:
   `HillChart.UpdatedAt`, `Notification.ReadAt`, `Notification.UnreadAt`,
   `SearchResult.CreatedAt`, `SearchResult.UpdatedAt`.
   *Class-A residue:* the two `SearchResult` tags also gained `omitempty`, so
   marshalling one now omits the key where it used to emit a fabricated
   `0001-01-01T00:00:00Z`. That half never raises — check persisted output and
   downstream strict decoders.
2. **Five *more* wrapper timestamps became `*time.Time` (#658).** #615's five
   were not the whole set — a second sweep found five that its `omitempty`-keyed
   check could not see. Fields: `QuestionReminder.RemindAt`,
   `ClientApprovalResponse.CreatedAt`, `ClientApprovalResponse.UpdatedAt`,
   `TimelineEvent.CreatedAt`, `WebhookDelivery.CreatedAt`. Identical failure
   shape to entry 1, so **audit ten wrapper fields, not five**.
   *Class-A residue:* all five gained `omitempty`, which none of them carried
   before, so marshalling now omits the key instead of emitting
   `0001-01-01T00:00:00Z`.
3. **~107 optional fields in `pkg/generated` with struct or named types became
   pointers (#560).** `a.Limits.CanUploadFiles` and `t.DueOn.String()` both
   compile and both panic. Of 653 value→pointer flips, 527 scalars and 19
   slices break at compile time; these ~107 do not.
4. **`Question.Schedule.Hour` and `.Minute` can now be nil (#560).** The one
   flip in the release running guaranteed-non-nil → nil, so
   `*q.Schedule.Hour` that was unconditionally safe now panics.
   *Class-A residue:* `WeekInstance`, `WeekInterval` and `MonthInterval` moved
   the other way — nil → non-nil pointer to 0 — so a `!= nil` presence test on
   those now fires where it did not, silently.

### Class A — no signal at all

1. **Nested optional objects switched to pointer presence (#560).** 34 guards
   across 17 files flipped from content-inference to `!= nil`. A
   present-but-empty object that used to yield nil now yields a non-nil struct.
2. **A non-nil empty slice now reaches the wire (#560).** `Types: []string{}`
   used to be a no-op; it now sends `{"types":[]}` and clears the list.
3. **`CardColumns().Move` always sends `position`, including 0 (#560).**
4. **`Page` is a selector (#561, #617)** — see cross-SDK #1, and the
   [Go carve-out](#go-is-the-exception).
5. **`Todolists().Update` is read-modify-write (#574)** — see cross-SDK #4.
6. **400/422 from the raw `Client`/`AccountClient` escape hatch now report
   `CodeValidation`, not `CodeAPI` (#549).**
7. **`pkg/generated` `Parse*Response` went lenient on 4xx/5xx (#541).**
8. **Two URLs changed (#586)** — see cross-SDK #2.
9. **`Documents().Update` is a read-modify-write emitting `ReplaceDocument`
   (#601).** The method signature and `UpdateDocumentRequest` are both
   unchanged, so every `pkg/basecamp` call site compiles untouched and silently
   becomes GET+PUT with preserve-on-omission and two hook events. Only direct
   `pkg/generated` importers get a compile error, from the `UpdateDocument*`
   symbols disappearing.
10. **Validation `Message` text and hint composition changed (#541).**
    `parseErrorBody` now decodes each member independently as `json.RawMessage`,
    so `{"error": {}, "error_description": "…"}` yields the hint where it
    previously yielded nothing. Typed service methods already returned
    `CodeValidation` for 400 and 422, so for most callers the *text* is the only
    thing that moved — and any string match on it is dead.
11. **`Cards().Update` dropped its preservation `GetCard` (#647).** The
    signature and `UpdateCardRequest` are unchanged, so every call site compiles
    untouched; what moves is the request count, the hook sequence and the
    encoding of a clear. See [Cards: the due-date fix](#cards-the-due-date-fix).
12. **`Schedules().CreateEntry` stopped validating `StartsAt`/`EndsAt` as
    RFC3339 (#664).** `CreateScheduleEntryRequest`'s two fields were already
    `string` and still are, so nothing about the call site changes — but the
    local `ErrUsage` guard is gone and the value goes on the wire verbatim. A
    bare date now creates an all-day entry where v0.12.0 refused it before the
    request left, and a genuinely malformed value now reaches bc3 instead of
    failing locally with `CodeUsage`. Code that used that error as its input
    validation has no validation.

One more is *partly* compiler-visible: `Error` gaining `FieldErrors` mid-struct
breaks unkeyed composite literals and nothing else. `Schedules().UpdateEntry` is
**not** in this class — `UpdateScheduleEntryRequest`'s fields became pointers, so
any `pkg/basecamp` call site that set even one field fails to compile. It is in
[Compile errors](#updatescheduleentryrequest-fields-became-pointers-632).

## Swift — 10 class A

1. **Error `message` recomposed at every status (#541).** The v0.12.0 fallback
   was `HTTPURLResponse.localizedString(forStatusCode:)` — measured on Darwin:
   400 → `"bad request"`, 422 → `"client error"`, 404 → `"not found"`. Lowercase
   and locale-dependent. Any string match on `error.message` is dead.
2. **`documents.update` became merge-safe under an identical signature (#601).**
   The generated `UpdateDocumentRequest` was renamed `ReplaceDocumentRequest`
   and a hand-written `UpdateDocumentRequest` with a character-identical public
   shape took the name.
3. **`schedules.updateEntry` became merge-safe under a superset signature
   (#632).** The new init's labels are a strict superset with pre-existing order
   preserved, so v0.12.0 call sites compile untouched.
4. **Two URLs changed (#586)** — `forwards.list` emits
   `/inbox_forwards.json`, and `todolistGroups.reposition` emits
   `/todolists/groups/{id}/position.json`. A regex stub on
   `todolists/\d+/position\.json` cannot match the new path and fails open.
5. **`todolists.update` is a merge-safe composite (#574, #628).** Mostly a
   compile error — but `try await account.todolists.update(id: 1, req:
   .init(name: "x"))` compiles unchanged, because leading-dot inference resolves
   `.init` against whichever type the parameter has and both expose
   `init(description:name:)`. That idiomatic shape silently becomes two requests
   with preserve-on-omission semantics. Call sites naming
   `UpdateTodolistOrGroupRequest` explicitly do get a type error.
6. **`page` selects a page (#617).** `BookmarksService`, `DraftsService`,
   `EverythingService` and `MyNotificationsService` all carried `page` at
   v0.12.0 and already appended `?page=`. What they did not do was stop
   link-following.
7. **A custom `Transport` that throws `BasecampError.network` is now retried
   (#592).** v0.12.0 failed any `BasecampError` from the transport on sight: one
   request, no `onRetry`, no backoff.
8. **A cancelled request now throws raw `URLError(.cancelled)` /
   `CancellationError` (#568).** `catch let error as BasecampError` no longer
   matches; the error escapes to your next handler.
9. **`downloadURL`'s first hop retries three times (#563).**
10. **`cards.update` dropped its preservation `GetCard` (#647).** The signature
    is byte-identical — `DueDate.preserve` still exists and still means "leave it
    alone", it just omits the key now instead of fetching and resending. One
    request, one hook event. See
    [Cards: the due-date fix](#cards-the-due-date-fix).

## TypeScript — 9 class A

1. **Every error message from a generated service call was previously the HTTP
   status text (#541).** At v0.12.0 `BaseService.handleError` discarded the body
   openapi-fetch had already parsed and re-read a spent `Response`, which throws,
   is swallowed, and falls back to `statusText`. So `403 {"error":"You are not
   allowed"}` gave `"Forbidden"`. On main the server's text reaches `message` at
   every status. **Any `e.message === "<statusText>"` comparison is now dead
   code.**
2. **Two operations emit different URLs under an unchanged signature (#586)**
   — see cross-SDK #2. The nine #619 bucket-scoping rewrites also move URLs but
   are compile errors here (TS2345 on `clientCorrespondences.list`, not TS2554),
   so they are in [Compile errors](#nine-operations-gain-a-leading-bucketid-619).
3. **Operation IDs seen by hooks were renamed, removed and doubled.**
   `OperationInfo.operation` is a plain `string`, so a stale comparison compiles
   and never matches again — an audit hook that was gating writes silently stops
   gating them.
4. **`page` selects a page (#617)** — see cross-SDK #1.
5. **`client.downloadURL()` now retries hop 1 (#563).** Single-shot mocks
   misbehave; if you wrapped `downloadURL` in your own retry loop you now have
   nested retry.
6. **`todolists.update()` is merge-safe (#574).** Nothing to change to compile;
   two requests per call and omission no longer clears. It additionally throws
   `Errors.usage` locally for `{ name: "" }`, which v0.12.0 sent and let bc3
   422 — not a happy path, so it does not disqualify the entry.
7. **`documents.update()` is merge-safe (#601).** `UpdateDocumentRequest` keeps
   its exact shape, so call sites are untouched.
8. **`schedules.updateEntry()` is merge-safe with a four-field carve-out
   (#632).** The request type was renamed, but the merge-safe type is a superset
   of the old field set, so an inline object literal — the common shape —
   compiles unchanged and changes semantics.
9. **`cards.update()` dropped its preservation `GetCard` (#647).**
   `UpdateCardRequest` is unchanged and every call site compiles untouched. One
   request instead of two, one hook event instead of two, and `dueOn: null` now
   goes on the wire as `""` rather than triggering a read-and-resend. See
   [Cards: the due-date fix](#cards-the-due-date-fix).

## Python — 8 class A

1. **`ValidationError` text changed and `field_errors` is new (#541, #549).**
   `str(e)` went from `"Validation failed"` to `"color: is not a valid color"`,
   and a bare unwrapped field map now populates `field_errors` too. Recognition
   is all-or-nothing by shape: one member that is not a non-empty list of
   non-empty strings disqualifies the whole body, so never assume a 400/422
   yields a field map.
2. **Two URLs changed (#586)** — see cross-SDK #2.
3. **`account.download_url`'s first hop retries and dropped its `Accept` header
   (#563).** v0.12.0 sent one request with `Accept: application/json` and raised
   on a 503. Main sends no `Accept` at all and retries `{429, 502, 503, 504}`
   plus network errors. Cassettes matching on `Accept` stop matching.
4. **`page` selects a page (#617)** — see cross-SDK #1. Measured with a mock
   transport that always returns a next link: v0.12.0
   `get_everything_open_todos(page=3)` issued 10,000 requests (the `max_pages`
   cap) starting at `?page=3`; main issues exactly one.
5. **`todolists.update()` is a merge-safe GET+PUT (#574).** Signature identical.
6. **`documents.update()` is a merge-safe GET+PUT (#601).**
7. **`schedules.update_entry()` is merge-safe (#632).** Every v0.12.0 keyword
   still binds.
8. **`cards.update()` dropped its preservation `get` (#647).** Keyword set
   identical. One request instead of two, one hook event instead of two, and
   `due_on=""` is now the clear encoding. See
   [Cards: the due-date fix](#cards-the-due-date-fix).

All three merge-safe composites keep byte-identical keyword sets, raise nothing
on the happy path, and produce no type-checker complaint — Python has no compile
step, so nothing anywhere warns you. The cards collapse is the same shape in
reverse, and just as quiet.

## Ruby — 10 class A, 1 class B

### Class A — no signal at all

1. **`page:` selects a page (#617)** — see cross-SDK #1. Measured against a
   4-page stub: v0.12.0 issued three requests and returned three items for
   `page: 2`; main issues one and returns one.
2. **`ValidationError#message` changed and `#field_errors` is new (#541, #549).**
   `e.message == "Request failed"` stops matching.
3. **Two URLs changed (#586)** — see cross-SDK #2. Loud in a stubbed suite,
   because WebMock raises on an unregistered request; silent against live bc3.
4. **`account.download_url`'s first hop retries and dropped its `Accept` header
   (#563).** v0.12.0 called `http.get_no_retry`, which sent
   `Accept: application/json` and did not retry. Main calls `get_download`,
   which is `request_with_retry(:get, url, retry_on: DOWNLOAD_RETRY_ON,
   accept: nil)` — so no `Accept` header at all, and up to three attempts on
   `{429, 502, 503, 504}` plus network errors. Cassettes matching on `Accept`
   stop matching, and single-shot download stubs see more requests.
5. **List methods request eagerly and return `ListEnumerator` (#557).**
   `enum = account.projects.list` is byte-identical source that now costs a
   request at call time. See [the detail below](#list-methods-now-request-eagerly-and-return-listenumerator-557)
   — errors move, and hook pairs no longer match per-iteration.
6. **`todolists.update` is a merge-safe GET+PUT (#574).**
7. **`documents.update` is a merge-safe GET+PUT (#601).**
8. **`schedules.update_entry` is a merge-safe GET+PUT (#632).** None of the
   three `update` keyword sets changed — only `replace`/`replace_entry`
   tightened — so every call site binds unchanged and behaves differently.
9. **`cards.update` dropped its preservation `get` (#647).** Keyword set
   identical. One request instead of two, one hook event instead of two, and
   `due_on: ""` is now the clear encoding. See
   [Cards: the due-date fix](#cards-the-due-date-fix).
10. **`max_retries: 0` now sends one request where it sent none (#656)** — see
    [the entry below](#max_retries-0-now-sends-one-request-where-it-sent-none-656).
    Narrow: only an *ungoverned* GET, only at `max_retries` 0.

### Class B — raises at runtime, on the wrong payload

1. **`Draft#scheduled_posting_at` and `MyNote#created_at`/`#updated_at` decode
   to `Time`, not `String` (#560).** `.start_with?` raises `NoMethodError` and
   `Time.parse(…)` raises `TypeError` — but only on a record where the field is
   populated, so a fixture that leaves it nil never trips either.
   `#iso8601` gets the old string back.
   *Class-A residue:* bare interpolation raises nothing and silently changes
   format from `"2026-01-02T03:04:05Z"` to `"2026-01-02 03:04:05 UTC"`, so
   anything writing that value into a log line, a cache key or an external
   payload changes what it emits with no error at all.

## Kotlin — 6 class A, 1 class B

1. **`documents.update` quietly stopped erasing omitted fields (#601).**
   `UpdateDocumentBody` was removed from the generator and re-declared by hand
   **in the same package with a character-identical public shape**, and
   `documents.update(id, body): Document` kept its exact signature. The call site
   compiles untouched and does something different.
2. **Two URLs changed (#586)** — see cross-SDK #2.
3. **Error message composition changed at every status (#541, #549)** — see
   cross-SDK #3. The same parser now backs `account.downloadURL`, so download
   failure messages moved too.

4. **`page` selects a page (#617)** — see cross-SDK #1. Seventeen operations,
   no build-time signal.
5. **`downloadURL` retries hop 1 (#563).** `DOWNLOAD_RETRY_ON = {429, 502, 503,
   504}` plus network errors, gated on `config.enableRetry` (default true). A
   single-shot 503 mock now sees three requests; 500 is deliberately not
   retried.
6. **`cards.update` dropped its preservation `get` (#647).** The signature is
   unchanged. One request instead of two, one hook event instead of two, and
   `dueOn = ""` is now the clear encoding. See
   [Cards: the due-date fix](#cards-the-due-date-fix).

### Class B — raises at runtime, on the wrong payload

1. **The client decoder stopped coercing a wrong-typed scalar into a `String`
   (#660).** No type, field or method signature moved — the only change is that
   the client-wide `Json` no longer sets `isLenient`. At v0.12.0 a response
   carrying `"description": 42` or `"title": false` decoded to `"42"` / `"false"`
   for any `String`/`String?` member on any model; it now throws a raw
   `kotlinx.serialization.SerializationException`.
   Two properties worth planning around. The trigger is a field that is
   **present and populated with a JSON number or boolean** — absence and explicit
   null are unaffected, so a fixture that omits the field never trips it. And the
   throw happens in the *response* decode, so on a write the mutation has already
   landed: `cards.update` issues its PUT, the card changes, and then the decode
   raises. `catch (e: BasecampException)` does not see it —
   `todolists.update`/`edit` are the exception and wrap it as
   `BasecampException.Api`.

Kotlin's other two merge-safe composites are **not** in class A because the
compiler does catch them: `todolists.update` takes a different body type and
`UpdateScheduleEntryBody` no longer exists. Only `documents.update` survives the
build, via the hand-written same-package shim, and that is class-A entry 1.

---

# Route corrections

Operations that declared URLs bc3 does not serve. **Read the removals carefully
— one of them was serving.**

## #586 — two spellings corrected, no signature change

Covered above as [class-A break #2](#2-two-operations-emit-a-different-url-under-an-unchanged-signature-586).
Both old paths were 404s.

## #619 — nine operations gained a bucket scope

Five campfire chatbot operations, `ListClientApprovals`,
`ListClientCorrespondences`, `ListClientReplies` and `GetClientReply` gained a
required leading bucket (project) ID and a `/buckets/{bucketId}` path prefix.

All nine flat paths were verified against bc3's `config/routes.rb`, not against
its documentation: the flat `chats` resource nests only `lines` and `uploads`
(no `integrations`), and the flat `client` namespace is `only: %i[show]` and
draws neither `recordings` nor `replies`. **Every prior call was a 404**, so
nothing that worked stops working. `GetClientApproval` and
`GetClientCorrespondence` are correctly flat and are untouched.

## #619 — three operations removed

`GetRecording` and `CreateForwardReply` were 404s too:

- **`GetRecording`** — bc3 draws `resources :recordings, only: []`, so the flat
  show does not exist. The bucket-scoped show is drawn, but
  `app/views/api/recordings/` holds only partials, so it cannot render on the API
  host either. There is no correct path in any shape. See
  [Known gaps](#recordingsget-has-no-generic-replacement).
- **`CreateForwardReply`** — the flat create was never drawn
  (`resources :inbox_forwards, only: %i[show]`). The bucket-scoped create exists
  but is undocumented with no upstream coverage, so it was not substituted in.
  No SDK replacement.

**`TrashTodo` is the exception, and the reassurance above does not cover it.**
`DELETE /{account}/todos/{todoId}` was drawn (`resources :todos, only: %i[show
edit update destroy]`), returned 204, and mutated data. It is the one removal
that takes away a call that worked.

What it did was not what its name said. `TodosController#destroy` writes
`destroy_status_param`, which **defaults to `"archived"`**; bc3's own test
asserts `archived?` after a bare DELETE. Every caller was archiving, not
trashing. It was removed rather than renamed so that this decision cannot be
skipped:

| you want | call |
|---|---|
| the behaviour you actually had | `recordings.archive(<todo id>)` |
| what the old name promised | `recordings.trash(<todo id>)` |

The compiler catches the removal. Nothing catches the wrong choice — do not sed
one into the other.

---

# Merge-safe composites

Three `update` methods were sparse PUTs. bc3 rebuilds the recordable from the
permitted params it receives, so a field you did not send was **erased**, with a
200 and no warning.

| Method | What omission used to erase |
|---|---|
| `todolists.update` (#574) | the list's description |
| `documents.update` (#601) | the document's content; an omitted title read back as "Untitled" |
| `schedules.updateEntry` (#632) | summary, start/end, description, and the all-day flag |

Each now issues a GET, overlays the fields you addressed, and PUTs the full
representation. The one-shot destructive PUT survives under an honest name —
`replace`, `replaceEntry` — and `edit` blocks give read-modify-write in one
call.

Schedule entries carry a **carve-out set** deliberately kept off the wire:
`participantIds`, `url` (the join link), `highlighted` and `notify` reach the
server only when the caller addresses them, because bc3 seeds them from the
existing recordable on omission. Echoing a stale read into those fields would be
wrong. Addressedness is key *presence*, not truthiness — `url: ""` is an explicit
clear.

#597 extends the same guards to the two composites that already existed.
Previously `todos.update` coalesced a `false` content to `""` and wrote it back
— erasing a field on a call that never mentioned it — and `cards.update` both
dropped a falsey non-string `due_on` (which is how bc3 erases a due date) and
forwarded a truthy non-string one.

---

# Cards: the due-date fix

Cards move the opposite way to the three composites above: `cards.update` was a
read-modify-write and becomes a **single PUT**. The reason is that the server
bug it existed to defend against is gone — bc3 became presence-aware on the card
JSON representation, so an omitted key is now left alone and there is nothing
left for a composite to protect.

The consequence for anyone still on an older SDK is
[operator checklist item 5](#5-clearing-a-cards-due-date-is-already-broken-in-production):
the released encoding for "clear the due date" is omission, and omission no
longer clears.

**Status:** shipped. #647 merged as `46b7f8225`, in all six SDKs.

It touches no schema — not `openapi.json`, not `spec/basecamp.smithy`, not
`go/pkg/generated`. An earlier draft of this guide said the fix would have to go
Smithy-first because `UpdateCardStepRequestContent.DueOn` could not express
`""`; that generated field did change from `types.Date` to `*types.Date` across
this release, but by #560's blanket pointerization, not by #647, and the card-step
wrapper never used that struct — it hand-builds a `map[string]any`. The two
changes rhyme and are unrelated.

### The wire encoding of an explicit clear changes

```
clear a due date:   omit "due_on"        →   "due_on": ""
```

bc3 blank-casts `""` to nil on the date attribute, so `""` clears. `null` is not
an option: it violates the body-compaction rule in SPEC §18, and five of the six
SDKs strip nulls before the wire, so `""` is the only clear encoding every SDK
can express identically.

### `UpdateStepRequest.DueOn` becomes `*string`

```go
// leave the step's due date alone
ac.CardSteps().Update(ctx, stepID, &basecamp.UpdateStepRequest{Title: "Draft"})

// clear it
ac.CardSteps().Update(ctx, stepID, &basecamp.UpdateStepRequest{DueOn: basecamp.Ptr("")})

// set it
ac.CardSteps().Update(ctx, stepID, &basecamp.UpdateStepRequest{DueOn: basecamp.Ptr("2026-08-14")})
```

Presence is `!= nil`, matching `UpdateCardRequest`. `Title` changes with it:
an empty `Title` now leaves the title unchanged rather than being sent, because
bc3 made title optional on update in the same change.

### The hook and request sequence collapses

`Cards().Update` no longer issues a `GetCard` first. In Go it is now literally
`return s.UpdateVerbatim(ctx, cardID, req)`.

**The v0.12.0 read was conditional, which narrows who is affected.** All six
SDKs took the GET only when the caller left `due_on` **unaddressed** — Go's
`if req.DueOn == nil`, Python's `current = self.get(...) if due_on is None`,
Kotlin's `dueOn == null ->`, and the equivalents in Ruby, TypeScript and Swift.
A call that named `dueOn` explicitly was already a single PUT and is unchanged.
The table below is therefore the *unaddressed-`due_on`* path:

| | v0.12.0 (`due_on` unaddressed) | v0.13.0 |
|---|---|---|
| wire operations | `GetCard` then `UpdateCard` | `UpdateCard` |
| Go `OperationInfo` | `{Cards, Get}` then `{Cards, UpdateVerbatim}` | `{Cards, UpdateVerbatim}` |
| requests per call | 2 | 1 |
| lost-update window | yes — a concurrent due-date change between GET and PUT was overwritten | none |

This is the **inverse** of the `{Todolists, Update}` split in
[operator checklist item 1](#1-operation-allowlists-and-denylists-will-start-denying--or-start-passing),
so do not reason about it by analogy. There the added read could be *denied*;
here the removed read means a gate that used to fire no longer does. **Audit
both lists — an allowlist is not safe just because nothing new appears on it.**

- **An allowlist can silently open.** Nothing starts being denied, which is the
  part that misleads. But if your allowlist names `UpdateCard` /
  `{Cards, UpdateVerbatim}` and deliberately omits `GetCard` / `{Cards, Get}`,
  then `cards.update` used to be rejected **at its read** and never reached the
  write. Collapsed to a single call, it is permitted end to end. A gate you were
  relying on stops holding, and no denial appears in your logs to tell you.
- **A denylist can silently open the same way.** If you blocked
  `{Cards, Get}` / `GetCard` to stop reads, that denial used to take
  `cards.update` down with it. It no longer does.
- **Audit trails lose a record.** Anything reconciling reads against writes, or
  billing per operation, sees one event where it saw two.

Those first two are the **same** hole seen from two policy shapes, which is why
reading only one is dangerous: in both, the thing actually stopping the write was
the *read*, expressed once as an omission from an allowlist and once as an entry
on a denylist. Remove the read and both stop working, for identical reasons.

The general rule, which is easy to get backwards: removing an operation from a
composite cannot cause a *denial*, but it can remove a denial you were depending
on. Gate on the write you actually mean to stop, not on a read that happened to
accompany it.

### A defended defect class leaves the Cards surface

Removing the preservation GET also removes what that GET was validated against.
Three `errorRaised` conformance kill cases go with it:

```
update-kill: an array due_on is refused before the replacement PUT
update-kill: an empty-object due_on is refused, not coerced or dropped
update-kill: a date-shaped array due_on is refused where the format check is blind
```

Those pinned the behaviour that a malformed `due_on` **read back from the
server** was refused rather than coerced or forwarded into the write. With no
read, there is no read-back to validate, so the guarantee is not weakened — it
stops being reachable on this surface. `cards_write.json` goes from 8 cases to 5,
and its `errorRaised` count from 3 to 0.

**The class itself is not retired.** It stays pinned on Todos, which still does
a real read-modify-write: `todos_write.json` carries 3 `errorRaised` cases — the
array and empty-object kills it already had, plus a bare-scalar kill added by
#660. If you were relying on Cards to be the canary for malformed-read-back
handling, it is not one any more — Todos is, and it now covers one shape more
than Cards ever did.

---

# Retry and transport

- **#571 — the 401 refresh replay now counts against the attempt budget, and the
  budget is checked *before* `refresh()` is invoked.** Consumers running a
  refreshable token provider with `max_retries` of 0 or 1 now get an auth error
  where the token used to refresh silently. Raise it to at least 2; the default
  of 3 is unaffected. Reads only — mutations keep the uncounted replay. Even at
  ≥2 the replay spends an attempt, so a token rotation consumes a retry you may
  have been relying on for a following 429 or 503.
- **#563 — the authenticated download hop retries** under a declared set (`429,
  502, 503, 504` plus network errors; never 500) in every SDK. Downloads that
  used to fail fast now recover, and single-shot download mocks now see up to
  three requests. **Python and Ruby** additionally stopped sending
  `Accept: application/json` on hop 1 — `accept=None` in
  `python/src/basecamp/_http.py`, `accept: nil` in `ruby/lib/basecamp/http.rb` —
  so a VCR cassette or WebMock stub that matches on `Accept` stops matching. The
  other four never sent it on that hop, at v0.12.0 or now, so nothing moved
  there.
- **#592 — the backoff formula gained a 30 s ceiling** in all six SDKs. Swift
  additionally retries a custom `Transport`'s own `BasecampError.network`
  instead of failing on sight.
- **#568 — Swift classifies cancellation as terminal** and rethrows it raw, so a
  cancelled request no longer burns the retry budget and no longer arrives as a
  `BasecampError`.
- **#557 — Ruby's list enumerators carry pagination metadata and accept
  `max_items`.** Page 1 is now fetched inside the call rather than on first
  iteration: errors surface at construction, and `meta.total_count` is available
  before you iterate.

---

# Go

Go has the most invasive changes in this release. Sixteen survive a clean
build: twelve give no signal at all, and four compile and then panic — see
[Go — 12 class A, 4 class B](#go--12-class-a-4-class-b) for the split.

The scale, so you can size the work before starting: `pkg/basecamp`'s exported
surface now carries **300 pointer-typed fields** — `*string` ×81, `*bool` ×23,
`*time.Time` ×16, 14 pointer-to-slice, and the rest struct pointers. Derive it
yourself with:

```bash
rg -N '^\s{1,2}[A-Z]\w*\s+\*[\w.\[\]]+\s' go/pkg/basecamp/*.go \
  | rg -v '_test\.go' | wc -l
```

Both halves of that round trip have an exported helper as of #643 — you do not
need to hand-roll one:

```go
basecamp.Ptr(v)   // func Ptr[T any](v T) *T   — set an optional field
basecamp.Deref(p) // func Deref[T any](p *T) T — read one, zero value when nil
```

`Ptr` never collapses `false` or `""` to nil: sending an explicit zero is the
whole reason these fields are pointers. `T` is inferred from the argument, so a
field whose type is not an untyped literal's default needs the conversion
written out — `basecamp.Ptr(int32(5))` for an `*int32` field, not
`basecamp.Ptr(5)`.

`Deref` is total, which makes it the wrong tool where absence carries meaning:
collapsing "the server omitted this" into `""` is only safe when your code
cannot tell the two apart. Compare against nil where it can. See
[Optional Fields](go/README.md#optional-fields) in the Go README.

## Silent

### Five optional timestamps became `*time.Time` (#615)

```go
// v0.12.0 — all five were value time.Time
if !hc.UpdatedAt.IsZero() { fmt.Println(hc.UpdatedAt.Format(time.RFC3339)) }
for _, n := range notifications {
    if n.ReadAt.IsZero() { unread = append(unread, n) }
}
```

```go
// main — the same lines COMPILE and panic on a nil pointer
if hc.UpdatedAt != nil && !hc.UpdatedAt.IsZero() { … }
for _, n := range notifications {
    if n.ReadAt == nil { unread = append(unread, n) }
}
```

Go inserts the dereference for a value-receiver method call on a pointer, so
`hc.UpdatedAt.IsZero()` is rewritten to `(*hc.UpdatedAt).IsZero()` and builds
clean. Assignments (`var t time.Time = n.ReadAt`) and arguments typed `time.Time`
**do** fail to compile; only selector-and-method access is silent.

Grep the five field names and audit every `.IsZero()`, `.Format(`, `.Before(`,
`.After(`, `.Sub(`, `.Unix()`, `.Year()`. To keep the old zero-value semantics
verbatim:

```go
t := basecamp.Deref(n.ReadAt)   // time.Time{} when the field is absent
```

That is exactly the old behaviour, because the old value field was the zero
time when absent. Only reach for it where absence and a genuine zero are
interchangeable to your code — `if n.ReadAt == nil` is the honest test when
they are not.

Two consequences past the panic: absence used to read as
`0001-01-01T00:00:00Z` and now reads as nil, and `json.Marshal` of a
`SearchResult` now omits `created_at`/`updated_at` when absent. Check persisted
serialization and downstream strict decoders.

### Five *more* wrapper timestamps became `*time.Time` (#658)

#615's five were not the whole set, and the check it shipped could not tell you
so: `TestNoValueTypedOptionalTimestamps` keyed on the `,omitempty` tag, and these
five did not carry one. A second sweep pairs each wrapper field against its
generated counterpart by struct name and json key, and found five more:

| File | Struct | Field |
|---|---|---|
| `go/pkg/basecamp/checkins.go` | `QuestionReminder` | `RemindAt` |
| `go/pkg/basecamp/client_approvals.go` | `ClientApprovalResponse` | `CreatedAt` |
| `go/pkg/basecamp/client_approvals.go` | `ClientApprovalResponse` | `UpdatedAt` |
| `go/pkg/basecamp/timeline.go` | `TimelineEvent` | `CreatedAt` |
| `go/pkg/basecamp/webhooks.go` | `WebhookDelivery` | `CreatedAt` |

The failure is identical to #615's — `ev.CreatedAt.Format(time.RFC3339)`
compiles, and panics when the server omits `created_at`. **So the audit is ten
fields, not five.** Watch the near-miss siblings, which did *not* change and are
easy to sed by accident: `Webhook.CreatedAt`/`UpdatedAt`,
`QuestionAnswer.CreatedAt`/`UpdatedAt`, and `ClientApproval.CreatedAt`/
`UpdatedAt` — note that the last pair is on `ClientApproval`, while the pair that
*did* move is on `ClientApprovalResponse`.

All five also gained `,omitempty`, which none of them carried before, so
`json.Marshal` now drops the key instead of emitting `0001-01-01T00:00:00Z`.

### `pkg/generated`: ~107 struct- and time-typed optional fields became pointers (#560)

```go
// both lines still COMPILE on main, and BOTH panic when the field is nil
_ = a.Limits.CanUploadFiles     // Account.Limits is now *AccountLimits
fmt.Println(t.DueOn.String())   // Todo.DueOn is now *types.Date
```

The method call is no safer than the field selector. `types.Date.String` has a
**value** receiver (`func (d Date) String() string`, `go/pkg/types/date.go`), so
Go rewrites `t.DueOn.String()` to `(*t.DueOn).String()` and the dereference of a
nil pointer panics before `String` is entered. The same holds for every
value-receiver method on these types — `IsZero`, `Before`, `After`, `Weekday` on
`types.Date`; `Format`, `Sub`, `Unix`, `Year` on `time.Time`. Only a
*pointer*-receiver method would survive a nil receiver, and it would then have to
handle nil in its own body; none of these do. Audit the method calls with the
same care as the field selectors.

Of 653 value→pointer flips in `pkg/generated`, 527 scalars and 19 slices break
at compile time. The other ~107 have a named or struct type with fields or
methods, so Go auto-dereferences for a field selector and for a value-receiver
method call: `types.Date` (24), `time.Time` (16), `Person` (15),
`RecordingParent` (7), `RecordingBucket` (7), `types.FlexInt` (2),
`types.FlexibleTime` (2), `TodoBucket` (2), `QuestionSchedule` (2), plus ~30
singletons (`EventDetails`, `MessageType`, `Project`, `Recording`,
`ClientCompany`, `CardColumnOnHold`, `TimelineEventData`, `WebhookCopy`, …).

This only affects you if you import
`github.com/basecamp/basecamp-sdk/go/pkg/generated` directly — no exported
`pkg/basecamp` function or field mentions a generated type.

### Nested optional objects switched to pointer presence (#560)

```go
// v0.12.0 — presence was inferred from CONTENT
if ge.Details.AddedPersonIds != nil || ge.Details.RemovedPersonIds != nil { … }
if gf.Parent.Id != 0 || gf.Parent.Title != "" { … }
if !gc.CompletedAt.IsZero() { … }
```

```go
// main — presence IS the pointer
if ge.Details != nil { … }
if gf.Parent != nil { … }
if gc.CompletedAt != nil { … }
```

34 guards across 17 files: `client_approvals` (4); `todolist_groups`, `search`,
`recordings`, `everything`, `cards` (3 each); `webhooks`, `timeline`, `reports`,
`boosts` (2 each); `vaults`, `tools`, `timesheet`, `templates`, `projects`,
`people`, `messages` (1 each).

The flip always runs the same direction — a present-but-empty or
present-but-zero object that used to yield nil now yields a non-nil struct. The
canonical fixture emits `"details": {}` for events with no membership change.
Sharpest: `Card.CompletedAt` and `CardStep.CompletedAt`, where a present zero
timestamp now reads as "completed". So stop treating a non-nil nested pointer as
"this thing happened":

```go
if e.Details != nil && (len(e.Details.AddedPersonIDs) > 0 || len(e.Details.RemovedPersonIDs) > 0) {
    log.Printf("membership changed on %d", e.RecordingID)
}
```

Audit every `x.Parent != nil`, `x.Bucket != nil`, `x.Creator != nil`,
`x.Category != nil`, `x.CompletedAt != nil`, `x.DueOn != nil`,
`x.Schedule != nil`.

### `Question.Schedule.Hour` and `.Minute` can now be nil (#560)

```go
// v0.12.0: Hour/Minute were ALWAYS non-nil whenever Schedule was set
if q.Schedule != nil {
    fmt.Printf("%02d:%02d\n", *q.Schedule.Hour, *q.Schedule.Minute)  // always safe
}

// main: the generated nil is carried through — this panics
if q.Schedule != nil && q.Schedule.Hour != nil && q.Schedule.Minute != nil { … }
```

The only flip in the release running guaranteed-non-nil → nil. In the same hunk,
`WeekInstance`, `WeekInterval` and `MonthInterval` moved the *other* way: they
used to be nil when the server sent 0 and are now a non-nil pointer to 0, so a
`!= nil` presence test on those now fires where it did not.

### A non-nil empty slice now reaches the wire (#560)

```go
// v0.12.0 guards were len()-based
ac.Webhooks().Update(ctx, id, &basecamp.UpdateWebhookRequest{Types: []string{}})
// → PUT with no "types" key: the webhook's event list was left alone

// main guards are nil-based and the generated field is *[]T
ac.Webhooks().Update(ctx, id, &basecamp.UpdateWebhookRequest{Types: []string{}})
// → PUT {"types":[]}: the webhook's event list is CLEARED
```

nil now means "leave it alone"; `[]T{}` means "set it to empty". If you build a
slice with `make([]T, 0)` and append conditionally, a zero-match filter now
clears the server-side list. Affected: `Webhooks().Update` (Types),
`Gauges().CreateNeedle` (Subscriptions), `Todos().Create`/`CreateInTodoset`
(AssigneeIDs, CompletionSubscriberIDs), `CardSteps().Create` (AssigneeIDs),
`Schedules().CreateEntry` (ParticipantIDs), `People().UpdateProjectAccess`
(Grant, Revoke, Create), `QuestionSchedule.Days`. `Todos().Update`/`Replace`
already used the nil guard.

### `CardColumns().Move` always sends `position` (#560)

`MoveColumnRequest.Position` lost `omitempty`, and the value is range-checked
then sent unconditionally: `POST {"source_id":a,"target_id":b,"position":0}`
where the key used to be absent. bc3 does `params[:position].to_i` so the server
outcome is unchanged, but body-exact stubs and any payload you marshal yourself
both change. A `Position` above `2147483647` is now
`ErrUsage("position must be between 0 and 2147483647")` instead of a silent wrap
to a negative int32.

### `Page` is a page selector (#561, #617)

```go
// v0.12.0 doc: "Page, if non-zero, disables pagination and returns only the
// first page. NOTE: the page number itself is not yet honored."
res, _ := ac.Todos().List(ctx, todolistID, &basecamp.TodoListOptions{Page: 3})
// → GET .../todos.json — page 1's rows, Meta.Truncated always false

// main
res, _ := ac.Todos().List(ctx, todolistID, &basecamp.TodoListOptions{Page: 3})
// → GET .../todos.json?page=3 — page 3's rows, Meta.Truncated populated
```

If you passed `Page` purely to mean "one request, don't auto-paginate", you now
get page N. A positive `Limit` alongside a positive `Page` trims that page.
`Page > 2147483647` is now `ErrUsage("page is out of range")`.

18 → 56 params structs carry `Page`; 38 wrapper call sites now pass one.

### `Todolists().Update` is now read-modify-write (#574)

Signature unchanged; the data-loss bug is fixed for free. Three things move: two
round-trips per call; non-atomic last-write-wins; and hooks now observe
`{Todolists, Get}` then `{Todolists, Replace}` — **no `{Todolists, Update}` is
emitted anywhere on main.**

```go
ac.Todolists().Update(ctx, id, &basecamp.UpdateTodolistRequest{Name: "Q3"})  // merge-safe
ac.Todolists().Edit(ctx, id, func(f *basecamp.TodolistFields) error { f.Description = ""; return nil })
ac.Todolists().Replace(ctx, id, &basecamp.ReplaceTodolistRequest{Name: "Q3"}) // verbatim, clears description
```

`Update` cannot clear a field — an empty string still reads as "unaddressed".
Use `Edit` or `Replace`.

### Raw escape-hatch 400/422 now report `CodeValidation` (#549)

If you already handle errors from the raw `Client`/`AccountClient`
`Get`/`Post`/`Put`/`Delete` methods, the code moved:

```go
if apiErr, ok := err.(*basecamp.Error); ok {
    switch apiErr.Code {
    case basecamp.CodeAPI:        // a 422 used to land here
    case basecamp.CodeValidation: // it lands here now, with FieldErrors populated
    }
}
```

Only those four changed; typed service methods already returned
`CodeValidation` for both statuses. Match on `apiErr.HTTPStatus` if you need
the old grouping.

This entry documents a change to an existing surface. It is **not** a
suggestion to reach for the raw client — see
[no raw-wire migrations](#there-is-no-raw-wire-migration-path).

### `pkg/generated` `Parse*Response` went lenient on 4xx/5xx (#541)

`if err := json.Unmarshal(...); err != nil { return nil, err }` became
`if err == nil { response.JSONxxx = &dest }` across all 1058 status arms. A nil
`resp.JSONxxx` no longer means "the status did not occur" — it can also mean
"the body did not decode". Check `resp.HTTPResponse.StatusCode` and `resp.Body`.
For `pkg/basecamp` users,
`if apiErr, ok := err.(*basecamp.Error); ok { … } else { /* parse error */ }`
now takes the first branch for malformed error bodies.

### Two URLs changed (#586)

`RepositionTodolistGroup` and `ListForwards`. Go call sites are byte-identical.

### `Cards().Update` dropped its preservation `GetCard` (#647)

`UpdateCardRequest` and the method signature are unchanged, so nothing in your
build moves. `Update` is now literally `return s.UpdateVerbatim(ctx, cardID, req)`
— one request where a call leaving `DueOn` nil used to make two, one
`OperationInfo` where there were two, and `DueOn: basecamp.Ptr("")` as the clear
encoding. Full detail, including what it does to allowlists and denylists, in
[Cards: the due-date fix](#cards-the-due-date-fix).

### `Schedules().CreateEntry` no longer validates the timestamps (#664)

```go
req := &basecamp.CreateScheduleEntryRequest{Summary: "Offsite",
    StartsAt: "2026-06-01", EndsAt: "2026-06-01"}

// v0.12.0 — never reached the wire
_, err := ac.Schedules().CreateEntry(ctx, scheduleID, req)
// err.Code == CodeUsage: "schedule entry starts_at must be in RFC3339 format …"

// main — sent verbatim, creates an all-day entry
_, err := ac.Schedules().CreateEntry(ctx, scheduleID, req)
```

`CreateScheduleEntryRequest.StartsAt` and `.EndsAt` were `string` at v0.12.0 and
still are; only the set of values they accept widens. bc3 takes a bare date for
an all-day entry and a timestamp otherwise, and the client-side
`time.Parse(time.RFC3339, …)` guard rejected the first — so the fix is
unambiguously in the right direction. What is silent is the other half: **a
genuinely malformed value now reaches bc3** instead of failing locally with
`CodeUsage`, and code that leaned on that error as its own input validation has
none. The `""`-is-required guards survive.

## Compile errors

### Nine operations gained a leading `bucketID` (#619)

```go
// before                                    // after
ac.Campfires().ListChatbots(ctx, cid, nil)   ac.Campfires().ListChatbots(ctx, bucketID, cid, nil)
ac.Campfires().GetChatbot(ctx, cid, bid)     ac.Campfires().GetChatbot(ctx, bucketID, cid, bid)
ac.Campfires().CreateChatbot(ctx, cid, req)  ac.Campfires().CreateChatbot(ctx, bucketID, cid, req)
ac.Campfires().UpdateChatbot(ctx, cid, b, r) ac.Campfires().UpdateChatbot(ctx, bucketID, cid, b, r)
ac.Campfires().DeleteChatbot(ctx, cid, bid)  ac.Campfires().DeleteChatbot(ctx, bucketID, cid, bid)
ac.ClientApprovals().List(ctx, nil)          ac.ClientApprovals().List(ctx, bucketID, nil)
ac.ClientCorrespondences().List(ctx, nil)    ac.ClientCorrespondences().List(ctx, bucketID, nil)
ac.ClientReplies().List(ctx, rid, nil)       ac.ClientReplies().List(ctx, bucketID, rid, nil)
ac.ClientReplies().Get(ctx, rid, replyID)    ac.ClientReplies().Get(ctx, bucketID, rid, replyID)
```

The bucket ID is `project.ID` / `Bucket.ID` on any recording you already hold.
`GetClientApproval` and `GetClientCorrespondence` are correctly flat and
untouched. `pkg/generated` users: the same nine gained the parameter across 55
signature sites.

### `Todos().Trash` removed (#619) — and it never trashed

```go
err := ac.Todos().Trash(ctx, todoID)        // gone
err := ac.Recordings().Archive(ctx, todoID) // what you were ACTUALLY getting
err := ac.Recordings().Trash(ctx, todoID)   // what the name promised
```

See [Route corrections](#619--three-operations-removed). The compiler catches
the removal; nothing catches the wrong choice.

### `Recordings().Get` removed (#619)

Use `Recordings().List(ctx, recordingType, opts)` and filter, or the
type-specific service. See [Known gaps](#recordingsget-has-no-generic-replacement).

### `Forwards().CreateReply` and `CreateForwardReplyRequest` removed (#619)

There is no supported replacement — see
[Known gaps](#there-is-no-raw-wire-migration-path). Reads are unaffected:
`Forwards().ListReplies` and `Forwards().GetReply` remain.

### `UpdateScheduleEntryRequest` fields became pointers (#632)

```go
_, err := ac.Schedules().UpdateEntry(ctx, entryID, &basecamp.UpdateScheduleEntryRequest{
    Summary:        basecamp.Ptr("Standup"),
    StartsAt:       basecamp.Ptr("2026-01-01T09:00:00Z"),
    EndsAt:         basecamp.Ptr("2026-01-01T09:15:00Z"),
    ParticipantIDs: basecamp.Ptr([]int64{7, 9}),
    Notify:         basecamp.Ptr(true),
})
```

The pointers are load-bearing: nil means "leave the fetched value alone", a
pointer to the zero value means "set it to empty" —
`Description: basecamp.Ptr("")` clears it. Two new fields: `URL` (the join link)
and `Highlighted`. `AllDay` was already `*bool`. **`StartsAt`/`EndsAt` are no
longer RFC3339-validated client-side** — a malformed timestamp now reaches the
server instead of failing locally, so bc3's bare-date all-day rendering
round-trips.

**The sharpest one to get wrong is `ParticipantIDs *[]int64`,** where nil and
empty are different instructions rather than degrees of the same one:

```go
ParticipantIDs: nil,                        // leave the participants alone
ParticipantIDs: basecamp.Ptr([]int64{}),    // remove EVERY participant
ParticipantIDs: basecamp.Ptr([]int64{7,9}), // set the participants to 7 and 9
```

A `[]int64` you build by filtering is `basecamp.Ptr(ids)` either way — so a
filter that matches nothing clears the entry's participant list instead of
leaving it untouched. Guard on `len(ids) > 0` if you meant "leave it alone".

### `Gauges().List` and `ListNeedles` gained options and a result struct (#617)

```go
res, err := ac.Gauges().List(ctx, nil)                    // *GaugeListResult
nres, err := ac.Gauges().ListNeedles(ctx, projectID, nil) // *GaugeNeedleListResult
fmt.Println(len(res.Gauges), res.Meta.TotalCount, res.Meta.Truncated)
```

`nil` opts reproduces v0.12.0 behaviour exactly (uncapped Link-header
pagination).

### `TodolistGroups().Update` removed in favour of `Replace` (#574)

**Do not blind-rename.** `ReplaceTodolistGroupRequest.Description` has
`omitempty`, and bc3 rebuilds the recordable from the permitted params — a
`Replace` that omits `Description` **erases the group's description**. For
merge-safe behaviour, route group writes through the todolists endpoint, which
is the same `PUT /{accountId}/todolists/{id}`:

```go
ac.Todolists().Update(ctx, groupID, &basecamp.UpdateTodolistRequest{Name: "Design"})
```

There is deliberately no `TodolistGroups().Update` or `.Edit`.

### `UpdateGaugeNeedleRequest.Description` became `*string` (#560)

Tri-state: nil leaves it untouched, `basecamp.Ptr("")` clears it. At v0.12.0 an empty
string was indistinguishable from unset and could not clear.

### `UpdateStepRequest.DueOn` became `*string` (#647)

```go
// leave the step's due date alone
ac.CardSteps().Update(ctx, stepID, &basecamp.UpdateStepRequest{Title: "Draft"})

// clear it
ac.CardSteps().Update(ctx, stepID, &basecamp.UpdateStepRequest{DueOn: basecamp.Ptr("")})

// set it
ac.CardSteps().Update(ctx, stepID, &basecamp.UpdateStepRequest{DueOn: basecamp.Ptr("2026-08-14")})
```

Presence is `!= nil`, matching `UpdateCardRequest`, whose `DueOn` was already
`*string` at v0.12.0 and did not move. `Title` changes with it: an empty `Title`
now leaves the title unchanged rather than being sent, because bc3 made title
optional on update in the same change. This is the one part of #647 the compiler
finds for you; the rest is [class A](#go--12-class-a-4-class-b).

### `UpcomingSchedule` returns reduced types, and `Assignable` is gone (#648)

`GetUpcomingSchedule` declared the full `ScheduleEntry` schema while bc3 renders
it through a reduced calendar partial, so the SDK promised fields the endpoint
never sends and its converters zero-filled them silently. The response type is
now a set of aliases onto purpose-built shapes:

```go
type (
    UpcomingScheduleResponse     = generated.GetUpcomingScheduleResponseContent
    UpcomingScheduleEntry        = generated.UpcomingScheduleEntry
    UpcomingAssignable           = generated.UpcomingAssignable
    UpcomingScheduleBucket       = generated.UpcomingScheduleBucket
    UpcomingSchedulePerson       = generated.UpcomingSchedulePerson
    UpcomingAssignableParent     = generated.UpcomingAssignableParent
    UpcomingAssignableCompletion = generated.UpcomingAssignableCompletion
)
```

`ReportsService.UpcomingSchedule(ctx, startDate, endDate)` keeps its signature;
everything else about the result moves, and every move is a build failure:

- **`basecamp.Assignable` and `generated.Assignable` are deleted.**
- **`Assignable.Title` is now `UpcomingAssignable.Content`.** bc3 has always
  emitted `content` and never `title`, so the field you were reading was
  permanently the zero value. This is the correction, not a regression.
- **`UpcomingScheduleResponse.RecurringOccurrences` → `RecurringScheduleEntryOccurrences`.**
- Because the aliases publish generated names, the initialisms flip: `ID`→`Id`,
  `URL`→`Url`, `AppURL`→`AppUrl`.
- `DueOn`/`StartsOn` go `string` → `*types.Date`; `Bucket` and `Parent` become
  the narrowed value types `UpcomingScheduleBucket{Id, Name}` and
  `UpcomingAssignableParent{Id, Title}`; `Assignees` becomes
  `[]UpcomingSchedulePerson`. `UpcomingScheduleEntry` drops fourteen members the
  partial never rendered and gains `Recurring`.
- `StartsAt`/`EndsAt` on the entry are `types.FlexibleTime`, which is what
  `basecamp.ScheduleEntry` already used — not a change for readers.

Also new and loud: an empty `startDate` or `endDate` is now
`ErrUsage("window_starts_on is required")` rather than a bc3 400.

### `pkg/generated` only

- **546 optional scalar and slice fields became pointers (#560),** enforced by
  `scripts/check-go-optional-pointers` in `make check`. Two silent holes remain
  even for scalars: `fmt.Println(a.OwnerName)` compiles and prints a pointer
  address, and `%v`/`%s` on a `*string` yields the address.
- **22 read operations gained a `params *XxxParams` argument (#561)** — 128
  signature sites. Pass `nil` to keep the old wire behaviour. The argument goes
  last among the fixed parameters, before the variadic `reqEditors`. Affected:
  `GetAnswersByPerson`, `GetPersonProgress`, `GetProgressReport`,
  `GetProjectTimeline`, `GetQuestionReminders`, `ListAnswers`, `ListCampfires`,
  `ListCards`, `ListClientReplies`, `ListComments`, `ListDocuments`,
  `ListEventBoosts`, `ListEvents`, `ListForwardReplies`, `ListGaugeNeedles`,
  `ListPeople`, `ListProjectPeople`, `ListQuestions`, `ListRecordingBoosts`,
  `ListTodolistGroups`, `ListUploads`, `ListVaults`.
- **`ClientInterface` and `ClientWithResponsesInterface` each lost 8 methods and
  gained 12.** Any hand-written mock asserting
  `var _ generated.ClientInterface = (*fake)(nil)` fails until you add
  `CreateFolder`, `CreateFolderWithBody`, `DeleteFolder`,
  `DestroyTimesheetEntry`, `GetFolder`, `ListFolders`, `ReplaceDocument`,
  `ReplaceDocumentWithBody`, `ReplaceScheduleEntry`,
  `ReplaceScheduleEntryWithBody`, `UpdateFolder`, `UpdateFolderWithBody`.
  Consider embedding the interface.
- **`UpdateMyNoteResponse.JSON422` and `UpdateMyPreferencesResponse.JSON422`
  changed type** to `*FieldValidationErrorResponseContent` (#549). These are the
  only type-changed fields that are not a plain pointer flip, so a mechanical
  migration misses them.
- **`CreateScheduleEntryRequestContent.StartsAt` and `.EndsAt` went `time.Time`
  → `string` (#664),** and so did `CreateScheduleEntryJSONRequestBody`, its
  alias. A `time.Time` in one of those literals no longer compiles; pass the
  string you actually want on the wire. The sibling
  `ReplaceScheduleEntryRequestContent` carries `string` too, but it is a *new*
  type as of #632 — there is nothing to migrate there from v0.12.0. The public
  `basecamp.CreateScheduleEntryRequest` is untouched; see
  [the silent half](#schedulescreateentry-no-longer-validates-the-timestamps-664).
- **Two of the eight new `Todolist` members are required, and they are typed
  asymmetrically (#628, #637).** `Color` is `*string` with the json tag
  `"color"` and **no** `omitempty`, because the field is required but nullable —
  so marshalling a `Todolist` always emits the key, `"color": null` included.
  `CommentsAppUrl` is a plain `string`, not a pointer, because it is required
  and never null on the wire; it is the one field in this release that does not
  follow #560's blanket pointerization, and dereferencing it does not compile.
  Neither member existed at v0.12.0, so for a Go consumer this is additive —
  it only changes what `json.Marshal` emits, which the eight-new-fields note
  above already covers.
- **The `TodolistOrGroup` union and `generated.TodolistGroup` are gone (#628).**
  `resp.JSON200` is a `Todolist` directly. In `pkg/basecamp`, `TodolistGroup` is
  now a true alias: `type TodolistGroup = Todolist`. All 24 old fields survive
  with byte-identical types and json tags; the type gains 8 (`Description`,
  `DescriptionAttachments`, `GroupsURL`, `GroupPositionURL`, `BoostsCount`,
  `BoostsURL`, `Color`, `CommentsAppURL`). Two edges break: a type switch
  carrying both `case Todolist:` and `case TodolistGroup:` is now a duplicate
  case, and `json.Marshal` of a `TodolistGroup` emits up to 8 more keys.

## Behavioural

- **`Documents().Update` is read-modify-write; the wire operation is
  `ReplaceDocument` (#601).** Hooks see `{Documents, Get}` then
  `{Documents, Replace}`. `UpdateDocumentRequest` is unchanged. `pkg/generated`
  users lose the `UpdateDocument*` symbols outright. New failure mode:
  `Update`/`Edit` abort with a `CodeAPI` error if the GET returns a blank title.
- **`Schedules().UpdateEntry` is read-modify-write; the wire operation is
  `ReplaceScheduleEntry` (#632).** Hooks see `{Schedules, GetEntry}` then
  `{Schedules, ReplaceEntry}`. `ScheduleEntryFields` splits its state:
  `Summary`, `StartsAt`, `EndsAt`, `Description`, `AllDay` are resent every
  time; `URL`, `Highlighted`, `ParticipantIDs`, `Notify` are behind setters and
  only reach the wire if you call the setter, because bc3 preserves those on
  omission. Recurring entries surface as a decode error on the GET.
- **`Error` gained `FieldErrors` mid-struct (#541).** An unkeyed composite
  literal `basecamp.Error{code, msg, hint, status, retryable, reqID, cause}` no
  longer compiles. Validation `Message` text changed. `parseErrorBody` now
  decodes each member independently as `json.RawMessage`, so
  `{"error": {}, "error_description": "…"}` yields the hint where it previously
  yielded nothing.

---

# Swift

Swift carries ten breaks with no signal at all — second only to Go's twelve —
because three of its request types were replaced by same-named hand-written
ones, and two of its retry and error policies changed under unchanged
signatures.

One soft edge to know before you start: optional → non-optional breaks `if let`,
`guard let` and `?.` hard, but `x ?? default` only **warns**. A consumer whose
entire usage is `??` sees nothing.

## Silent

All ten are in [class A](#swift--10-class-a). Two deserve code here, and a
third — `todolists.update`'s leading-dot `.init` shape — is under
[Compile errors](#todolistsupdate-is-a-merge-safe-composite-574-628) because
every other shape of that call does fail to build.

### A cancelled request now throws raw (#568)

```swift
do { _ = try await account.projects.list() }
catch let error as BasecampError { report(error) }   // no longer matches
catch is CancellationError { … }
catch let error as URLError where error.code == .cancelled { … }
```

The classifier looks *through* `BasecampError.network(cause:)`, bounded to 8
links, so a custom `Transport` that wraps `URLError(.cancelled)` in `.network`
is treated as cancellation too — and that wrapped shape, which used to be
retried, is now terminal. The upside: cancellation no longer burns the retry
budget on a request the caller abandoned.

### A custom `Transport`'s own `.network` error is now retried (#592)

v0.12.0 failed any `BasecampError` out of the transport on sight. Main routes
`.network` to the retry branch: up to `maxAttempts` calls where you previously
got one, and the final error is your own rather than a re-wrapped
`BasecampError.network(message: "Network error", cause:)`. To keep an error
terminal, throw a non-network case. A single backoff sleep is now capped at 30 s
plus jitter.

## Compile errors

### `BasecampError.validation` gained a fifth associated value (#549)

```swift
case .validation(let message, let status, _, _, let fieldErrors):
    print("Validation (\(status)): \(message)")
    for (field, messages) in fieldErrors ?? [:] { … }
```

The case is now
`validation(message: String, httpStatus: Int, hint: String?, requestId: String?, fieldErrors: [String: [String]]?)`.
Add one more `_` to every match and every construction — or better, stop matching
the case and read the new `error.fieldErrors` property, defined on every
`BasecampError` (nil for all other shapes), which will not move again when a case
gains a sixth value.

### `TodolistGroup` and `TodolistOrGroup` are gone (#628)

```swift
let list = try await account.todolists.get(id: 987654)   // Todolist, not an envelope
if list.groupPositionUrl != nil { /* …and it is a group */ }
```

`Todolist` is a strict superset of `TodolistGroup`'s 24 members. The old
arm-unwrapping code was already dead: `TodolistOrGroup` was
`struct { var todolist: Todolist?; var group: TodolistGroup? }` with synthesized
`Codable`, so decoding bc3's flat body found neither key, left both nil via
`decodeIfPresent`, and reported success. Both arms were nil for every real
response.

### `Todolist.description` is now `let description: String` (#628)

Drop the optional handling; a description-less list reads back as `""`. In
`Todolist` literals, `description:` moves up into the required block (the emitter
sorts required members first) and loses its default. Add `"description"` to any
`URLProtocol` stub or cassette or decoding throws. One new member is optional and
purely additive: `groupPositionUrl`. The other two new members, `color` and
`commentsAppUrl`, are required — see the next section.

### `Todolist.color` and `.commentsAppUrl` are new *and* required (#637)

```swift
public let color: String?           // required, but nullable
public let commentsAppUrl: String   // required and non-optional
```

Neither member existed on `Todolist` at v0.12.0 — both arrived with #628 earlier
in this release, and #637 landed them in the required block rather than the
defaulted one. So this is not an optional member turning required; it is two new
members every `Todolist(...)` literal must pass from the start. The wire is
unchanged: bc3 has always emitted both keys, which is why they are required.

The decoder distinction matters for fixtures. `color` is required **and
nullable**, so the generator emits `try container.decode(String?.self, forKey:
.color)` — an explicit `"color": null` decodes fine, and only a **missing** key
throws. `commentsAppUrl` is required and non-nullable
(`decode(String.self, forKey:)`), so it rejects both null and absence. Fixtures
rendering `"color": null` need no change; fixtures omitting either key do.

### `UpdateTodolistOrGroupRequest.name` is required (#574)

Not a new wire restriction — bc3 presence-validates the attribute, so a body
omitting `name` was always a 422. If you do not have the current name to hand,
that is the signal to use the merge-safe path.

### `todolists.update` is a merge-safe composite (#574, #628)

```swift
let merged = try await account.todolists.update(id: 987654, req: UpdateTodolistRequest(name: "Launch Tasks"))
let replaced = try await account.todolists.replace(id: 987654,
    req: UpdateTodolistOrGroupRequest(description: current.description, name: "Launch Tasks"))
let edited = try await account.todolists.edit(id: 987654) { $0.name = "🚨 " + $0.name }
```

**One shape stays silent:**
`_ = try await account.todolists.update(id: 1, req: .init(name: "x"))` compiles
unchanged, because leading-dot inference resolves `.init` against whichever type
the parameter has and both expose `init(description:name:)`. That call silently
becomes two requests with preserve-on-omission semantics.

New pre-write failures on `update`/`edit`: a read-back that does not decode is
wrapped as `BasecampError.api`, and a read-back with an empty `name` throws
before any write.

### `ScheduleEntry.allDay`, `.endsAt`, `.startsAt` are non-optional (#632)

Remove the optional binding. All three move into the required init block and
lose their defaults. New optional members: `highlighted` and `joinUrl` — the join
link. **The request spells the same thing `url`.**

### `reports.upcoming` takes two required arguments and returns reduced types (#648)

```swift
// v0.12.0
let r = try await account.reports.upcoming(options: UpcomingReportOptions(windowStartsOn: a))
if let entries = r.scheduleEntries { … }

// v0.13.0
let r = try await account.reports.upcoming(windowStartsOn: a, windowEndsOn: b)
for entry in r.scheduleEntries { … }        // no longer optional
```

Four separate build failures, and they land in this order:

- **`UpcomingReportOptions` no longer exists** — it was a `public struct`
  declared inline in `ReportsService.swift`, not under `Models/`, so grep for the
  name rather than for a file. Both window bounds are now required positional
  labels; bc3 has always required them.
- **`Assignable` is deleted** (`Generated/Models/Assignable.swift` is gone), and
  its replacement `UpcomingAssignable` spells the field `content`, not `title`.
  `assignable.title` is *value of type 'UpcomingAssignable' has no member
  'title'* — not a silent nil. bc3 never sent `title`; the old model was fiction.
- **The three envelope arrays went `var … ?` to `let …`** —
  `scheduleEntries`, `recurringScheduleEntryOccurrences` and `assignables` are
  non-optional, so `if let` on any of them is *initializer for conditional
  binding must have Optional type*.
- **Five more new models** carry the reduced shapes:
  `UpcomingScheduleEntry`, `UpcomingScheduleBucket`, `UpcomingSchedulePerson`,
  `UpcomingAssignableParent`, `UpcomingAssignableCompletion`.

This is a fix in the strict sense: at v0.12.0 the call threw
`DecodingError.keyNotFound` on `bucket.type` for *any* non-empty window, so the
old signature could not return a populated result at all.

### Nine operations gained `bucketId:` (#619), three were removed (#619)

`campfires.{listChatbots, getChatbot, createChatbot, updateChatbot,
deleteChatbot}`, `clientApprovals.list`, `clientCorrespondences.list`,
`clientReplies.{get, list}` all take a leading `bucketId: Int`.
`clientApprovals.get(approvalId:)` and
`clientCorrespondences.get(correspondenceId:)` stay flat.

Removed: `recordings.get(recordingId:)` (no replacement — see
[Known gaps](#recordingsget-has-no-generic-replacement)),
`todos.trash(todoId:)` (→ `recordings.archive(recordingId:)` to preserve
behaviour, `recordings.trash(recordingId:)` to actually trash), and
`forwards.createReply` with `CreateForwardReplyRequest` (no supported
replacement — see [Known gaps](#there-is-no-raw-wire-migration-path);
`listReplies`/`getReply` are unaffected).

## Behavioural

- **`documents.update` and `schedules.updateEntry` are merge-safe composites** —
  see Silent. For schedule entries, the two-tier rule is: full-state fields
  (`summary`, `startsAt`, `endsAt`, `description`, `allDay`) are resent from the
  read-back when you pass nil, so nil means untouched — but what an explicit
  value does is per field, not a uniform clear. `description` is the only one
  `""` clears; `""` on `summary` is accepted and reads back `"Untitled"`;
  `startsAt`/`endsAt` cannot be cleared at all (bc3
  `validates_presence_of :starts_at, :ends_at`) and take a bare date or a
  timestamp to match `allDay`, which is a `Bool?` — `*bool` in Go, `boolean`
  in TypeScript — so `""` does not typecheck for it in any SDK. Carve-outs
  (`participantIds`, `url`, `highlighted`) are omitted entirely when nil so bc3
  preserves them, and `[]`, `""` and `false` clear them respectively; the
  fourth, `notify`, is a send directive rather than state — omitted when nil,
  nothing to clear. Recurring entries are out of reach on this route — bc3
  302-redirects both show and update for them.
- **`downloadURL`'s first hop now makes three attempts (#563)**, retrying network
  errors plus `{429, 502, 503, 504}` — never 500. Every attempt is
  authenticated; the signed second hop is still exempt. There is no public
  numeric knob: `DownloadURL` is deliberately absent from `behavior-model.json`.
  `enableRetry: false` collapses it to one attempt.
- **`cards.update` is a single PUT (#647).** The signature is byte-identical and
  `DueDate.preserve` still exists — it now omits the key instead of fetching the
  card and resending the value, so a call that used to make two requests and emit
  two hook events makes and emits one. `.clear` sends `"due_on": ""`. See
  [Cards: the due-date fix](#cards-the-due-date-fix).

---

# TypeScript

TypeScript catches most of this at build time. The exception is the error path —
at v0.12.0 every error from a generated service call carried the HTTP status text
rather than the server's message, and fixing that silently changed every message
string you might be matching on.

## Silent

See [class A](#typescript--9-class-a). Two details on `fieldErrors`
that bite:

```ts
catch (e) {
  // fieldErrors is a NULL-PROTOTYPE object
  e.fieldErrors.hasOwnProperty("color");   // TypeError: not a function
  Object.hasOwn(e.fieldErrors, "color");   // use this
  for (const [field, messages] of Object.entries(e.fieldErrors ?? {})) markInvalid(field, messages);
}
```

It is `undefined` for every status other than 400/422 even when the body carries
an `errors` key, and `BasecampError.toJSON()` gained a `fieldErrors` key — a diff
in serialised-error snapshot tests.

## Compile errors

### Nine operations gain a leading `bucketId` (#619)

```ts
await client.campfires.listChatbots(bucketId, campfireId);
await client.campfires.createChatbot(bucketId, campfireId, { serviceName: "deploybot" });
await client.campfires.getChatbot(bucketId, campfireId, chatbotId);
await client.campfires.updateChatbot(bucketId, campfireId, chatbotId, { … });
await client.campfires.deleteChatbot(bucketId, campfireId, chatbotId);
await client.clientApprovals.list(bucketId);
await client.clientCorrespondences.list(bucketId, { sort: "created_at" });
await client.clientReplies.list(bucketId, recordingId);
await client.clientReplies.get(bucketId, recordingId, replyId);
```

Watch `clientCorrespondences.list` — the old options object slides into the new
`bucketId` slot and reports TS2345, not TS2554. `campfires.list/get/listLines/
createLine` and the two flat client-side reads are deliberately untouched.

### `todos.trash()`, `recordings.get()` and `forwards.createReply()` removed (#619)

```ts
await client.recordings.archive(todoId);  // what todos.trash actually did
await client.recordings.trash(todoId);    // what its name promised
const recordings = await client.recordings.list("Todo", { bucket: [projectId] });
```

Drop the `CreateReplyForwardRequest` import — tsc suggests `CreateUploadRequest`,
which is unrelated noise. `forwards.createReply()` has no supported replacement;
see [Known gaps](#there-is-no-raw-wire-migration-path).

### One flat `Todolist` replaces `Todolist`, `TodolistGroup` and the `TodolistOrGroup` envelope (#628)

```ts
// before
const r = await client.todolists.get(id);
const list = "todolist" in r ? r.todolist : r.group;

// after
const list = await client.todolists.get(id);   // Todolist
const isGroup = list.group_position_url !== undefined;  // a list has groups_url instead
```

Only the *else* arm errors: `"todolist" in r` narrows `r` to
`Todolist & Record<"todolist", unknown>`, so `r.todolist` typechecks as `unknown`
and is silent.

The v0.12.0 envelope was fiction — bc3 has always returned the flat record, so
`"todolist" in r` was already false at runtime and `r.group` was already
`undefined`. Nobody's narrowing ever worked.

If you *build* group objects, the flat `Todolist` is a strict superset of the old
`TodolistGroup`, but it requires two members `TodolistGroup` did not have: add
`description: ""` and `description_attachments: []`.

### `Todolist.description` is now required (#628)

Construction sites only (TS2741). Readers are unaffected. Also new and optional:
`group_position_url`.

### `Todolist.color` and `.comments_app_url` are new *and* required (#637)

Construction sites only, again — TS2741 for each missing member, on top of the
`description` one above. Both members are new in this release (#628) and
required from the start (#637), so nothing you wrote against v0.12.0 referenced
them. Readers gain certainty rather than losing it: `comments_app_url` is
`string`, and `color` is `string | null`, so it is always **present** but may be
null. `null` is the
ordinary case for a group, so `list.color.toUpperCase()` still throws — narrow
with `list.color?.toUpperCase()` or an explicit null check. The wire is
unchanged; bc3 has always emitted both keys.

### `UpdateEntryScheduleRequest` → two types (#632)

`UpdateScheduleEntryRequest` (merge-safe; every field optional, plus `url?` and
`highlighted?`) and `ReplaceEntryScheduleRequest` (full replace; `startsAt` **and**
`endsAt` required, guarded by `Errors.validation("Starts at is required")` before
the request leaves). Most callers want the first — it is a superset of the old
field set, so the object literals need no change.

### `ScheduleEntry.starts_at`, `.ends_at`, `.all_day` are required (#632)

Construction sites only. New optional members: `join_url` (the video-call link —
read this, **not** `url`, which is the entry's own API URL) and `highlighted`.
Note `starts_at` is a bare date (`"2026-06-01"`) for an all-day entry and a full
timestamp otherwise; round-trip it verbatim.

### `reports.upcoming()` takes two required arguments and returns reduced types (#648)

```ts
// v0.12.0
const r = await client.reports.upcoming({ windowStartsOn: a, windowEndsOn: b });

// v0.13.0
const r = await client.reports.upcoming(a, b);
```

`upcoming()` with no arguments is TS2554 (*Expected 2 arguments, but got 0*);
passing the old object literal is TS2345, because the first parameter is now a
`string`. **`UpcomingReportOptions` is deleted** — it was exported from
`src/generated/services/reports.ts` but never re-exported from `src/index.ts`, so
only a deep import references it by name.

`components["schemas"]["Assignable"]` is gone, replaced by `UpcomingAssignable`
(and `UpcomingScheduleEntry`, `UpcomingScheduleBucket`, `UpcomingSchedulePerson`,
`UpcomingAssignableParent`, `UpcomingAssignableCompletion`). Two consequences:

- `assignable.title` no longer exists — the field is `content`. bc3 has always
  sent `content`, so this is the model catching up with the wire, not a loss.
  TypeScript reports it; **plain JS or a value you widened to `any` gets
  `undefined`, exactly as it already did at v0.12.0.**
- The three envelope arrays — `schedule_entries`,
  `recurring_schedule_entry_occurrences`, `assignables` — went from optional to
  required, so `r.schedule_entries?.map(…)` still compiles but the `?.` is now
  dead, and code that narrowed on their absence has an unreachable branch.

### The exported `paths` type lost nine route keys and re-typed two 422 bodies (#619, #586, #549)

Only affects code that types itself off the root `paths` export. The trap is the
five **removed operation slots** (`TrashTodo`, `GetRecording`,
`CreateForwardReply`, `UpdateDocument`, `UpdateScheduleEntry`): their path keys
survive because a sibling verb still lives there, so
`paths["/todos/{todoId}"]["delete"]` resolves to `never | undefined` and compiles
clean. Grep for those five names rather than trusting tsc. The 422 re-tags are
exactly `UpdateMyNote` and `UpdateMyPreferences`.

### Five `Identity` and `AuthorizedAccount` fields became optional (#681)

Five fields went from required `string` to optional:
`Identity.firstName`, `.lastName`, `.emailAddress`, `AuthorizedAccount.product`
and `AuthorizedAccount.appHref`. Under `strictNullChecks`, anything assigning one
to a `string` or calling a method on it is now a compile error:

```ts
// v0.12.0 — compiled
const email: string = info.identity.emailAddress;
const initial = info.identity.firstName.charAt(0);

// main — TS2322 / TS18048 ("possibly undefined")
const email: string = info.identity.emailAddress ?? "";
const initial = info.identity.firstName?.charAt(0) ?? "";
```

This is a **correction, not a restriction.** Those fields are Launchpad's. bc3
serves its own `GET /authorization.json` from a different template, and it emits
`identity.id` and nothing else of the identity, and no `product` or `app_href` on
accounts. You reach that document by passing `endpoint:` to
`authorization.getInfo()` — the documented way to point at a BC5 issuer — and at
v0.12.0 all five were typed `string` and arrived `undefined`. The type was
lying; it now describes both issuers.

`appHref` is the one to grep for rather than reason about. `discoverIdentity()`
coerced a missing `app_href` to `""` while `AuthorizationService` typed it
required — so at v0.12.0 the same field was already reaching consumers as an
empty string from one call site and as `undefined` from the other, and only the
latter was a type error waiting to happen. It is now honestly optional on both.

`AuthorizedAccount.resource` (the RFC 8707 indicator, BC5 only) and a top-level
`AuthorizationInfo.scope` are new and optional, so they break nothing.

The same change lands on `discoverIdentity()`, which returns the same
`AuthorizationInfo`. It had already drifted from `AuthorizationService`'s copy of
this mapping — `app_href` was optional in one and required in the other — and
both now share one parser in `oauth/authorization-document.ts`.

Two behavioural changes ride along, neither of which the compiler will flag:

- **`expires_at` is read as epoch seconds when it arrives as a number.** bc3
  renders `@token.expires_at.to_i`. v0.12.0 passed that integer straight to
  `new Date()`, which reads it as *milliseconds* — so a token expiring in 2036
  parsed as a date in **1970**, and every "is my token still valid" check said
  no. A string is still parsed as ISO-8601, unchanged.
- **`filterProduct` no longer empties the list when the filter cannot apply.**
  A BC5 document carries no `product` on any account, so
  `getInfo({ filterProduct: "bc3" })` matched nothing and returned `[]` — while
  the accounts it was meant to filter existed and were what you needed the
  `href` from. When the document carries at least one account and *none* of them
  carries a `product`, the filter is now reported inapplicable: every account is
  returned and the new `AuthorizationInfo.productFilterApplied` is `false`. When
  at least one account does carry a `product` the filter applies exactly as
  before, so an empty result still means "nothing matched". An **empty** account
  list — what Launchpad returns for an identity with no currently accessible
  accounts — reports `applied: true`: the list is empty either way, and an empty
  list is no evidence that the issuer omits `product`.

Go gets the same `filterProduct` correction and the same two additive fields
(`AuthorizedAccount.Resource`, `AuthorizationInfo.Scope`, plus
`ProductFilterApplied`); its timestamp already decoded both spellings via
`FlexTime`. Nothing there is a compile break — Go's fields were already
zero-valued rather than required. Ruby and Python return the raw document
unchanged. See `spec/api-gaps/bc5-authorization-document-shape.md`.

## Behavioural

- **`todolists.update()` is merge-safe; the one-shot replace is
  `todolists.replace()` (#574).** Nothing to change to compile. Four decisions:
  two requests per call; pass `description: ""` if you relied on
  omission-clears (`replace()` requires `name`); `update`/`edit` now throw a
  statusless `BasecampError` with code `api_error` on a malformed read-back; and
  `update(id, { name: "" })` — which v0.12.0 happily PUT and let bc3 422 — now
  throws `Errors.usage("todolist name must not be empty")` locally.
- **`documents.update()` is merge-safe; `documents.replace()` is the one-shot
  (#601).** `UpdateDocumentRequest` keeps its exact shape.
- **`schedules.updateEntry()` is merge-safe with a four-field carve-out (#632).**
  Full state, always resent: `summary`, `startsAt`, `endsAt`, `description`,
  `allDay`. Carve-outs, sent only when the key is present on your object:
  `participantIds`, `url`, `highlighted`, `notify`. **Addressedness is property
  presence, not truthiness** — the runtime test is
  `Object.prototype.hasOwnProperty.call`, so a `{...maybeUndefined}` spread
  carrying an explicit `undefined` counts as addressed. `editEntry` tracks
  carve-outs by *setter invocation* via a Proxy, so `e.url = e.url` does send the
  join link.
- **`todos.update()` and `todos.edit()` now throw on a malformed read-back
  (#597).** v0.12.0's `?? ""` coalesced only null/undefined; a wrong-typed field
  rode into the replacement PUT. The new error is statusless,
  `code === "api_error"`, `retryable === false`, and never reaches the wire.
  `cards.update()` was in this set and is no longer — see the next bullet.
- **`cards.update()` is a single PUT (#647).** `UpdateCardRequest` is unchanged
  and nothing in your build moves. A call leaving `dueOn` unaddressed used to
  fetch the card first; it no longer does, so it costs one request and one hook
  event instead of two, and `dueOn: null` goes on the wire as `""`. This also
  retires the #597 guard on this method — `writableString` is not invoked for
  Cards any more, because there is no read-back to validate. See
  [Cards: the due-date fix](#cards-the-due-date-fix).

---

# Python

Python has no compile step: every one of these lands at runtime, in production,
on the first call that takes that branch. The package ships `py.typed`, so
mypy/pyright catch the signature changes if you type-check — untyped code finds
out on the call.

## Silent

See [class A](#python--8-class-a), which now includes the three merge-safe
composites and the cards collapse — all four have byte-identical keyword sets
and no signal of any kind.

One correction worth knowing if you saw an earlier draft: at v0.12.0, a body like
`{"error": {"code": 7}}` did not "hand you the dict" — it raised
`AttributeError: 'dict' object has no attribute 'encode'` inside
`error_from_response` on 400/422/404 and the generic else. Only 401/403 returned
the dict. On main all of those return the clean default. That half is a crash
fix.

## Runtime errors

### Nine operations now require `bucket_id` (#619)

```python
bots      = account.campfires.list_chatbots(bucket_id=project_id, campfire_id=campfire_id)
approvals = account.client_approvals.list(bucket_id=project_id, sort="created_at")
reply     = account.client_replies.get(bucket_id=project_id, recording_id=rec_id, reply_id=reply_id)
```

`TypeError: missing a required argument: 'bucket_id'` on all nine and their async
twins. `client_approvals.get(approval_id=…)` and
`client_correspondences.get(correspondence_id=…)` are unchanged — do not add
`bucket_id` there.

### `todos.trash()` removed — and it archived (#619)

```python
account.recordings.archive(recording_id=todo_id)  # preserves what you had
account.recordings.trash(recording_id=todo_id)    # a real change to your data
```

Decide per call site; do not sed one into the other.

### `recordings.get()` removed (#619)

No generic recording read remains. Use `account.todos.get(todo_id=…)` /
`messages.get` / `documents.get`, or enumerate with
`account.recordings.list(type="Todo", …)`. See
[Known gaps](#recordingsget-has-no-generic-replacement).

### `forwards.create_reply()` removed, with `CreateForwardReplyRequestContent` (#619)

No supported replacement — see
[Known gaps](#there-is-no-raw-wire-migration-path). Reads are unaffected:
`forwards.list_replies` and `forwards.get_reply` remain.

### `TodolistGroup` is gone (#628)

```python
from basecamp.generated.types import Todolist

def render(item: Todolist) -> str:
    if "group_position_url" in item:   # a group; a plain list carries groups_url
        return group_label(item)
    return item["description"]         # now required and never null
```

Discriminate structurally, never on `type` — it reads `"Todolist"` for a group
and a list alike. `description` moved from `NotRequired[str]` to required `str`;
the `.get(..., "")` guard is dead. `description_attachments` did **not** change —
it was already required.

Two more members are required, but unlike `description` they are also new:
`color` (`str | None`) and `comments_app_url` (`str`) did not exist on the
v0.12.0 `Todolist`, arriving with #628 and landed as required by #637. Note the
asymmetry — `color` is always **present** but may be `None`, so `item["color"]`
is safe while `item["color"].upper()` is not.

Everything else about this is silent in Python: `basecamp/generated/types.py` is
imported by nothing in the SDK and every generated service method is annotated
`-> dict[str, Any]`. There is no decoder to throw.

### `reports.upcoming()` requires both window bounds (#648)

```python
# v0.12.0
def upcoming(self, *, window_starts_on: str | None = None, window_ends_on: str | None = None) -> dict[str, Any]
# v0.13.0
def upcoming(self, *, window_starts_on: str, window_ends_on: str) -> dict[str, Any]
```

Both are now required keyword-only arguments, on the sync method and its async
twin. A call that omits either raises
`TypeError: upcoming() missing 1 required keyword-only argument: 'window_ends_on'`
at call time — where v0.12.0 reached bc3 and came back a 400. bc3 has always
required both.

The return type is unchanged (`dict[str, Any]`; Python never decodes into the
TypedDicts at runtime), so **nothing about reading the result changes at
runtime.** What changes is the static picture: `Assignable` is deleted from
`basecamp.generated.types` and replaced by `UpcomingAssignable` and five
siblings, and the envelope's three members went `NotRequired[list[...]]` to
required. `from basecamp.generated.types import Assignable` is an `ImportError`;
everything else here is mypy/pyright-only. In particular
`result["assignables"][0]["title"]` was already a `KeyError` at v0.12.0 — bc3
never sent `title` — so #648 changes nothing about it.

### With `max_retries` 0 or 1, a 401 no longer refreshes the token on reads (#571)

```python
# v0.12.0: the refresh replay lived inside _single_request and was UNCOUNTED
client = Client(token_provider=oauth_provider, config=Config(max_retries=1))
account.projects.get(project_id=1)   # 401 → refresh → replay → 200

# main: the replay is a request on the attempt budget, checked BEFORE refresh()
client = Client(token_provider=oauth_provider, config=Config(max_retries=2))
account.projects.get(project_id=1)   # 401 → refresh → replay → 200
```

Measured on main: `max_retries=0` → `AuthError`, refreshes=0, one request.
`max_retries=1` → same. `2` and `3` → OK, refreshes=1. At v0.12.0 all four
succeeded.

If you set `max_retries` (or `BASECAMP_MAX_RETRIES`) to 0 or 1 **and** use a
refreshable token provider, raise it to at least 2. Reads only: mutations bypass
the retry loop and keep the uncounted replay.

## Behavioural

- **`todolists.update()` is a merge-safe GET+PUT (#574).** Signature identical.
  Hooks now observe `GetTodolistOrGroup` then `UpdateTodolistOrGroup`. Four
  things move: two round-trips; `ApiError` when the GET body is not a dict or
  `name`/`description` is absent/null/non-string/(for name) empty; `UsageError`
  (not `ApiError`) when *you* pass a non-string, because `_caller_string` owns
  caller provenance and `_writable_string` owns response provenance; and
  clear-by-omission no longer works — pass `description=""`.
  `replace(id=…, name=…)` is the destructive PUT and `name` is now mandatory.
- **`documents.update()` is a merge-safe GET+PUT (#601).** `replace()` keeps both
  params optional here, so omitting `title` is not a 422 but a 200 that leaves
  the document reading back as "Untitled".
- **`schedules.update_entry()` is merge-safe (#632).** Every v0.12.0 keyword still
  binds. Two new keywords: `url=` (the join link) and `highlighted=`. Those two
  plus `participant_ids` and `notify` are addressed-only carve-outs: they reach
  the wire only if you pass them, and `""`/`[]`/`False` is an explicit clear.
  **Asymmetry:** you write the join link as `url=` but read it back as
  `join_url` — the response's own `url` is the entry's API URL, and echoing it
  into `url=` overwrites the join link. `replace_entry` now requires `starts_at`
  and `ends_at`.
- **`page` selects a page (#617).** Proven with a mock transport that always
  returns a next link: v0.12.0 `get_everything_open_todos(page=3)` issued 10,000
  requests (the `max_pages` cap) starting at `?page=3`; main issues exactly one
  and reports `meta.truncated=True`. Scope is 17 sync methods and their async
  twins. `page=True` is explicitly rejected as a selector (bool subclasses int).
- **`cards.update()` is a single PUT (#647).** Keyword set identical. A call
  leaving `due_on` unaddressed used to run `self.get(card_id=card_id)` first and
  no longer does — one request and one hook event instead of two — and
  `due_on=""` is now the clear encoding. The #597 `writable_string` guard is no
  longer reached on this method, because there is no read-back to validate. See
  [Cards: the due-date fix](#cards-the-due-date-fix).

---

# Ruby

Ruby's breaks are all runtime, and one of them changes when your list methods hit
the network — `list` now performs a request at call time instead of on first
iteration, so a `rescue` placed around only the loop stops catching.

## Silent

### `page:` returns exactly that page (#617)

```ruby
# v0.12.0, against a 4-page stub
drafts = account.drafts.list_my_drafts(page: 2).to_a
#   requests: ?page=2, ?page=3, ?page=4      items: [{id=>2},{id=>3},{id=>4}]

# main, same stub
drafts = account.drafts.list_my_drafts(page: 2).to_a
#   requests: ?page=2                        items: [{id=>2}]
```

There is no single call for "everything from page 2 onward" any more. Drop
`page:` and slice, or drive the loop yourself. Note that six list methods gained
only `max_items:` and no `page:` at all: `campfires.list_chatbots`,
`checkins.answerers`, `message_types.list`, `people.list_pingable`,
`uploads.list_versions`, `webhooks.list`.

### `max_retries: 0` now sends one request where it sent none (#656)

```ruby
# v0.12.0 — max_attempts was @config.max_retries, i.e. 0, so the loop broke
# before single_request was ever called
client = Basecamp::Client.new(..., max_retries: 0)
client.get_absolute(url)
#   requests: (none)   raises Basecamp::ApiError, "Request failed after 0 attempts"

# main — the cap is floored at one attempt on every path
client.get_absolute(url)
#   requests: 1
```

**Narrow, and worth reading the scope before auditing anything.** Two conditions
have to hold together:

- `max_retries` must be **0**. For any value ≥ 1 the new expression
  `[@config.max_retries, 1].max` is byte-identical to the old one.
- The GET must be **ungoverned** — carrying no operation ID. Every one of the
  249 operations in `metadata.json` declares a retry block, so this is not "an
  operation without a policy"; it is a call site that passes no `operation:`.
  In practice that means `Http#get_absolute` and the Launchpad authorization
  fetch it backs, plus `AccountClient#get` / `Http#get` when *you* call them
  without naming an operation.

Mutations were never affected — they never entered the retry loop — and the
download hop was already floored. If you had `max_retries: 0` standing in for a
kill switch on those escape hatches, it is not one any more.

### `ValidationError#message` changed; `#field_errors` is new (#541, #549)

`e.message == "Request failed"` stops matching. Ruby is unaffected by the
cross-SDK "400 now maps to validation" note — `Basecamp.error_from_response(400, …)`
returned `ValidationError` on both trees. A latent bug also disappears: a 422
whose body was a bare JSON array used to raise
`TypeError: no implicit conversion of String into Integer` out of
`error_from_response`; it now yields a clean `ValidationError`. A
`rescue TypeError` around error handling is dead code.

### `Draft#scheduled_posting_at` and `MyNote#created_at`/`#updated_at` decode to `Time` (#560)

```ruby
draft.scheduled_posting_at.start_with?("2026")  # NoMethodError
Time.parse(draft.scheduled_posting_at)          # TypeError
"#{draft.scheduled_posting_at}"                 # "2026-01-02 03:04:05 UTC", was "2026-01-02T03:04:05Z"
draft.scheduled_posting_at.iso8601              # the old string
```

These are the only three decoder changes in the window. `parse_datetime` silently
rescues to nil, so there is no decode failure to catch. Blast radius is narrow —
no service method returns a `Basecamp::Types::` object.

## Runtime errors

### `bucket_id:` on nine operations (#619)

```ruby
account.campfires.list_chatbots(bucket_id: 2085958499, campfire_id: 1069479351)
account.campfires.create_chatbot(bucket_id: …, campfire_id: …, service_name: "deploybot", command_url: …)
account.campfires.get_chatbot(bucket_id: …, campfire_id: …, chatbot_id: …)
account.campfires.update_chatbot(bucket_id: …, campfire_id: …, chatbot_id: …, service_name: …)
account.campfires.delete_chatbot(bucket_id: …, campfire_id: …, chatbot_id: …)
account.client_approvals.list(bucket_id: …, sort: "created_at")
account.client_correspondences.list(bucket_id: …)
account.client_replies.list(bucket_id: …, recording_id: …)
account.client_replies.get(bucket_id: …, recording_id: …, reply_id: …)
```

`ArgumentError: missing keyword: :bucket_id` the first time each line executes —
so a call site behind a rarely-taken branch stays broken until it runs.
`client_approvals.get(approval_id:)` and
`client_correspondences.get(correspondence_id:)` stay flat.

### Three removals (#619)

`account.todos.trash` → `account.recordings.archive(recording_id:)` to preserve
behaviour, or `recordings.trash(recording_id:)` to actually trash.
`account.recordings.get` → the type-specific getter or `recordings.list(type:)`.
`account.forwards.create_reply` → nothing; there is no supported replacement
(see [Known gaps](#there-is-no-raw-wire-migration-path)) and `list_replies` is
unaffected.

### `Basecamp::Types::TodolistGroup` no longer exists (#628)

`NameError` at first reference. Use `Basecamp::Types::Todolist`;
`Todolist.required_fields` gained `:description`, plus `:color` and
`:comments_app_url` — the latter two being members #628 added and #637 landed as
required, neither present at v0.12.0. `#to_h` changed with them: it used to end
in `.compact`, dropping every nil member, and now keeps `color` even when nil,
so a serialized todolist carries `"color" => nil` rather than omitting the key. Two parallel changes bite
identical code: `ScheduleEntry.required_fields` gained `:all_day`, `:ends_at`,
`:starts_at`, and `ScheduleEntry` gained `#highlighted` and `#join_url`. Blast
radius is narrow — **no SDK service method returns a `Types::` object**; the
service layer returns raw Hashes. This only bites code that constructs these
value objects itself.

### `reports.upcoming` requires both window bounds (#648)

```ruby
# v0.12.0
def upcoming(window_starts_on: nil, window_ends_on: nil)
# v0.13.0
def upcoming(window_starts_on:, window_ends_on:)
```

`account.reports.upcoming` with either omitted raises
`ArgumentError: missing keywords: :window_starts_on, :window_ends_on` at call
time. bc3 has always required both, so v0.12.0's defaults produced a 400.

The return value is unchanged — a raw `Hash` — so **reading the result behaves
exactly as it did.** `result["assignables"][0]["title"]` was `nil` at v0.12.0
because bc3 sends `content`, and it is `nil` now for the same reason.
`Basecamp::Types::Assignable` is deleted (`NameError` at first reference) in
favour of `Basecamp::Types::UpcomingAssignable` and five siblings, but as with
the other `Types::` classes, no service method returns one — this only bites code
that constructs them itself.

### A refreshable 401 no longer replays when `max_retries` is 0 or 1 (#571)

```ruby
config = Basecamp::Config.new(max_retries: 2)   # was 1
client = Basecamp::Client.new(config: config, token_provider: oauth_provider)
```

Measured against a stub that 401s once then 200s: v0.12.0 succeeded at
`max_retries` 0, 1, 2 and 3 alike (two requests, one refresh). Main raises
`Basecamp::AuthError` with zero refreshes at 0 and 1, and succeeds from 2 up.
Only bites a refreshing provider (`OauthTokenProvider` or your own
`refreshable?`-true provider); `StaticTokenProvider` is unaffected and the
default is 3. Reads only — mutations bypass the retry loop.

## Behavioural

### List methods now request eagerly and return `ListEnumerator` (#557)

```ruby
# v0.12.0: fully lazy — 0 requests, 0 hooks until you iterate
enum = account.projects.list
begin
  enum.each { |p| … }          # auth/404 errors surfaced HERE
rescue Basecamp::NotFoundError => e
end

# main: page 1 is fetched inside the call
begin
  enum = account.projects.list  # 1 request, hooks=[start, page:1]; errors surface HERE
  enum.meta.total_count         # X-Total-Count, available already
  enum.each { |p| … }
rescue Basecamp::NotFoundError => e
end
```

Move the `begin` above the list call. Stop building enumerators you may not
iterate — construction now costs a request and emits an `on_operation_start` that
never gets a matching `on_operation_end` if you never iterate.
`ListEnumerator < Enumerator`, so `each`/`next`/`take`/`first`/`lazy`/`to_a` are
unchanged; only `.class` assertions move. The total request count for a full
traversal is **unchanged** (page 1 is fetched once either way). Hook-driven
metrics need the most care: a re-enumerated list used to emit one start/end pair
per pass and now emits one pair total.

### Three merge-safe composites

- **`todolists.update` is GET+PUT (#574).** Wire trace on main:
  `GET /1/todolists/3` then
  `PUT /1/todolists/3 {"name":"New name","description":"<div>keep me</div>"}`.
  `todolists.replace` is the old destructive PUT and `name:` is now required.
  `todolists.edit(id:) { |list| … }` is the read-modify-write form. New:
  `ApiError` when the GET body is not a Hash or `name`/`description` is
  missing/null/non-String/(for name) empty. Doc correction: `description` is
  writable for a todolist **group** too, contrary to v0.12.0's doc.
- **`documents.update` is GET+PUT (#601).** `documents.replace` carries the old
  signature exactly. `title` goes through `required_writable_string` (absent,
  null, non-String **or whitespace-only** is refused); `content` through
  `writable_string`.
- **`schedules.update_entry` is GET+PUT (#632).** Full-state fields resent:
  `summary`, `starts_at`, `ends_at`, `description`, `all_day`. Carve-outs omitted
  unless addressed: `participant_ids`, `url`, `highlighted`, `notify`. **Read the
  join link back as `join_url`, never as `url`** — `fields_from_entry` seeds its
  `url` member from `join_url` because the response's own `url` is the entry's
  API URL. Omitting `all_day:` no longer resets it to false. `replace_entry` now
  requires `starts_at:` and `ends_at:`.

### `todos.update`/`edit` refuse a malformed read-back (#597)

v0.12.0's `todo["content"] || ""` turned a `false` content into `""` and wrote it
back — erasing the field on a call that never mentioned it. It now raises
`Basecamp::ApiError` (statusless, `retryable: false`) before the PUT. Should
never fire against a healthy server; it fires instead of silently corrupting the
record when one misbehaves. `cards.update` was in this set at the time #597
landed and is no longer — see the next entry.

### `cards.update` is a single PUT and no longer reads first (#647)

Keyword set identical, so every call site binds unchanged. A call that left
`due_on` unaddressed used to fetch the card first; it no longer does, so it costs
one request and one hook event instead of two, and `due_on: ""` is now the clear
encoding — `compact_params` is `kwargs.compact`, so the empty string survives to
the wire where a `nil` would not. `MergeSafe.writable_string` is not reached on
this method any more, because there is no read-back to validate. See
[Cards: the due-date fix](#cards-the-due-date-fix).

### Two URLs changed (#586)

Loud in a stubbed suite (WebMock raises on an unregistered request), silent
against live bc3.

### `download_url`'s first hop retries and no longer sends `Accept` (#563)

```ruby
# v0.12.0 — one request, Accept: application/json, raised on a 503
response = http.get_no_retry(rewritten_url)

# main — up to three attempts, no Accept header at all
def get_download(url)
  request_with_retry(:get, url, retry_on: DOWNLOAD_RETRY_ON, accept: nil)
end
```

`DOWNLOAD_RETRY_ON` is `{429, 502, 503, 504}` plus network errors — never 500.
Two consequences, both silent against a live server:

- **Cassettes and stubs that match on `Accept` stop matching**, because the
  header is no longer sent at all. `accept: nil` is load-bearing:
  `request_headers` only sets `Accept` when it is non-nil.
- **Single-shot download stubs see more requests than they expect.** A stub that
  returns one 503 used to surface as an error and now gets retried.

The signed second hop is unaffected. This is the same change Python got, and it
is the one download-hop entry that does **not** apply to Go — Go's download path
already retried at v0.12.0.

---

# Kotlin

Kotlin catches the type work at compile time. The things to watch are decoder
throws on payloads your fixtures may not carry — including one that no signature
change announces at all — and one silent semantic change hidden behind a
hand-written compatibility shim.

## Silent

### `documents.update` stopped erasing omitted fields (#601)

`UpdateDocumentBody` was removed from the generator's `Types.kt` and re-declared
by hand in the **same package** (`com.basecamp.sdk.generated.services`, file
`generated-compat/UpdateDocumentBody.kt`) with the identical two nullable fields,
and `documents.update(id, body): Document` kept its exact signature. Your call
site compiles untouched and now issues `GetDocument` then `ReplaceDocument`.
`UpdateDocument` is gone from `generated/Metadata.kt`. Test doubles that stub
only the PUT now fail on the unstubbed GET.

```kotlin
import com.basecamp.sdk.generated.services.ReplaceDocumentBody
account.documents.replace(documentId, ReplaceDocumentBody(title = "Q3 plan"))  // old behaviour
account.documents.edit(documentId) { title = "🚨 $title" }
```

New failure mode: `update`/`edit` throw `BasecampException.Api` if the fetched
document has a blank title.

### `cards.update` stopped fetching the card first (#647)

The signature is unchanged. A call leaving `dueOn` null used to run
`get(cardId).dueOn` first and no longer does — one request and one hook event
instead of two — and `dueOn = ""` is now the clear encoding, since the generated
body builder drops only nulls. See
[Cards: the due-date fix](#cards-the-due-date-fix).

### Two URLs changed (#586), error composition changed (#541, #549)

See [class A](#kotlin--6-class-a-1-class-b). Two source-compatible widenings
ship alongside the error work: `BasecampException.Api(httpStatus: Int)` became
`Int? = null`, and `Validation` gained a trailing `fieldErrors` parameter with a
default — every existing construction still binds.

## Runtime errors

### `Todolist.description` became required and non-null (#628)

```
kotlinx.serialization.MissingFieldException: Fields [description, description_attachments]
are required for type with serial name 'com.basecamp.sdk.generated.models.Todolist',
but they were missing at path: $
```

That is the real message, captured by decoding v0.12.0's own group fixture
against main's model. Note it is **two** fields, not one:
`description_attachments` was already required on `Todolist` at v0.12.0, and the
old `TodolistGroup` had neither member — so a captured *group* payload is missing
both.

Re-record group payloads with `"description"` (a string, `""` for empty) and
`"description_attachments"` (an array, `[]` for none). Plain *todolist* fixtures
usually need nothing — v0.12.0's own `spec/fixtures/todolists/get.json` already
carried `"description": ""`.

**Two further members are required**, `color` and `commentsAppUrl`, both
declared without a default (#628 added them, #637 made them required before the
release shipped — neither existed at v0.12.0). They differ in nullability, and
the difference is exactly what your fixtures hit: `color` is `String?`, so an explicit
`"color": null` is accepted and only a *missing* key raises
`MissingFieldException`; `commentsAppUrl` is `String`, so it rejects null and
absence alike. A fixture already rendering `"color": null` is fine as-is.

A missing key gives `MissingFieldException`; an explicit `"description": null`
gives `JsonDecodingException`, because `description` is required and **not**
nullable. The client's `coerceInputValues = true` rescues neither. The throw
escapes as a raw `SerializationException` from
`todolists.get/replace/list/create` and `todolistGroups.list/create` —
`catch (e: BasecampException)` does not see it. The merge-safe
`todolists.update`/`edit` are the exception; they wrap it into
`BasecampException.Api`.

### A wrong-typed scalar no longer decodes into a `String` (#660)

```kotlin
// v0.12.0 — isLenient = true on the client-wide Json
// {"description": 42}     -> description == "42"
// {"description": false}  -> description == "false"

// v0.13.0 — isLenient is gone
// kotlinx.serialization.SerializationException
```

**Nothing in your build tells you.** No type, field or method signature moved;
the only change is one line of `Json { }` configuration in `BasecampClient`. That
`Json` backs every response decode in the SDK, so the scope is every
`String`/`String?` member on every model, not a named list of fields. `isLenient`
also relaxed other RFC-4627 rules — unquoted literals — so a body relying on that
laxity now fails too. Structural mismatches (`[]` or `{}` where a string is
declared) were already refused and are unchanged, as is `coerceInputValues`,
which stays: it rewrites an explicit null to a declared default and has nothing
to say about a scalar's type.

Two things to plan around:

- **The trigger is a present, populated, wrong-typed field.** Absence and
  explicit null behave exactly as they did, so a fixture that omits the field
  proves nothing. Re-record any cassette carrying a number or boolean in a string
  slot.
- **On a write, the mutation has already happened.** The throw is in the
  *response* decode, so `cards.update` issues its PUT, the card changes, and
  *then* it raises. And it raises a raw `SerializationException`, not a
  `BasecampException` — `catch (e: BasecampException)` does not cover it. The
  merge-safe `todolists.update`/`edit` are the exception; they wrap it as
  `BasecampException.Api`.

This is the fix for a real corruption path: under `isLenient`, a merge-safe
composite read `"description": 42`, got the string `"42"`, and wrote that
fabricated value back to a full-replace endpoint on a call that never mentioned
the field. The per-composite guard that fixed Python, Ruby and TypeScript in #597
could not see it, because the coercion happened inside the decoder.

### `ScheduleEntry.allDay`, `.startsAt`, `.endsAt` became required and moved (#632)

Same decoder throw for payloads that omitted them. The SDK's own
`spec/fixtures/schedules/entry_get.json` already carried all three, so this bites
hand-built JSON and cassettes recorded from reduced renderings, not the shipped
recordings.

The three fields also **moved from the tail of the constructor to positions
13–15**, so positional construction of a `ScheduleEntry` (test fakes) and
`componentN` destructuring past position 12 break at compile time. Switch to
named arguments. Treat `startsAt`/`endsAt` as opaque strings — a bare date for an
all-day entry, a full timestamp otherwise.

## Compile errors

### `TodolistGroup` is gone (#628)

```kotlin
import com.basecamp.sdk.generated.models.Todolist

val groups: ListResult<Todolist> = account.todolistGroups.list(todolistId)
val created: Todolist = account.todolistGroups.create(todolistId, CreateTodolistGroupBody(name = "Phase 1"))
```

The accessor, the method names and `CreateTodolistGroupBody` are unchanged.

### `todolists.get` returns `Todolist`, not `JsonElement` (#628)

Delete the hand-rolled `jsonObject[...]!!.jsonPrimitive.content` digging. There
is no `JsonElement` escape hatch on this method any more.

### `todolists.update` takes a different body; `replace` is the destructive PUT (#574, #628)

```kotlin
import com.basecamp.sdk.generated.services.UpdateTodolistBody

account.todolists.update(todolistId, UpdateTodolistBody(name = "Hardware"))     // merge-safe
account.todolists.edit(todolistId) { name = "🚨 $name" }
account.todolists.replace(todolistId, UpdateTodolistOrGroupBody(name = "Hardware", description = ""))
```

Same field names, both nullable, but null now means "leave alone".
`UpdateTodolistOrGroupBody.name` also became non-null and required, so
`UpdateTodolistOrGroupBody(description = "x")` no longer compiles. Hooks see
`GetTodolistOrGroup` then `UpdateTodolistOrGroup` — the wire operation name did
not change, so a hook keyed on those keeps firing but fires twice as often, and
an allowlist that omits `GetTodolistOrGroup` now denies the read half.

### `UpdateScheduleEntryBody` no longer exists (#632)

```kotlin
account.schedules.updateEntry(entryId, summary = "Weekly sync")
account.schedules.editEntry(entryId) { summary = "Weekly sync" }
account.schedules.replaceEntry(entryId, ReplaceScheduleEntryBody(
    summary = "Weekly sync",
    startsAt = "2026-08-10T09:00:00.000Z",   // now mandatory
    endsAt   = "2026-08-10T10:00:00.000Z",
))
```

Flatten the old body's fields into named arguments; declared order is `summary,
startsAt, endsAt, description, allDay, participantIds, url, highlighted, notify`.
Every argument is nullable and null means "do not address", so a call that used
to erase participants by omission now preserves them — clear deliberately with
`participantIds = emptyList()`, `url = ""`, `highlighted = false`.

### Nine operations gained a leading `bucketId` (#619)

`campfires.{listChatbots, createChatbot, getChatbot, updateChatbot,
deleteChatbot}`, `clientApprovals.list`, `clientCorrespondences.list`,
`clientReplies.{list, get}`. The new parameter is a `Long` in first position, so
a call passing the same number of arguments can bind wrongly and fail on a
*later* parameter — check each site rather than trusting the error text.
`listChatbots` did not gain a second overload, so the ambiguity break below does
not apply to it.

### Three operations removed (#619)

`recordings.get` (no replacement — see
[Known gaps](#recordingsget-has-no-generic-replacement)), `todos.trash`
(→ `recordings.trash(todoId)`, which is what the name promised;
`recordings.archive` is what the old call actually did), `forwards.createReply`
(no supported substitute — see
[Known gaps](#there-is-no-raw-wire-migration-path); `CreateForwardReplyBody` is
gone from `Types.kt` with no compat shim).

### `reports.upcoming` takes two required arguments and returns a typed result (#648)

```kotlin
// v0.12.0
suspend fun upcoming(options: GetUpcomingScheduleOptions? = null): JsonElement
// v0.13.0
suspend fun upcoming(windowStartsOn: String, windowEndsOn: String): UpcomingScheduleResult
```

Three build failures. `upcoming()` no longer has a default argument, so a bare
call is unresolved; `GetUpcomingScheduleOptions` is deleted from
`generated/services/Types.kt` (and from `options-param-order.json`); and the
return type is no longer `JsonElement`, so every `.jsonObject["schedule_entries"]`
navigation stops compiling. `assignable.title` is an unresolved reference — the
member is `content`, which is what bc3 has always sent.

`UpcomingScheduleResult` and its six models are `@Serializable` with
non-nullable required members, so a body missing any of them now throws
`MissingFieldException` where v0.12.0 handed back a `JsonElement` unconditionally.
That is deliberate, and it is the same fixture hazard as the two entries above.

### Untyped callable references to 22 list methods are now ambiguous (#617)

```kotlin
val fetch = account.documents::list   // Overload resolution ambiguity

val fetch: suspend (Long, PaginationOptions?) -> ListResult<Document> = account.documents::list
```

Each method now has two overloads: a primary taking the operation's own
`List…Options` (which carries `page`, non-nullable, no default) and a
source-compatibility overload keeping `PaginationOptions? = null`. Ordinary calls
— `list(id)`, `list(id, null)`, `list(id, PaginationOptions(maxItems = 50))` —
resolve unchanged. The 22: `boosts.listForRecording`, `boosts.listForEvent`,
`campfires.list`, `cards.list`,
`checkins.{reminders, listQuestions, listAnswers, byPerson}`,
`clientReplies.list`, `comments.list`, `documents.list`, `events.list`,
`forwards.listReplies`, `gauges.listGaugeNeedles`, `people.{list, listForProject}`,
`reports.{progress, personProgress}`, `timeline.projectTimeline`,
`todolistGroups.list`, `uploads.list`, `vaults.list`.

## Behavioural

- **`page` selects a page (#617)** — 17 operations, no build-time signal.
- **`downloadURL` retries hop 1 (#563)** — `DOWNLOAD_RETRY_ON = {429, 502, 503,
  504}` plus network errors, gated on `config.enableRetry` (default true) with
  `maxRetries` (default 3). A single-shot 503 mock now sees three requests. 500 is
  deliberately not retried.

---

# Coverage: corrected and re-scoped

**This section is about what did *not* ship.** The coverage work landed a revised
scope, not the originally planned one.

## #581/#582 — one route modelled, five retired as phantoms (#626)

Of the six routes tracked, only `DELETE /timesheet_entries/{id}` was a real gap.
It ships as `DestroyTimesheetEntry` (204, naturally idempotent; 403 when the
caller may not archive or trash the entry). The other five were **already
modelled at their flat spelling and were never missing**:

| Route | Already modelled as |
|---|---|
| `GET/PUT/DELETE /projects/{id}/gauge/needles/{id}` | `GetGaugeNeedle` / `UpdateGaugeNeedle` / `DestroyGaugeNeedle` |
| `GET /projects/{id}/recordings/{id}/timesheet` | `GetRecordingTimesheet` |
| `POST /projects/{id}/recordings/{id}/timesheet/…` | `CreateTimesheetEntry` |

bc3 draws each of those twice, both draws name the same controller action, and
bc3's own API tests assert the two return identical data. They surfaced in the
gap ledger only because the comparison key collapses a leading `/buckets/{id}`
and nothing else. Tracking them was a promise to ship six duplicate operations
across six SDKs for no reachable capability.

They now carry a `modeled_as:` disposition in `spec/bc3-route-allowlist.yml`, and
the disposition is **checked rather than asserted** — "we already model that at
the other spelling" is precisely the claim that shipped `ListForwards` and
`RepositionTodolistGroup` as 404s. The gate requires the named operation to
exist, share the verb, sit elsewhere, be documented at that elsewhere in bc3's
route table, and have this path with a *leading* scope removed — a suffix test,
not a subset test.

**Net: coverage moved by one operation, not by six.**

## Dock-door creation: deferred (#627)

`POST /buckets/{id}/dock/doors` is a real API route and keeps its registry
disposition. The blockers are cost, unchanged — now recorded as a closed decision
rather than an open question. It is **not modelled in v0.13.0**.

Its sibling, `GET /buckets/{id}/dock/doors/{id}`, moved to `out_of_scope`: it is
a redirector (`redirect_to @recording.door.url`), so modelling it would make a
credentialed client follow a redirect to a user-supplied third-party URL in four
of six transports that follow redirects by default — for a value
`ListRecordings` already returns as a plain field.

## Schedule-recurrence writes: deferred (#627)

What shipped is the pre-modelling gate, not the modelling. Recurrence writes
remain **unmodelled**. Reading the source at the pin corrected the ledger entry
twice: bc3 #12362 does not produce a wire-visible validation error — it makes
`valid?` false and the record takes the pre-existing silent-discard path, so
`week_instance: 0` returns 201 with a non-recurring entry — and the read side
does **not** already model `recurrence_schedule`; there is no such shape in
`spec/basecamp.smithy` or `openapi.json`, so absorption must add the output
structure too. The decision is now approvable rather than blocked.

---

# Known gaps

## `recordings.get` has no generic replacement

`GetRecording` was removed with nothing equivalent behind it (see
[Route corrections](#619--three-operations-removed)). If you hold a recording ID
and already know its type, use the type-specific getter — `todos.get`,
`messages.get`, `documents.get`, and so on. That is the intended path and it is
strictly better than the old call, which 404'd.

If you hold an ID whose type you **do not** know, there is no direct read. The
only route left is list-and-filter, and it is expensive enough that you should
treat it as a last resort rather than a migration:

```go
// Go — scan one type at a time; there is no "any type" list.
for _, t := range []basecamp.RecordingType{
    basecamp.RecordingTypeTodo, basecamp.RecordingTypeMessage, basecamp.RecordingTypeDocument,
    basecamp.RecordingTypeComment, basecamp.RecordingTypeUpload, basecamp.RecordingTypeVault,
    basecamp.RecordingTypeScheduleEntry, basecamp.RecordingTypeQuestionAnswer,
    basecamp.RecordingTypeTodolist, basecamp.RecordingTypeKanbanCard,
    basecamp.RecordingTypeKanbanStep, basecamp.RecordingTypeDoor,
} {
    res, err := ac.Recordings().List(ctx, t, &basecamp.RecordingsListOptions{
        Bucket: []int64{projectID},   // narrow it if you can
        Status: "active",             // "archived" and "trashed" need separate passes
    })
    if err != nil { return err }
    for _, r := range res.Recordings {
        if r.ID == wanted { return handle(r) }
    }
}
```

Costs to be clear about: twelve list operations, each paginated across the whole
account unless you can narrow `Bucket`; `Status` defaults to `"active"`, so an
archived or trashed recording needs additional passes; and the polymorphic
`Recording` projection is thinner than the type-specific one, so you may need a
second, typed read once you know the type.

**The durable fix is on your side: persist the type next to the ID.** Every
recording the API hands you carries `type` — `Recording.Type` in Go,
`recording["type"]` elsewhere — and it is the discriminator the type-specific
getters need. An ID stored without its type is an ID you cannot cheaply resolve,
and that was already true at v0.12.0; the removed operation did not resolve it
either, because it 404'd.

This is a real gap and it is recorded as one, not papered over.

## There is no raw-wire migration path

`CreateForwardReply` was removed with nothing behind it. The flat route it
declared was never drawn; a bucket-scoped create does exist upstream, but it is
undocumented and has no upstream coverage, so it is not modelled here.

**Do not reach for the raw client to reinstate it.** Hand-building a path and
calling `Client`/`AccountClient` `Get`/`Post`/`Put`/`Delete` — or the equivalent
in any other SDK — gives up everything the generated operation owns: the path
and verb, the observability hooks, the retry and idempotency configuration,
response decoding, and the error mapping described throughout this guide. It
also pins your code to a route nothing in this repository tests, so it can break
without any signal from a release. The repository holds its own contributors to
the same rule ([AGENTS.md](AGENTS.md), "Never Do These" §4 and §5: never
construct API paths manually, never bypass the SDK).

If you need this operation, the fix is to model it: open an issue, or add the
bucket-scoped route to `spec/basecamp.smithy` and regenerate. That is a change
to the SDK, not a workaround inside your application.

The same applies to any other removal in this release. Where a supported
replacement exists — `recordings.archive` for `todos.trash`, the type-specific
getters for `recordings.get` — it is named in the per-SDK section. Where none
exists, that is stated plainly and no substitute is implied.

## Binary compatibility on the JVM and in Swift was not assessed

Everything in this guide is about **source** compatibility. Nobody measured
whether a binary compiled against v0.12.0 links against v0.13.0.

For Kotlin, [`kotlin/README.md`](kotlin/README.md) has always disclaimed binary
compatibility — default-argument and data-class synthetics make it infeasible to
promise. This release gives that disclaimer plenty of work: data-class `copy` and
`componentN` signatures changed, and `ScheduleEntry` moved three constructor
parameters into positions 13–15. **Recompile. Do not drop a v0.13.0 jar under a
binary built against v0.12.0** — you can hit `NoSuchMethodError` at runtime where
the source would have compiled clean.

That README's *source*-compatibility wording did change in this release, because
v0.13.0 broke it. It previously promised that 0.x public APIs evolve append-only,
so code compiles unchanged across minor versions, with one carve-out for
endpoints Basecamp withdraws. This release violates that repeatedly and
deliberately — `TodolistGroup` and `UpdateScheduleEntryBody` removed,
`Todolist.description` and three `ScheduleEntry` members made required, a leading
`bucketId` added to nine operations — and none of it is a withdrawn endpoint.
Each is a correction to a model that described bc3 wrongly. The policy now says
what the project actually does: append-only by default, but a minor version may
break source compatibility to fix the model, and when it does the breaks are
documented here and the release carries the `breaking` label.

There is deliberately no binary-compatibility-validator `.api` dump in `kotlin/`;
adding one would imply a guarantee the project does not make.

Swift has no equivalent written policy. The same caution applies for the same
reason: member ordering changed on `Todolist` and `ScheduleEntry`, memberwise
initializers moved parameters between the required and defaulted blocks, and the
SDK is distributed as source through SPM, so the ordinary consumer recompiles
anyway. If you vend a prebuilt `.framework` or an `.xcframework` built against
v0.12.0, rebuild it.

For a 0.x release this is an acceptable position — but it is a position, not an
oversight, and it should be stated rather than assumed.

---

# Not in this release

**Nothing is in flight.** Every change this guide describes is merged at
`9a819e44d`, and every count above is a measurement at that commit rather than a
projection. Earlier drafts carried an "if it lands" list; all of it landed, and
the counts were re-derived rather than incremented.

For the record, since the earlier drafts named them and reviewers may be looking
for them:

- **#647 — Cards: the due-date fix** (`46b7f8225`). Folded into
  [Cards: the due-date fix](#cards-the-due-date-fix) and into a class-A entry for
  all six SDKs, plus a Go compile-error entry for
  [`UpdateStepRequest.DueOn`](#updatesteprequestdueon-became-string-647). One
  draft claimed it would have to go Smithy-first; the merged commit touches no
  schema at all, and the `UpdateCardStepRequestContent.DueOn` pointerization that
  prompted that claim came from #560.
- **#648 — the `GetUpcomingSchedule` projection** (#635, #641, #644 —
  `e0431a722`). One compile-or-runtime entry per SDK. It adds **no** class-A or
  class-B entry anywhere: bc3's response body is byte-identical before and after,
  so nothing that used to be populated silently stops being so, and every rename
  and retype is caught statically in Go, Swift, TypeScript and Kotlin and raised
  immediately in Python and Ruby. The operation inventory does not move — 247 on
  both sides, same 14 added / 5 removed / 11 route-moved delta from v0.12.0.
  It does **not** fix `ScheduleEntry.join_url`/`.highlighted`, which were optional
  only *because* `GetUpcomingSchedule` shared the shape; #648 retires that reason
  without tightening them, so they are now under-modelled rather than correctly
  modelled. Tightening touches five other operations and every inline stub in six
  SDKs, so it is deliberately its own diff. #641's three additive members on
  `CreateScheduleEntry` — `url`, `highlighted`, `status` — ride along; the
  read/write spelling split stands, write `url` and read `join_url`.
- **#652 — the projected-example gate** (#638 — `fd939d0bb`). Repository-internal:
  it validates the projected examples against the schema the projection
  publishes. Nothing consumer-visible, no operations added.

Between them #648 and #652 took `make check` from **41 targets to 43**
(`smithy-mapper-test`, then `check-projected-examples`). Derive it with:

```bash
sed -n 's/^check-targets: *//p' Makefile | tr ' ' '\n' | grep -c .
```

**Six PRs landed after this guide's first draft.** All are merged, all are
inside the counts above, and none is pending. They are listed because a reviewer
comparing the guide against `git log` will find them and should not have to work
out which ones mattered:

- **#676 — Python entered the security tooling** (`b0567a77e`). Repository-internal:
  CodeQL, Trivy and ruff's flake8-bandit cover Python for the first time. No
  consumer-visible change and no operations.
- **#679 — bc3 repin plus project archive/unarchive** (`a5bcb3fb2`). Adds
  `ArchiveProject` and `UnarchiveProject`, taking the inventory from 247 to 249.
  It adds no class-A and no class-B entry, for the reason new operations never
  do: there was no v0.12.0 caller of an operation that did not exist.
- **#678 — Link-header parsing** (`a6aa8a1c2`). All six SDKs parsed `Link`
  wrongly, in two different directions, and that part is a fix rather than a
  break. Its `breaking` label is earned by the `maxPages` validation it also
  introduced — see [What shipped](#what-shipped).
- **#645 — event-feed conformance** (`7a32b22c3`). Conformance families and the
  SPEC §23 G-SD repair. Test-suite scope only; it moves no SDK surface, and it
  adds conformance families rather than `make check` targets, so the 43 above
  still holds.
- **#610 — Actions group bump** (`9f0ebad16`). CI only.
- **#680 — two `maxPages` guards that disagreed with their siblings**
  (`9a819e44d`). Fixes, not breaks: Python stopped accepting `max_pages=True`,
  and the TypeScript factory stopped rejecting a `null` that a directly
  constructed service defaulted. Both are described under
  [What shipped](#what-shipped).

Also folded in rather than listed: **#637** (`Todolist.color` and
`comments_app_url` required — `0fd25079c`), **#643** (`basecamp.Ptr` and
`basecamp.Deref` — `51d0d86cf`), **#629** (bare field-map error bodies plus the
cloud-file and Google-document operations — `a373b004c`, which took the inventory
from 241 to 247), **#656** (Ruby's floored attempt cap — `0fa8b461f`), **#658**
(five more Go wrapper timestamps — `6a8a833b3`), **#660** (Kotlin's decoder
strictness — `a3174cf3e`) and **#664** (bare dates on schedule-entry creation —
`2afc97707`).
