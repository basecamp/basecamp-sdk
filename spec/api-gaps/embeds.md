---
gap: embeds
status: absorbed-in-sdk
detected: 2026-10-20
sdk_demand: medium
bc3_pr: 13962
smithy_refs:
  - CreateEmbed
  - GetEmbed
  - Embed
  - EmbedSgid
bc3_refs:
  introduced_in: "EmbedsController served the web editor only; bc3 #13962 (branch api-create-embeds, open) serves create and show on the API host and adds doc/api/sections/embeds.md"
  routes:
    - POST /:account_id/embeds.json
    - GET /:account_id/embeds/:sgid.json
  controllers:
    - app/controllers/embeds_controller.rb
  related_existing_api:
    - Rich text `<bc-attachment sgid>` (doc/api/sections/rich_text.md, "Inserting an embed")
---

# Embeds

## What's missing

Rich text written through the API could carry a link but not an embed. The
editor turns a pasted YouTube or Dash link into an embed through
`POST /embeds.json`, but the API host refused that action with a 404, and
`GET /embeds/:sgid.json` had no API template and answered 500. An agent writing
a card or comment had no way to get the `sgid` a `<bc-attachment>` needs.

## Why it matters

Agents post links to videos and dashboards in cards, comments and messages, and
those read far better embedded than as bare links. Without an sgid there is no
embed.

## Suggested API shape

What bc3 #13962 serves, and what the SDK models as the `Embeds` service:

- `CreateEmbed` — `POST /embeds.json` with `url`. Find-or-create, answering
  `200` either way: a new embed's `status` is `pending`, and an existing one
  comes back in whatever status it has. `422 {"error": "Basecamp can't embed
  this URL. Link to it instead."}` when no oEmbed provider matches, `429` past
  100 creates an hour per person. Modeled naturally idempotent, since a
  repeated create finds the embed the first one made.
- `GetEmbed` — `GET /embeds/{sgid}`, `404` for an sgid that names no embed.
  The sgid is opaque and can hold `/`, so every SDK sends it as one
  percent-encoded path segment.
- `Embed` — always `url`, `status` (`pending`, `active`, `not_found`) and
  `sgid`; an active embed adds `type`, `content_type`, `image_url` (nullable)
  and, when the provider gave HTML, sanitized `content`.

## Implementation notes for BC3

Both actions allow agent principals, personal and team. Create answers the
browser's editor exactly as before, `200` for a new embed too. The rate limit
counts every create it answers and none it refuses.

## SDK absorption plan when this lands

Absorbed ahead of bc3 #13962's merge. The two routes ride temporary
`sdk_routes_absent_from_bc3_docs` entries in `spec/bc3-route-allowlist.yml`;
the repin past #13962 brings them into `spec/bc3-routes.json`, the gate reports
the entries stale, and they are deleted.
