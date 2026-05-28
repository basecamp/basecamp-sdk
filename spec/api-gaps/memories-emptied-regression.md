---
gap: memories-emptied-regression
status: partial-coverage
detected: 2026-05-27
sdk_demand: high
bc3_pr: 10947
bc3_refs:
  introduced_in: master
  routes:
    - "GET /:account_id/my/readings.json"
  related_existing_api:
    - GetMyNotifications
---

# Memories emptied on BC5 (subtractive regression)

> **Not an additive gap.** Every other entry in this registry tracks *new* BC5
> surface awaiting JSON coverage. This entry tracks a *subtractive regression*:
> a field BC4 populates that BC5 emptied. `status: partial-coverage` is the
> closest schema fit — read it as "coverage regressed," not "coverage partially
> added." Status flips to `addressed-in-bc3-pr-10947` when #10947 merges.

## What's missing

`GET /:account_id/my/readings.json` emits a top-level `memories` array. On BC4
(the `four` branch) it is populated; on BC5 (`master`, production) it ships as
an unconditional empty array. Source diff of
`app/views/api/my/readings/index.json.jbuilder`:

- BC4 (`four`): `json.memories @memories, partial: "my/readings/reading", as: :reading`
  — **populated**.
- BC5 (`master`, production): `json.memories []` — **unconditional empty
  array**, no account-gating.

The per-reading `memory_url` field is preserved on the wire; only the top-level
collection regressed. There is **no** "alias to `bubble_ups`" on production:
the once-assumed commit `64acf34` does not exist on `four`, `five`, or `master`.

## Why it matters

This is silent data loss for existing BC4 integrations: the request still
succeeds, the field is still present and still type-conformant (an array), so
per-backend schema validation passes on both sides. Only a *pairwise* BC4↔BC5
comparison catches it — exactly the additive-only invariant the live canary
exists to enforce. It is the canonical demonstration of why per-backend schema
checks are necessary but not sufficient.

## Suggested API shape

No new shape is needed, and the SDK proposes none: BC3 has already chosen the
fix. Open PR **#10947** (head `9dc63e2e`) changes the jbuilder to
`json.memories @bubble_ups`, repopulating the top-level collection from the
Bubble Up successor so BC4 readers keep receiving a populated array. This entry
records that as the fix-in-flight rather than re-proposing a shape.

## Implementation notes for BC3

- The fix is one line and **already written** in open PR #10947
  (`json.memories @bubble_ups`). This is a merge/ship item, not a design item.
- Until #10947 merges, production `master` stays regressed.
- BC3 provenance at the time of this finding: `five+api` @ `716e710ee5`
  (the reconciliation handoff, suite green).

## SDK absorption plan when this lands

- **Now (regression live):** the live-canary invariant for `memories` lives in
  **PR #308**, not on this branch. That PR adds the
  `pairwiseSupersetArray: ["memories"]` rule on `GetMyNotifications` plus a
  temporary `pairwiseDeltaAllowed: ["memories"]` waiver (in
  `conformance/tests/live-my-surface.json`, which exists on #308's branch) that
  keeps the canary green on this known regression while still protecting every
  other path. This branch carries the registry record only — no canary files —
  and is what that waiver points back to.
- **Once PR #308 and BC3 #10947 have both landed:** remove the
  `pairwiseDeltaAllowed: ["memories"]` waiver from
  `conformance/tests/live-my-surface.json` (restoring the hard-fail), repin
  `spec/api-provenance.json` past the fix, and flip this entry's `status` to
  `addressed-in-bc3-pr-10947` (the `bc3_pr: 10947` field is already set).
- No **structural** Smithy change is required — `memories` is already modeled
  on `GetMyNotificationsOutput`. (This PR does realign that field's doc comment
  to describe the regression + the #10947 fix and regenerates the artifacts
  that inherit it; that is documentation, not a shape change.)
