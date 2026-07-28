---
gap: everything-tasks-aggregate
status: confirmed-not-api-resource
detected: 2026-07-28
sdk_demand: none
bc3_refs:
  introduced_in: five
  bc3_plan_section: "Not in API scope — web-only 'All tasks' page (bc3 e4bf93c3c2 + f697d8604d @ dffa7e11b3); HTML/Turbo Frame controllers, no respond_to :json, no jbuilder, no doc/api section"
  routes:
    - "GET /:account_id/tasks → everything/assignments#index (HTML only)"
    - "GET /:account_id/tasks/results → everything/assignments/results#index (Turbo Frame, HTML only)"
  controllers:
    - "Everything::AssignmentsController"
    - "Everything::Assignments::ResultsController"
  related_existing_api:
    - "GetEverythingOpenTodos / GetEverythingOverdueTodos / … (the modeled Everything JSON aggregates)"
    - "GetMyAssignments / GetMyDueAssignments / GetMyCompletedAssignments (my/assignments.json — unchanged)"
---

# Everything "tasks" — a web view, not an API resource

## What's missing

Nothing. This entry exists to record a **negative** result, so the same routes
don't get re-flagged at the next provenance sync.

BC3 gained routes under the `scope module: :everything` block — the same block
that produces the modeled `/todos/open.json` and `/cards/completed.json`:

```ruby
get "tasks" => "assignments#index", as: :assignments
namespace :assignments, path: "tasks" do
  get "results" => "results#index"
end
```

Sitting in that block makes them *look* like siblings of the JSON aggregates.
They are not. At the pinned `dffa7e11b3` both controllers are HTML-only:

```ruby
class Everything::AssignmentsController < ApplicationController
  include Everything::Assignments::Filters
  def index
  end
end

class Everything::Assignments::ResultsController < ApplicationController
  include Assignments::SlicedBuckets, Everything::Assignments::Filters
  layout "turbo_rails/frame"
  def index
    @assignments = Everything::Assignments.new(Current.person, **assignment_filter_params)
    @page = sliced_buckets @assignments.recordings, buckets: @assignments.sorted_buckets, viewer: Current.person
  end
end
```

- no `respond_to`, no JSON branch;
- every view under `app/views/everything/assignments/` is `.html.erb`;
- no `.json.jbuilder` template anywhere;
- `results` declares `layout "turbo_rails/frame"` — it exists to serve a Turbo
  Frame for the filter UI;
- nothing in `doc/api/`.

**A Rails route that tolerates a `.json` suffix does not create a JSON
representation.** Requesting these with `.json` gets the HTML template or a
missing-template error, not a payload.

## Why it matters

Only as a guard against a false positive. The provenance-drift triage from
`c308693171` to `dffa7e11b3` surfaced two commits — `e4bf93c3c2` added
`everything/assignments`, `f697d8604d` renamed it to `everything/tasks` — and
the naming makes them read as an unmodeled API aggregate. They are the
web-only "All tasks" page.

**This is also not a break of `my/assignments`.** The commit subject "Move
/assignments to /tasks" reads alarming against an SDK that ships
`GetMyAssignments`, but the moved routes are the Everything ones. The SDK's
assignment operations use `/:account_id/my/assignments.json` and its
`completed`/`due` variants, which live under a different scope and are
untouched across the whole drift range — `git diff c308693171..dffa7e11b3 --
config/routes.rb` contains only the two `tasks` additions.

## Suggested API shape

None. Deliberately not proposed: there is no captured response to model, and
sketching one would invite modeling a payload that does not exist. If an
account-wide assignments JSON aggregate is ever wanted, it needs a BC3-side
JSON representation first — at which point this entry should be reopened as a
real gap with a shape read off an actual response.

## Implementation notes for BC3

None required. The routes serve the web UI as intended.

## SDK absorption plan when this lands

Not applicable — there is nothing to absorb. Reopen this entry only if BC3 adds
a JSON representation for these controllers; until then the correct SDK action
is to do nothing.
