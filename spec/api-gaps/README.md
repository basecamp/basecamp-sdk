# API Gap Registry

Each markdown file in this directory describes a BC5 user-visible feature or
contract that ships without (or with incomplete) JSON API coverage. The
registry is the SDK side of the [SDK ↔ BC3 coordination](../../COORDINATION.md):
the BC3 parity plan owns server-side delivery; entries here track each gap
from detection through absorption. Status changes flow through git history,
making the absorption journey publicly auditable.

## Lifecycle

1. **Detect**: a gap is identified — by the API gap detector
   (`make detect-api-gaps`), by editorial review of the BC3 parity plan, or
   by an SDK consumer raising a request. A starter entry gets generated or
   authored.
2. **Address**: BC3 ships a JSON API contract for the gap. Entry frontmatter
   updates to `addressed-in-bc3-pr-N`.
3. **Absorb**: SDK opens a follow-up PR adding the Smithy operations and
   regenerated SDK code. Frontmatter updates to `absorbed-in-sdk` with
   Smithy structure refs.
4. **Archive**: entries more than a year past `absorbed-in-sdk` may be moved
   to `archive/` for tidiness; they remain readable as historical record.

## Statuses

| Status | Meaning |
|---|---|
| `no-json-contract` | Detected gap; no JSON API exists yet. |
| `partial-coverage` | Some elements exist (partial, render path) but doc and/or Smithy are missing. |
| `ambiguous` | BC3 has not yet classified whether this is API-shaped or UI-only. |
| `confirmed-not-api-resource` | BC3 confirmed UI-only / not part of the API surface; entry retained as classification record. |
| `addressed-in-bc3-pr-N` | BC3 has shipped a JSON API contract; SDK absorption pending. |
| `absorbed-in-sdk` | SDK has absorbed the contract via Smithy + regenerated code. |

## Entries (current)

| Gap | Status | BC3 plan phase | SDK demand |
|---|---|---|---|
| [calendar](calendar.md) | absorbed-in-sdk | 3b | medium |
| [scratchpad](scratchpad.md) | absorbed-in-sdk | 3b | medium |
| [step-top-level](step-top-level.md) | absorbed-in-sdk | 3b | low |
| [everything-aggregates](everything-aggregates.md) | absorbed-in-sdk | 3c | high |
| [activity-timeline](activity-timeline.md) | absorbed-in-sdk | 3d | high |
| [recordable-subtypes-doc](recordable-subtypes-doc.md) | partial-coverage | 3a | medium |
| [stack-doc-and-smithy](stack-doc-and-smithy.md) | confirmed-not-api-resource | 3b | medium |
| [search-filter-additions](search-filter-additions.md) | absorbed-in-sdk | 3e | medium |
| [rich-text-project-attachable](rich-text-project-attachable.md) | no-json-contract | 3e | low |
| [recording-bubbleupable-field](recording-bubbleupable-field.md) | no-json-contract | 3e | low |
| [todoset-completed-list-visibility](todoset-completed-list-visibility.md) | ambiguous | 3a | low |
| [memories-emptied-regression](memories-emptied-regression.md) | absorbed-in-sdk | launch | high |
| [campfire-line-edit](campfire-line-edit.md) | absorbed-in-sdk | post-train | medium |
| [todoset-direct-todo-create](todoset-direct-todo-create.md) | absorbed-in-sdk | post-train | medium |
| [schedule-recurrence-writes](schedule-recurrence-writes.md) | addressed-in-bc3-pr-12359 | post-train | medium |
| [dock-tool-create-contract](dock-tool-create-contract.md) | absorbed-in-sdk | launch | medium |
| [upload-new-version](upload-new-version.md) | no-json-contract | post-train | medium |
| [todolist-reposition](todolist-reposition.md) | absorbed-in-sdk | pre-BC5 | medium |
| [rich-text-attachments-coverage](rich-text-attachments-coverage.md) | absorbed-in-sdk | n/a | medium |
| [visible-to-clients-on-creates](visible-to-clients-on-creates.md) | absorbed-in-sdk | post-train | medium |
| [external-links-doors](external-links-doors.md) | partial-coverage | post-train | low |
| [my-bookmarks](my-bookmarks.md) | absorbed-in-sdk | master | medium |
| [my-drafts](my-drafts.md) | absorbed-in-sdk | master | medium |
| [my-assignments-priorities](my-assignments-priorities.md) | absorbed-in-sdk | master | medium |
| [dock-tool-visible-to-clients](dock-tool-visible-to-clients.md) | absorbed-in-sdk | post-train | low |
| [card-table-wormholes](card-table-wormholes.md) | absorbed-in-sdk | post-train | medium |
| [bubble-ups-surface](bubble-ups-surface.md) | absorbed-in-sdk | launch | high |
| [everything-boosts-withdrawn](everything-boosts-withdrawn.md) | no-json-contract | post-train | medium |
| [everything-todo-card-filters](everything-todo-card-filters.md) | absorbed-in-sdk | post-train | medium |
| [folders-api](folders-api.md) | absorbed-in-sdk | master | medium |
| [event-feed](event-feed.md) | no-json-contract | n/a | high |

> Statuses reflect how BC3's **BC5 API train** actually shipped (8 PRs merged
> to `master`, 2026-07-18..21); BC3 #10947 closed unmerged, superseded by the
> train. Entries with plan phase `post-train` track contracts documented
> after the train (BC3 #12359, merged 2026-07-22). `memories-emptied-regression` is a *subtractive* delta (a field BC4
> populates that BC5 emptied), settled as **permanently empty by documented
> contract** (codified by BC3 #11628) and closed out `absorbed-in-sdk` —
> `GetMyNotificationsOutput.memories` models the settled contract; the flip
> was docs-only, no repopulation; see the entry.
> `stack-doc-and-smithy` is retained as a `confirmed-not-api-resource`
> classification record of the **launch** decision, and is now **superseded by
> [`folders-api`](folders-api.md)**: Stacks — renamed Folders in the product,
> though the wire `type` is still `Stack` — were web-only at launch, but BC3
> **#12384** (`dc6cd10714`) has since shipped full CRUD JSON on `master`
> (`GET`/`POST /stacks.json`, `GET`/`PUT`/`DELETE /stacks/{id}.json`), live in
> production with public docs at `basecamp/bc-api` #420 (`401c8ebcc9`). Read
> `folders-api.md`, not the superseded entry, for the contract. All five
> routes are now modelled (`FoldersService`), so `folders-api` is
> `absorbed-in-sdk`; `stack-doc-and-smithy` keeps its
> `confirmed-not-api-resource` status deliberately — it records a decision that
> was correct when made, and rewriting it would destroy the history the
> supersession note exists to preserve.
> `everything-boosts-withdrawn` is likewise *subtractive*: it records BC5
> withdrawing the account-wide `/boosts.json` feed (BC3 #12464, reintroduction
> tracked in #12463) and the SDK's matching removal of `GetEverythingBoosts`;
> its `no-json-contract` is literal — the feed has no JSON API today.
>
> The provenance pin is `4dd2926f8a` (2026-08-03). <!-- @bc3-pin -->
> That line is checked by `make doc-constants-check` and deliberately *not*
> rewritten by `make sync-api-version`: this file is in
> `spec/doc-constants.json` `.writerExcludes`, because the pin sentence heads
> the range triage below and cannot advance without that triage advancing too.
> The ranges themselves are settled history and stay unmarked.
>
> The `2c0dafba13..4dd2926f8a` range (6 commits) contains exactly **one**
> API-contract change, and it is the one this repin exists to absorb: BC3
> **#12502** (`4dd2926f8a`) preserves a schedule entry's join link and
> highlight across a sparse update — `PRESERVED_ON_OMISSION = %i[ url
> highlighted ]` in `Schedules::EntriesController`, plus the other half, which
> is *emitting* them: `highlighted`, and the join link as **`join_url`** under
> a non-colliding key, in `api/schedules/entries/_entry.json.jbuilder` and
> `api/schedules/entries/occurrences/show.json.jbuilder`. It is the only commit
> in the range touching `doc/api` (8 added lines in
> `schedule_entries.md`) or `config/routes.rb` (none), so `spec/bc3-routes.json`
> regenerates with no route delta — the change is to a payload and an omission
> rule on an already-modelled endpoint, absorbed here as `ReplaceScheduleEntry`
> and its `preservedOnOmission` carve-out. The remaining five commits are one
> mail-infrastructure map, two CSS-only, one account-calendar authorization
> reassessment on profile change, and one authentication-cache crash fix; none
> touches `doc/api`, `config/routes.rb`, or an API view.
>
> Earlier pins, kept as the triage record. Each names the pin it was written
> against, in the past tense, because that is what it is — a range triaged
> once, at the repin that set its end. Only the sentence above is a claim
> about today.
>
> The pin was `2c0dafba13` (2026-08-02). The `d0edc1283b..2c0dafba13` range (11 commits) contained exactly **one**
> API-contract change: BC3 **#12384** (`dc6cd10714`, the Folders API),
> absorbed here as `FoldersService` and recorded in
> [`folders-api.md`](folders-api.md). BC3 **#12494** (`344581a379`) and
> **#12501** (`2c0dafba13`) preserve a draft's subscribers when an update does
> not address them — behaviour fixes to already-modelled endpoints that add
> prose to five `doc/api` sections without adding a route or changing a payload
> shape, confirmed by regenerating `spec/bc3-routes.json`, whose only delta is
> the five `/stacks` routes. BC3 **#12488** (`19956c5579`) reads alarming
> because it deletes a `request.format.json?` branch, but it moves the **web**
> deprioritize path onto the exact-target contract the JSON API has had since
> bc3#12483 (absorbed in SDK #528): the removed branch returned `nil` for JSON,
> and the replacement returns the same recording's own assignment, so
> `DELETE /my/priorities/{id}.json` still targets exactly the id in the URL and
> still answers `204`. The remaining seven commits are four dev-tooling, two
> Turbo-morph web-only, and one push-notification backend swap.
>
> The pin was `d0edc128` (2026-07-31). The `e83b2733..d0edc128` range was
> triaged at the Up Next priority-writes repin (SDK #528): the only `doc/api`
> or routes change in it is `my_assignments.md`'s exact-target Deprioritize
> contract (BC3 **#12483**), absorbed by that PR and recorded in
> [`my-assignments-priorities.md`](my-assignments-priorities.md); everything
> else (BC3 #12478/#12479/#12480/#12481/#12444, a completion-lock race fix and
> relay-revocation ops tooling) is wire-neutral internals.
>
> The pin was `e83b2733` (2026-07-30); the `dffa7e11..e83b2733` range (96
> commits) contains exactly two API-contract changes, both handled:
> BC3 **#12464** (`b06acfac1`, boosts-feed withdrawal — absorbed by the SDK's
> removal, recorded in `everything-boosts-withdrawn.md`) and BC3 **#12442**
> (`b238a0743`, `assignee_ids[]`/`due` filters on the everything to-do/card
> API — registered in `everything-todo-card-filters.md`). The only other
> route changes are BC3 #12339's removal of the bare `/todos` and `/cards`
> flat routes (HTML shells, never API contract — see
> `everything-aggregates.md`'s standing exclusions) and two non-API engine
> mounts; the remainder of the range is UI/CSS/mobile/infra with no
> `doc/api` impact.
>
> The note below records the triage that accompanied the earlier `c3086931`
> (2026-07-26) repin. The earlier
> `ca1d34bc..d7bc88da` sub-range — 30 commits — is a reviewed no-op for the SDK:
> **26** UI/CSS/JS/lexxy commits (including their PR merge commits
> #12208/#12399/#12400/#12401); the **3** duplicate-cookie migration code commits
> (`88cb86d2a`, `52f1e9974`, `7d86f1d06`) that touch only
> `app/controllers/concerns/authenticate/by_cookie.rb` (a session-cookie concern)
> and its test; and the **1** merge commit of the cookie PR (#12335, `d7bc88da`).
> The `d7bc88da..640389c2` step is a **single** commit — BC3 **#12383** ("Add My
> Bookmarks JSON API", `640389c2`) — a real, net-new API surface (paginated
> `GET /my/bookmarks.json` plus `GET`/`POST`(201)/`DELETE`(204)
> `/recordings/{id}/bookmark.json`, the two mutations idempotent), registered in
> [`my-bookmarks.md`](my-bookmarks.md). The final `640389c2..c3086931` step is
> **two** net-new API commits, both registered (not absorbed here):
> BC3 **#12381** ("Add My Drafts API", `123b2320`) — paginated
> `GET /my/drafts.json` returning a flat draft envelope across the user's active
> projects — in [`my-drafts.md`](my-drafts.md); and BC3 **#12380** ("My Tasks:
> harden Up Next reorder and document the assignment API", `c3086931`) — the
> Up Next priority-management writes `POST /my/priorities.json`,
> `DELETE /my/priorities/{id}.json`, and `POST /my/priority_moves.json` (all
> `204`, the reorder with a documented 400/422/404 error contract) — in
> [`my-assignments-priorities.md`](my-assignments-priorities.md). SDK absorption
> of all three is tracked as follow-ups. The pin **date advances** to 2026-07-26,
> so the SDK spec/API version bumps with this sync.

The detector also maintains [`allowlist.yml`](allowlist.yml) for routes
classified as not-an-API-resource or absorbed under another entry. Allowlist
records are lighter-weight than entries and serve a different purpose:
entries preserve the *investigation history* of candidates that warranted
SDK-side review; allowlist records cover routes that should never have
warranted an entry in the first place. Pick one per candidate, never both.

## Validating

```
make validate-api-gaps
```

Validates frontmatter on every entry against [`schema.json`](schema.json)
and the allowlist against [`allowlist-schema.json`](allowlist-schema.json).
Wired into `make check`.

## Detecting new gaps (planned)

Today, entries are added by hand when a gap is identified. Automated
detection — diffing routes between BC3 master and the active branch,
classifying each new route against multi-signal heuristics, and emitting
starter entries for human review — will arrive in a later PR. The intended
invocation will be:

```
BC3_REPO_PATH=~/Work/basecamp/bc3 make detect-api-gaps
```

The `detect-api-gaps` Make target does not yet exist; running this now will
error.
