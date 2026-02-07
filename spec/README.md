# Spec Layout

This directory contains the canonical Basecamp spec used for SDK generation.
We start with a minimal Smithy model and layer overlays for behavior and
ergonomics.

## Files
- `basecamp.smithy` — canonical model (types + operations)
- `overlays/` — semantic overlays (pagination, retries, idempotency)
- `api-provenance.json` — upstream revision tracking (bc3-api + bc3 SHAs)

## Grounding
- API reference: `basecamp/bc3-api` → `sections/`
- App code: `basecamp/bc3` → `app/controllers/`

## Conventions
- Keep shapes minimal and accurate; prefer additive updates over churn.
- Behavior lives in overlays, not in operation docs.
- OpenAPI is derived output only.
