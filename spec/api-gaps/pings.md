---
gap: pings
status: absorbed-in-sdk
detected: 2026-10-19
sdk_demand: high
bc3_pr: 13943
smithy_refs:
  - ListPings
  - GetPing
  - CreatePing
  - Ping
  - PingLinks
  - BareValidationError
bc3_refs:
  introduced_in: "Circle creation over JSON (BC3 #9949, 74320ef45b3) with app/views/api/circles/ show and create partials; the index template and doc/api/sections/pings.md arrive in BC3 #13943, merged 2026-10-19"
  routes:
    - GET /:account_id/circles.json
    - GET /:account_id/circles/:id.json
    - POST /:account_id/circles.json
  controllers:
    - app/controllers/circles_controller.rb
  related_existing_api:
    - GetCampfire
    - ListCampfireLines
    - CreateCampfireLine
    - RecordingsSummarize (Campfire discovery reads GetPing for a bucket that is not a project)
---

# Pings

## What's missing

A Ping is a private conversation held in its own bucket, a `Circle`, with one
Campfire. The SDK could read and write a Ping's lines through the Campfires
service only when the caller already knew the Campfire's id, and it had no way
to list Pings, open one, or learn a Ping's Campfire from its bucket.

## Why it matters

That last gap broke chat-line discovery. A `chat.line.created` event carries
the line and its bucket, not its Campfire, and `RecordingsSummarize` found the
Campfire from the bucket's project dock and then from `GET /chats.json`
filtered to the bucket. A Circle is not a project, so the dock read 404s, and
BC3's `/chats.json` lists only dock-positioned Campfires, so a Ping's Campfire
is never in it. Every Ping line came back `recording_unresolved` with no
candidates tried, and an agent connector built on the SDK could not read a
single Ping it was sent.

## Suggested API shape

What BC3 serves, and what the SDK now models as the `Pings` service. The wire
path and the bucket type are `Circle`; every SDK name says Ping, the way
`Campfires` sits over `/chats.json`. bc3 #13943 titles the section "Pings"
with "Get Pings", "Get a Ping" and "Create a Ping".

- `ListPings` (Get Pings) — `GET /circles.json`, the caller's active Pings
  that have a line in them, newest first, unpaginated. On the API host before
  BC3 #13943 there is no index template and the answer is a bodiless 204. Go
  and TypeScript read that as an empty list; the generated list reads in the
  other five SDKs decode the body and fail on it, as every unpaginated array
  read there does. The template ends that window.
- `GetPing` (Get a Ping) — `GET /circles/{pingId}.json`. The id is the Ping's
  bucket id.
- `CreatePing` (Create a Ping) — `POST /circles.json` with `user_ids` and an
  optional `new_group`. Without `new_group` the Ping that already holds
  exactly those people is returned; either way the answer is 201. A bodiless
  422 (`BareValidationError`) when nobody in the Ping could use Pings.
- `Ping.room_url` names the Campfire. `PingLinks` models only `lines` and
  `files`; the wire object's `self`, `app`, `bookmark` and `edit_title` repeat
  the Ping's own `url`, `app_url`, `bookmark_url` and `edit_title_url` (and
  `self` is not a legal Rust field name).

All three answer `403 {"error": "Pings have been disabled"}` when the caller
cannot ping.

## Implementation notes for BC3

The routes were drawn and served on BC3 master (`resources :circles`,
`CirclesController`, `allow_agent_principal` on index, show and create), and
`GetPing` was verified against production on 2026-10-19. BC3 #13943 (merged
2026-10-19) added the API index template and `doc/api/sections/pings.md`, so
all three routes are now documented upstream. A Ping payload carries no
Campfire id of its own, so clients read it from `room_url`; a `room_id` member
would spare that, and is not required.

## SDK absorption plan when this lands

Absorbed ahead of BC3 #13943's merge, then repinned past it. The three routes
now come from the documented route table (`spec/bc3-routes.json`), and the
temporary `sdk_routes_absent_from_bc3_docs` entries that carried them in
`spec/bc3-route-allowlist.yml` are deleted.

Discovery now reads the bucket's own Campfire: the dock for a project, and for
a bucket whose project read 404s, the Ping read, cached per bucket under the
same TTL and refresh floor, in all seven SDKs against the shared
`recording_summary.json` fixture. A project costs no extra request; a bucket
that is neither costs one extra 404 per TTL before the listing, as before.
