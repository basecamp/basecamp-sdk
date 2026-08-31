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
   Smithy structure refs. Surfaces the architecture deliberately keeps
   outside the OpenAPI spec (OAuth, SPEC §16) can never take this step —
   they close as `covered-outside-spec` instead, pointing at the covering
   hand-written surface via `bc3_refs.related_existing_api`.
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
| `covered-outside-spec` | Terminal. Contract shipped upstream and covered by the SDK's sanctioned hand-written surface (`bc3_refs.related_existing_api`); deliberately outside the OpenAPI spec, so `absorbed-in-sdk` can never apply. |

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
| [upload-new-version](upload-new-version.md) | absorbed-in-sdk | post-train | medium |
| [upload-create-subscriptions](upload-create-subscriptions.md) | partial-coverage | n/a | low |
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
| [project-archive-unarchive](project-archive-unarchive.md) | absorbed-in-sdk | master | medium |
| [recording-spotlights](recording-spotlights.md) | absorbed-in-sdk | master | medium |
| [notifications-sort-pings-first](notifications-sort-pings-first.md) | partial-coverage | master | low |
| [bc5-authorization-document-shape](bc5-authorization-document-shape.md) | covered-outside-spec | master | medium |
| [subtasks-canonical-rename](subtasks-canonical-rename.md) | partial-coverage | master | low |

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
> The provenance pin is `824013d672d` (2026-08-31). <!-- @bc3-pin -->
> That line is checked by `make doc-constants-check` and deliberately *not*
> rewritten by `make sync-api-version`: this file is in
> `spec/doc-constants.json` `.writerExcludes`, because the pin sentence heads
> the range triage below and cannot advance without that triage advancing too.
> The ranges themselves are settled history and stay unmarked.
>
> The `71b43f3d9fa..824013d672d` range is **1,109 commits** (156 merges, 953
> non-merge). It adds exactly one documented API family: BC3 #12860
> (`06edca6f0b`), the two recording Spotlight state transitions absorbed in
> [`recording-spotlights.md`](recording-spotlights.md). `POST`
> `/recordings/{id}/spotlight.json` returns the ordinary recording projection
> with 201 and is naturally idempotent; `DELETE` on the same path returns 204
> and is also naturally idempotent. The bucket-scoped spellings remain legacy
> aliases, and the undocumented position route remains web-only.
>
> The rest of the final `doc/api` delta is wire-neutral. `questions.md` now
> states that a client may update only a question they created and otherwise
> receives 403; `UpdateQuestion` already includes `ForbiddenError`.
> `schedule_entries.md` now states that a failed create returns 422 with an
> `errors` payload; `CreateScheduleEntry` already includes `ValidationError`.
> `doc/api/README.md` only links the new Spotlights page.
>
> The final `app/views/api` delta was also inspected rather than inferred from
> the docs alone. Authorization accounts switch their implementation from the
> Queenbee identifier to the account's public identifier and construct `href`
> from the API origin, without changing either field's JSON type or meaning.
> Three new attachment render partials prevent missing-template failures while
> emitting HTML inside already-modelled rich-text/search strings. The internal
> Help Scout bridge is cookie-only and undocumented. Project dock JSON now
> excludes unreleased recordable types (including the abandoned separate
> Spotlight-record design), preserving the documented dock contract; the
> recording partial emits boost links only for actually boostable recordings,
> whose polymorphic members were already optional. None requires another
> Smithy member or operation.
>
> Controller and route drift was checked through the pinned route-table
> regeneration and bidirectional route parity, not treated as API merely
> because a Rails controller changed. The table moves from 373 routes across
> 64 sections to 377 across 65: exactly the Spotlight canonical pair and their
> documented compatibility aliases. The `bc3-four` compatibility pin does not move: this sync
> re-verifies only `master`.
>
> The previous `b5d8c9df8d..71b43f3d9fa` range was **213 commits** (29 merges, 184
> non-merge), and this repin absorbs no operations: openapi.json's operation
> set is untouched, and the only Smithy diff is member documentation (item 3).
> Two UI programs dominate the bulk — the unified sidebar (BC3 #12279, 79
> non-merge commits including its #12397 sub-branch) and the server-rendered
> calendar (BC3 #12405, 40) — and neither changes a documented payload. Under
> `--full-history`, exactly **six** non-merge commits touch `doc/api` or
> `app/views/api`; a default `git log` over those paths shows only four,
> because two are tree-same twins (item 4) — a path-filtered log is not a
> complete triage instrument for this range. Six more commits change
> API-reachable behavior from the controller layer with no doc or view diff
> (items 6–9). `gems/*/app/views/api` is untouched.
>
> 1. BC3 **#12646** (`71b43f3d9fa`) — **already closed; this repin ratifies
>    it.** `/authorization.json`'s `expires_at` becomes ISO 8601 — it was the
>    only integer-epoch timestamp in bc3's public JSON API — and
>    `doc/api/sections/authentication.md` gains a "Get authorization from
>    Basecamp" section documenting the endpoint bc3 has served since #9471.
>    [`bc5-authorization-document-shape.md`](bc5-authorization-document-shape.md)
>    recorded both halves and closed `covered-outside-spec` before this repin
>    landed (the hand-written-lane consequences were absorbed by SDK #703 and
>    the fixture work in #709). What the repin adds is the pin containing the
>    commit, and a reasoning refresh on `spec/bc3-route-allowlist.yml`'s
>    `GET /authorization` entry, which was silent on the endpoint now being
>    documented.
> 2. BC3 **#12544** (`4547876f10b`) — **registered** as
>    [`subtasks-canonical-rename.md`](subtasks-canonical-rename.md),
>    `partial-coverage`. Step is renamed Subtask: the canonical route
>    declarations are `/subtasks` now and undocumented, every documented
>    `/steps` form is re-declared as a permanent alias pinned by bc3's
>    `route_aliases_test.rb`, and the wire is byte-identical (`json.steps`,
>    type `"Kanban::Step"`, 100%-renamed partial). All five modelled
>    operations keep working — four via the aliases, and
>    `RepositionCardStep`'s positions path never contained "steps", so it is
>    still canonical. Cross-referenced with
>    [`step-top-level.md`](step-top-level.md), which stays the record of how
>    the `/steps` spellings were absorbed.
> 3. `2b1225848c6` (direct to master) — `command_url` becomes **admin-only**
>    in chat-integration JSON, joining the already-gated `lines_url`, and
>    `chatbots.md` now documents both as admin-only plus the fact that
>    command payloads are unauthenticated. **Absorbed as documentation
>    only**: both Smithy `Chatbot` members were already un-required and now
>    carry a member doc noting admin-only visibility. No shape change, no
>    route delta.
> 4. BC3 #12279's `83eb14da06c` + `f127e52c1f5` — **tree-same twins of the
>    registered #12396 work; zero wire delta.** The sidebar branch carried
>    its own copies of the pings-sort-preference commits (as did its #12397
>    sub-branch: `817fdb9c50c`, `e0b54c027c1`, `54c96fc0de0`), including a
>    rename-migration route #12396 had abandoned. #12396 merged first,
>    before the previous pin, so these merged tree-same — which is also why
>    default log path filtering hides them. `sort_pings_first` was already
>    on the wire at the previous pin and stays registered-not-absorbed in
>    [`notifications-sort-pings-first.md`](notifications-sort-pings-first.md),
>    which gains an as-of note resolving the twin spellings to one contract.
> 5. BC3 **#12614** (`984d570baf8`) — the chat lines API's per-kind partial
>    dispatch gains JSON partials for the two announcement kinds a ping
>    records on rename or image change; a page of lines holding one
>    previously 500'd on `MissingTemplate`. No documented shape changes;
>    as-of note on
>    [`recordable-subtypes-doc.md`](recordable-subtypes-doc.md).
> 6. `a1be6ef581f` / `898939aae37` / `266e2257d63` (direct to master) —
>    Admin Pro Pack enforcement reaches modelled operations:
>    `UpdateTimesheetEntry` gains a reachable **403** it never had (under the
>    communication-editing limit), `DestroyTimesheetEntry`'s existing 403
>    re-keys from the archive-and-trash check to the same comment-style
>    rules, and `DestroyGaugeNeedle` gains one under the archive-and-trash
>    limit (as do unmodelled subtask and hill-chart-version destroys).
>    SPEC §6 already maps 403 generically, so
>    no per-operation Smithy errors — the same treatment as every other
>    permission gate. One **operator trap**: the shared guard's third branch
>    (recording carries descendants created by others) answers
>    `redirect_back_or_to` with no `respond_to`, so a JSON `DELETE` on such
>    a needle gets a **302** to an HTML URL rather than the 403 the other
>    two branches return.
> 7. BC3 **#12624** (`c059aaedd91`) — a cookie-authenticated request that
>    also carries an unverifiable bearer token no longer 500s populating the
>    OAuth cache; the clean **401** the authorizer already produced stands.
>    Recorded; 500-class behavior was never contract.
> 8. `74ac283a0e8` (direct to master) — template-copy requests missing their
>    template or destination now raise `RecordNotFound` (**404**) instead of
>    dying on a nil several layers down (**500**). Recorded; same reasoning.
> 9. BC3 **#12561** (`7785168089b`) — removes the iOS unreads endpoint,
>    which rendered under `app/views/internal`, not `app/views/api`; never
>    SDK surface. Recorded.
>
> **The wire-neutral remainder, accounted.** The 65 non-merge commits outside
> the two programs: the ten in items 1–3 and 5–9; two that touch controllers
> without touching API contract — `77f6cd669b3` guards the admin-only HTML
> person-merge confirmation flow, and `bd52b4f3b58` moves
> `GET /my/readings.json`'s native-app unreads cap into the query
> (`user_agent.native? && json_request?` only, so SDK callers never enter the
> branch; where it applies, a response may now return fewer than the cap —
> an undocumented sizing behavior, not a payload change); **six** gems-only
> commits (Trek and saas admin); **four** infra; and **43**
> web/model/test-only. Inside the programs, the sidebar's only API-surface
> contact is item 4's twins, and the calendar program's two
> `config/routes.rb` commits draw HTML agenda/display frames with no
> `app/views/api` template, so they are not API-renderable under bc3's own
> API-ness test.
>
> **The route table does not move at all.** `make bc3-routes` at the new pin
> writes 373 routes across 64 sections with only its `revision` line
> changing. #12646's new documentation normalizes onto the existing
> `GET /authorization` row — authentication.md was already that row's
> section and both evidence classes were already present — and the
> chatbots.md edit reorders example fields without adding a bullet or
> marker. That nil delta is the mechanical confirmation that items 2 and
> 4–9 are undocumented or documentation-free, and it is why item 2 is a
> registry entry rather than a Smithy change.
>
> The `bc3-four` compatibility pin does not move: advancing `master`
> re-verifies nothing about the `four` branch's API surface, and no
> re-verification of that branch happened here.
>
> Earlier pins, kept as the triage record. Each names the pin it was written
> against, in the past tense, because that is what it is — a range triaged
> once, at the repin that set its end. Only the sentence above is a claim
> about today.
>
> The pin was `b5d8c9df8d` (2026-08-05). The `7fe1c63ab3..b5d8c9df8d` range was **one commit**, and it was the companion
> to the absorption that repin carried: BC3 **#12565** (`b5d8c9df8d`) settled the
> input contract of the replacement endpoint #12555 shipped. It documents
> `notify` and `subscriptions` — `Subscribers#notify_param` defaults to
> `"custom"`, so an audience arrives either through `notify` naming a mode or
> through a bare `subscriptions` array, and the web form already relied on the
> second — and it removes `visible_to_clients` from the endpoint's reachable
> surface. That parameter never set the recording's visibility; it only widened
> `Subscribers`' audience, so a request could announce a client-invisible file to
> a project's clients. It also pins `""` as a description clear on both the
> replacement and `PUT /uploads/{id}.json`, which is the spelling the SDKs can
> express — five of six strip nulls structurally before the wire.
>
> Absorbed here as `CreateUploadVersion`, `UploadVersion` / `UploadVersionFile`
> and `StorageLimitError`, closing [`upload-new-version`](upload-new-version.md)
> and basecamp-sdk#649. The `POST /uploads/:id/versions` waivers the previous
> repin added to `spec/bc3-route-allowlist.yml` are **deleted** by that
> absorption, which is what the gate demands — a waiver matching nothing is a
> hard failure, not a shrug.
>
> One correction to the previous range's disposition, not a new finding: that
> triage recorded the 507 as needing "its own error shape", which it got
> (`StorageLimitError`, distinct from `ProjectLimitError` as required). What it
> did not record is that **neither** shape was classified correctly. SPEC §6 had
> no 507 step, so both fell through to `status >= 500` and surfaced as
> `api_error` with `retryable: true` — a plan limit reported as a transient
> server error. §6 now maps 507 to `limit_exceeded`, non-retryable, ahead of the
> 5xx catch-all, which fixes `ProjectLimitError` as well as the new shape.
>
> The `4e34dc83eb..7fe1c63ab3` range (71 commits) contains exactly **six**
> API-contract or API-documentation changes. Three touch `doc/api`; the other
> three change the wire, or a payload's backing field, without a documentation or
> route diff — which is why the sweep below is a classification of all 71 commits
> and not a glob over three paths. Only **two** of the three `doc/api` commits move
> the route table; the third is prose on an endpoint already documented.
>
> 1. BC3 **#12550** (`6f4781bbd4`) — **absorbed here.** `Projects::StatusController`
>    gains the `respond_to` it never had, so `active`/`archived`/`trashed` answer
>    a JSON request with `head :no_content` instead of a **302** to an HTML URL
>    that then returned **406**. The routes are not new; the contract is. Adds 32
>    lines to `doc/api/sections/projects.md` ("Archive a project", "Unarchive a
>    project"). Absorbed as `ArchiveProject` / `UnarchiveProject` and the shared
>    507 `ProjectLimitError`, recorded in
>    [`project-archive-unarchive.md`](project-archive-unarchive.md).
> 2. BC3 **#12555** (`a26c2e479f`) — **registered, absorbed elsewhere.** Upload
>    file replacement over the API: `POST /uploads/:id/versions.json` flat and
>    bucket-scoped, an index alongside it, new version fields including
>    `current`, and a **507 storage-limit** response. It closes the write side
>    [`upload-new-version.md`](upload-new-version.md) already describes, so its
>    registration is a status flip to `addressed-in-bc3-pr-12555`, not a new
>    brief. Absorption belongs to the `upload-versions-api` branch; that repin
>    only made the pin honest about the contract existing. Its routes therefore
>    arrive in `spec/bc3-routes.json` with no operation behind them and carry
>    `registry:` dispositions in `spec/bc3-route-allowlist.yml`.
> 3. BC3 **#12396** (`98eb24b22f`, `pings-sort-preference`) — **registered.** A
>    new `sort_pings_first` field on the notification *settings* payload
>    (`app/views/api/my/notifications/show.json.jbuilder`) plus an update action
>    to persist it, across `5561c42106`, `0e015cacd6` and four migrations
>    (`9cf4e17817`, `09ff95f35d`, `8c5af3956f`, `2480131f78`). bc3 documents none
>    of it, so no route-table row moves, but it renders under `app/views/api`, so
>    it is API surface. Registered as
>    [`notifications-sort-pings-first.md`](notifications-sort-pings-first.md),
>    `partial-coverage`.
> 4. BC3 **#9471** (`eac8b2b476`) — the modern OAuth 2.1 stack; **two halves,
>    two dispositions.** The discovery half **matches, no action**: bc3's new
>    `.well-known/oauth-authorization-server` (RFC 8414) and
>    `.well-known/oauth-protected-resource` (RFC 9728) are the exact paths the
>    SDK's hand-written discovery already fetches — `go/pkg/basecamp/oauth/discovery.go:24-25`,
>    `typescript/src/oauth/discovery.ts:400,514`, `python/src/basecamp/oauth/discovery.py:223,301`,
>    `ruby/lib/basecamp/oauth/discovery.rb:88`. The **authorization-document half
>    diverges** and is registered as
>    [`bc5-authorization-document-shape.md`](bc5-authorization-document-shape.md):
>    bc3 now draws `resource :authorization, only: :show` and renders it from
>    `app/views/api/authorizations/show.json.jbuilder`, whose shape is *not*
>    Launchpad's — `identity` carries only `id`, accounts carry `resource` but
>    neither `product` nor `app_href`, and `expires_at` is integer epoch seconds
>    rather than an ISO-8601 string. That document is reachable from the SDK
>    today: `Http#get_authorization_document` binds to the discovered BC5 issuer
>    and fetches `{issuer_origin}/authorization.json`. OAuth is deliberately
>    outside the OpenAPI spec, so this is a hand-written-lane gap, and
>    `spec/bc3-route-allowlist.yml`'s `GET /authorization` entry keeps its
>    `out_of_scope` disposition with its reasoning corrected — bc3 serves that
>    route itself now, and the claim that it does not was true when written.
> 5. `5c0e774b0d`, with `84569d7a92`, `d2f17f4489` and `25633614d3` — **no SDK
>    action, recorded because it is invisible to every path-based filter.**
>    Selecting completions through the denormalized column reroutes
>    `Recording.completed` to `completed_recently_first`
>    (`reorder("completions.created_at DESC")`) in
>    `app/controllers/concerns/everything/{cards,todos}/recordings.rb` and both
>    `app/controllers/my/assign{ings,ments}/completed_controller.rb`. Three
>    modelled operations reorder — `GetEverythingCompletedTodos`,
>    `GetEverythingCompletedCards`, `GetMyCompletedAssignments` — with no field
>    added, removed or retyped. None of the three declares an ordering in its
>    Smithy documentation, so there is no claim to correct. The paired
>    `my/assignings` and `my/assignments` controllers are **not** a rename: both
>    exist before and after the range, one line changed in each, so no modelled
>    path moved.
> 6. BC3 **#12566** (`7fe1c63ab3`) — **registered, no SDK action, and it ratifies
>    a decision that repin's absorption had already made.** Two added lines of prose in
>    `doc/api/sections/projects.md` stating that a project's `status` is read-only
>    on Update a project: passing one has no effect and still answers **200**, and
>    the note points callers at Archive, Unarchive and Trash instead. No route, no
>    bullet, no payload field — `spec/bc3-routes.json` regenerates at 373 routes
>    with only its `revision` line changing, which is the mechanical proof this is
>    documentation and not contract. It is registered rather than absorbed because
>    there is nothing to absorb: the SDK deliberately does **not** model `status`
>    as writable on `UpdateProject` (it is absent from `create_project_params`, so
>    bc3 silently drops it), and that omission was inferred from the permit list
>    when [`project-archive-unarchive.md`](project-archive-unarchive.md) was
>    written. #12566 makes it bc3's documented contract, so the SDK's silence is
>    now backed by upstream prose rather than by reading a controller.
>
> **The wire-neutral sweep, earned.** All 71 commits classified by touched path,
> residual bucket empty. Beyond the six above: `eac8b2b476` also adds
> `app_url_options` to `app/controllers/concerns/api_request.rb` — the API
> boundary concern itself — but it is a **20-line addition of a new private
> method** that computes web-host URL options for OAuth's HTML redirects;
> `api_request?` and `restrict_view_paths_to_api_root` are untouched, so what
> counts as an API route is unchanged. `9a162bca80` (#12475) registers `Gallery`
> as a recordable and adds it to the `dock_tools` group, so a gallery can appear
> in a project dock — additive only, because `DockItem.name` is modelled as a
> plain `String`, not an enum, and no `app/views/api/galleries` template exists,
> so galleries are not themselves API-renderable. `c76eceeec9` (#12529) makes
> OAuth consent proofs single-use, inside the OAuth stack's own controllers.
> `b41cae851b`, `625d3db323`, `8015fea19e` and `72bf7118ca` are model-layer
> changes behind no `app/views/api` template: a recordings account association,
> a timesheet-entry recording window, jump-menu project search by client name,
> and an email-pattern validation. `a4cc23cc48` casts a ChatChannel room number
> before SQL — Action Cable, not the §23 event-feed contract. `4536d5952a`
> keeps the cable server's `Origin` whitelist from arriving empty, a
> configuration guard on the cable host the §23 connector dials, with no change
> to the dial contract. The `gems/saas/**` commits are Trek admin UI
> (`057d7783e8`, `02eee3163e`, `03d591b5cd`, `f0c279424a`, `c4a95e15bf`), invite
> spam classification (`7fe9f8c139`, `1de902f11c`, `893af26c1c`, `ef40bb5fcc`)
> and a reverted Help Scout beacon signature (`b0eb1f4119`, `7bbca0ccea`,
> `683e021d1c`); `gems/saas`'s only appearance in an absorbed contract is
> #12550's own 507 test. The remaining commits are merges, iOS auto-scroll
> refactoring, web-only view changes, host maintenance and schema dumps.
>
> **The route table moved for two of the three `doc/api` commits.**
> `make bc3-routes` at that pin landed 373 routes across 64 sections and added
> exactly four rows: the two project-status PUTs, which the absorption models, and
> #12555's flat and bucket-scoped upload-version POSTs, which carried a `registry:`
> disposition. #12566 added none, because it documents an endpoint the table
> already carries — a `doc/api` diff is necessary but not sufficient for a route
> delta, and the distinction is the whole reason this paragraph reads the
> regenerated diff instead of asserting a row count. Nothing else moved, which is
> the mechanical confirmation that items 3–5 are undocumented and item 4's OAuth
> routes are not drawn under `doc/api`.
>
> The pin was `4e34dc83eb` (2026-08-03). The `4dd2926f8a..4e34dc83eb` range (2 commits) contained exactly **one**
> API-contract change, and it was the one that repin existed to absorb: BC3
> **#12521** (`4e34dc83eb`) made card and card table step updates
> presence-aware on the **JSON representation only** — `card_update_params`
> returned bare `card_params` under `request.format.json?` instead of merging
> over `{ due_on: nil }`, and `steps#update` changed the existing recordable
> rather than rebuilding it, replacing assignees only when an assignee key was
> present. An omitted field is now left unchanged; `"due_on": null` or `""`
> clears; `"assignee_ids": []` removes everyone; `title` became optional on
> step update. The HTML/turbo_stream leg kept `with_defaults(due_on: nil)`,
> so this was a representation-level fork invisible to web callers.
> It was the only commit in the range touching `doc/api` (2 lines in
> `card_table_cards.md`, 8 in `card_table_steps.md`) and it touched neither
> `config/routes.rb` nor any view under `app/views/api`, so
> `spec/bc3-routes.json` regenerated with no route delta — the change was to an
> omission rule on already-modelled endpoints, absorbed there as the
> `"due_on": ""` clear encoding across all six SDKs and the removal of the
> Cards preservation GET. That repin was **reactive, not preventive**: the
> commit reached production before the compatibility release, so every released
> SDK's explicit card due-date clear silently no-opped until the absorption
> landed. The other commit was a single HTML view change hiding the boost emoji
> picker on narrow columns; it touched no `doc/api`, no `config/routes.rb` and
> no API view.
>
> The pin was `4dd2926f8a` (2026-08-03). The `2c0dafba13..4dd2926f8a` range (6 commits) contained exactly **one**
> API-contract change, and it was the one that repin absorbed: BC3
> **#12502** (`4dd2926f8a`) preserved a schedule entry's join link and
> highlight across a sparse update — `PRESERVED_ON_OMISSION = %i[ url
> highlighted ]` in `Schedules::EntriesController`, plus the other half, which
> was *emitting* them: `highlighted`, and the join link as **`join_url`** under
> a non-colliding key, in `api/schedules/entries/_entry.json.jbuilder` and
> `api/schedules/entries/occurrences/show.json.jbuilder`. It was the only commit
> in the range touching `doc/api` (8 added lines in
> `schedule_entries.md`) or `config/routes.rb` (none), so `spec/bc3-routes.json`
> regenerated with no route delta — the change was to a payload and an omission
> rule on an already-modelled endpoint, absorbed here as `ReplaceScheduleEntry`
> and its `preservedOnOmission` carve-out. The remaining five commits were one
> mail-infrastructure map, two CSS-only, one account-calendar authorization
> reassessment on profile change, and one authentication-cache crash fix; none
> touched `doc/api`, `config/routes.rb`, or an API view.
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
