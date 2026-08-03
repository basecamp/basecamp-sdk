package com.basecamp.sdk.conformance

/**
 * Validates one `requestCount` assertion, returning `null` when it holds and a
 * failure message otherwise.
 *
 * EXACT, always — including the auto-paginating fixtures. The runner used to
 * relax this to a lower bound whenever any mock response carried
 * `Link: rel="next"`, on the theory that an auto-paginating SDK would
 * legitimately make more requests than the fixture named. That is backwards for
 * the fixtures the relaxation covered: in conformance/tests/pagination.json,
 * "Pagination stops at maxPages safety cap" and "maxItems caps results across
 * pages" each queue THREE pages and expect TWO requests, because stopping early
 * is the behavior under test. `>=` passes an SDK that ignored the cap and
 * walked every page. "Auto-pagination follows Link headers across multiple
 * pages" is the exposed case: its only assertions are requestCount and noError,
 * so an over-fetch has nothing else to catch it — the other two happen to carry
 * a `responseMeta` truncated assertion that fires instead, coverage by luck.
 *
 * The one fixture where the count genuinely does not apply to an
 * auto-paginating SDK — "List operation returns first page with Link header",
 * which asserts a single request — carries the `link-header` tag and is already
 * excluded by the runner before any assertion is evaluated. Nothing that still
 * reaches this function needs the relaxation.
 *
 * Kotlin excludes the whole CASE where Go, Python, Ruby and TypeScript exclude
 * only the requestCount ASSERTION (#573). That asymmetry is deliberate, not
 * drift: `httpStatusCode` here is the status of the last MockEngine response
 * the SDK consumed, and an auto-paginating SDK walks past the end of a
 * one-response queue, so the fixture's `statusCode: 200` cannot be satisfied.
 * Narrowing this arm to the assertion was tried and reverted — `make
 * conformance-kotlin` then reports `Expected status code 200, but got no
 * response` and exits 2. Widening the status model is separate work; until
 * then, do not "align" this branch with the other four.
 *
 * The MockEngine's auto-pagination tolerance stays: answering an over-walk with
 * a terminal empty page rather than an error is what lets this tightened
 * assertion report a clean count mismatch instead of an opaque transport error.
 *
 * Swift took this in #558; #573 is the same fix for the other five runners.
 */
fun checkRequestCount(actual: Int, expected: Int): String? =
    if (actual == expected) null else "Expected $expected requests, got $actual"
