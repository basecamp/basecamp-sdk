#!/bin/bash
# normalize-go-error-response-parsing.sh
#
# Makes the generated ParseXxxResponse functions tolerant of malformed
# error-status (4xx/5xx) JSON bodies. oapi-codegen's genResponseUnmarshal emits
# a strict unmarshal for every declared response:
#
#   case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 422:
#       var dest FieldValidationErrorResponseContent
#       if err := json.Unmarshal(bodyBytes, &dest); err != nil {
#           return nil, err
#       }
#       response.JSON422 = &dest
#
# For error statuses that strictness is wrong: a 422 body like
# {"errors": {"color": ["bad"], "base": "invalid"}} fails to decode into the
# typed map[string][]string, the raw unmarshal error propagates, and the
# wrapper's checkResponse — the tolerant SPEC §6 parser that maps the raw body
# to a structured *Error — never runs. The typed JSON4xx/JSON5xx fields have no
# production consumers (wrappers always feed resp.Body to checkResponse), so an
# undecodable error body should simply leave the typed field nil:
#
#   case strings.Contains(rsp.Header.Get("Content-Type"), "json") && rsp.StatusCode == 422:
#       var dest FieldValidationErrorResponseContent
#       if err := json.Unmarshal(bodyBytes, &dest); err == nil {
#           response.JSON422 = &dest
#       }
#
# 2xx parsing stays strict — a malformed success body IS an error.
#
# This runs as a post-generation normalization step (see go/generate.go), the
# same mechanism as normalize-go-deprecation-godoc.sh, and is applied
# identically by scripts/check-go-generated-drift.sh so the freshness gate
# compares like with like. The rewrite is a fail-loud state machine: any
# deviation from the expected 5-line case body aborts, so a future
# oapi-codegen upgrade that changes the emission shape cannot be silently
# half-normalized.
#
# Usage: normalize-go-error-response-parsing.sh <path-to-client.gen.go>

set -euo pipefail

FILE="${1:?usage: normalize-go-error-response-parsing.sh <path-to-client.gen.go>}"

[ -f "$FILE" ] || { echo "ERROR: no such file: $FILE" >&2; exit 1; }

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

awk '
function fail(msg) {
    printf "ERROR: normalize-go-error-response-parsing: %s at line %d: %s\n", msg, NR, $0 > "/dev/stderr"
    exit 1
}
BEGIN { state = 0; count = 0 }
{
    if (state == 0) {
        if ($0 ~ /^\tcase strings\.Contains\(rsp\.Header\.Get\("Content-Type"\), "json"\) && rsp\.StatusCode == [45][0-9][0-9]:$/) {
            status = $0
            sub(/^.*rsp\.StatusCode == /, "", status)
            sub(/:$/, "", status)
            print
            state = 1
            next
        }
        print
        next
    }
    if (state == 1) {
        if ($0 !~ /^\t\tvar dest [A-Za-z0-9_]+$/) fail("expected var dest line")
        print
        state = 2
        next
    }
    if (state == 2) {
        if ($0 != "\t\tif err := json.Unmarshal(bodyBytes, &dest); err != nil {") fail("expected strict unmarshal if-line")
        print "\t\tif err := json.Unmarshal(bodyBytes, &dest); err == nil {"
        state = 3
        next
    }
    if (state == 3) {
        if ($0 != "\t\t\treturn nil, err") fail("expected return nil, err")
        print "\t\t\tresponse.JSON" status " = &dest"
        state = 4
        next
    }
    if (state == 4) {
        if ($0 != "\t\t}") fail("expected closing brace")
        print
        state = 5
        next
    }
    if (state == 5) {
        if ($0 != "\t\tresponse.JSON" status " = &dest") fail("expected response assignment")
        count++
        state = 0
        next
    }
}
END {
    if (state != 0) { printf "ERROR: normalize-go-error-response-parsing: truncated case body (state %d) at EOF\n", state > "/dev/stderr"; exit 1 }
    if (count == 0) { print "ERROR: normalize-go-error-response-parsing: no error-status cases rewritten — pattern drift?" > "/dev/stderr"; exit 1 }
    printf "normalized %d error-status response cases\n", count > "/dev/stderr"
}
' "$FILE" > "$TMP"

mv "$TMP" "$FILE"
trap - EXIT
