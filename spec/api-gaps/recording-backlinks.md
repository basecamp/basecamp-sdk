---
gap: recording-backlinks
status: addressed-in-bc3-pr-13121
detected: 2026-09-15
sdk_demand: medium
bc3_pr: 13121
bc3_refs:
  introduced_in: "Serve a recording's backlinks as JSON to API clients (BC3 #13121, b6ab4d76b1b)"
  routes:
    - "GET /:account_id/recordings/:recording_id/backlinks.json (canonical)"
    - "GET /:account_id/buckets/:bucket_id/recordings/:recording_id/backlinks.json (legacy alias)"
  controllers:
    - app/controllers/recordings/backlinks_controller.rb
  related_existing_api:
    - Recording
    - ListEvents
---

# Backlinks — the recordings that reference a recording

## What's missing

A backlink records that one recording references another in its rich text.
The machinery — `incoming_backlinks`, the visibility scope, the index action —
predates BC5, but it only ever rendered for the web host. BC3 #13121 adds
`app/views/api/recordings/backlinks/index.json.jbuilder`, mirroring the events
sub-resource, and documents it in a new `doc/api/sections/backlinks.md`:
`GET /recordings/:id/backlinks.json` returns the most recent 20 recordings
that reference the given one, newest first, each rendered through the shared
`recordings/_recording` partial. The list is unpaginated and visibility-scoped
(`with_linking_recordings_visible_to Current.person`), so two people can see
different lists for the same recording.

The SDK models no operation for it.

## Why it matters

Backlinks are the "references to this" panel in the product, and the only
inbound-reference signal the API exposes; events and comments are outbound.
Medium demand: useful for agents assembling context around a recording, not
blocking for any known consumer.

## Suggested API shape

`ListBacklinks`: `GET /{accountId}/recordings/{recordingId}/backlinks.json`
returning `RecordingList` (the referencing recordings, not the backlinks
themselves), `@readonly`, the standard 3-attempt retry policy, **no**
pagination trait — the controller hard-limits to 20 and emits no Link header
— and errors `NotFoundError`, `UnauthorizedError`, `ForbiddenError`,
`InternalServerError`. Tag it `Automation` beside `ListEvents` so it lands on
the `Recordings` service. Model the flat form only.

## Implementation notes for BC3

Nothing further: template, docs, preloads and API coverage
(`test/api/recordings/backlinks_controller_api_test.rb`) landed in #13121.
The documented 20-item cap is a controller `limit(20)`, not a page size; if
bc3 ever paginates it the SDK operation gains the pagination trait then.

## SDK absorption plan when this lands

A spec PR adds the operation and tag, regenerates, and adds the per-SDK
happy-path and 404 tests. The response elements are the generic `Recording`
projection the manifest already covers, so a fixture is optional; Go gains
`RecordingsService.ListBacklinks` returning `[]Recording` without a
`ListOptions` since nothing paginates.
