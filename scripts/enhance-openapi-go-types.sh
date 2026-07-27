#!/usr/bin/env bash
#
# Enhance OpenAPI spec with Go-specific type extensions for oapi-codegen.
#
# Type mappings:
#   - _at fields (created_at, updated_at, etc.) → time.Time (full timestamps)
#   - ScheduleEntry starts_at/ends_at → types.FlexibleTime (handles date-only for all-day events)
#   - _on fields (due_on, starts_on, etc.) → types.Date (date-only)
#   - width/height fields → types.FlexInt (accepts float-encoded integers from API)
#   - id fields → keep as pointers to distinguish nil from zero
#
# Usage: ./enhance-openapi-go-types.sh [input.json] [output.json]
#        ./enhance-openapi-go-types.sh               # defaults to openapi.json in-place

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

INPUT_FILE="${1:-$PROJECT_ROOT/openapi.json}"
OUTPUT_FILE="${2:-$INPUT_FILE}"

if [[ ! -f "$INPUT_FILE" ]]; then
    echo "Error: Input file not found: $INPUT_FILE" >&2
    exit 1
fi

jq '
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

# First pass: add x-go-type extensions for timestamps, dates, and ids
walk(
  if type == "object" then
    to_entries | map(
      # Timestamp fields (_at): use time.Time
      if (.key | test("_at$")) and (.value | type == "object") and (.value.type == "string") then
        .value += {
          "x-go-type": "time.Time",
          "x-go-type-import": {"path": "time"},
          "x-go-type-skip-optional-pointer": true
        }
      # Date-only fields (_on): use types.Date
      elif (.key | test("_on$")) and (.value | type == "object") and (.value.type == "string") then
        .value += {
          "x-go-type": "types.Date",
          "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
          "x-go-type-skip-optional-pointer": true
        }
      # Id fields: keep as pointers (to distinguish nil from zero)
      # Matches "id", "*_id" (e.g., recording_id, category_id, todolist_id)
      elif (.key | test("^id$|_id$")) and (.value | type == "object") and (.value.type == "integer") then
        .value += {
          "x-go-type-skip-optional-pointer": false
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
# Second pass: mark optional booleans in REQUEST schemas with x-go-type-skip-optional-pointer: false
# This forces oapi-codegen to generate *bool instead of bool, allowing
# Go clients to distinguish "not set" (nil) from "false" in request bodies
# Only applies to schemas ending in "RequestContent" (request body schemas)
.components.schemas |= with_entries(
  if .key | test("RequestContent$") then
    .value |= (
      if type == "object" and .type == "object" and .properties then
        (.required // []) as $required |
        .properties |= with_entries(
          .key as $propName |
          if .value.type == "boolean" and ($required | index($propName) | not) then
            .value += { "x-go-type-skip-optional-pointer": false }
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
# Third pass: mark subscriptions arrays in Create* request schemas as pointer
# Distinguishes nil (omit → server default) from [] (subscribe nobody)
.components.schemas |= with_entries(
  if .key | test("^Create.*RequestContent$") then
    .value |= (
      if type == "object" and .type == "object" and .properties
         and .properties.subscriptions then
        .properties.subscriptions += { "x-go-type-skip-optional-pointer": false }
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
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
    "x-go-type-skip-optional-pointer": true
  } |
  (.height // empty) += {
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
    "x-go-type-skip-optional-pointer": true
  }
)
|
# Fourth-b pass: RichTextAttachment width/height → nullable *types.FlexInt
# Same float-encoded-integer wire format as Upload (1024.0), but here the
# dimensions are nullable (the key is always emitted but the value is null for
# non-image blobs). Keep the optional pointer (skip-optional-pointer: false)
# and mark the schema nullable so the static types across SDKs capture the
# present-null value: Go *types.FlexInt, TypeScript `number | null`, Python
# `Optional[int]`. Scoped to the RichTextAttachment schema.
.components.schemas.RichTextAttachment.properties |= (
  (.width // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
    "x-go-type-skip-optional-pointer": false
  } |
  (.height // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
    "x-go-type-skip-optional-pointer": false
  }
)
|
# Fifth pass: override starts_at/ends_at on ScheduleEntry response to use types.FlexibleTime
# The API returns date-only strings ("2006-01-02") for all-day schedule entries,
# which time.Time cannot parse. FlexibleTime handles RFC3339, RFC3339Nano, and date-only.
# Only the response schema needs this; request schemas keep time.Time since we always send RFC3339.
.components.schemas.ScheduleEntry.properties.starts_at += {
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
  "x-go-type-skip-optional-pointer": true
}
|
.components.schemas.ScheduleEntry.properties.ends_at += {
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
  "x-go-type-skip-optional-pointer": true
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
# static SDK models them required-but-nullable (`string | null`). Go keeps its
# FlexibleTime value type (via x-go-type), which already decodes null to the zero
# time.
.components.schemas.TimelineEventData.properties.starts_at += {
  "type": ["string", "null"],
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
  "x-go-type-skip-optional-pointer": true
}
|
.components.schemas.TimelineEventData.properties.ends_at += {
  "type": ["string", "null"],
  "x-go-type": "types.FlexibleTime",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
  "x-go-type-skip-optional-pointer": true
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
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
    "x-go-type-skip-optional-pointer": false
  } |
  (.height // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
    "x-go-type-skip-optional-pointer": false
  }
)
|
# Fifth-e pass: TimelineAttachment presence-faithful optional fields.
# The optional-field superset populates only one variant per instance, so ALL of
# its optional fields must round-trip presence: a plain time.Time re-marshals an
# absent field as the zero time, a plain bool with omitempty drops an explicit
# false, a plain int drops an explicit zero, and a plain string cannot tell an
# absent field from an explicit empty (SPEC.md §10 forbids empty-string as an
# absence sentinel). Make them all pointers so nil (absent) omits and an explicit
# value is preserved. width/height are handled by the Fifth-c FlexInt pass.
.components.schemas.TimelineAttachment.properties |= (
  (.created_at // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.updated_at // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.visible_to_clients // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.previewable // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.id // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.byte_size // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.content_type // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.filename // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.download_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.type // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.title // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.status // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.app_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.app_download_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.attachable_sgid // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.sgid // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.status_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.caption // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.key // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.preview_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.thumbnail_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  # Upload-recording projection fields (mirroring uploads/_upload). Optional in
  # the superset, so each must be pointer-backed for the same presence reason:
  # a plain string/int/bool/struct-with-omitempty cannot distinguish absent from
  # an explicit empty/zero/default value.
  (.inherits_status // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.bookmark_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.subscription_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.comments_count // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.comments_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.boosts_count // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.boosts_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.position // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.parent // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.bucket // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.creator // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.description // empty) += { "x-go-type-skip-optional-pointer": false }
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
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
    "x-go-type-skip-optional-pointer": false
  } |
  (.height // empty) += {
    "nullable": true,
    "x-go-type": "types.FlexInt",
    "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
    "x-go-type-skip-optional-pointer": false
  }
)
|
# Fifth-f pass: EverythingFile presence-faithful optional fields (same rationale
# as TimelineAttachment). The /files.json superset populates only one variant per
# instance, so ALL of its optional fields must round-trip presence: an absent
# field on the variant this instance is not must stay nil and re-marshal as
# omitted rather than a fabricated sentinel. That covers timestamps
# (created_at/updated_at *time.Time), booleans (visible_to_clients/inherits_status
# *bool), numeric scalars (counts/position/byte_size), AND the optional strings
# (SPEC.md section 10 forbids empty-string as an absence sentinel: a Document with
# no filename and an upload with an explicit empty filename must not collapse to
# the same value). id is already an optional pointer via the oapi default.
.components.schemas.EverythingFile.properties |= (
  (.created_at // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.updated_at // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.visible_to_clients // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.inherits_status // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.comments_count // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.boosts_count // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.position // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.byte_size // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.status // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.title // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.type // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.app_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.bookmark_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.subscription_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.comments_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.boosts_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.attachable_sgid // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.content_type // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.filename // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.download_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.app_download_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.description // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.content // empty) += { "x-go-type-skip-optional-pointer": false }
)
|
# Fifth-h pass: the Recording aggregate-feed type-specific scalars are presence-
# faithful. BC3 renders boosts_count/subject/group_on/from/replies_count/
# replies_url conditionally (only on the matching recording type — boosts_count
# via boostable, subject on Message, group_on on Question::Answer, from/replies_*
# on Inbox::Forward). Per SPEC.md §10 an optional scalar must round-trip absence,
# so a plain int/string with omitempty (which cannot distinguish absent from an
# explicit 0/"") is not acceptable. Make them pointer-backed (*int32 / *string /
# *types.Date) so nil (absent) omits and an explicit value is preserved. Scoped
# to the new fields only — the pre-existing comments_count/comments_url debt is
# out of scope here.
.components.schemas.Recording.properties |= (
  (.boosts_count // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.boosts_url // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.subject // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.group_on // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.from // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.replies_count // empty) += { "x-go-type-skip-optional-pointer": false } |
  (.replies_url // empty) += { "x-go-type-skip-optional-pointer": false }
)
|
# RecordingCategory.icon is optional (a category may omit its icon), so make it
# pointer-backed too — an empty string must not stand in for absence (SPEC.md §10).
.components.schemas.RecordingCategory.properties.icon += { "x-go-type-skip-optional-pointer": false }
|
# Fifth-g pass: Todo/Card date fields are nullable-when-present.
# BC3 renders due_on (and, for to-dos, starts_on) through the shared todos/_todo
# and cards/_card partials with a JSON null value when the record has no date
# (documented across the resource docs and the account-wide feeds, e.g.
# /todos/no_due_date.json). Mark them nullable so a null value is accepted by the
# schema (AJV) and typed as such by the static SDKs, instead of the types lying
# that the value is always a non-null string. These stay OPTIONAL in the schema
# (not @required): the static SDKs type them `string | null | undefined`, which
# also tolerates a partial payload that omits the key. Go keeps its types.Date
# value type (the _on pass already sets skip-optional-pointer: true), and
# types.Date decodes JSON null to the zero Date, so no Go change is needed.
.components.schemas.Todo.properties.due_on += { "nullable": true }
|
.components.schemas.Todo.properties.starts_on += { "nullable": true }
|
.components.schemas.Card.properties.due_on += { "nullable": true }
|
# Sixth pass: Person.id → types.FlexibleInt64
# The API sometimes returns person IDs as JSON strings (e.g. in notification
# responses); Go rejects those into int64 fields. Scoped to Person schema only.
.components.schemas.Person.properties.id += {
  "x-go-type": "types.FlexibleInt64",
  "x-go-type-import": {"path": "github.com/basecamp/basecamp-sdk/go/pkg/types"},
  "x-go-type-skip-optional-pointer": true
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
# Eighth pass: array-typed query params → pointer slices (*[]T).
# oapi-codegen serializes a non-pointer optional slice UNCONDITIONALLY, so a
# nil/empty slice still emits an empty `foo[]=` entry. Rails parses that as an
# array containing "" (and e.g. bucket_ids[]= normalizes to [0]), turning an
# unfiltered request into a bogus filtered one. Forcing a pointer makes
# oapi-codegen guard the param with `if params.X != nil`, so the hand-written
# wrapper omits it entirely when the slice is empty.
.paths |= map_values(
  map_values(
    if (type == "object") and (.parameters | type == "array") then
      .parameters |= map(
        if (.in == "query") and (.schema.type == "array") then
          .schema += { "x-go-type-skip-optional-pointer": false }
        else . end
      )
    else . end
  )
)
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

mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"

# Count enhancements
timestamp_count=$(jq '[.. | objects | select(.["x-go-type"] == "time.Time")] | length' "$OUTPUT_FILE")
date_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.Date")] | length' "$OUTPUT_FILE")
flexint_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.FlexInt")] | length' "$OUTPUT_FILE")
flexibleint64_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.FlexibleInt64")] | length' "$OUTPUT_FILE")
id_count=$(jq '[.. | objects | select(.["x-go-type-skip-optional-pointer"] == false and (.type == "integer" or .type == "number"))] | length' "$OUTPUT_FILE")
nullable_bool_count=$(jq '[.components.schemas | to_entries[] | select(.key | test("RequestContent$")) | .value.properties // {} | to_entries[] | select(.value.type == "boolean" and .value["x-go-type-skip-optional-pointer"] == false)] | length' "$OUTPUT_FILE")
subscription_ptr_count=$(jq '[.components.schemas | to_entries[] | select(.key | test("^Create.*RequestContent$")) | .value.properties // {} | .subscriptions // empty | select(.["x-go-type-skip-optional-pointer"] == false)] | length' "$OUTPUT_FILE")

flexible_time_count=$(jq '[.. | objects | select(.["x-go-type"] == "types.FlexibleTime")] | length' "$OUTPUT_FILE")

echo "Enhanced OpenAPI spec with Go type extensions:"
echo "  Timestamp fields (time.Time): $timestamp_count"
echo "  FlexibleTime fields (types.FlexibleTime): $flexible_time_count"
echo "  Date fields (types.Date): $date_count"
echo "  Dimension fields (types.FlexInt): $flexint_count"
echo "  Flexible ID fields (types.FlexibleInt64): $flexibleint64_count"
echo "  Id fields (keeping pointers): $id_count"
echo "  Nullable booleans (*bool): $nullable_bool_count"
echo "  Subscription pointers (*[]int64): $subscription_ptr_count"
