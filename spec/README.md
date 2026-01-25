# Spec Layout

This directory contains the canonical Basecamp spec used for SDK generation.
We start with a minimal Smithy model and layer overlays for behavior and
ergonomics.

## Files
- `basecamp.smithy` — canonical model (types + operations)
- `overlays/` — semantic overlays (pagination, retries, idempotency)

## Grounding
- API reference: `~/Work/basecamp/bc3-api/sections/`
- App code: `~/Work/basecamp/bc3/`

## Conventions
- Keep shapes minimal and accurate; prefer additive updates over churn.
- Behavior lives in overlays, not in operation docs.
- OpenAPI is derived output only.
