# Strips generated-code results from CodeQL SARIF before upload.
# Single source of truth: codeql.yml applies this file with `jq -f`, and
# test-sarif-filter.sh asserts it against testdata/sarif-filter-fixture.json,
# so the test always exercises the exact program the workflow runs.
#
# Generated Python is stripped with a lookahead sparing services/_ — that
# directory's _base.py and _async_base.py are hand-written (see
# codeql-config.yml) and paths-ignore can't express the exception.
.runs |= map(.results |= (. // [] | map(
  select(
    (.locations // [])[0].physicalLocation.artifactLocation.uri // "" |
    test("(^|/)(go/pkg/generated/|typescript/(src/generated|dist)/|ruby/lib/basecamp/generated/|swift/Sources/Basecamp/Generated/|kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/|python/src/basecamp/generated/(?!services/_))") | not
  )
)))
