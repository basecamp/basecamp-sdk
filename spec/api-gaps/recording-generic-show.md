---
gap: recording-generic-show
status: addressed-in-bc3-pr-10158
detected: 2026-09-15
sdk_demand: medium
bc3_pr: 10158
bc3_refs:
  introduced_in: "Add unscoped recording show route (BC3 #10158, merged 87a3d3603c1)"
  routes:
    - "GET /:account_id/recordings/:id.json (documented, doc/api/sections/recordings.md)"
    - "GET /:account_id/buckets/:bucket_id/recordings/:id.json (documented alias)"
  sections:
    - doc/api/sections/recordings.md
  related_existing_api:
    - ListRecordings
---

# A generic recording show route, at the spelling the SDK removed as a 404

## What's missing

BC3 **#10158** (`87a3d3603c1`) documents `GET /recordings/2.json`, which returns
the generic representation of a recording without the caller knowing its type
first. `doc/api/sections/recordings.md` states the use case directly: you hold a
recording ID, you read `type` off the response, and you follow up with the
type-specific endpoint if you need more. The bucket-scoped
`GET /buckets/1/recordings/2.json` is documented as an alias of the same action.

The SDK models no operation at either spelling.

What makes this entry worth reading rather than filing: **the SDK used to
declare this route and deleted it.** `GetRecording` was removed in the v0.13.0
breaking window as issue #584, one of the confirmed live 404s that
`spec/bc3-route-allowlist.yml` still cites twice as the cautionary case — a
route that was *drawn* in `config/routes.rb` but not renderable under
`restrict_view_paths_to_api_root`, so it answered 404 to every caller. That
diagnosis was correct when it was made. #10158 changes the fact it rested on.
Re-adding the operation is therefore a deliberate re-entry, not a revert, and
whoever does it should confirm the view half now exists rather than trusting
this paragraph.

## Why it matters

Medium demand. The endpoint removes a round trip from a real pattern: today a
client holding a bare recording ID — from a webhook payload, an
`app_url`, a bookmark, a backlink entry (see [[recording-backlinks]], whose
whole payload is a list of recordings you may want to resolve) — has to guess
the type or fan out across type-specific endpoints. It is also the natural
companion to the event and timeline surfaces, which hand out recording IDs
freely.

Leaving it unmodeled is defensible while nobody has asked for it. Leaving it
unmodeled *silently*, given the SDK once shipped this exact path as a defect,
is how the next person re-derives the #584 analysis from scratch and reaches
the stale conclusion.

## Suggested API shape

A single operation, at the flat spelling bc3 documents first:

```
GetRecording: GET /{accountId}/recordings/{recordingId}.json -> Recording
```

The response is the generic recording envelope the SDK already models for
`ListRecordings` — `id`, `status`, `visible_to_clients`, `title`, `type`,
`url`, `app_url`, `bookmark_url`, `parent`, `bucket`, `creator`. No new
structure is expected; `type` is already a string, so the polymorphic follow-up
stays the caller's decision.

Name it `GetRecording` again only after checking that nothing in the removal
left a reserved-name trap — the operation ID is free in `openapi.json` today.

## Implementation notes for BC3

Nothing required. The route is drawn, documented, and (unlike at #584) carries
a documented response. If the unscoped form is ever withdrawn in favour of the
bucket-scoped alias, say so in `doc/api/sections/recordings.md` — the SDK has
been burned once by the gap between a drawn route and a renderable one.

## SDK absorption plan when this lands

- One Smithy operation, one tag in `spec/overlays/tags.smithy`, `TAG_TO_SERVICE`
  already has a Recordings group.
- Reuse the existing `Recording` structure; verify against
  `spec/fixtures/` that the generic envelope needs no new required fields.
- Conformance: one happy-path case plus a 404, in `recordings.json`.
- Delete this file and its two `spec/bc3-route-allowlist.yml` entries in the
  same PR — the gate reports a registry entry that no longer matches as stale.
