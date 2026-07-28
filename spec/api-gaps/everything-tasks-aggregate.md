---
gap: everything-tasks-aggregate
status: partial-coverage
detected: 2026-07-28
sdk_demand: medium
bc3_refs:
  introduced_in: five
  routes:
    - "GET /:account_id/tasks.json → everything/assignments#index"
    - "GET /:account_id/tasks/results.json → everything/assignments/results#index"
  controllers:
    - "Everything::AssignmentsController"
    - "Everything::Assignments::ResultsController"
  related_existing_api:
    - "GetEverythingOpenTodos / GetEverythingOverdueTodos / … (the modeled Everything aggregates)"
    - "GetMyAssignments / GetMyDueAssignments / GetMyCompletedAssignments (my/assignments.json — unchanged)"
---

# Everything "tasks" aggregate

## What's missing

BC3 gained a new account-wide aggregate under the `scope module: :everything`
block — the same block that produces the already-modeled `/todos/open.json`,
`/cards/completed.json`, and friends:

```ruby
get "tasks" => "assignments#index", as: :assignments
namespace :assignments, path: "tasks" do
  get "results" => "results#index"
end
```

Neither `GET /:account_id/tasks.json` nor `GET /:account_id/tasks/results.json`
is modeled in `spec/basecamp.smithy`.

## Provenance

Surfaced while triaging drift from `c308693171` to `dffa7e11b3` for the #477
repin. Two commits are involved and the net effect is one new aggregate, not a
rename of anything the SDK already ships:

- `e4bf93c3c2` "All tasks & Everything::Assignments" **added**
  `get "assignments" => "assignments#index"` plus an `assignments/results`
  namespace.
- `f697d8604d` "Move /assignments to /tasks" then **renamed the new routes**
  to `/tasks` before they ever appeared under a provenance pin.

## This is NOT a break of `my/assignments`

Worth stating explicitly, because the commit subject "Move /assignments to
/tasks" reads alarming against an SDK that ships `GetMyAssignments`. The moved
routes are the **Everything** ones inside `scope module: :everything`. The
SDK's assignment operations use `/:account_id/my/assignments.json` and its
`completed`/`due` variants, which live under a different scope and are
untouched across the whole drift range (`git diff c308693171..dffa7e11b3 --
config/routes.rb` shows only the two `tasks` additions).

## Why it matters

The Everything aggregates are how a cross-project consumer avoids enumerating
projects. `tasks` is the assignment-shaped member of that family, and its
absence means the only account-wide assignment view the SDK exposes is the
*current user's* (`my/assignments`), not the account's.

## Suggested API shape

Follows the sibling aggregates exactly — same flat account-scoped path, same
pagination:

```
GET /:account_id/tasks.json
GET /:account_id/tasks/results.json
```

```json
[
  {
    "id": 1069479520,
    "type": "Todo",
    "title": "Ship the thing",
    "due_on": "2026-08-01",
    "assignees": [ { "id": 1049715914, "name": "Victor Cooper" } ],
    "bucket": { "id": 2085958499, "name": "The Leto Laptop", "type": "Project" },
    "url": "https://3.basecampapi.com/195539477/buckets/2085958499/todos/1069479520.json",
    "app_url": "https://3.basecamp.com/195539477/buckets/2085958499/todos/1069479520"
  }
]
```

The element shape above is **provisional** — it is the union the controller
appears to span (Todos and Kanban::Cards, plus their Steps), not a shape read
off a captured response. Pin it before modeling.

## Implementation notes for BC3

Nothing is required of BC3 — the routes exist and are serving. This entry
records that the SDK has not caught up, not that the API is incomplete.

If anything, a doc entry would help: the sibling Everything aggregates are
documented under `doc/api/`, and `tasks` currently is not, which is part of why
it went unnoticed through a pin bump.

## SDK absorption plan when this lands

1. Pin the response shape. `Everything::AssignmentsController#index` renders an
   assignment projection; its element type has to be checked against the
   existing `Todo` and `Kanban::Card` structures before a Smithy shape is
   written, since an aggregate spanning both may need a polymorphic projection
   in the style of `Recording`/`SearchResult` rather than a new concrete type.
2. Decide whether `tasks/results` is a distinct paginated resource or a filtered
   view of the same collection — that is the difference between one operation
   and two.
3. Model the pagination and filter parameters against the sibling aggregates,
   which take `page` plus type-specific filters.
4. Follow the Everything naming convention already in the spec:
   `GetEverythingTasks` (and `GetEverythingTaskResults` if it is a second
   operation), landing on the `everything` service alongside
   `GetEverythingOpenTodos` and friends.
5. Regenerate all six SDKs and add a fixture plus a manifest target, since the
   aggregate returns a list shape the fixture-coverage guard will want covered.
