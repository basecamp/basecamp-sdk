# Event Feed connector tier-2 scenario fixtures

> **PROVISIONAL — bc3-derived rows verified at bc3 `8be5c67de5`** (pre-merge; lineage
> `ee19670c02`). Everything bc3-derived below is normative for SDK drafting but frozen
> only at bc3's merge-time gate; the dependency table classifies each row and the PR-T
> true-up checklist governs the re-verification. The SDK-owned contract rows (state
> inventory, ownership cut, conjunctive save-ordering invariant, handler contract,
> dedupe, timers, checkpoint identity, URL validation) are final as written and not
> gated on bc3.

Data-only, cross-language scenario scripts for the SPEC §23 Event Feed connector —
tier 2 of the three-tier verification architecture: the cross-lane state machine,
frame-level cable behavior, mint/connect/poll interleave, and virtual time. Tier 1
(poll-lane wire behavior in `conformance/tests/`) is deferred on the generated
`PollEvents` landing; tier 3 (LRU bounds, jitter formula, real-transport adapter
contracts, test-clock semantics) is per-SDK native tests.

Each fixture is one strictly-ordered interleaved script: HTTP exchanges, cable
frames, time directives, and observations in a single `steps` array. **Strictness is
the default and not optional** — every protocol action (a mint, poll, connect,
outbound frame, client close, or checkpoint save) must be matched by an expect step,
under the per-action-class rules in "Strictness semantics, per action class" below.
That is what makes "zero reconnects after rejection", "no save before the retained
drain", and "zero requests to a hostile origin" structural properties rather than
heuristics. Observation directives (`expectDelivered`, `expectBuffered`,
`expectTimers`, `expectState`, `expectSignal`, `expectHandlerInvocations`) are
rendezvous assertions: the driver waits under a small wall-clock watchdog (~5s) while
virtual time is frozen between directives, and an assertion that can no longer be
satisfied fails hard — nothing here passes vacuously.

## Strictness semantics, per action class (normative)

The strict-match rule splits by action class; every driver implements exactly this:

- **Arrival-strict — checkpoint saves and outbound frames** (the subscribe command,
  the client close): observed while the current step is anything other than their
  matching expect step, the scenario fails immediately. Rendezvous steps advance
  atomically on satisfaction, so an action that becomes legal the instant a
  rendezvous is satisfied matches the next step rather than failing against the
  stale one.
- **Parked — mint and poll seam calls**: an early seam call does not fail the
  scenario on arrival; it PARKS, and calls are matched to their expect steps in
  order. The scripted response is released only when the driver reaches the matching
  expect step — until then the connector waits on its own call. A parked call still
  unmatched when the script ends fails the scenario, and the seam-call counts in
  `finally` count it either way.

A driver's step pointer moves in two ways, and both are needed for the atomic
handoff to hold. It advances inside the critical section that satisfies a
rendezvous, which covers a driver blocked ON one; and where the pointer has not
yet reached a step the recorded history has already satisfied, the arrival rule
reads THROUGH that step to the one after it — a poll's page is delivered and
then checkpointed by one causal chain in the connector, and the driver need not
have reached its `expectDelivered` step by the time the save lands. A step the
DRIVER performs rather than waits for (`serve`, `advance`, `fireTimer`,
`sever`, `serverClose`) is not read through: the connector cannot react before
the driver acts, so the pointer is handed to the next step before such a step
acts, and an action arriving under one the driver has not performed is early.

What a driver can hold an action to is when it OBSERVED it. A save is observed
where the connector makes it, but a driver running a real socket observes an
outbound frame only when it reads one, which lags the write by a hop — so a
frame written a step early can still be read after the driver has stepped on.
The rule is not weaker for it; the observation is. A driver that wants the
write's own instant pinned needs a witness at the write, not a stricter reading
of this rule.

## Validation

`schema.json` is the contract. `make event-feed-fixtures-check` validates the schema
against the JSON Schema metaschema and every fixture against the schema, with a
pinned `check-jsonschema` run through `uvx` (part of `make conformance`, so
`make check` gates it).

The same gate then verifies the schema's load-bearing `allOf` pins with the
probes in `pin-probes/` (`scripts/check-event-feed-pin-probes.py`). Each probe
declares a `control` scenario and one `mutation`; the gate requires the control
to VALIDATE, derives the mutant from it in-process, and requires the mutant to be
REJECTED. Deriving (rather than committing an invalid file) is what makes the
isolation claim true by construction: the accepted/rejected delta is exactly the
declared mutation, a control that stops validating fails the gate rather than
masking a wrong-reason rejection, and there is no invalid artifact to go
malformed or drift extra deltas. A mutation whose path is absent from the
control, or whose value equals the control's, fails as vacuous. The current pair
pins the per-signal default-terminal rules: a phantom invocation of a signal kind
whose disposition key is absent. Harnesses must never glob `pin-probes/` — probe
files are gate inputs, not scenario fixtures (and are not scenario-shaped).

## Directory is a schema boundary

This directory contains exactly one shape: tier-2 scenario scripts. If a second
shape is ever needed, it gets a sibling directory with its own schema — never a
second schema here. The srv1 digest vectors are exactly that sibling:
`conformance/event-feed-digest/` (gated by `make event-feed-digest-fixtures-check`),
following the `conformance/oauth/` / `conformance/oauth-token/` precedent.

## Placeholder substitution

Each SDK's harness substitutes these tokens with its own loopback origins and tokens
**before** driving the scenario, and must guarantee distinctness across indices
(`{{CABLE_URL:1}} ≠ {{CABLE_URL:2}}` is what gives fresh-ticket reconnect assertions
teeth):

| Placeholder | Meaning |
|---|---|
| `{{API_ORIGIN}}` | The HTTP loopback origin serving mint + poll. |
| `{{TICKET:n}}` | The nth minted stream ticket (opaque; never logged). |
| `{{CABLE_URL:n}}` | The nth mint's cable URL; the connector dials it verbatim. |
| `{{POS:n}}` | The nth position token (durable resume/repair cursor). |
| `{{NEXT:n}}` | The nth same-origin continuation URL. |

Literal origins (e.g. `https://attacker.example.com`) are intentional and must
**not** be substituted — the hostile-continuation fixtures depend on them staying
foreign, and the harness must assert those hosts receive **zero** requests
(structurally guaranteed: no expect step ever serves them).

## Count semantics: seam calls, never wire attempts

`finally.mintCount`, `finally.connectCount`, and every poll expectation count **seam
calls** — one fully-governed generated call each, with its full SPEC §7 retry
contract *inside* the seam. Scenarios never depend on advancing seam-internal time;
the connector's own delays all flow through the injected Clock, which is why these
scripts are deterministic.

## Virtual-advance algorithm (normative)

Every language's test clock must honor the same semantics, so a script means the
same thing everywhere: **advancing virtual time fires due timers in deadline order,
re-evaluating after each fire; timers scheduled during the advance whose deadlines
land inside the window also fire; ties break by creation order.** `fireTimer` fires
the earliest outstanding timer of the named kind *without* advancing the clock,
asserting its scheduled delay against a `{min, max}` envelope — that is how jitter
is asserted without a cross-language RNG seam (Go additionally pins the full-jitter
formula exactly in tier 3; a degenerate always-0 RNG is caught only there — a
documented divergence). Each language's test clock passes the shared semantics
checklist (deadline order, reentrant scheduling within an advance, creation-order
tie-break) before its tier-2 results count.

## Contract notes the fixtures encode (SDK-owned, final)

- **Connect-to-mint-URL-verbatim.** The connector never assembles cable topology
  client-side; every `expectConnect.url` is the immediately preceding mint's `url`,
  verbatim, query string included. A fresh ticket is minted on **every** reconnect
  pass — the connector never stores a mint URL across attempts.
- **Dedupe tracks actually-delivered event ids** — never position ordering. A
  buffered live event with an id ≤ the current position is still delivered (it was
  never served by poll); discarding live ids at or below the position is a named
  mutant (fixture 20).
- **The ownership cut** (present-class entries): after accepting the entry-poll
  response, the state machine performs one **bounded admission pass** — receiving
  from the frame pump's queue without blocking until the queue is momentarily empty
  OR the pass has **dequeued** `liveBufferCapacity` frames of any kind, whichever
  comes first. The bound counts dequeued frames, not admitted events (pings and
  control frames dequeue without admitting; an event-counting bound would spin
  forever under heartbeat replenishment). "Observed" means admitted into the
  state-machine-owned buffer at or before the cut; `expectBuffered` asserts exactly
  that.
- **The semantic-handler contract.** Conditions that change what the feed can
  promise (`BufferOverflow`, `FeedGap`) dispatch to a separate synchronous handler
  returning Accept or Terminate. A registered handler is invoked **exactly once per
  signal**, on the consumer's execution context, before its disposition takes
  effect; no handler means the typed terminal (`buffer_overflow` / `feed_gap`), and
  **a 410 never silently auto-continues**. Fixtures assert exact invocation records
  `{kind, disposition}`; the default-terminal fixtures assert zero invocations —
  the record is the only thing distinguishing handler-Terminate from no-handler.
- **The conjunctive save-ordering invariant** (the published delivery promise, and
  nothing stronger): `save(P)` only after ALL retained pre-cut events have been
  accepted AND every pre-cut loss condition has been explicitly accepted; Terminate
  — or no handler — means no save. A disjunctive form would let an accepted
  overflow bypass delivery of the other retained events. **Explicitly excluded:
  crash or cancellation before the first usable checkpoint** — on a first present
  entry there is no older durable cursor, and on a 410 reset the old cursor is
  unusable, so an event admitted pre-cut and lost to a crash before delivery and
  save is unrecoverable, with no signal. Client-side ordering cannot manufacture
  durability the server does not offer. **No blanket loss-prevention or global
  delivery-completeness claim is published anywhere in this family** — such a claim
  would contradict the exclusion; do not add one to a fixture description.
- **Checkpoint saves are strict-matched.** Every `CheckpointStore.save` call must
  match a current `expectCheckpoint` step, and `finally.checkpoints` re-checks the
  full ordered ledger. A save observed while the script is still waiting on an
  `expectDelivered` rendezvous fails the scenario — that ordering is the teeth of
  delivery-precedes-checkpoint and of the conjunctive invariant.

## Test-(d) split of duty

Terminal rejection is proven in two halves. **This family's half** (fixture 04): on
`reject_subscription` the connector cancels the deadline, explicitly closes the
still-open socket (`expectClientClose`), surfaces `subscription_rejected`, and makes
zero further mint/connect seam calls with an empty exact timer set. **bc3's half**
(its own suite, landed at the verified head): the rejection residue server-side is
heartbeat registration plus a persistent claim with **zero stream registrations
ever**. Neither suite asserts the other's half.

## Consumers

Tier 2 drives the **real transport adapter** over an in-process loopback cable
server wherever a lightweight one exists, with only the Clock faked; divergences are
honest, recorded here and in SPEC Appendix F, and compensated by named tier-3 tests.

| SDK | Driver | Lane / divergence |
|---|---|---|
| Go | `go/pkg/basecamp/eventfeed/scenario_conformance_test.go` | Real default transport over an in-process loopback ws server (`httptest`); fixtures consumed as data. Tier 3 additionally pins the full-jitter formula via an injected deterministic rand source. |
| TypeScript | `typescript/tests/event-feed/scenario-conformance.test.ts` | Real transport under MSW `ws` interception. Default transport must use the global `WebSocket` (Node ≥ 22), never the `ws` package. The global API has no read limit, so `max_frame_bytes` is enforced at message receipt rather than during the read — accepted, documented divergence; an injected transport MAY truly bound reads. |
| Python | `python/tests/event_feed/test_scenario_conformance.py` | Real transport over an in-process `websockets` loopback; connector transport ships under the optional `stream` extra. |
| Ruby | `ruby/test/basecamp/event_feed_scenario_conformance_test.rb` | Real transport over a `websocket-driver` loopback; `websocket-driver` becomes a runtime dependency. |
| Kotlin | `kotlin/sdk/src/jvmTest/kotlin/com/basecamp/sdk/EventFeedScenarioTest.kt` | **jvmTest-only** — ktor's `MockEngine` cannot mock WebSockets; a test-scoped ktor server drives the real ktor ws client. The state machine is additionally mirrored in commonTest tier 3 for the four acceptance scenarios. |
| Swift | `swift/Tests/BasecampTests/EventFeedScenarioTests.swift` | **Fake-transport lane** — no lightweight in-process ws server exists without SwiftNIO; scenarios drive a `FakeCableTransport`. A macOS-gated tier-3 `URLSessionWebSocketTask` adapter contract test proves the real adapter honors the transport contract (verbatim frames, close mapping). |

## Fixture inventory (PR 2)

The four §23 acceptance behaviors: (a) fresh-ticket reconnect → 05; (b)
confirmation gating → 02; (c) deadline teardown → 03; (d) terminal rejection → 04.
Numbers 08–11, 13–15, and 18 are reserved for PR 4 (staleness, single-flight
reconnect, reconnect-catchup-before-trust, poll/push dedupe overlap, backoff
envelope growth, repair-poll jitter, identical-subscribe retransmit,
revoked-mint threshold).

| # | File | Proves |
|---|---|---|
| 01 | `01-happy-path-confirm-catchup-stream.json` | mint → connect → welcome → subscribe → confirm → catch-up walk → drain → stream; delivery/checkpoint ordering; live ids never checkpoint |
| 02 | `02-confirmation-gating.json` | **(b)** zero deliveries, zero polls, zero saves before `confirm_subscription`; the pre-confirm event buffers and drains exactly once |
| 03 | `03-confirmation-deadline-teardown.json` | **(c)** deadline teardown leaves exactly `{backoff}`; fresh ticket + new cable URL on retry |
| 04 | `04-terminal-rejection.json` | **(d)** reject → explicit close of the open socket, `subscription_rejected`, zero reconnects, empty timer set |
| 05 | `05-fresh-ticket-reconnect-after-ttl.json` | **(a)** sever past ticket TTL → newly minted ticket URL on reconnect; one catch-up poll before any live delivery is trusted |
| 06 | `06-protocol-fatal-disconnect.json` | raw `invalid_event_stream_command` → Terminal(`protocol_fatal`), zero further mints — raw-frame interception |
| 07 | `07-unauthorized-disconnect-reconnects.json` | `unauthorized` (pre-welcome, `reconnect:false`) → NOT terminal; fresh-ticket retry — reason-level dispatch, not `reconnect:false`-level |
| 12 | `12-checkpoint-after-handoff.json` | per page: delivery strictly precedes that page's checkpoint save |
| 16 | `16-gap-410-accepted-resume.json` | 410 with accepting handler: invocation `{feedGap, accept}`, resume via the provided URL, stream continues |
| 17 | `17-remote-disconnect-remint.json` | `remote`/`reconnect:true` → re-mint + reconnect through existing Backoff transitions |
| 19 | `19-present-entry-buffered-lower-id.json` | admitted straggler N (proven via `expectBuffered`) delivered BEFORE the present-entry position saves |
| 20 | `20-present-entry-post-snapshot-straggler.json` | post-snapshot lower-id straggler delivered live + deduped; position never regresses |
| 21 | `21-overflow-default-terminal.json` | no handler: overflow → Terminal(`buffer_overflow`), no save, zero invocations |
| 22 | `22-overflow-accepted.json` | accepting handler: invocation recorded AND retained deliveries complete BEFORE the save — acceptance never bypasses retained deliveries |
| 23 | `23-gap-410-default-terminal.json` | no handler: 410 → Terminal(`feed_gap`), zero invocations; its exact-set `finally` is the direct test of any auto-continue mutant |
| 24 | `24-overflow-handler-terminate.json` | handler Terminate → Terminal(`buffer_overflow`), no save, invocation `{bufferOverflow, terminate}` — the branch distinct from no-handler |
| 25 | `25-gap-handler-terminate.json` | handler Terminate on 410 → Terminal(`feed_gap`), no save, invocation `{feedGap, terminate}` |
| 26 | `26-hostile-next-cross-origin.json` | cross-origin `next` mid-walk → Terminal(`invalid_continuation`), zero requests to the foreign origin |
| 27 | `27-hostile-resume-cross-origin.json` | accepted 410 with a cross-origin `resume` → Terminal(`invalid_continuation`), zero foreign requests |
| 28 | `28-checkpoint-load-failure.json` | store load Failed → Terminal(`checkpoint_load`) with ZERO wire attempts; distinct from Missing (which proceeds to a present entry) |
| 29 | `29-checkpoint-save-failure-continues.json` | save Failed → feed continues and a SUBSEQUENT save is attempted (exact store-call script: no save circuit breaker) |
| 30 | `30-continuation-redirect-cross-origin.json` | validated same-origin `next` answering 302 + cross-origin Location → Terminal(`invalid_continuation`), zero foreign egress |

**Hostile-URL coverage note (stated author's choice, per the PR-1 review):** the
downgrade (HTTPS→HTTP) variant is deliberately not a separate fixture here. Tier-2
harness base origins are localhost HTTP under the §9 carve-out, so a downgrade is
not reproducible at this seam; §23 routes continuation validation through §8's
shared Same-Origin Validation Algorithm plus downgrade rejection, whose downgrade
branch is already pinned by `conformance/tests/security.json` ("Protocol downgrade
in Link header rejected"). This family pins the two feed-specific URL sources as
cross-origin
(`next` → 26, `resume` → 27) plus the redirect hop (30).

## Fixture → server-behavior dependency table

Every row a fixture encodes about bc3, with its provenance class. Class 1 (wire
literals) freezes when bc3 regenerates transcripts at the rebased head; class 2
(semantic behavior) is re-verified row-by-row at the gate; SDK-owned rows are not
gated. Until the gate clears, **these fixtures are the only literal pins anywhere**
for the reason strings — bc3's own connection test asserts the protocol-fatal
reason via a constant, not the literal.

| Server behavior | Class | Pinned by |
|---|---|---|
| Disconnect reason literal `unauthorized` (arrives only pre-welcome) | 1 (+ 2 for the pre-welcome timing) | 07 |
| Disconnect reason literal `invalid_event_stream_command`, `reconnect:false` | 1 | 06 |
| Disconnect reason literal `remote`, `reconnect:true` | 1 — **no transcript capture exists**; source-verified against the pinned Rails; its freeze rides bc3's disconnect-matrix re-verification plus the one requested capture frame | 17 |
| Poll body envelope keys `events` / `position` / `next` | 1 | every fixture serving a 200 poll: 01, 02, 05, 07, 12, 16, 17, 19, 20, 22, 26, 29, 30 (mechanically derived from the fixture files; re-derive when the set changes) |
| Mint response body `{ticket, expires_in, url}`, status 200 | 1 | every fixture with `expectMint` (all but 28) |
| Subscribe identifier literals: channel `EventsChannel`, param spellings `types`/`buckets`/`creators`, comma-joined values | 1 | channel: every `expectSubscribe`; `types` spelling: 01 (its `expectSubscribe` pins `params` explicitly, single-valued); `buckets`/`creators` spellings + comma-joining: no PR-2 fixture — pinned at PR-4 (fixture 15, whose retransmit case also pins byte-identity of the identifier) |
| 409 body: all three keys `error` / `position_digest` / `filters_digest` required; digest values bare 16-hex (no `srv1-` prefix), `error` content unconstrained | 1 | schema-pinned shape only (the 409 respond variant requires all three keys); **no PR-2 fixture serves a 409** — pinned live at PR-4 (the tier-1 dispatch case additionally owns the wire pin when tier 1 lands) |
| 410 body keys `epoch_after_id` / `resume` | 1 | 16, 23, 25, 27 |
| 400 position-vs-filter discriminating bodies (verbatim transcript shapes) | 1 | no PR-2 fixture — pinned at PR-4 (tier 1 additionally owns it when `PollEvents` lands); the schema's 400 variant requires a verbatim body and this table is its source of truth |
| srv1 digest vectors (five-vector table) + canonicalization algorithm | 1 (vectors) / 2 (algorithm) | sibling family `conformance/event-feed-digest/` |
| Maximum inbound frame (`EVENT_FEED_MAX_FRAME_BYTES`, 1 MiB), transport-enforced during the read | SDK-owned constant; enforcement seam-contractual | no PR-2 fixture — tier 3 + a later raw-bounds fixture |
| Filter raw bounds: a filter list of > 1,000 elements or > 16 KB → filter 400 | 2 | unreachable through validated construction (the client caps at 100 ids); recorded, unpinned |
| `since=now` / bare entry mints the cursor at the newest visible id; an empty entry page positions above an in-flight lower id N | 2 | 19, 20 |
| Safety-horizon bound: position-relative, best-effort, ~30s — never wall-clock | 2 | premise of 19/20 (not directly assertable client-side; the entry-boundary fixtures encode its consequence) |
| Frozen-head `next` predicate: absent `next` = the walk reached its head | 2 | every fixture whose walk ends on a 200 page without `next`: 01, 02, 05, 07, 12, 16, 17, 19, 20, 22, 29 (mechanically derived; re-derive when the set changes) |
| 410 `resume` re-enters at `since=now` with the canonical filter set preserved | 2 | 16 (resume URL followed verbatim); 27 (hostile variant) |
| 400-position / 409 re-entry semantics (`since=<last poll-served id>`, present-class fallback) | 2 | no PR-2 fixture — pinned at PR-4 |
| Ticket statelessness + ~120s TTL (server-owned `expires_in`) | 2 | 05 (TTL-advance premise; `expires_in` never schedules anything) |
| 3-second server heartbeat cadence (input to the 7500ms staleness policy) | 2 | no PR-2 fixture — PR 4 (staleness fixture 08) |
| Subscribe retransmit contract (identical absorbed, different rejected) | 2 | no PR-2 fixture — PR 4 (fixture 15) |
| Push payload 9-key shape incl. the `visible_to_clients` presence asymmetry (push carries it, poll rows omit it) | 2 | schema-enforced on every `serve message` (9 keys) and every poll envelope row (8 keys, `visible_to_clients` forbidden) |

## Deliberate tier-2 slack (tier-3/PR-4 ownership)

Gaps stated rather than latent — each is deliberate, with a named owner:

- **400-body discrimination is unenforced in the schema** while the class-1 literal
  shapes are PROVISIONAL: the 400 respond variant accepts any non-empty object, so a
  fixture could ship a non-discriminating body. When the dependency-table 400 shapes
  freeze, the schema's 400 branch becomes a oneOf of the two literal discriminating
  shapes.
- **Malformed mint SUCCESS** (SPEC: `mint_failed` on "a malformed success") is
  inexpressible here — the 200 mint branch mandates the exact well-formed three-key
  body. Tier 3 owns it.
- **Start-entry modes** (beginning `since=0` / after-id `since=<id>`) have no config
  surface in tier 2, so their entry-query pins are unreachable here — tier-1 wire
  fixtures and PR 4 own them.
- **The counter-reset kill** (reset on the first successful poll page, never on
  `confirm_subscription`) lives in the PR-4 authorization block — see fixture 07's
  deferral note in the authoring manifest.

## PR-T true-up checklist (bc3 merge-time gate)

Run when bc3's event-feed branch merges; the pin moves and PROVISIONAL drops only
when every line is done:

1. Record the bc3 merge SHA; regenerate transcripts at the rebased head; diff every
   class-1 row above against them.
2. Re-verify every class-2 row against the rebased source and docs, row by row —
   transcript diffs cannot prove these.
3. `remote`: confirm the disconnect-matrix re-verification covers it and land the
   requested capture frame; only then does its row carry transcript provenance.
4. Re-verify the srv1 five-vector table at the rebased head (sibling digest family).
5. Re-confirm `CreateStreamTicket`'s `idempotent: true` (safe-to-retry sense) — flag
   if changed.
6. Update this README's banner SHA and the fixtures' provenance note; remove
   PROVISIONAL.
7. Any drifted row: fix fixtures, schema, and SPEC §23 together in the true-up PR —
   never fixture-only.

## Mutation kill matrix (fifteen)

Each mutation is shown red against at least one fixture in the reference
implementation PR's body before it counts.

| # | Mutation | Killed by |
|---|---|---|
| 1 | `reuse-old-url` (reconnect dials the previous mint's URL) | 05 |
| 2 | `deliver-before-confirm` | 02 |
| 3 | `start-catchup-before-confirm` | 02 |
| 4 | `leak-deadline-timer` (ghost watchdog survives teardown) | 03 |
| 5 | `reconnect-after-reject` | 04 |
| 6 | `checkpoint-before-handoff` (saves before the page's delivery) | 12 |
| 7 | `retry-into-protocol-fatal` | 06 |
| 8 | `save-present-position-before-buffer-drain` | 19 |
| 9 | `discard-live-id-at-or-below-position` | 20 |
| 10 | `checkpoint-after-silent-overflow` (saves despite the unhandled overflow) | 21 |
| 11 | `accept-overflow-then-save-before-retained-drain` (acceptance as license to skip retained deliveries) | 22 |
| 12 | `bypass-configured-handler` (handler registered but skipped; default-terminal applied) | 24, 25 (via `handlerInvocations` exact-set) |
| 13 | `follow-cross-origin-continuation` (skips §8 validation, polls the hostile URL) | 26, 27 |
| 14 | `collapse-load-error-to-missing` | 28 |
| 15 | `follow-cross-origin-redirect` (follows a 302 to a foreign Location) | 30 — **partially**, and the boundary is below the seam. See the note under this table. |

**Row 15 is the family's one partial kill, and the reason is structural.** In
tier 2 the poll lane is a SEAM: the driver receives the fixture's scripted 302
and hands the connector an already-formed redirect-refused verdict. The
connector never sees a `Location` header and never decides whether to follow
one, so the `follow-cross-origin-redirect` mutation lives **below** the seam
and no tier-2 harness can reach it. What fixture 30 does kill is the half above
the seam: a connector that mishandles the verdict — retrying it, classifying it
as anything but Terminal(`invalid_continuation`), or echoing more of the
`Location` than its origin — diverges on `finally` and fails.

An earlier revision of this row claimed a harness obligation to "bind the
foreign origin to a sentinel listener whose any-request fails the scenario".
That is withdrawn. No implementation met it, and meeting it would prove
nothing: the foreign origin is unreachable **by construction of the harness**,
because the harness is the seam, so a silent sentinel is a statement about the
driver rather than about the connector. Zero egress to a foreign redirect
target is a Layer-1 property, and its proof is the Layer-1 seam adapter's own
302 test, where a real generated `PollEvents` call meets a real redirect.
Tracked for G1b.

Auto-continue-past-unhandled-gap needs no separate mutation — fixture 23's
exact-set `finally` is its direct test. Fixture 29's exact store-call script is the
no-save-circuit-breaker pin, likewise not a numbered mutation.
