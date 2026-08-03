/**
 * The `requestCount` assertion contract, kept apart from the runner so its
 * bounds branches are unit-testable (request-count.test.ts).
 */

/**
 * Validates one assertion, returning `undefined` when it holds and a failure
 * message otherwise.
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
 * which asserts a single request — carries the `link-header` tag, and
 * `requestCountApplies` returns false for it. Nothing that still reaches this
 * function needs the relaxation.
 *
 * Swift took this in #558; #573 is the same fix for the other five runners.
 */
export function checkRequestCount(actual: number, expected: number): string | undefined {
  return actual === expected ? undefined : `Expected ${expected} requests, got ${actual}`;
}

/**
 * Marks a fixture whose requestCount counts first-page requests only, which an
 * auto-paginating SDK cannot satisfy.
 */
export const LINK_HEADER_TAG = "link-header";

/**
 * Whether a fixture's `requestCount` assertion is meaningful for this SDK.
 *
 * SCOPE: this suppresses ONE ASSERTION, not the whole test case. An earlier
 * revision skipped the entire `link-header` case in every runner, which took
 * its `statusCode: 200` and `noError` assertions down with the inapplicable
 * `requestCount` — Kotlin and Swift had always skipped the case wholesale, so
 * once Go, Python, Ruby and TypeScript joined them the fixture was executed by
 * nothing at all while still sitting in conformance/tests/pagination.json,
 * passing conformance-fixtures-check and check-fixture-coverage. That is the
 * #572 shape ("present, run by nothing") one layer down. Only the count is
 * inapplicable; the status code and the absence of an error are not, and they
 * are the assertions that catch an auto-paginating SDK that walked the Link
 * header into an error.
 */
export function requestCountApplies(tags: string[] | undefined): boolean {
  return !(tags ?? []).includes(LINK_HEADER_TAG);
}
