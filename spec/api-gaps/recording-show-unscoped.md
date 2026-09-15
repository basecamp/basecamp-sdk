---
gap: recording-show-unscoped
status: addressed-in-bc3-pr-10158
detected: 2026-09-15
sdk_demand: medium
bc3_pr: 10158
bc3_refs:
  introduced_in: "Add unscoped recording show route (BC3 #10158, 87a3d3603c1)"
  routes:
    - "GET /:account_id/recordings/:id.json (canonical)"
    - "GET /:account_id/buckets/:bucket_id/recordings/:id.json (legacy alias)"
  controllers:
    - app/controllers/recordings_controller.rb
  related_existing_api:
    - Recording
    - ListRecordings
    - TrashRecording
    - ArchiveRecording
    - UnarchiveRecording
---

# Get a recording — the generic projection by id

## What's missing

BC3 #10158 draws `GET /recordings/:id.json` as a canonical flat route, keeps
the bucket-scoped spelling as a documented alias, hides deleted recordings from
both, and — the half that matters here — adds
`app/views/api/recordings/show.json.jbuilder`, so the endpoint renders under
bc3's API view-path restriction. `doc/api/sections/recordings.md` documents
it as "Get a recording": the generic JSON representation of a recording whose
type you do not yet know, carrying `type` so a caller can follow up with the
type-specific endpoint.

The SDK models no operation for it. It used to: `GetRecording` shipped
against the bucket-scoped spelling and 404'd in every consumer, because no
`app/views/api` template existed for the action — the "GetRecording trap"
`spec/bc3-route-allowlist.yml` describes — and it was removed in the v0.13.0
breaking window (#584). #10158 removes the reason it was removed.

## Why it matters

A recording id arrives without its type all the time — webhook payloads,
notification `recording` envelopes, backlinks, search hits, a pasted URL — and
the SDKs have no single call that resolves one. Consumers work around it by
guessing the type and trying the type-specific `Get`, or by decoding the
generic `Recording` projection from a listing they already hold. The CLI's
`basecamp show <id>` reaches through the raw path today for exactly this
reason.

## Suggested API shape

`GetRecording`: `GET /{accountId}/recordings/{recordingId}` returning the
existing `Recording` structure, `@readonly`, the standard 3-attempt retry
policy, errors `NotFoundError` (a deleted or inaccessible recording),
`UnauthorizedError`, `ForbiddenError`, `InternalServerError`. Tag it
`Automation` next to the other recording actions so it lands on the
`Recordings` service every SDK already exposes. Model only the flat form; the
bucket-scoped alias is the same action.

## Implementation notes for BC3

Nothing further: routes, template, docs and API coverage
(`test/api/recordings_controller_api_test.rb`) all landed in #10158. The
show template renders the recordable's own partial plus `bookmark_url` and
`subscription_url`, so the payload is the type-specific shape, not the base
recording envelope — the `Recording` structure already models that polymorphic
projection.

## SDK absorption plan when this lands

A spec PR adds the operation and tag, wires it on the `Recordings` service in
the four hand-wired clients, and adds the per-SDK happy-path and 404 tests
plus a `spec/fixtures/recordings/get.json` operation entry in the manifest
(the pointer entry for `Recording` already exists). Go regains
`RecordingsService.Get`. The CLI's `show` command can then drop its raw-path
fallback for the generic lookup.
