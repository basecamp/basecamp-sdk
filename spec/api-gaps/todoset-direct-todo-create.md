---
gap: todoset-direct-todo-create
status: addressed-in-bc3-pr-12359
detected: 2026-07-22
sdk_demand: medium
bc3_pr: 12359
bc3_refs:
  introduced_in: five
  routes:
    - "POST /:account_id/buckets/:project_id/todosets/:todoset_id/todos.json"
  controllers:
    - app/controllers/todos_controller.rb
  related_existing_api:
    - CreateTodo
    - GetTodoset
---

# To-do set — create a to-do directly (outside any list)

## What's missing

SDK absorption only — the contract shipped via BC3 **#12359** (merged
2026-07-22, post-train follow-up to the BC5 API docs). "Create a to-do" in
`doc/api/sections/todos.md` on `master` now documents a second create form:

- `POST /buckets/:project_id/todosets/:todoset_id/todos.json` creates a
  to-do **directly under the to-do set**, outside of any to-do list.
- **This form is only available project-scoped** — there is no account-scoped
  variant (unlike most BC5 canonical routes).
- Parameters and response match the existing to-do-list create: required
  `content`, optional `description`/`assignee_ids`/`completion_subscriber_ids`/
  `notify`/`due_on`/`starts_on`, returns `201 Created` with the Todo payload.
- Find a project's to-do set via the existing Get to-do set endpoint
  (`GetTodoset` in the SDK).

## Why it matters

Loose to-dos (to-dos that live directly on the to-do set rather than in a
list) are a BC5 feature. SDK consumers can see them in list/read surfaces
but can't create them — the modeled `CreateTodo` requires a to-do list id,
so integrations that mirror BC5's "add a to-do without picking a list" flow
have no API path.

## Suggested API shape

`CreateTodosetTodo` operation:
`POST /{accountId}/buckets/{projectId}/todosets/{todosetId}/todos.json`,
reusing `CreateTodo`'s payload members and the `Todo` output shape. `201`
with the created Todo.

## Implementation notes for BC3

Shipped — nothing pending. Routed via `resources :todosets do resources
:todos, only: %i[index create]` in the bucket scope, served by
`todos_controller.rb`; the doc documents the create form.

## SDK absorption plan when this lands

- Vehicle: the §Q absorption queue's **PR-2 build-ahead pair**
  (`UpdateCampfireLine` + `CreateTodosetTodo`), pre-approved.
- New Smithy operation `CreateTodosetTodo` sharing payload/output shapes
  with `CreateTodo`; registers on the existing Todos (or Todosets) service.
- Status flips to `absorbed-in-sdk` with the absorption PR (which adds the
  Smithy refs).
- Pairwise check: BC4 has no loose to-dos — expect 404 on BC4, 201 on BC5;
  additive-only.
