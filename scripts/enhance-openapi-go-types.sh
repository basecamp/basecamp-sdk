#!/usr/bin/env bash
#
# Enhance OpenAPI spec with Go-specific type extensions for oapi-codegen.
#
# Type mappings:
#   - _at fields (created_at, updated_at, etc.) → time.Time (full timestamps)
#   - ScheduleEntry starts_at/ends_at → types.FlexibleTime (handles date-only for all-day events)
#   - _on fields (due_on, starts_on, etc.) → types.Date (date-only)
#   - width/height fields → types.FlexInt (accepts float-encoded integers from API)
#
# Optional-pointer policy (SPEC.md §10): optional fields are pointers — the
# oapi-codegen default, no prefer-skip-optional-pointer — because a value type
# cannot represent absence. The single x-go-type-skip-optional-pointer pass
# below keeps optional NON-NULLABLE ARRAYS on response-shaped schemas as native
# []T (nil already represents absence; *[]T would be churn with no semantic
# gain). Request-shaped arrays stay pointers so an explicit empty array is
# sendable (omitempty drops a len-0 slice), and nullable arrays stay pointers
# to distinguish present-null. make go-check-optional-pointers guards the
# generated output.
#
# Usage: ./enhance-openapi-go-types.sh [input.json] [output.json]
#        ./enhance-openapi-go-types.sh               # defaults to openapi.json in-place

set -euo pipefail

# EDITING NOTE: the jq programs below are single-quoted shell strings. An
# apostrophe anywhere inside them — including in a comment, e.g. "the pass's
# condition" — terminates the string and produces a confusing shell syntax
# error (or, worse, a jq compile error that leaves the output unenhanced).
# Write comments without apostrophes. `bash -n` catches it; so does running
# this script and checking its exit status, which is the only reliable signal.
#
# SECOND EDITING NOTE: in jq, `index(.)` and friends evaluate their argument
# against the value being indexed, NOT against the surrounding element. Inside
# `map(...)` or after `to_entries[]`, always bind first — `. as $x | select($arr
# | index($x))`. This has silently broken membership tests in this file more
# than once; the symptom is a filter that quietly matches nothing.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

INPUT_FILE="${1:-$PROJECT_ROOT/openapi.json}"
OUTPUT_FILE="${2:-$INPUT_FILE}"

if [[ ! -f "$INPUT_FILE" ]]; then
    echo "Error: Input file not found: $INPUT_FILE" >&2
    exit 1
fi

# Compute the request-body reachability closure ONCE, here, and hand it to both
# consumers via --argjson: the enhancement pass below and its self-check further
# down. The two used to inline their own copies, which had already drifted apart
# in formatting — and a self-check computing a different closure than the pass
# it validates is worse than no self-check.
REQUEST_REACHABLE=$(jq -c '
  ( .components.schemas ) as $all
  # Every $ref reachable from an operation requestBody. Two forms are handled:
  # a direct schema ref, and the reusable-component indirection
  # (#/components/requestBodies/X), whose component is resolved and its own
  # schema refs collected. An unused component contributes nothing, which is
  # the correct outcome rather than an error.
  | ( .components.requestBodies // {} ) as $bodies
  | ( [ .paths[]?[]? | objects | .requestBody? | objects
        | [.. | objects | select(has("$ref")) | .["$ref"]] | .[]
        | select(type == "string") ] | unique ) as $initial_refs
  # Follow requestBodies refs TRANSITIVELY: a component may alias another
  # component, and stopping after one hop silently drops the schemas behind the
  # chain, marking their arrays response-only.
  | ( { seen: [], frontier: $initial_refs }
      | until(.frontier | length == 0;
          . as $s
          | ( $s.seen + $s.frontier | unique ) as $seen_next
          | ( [ $s.frontier[] | select(startswith("#/components/requestBodies/"))
                | sub("^#/components/requestBodies/"; "") as $n
                | ($bodies[$n] // {})
                | [.. | objects | select(has("$ref")) | .["$ref"]] | .[]
                | select(type == "string") ] | unique ) as $discovered
          # Bind the element: index(.) inside map would evaluate . against the
          # ARRAY being indexed, not against the element under test.
          | { seen: $seen_next,
              frontier: ( $discovered | map(. as $r | select($seen_next | index($r) | not)) ) }
        )
      | .seen ) as $all_body_refs
  | ( [ $all_body_refs[] | select(startswith("#/components/schemas/"))
        | sub("^#/components/schemas/"; "") ] | unique ) as $seeds
  | ( { seen: ($seeds | map({key: ., value: true}) | from_entries), frontier: $seeds }
      | until(.frontier | length == 0;
          . as $s
          | ( [ $s.frontier[] | ($all[.] // {}) | [.. | objects | select(has("$ref")) | .["$ref"]] ]
              | flatten
              | map(select(type == "string" and startswith("#/components/schemas/"))
                    | sub("^#/components/schemas/"; ""))
              | unique ) as $next
          | ( $next | map(select($s.seen[.] | not)) ) as $new
          | { seen: ($s.seen + ($new | map({key: ., value: true}) | from_entries)), frontier: $new }
        )
      | .seen ) | keys
' "$INPUT_FILE")

# An empty closure is only suspicious when some request body actually REFERENCES
# a component. Three shapes legitimately reach nothing and must not abort:
# a spec with no request bodies, one whose requestBodies components are all
# unreferenced, and one whose bodies carry INLINE schemas (no $ref at all).
BODY_COMPONENT_REFS=$(jq -r '
  [ .paths[]?[]? | objects | .requestBody? | objects
    | [.. | objects | select(has("$ref")) | .["$ref"]] | .[]
    | select(type == "string" and startswith("#/components/")) ] | length' "$INPUT_FILE")

if [[ "$BODY_COMPONENT_REFS" -gt 0 && ( -z "$REQUEST_REACHABLE" || "$REQUEST_REACHABLE" == "[]" ) ]]; then
    echo "Error: request bodies carry $BODY_COMPONENT_REFS component reference(s) but the reachability closure is empty — the walker is broken." >&2
    exit 1
fi

jq --argjson request_reachable "$REQUEST_REACHABLE" '
# normalize_deprecation_reason strips exactly one leading "Deprecated:"
# (case-insensitive) plus following whitespace and trims, so oapi-codegen can
# prepend its own "// Deprecated: " once instead of producing a doubled
# "// Deprecated: Deprecated: ..." marker. Mirrors the resolver rule shared by
# every SDK generator (see scripts/check-deprecation-parity).
def normalize_deprecation_reason:
  (. // "")
  | sub("^\\s*Deprecated:\\s*"; ""; "i")
  | sub("^\\s+"; "")
  | sub("\\s+$"; "");

# mark_deprecated_query_param hoists deprecated + normalized x-deprecated-reason
# onto a single query parameter object (a no-op for non-deprecated params).
def mark_deprecated_query_param:
  if (.in == "query") and ((.deprecated == true) or (.schema.deprecated == true)) then
    .deprecated = true
    | .["x-deprecated-reason"] = ((.description // .schema.description // "deprecated") | normalize_deprecation_reason)
  else . end;

# First pass: add x-go-type extensions for timestamps and dates. No
# skip-optional-pointer: optional temporal fields are *time.Time/*types.Date
# like every other optional value type (IsZero() is a zero-value sentinel,
# not a representation of absence).
walk(
  if type == "object" then
    to_entries | map(
      # Timestamp fields (_at): use time.Time
      if (.key | test("_at$")) and (.value | type == "object") and (.value.type == "string") then
        .value += {
          "x-go-type": "time.Time",
          "x-go-type-import": {"path": "time"}
        }
      # Date-only fields (_on): use types.Date
      elif (.key | test("_on$")) and (.value | type == "object") and (.value.type == "string") then
        .value += {
          "x-go-type": "types.Date",
          "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
        }
      else
        .
      end
    ) | from_entries
  else
    .
  end
)
|
# Second pass: optional non-nullable ARRAYS on response-shaped schemas keep
# native []T (skip the optional pointer). A nil slice already represents
# absence; *[]T adds a deref layer with no semantic gain. Excluded on purpose:
#   * REQUEST-REACHABLE schemas — *[]T is required to SEND an explicit empty
#     array (omitempty drops a len-0 slice), e.g. Create* subscriptions where
#     nil (server default) and [] (subscribe nobody) differ. Reachability is
#     the transitive $ref closure from every *RequestContent schema, NOT a name
#     match: nested shapes like QuestionSchedule (referenced by
#     Create/UpdateQuestion bodies) are request-reachable without carrying the
#     suffix, and a name-only test silently made their arrays unsendable-empty.
#   * nullable arrays — kept pointer-shaped so an explicit JSON null can be
#     MARSHALLED (a non-nil pointer to a nil slice emits `null`). Note the
#     honest limit: on DECODE, encoding/json maps both an omitted key and an
#     explicit null to a nil pointer, so *[]T cannot tell them apart. No schema
#     currently pairs optional with nullable on an array — verified empty — so
#     nothing depends on that distinction today; a future one would need a
#     different representation (a wrapper type with an explicit presence flag),
#     not this pass.
# $request_reachable is computed once in the shell above (see the preamble for
# why the seed comes from the operations rather than a name convention) and
# injected with --argjson.
( $request_reachable ) as $reachable_names
|
.components.schemas |= with_entries(
  # NOTE: bind the key before the membership test — `index(.key)` would
  # evaluate `.key` against the ARRAY being indexed, not against this entry.
  .key as $schema_name
  | if ($reachable_names | index($schema_name) | not) then
    .value |= (
      if type == "object" and .type == "object" and .properties then
        (.required // []) as $required |
        .properties |= with_entries(
          .key as $propName |
          if .value.type == "array" and (.value.nullable != true) and ($required | index($propName) | not) then
            .value += { "x-go-type-skip-optional-pointer": true }
          else
            .
          end
        )
      else
        .
      end
    )
  else
    .
  end
)
|
# Fourth pass: Upload width/height → types.FlexInt
# The BC3 API serializes pixel dimensions as floats (1024.0); Go rejects
# those into int fields. Scoped to the Upload schema to avoid surprising
# any future integer width/height elsewhere in the spec.
.components.schemas.Upload.properties |= (
  (.width // empty) += {
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
  } |
  (.height // empty) += {
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
  }
)
|
# Fourth-b pass: RichTextAttachment width/height → nullable *types.FlexInt
# Same float-encoded-integer wire format as Upload (1024.0), but here the
# dimensions are nullable (the key is always emitted but the value is null for
# non-image blobs). The optional pointer (now the default) plus nullable lets
# the static types across SDKs capture the present-null value: Go
# *types.FlexInt, TypeScript `number | null`, Python `Optional[int]`.
.components.schemas.RichTextAttachment.properties |= (
  (.width // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
  } |
  (.height // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
  }
)
|
# Fifth pass: override starts_at/ends_at on ScheduleEntry response to use types.FlexibleTime
# The API returns date-only strings ("2006-01-02") for all-day schedule entries,
# which time.Time cannot parse. FlexibleTime handles RFC3339, RFC3339Nano, and date-only.
# Only the response schema needs this; request schemas keep time.Time since we always send RFC3339.
.components.schemas.ScheduleEntry.properties.starts_at += {
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
}
|
.components.schemas.ScheduleEntry.properties.ends_at += {
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
}
|
# Fifth-a2 pass: same for the upcoming-schedule reports reduced projection.
# app/views/api/schedules/calendar/_entry.json.jbuilder renders the identical
# starts_at_date_or_time / ends_at_date_or_time pair, so an all-day entry in the
# upcoming report is date-only on the wire too. Not nullable: Schedule::Entry
# validates the presence of both bounds, so unlike the timeline event data below
# these are always a real value.
.components.schemas.UpcomingScheduleEntry.properties.starts_at += {
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
}
|
.components.schemas.UpcomingScheduleEntry.properties.ends_at += {
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
}
|
# Fifth-b pass: override starts_at/ends_at on TimelineEventData to use
# types.FlexibleTime. Same all-day date-only wire form as ScheduleEntry: the
# schedule_entry_* timeline events carry a date ("2006-01-02") when all_day is
# true, which time.Time cannot parse. Overrides the first pass _at to time.Time.
# Also nullable: BC3 emits all three members unconditionally when the data object
# is present (so they are @required for presence), but starts_at/ends_at may be
# JSON null (starts_at_date_or_time returns nil for an entry with no bound). These
# are @required, so `nullable: true` alone would NOT nullable them in Kotlin/Swift
# (a required field stays non-null `String`, and a null bound fails decode). Encode
# the value nullability as a `type: ["string","null"]` union — the required-and-
# nullable treatment used for Wormhole.destination_url / SearchType.key — so every
# static SDK models them required-but-nullable (`string | null`). In Go these
# generate as *types.FlexibleTime (required-and-nullable takes a pointer), and
# FlexibleTime itself decodes a JSON null to the zero time.
.components.schemas.TimelineEventData.properties.starts_at += {
  "type": ["string", "null"],
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
}
|
.components.schemas.TimelineEventData.properties.ends_at += {
  "type": ["string", "null"],
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
}
|
# Fifth-c pass: TimelineAttachment width/height → nullable *types.FlexInt
# The attachment/blob variant serializes pixel dimensions float-spelled (1024.0)
# and null for non-image blobs, exactly like RichTextAttachment. Keep the
# optional pointer and mark nullable so the present-null value types faithfully.
.components.schemas.TimelineAttachment.properties |= (
  (.width // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
  } |
  (.height // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
  }
)
|
# Fifth-d pass: EverythingFile width/height → nullable *types.FlexInt
# The /files.json feed mixes uploads and attachments whose pixel dimensions are
# float-spelled (1024.0) and null for non-image blobs, exactly like
# RichTextAttachment/TimelineAttachment. Keep the optional pointer and mark
# nullable so the present-null value types faithfully.
.components.schemas.EverythingFile.properties |= (
  (.width // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
  } |
  (.height // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
  }
)
|
# Fifth-g pass: Todo/Card date fields are nullable-when-present.
# BC3 renders due_on (and, for to-dos, starts_on) through the shared todos/_todo
# and cards/_card partials with a JSON null value when the record has no date
# (documented across the resource docs and the account-wide feeds, e.g.
# /todos/no_due_date.json). Mark them nullable so a null value is accepted by the
# schema (AJV) and typed as such by the static SDKs, instead of the types lying
# that the value is always a non-null string. These stay OPTIONAL in the schema
# (not @required): the static SDKs type them `string | null | undefined`, which
# also tolerates a partial payload that omits the key. Go types them
# *types.Date (optional-pointer default), and types.Date decodes JSON null to
# the zero Date.
.components.schemas.Todo.properties.due_on += { "nullable": true }
|
.components.schemas.Todo.properties.starts_on += { "nullable": true }
|
.components.schemas.Card.properties.due_on += { "nullable": true }
|
# Same treatment for the upcoming-schedule report reduced projection. Its
# calendar partial writes both keys unconditionally for to-dos, cards and steps
# alike, and Kanban::Card and Step each define starts_on as a literal nil to
# duck-type Todo — so a null value is the common case here, not the edge one.
.components.schemas.UpcomingAssignable.properties.due_on += { "nullable": true }
|
.components.schemas.UpcomingAssignable.properties.starts_on += { "nullable": true }
|
# Sixth pass: Person.id → types.FlexibleInt64
# The API sometimes returns person IDs as JSON strings (e.g. in notification
# responses); Go rejects those into int64 fields. Scoped to Person schema only.
.components.schemas.Person.properties.id += {
  "x-go-type": "types.FlexibleInt64",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"}
}
|
# Seventh pass: append .json to path keys where Smithy cannot express it
# (labeled terminal segments like /{personId} need .json but Smithy forbids it)
.paths |= (to_entries | map(
  if .key == "/{accountId}/reports/users/progress/{personId}" then
    .key = "/{accountId}/reports/users/progress/{personId}.json"
  else . end
) | from_entries)
|
# Ninth pass: hoist normalized x-deprecated-reason so oapi-codegen (v2.8.0) emits
# a precise "// Deprecated: <reason>" godoc instead of its generic fallback. Data-
# driven, not keyed on specific names:
#   * query params — any deprecated query param (deprecated on the param or its
#     schema). Reason priority: param.description -> schema.description -> generic.
#   * component schemas — any deprecated component. Reason: its own description.
#   * $ref-sibling deprecated properties — reason resolves to the referenced
#     component description (the property carries no local description).
# See scripts/check-deprecation-parity for the shared cross-SDK contract.
( .components.schemas ) as $schemas
|
.paths |= map_values(
  # Path-item-level (shared) parameters — inherited by every method on the path.
  ( if (.parameters | type == "array") then
      .parameters |= map(mark_deprecated_query_param)
    else . end )
  |
  # Operation-level parameters (the .parameters array is skipped here since it is
  # not an object, so it is not double-processed).
  map_values(
    if (type == "object") and (.parameters | type == "array") then
      .parameters |= map(mark_deprecated_query_param)
    else . end
  )
)
|
.components.schemas |= map_values(
  ( if .deprecated == true then
      .["x-deprecated-reason"] = ((.description // "deprecated") | normalize_deprecation_reason)
    else . end )
  |
  ( if (.properties? // {} | length) > 0 then
      .properties |= map_values(
        if .deprecated == true then
          .["x-deprecated-reason"] = (
            ( .description
              // ( if has("$ref") then ($schemas[(.["$ref"] | sub(".*/"; ""))].description) else null end )
              // "deprecated"
            ) | normalize_deprecation_reason )
        else . end
      )
    else . end )
)
' "$INPUT_FILE" > "${OUTPUT_FILE}.tmp"

# Self-verify the array policy. The Go guard (check-go-optional-pointers)
# enforces nil-CAPABILITY, which a native []T satisfies — so it cannot catch a
# request-reachable array regressing to native and becoming unsendable-empty.
# That invariant is only checkable here, against the spec, so assert it where
# the data is: no schema reachable from an operation requestBody may carry
# x-go-type-skip-optional-pointer on an optional array.
leaked=$(jq -r --argjson request_reachable "$REQUEST_REACHABLE" '
  ( .components.schemas ) as $all
  | ( $request_reachable ) as $reachable
  | [ $all | to_entries[]
      | .key as $schema
      | select($reachable | index($schema)) | select(.value | type == "object")
      | ((.value.required // []) | select(type == "array")) as $req
      | ((.value.properties // {}) | select(type == "object")) | to_entries[]
      | select(.value | type == "object")
      | .key as $prop
      | select(.value.type == "array" and .value["x-go-type-skip-optional-pointer"] == true)
      | select($req | index($prop) | not)
      | "\($schema).\($prop)" ] | .[]
' "${OUTPUT_FILE}.tmp")

if [[ -n "$leaked" ]]; then
    echo "Error: request-reachable optional array(s) kept native []T — an explicit empty array would be unsendable:" >&2
    echo "$leaked" | sed 's/^/  /' >&2
    rm -f "${OUTPUT_FILE}.tmp"
    exit 1
fi

# ...and the other direction. Checking only the first half would let the pass
# silently stop marking anything: every optional array would become *[]T, which
# is not wrong but is exactly the churn the policy avoids — and a self-check
# that still passes when the pass does nothing is not a self-check.
unmarked=$(jq -r --argjson request_reachable "$REQUEST_REACHABLE" '
  ( $request_reachable ) as $reachable
  | [ .components.schemas | to_entries[]
      | .key as $schema
      | select($reachable | index($schema) | not) | select(.value | type == "object")
      # Mirror the entry condition of the marking pass exactly. That pass only
      # descends into schemas that declare `"type": "object"` AND carry
      # properties; a checker with a looser guard would flag a schema the pass
      # deliberately skipped, turning a legitimate spec into a spurious
      # generation failure.
      | select(.value.type == "object") | select(.value.properties)
      | ((.value.required // []) | select(type == "array")) as $req
      | ((.value.properties // {}) | select(type == "object")) | to_entries[]
      | select(.value | type == "object")
      | .key as $prop
      | select(.value.type == "array" and (.value.nullable != true)
               and (.value["x-go-type-skip-optional-pointer"] != true))
      | select($req | index($prop) | not)
      | "\($schema).\($prop)" ] | .[]
' "${OUTPUT_FILE}.tmp")

if [[ -n "$unmarked" ]]; then
    echo "Error: response-only optional array(s) NOT kept native []T — the skip-optional-pointer pass did not reach them:" >&2
    echo "$unmarked" | sed 's/^/  /' >&2
    rm -f "${OUTPUT_FILE}.tmp"
    exit 1
fi

# Only now is the output known-good. Publishing before validating would leave a
# spec that violates the policy in place for the next generator to consume.
mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"

# Count enhancements
timestamp_count=$(jq '[.. | objects | select(.["x-go-type"] == "time.Time")] | length' "$OUTPUT_FILE")
date_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.Date")] | length' "$OUTPUT_FILE")
flexint_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.FlexInt")] | length' "$OUTPUT_FILE")
flexibleint64_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.FlexibleInt64")] | length' "$OUTPUT_FILE")
native_array_count=$(jq '[.. | objects | select(.["x-go-type-skip-optional-pointer"] == true and .type == "array")] | length' "$OUTPUT_FILE")

flexible_time_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.FlexibleTime")] | length' "$OUTPUT_FILE")

echo "Enhanced OpenAPI spec with Go type extensions:"
echo "  Timestamp fields (time.Time): $timestamp_count"
echo "  FlexibleTime fields (types.FlexibleTime): $flexible_time_count"
echo "  Date fields (types.Date): $date_count"
echo "  Dimension fields (types.FlexInt): $flexint_count"
echo "  Flexible ID fields (types.FlexibleInt64): $flexibleint64_count"
echo "  Native optional arrays ([]T, skip-optional-pointer): $native_array_count"
