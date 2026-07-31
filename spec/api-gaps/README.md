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

> Statuses reflect how BC3's **BC5 API train** actually shipped (8 PRs merged
> to `master`, 2026-07-18..21); BC3 #10947 closed unmerged, superseded by the
> train. Entries with plan phase `post-train` track contracts documented
> after the train (BC3 #12359, merged 2026-07-22). `memories-emptied-regression` is a *subtractive* delta (a field BC4
> populates that BC5 emptied), settled as **permanently empty by documented
> contract** (codified by BC3 #11628) and closed out `absorbed-in-sdk` —
> `GetMyNotificationsOutput.memories` models the settled contract; the flip
> was docs-only, no repopulation; see the entry.
> `stack-doc-and-smithy` is retained as a `confirmed-not-api-resource`
> classification record (Stacks — renamed Folders in the product — are
> web-only on both `four` and `master`).
> `everything-boosts-withdrawn` is likewise *subtractive*: it records BC5
> withdrawing the account-wide `/boosts.json` feed (BC3 #12464, reintroduction
> tracked in #12463) and the SDK's matching removal of `GetEverythingBoosts`;
> its `no-json-contract` is literal — the feed has no JSON API today.
>
> The provenance pin is `e83b2733` (2026-07-30). The `dffa7e11..e83b2733`
> range (96 commits) contains exactly two API-contract changes, both handled:
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
