---
gap: everything-boosts-withdrawn
status: addressed-in-bc3-pr-12464
detected: 2026-07-29
sdk_demand: medium
bc3_pr: 12464
bc3_refs:
  introduced_in: master
  routes:
    - "GET /:account_id/boosts.json"
  controllers:
    - app/controllers/everything/boosts_controller.rb
  related_existing_api:
    - GetEverythingMessages (surviving flat everything root — same contract family)
    - ListRecordingBoosts (item-scoped boosts, unaffected by the withdrawal)
  reintroduction_pr: 12463
  diagnosis_pr: 12458
---

# Everything boosts feed withdrawn (subtractive delta, endpoint removed)

> **Not an additive gap.** Every other entry in this registry tracks *new* BC5
> surface awaiting JSON coverage. This entry tracks a *subtractive* delta — an
> endpoint BC5 shipped and later withdrew — and records the SDK's matching
> removal. `addressed-in-bc3-pr-12464` here means BC3 shipped the *withdrawal*
> of the route, not a new contract. #12464 merged to `master` on 2026-07-30
> (`b06acfac1`), after the provenance pin (`dffa7e11` / 2026-07-28) — the SDK
> removal is absorption ahead of the pin; the next repin will find it already
> absorbed.

## What's missing

The account-wide `GET /:account_id/boosts.json` aggregate feed. BC5 shipped it
in the everything-aggregates flat family (BC3 #11627, see
[everything-aggregates.md](everything-aggregates.md)), then withdrew it: the
feed's rendering was diagnosed as broken in BC3 **#12458**, and BC3 **#12464**
(merged 2026-07-30, `b06acfac1`) removes both routes to the endpoint. The SDK
removed its `GetEverythingBoosts`
operation, the `EverythingBoost` feed element, and all downstream artifacts
across the six SDKs to match — the SDK does not model an endpoint the server
no longer serves.

Item-scoped boost operations (recording/event boosts list/get/create/delete)
are unaffected — BC5 keeps those. The shared `Boost.recording` field and
`RecordingParent.bucket` also stay: the `my/boosts` feed still embeds the
boosted recording with its project context.

## Why it matters

Integrations that surfaced "all boosts across all projects" lose the single
paginated request for it; the workaround is walking projects (or recordings)
and concatenating item-scoped boost listings. The withdrawal is deliberate on
the BC3 side — a broken feed is worse than an absent one — and reintroduction
is already tracked, so the SDK gap is expected to be temporary.

## Suggested API shape

None to propose — the shape is settled by history. When the feed returns it is
expected to match the withdrawn contract: a flat, recency-ordered
(newest-first), Link-paginated array of boosts, each carrying its `booster`
and the boosted `recording` rendered through the full recording projection
with embedded `bucket`.

## Implementation notes for BC3

BC3 **#12463** tracks reintroducing the feed once the rendering is fixed
(diagnosis in **#12458**). Nothing further is pending SDK-side until that
ships.

## SDK absorption plan when this lands

When BC3 #12463 reintroduces the feed, re-add `GetEverythingBoosts` to
`spec/basecamp.smithy` (operation + input/output + `EverythingBoostList` /
`EverythingBoost` shapes), re-tag it in `spec/overlays/tags.smithy`, restore
the TS/Kotlin generator type-registry entries, regenerate all six SDKs, and
restore the Go wrapper, tests, and conformance fixtures/dispatch. The removal
commit for SDK PR (branch `drop-boosts`) is the reference for an exact
inverse. Reintroduction ships in its own SDK release.
