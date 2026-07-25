#!/usr/bin/env bash
#
# normalize-go-deprecation-godoc.sh — post-process oapi-codegen output so
# every `// Deprecated:` marker sits in its own doc-comment paragraph.
#
# Why: Go's deprecation convention (and the staticcheck/gopls/pkg.go.dev tools
# that consume it) only recognize a `Deprecated:` marker when it *begins a
# paragraph* — i.e. is preceded by a blank `//` line. oapi-codegen's typedef
# path (`.DocComment`) already does this for deprecated *types*, but its
# struct-field path (schema.go) appends the deprecation line directly after the
# field's description line with no blank separator. That leaves deprecated
# fields — the Search params (`type`/`bucket_id`/`creator_id`) and the
# `Project.clientside` property — with a marker that tools silently ignore.
#
# This pass inserts a blank `//` line before any `// Deprecated:` line whose
# immediately preceding line is a non-blank comment, matching the current
# indentation. It is a no-op where the paragraph break already exists (the
# type path), and idempotent. General by construction — keyed on the comment
# shape, not on any field name.
#
# Usage: normalize-go-deprecation-godoc.sh <file.go> [<file.go> ...]
set -euo pipefail

if [[ $# -eq 0 ]]; then
    echo "Usage: $0 <file.go> [<file.go> ...]" >&2
    exit 1
fi

for file in "$@"; do
    if [[ ! -f "$file" ]]; then
        echo "Error: file not found: $file" >&2
        exit 1
    fi

    awk '
    function is_comment(s)       { return s ~ /^[[:space:]]*\/\// }
    function is_blank_comment(s) { return s ~ /^[[:space:]]*\/\/[[:space:]]*$/ }
    {
        if ($0 ~ /^[[:space:]]*\/\/ Deprecated:/ && NR > 1 &&
            is_comment(prev) && !is_blank_comment(prev)) {
            match($0, /^[[:space:]]*/)
            print substr($0, 1, RLENGTH) "//"
        }
        print
        prev = $0
    }
    ' "$file" > "${file}.tmp"
    mv "${file}.tmp" "$file"
done
