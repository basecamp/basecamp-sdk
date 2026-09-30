#!/usr/bin/env bash
#
# emit-go-deprecated-aliases.sh — add a Go type alias for every deprecated
# former component name in openapi.json.
#
# A rename kept for compatibility (CardStep -> Subtask) leaves the old
# component in openapi.json as nothing but a deprecated `$ref` to the new one.
# oapi-codegen prunes it (skip-prune: false) because no operation references it
# any more, so the old name would vanish from pkg/generated. This pass puts it
# back as
#
#     // CardStep is the deprecated former name of Subtask.
#     //
#     // Deprecated: <x-deprecated-reason>
#     type CardStep = Subtask
#
# directly after the target type's declaration. Data-driven — keyed on the
# component shape (deprecated, `$ref`, no other schema keywords), not on any
# name — and idempotent. The other six generators emit the same alias from the
# same shape; scripts/check-deprecation-parity pins all seven.
#
# Usage: emit-go-deprecated-aliases.sh <openapi.json> <file.go>
set -euo pipefail

if [[ $# -ne 2 ]]; then
    echo "Usage: $0 <openapi.json> <file.go>" >&2
    exit 1
fi
spec=$1
file=$2
for f in "$spec" "$file"; do
    if [[ ! -f "$f" ]]; then
        echo "Error: file not found: $f" >&2
        exit 1
    fi
done

# alias<TAB>target<TAB>problem<TAB>reason, one line per deprecated alias.
#
# `problem` is "-" for an alias this pass can place (not empty: a tab is IFS
# whitespace, so `read` would collapse an empty field into the next one). The shape rules are the
# ones all seven generators apply, so a spec one SDK refuses is refused by all
# of them rather than quietly losing the old name in some:
#   - the target must exist;
#   - it must not itself be a deprecated alias (a chain is rejected, not
#     resolved: point the alias at the model directly);
#   - it must be an object model, not an enum or a scalar.
# `deprecated` must be the JSON boolean true: 1 or "false" is not an alias.
#
# Sorted DESCENDING, in the C locale: each alias is inserted directly after its
# target's declaration, so several aliases of one target land in the file in
# the reverse of processing order, and descending processing leaves them
# ascending. Plain `sort` orders by the caller's locale (aAlias/BAlias swap
# between C and en_US.UTF-8), which would make the output machine-dependent.
aliases=$(jq -r '
  .components.schemas as $schemas
  | def is_alias: type == "object" and .deprecated == true and (.["$ref"] | type == "string")
      and ((keys - ["$ref", "deprecated", "description", "x-deprecated-reason"]) | length == 0);
  $schemas | to_entries[]
  | select(.value | is_alias)
  | (.value["$ref"] | sub(".*/"; "")) as $target
  | [ .key,
      $target,
      ( if ($schemas | has($target) | not) then "target schema does not exist"
        elif ($schemas[$target] | is_alias) then "target is itself a deprecated alias; point \(.key) at the model directly"
        elif ($schemas[$target] | (.type == "object" and ((.properties // {}) | length > 0)) | not) then "target is not an object model"
        else "-" end ),
      (.value["x-deprecated-reason"] // "deprecated" | gsub("[\r\n]+"; " "))
    ]
  | @tsv
' "$spec" | LC_ALL=C sort -r)

[[ -z "$aliases" ]] && exit 0

# Refuse before writing anything, so a bad alias never leaves a half-edited file.
while IFS=$'\t' read -r alias target problem _; do
    if [[ "$problem" != "-" ]]; then
        echo "Error: deprecated alias ${alias} -> ${target}: ${problem}" >&2
        exit 1
    fi
    if grep -Eq "^type ${alias} = ${target}$" "$file"; then
        continue # already emitted (idempotent)
    fi
    if grep -Eq "^type ${alias}( |$)" "$file"; then
        echo "Error: deprecated alias ${alias} -> ${target}: ${alias} collides with an existing type in ${file}" >&2
        exit 1
    fi
    if ! grep -Eq "^type ${target} struct \{" "$file"; then
        echo "Error: deprecated alias ${alias} -> ${target}: target model was not emitted (${file} does not declare it as a struct)" >&2
        exit 1
    fi
done <<< "$aliases"

while IFS=$'\t' read -r alias target _ reason; do
    if grep -Eq "^type ${alias} = ${target}$" "$file"; then
        continue # already emitted (idempotent)
    fi
    # The reason goes through the environment, not `awk -v`, which would expand
    # backslash escapes in it.
    ALIAS_REASON="$reason" awk -v alias="$alias" -v target="$target" '
    { print }
    $0 ~ ("^type " target " struct \\{") { in_target = 1; next }
    in_target && /^}/ {
        print ""
        print "// " alias " is the deprecated former name of " target "."
        print "//"
        print "// Deprecated: " ENVIRON["ALIAS_REASON"]
        print "type " alias " = " target
        in_target = 0
    }
    ' "$file" > "${file}.tmp"
    mv "${file}.tmp" "$file"
done <<< "$aliases"
