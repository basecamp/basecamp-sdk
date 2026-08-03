---
gap: external-links-doors
status: partial-coverage
detected: 2026-07-22
sdk_demand: low
bc3_pr: 12375
smithy_refs:
  - "Door in the documented recording type string (spec/basecamp.smithy:6223)"
  - "Recording.position/description/service (spec/basecamp.smithy:6282-6293)"
  - "DoorService (spec/basecamp.smithy:6309)"
bc3_refs:
  routes:
    - GET /:account_id/projects/recordings.json?type=Door
    - POST /:account_id/buckets/:bucket_id/dock/doors.json
    - GET /:account_id/dock/tools/:id.json
    - PUT /:account_id/dock/tools/:id.json
    - DELETE /:account_id/dock/tools/:id.json
  controllers:
    - app/controllers/docks/doors_controller.rb
    - app/controllers/docks/tools_controller.rb
    - app/controllers/recordings_controller.rb
  related_existing_api:
    - ListRecordings
    - GetTool
    - UpdateTool
    - DeleteTool
---

# External links (doors) API surface

## What's missing

BC3 **#12375** ("Add API documentation for External links (doors)", merged
`60a7f598`, 2026-07-22) newly documents the external-link (historically "door")
resource — a dock tool that points to an outside URL (Figma, Dropbox, GitHub,
etc.). New `doc/api/sections/external_links.md` documents:

- **List** — `GET /:account_id/projects/recordings.json?type=Door` (the canonical
  enumeration and the only endpoint returning the full door shape: `url`,
  `service` struct, `description`). Accepts the generic recordings `bucket`,
  `status`, `sort`, `direction` params.
- **Create** — `POST /:account_id/buckets/:bucket_id/dock/doors.json`. Bespoke path;
  returns **302** (redirect), no created-resource JSON body / no ID in the
  response.
- **Get / Rename / Trash** — via the generic dock-tool operations at
  `/:account_id/dock/tools/:id.json`: `GET` (get), `PUT` with a `title` (rename), and
  `DELETE` (trash — soft-deletes the door, `status` → `"deleted"`). The legacy
  `DELETE /:account_id/buckets/:bucket_id/dock/doors/:id.json` is an alias.
- **No JSON update path** for `url`/`service`/`description`. A prior revision of
  this entry said "a `PUT` to the tool returns 406" — that is wrong, and it
  matters because it maligns an operation the SDK ships. `PUT /dock/tools/:id.json`
  is the **rename** and returns `200 OK`; it just doesn't reach these three
  fields. It is the door-scoped `PUT /buckets/:b/dock/doors/:id.json` that
  returns **406**, and it does so *after* applying the change —
  `Docks::DoorsController#update` calls `@recording.update!` before a
  `respond_to` block that offers only `turbo_stream` and `html`, so content
  negotiation fails on a record that has already been written. Callers change
  these fields in the web app, or trash the link and create a new one.

The same PR adds `Door` to the documented `type` enum for
`GET /:account_id/projects/recordings.json` in `recordings.md`.

## Relationship to existing entries

This **supersedes the "Door is string-only" classification** in
[[recordable-subtypes-doc]], which stated Door "appears only as a string `type`
value" with no create/list surface. That is now stale: #12375 documents a
door-specific list endpoint (full shape), a bespoke create path, and dock-tool
get/rename/trash. Absorption should be tracked here, not there. Door as a
`RecordingType` enum member is also a doc-string delta on the existing
`ListRecordings` output.

## Why it matters

SDK consumers cannot currently enumerate a project's external links (the shape
is only returned by the `type=Door` recordings query) or create one through a
typed operation. Demand is low — external links are a legacy dock surface — but
the contract is now documented and must be tracked to keep detection honest.

## Suggested API shape

- A `type=Door`-scoped recordings list (or dedicated `ListExternalLinks`)
  returning the full door shape: `url`, `service` struct, `description`.
- A create operation that honors the 302/no-body contract (returns no
  resource JSON).
- Reuse the existing dock-tool GetTool/UpdateTool/DeleteTool operations for
  get/rename/trash.
- `Door` added to the `RecordingType` documented enum.

## Implementation notes for BC3

- Already merged in BC3 #12375 (docs). Endpoint URLs keep the legacy `doors`
  resource name.
- External links are deliberately omitted from a project's `dock` array, so
  the "discover via dock" advice does not surface them — the `type=Door` list
  is the canonical enumeration.
- There is no JSON update path for `url`/`service`/`description`. The 406 comes
  from the door-scoped `PUT`, not from `UpdateTool` — see the corrected bullet
  under "What's missing".

## SDK absorption plan when this lands

**Partially absorbed** (basecamp-sdk PR-4 of the post-#401 follow-up program).
A pre-implementation spike resolved the three blockers and scoped this PR to the
cleanly-absorbable surface; the redirect-dependent create and the multipart
image are recorded as residual gaps below.

**Absorbed in this PR:**

- The door **list** is the existing `type=Door`-scoped `ListRecordings` query —
  **not** a new operation. `ListRecordings` already carries a `type` query and a
  `Recording.url`; the full door shape is reached by adding `Door` to the
  `RecordingType` documented enum and adding optional `position`, `description`,
  and a `service` struct (`DoorService`) to the shared `Recording` projection.
  A runtime decode test proves the door shape (external `url` + `service` +
  `description`) decodes through the projection.
- Get/rename/trash reuse the existing `GetTool` / `UpdateTool` / `DeleteTool`
  operations — no new work.
- A `type=Door` recordings canary entry validates statically (live dormant).

**Residual gaps (why this entry is `partial-coverage`, not
`absorbed-in-sdk`):**

- **Create (`POST /buckets/:b/dock/doors.json`) — deferred.** The endpoint
  returns **302 with an empty body** (no ID), redirecting cross-origin to the
  web project page. The spike found the six SDK transports handle this
  inconsistently: Go, TypeScript (fetch default), Kotlin (Ktor default), and
  Swift (URLSession default) **follow** the redirect (landing on the web page,
  after cross-origin auth stripping); Python (`follow_redirects=False`) and Ruby
  (Faraday, no follow middleware) do **not** follow but then face empty-body
  decode, and each classifies a bare 302 differently. Modeling "return only the
  302/empty response" would require coordinated per-operation redirect
  suppression + 302-as-success handling across all six generated transports — a
  cross-cutting transport change, not an additive absorption. Deferred to a
  dedicated PR.
- **`image` thumbnail (multipart) — unmodeled.** `door[image]` is
  `multipart/form-data` only; not modeled. Independent of the create-transport
  question, this alone keeps the entry from honestly flipping to
  `absorbed-in-sdk`.
- **Create-and-discover composite (SPEC §18) — deferred.** It depends on a
  shippable `CreateExternalLink`, so it is out of scope until create lands.
- **No JSON update path** for `url`/`service`/`description`: never modeled, by
  contract. The door-scoped `PUT` returns 406 (see above).

### Disposition of the two allowlisted door routes — decided 2026-08-03

[`spec/bc3-route-allowlist.yml`](../bc3-route-allowlist.yml) carries two
`bc3_routes_not_modeled` entries pointing here. `partial-coverage` is the
honest *status* — list, get, rename, and trash are absorbed; create is not — but
it was being read as an open question about these two routes. It is not. They
have different answers, and both are settled below. Verified against `bc3` at
the revision pinned in [`spec/api-provenance.json`](../api-provenance.json).

**What `routes.rb` draws.** `config/routes.rb` has a bare `resources :doors`
inside `resource :dock` inside the bucket scope — no `only:`, no `except:`, so
all seven default actions exist at `/buckets/:id/dock/doors`. Drawn is not the
same as API-reachable: `api_request?` keys on `request.host == BC3.api_host` and
`restrict_view_paths_to_api_root` then limits view paths to `app/views/api`. The
only door template under that root is
`app/views/api/doors/_door.json.jbuilder` — a **partial**, rendered by the
recordings projection for the `type=Door` list. There is no
`app/views/api/doors/index.json.jbuilder` and no `show`, so `Docks::DoorsController#index`
(an empty action) is drawn but **not renderable** under the API host. It is
absent from the allowlist only because
[`spec/bc3-routes.json`](../bc3-routes.json) is extracted from
`doc/api/sections/*.md` and the doc does not bullet it.

**`test/api` agrees.** There is no `test/api` coverage of the door-scoped routes
at all. Doors are exercised only through the flat dock-tool routes
(`test/api/docks/tools_controller_api_test.rb` — show, rename, and trash a door
recording via `docks_tool_url`) and through the `type=Door` recordings query
(`test/api/projects/recordings_controller_api_test.rb`, which asserts the full
door shape: external `url`, `service.code`, `description`). Every door surface
bc3 tests as an API is a surface this SDK already models. With `routes.rb` and
`test/api` agreeing, `doc/api` is safe here as negative evidence.

#### `GET /buckets/:id/dock/doors/:id` — permanently out of scope

Not deferred. **Never modelable**, and the allowlist disposition is changed from
`registry:` to `out_of_scope:` to say so at the point of use.

`Docks::DoorsController#show` is one line — `redirect_to @recording.door.url`.
`doc/api/sections/external_links.md` states the same thing in the affirmative:
"The legacy bucket-scoped route is a redirector rather than a JSON resource:
`GET /buckets/1/dock/doors/2.json` responds with `302 Found` and a `Location`
header pointing at the external link's outside `url`." It is template-less, so
it technically survives the view-path restriction — which is exactly why the
"drawn *and* renderable" test alone does not settle it. It renders nothing
because it returns nothing: there is no representation, at any content type,
ever.

Modeling it would mean shipping an SDK operation whose success value is a
cross-origin redirect to a **user-supplied third-party URL**. The redirect
target is attacker-influenced by construction — anyone who can create an
external link chooses it. The transport spike recorded above found that four of
the six SDK transports (Go, TypeScript, Kotlin, Swift) follow redirects by
default, so the natural implementation issues an outbound request to that
address. Cross-origin auth stripping is what keeps the Bearer token off it, and
that is a property of each HTTP stack's defaults rather than something this SDK
asserts. An SDK should not turn a credentialed client into a general-purpose
fetcher of arbitrary URLs, and there is no upside to weigh against it: the
external `url` is already delivered as a plain field by the modeled `type=Door`
`ListRecordings` query. This does not become absorbable if the create question
is solved later.

#### `POST /buckets/:id/dock/doors` — real route, keeps the registry disposition

This one **is** a real API route: drawn, template-less by redirect rather than by
omission, and documented with a JSON request body, a parameter table, and a cURL
example. It stays dispositioned to this entry, and the reason it is unmodeled is
cost, not principle — spelled out under "Residual gaps" above and unchanged by
this reassessment. Two independent blockers, either of which alone is
disqualifying:

1. The 302/empty-body/no-ID response needs coordinated per-operation redirect
   suppression plus 302-as-success handling across all six generated transports.
   That is a cross-cutting transport change, not an additive absorption.
2. `door[image]` is `multipart/form-data` only and is unmodeled regardless.

`sdk_demand` is `low` and external links are a legacy dock surface, so nothing
here argues for paying that cost soon. If it is ever paid, the create-and-discover
composite (SPEC §18) becomes available in the same PR, since discovery of the new
ID already works through `ListRecordings`.

**Next reader:** the open item on this entry is create, and only create. `GET`
on the door-scoped route is closed. Do not re-litigate the door redirector.

## Compatibility

Mostly additive, with one deliberate optionality **correction** — not purely
additive. No new operation and no change to existing modeled operations.

- **Additive:** the `Door` recording type value plus the optional
  `position`/`description`/`service` fields on the shared `Recording` output.
- **Optionality correction (`Recording.parent`):** this entry relaxes
  `Recording.parent` from required to optional. That changes the generated
  public types (Swift/Kotlin/TypeScript/Python surface `parent` as optional), so
  it is technically source-affecting for consumers that assumed `parent` is
  always present. It is nonetheless a **fix of a pre-existing contract defect,
  not a gratuitous break**: BC3's shared recording projection
  (`app/views/api/recordings/_recording.json.jbuilder`) emits `parent` only
  `if !recording.docked? && recording.parent`, so it is **omitted for every
  docked recording** — message boards, to-do sets, schedules, campfires,
  questionnaires, and doors (all dock items, `docked? == parent&.dock?`) — and
  for any parentless recording. The prior `@required` therefore over-asserted a
  field the API has always emitted conditionally, and strict decoders
  (Swift/Kotlin) would already have failed to decode any dock item. Door (a
  docked recording) only made the latent defect unavoidable. Because it changes
  a public type, it should still be called out in the release notes as a
  compatibility-affecting correction.
