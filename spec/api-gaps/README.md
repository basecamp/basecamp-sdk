# API Gap Registry

Each markdown file in this directory describes a BC5 user-visible feature or
contract that ships without (or with incomplete) JSON API coverage. The
registry is the SDK side of the [SDK ↔ BC3 coordination](../../COORDINATION.md):
the BC3 parity plan owns server-side delivery; entries here track each gap
from detection through absorption. Status changes flow through git history,
making the absorption journey publicly auditable.

## Lifecycle

1. **Detect**: a gap is identified — by the API gap detector
   (`make detect-api-gaps`), by editorial review of the BC3 parity plan, or
   by an SDK consumer raising a request. A starter entry gets generated or
   authored.
2. **Address**: BC3 ships a JSON API contract for the gap. Entry frontmatter
   updates to `addressed-in-bc3-pr-N`.
3. **Absorb**: SDK opens a follow-up PR adding the Smithy operations and
   regenerated SDK code. Frontmatter updates to `absorbed-in-sdk` with
   Smithy structure refs.
4. **Archive**: entries more than a year past `absorbed-in-sdk` may be moved
   to `archive/` for tidiness; they remain readable as historical record.

## Statuses

| Status | Meaning |
|---|---|
| `no-json-contract` | Detected gap; no JSON API exists yet. |
| `partial-coverage` | Some elements exist (partial, render path) but doc and/or Smithy are missing. |
| `ambiguous` | BC3 has not yet classified whether this is API-shaped or UI-only. |
| `confirmed-not-api-resource` | BC3 confirmed UI-only / not part of the API surface; entry retained as classification record. |
| `addressed-in-bc3-pr-N` | BC3 has shipped a JSON API contract; SDK absorption pending. |
| `absorbed-in-sdk` | SDK has absorbed the contract via Smithy + regenerated code. |

## Entries (current)

| Gap | Status | BC3 plan phase | SDK demand |
|---|---|---|---|
| [calendar](calendar.md) | no-json-contract | 3b | medium |
| [scratchpad](scratchpad.md) | no-json-contract | 3b | medium |
| [step-top-level](step-top-level.md) | partial-coverage | 3b | low |
| [everything-aggregates](everything-aggregates.md) | no-json-contract | 3c | high |
| [activity-timeline](activity-timeline.md) | no-json-contract | 3d | high |
| [recordable-subtypes-doc](recordable-subtypes-doc.md) | partial-coverage | 3a | medium |
| [stack-doc-and-smithy](stack-doc-and-smithy.md) | partial-coverage | 3b | medium |
| [search-filter-additions](search-filter-additions.md) | no-json-contract | 3e | medium |
| [rich-text-project-attachable](rich-text-project-attachable.md) | no-json-contract | 3e | low |
| [recording-bubbleupable-field](recording-bubbleupable-field.md) | no-json-contract | 3e | low |
| [todoset-completed-list-visibility](todoset-completed-list-visibility.md) | ambiguous | 3a | low |

The detector also maintains [`allowlist.yml`](allowlist.yml) for routes
classified as not-an-API-resource or absorbed under another entry. Allowlist
records are lighter-weight than entries and serve a different purpose:
entries preserve the *investigation history* of candidates that warranted
SDK-side review; allowlist records cover routes that should never have
warranted an entry in the first place. Pick one per candidate, never both.

## Validating

```
make validate-api-gaps
```

Validates frontmatter on every entry against [`schema.json`](schema.json)
and the allowlist against [`allowlist-schema.json`](allowlist-schema.json).
Wired into `make check`.

## Detecting new gaps (planned)

Today, entries are added by hand when a gap is identified. Automated
detection — diffing routes between BC3 master and the active branch,
classifying each new route against multi-signal heuristics, and emitting
starter entries for human review — will arrive in a later PR. The intended
invocation will be:

```
BC3_REPO_PATH=~/Work/basecamp/bc3 make detect-api-gaps
```

The `detect-api-gaps` Make target does not yet exist; running this now will
error.
