---
gap: vault-view-preference
status: addressed-in-bc3-pr-13372
detected: 2026-10-19
sdk_demand: low
bc3_pr: 13372
bc3_refs:
  introduced_in: "Let API clients set a folder's Docs & Files view (BC3 #13372, 1b5ac5ea73)"
  routes:
    - "PUT /:account_id/vaults/:vault_id/view_preference.json (canonical)"
    - "PUT /:account_id/buckets/:bucket_id/vaults/:vault_id/view_preference.json (legacy alias)"
  related_existing_api:
    - GetVault
    - UpdateVault
---

# Set a vault's Docs & Files view

## What's missing

BC3 #13372 lets API clients set how the current user sees one vault in Docs &
Files: `PUT /vaults/{vaultId}/view_preference.json` with `view` of `grid` or
`list`, answering `204`, or `400` for any other value. The view belongs to the
caller and to that one vault. The SDK models no operation for it.

## Why it matters

It is a per-person display setting. Low demand: no known SDK consumer needs it.

## Suggested API shape

`UpdateVaultViewPreference` (`PUT /vaults/{vaultId}/view_preference.json`),
`view` as an enum of `grid` and `list`, `204` with no content, and the standard
error set with `ValidationError` for the `400`. Model the flat form only.

## Implementation notes for BC3

Nothing further.

## SDK absorption plan when this lands

A spec PR adds the operation, regenerates, and adds the per-SDK tests. Its
allowlist entry in `spec/bc3-route-allowlist.yml` is deleted in the same PR.
