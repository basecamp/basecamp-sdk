---
gap: scratchpad
status: addressed-in-bc3-pr-12322
detected: 2026-05-01
sdk_demand: medium
bc3_pr: 12322
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3b
  routes:
    - "GET /:account_id/my/notes.json"
    - "PUT /:account_id/my/notes.json"
  controllers:
    - app/controllers/my/notes_controller.rb
  related_existing_api: []
---

# Scratchpad / My Notes (per-user note)

## What's missing

SDK absorption only — the contract shipped via BC3 **#12322** ("My Notes") in
the BC5 API train (2026-07-18..21). The once-deferred URL question is settled:
the shipped route is **`/my/notes.json`**, documented in
`doc/api/sections/my_notes.md` on `master`.

The resource is a per-person notebook — a single rich-text note that follows
the user across devices. The API treats it as a singleton at `/my/notes.json`:

- `GET /my/notes.json` — returns the authenticated user's note. If the user
  has not yet written anything, the shape is the same with empty `content` and
  `null` `id` / `created_at` / `updated_at` — the record is created on first
  update.
- `PUT /my/notes.json` — updates (or first-creates) the note. Saves are
  versioned server-side; the API always returns the current note.

The wire `type` is `Notebook::Note`.

## Why it matters

External SDK consumers building dashboards or note-syncing tools can't read or
write the user's note without screen-scraping. As the navigation gets more
first-class data attached to it, leaving notes off the API surface is a
predictable source of demand-signal complaints.

## Suggested API shape

Per the merged `doc/api/sections/my_notes.md`: `id`, `type`
(`"Notebook::Note"`), `content` (rich text), `created_at`, `updated_at` —
with the documented null-until-first-write behavior on `GET`.

## Implementation notes for BC3

Shipped — nothing pending. `my/notes_controller.rb` serves the singleton
routes and `doc/api/sections/my_notes.md` documents them.

## SDK absorption plan when this lands

- Smithy: `GetMyNote`, `UpdateMyNote` (singleton — no id path param), grouped
  with the `My*` services per the shipped `/my/notes.json` path.
- Model the nullable `id` / `created_at` / `updated_at` on the GET response
  (pre-first-write state).
- Status flips to `absorbed-in-sdk` with the absorption PR (which adds the
  Smithy refs).
- Canary fixture: a single per-account fixture-id-free GET works well since
  the resource is implicit-self.
- Pairwise check: BC4 absent → BC5 present is fine.
