# Migrating

Upgrade notes for consumers of the Basecamp SDKs. One section per release that
breaks something, newest first. Read the section for your version range before
you upgrade, not after.

This file exists because the repo generates its release notes from PR labels
(see [CONTRIBUTING.md](CONTRIBUTING.md)). That produces an accurate list of what
merged; it cannot tell you which of those changes your code has to react to, or
what wrong behaviour you get if you ignore one. This file is that half.

---

# v0.13.0

Breaking across all six SDKs — Go, TypeScript, Python, Ruby, Kotlin, Swift.

**Read [Silent breaks](#silent-breaks) first.** A large share of this release
gives the consumer no signal at all: no compile error, no exception, no decoder
failure. The call keeps working and does something different.

| SDK | breaking changes | of which silent |
|---|---:|---:|
| [Go](#go) | 27 | **11** |
| [Swift](#swift) | 20 | **9** |
| [TypeScript](#typescript) | 16 | 5 |
| [Python](#python) | 14 | 4 |
| [Ruby](#ruby) | 16 | 3 |
| [Kotlin](#kotlin) | 14 | 3 |

Every claim below was read out of `git diff v0.12.0..main`, not out of a PR
body. Base `v0.12.0` = `7e2925d25`.

## What shipped

**Operation inventory: 238 → 241.** Re-derive it yourself rather than trusting
the number:

```bash
python3 -c "import json;d=json.load(open('openapi.json'));\
print(sum(1 for p,v in d['paths'].items() for m in v if m in ('get','post','put','patch','delete')))"
```

Eight operation IDs added, five removed. At the level of *capability* that is
six additions, three removals and two renames:

| | |
|---|---|
| **Added (6)** | `ListFolders`, `GetFolder`, `CreateFolder`, `UpdateFolder`, `DeleteFolder` (#593); `DestroyTimesheetEntry` (#626) |
| **Renamed (2)** | `UpdateDocument` → `ReplaceDocument` (#601); `UpdateScheduleEntry` → `ReplaceScheduleEntry` (#632). Same route, same verb, honest name. |
| **Removed (3)** | `GetRecording`, `TrashTodo`, `CreateForwardReply` (#619) |

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

---

# Silent breaks

A break is listed here if **nothing tells you**: no compiler error, no
exception, no decoder failure, no test that fails for the right reason. The
call keeps working and does something different.

## Hits every SDK

### 1. `page` now selects one page instead of starting a walk (#617)

At v0.12.0, seventeen already-paginated operations treated `page` as a
**starting offset**: the SDK put `page=N` on the first request and then
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

## Go — 11 silent breaks

1. **Five optional timestamps became `*time.Time` (#615).** `hc.UpdatedAt.IsZero()`
   still compiles — Go inserts the dereference — and panics on nil. Fields:
   `HillChart.UpdatedAt`, `Notification.ReadAt`, `Notification.UnreadAt`,
   `SearchResult.CreatedAt`, `SearchResult.UpdatedAt`. The two `SearchResult`
   tags also gained `omitempty`, so marshalling one now omits the key where it
   used to emit a fabricated `0001-01-01T00:00:00Z`.
2. **~107 optional fields in `pkg/generated` with struct or named types became
   pointers (#560).** `a.Limits.CanUploadFiles` and `t.DueOn.String()` both
   compile and both panic. Of 653 value→pointer flips, 527 scalars and 19 slices
   break loudly; the rest do not.
3. **Nested optional objects switched to pointer presence (#560).** 34 guards
   across 17 files flipped from content-inference to `!= nil`. A
   present-but-empty object that used to yield nil now yields a non-nil struct.
4. **`Question.Schedule.Hour` and `.Minute` can now be nil (#560).** The one flip
   in the release running non-nil → nil.
5. **A non-nil empty slice now reaches the wire (#560).** `Types: []string{}`
   used to be a no-op; it now sends `{"types":[]}` and clears the list.
6. **`CardColumns().Move` always sends `position`, including 0 (#560).**
7. **`Page` is a selector (#561, #617)** — see cross-SDK #1.
8. **`Todolists().Update` is read-modify-write (#574)** — see cross-SDK #4.
9. **400/422 from the raw `Client`/`AccountClient` escape hatch now report
   `CodeValidation`, not `CodeAPI` (#549).**
10. **`pkg/generated` `Parse*Response` went lenient on 4xx/5xx (#541).**
11. **Two URLs changed (#586)** — see cross-SDK #2.

Three more are *partly* silent: `Documents().Update` and
`Schedules().UpdateEntry` (the compiler catches only the `pkg/generated` half),
and `Error` gaining `FieldErrors` (the compiler catches only unkeyed composite
literals).

## Swift — 9 silent breaks

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
4. **`forwards.list` emits `/inbox_forwards.json` (#586).**
5. **`todolistGroups.reposition` emits `/todolists/groups/{id}/position.json`
   (#586).** A regex stub on `todolists/\d+/position\.json` cannot match the new
   path and fails open.
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

## TypeScript — 5 silent breaks

1. **Every error message from a generated service call was previously the HTTP
   status text (#541).** At v0.12.0 `BaseService.handleError` discarded the body
   openapi-fetch had already parsed and re-read a spent `Response`, which throws,
   is swallowed, and falls back to `statusText`. So `403 {"error":"You are not
   allowed"}` gave `"Forbidden"`. On main the server's text reaches `message` at
   every status. **Any `e.message === "<statusText>"` comparison is now dead
   code.**
2. **Eleven operations emit different URLs (#586, #619).**
3. **Operation IDs seen by hooks were renamed, removed and doubled.**
   `OperationInfo.operation` is a plain `string`, so a stale comparison compiles
   and never matches again — an audit hook that was gating writes silently stops
   gating them.
4. **`page` selects a page (#617)** — see cross-SDK #1.
5. **`client.downloadURL()` now retries hop 1 (#563).** Single-shot mocks
   misbehave; if you wrapped `downloadURL` in your own retry loop you now have
   nested retry.

## Python — 4 silent breaks

1. **`ValidationError` message text changed and `field_errors` is new (#541).**
   `str(e)` went from `"Validation failed"` to `"color: is not a valid color"`.
2. **Bare field-map bodies now populate `field_errors` (#549).** Recognition is
   all-or-nothing by shape: one member that is not a non-empty list of non-empty
   strings disqualifies the whole body, so never assume a 400/422 yields a field
   map.
3. **Two URLs changed (#586)** — see cross-SDK #2.
4. **`account.download_url`'s first hop retries and dropped its `Accept` header
   (#563).** v0.12.0 sent one request with `Accept: application/json` and raised
   on a 503. Main sends no `Accept` at all and retries `{429, 502, 503, 504}`
   plus network errors. Cassettes matching on `Accept` stop matching.

Add three more with no signal whatsoever, filed as behavioural: `todolists.update`,
`documents.update` and `schedules.update_entry` keep byte-identical keyword
sets, raise nothing on the happy path, and produce no type-checker complaint.

## Ruby — 3 silent breaks

1. **`page:` selects a page (#617)** — see cross-SDK #1. Measured against a
   4-page stub: v0.12.0 issued three requests and returned three items for
   `page: 2`; main issues one and returns one.
2. **`ValidationError#message` changed and `#field_errors` is new (#541, #549).**
   `e.message == "Request failed"` stops matching.
3. **`Draft#scheduled_posting_at` and `MyNote#created_at`/`#updated_at` decode to
   `Time`, not `String` (#560).** `.start_with?` raises `NoMethodError`,
   `Time.parse(…)` raises `TypeError`, and bare interpolation silently changes
   format from `"2026-01-02T03:04:05Z"` to `"2026-01-02 03:04:05 UTC"`.
   `#iso8601` gets the old string back.

## Kotlin — 3 silent breaks

1. **`documents.update` quietly stopped erasing omitted fields (#601).**
   `UpdateDocumentBody` was removed from the generator and re-declared by hand
   **in the same package with a character-identical public shape**, and
   `documents.update(id, body): Document` kept its exact signature. The call site
   compiles untouched and does something different.
2. **Two URLs changed (#586)** — see cross-SDK #2.
3. **Error message composition changed at every status (#541, #549)** — see
   cross-SDK #3. The same parser now backs `account.downloadURL`, so download
   failure messages moved too.

Two more with no build-time signal, filed as behavioural: `page` selection
(#617) and `downloadURL` retrying hop 1 three times (#563).

---

# Route corrections

Operations that declared URLs bc3 does not serve. **Read the removals carefully
— one of them was serving.**

## #586 — two spellings corrected, no signature change

Covered above as [silent break #2](#2-two-operations-emit-a-different-url-under-an-unchanged-signature-586).
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
  three requests. Python's hop 1 also stopped sending `Accept: application/json`.
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

Go has the most invasive changes in this release — eleven are silent, and two of
those panic at runtime on code the compiler accepts.

Before anything else: **the SDK exports no pointer helper.** Declare your own
once:

```go
func ptr[T any](v T) *T { return &v }
```

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
t := time.Time{}
if n.ReadAt != nil { t = *n.ReadAt }
```

Two consequences past the panic: absence used to read as
`0001-01-01T00:00:00Z` and now reads as nil, and `json.Marshal` of a
`SearchResult` now omits `created_at`/`updated_at` when absent. Check persisted
serialization and downstream strict decoders.

### `pkg/generated`: ~107 struct- and time-typed optional fields became pointers (#560)

```go
// both lines still COMPILE on main; the first one panics
_ = a.Limits.CanUploadFiles     // Account.Limits is now *AccountLimits
fmt.Println(t.DueOn.String())   // Todo.DueOn is now *types.Date
```

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

```go
resp, err := ac.Post(ctx, "/999/buckets/1/todolists/2/todos.json", body)
if apiErr, ok := err.(*basecamp.Error); ok {
    switch apiErr.Code {
    case basecamp.CodeAPI:        // a 422 used to land here
    case basecamp.CodeValidation: // it lands here now, with FieldErrors populated
    }
}
```

Only `Client`/`AccountClient` `Get`/`Post`/`Put`/`Delete` changed; typed service
methods already returned `CodeValidation` for both statuses. Match on
`apiErr.HTTPStatus` if you need the old grouping.

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

No SDK replacement. If you need it, use the escape hatch against the
bucket-scoped route and accept that it is untested upstream:

```go
ac.Post(ctx, fmt.Sprintf("/buckets/%d/inbox_forwards/%d/replies.json", bucketID, forwardID), body)
```

### `UpdateScheduleEntryRequest` fields became pointers (#632)

```go
_, err := ac.Schedules().UpdateEntry(ctx, entryID, &basecamp.UpdateScheduleEntryRequest{
    Summary:        ptr("Standup"),
    StartsAt:       ptr("2026-01-01T09:00:00Z"),
    EndsAt:         ptr("2026-01-01T09:15:00Z"),
    ParticipantIDs: ptr([]int64{7, 9}),
    Notify:         ptr(true),
})
```

The pointers are load-bearing: nil means "leave the fetched value alone", a
pointer to the zero value means "set it to empty" — `Description: ptr("")` clears
it. Two new fields: `URL` (the join link) and `Highlighted`. `AllDay` was already
`*bool`. **`StartsAt`/`EndsAt` are no longer RFC3339-validated client-side** — a
malformed timestamp now reaches the server instead of failing locally, so bc3's
bare-date all-day rendering round-trips.

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

Tri-state: nil leaves it untouched, `ptr("")` clears it. At v0.12.0 an empty
string was indistinguishable from unset and could not clear.

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

Swift has the most no-signal breaks — nine — because three of its request types
were replaced by same-named hand-written ones, and two of its retry and error
policies changed under unchanged signatures.

One soft edge to know before you start: optional → non-optional breaks `if let`,
`guard let` and `?.` hard, but `x ?? default` only **warns**. A consumer whose
entire usage is `??` sees nothing.

## Silent

All nine are in [Silent breaks](#swift--9-silent-breaks). Two deserve code here.

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
`URLProtocol` stub or cassette or decoding throws. Three new optional members are
additive: `color`, `commentsAppUrl`, `groupPositionUrl`.

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
`forwards.createReply` with `CreateForwardReplyRequest` (no replacement;
`listReplies`/`getReply` are unaffected).

## Behavioural

- **`documents.update` and `schedules.updateEntry` are merge-safe composites** —
  see Silent. For schedule entries, the two-tier rule is: full-state fields
  (`summary`, `startsAt`, `endsAt`, `description`, `allDay`) are resent from the
  read-back when you pass nil, so nil means untouched and `""` means clear;
  carve-outs (`participantIds`, `url`, `highlighted`, `notify`) are omitted
  entirely when nil so bc3 preserves them, and `[]`/`""`/`false` explicitly
  clears. Recurring entries are out of reach on this route — bc3 302-redirects
  both show and update for them.
- **`downloadURL`'s first hop now makes three attempts (#563)**, retrying network
  errors plus `{429, 502, 503, 504}` — never 500. Every attempt is
  authenticated; the signed second hop is still exempt. There is no public
  numeric knob: `DownloadURL` is deliberately absent from `behavior-model.json`.
  `enableRetry: false` collapses it to one attempt.

---

# TypeScript

TypeScript catches most of this at build time. The exception is the error path —
at v0.12.0 every error from a generated service call carried the HTTP status text
rather than the server's message, and fixing that silently changed every message
string you might be matching on.

## Silent

See [Silent breaks](#typescript--5-silent-breaks). Two details on `fieldErrors`
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
which is unrelated noise.

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
`group_position_url`, `color` (`string | null` — `null` is the ordinary case for
a group, so `list.color.toUpperCase()` throws), `comments_app_url`.

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

### The exported `paths` type lost nine route keys and re-typed two 422 bodies (#619, #586, #549)

Only affects code that types itself off the root `paths` export. The trap is the
five **removed operation slots** (`TrashTodo`, `GetRecording`,
`CreateForwardReply`, `UpdateDocument`, `UpdateScheduleEntry`): their path keys
survive because a sibling verb still lives there, so
`paths["/todos/{todoId}"]["delete"]` resolves to `never | undefined` and compiles
clean. Grep for those five names rather than trusting tsc. The 422 re-tags are
exactly `UpdateMyNote` and `UpdateMyPreferences`.

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
- **`todos.update()`, `todos.edit()` and `cards.update()` now throw on a
  malformed read-back (#597).** v0.12.0's `?? ""` coalesced only null/undefined;
  a wrong-typed field rode into the replacement PUT. `cards.update` was worst —
  `if (current.due_on)` both dropped a falsey non-string (which is how bc3 erases
  a due date) and forwarded a truthy non-string. The new error is statusless,
  `code === "api_error"`, `retryable === false`, and never reaches the wire.

---

# Python

Python has no compile step: every one of these lands at runtime, in production,
on the first call that takes that branch. The package ships `py.typed`, so
mypy/pyright catch the signature changes if you type-check — untyped code finds
out on the call.

## Silent

See [Silent breaks](#python--4-silent-breaks). Plus the three merge-safe
composites, which have byte-identical keyword sets and no signal of any kind.

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

No replacement. Reads are unaffected: `forwards.list_replies` and
`forwards.get_reply` remain.

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

Everything else about this is silent in Python: `basecamp/generated/types.py` is
imported by nothing in the SDK and every generated service method is annotated
`-> dict[str, Any]`. There is no decoder to throw.

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
`account.forwards.create_reply` → nothing; `list_replies` is unaffected.

### `Basecamp::Types::TodolistGroup` no longer exists (#628)

`NameError` at first reference. Use `Basecamp::Types::Todolist`;
`Todolist.required_fields` gained `:description`. Two parallel changes bite
identical code: `ScheduleEntry.required_fields` gained `:all_day`, `:ends_at`,
`:starts_at`, and `ScheduleEntry` gained `#highlighted` and `#join_url`. Blast
radius is narrow — **no SDK service method returns a `Types::` object**; the
service layer returns raw Hashes. This only bites code that constructs these
value objects itself.

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

### `todos.update`/`edit` and `cards.update` refuse a malformed read-back (#597)

v0.12.0's `todo["content"] || ""` turned a `false` content into `""` and wrote it
back — erasing the field on a call that never mentioned it. `cards.update`
forwarded `due_on` verbatim. Both now raise `Basecamp::ApiError` (statusless,
`retryable: false`) before the PUT. Should never fire against a healthy server;
it fires instead of silently corrupting the record when one misbehaves.

### Two URLs changed (#586)

Loud in a stubbed suite (WebMock raises on an unregistered request), silent
against live bc3.

---

# Kotlin

Kotlin catches the type work at compile time. The two things to watch are decoder
throws on payloads your fixtures may not carry, and one silent semantic change
hidden behind a hand-written compatibility shim.

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

### Two URLs changed (#586), error composition changed (#541, #549)

See [Silent breaks](#kotlin--3-silent-breaks). Two source-compatible widenings
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

A missing key gives `MissingFieldException`; an explicit `"description": null`
gives `JsonDecodingException`, because `description` is required and **not**
nullable. The client's `coerceInputValues = true` rescues neither. The throw
escapes as a raw `SerializationException` from
`todolists.get/replace/list/create` and `todolistGroups.list/create` —
`catch (e: BasecampException)` does not see it. The merge-safe
`todolists.update`/`edit` are the exception; they wrap it into
`BasecampException.Api`.

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
(no substitute; `CreateForwardReplyBody` is gone from `Types.kt` with no compat
shim).

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

## Binary compatibility on the JVM and in Swift was not assessed

Everything in this guide is about **source** compatibility. Nobody measured
whether a binary compiled against v0.12.0 links against v0.13.0.

For Kotlin the answer is already policy, and it has not changed:
[`kotlin/README.md`](kotlin/README.md) states that the 0.x series guarantees
source compatibility only, that Kotlin default-argument and data-class synthetics
make JVM binary compatibility infeasible to promise, and that you should
recompile against each release. This release gives that policy plenty of work:
data-class `copy` and `componentN` signatures changed, and `ScheduleEntry` moved
three constructor parameters into positions 13–15. **Recompile. Do not drop a
v0.13.0 jar under a binary built against v0.12.0** — you can hit
`NoSuchMethodError` at runtime where the source would have compiled clean.

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

Three changes were in flight when v0.13.0 was cut. If you are reading this
against a later release, check whether they landed.

- **#637 — `Todolist.color` and `Todolist.comments_app_url` become required.**
  Open at the time of writing. When it lands it is source-breaking for Go,
  Kotlin, Swift, TypeScript, Python and Ruby, but **the wire does not change**:
  bc3 has always emitted both keys. `color` is modelled required **and
  nullable**, so an explicit `"color": null` stays valid everywhere — Swift
  emits `decode(String?.self, forKey:)` and Kotlin types it `String?` with no
  default, and both accept null and reject only *absence*. Existing fixtures
  that render `"color": null` need no change; fixtures that omit the key do.
  `comments_app_url` is required and non-nullable, and additionally loses its
  optionality in the typed SDKs.
- **#629 — bare field-map error bodies**, widening the shapes `field_errors`
  recognises and adding six cloud-file and Google-document operations. Held as a
  draft at the time of writing; if it lands before the tag, the operation
  inventory is no longer 241 and the number above must be re-derived.
- **#635 / #641 — the `GetUpcomingSchedule` projection and the
  `CreateScheduleEntry` `url`/`highlighted` asymmetry.** Both are open issues
  with no implementation pushed. #641 in particular describes an asymmetry
  introduced on main by #632 — `url` and `highlighted` are accepted on replace
  but not on create — which v0.13.0 would be the first release to ship.
