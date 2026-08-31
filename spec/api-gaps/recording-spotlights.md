---
gap: recording-spotlights
status: absorbed-in-sdk
detected: 2026-08-24
sdk_demand: medium
bc3_pr: 12860
smithy_refs:
  - SpotlightRecording
  - UnspotlightRecording
bc3_refs:
  introduced_in: "Expose spotlighting as recording state (06edca6f0b)"
  routes:
    - POST /:account_id/recordings/:recording_id/spotlight.json
    - DELETE /:account_id/recordings/:recording_id/spotlight.json
  controllers:
    - app/controllers/recordings/spotlights_controller.rb
  related_existing_api:
    - Recording
    - ArchiveRecording
    - TrashRecording
---

# Recording spotlights

## What's missing

BC3 exposes two canonical flat recording actions: `POST` spotlights a recording
and returns its ordinary recording representation with `201 Created`; `DELETE`
removes the spotlight and returns `204 No Content`. The SDK had neither
operation.

A spotlight is presentation state on a recording, not a separate API resource.
The project's `dock` is unchanged and recording JSON gains no `spotlighted`
member. The documented bucket-scoped spellings are compatibility aliases, not
the canonical routes for new integrations.

## Why it matters

An integration can now feature or unfeature an eligible recording on its project
or template home page. Without these operations it cannot perform either state
transition through any generated SDK method.

The public API still has no way to enumerate or reorder spotlights. BC3 has a
web-only position route, but it has no public JSON documentation or API test and
is deliberately outside this absorption.

## Suggested API shape

Add both operations to the existing Recordings service:

- `SpotlightRecording`: `POST /recordings/{recordingId}/spotlight.json`, no
  request body, `201`, returning `Recording`. Repeating the request returns the
  same state and remains `201`, so the POST is naturally idempotent.
- `UnspotlightRecording`: `DELETE` on the same path, no request body, `204`.
  Removing an absent spotlight also returns `204`, so the delete is naturally
  idempotent.

Create includes the documented `403` permission and `422` eligibility failures,
plus the standard recording lookup/authentication/rate/server failures. Delete
uses the standard recording failures.

## Implementation notes for BC3

BC3 commit `06edca6f0b` adds the public documentation, JSON response branch and
API tests. `create.json.jbuilder` renders the existing recording partial;
destroy returns no content. The controller requires project-edit permission.
Archived, trashed and non-spotlightable container recordings are rejected.

The separate `Recordings::Spotlights::PositionsController` remains an internal
HTML/Turbo interaction and is not a supported public JSON API.

## SDK absorption plan when this lands

Absorbed here as `recordings.spotlight` and `recordings.unspotlight` in all six
SDKs. Smithy owns both wire operations; generated service layers call the
canonical flat routes, and Go's hand-written service wrapper calls only those
generated methods. Cross-SDK tests pin the paths, response decoding, errors and
the retry behavior of the naturally idempotent POST.
