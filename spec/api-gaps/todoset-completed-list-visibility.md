---
gap: todoset-completed-list-visibility
status: ambiguous
detected: 2026-05-01
sdk_demand: low
bc3_refs:
  introduced_in: five
  bc3_plan_phase: 3a
  routes:
    - "todosets/completed_list_visibility (verb pending — likely PUT)"
  controllers:
    - app/controllers/todosets_controller.rb
  related_existing_api:
    - GetTodoset
---

# Todoset — completed-list visibility toggle

## What's missing

A new route `todosets/completed_list_visibility` was detected on
`origin/five` and is likely a PUT endpoint to show/hide the completed-todos
list at the todoset level (matches the existing UI toggle).

**This entry is currently `ambiguous`**: the BC3 plan has not classified
whether this is an API-shaped resource or UI-only state. Classification
must happen on the BC3 side (the SDK can't determine the right answer
unilaterally); the brief stays in the registry either way:

- If JSON-API: SDK absorbs as `UpdateTodosetCompletedListVisibility` op,
  this entry flips to `no-json-contract` and the absorption PR ships once
  BC3 lands the contract.
- If UI-only: this entry flips to `confirmed-not-api-resource` and stays
  as a record (per the entry-vs-allowlist rule — registry entries are
  durable records for candidates that warranted SDK-side investigation,
  regardless of final classification).

## Why it matters

If this is an API-shaped toggle and the SDK doesn't model it, integrations
can't replicate the in-app behavior. If it's UI-only, modelling it would
introduce a no-op endpoint — worse than missing it.

The right answer comes from BC3, not from the SDK. Hence the brief.

## Suggested API shape (if API-shaped)

`PUT /:account_id/todosets/:id/completed_list_visibility.json`:
- Input: `visible` (boolean)
- Returns 204 or the updated Todoset.

## Implementation notes for BC3

- Confirm whether the route is reached only by the web form (CSRF-token,
  session-cookie auth) or also by API auth.
- If API-shaped: add JSON branch + jbuilder + doc entry.
- If UI-only: explicit note in BC3 plan would close this brief.

## SDK absorption plan when this lands

- If API-shaped: new operation `UpdateTodosetCompletedListVisibility` on
  the existing `TodosetsService`.
- If UI-only: this brief stays at `confirmed-not-api-resource` as the
  record of the classification.
