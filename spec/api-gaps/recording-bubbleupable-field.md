---
gap: recording-bubbleupable-field
status: no-json-contract
detected: 2026-05-01
sdk_demand: low
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3e
  routes:
    - "(no new routes — additive field on existing Recording-shaped responses)"
  controllers: []
  related_existing_api:
    - "Recording (polymorphic shape used by GetRecording, search results, activity feeds)"
---

# Recording#bubbleupable? field

## What's missing

BC5 adds a `bubbleupable` (or `bubbleupable?`) boolean field to the
Recording envelope to indicate whether the current user can bubble-up the
recording. The BC3 plan Phase 3e covers this addition along with any other
new envelope fields.

## Why it matters

Without the field, SDK consumers can't pre-compute the eligibility of UI
affordances ("Save to Bubble Up" button) without a separate roundtrip.
Bubble-up eligibility is per-user, per-recording, and not derivable
client-side from the recording type alone.

## Suggested API shape

Additive boolean field on every Recording-shaped response:
```json
{ ..., "bubbleupable": true }
```

If BC3 surfaces additional envelope fields in the same phase, document each
here as they're confirmed.

## Implementation notes for BC3

- Add to the shared `_recording.json.jbuilder` partial.
- Document in `doc/api/sections/recordings.md`.
- Verify behavior against the `Recording#bubbleupable?` predicate in the
  Rails model.

## SDK absorption plan when this lands

- Extend the Smithy `Recording` shape (or whichever shape carries the
  envelope fields — `Notification`, `MyAssignment`, etc.) with an optional
  `bubbleupable: Boolean` field.
- No new service registrations.
- Canary: extras-observed reporting will surface the field automatically
  during the first live run after BC5 ships; the absorption PR adds it to
  the spec.
