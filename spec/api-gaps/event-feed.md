---
gap: event-feed
status: no-json-contract
sdk_demand: high
detected: 2026-08-01
bc3_refs:
  introduced_in: eventstream+accountid (unmerged branch; BC3 PRs 9646/9659)
  routes:
    - GET /:account_id/events.json
    - POST /:account_id/events/stream_ticket.json
  controllers:
    - app/controllers/events_controller.rb
    - app/controllers/events/stream_tickets_controller.rb
    - app/channels/events_channel.rb
  related_existing_api:
    - ListEvents (recording-scoped audit trail on one recording — a different concept from the account-wide feed)
---

# Account-wide event feed (poll lane + Action Cable push lane)

## What's missing

Everything — this is a **pre-merge** gap. BC3 PRs **#9646**/**#9659** implement an
account-wide, resumable event feed on the `eventstream+accountid` branch: a catch-up
**poll lane** (`GET /events.json` with `types`/`buckets`/`creators` filters, returning a
body envelope `{"events": [...], "position": "...", "next": "..."}` — signed opaque
positions and the continuation URL live in the body; the `X-Feed-Position` and
`Link: rel="next"` response headers merely echo it — with a 400-position / 400-filter /
409 / 410 error matrix) and a **push lane** (native Action Cable `EventsChannel`,
authenticated by short-lived stream tickets minted at `POST /events/stream_ticket.json`).
Neither route exists on BC3 `master`; the SDK models nothing for it.

The SDK plan is two layers:

- **Layer 1 (generated):** `PollEvents` + `CreateStreamTicket` operations on a new
  `eventFeed` SDK service (the `events` service name is taken by the recording-scoped audit
  trail). Blocked on the BC3 merge.
- **Layer 2 (hand-written):** the one blessed connector — reconnect/backoff state machine,
  ticket lifecycle, confirmation gating, dedupe, checkpoint discipline — shipped in all six
  languages as spec-normative infrastructure in the same class as SPEC §14 (Download),
  §15 (Webhooks), and §16 (OAuth Utilities). It is **not** a §18 composite
  (`SPEC.md` §"Hand-Written Composite Methods"): composites are stateless multi-call
  orchestrations; this is a long-lived stateful transport with its own liveness contract.
  A new SPEC **§23** will carry the connector contract, and SPEC §22's out-of-scope bullet
  will be narrowed to stop excluding WebSocket transport for this one sanctioned lane.

## Settled pre-merge (bc3, verified `8be5c67de5`; lineage `ee19670c02`)

The SDK's pre-merge review and BC3's recorded reply settled the following on the branch.
SHAs here name the branch head each item was verified against — they are verification
records, never the SDK's provenance pin (the pin never references unmerged history).
Everything below is re-verified at BC3's merge-time gate before any fixture freezes.

- **The body envelope is the contract.** Every `200` poll response carries
  `{"events", "position", "next"}` (`next` present only while a walk continues, bound to
  the walk's frozen head — never a checkpoint). Headers are echoes. `PollEvents` is an
  ordinary generated operation; the response-header generator workstream is dead.
- **The filter digest is published as a versioned contract (`srv1`)** — canonicalization
  rules and test vectors in BC3 `doc/api/sections/event_feed.md`. The server's wire format
  is the bare 16-lowercase-hex digest; `srv1-<digest>` is the SDK-side checkpoint-lineage
  namespace. The SDK's fallback local codec is retired unused.
- **The 409 filter-mismatch body is enriched:** it names `position_digest` (the digest the
  held position was minted for) and `filters_digest` (the digest of the filters the request
  presented), both bare srv1 hex.
- **The disconnect-reason matrix is four rows:** handshake failure (HTTP upgrade failure,
  no frame); `unauthorized`/`reconnect:false` (retriable with a fresh ticket; terminal
  `authorization_failed` after 3 consecutive); `remote`/`reconnect:true` (server-initiated
  disconnect, including mid-connection revocation → re-mint and reconnect; a genuinely
  revoked user's mint then fails, and that mint failure **is** the designed detection path
  — revocation is not wire-distinguishable at disconnect time);
  `invalid_event_stream_command`/`reconnect:false` (protocol violation, terminal).
- **Stream tickets are stateless, replayable bearer credentials.** The mint is a pure
  verifier check with no server-side nonce or consumption, so `CreateStreamTicket` is
  safe-to-retry (`idempotent: true` in that sense — deliberately not a claim of identical
  responses). Statelessness is re-confirmed at the merge-time gate. Mint-per-connection is
  connector discipline, not server enforcement.
- **Raw filter bounds:** over 1,000 elements or 16 KB per filter list → filter 400;
  `buckets`/`creators` cap at 100 ids each (400 above the cap). The page cap of 100 is
  documented for consumer math; scan internals stay internal — follow `next`.
- **A wrong-account position** is the plain 400 malformed-position (re-enter via `since=`).
- **`visible_to_clients` is presence-bearing:** push payloads carry it, poll rows omit it;
  absent ≠ false. Model it as an optional field, never a defaulted boolean.
- **"Guarantor"** = Action Cable JS's `SubscriptionGuarantor`, the subscriptions layer's
  self-perpetuating ~500ms unconfirmed-subscribe retransmit chain; disposing a subscription
  must kill that retransmit timer.
- **Entry-boundary semantics are position-relative, never wall-clock.** `since=now` (and
  the bare present entry) mints the cursor at the newest **visible** event id. An in-flight
  transaction that drew a lower id before entry and commits after it falls permanently
  behind that cursor — the poll lane never serves it (a deterministic regression on the
  branch pins this). The delivery bound covers ids above your position whose transactions
  commit within the safety delay. Consequences the SDK connector encodes:
  connect-live-before-entering is load-bearing (the live buffer is the only carrier of an
  in-flight-at-entry straggler), and on a bare-present or 410-reset entry the connector
  delivers every live-buffered event observed at its entry snapshot **before** persisting
  the entry poll's position (drain-before-save). This is a save-ordering invariant scoped
  to a defined observation point — not a loss-prevention guarantee, and no global
  at-least-once claim is made: post-snapshot lower-id stragglers are the feed's documented
  best-effort case, and completeness-critical workflows corroborate against canonical
  resource APIs.
- **Smithy ownership:** the SDK authors the `PollEvents` + `CreateStreamTicket` Smithy +
  regeneration PRs; BC3 reviews (COORDINATION.md's Division-of-labor line now matches its
  Lifecycle step 3).

What remains open is BC3's **merge-time gate**: rebase both PRs onto frozen `master`,
re-run full CI, obtain fresh exact-head reviews, re-confirm ticket statelessness, and
regenerate the wire transcripts against the rebased head. Provisional transcripts exist
and SDK runners/parsers build against them, but fixtures freeze only at that gate.

## Why it matters

Integrations that need "tell me when something happens" currently hand-roll polling loops
against list endpoints with timestamp cursors — an approach that is lossy at page boundaries
by construction (same-timestamp ties dropped across a page edge). A production integration
has already built a 662-line poller plus cursor store, dedupe store, and reconciliation pass
to approximate this feed. The feed's signed, strictly-id-ordered positions are the fix, and
the connector is how six languages consume it without six divergent reinventions of a
subtle stateful protocol.

## Suggested API shape

Layer 1, once the BC3 contract merges (names final, shapes provisional until the
merge-time gate):

| Operation | Method + path | Notes |
|---|---|---|
| `PollEvents` | `GET /{accountId}/events.json` | filters `types`/`buckets`/`creators`; entry via bare (present), `since=`, or `position=`; returns the `{events, position, next}` envelope — the durable cursor and continuation URL are body fields |
| `CreateStreamTicket` | `POST /{accountId}/events/stream_ticket.json` | returns `{ticket, expires_in, url}`; safe-to-retry (stateless mint), **not** identical-response-idempotent |

`PollEvents` needs a **cursor-page pagination mode** (pages yielded with their per-page
position), not the flattening `Link`-follow auto-pagination — flattening swallows the
per-page `position` that is the durable checkpoint.

Both operations join the single `Basecamp` Smithy service (no second service shape), tagged
into a new `eventFeed` service group; SPEC §5's service list grows by one.

## Implementation notes for BC3

Server side is implemented on the branch, and the pre-merge contract decisions above are
recorded — nothing further is needed from BC3 before the merge-time gate. At that gate:
rebase, re-CI, fresh exact-head reviews, re-confirmed ticket statelessness, regenerated
transcripts; the SDK then re-verifies every settled item above (wire literals **and**
semantic behaviors — entry-boundary semantics, frozen-head `next` predicate, safety-bound
behavior) before freezing fixtures. The branch is diverged from `master` (re-derive the
count; it grows), so the SDK treats the contract as provisional until then.

**Spec-train note:** the SDK-side work follows the one-spec-changing-PR-in-flight rule.
Occupants are re-derived at every branch and PR-open boundary via the standing census
(`gh pr list` filtered for `SPEC.md`, `Makefile`, `openapi.json`, `spec/basecamp.smithy`,
`spec/api-gaps/`, `conformance/`) rather than named here — a named list stales overnight.
Layer 1 will not open until the BC3 contract merges, the merge-time gate clears, and the
census clears for the surfaces it touches.

## SDK absorption plan when this lands

Layer 1 is the full `AGENTS.md` §"SDK Change Completeness Bar", not just two Smithy
operations:

1. **Smithy spec** — both operations + input/output structures (the `{events, position,
   next}` envelope as the `PollEvents` output) + error lists (400-position/400-filter as
   distinct typed reasons, 409 with both digest fields, 410 with
   `epoch_after_id`/`resume`); provenance repin.
2. **Tags** — `spec/overlays/tags.smithy` entries for a new `EventFeed` tag.
3. **Generator mappings** — `TAG_TO_SERVICE` updates where the default fallback doesn't
   resolve, plus the cursor-page pagination mode's first consumer.
4. **Client wiring, all six SDKs, named exactly:**
   - Go: `AccountClient` field/accessor plus a generated-client-backed service wrapper
     (wiring under `go/pkg/basecamp/` follows the `bookmarks.go` precedent)
   - TypeScript: `typescript/src/client.ts` + re-export from `typescript/src/index.ts`
   - Ruby: `ruby/lib/basecamp/client.rb`
   - Python: `python/src/basecamp/client.py` **and** `python/src/basecamp/async_client.py`
   - Kotlin: regenerated `ServiceAccessors`
   - Swift: regenerated `AccountClient+Services`
5. **Tests** — TypeScript, Ruby, and Python service tests (happy path + error case per
   operation), plus conformance `paths.json`/dispatch entries.
6. **Regeneration + drift gates** — full pipeline, `go-check-drift`/`kt-check-drift`/
   `py-check-drift`/`swift-check-drift` clean.
7. **Seam adapters** — thin `TicketMinter`/`PollSource` adapters over the generated
   operations for the Layer-2 connector, with an adapter contract test pinning retry
   ownership (generated ops keep SPEC §7 internal retry; the connector governs reconnect
   cycles only).

Layer 2 (the connector, SPEC §23, the `conformance/event-feed/` fixture family, and the
`event-feed-fixtures-check` gate) proceeds ahead of absorption against the recorded
decisions above, marked provisional where it pins BC3-derived wire literals or semantics;
fixture freezing is gated on BC3's merge-time gate.
