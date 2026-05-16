# Brief: Surface BC5 forward-compat fields through Go hand-written wrappers

**Status**: blocker for `basecamp-cli` Phase 1 absorption.
**Audience**: SDK dev agent working from `basecamp-5-is-releasing-eager-spring.md`.
**Source**: filed by `basecamp-cli` `five` branch after attempting Phase 1 against SDK commit `7e9c4345` (Go regen of BC5 spec additions).

## Summary

SDK PR 1 added BC5 forward-compat fields to the **generated** Go client
(`go/pkg/generated/client.gen.go`), but did not propagate them through the
**hand-written wrappers** in `go/pkg/basecamp/`. The wrappers are the
public Go API; CLI / external Go consumers never see the generated types
directly. As a result, the new fields are dropped during the
`*FromGenerated` conversion step and Phase 1 of the CLI absorption plan
cannot deliver any user-visible value.

The CLI's andon cord — *"if the SDK lacks a Go service wrapper for a
generated endpoint, never call the raw generated client from CLI code"* —
applies here in spirit: the wrapper exists, but it discards fields the
spec now declares. The fix lives in the SDK, not the CLI.

## Evidence

`generated.Todo` now has `Steps []CardStep` (per `7e9c4345`'s diff to
`client.gen.go:~1990`). The wrapper:

```go
// go/pkg/basecamp/todos.go:543
func todoFromGenerated(gt generated.Todo) Todo {
    t := Todo{
        Status:      gt.Status,
        Title:       gt.Title,
        // … no Steps assignment …
    }
    // …
    return t
}
```

`Todo` (`go/pkg/basecamp/todos.go:17`) has no `Steps` field. CLI builds
fail when referencing `todo.Steps` after `app.Account().Todos().Get(...)`.

The same gap exists for every Phase-1 surface the CLI plan calls out:

| Generated type (after PR 1) | New fields in `generated/client.gen.go` | Hand-written wrapper file | Wrapper exposes them? |
|---|---|---|---|
| `generated.Todo` | `Steps []CardStep` | `pkg/basecamp/todos.go` `Todo` (line 17) | **no** |
| `generated.Todoset` | `TodosCount`, `CompletedLooseTodosCount`, `TodosUrl`, `AppTodosUrl` | `pkg/basecamp/todosets.go` `Todoset` | **no** |
| `generated.Person` | `Tagline` (alongside existing `Bio`) | `pkg/basecamp/todos.go` `Person` (line 50) | **no** |
| `generated.Notification` | `BubbleUpUrl`, `BubbleUpAt` | `pkg/basecamp/my_notifications.go` `Notification` (line 13) | **no** |
| `generated.GetMyNotificationsResponseContent` | `BubbleUps []Notification`, `ScheduledBubbleUps []Notification` | `pkg/basecamp/my_notifications.go` `NotificationsResult` (line 38) | **no** |

(There are likely more — these are just the ones the CLI Phase 1 hits
first. A wider audit comparing every `*FromGenerated` function against
its generated source after `7e9c4345` would be welcome.)

## Why this happened

The SDK plan §1 ("Smithy spec — forward-compat additions") prescribes
the spec edits and PR 1 ships their regenerated client code. There is no
explicit step in the plan that says *"after regen, propagate new fields
through the hand-written `*FromGenerated` wrappers."* Implicit because
the Go SDK is the only one with this layer, easy to miss when the rest
of the SDK family is purely generated.

`SPEC.md` documents the architecture ("Go demonstrates the hand-written
service wrapper pattern"), and `CONTRIBUTING.md` mentions the
`go-check-drift` target — that target verifies *all generated operations
are covered by hand-written services*, but does **not** verify that
every generated *field* on covered structures is propagated. The current
drift check is operation-level, not field-level.

## Contract / acceptance criteria

For each generated field added in PR 1, do exactly one of:

1. **Add a corresponding field on the wrapper struct** with the same
   semantic meaning, JSON tag matching the wire format, and propagate
   it inside the relevant `*FromGenerated` conversion. Default to this
   for fields the CLI / external consumers will want to read.
2. **Document the omission inline** with a `// intentionally not
   surfaced because <reason>` comment on the wrapper struct, if a field
   is genuinely not appropriate for the public Go surface (e.g. an
   internal echo). Phase 1 fields should not need this — they're all
   user-visible.

Per-type concrete proposals (signatures, no behavior change for
existing fields):

```go
// pkg/basecamp/todos.go — Todo
type Todo struct {
    // … existing fields …
    Steps []CardStep `json:"steps,omitempty"`
}

func todoFromGenerated(gt generated.Todo) Todo {
    t := Todo{ /* existing assignments */ }
    if len(gt.Steps) > 0 {
        t.Steps = make([]CardStep, 0, len(gt.Steps))
        for _, gs := range gt.Steps {
            t.Steps = append(t.Steps, cardStepFromGenerated(gs))
        }
    }
    return t
}
```

```go
// pkg/basecamp/todosets.go — Todoset
type Todoset struct {
    // … existing fields …
    TodosCount               int    `json:"todos_count"`                 // BC5
    CompletedLooseTodosCount int    `json:"completed_loose_todos_count"` // BC5
    TodosURL                 string `json:"todos_url,omitempty"`         // BC5
    AppTodosURL              string `json:"app_todos_url,omitempty"`     // BC5
}
```

```go
// pkg/basecamp/todos.go — Person
type Person struct {
    // … existing fields, including Bio …
    Tagline string `json:"tagline,omitempty"` // BC5; alias of Bio per spec note
}
```

```go
// pkg/basecamp/my_notifications.go
type Notification struct {
    // … existing fields …
    BubbleUpURL string    `json:"bubble_up_url,omitempty"` // BC5
    BubbleUpAt  time.Time `json:"bubble_up_at,omitempty"`  // BC5
}

type NotificationsResult struct {
    // … existing fields, including Memories …
    BubbleUps          []Notification `json:"bubble_ups,omitempty"`           // BC5
    ScheduledBubbleUps []Notification `json:"scheduled_bubble_ups,omitempty"` // BC5
}
```

For `Notification.BubbleUpAt`, follow the same `time.Time` zero-value
pattern the wrapper already uses for `ReadAt` / `UnreadAt` — the CLI
checks `IsZero()` on those today.

## Verification

A field-level drift check would prevent recurrence. Sketch:

- Walk every wrapper struct in `pkg/basecamp/*.go`.
- Locate the corresponding generated struct (by either explicit
  conversion func or by name match).
- For each generated field, fail if the wrapper has no field with a
  matching JSON tag *and* no `// intentionally-omitted: ...` marker on
  the wrapper struct.

This is a separate hardening PR; not blocking the immediate wrapper
update.

For the immediate PR, manual verification is enough:

```
go build ./...                         # cli & sdk both clean
go test ./pkg/basecamp/...             # existing wrapper tests pass
make go-check-drift                    # current drift check passes
```

Then the CLI side runs:

```
make bump-sdk REF=<wrapper-PR-merge-commit>
go build ./...                         # cli compiles with new fields
go test ./...                          # cli tests still pass
```

## Out of scope for this brief

- Other-language SDK behavior. TS / Ruby / Python / Swift / Kotlin
  consume generated types directly (per `AGENTS.md`: "No hand-written
  API methods exist in any SDK runtime"); those SDKs picked up the new
  fields automatically when their generators ran.
- Spec changes. Spec is correct; this is purely a Go-layer omission.
- Recording.Bubbleupable. Per the CLI plan, that field tracks Phase 3e
  (`recording-bubbleupable-field` brief, currently `no-json-contract`)
  rather than Phase 1.

## Why now

`basecamp-cli` Phase 1 is the first downstream consumer to attempt
absorbing the BC5 forward-compat fields. The Phase 1 work is presenter-only
on paper and could ship in a single commit per surface — but every
surface is gated on the wrapper exposing the field. CLI work will resume
the moment a wrapper-update PR lands; the bump-sdk → presenter changes
chain is well understood.
