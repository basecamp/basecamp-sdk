#!/usr/bin/env bash
# Regression test for the SARIF generated-code filter used in codeql.yml.
# Runs the same jq expression against a fixture and asserts the output.
set -euo pipefail

FILTER='
  .runs |= map(.results |= (. // [] | map(
    select(
      (.locations // [])[0].physicalLocation.artifactLocation.uri // "" |
      test("(^|/)(go/pkg/generated/|typescript/(src/generated|dist)/|ruby/lib/basecamp/generated/|swift/Sources/Basecamp/Generated/|kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/)") | not
    )
  )))
'

DIR="$(cd "$(dirname "$0")" && pwd)"
FIXTURE="$DIR/testdata/sarif-filter-fixture.json"

actual=$(jq "$FILTER" "$FIXTURE")
kept=$(echo "$actual" | jq -c '[.runs[].results[].ruleId] | sort')
expected='["keep-kotlin-generator","keep-no-locations","keep-null-locations","keep-real-go","keep-swift-generator"]'

if [ "$kept" = "$expected" ]; then
  echo "PASS: SARIF filter kept correct results"
else
  echo "FAIL: expected $expected"
  echo "       got      $kept"
  exit 1
fi

# Verify null/missing .results don't blow up
null_run_count=$(echo "$actual" | jq '[.runs[] | select(.results == [])] | length')
if [ "$null_run_count" -eq 2 ]; then
  echo "PASS: null/missing .results handled"
else
  echo "FAIL: expected 2 empty-result runs, got $null_run_count"
  exit 1
fi
