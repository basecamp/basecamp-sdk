---
gap: memories-emptied-regression
status: addressed-in-bc3-pr-11628
detected: 2026-05-27
sdk_demand: high
bc3_pr: 11628
bc3_refs:
  introduced_in: master
  routes:
    - "GET /:account_id/my/readings.json"
  related_existing_api:
    - GetMyNotifications
---

# Memories emptied on BC5 (subtractive delta, settled by contract)

> **Not an additive gap.** Every other entry in this registry tracks *new* BC5
> surface awaiting JSON coverage. This entry tracked a *subtractive* delta —
> a field BC4 populates that BC5 emptied — and records how it settled:
> **permanently empty by documented contract**. `addressed-in-bc3-pr-11628`
> here means BC3 shipped the *documented contract for the empty field*, not a
> repopulation.

## What's missing

Nothing anymore — the contract is settled. `GET /:account_id/my/readings.json`
emits a top-level `memories` array that is **permanently `[]` on BC5**.
`doc/api/sections/my_notifications.md` (language codified by BC3 **#11628**,
the BC4 wire-format compatibility PR in the BC5 API train) documents it
explicitly: `memories` "remains in the payload as an always-empty placeholder,"
replaced by `bubble_ups` (capped at the 50 most recently read items, with
scheduling surfaced separately under `scheduled_bubble_ups`).

History of the finding, kept as narrative:

- **2026-05-27 (regression discovery):** source diff of
  `app/views/api/my/readings/index.json.jbuilder` showed BC4 (`four`) rendering
  `json.memories @memories, partial: "my/readings/reading", as: :reading`
  (populated) while BC5 (`master`, production) shipped `json.memories []` —
  an unconditional empty array, no account-gating. The per-reading
  `memory_url` field was preserved; only the top-level collection emptied.
  There was never an "alias to `bubble_ups`" on production — the once-assumed
  commit `64acf34` does not exist on `four`, `five`, or `master`.
- **Interim (fix-in-flight framing):** then-open BC3 PR #10947 carried a one-line
  `json.memories @bubble_ups` alias that would have repopulated the collection.
  This entry tracked that as the pending fix.
- **2026-07-18..21 (settled):** the BC5 API train shipped and **#10947 closed
  unmerged**, superseded by the train. The alias never shipped and never will.
  #11628 codified the always-empty placeholder language in
  `doc/api/sections/my_notifications.md`. This is now a **permanent, accepted
  BC4→BC5 subtractive delta**, no longer a pending regression.

## Why it matters

For existing BC4 integrations this remains a real behavior change: the request
still succeeds, the field is still present and still type-conformant (an empty
array), so schema validation against BC5 passes. The delta is now *documented
and intentional* — an accepted contract, not a regression — so it no longer
needs active enforcement. Historically only a *pairwise* BC4↔BC5 comparison
could surface it, and the live canary carried exactly that additive-only
invariant; with BC5 having replaced BC4 in production there is no live BC4 to
compare against, so the pairwise rule is retired and the contract is settled by
this document. `GetMyNotifications` is still schema-validated by the
single-backend canary, which is where new BC5 additions (`bubble_ups`,
`scheduled_bubble_ups`) surface as extras-observed.

## Suggested API shape

None — BC3 decided the shape by documentation: `memories` stays in the payload
as an always-empty placeholder, and `bubble_ups` / `scheduled_bubble_ups` are
the durable successors. The same doc page documents the adjacent bubble-up
surface the SDK should absorb next:

- `bubble_ups_count` and `scheduled_bubble_ups_count` — top-level counts for
  notification UI.
- `limit_bubble_ups` query param — `true` caps `bubble_ups` at 2 current
  items and omits the `scheduled_bubble_ups` key (defaults to `false`).
- `GET /my/readings/bubble_ups.json` — a dedicated paginated list (50 items
  per page) of current and scheduled bubble-ups.

## Implementation notes for BC3

None pending — BC3's side is done. `doc/api/sections/my_notifications.md` on
`master` is the contract of record: `memories` is an always-empty placeholder
superseded by `bubble_ups`. Any future repopulation of `memories` would be a
contract change requiring a doc update, at which point this entry gets a
follow-up.

## SDK absorption plan when this lands

- **Pairwise enforcement retired; contract settled by this document:** the
  live-canary invariant for `memories` was a `pairwiseSupersetArray:
  ["memories"]` rule on `GetMyNotifications` plus a `pairwiseDeltaAllowed:
  ["memories"]` waiver (introduced in **PR #308**). BC5 replaced BC4 in
  production, so there is no live BC4 backend to compare against — the pairwise
  machinery was removed and the `memories: []` contract now rests on
  `doc/api/sections/my_notifications.md` (BC3 #11628) and this entry.
  `GetMyNotifications` is still schema-validated by the single-backend canary
  (`conformance/tests/live-my-surface.json`). The pairwise engine is
  recoverable from git history (PR #308) should a reachable legacy backend ever
  warrant restoring it.
- **Future absorption items** (separate additive PR, not this entry's
  regression scope): `bubble_ups_count` / `scheduled_bubble_ups_count` fields
  and the `limit_bubble_ups` query param on `GetMyNotifications`, plus a new
  operation for `GET /my/readings/bubble_ups.json`.
- No **structural** Smithy change is required for `memories` itself — it is
  already modeled on `GetMyNotificationsOutput`. Its doc comment describes the
  settled contract (permanently empty on BC5; use `bubble_ups`); the artifacts
  that inherit it are regenerated in this PR. New integrations must not rely
  on `memories` being populated on BC5.
