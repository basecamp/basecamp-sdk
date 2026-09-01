---
gap: delegated-events-performed-by
status: addressed-in-bc3-pr-13040
detected: 2026-08-31
sdk_demand: low
bc3_pr: 13040
bc3_refs:
  introduced_in: "Port delegated-events API docs back from the bc-api mirror (BC3 #13040; first documented in bc-api #436)"
  routes:
    - "GET /:account_id/buckets/:bucket_id/recordings/:recording_id/events.json (existing — field additive)"
  related_existing_api:
    - Event
    - ListEvents
    - CreateWebhook
---

# Delegated events — the `performed_by` person

## What's missing

Events and webhook payloads for actions an agent executed on someone's behalf
carry a `performed_by` member: a person object in the same shape as `creator`,
with `personable_type: "Agent"` (or `"Tombstone"` once the agent is deleted).
`creator` remains the person the action is attributed to; `performed_by`
identifies the agent that carried it out, and events for directly performed
actions omit the field entirely. The contract was documented in the public
mirror by bc-api #436 and ported back into `doc/api/sections/events.md` and
`webhooks.md` by BC3 #13040, making it part of the SDK's grounding surface.

The SDK's `Event` structure has no `performed_by` member, and the webhook
payload documentation note is likewise unmodeled.

## Why it matters

The field's presence is the durable signal that an action was performed by an
agent — exactly the provenance an SDK consumer auditing delegated activity
needs. Today the member survives only in decoders that keep unknown fields;
typed surfaces drop it.

## Suggested API shape

Add an optional `performed_by: Person` member to `Event` (and to the webhook
payload shape, which shares the envelope), documented as present only on
delegated actions. Purely additive; no new operations.

## Implementation notes for BC3

Already shipped and documented upstream — the events partial has emitted
`performed_by` since agent delegation landed; #13040 only restored the public
docs to `doc/api/`, keeping it the mirror's superset source of truth.

## SDK absorption plan when this lands

A follow-up spec PR adds the optional member, regenerates all six SDKs, and
extends an events fixture with a delegated entry so the coverage gates hold
the shape. No service or route changes.
