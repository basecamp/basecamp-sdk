---
gap: folders-api
status: addressed-in-bc3-pr-12384
bc3_pr: 12384
sdk_demand: medium
detected: 2026-07-31
bc3_refs:
  introduced_in: master
  routes:
    - GET /:account_id/stacks.json
    - POST /:account_id/stacks.json
    - GET /:account_id/stacks/:id.json
    - PUT /:account_id/stacks/:id.json
    - DELETE /:account_id/stacks/:id.json
  controllers:
    - app/controllers/stacks_controller.rb
  related_existing_api:
    - ListProjects (the expanded `projects` payload is the project projection)
    - GetProject (a folder groups projects; membership is per-user)
---

# Folders API

> **Supersedes [`stack-doc-and-smithy`](stack-doc-and-smithy.md)**, which
> classified Stacks as `confirmed-not-api-resource` ("no JSON contract to
> model"). That classification was correct when written and is now false: BC3
> **#12384** ("Add public API for personal project folders", `dc6cd10714`)
> shipped the contract to `master` and it is live in production, with public
> docs live at `basecamp/bc-api` (#420, `401c8ebcc9`). The superseded entry
> named this successor path itself and stays in place as the historical record.

## What's missing

SDK registration and absorption. The server contract exists and is documented;
the SDK models no `FoldersService` and the canary does not expect folder
endpoints.

**Folders** group [projects][projects] together on a person's home screen. They
are **per-user**: each person arranges their own home, and filing a project into
a folder for yourself does not change how anyone else sees it. Membership,
ordering, and appearance all belong to the authenticated user — there is no
shared/account-wide folder concept to model.

Five operations, full CRUD on a flat top-level collection (not bucket-scoped):

- **`GET /stacks.json`** — the authenticated user's folders, in home-screen
  order. Returns a **top-level array**, no pagination envelope.
- **`GET /stacks/{id}.json`** — one folder, with its grouped projects expanded.
- **`POST /stacks.json`** — creates a folder for the authenticated user and
  files the given projects into it. **`201 Created`**.
- **`PUT /stacks/{id}.json`** — renames. `name` is the only writable attribute.
  **`200 OK`**.
- **`DELETE /stacks/{id}.json`** — **`204 No Content`**.

`doc/api/sections/folders.md` on BC3 `master` is the contract of record,
mirrored live at [`basecamp/bc-api`](https://github.com/basecamp/bc-api).

## Why it matters

The home screen is the first surface a Basecamp user sees, and folders are how
they organize it. Without this, an integration can list a user's projects but
cannot read or reproduce the grouping the user actually looks at, and cannot
build a "file this project away" affordance. There is no client-side workaround:
the grouping is per-user state stored server-side and is not derivable from the
project projection.

It also matters as a **correction**: the registry currently tells a reader that
Folders are web-only. Any future detector run or contributor consulting
`stack-doc-and-smithy` gets a wrong answer until this entry lands.

## Suggested API shape

A new `FoldersService`, tag `Folders`. Proposed operation names use the product
noun (`Folder`), while paths and the wire discriminator keep `stack`:

| Operation | Method + path | Success | Output |
|---|---|---|---|
| `ListFolders` | `GET /{accountId}/stacks.json` | 200 | top-level **array** of `Folder` |
| `GetFolder` | `GET /{accountId}/stacks/{folderId}.json` | 200 | one `FolderWithProjects` |
| `CreateFolder` | `POST /{accountId}/stacks.json` | **201** | one `FolderWithProjects` |
| `UpdateFolder` | `PUT /{accountId}/stacks/{folderId}.json` | 200 | one `FolderWithProjects` |
| `DeleteFolder` | `DELETE /{accountId}/stacks/{folderId}.json` | **204** | no body |

### Response shapes differ per operation

This is the single most important modelling fact here — it drives the Smithy
output structures, and a generator that assumes one shape for all five will emit
a `projects` field that is never populated on list:

| Op | Returns |
|---|---|
| List | top-level **array** of base `Folder` objects, each carrying `bucket_ids`, and **no** `projects` |
| Get / Create / Update | **one** folder **plus** the expanded `projects` array |
| Delete | no body (204) |

Base `Folder` fields, as documented:

```
id, name, type ("Stack"), created_at, updated_at, bucket_ids,
is_emoji_only_name, star_url, gauges_url, color, image_url, url
```

`FolderWithProjects` is that shape plus `projects` (the project projection).

**Model these as two distinct structures.** A single structure with an optional
`projects` member is not an acceptable substitute: `ListFoldersOutput` would
reference that structure, so every generated list-item type would declare a
`projects` field the list response never populates — exactly the outcome this
section exists to prevent. Two shapes make the difference in the static types,
where a consumer can see it.

### Modelling traps

Each of these will bite a generator or a consumer if it is not modelled
deliberately:

- **The wire `type` is `Stack`, not `Folder`.** The product was renamed; the
  payload was not. Any discriminator, polymorphic union, or `type`-keyed
  dispatch must match on `"Stack"`. The superseded brief already warns about
  this and it is repeated here on purpose — it is the mistake most likely to be
  made by someone reading only the product docs.
- **`project_ids` input maps to two different output fields.** The create input
  takes `project_ids`; the response reports the same ids back as **`bucket_ids`**
  on the base object *and* expands them as the **`projects`** array. Three names,
  one relationship. Do not model `project_ids` as a round-tripping field.
- **`image_url` is read-only.** There is no image create or update in v1; the
  field appears in every response and is not writable through any documented
  parameter.
- **Delete unpins, it does not trash.** `DELETE` removes the folder and unpins
  its projects from the person's home screen. The projects are not deleted, and
  they are not moved back out onto the home screen — they simply stop appearing
  there until pinned again. A wrapper doc comment saying "deletes the projects"
  would be actively wrong.
- **Create admits all-access projects the person isn't a member of.** Filing an
  all-access project the user is not yet on **grants** them access (recording a
  `joined` event, non-subscribing). This is a documented side effect of create,
  not a no-op.
- **Create preflights every id; an unreachable one is a zero-write 404.** If any
  `project_ids` entry is archived, trashed, or an invitation-only project the
  user is not on, the whole request returns `404 Not Found` and **nothing is
  created**. There is no partial success and no per-id error detail — model it
  as `NotFoundError` on `CreateFolder`, not as a validation error.
- **Accounts without the pinning entitlement get a 404 from create.**
  *Implementation-observed, not documented contract.* The code behaves this way
  deliberately — the pin is skipped, so `Stack#pin`'s `find_sole_by` raises
  inside the transaction and rolls the create back — but neither the public docs
  nor the merged tests promise it. Treat it as a known behavior to expect in
  the wild, not as a contract to assert in a conformance case.

### Placement in the spec

Operations append to the `service Basecamp` operations list in
`spec/basecamp.smithy` as flat `operation` + `Input`/`Output` triples. There are
**no Smithy `resource` shapes** in this spec; `Projects` (~L425-500) is the
model to imitate, including how `ListProjectsOutput` declares a bare list member
for a top-level-array response.

## Implementation notes for BC3

Shipped — nothing pending server-side. `app/controllers/stacks_controller.rb`
serves all five actions; the JSON views are the standalone folder shape plus the
expanded `projects` on the singular actions. `doc/api/sections/folders.md` is
the contract of record and the public mirror is already live.

Two things the SDK should **not** ask BC3 for as part of absorption:

- **Image upload/update.** Out of scope for v1 by design, not an oversight.
- **A documented entitlement error.** The 404-on-unentitled-account behavior
  above is real but unpromised. If the SDK wants to depend on it, that is a
  separate ask for BC3 to document and test it, filed as its own brief.

## SDK absorption plan when this lands

The eight requirements in [`AGENTS.md`](../../AGENTS.md) §"SDK Change
Completeness Bar" → "Every New Operation Requires", applied to all five
operations:

1. **Smithy spec** — five operations plus their `Input`/`Output` structures and
   error lists, in `spec/basecamp.smithy`. Two output shapes, per the table
   above. `DeleteFolder` has no output body.
2. **Tag** — `apply ListFolders @tags(["Folders"])` and the other four, in
   `spec/overlays/tags.smithy`.
3. **Generator mapping** — *only if generation actually requires it.* Expect the
   `Folders` tag to resolve to `FoldersService` via the **default fallback** in
   every generator, as the `Bookmarks` tag did (zero service-group overrides).
   Note what Bookmarks *did* need: an element-name entry for its list operation
   in three generator tables (`typescript/scripts/generate-services.ts:311`,
   `kotlin/…/generator/Config.kt:282`, `swift/…/BasecampGenerator/Utilities.swift:153`),
   because `ListMyBookmarks` does not singularize to the payload element name.
   `ListFolders` should singularize cleanly to `folder`; verify by generating,
   and add an override only if it does not.
4. **Client wiring** — import, type declaration, `defineService`/`service()`
   call, re-export from `index.ts`.
5. **TypeScript test** — `typescript/tests/services/folders.test.ts`, happy path
   plus an error case.
6. **Ruby test** — `ruby/test/basecamp/services/folders_service_test.rb`, same
   coverage.
7. **Python test** — `python/tests/services/test_folders_service.py`, same
   coverage.
8. **Regeneration** — all generated artifacts freshly regenerated, not stale;
   `make go-check-drift`, `make kt-check-drift`, `make py-check-drift` clean.

Worth covering in those tests specifically: that a list response deserializes
without `projects`, and that a get/create/update response carries it.

**Beyond the bar**, two items the shipped `BookmarksService` precedent shows are
expected in practice, though the bar does not name them:

- A **Go wrapper** — `go/pkg/basecamp/folders.go`, alongside an
  `AccountClient.Folders()` accessor, following `go/pkg/basecamp/bookmarks.go`.
  The standing Go rule (never call the raw generated client from a consumer)
  makes the wrapper the real public Go API.
- **`conformance/` path entries** — `conformance/tests/paths.json` plus dispatch
  in the runners.

**Not** required: registration in `spec/fixtures/manifest.yaml`. Bookmarks added
no shared fixture, and that manifest deliberately covers a limited schema set.

[projects]: https://github.com/basecamp/bc-api/blob/master/sections/projects.md
