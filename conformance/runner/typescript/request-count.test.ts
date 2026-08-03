/**
 * Bounds contract for the requestCount assertion (#573).
 *
 * Until this commit the five non-Swift runners evaluated requestCount as a
 * LOWER bound whenever any mock response carried `Link: rel="next"`. Every
 * committed fixture passes under both rules, so nothing in the suite could tell
 * them apart — the same shape as the #563 delayBetweenRequests regression these
 * support modules exist to pin. The over-fetch case below is the one that
 * distinguishes them, and it is the case that matters: pagination.json's
 * maxPages and maxItems fixtures each queue three pages and assert two
 * requests, so a lower bound green-passes an SDK that ignored the cap.
 */

import { describe, expect, it } from "vitest";
import { checkRequestCount, requestCountApplies } from "./request-count.js";

describe("checkRequestCount", () => {
  it("passes on the exact count", () => {
    expect(checkRequestCount(2, 2)).toBeUndefined();
  });

  it("fails an under-fetch", () => {
    expect(checkRequestCount(1, 2)).toBeDefined();
  });

  it("fails an over-fetch", () => {
    // The regression. Under the old lower bound this returned undefined — an
    // SDK that walked all three queued pages instead of stopping at the
    // maxPages cap reported a clean pass.
    expect(checkRequestCount(3, 2)).toBeDefined();
  });

  it("names both counts in the failure message", () => {
    expect(checkRequestCount(3, 2)).toBe("Expected 2 requests, got 3");
  });

  it("does not treat a zero-request run as a free pass", () => {
    // A test whose operation never reached the wire records zero requests.
    // That must fail an assertion expecting one, not read as "no data, no
    // opinion".
    expect(checkRequestCount(0, 1)).toBeDefined();
  });

  it("requires zero actual when zero is expected", () => {
    expect(checkRequestCount(0, 0)).toBeUndefined();
    expect(checkRequestCount(1, 0)).toBeDefined();
  });
});

/**
 * Applicability contract (#573). The `link-header` fixture's requestCount is
 * inapplicable to an auto-paginating SDK; its statusCode and noError
 * assertions are not. Suppressing the CASE instead of the ASSERTION left the
 * fixture executed by nothing at all — it stays in pagination.json and passes
 * conformance-fixtures-check and check-fixture-coverage either way, so nothing
 * else would have reported it.
 */
describe("requestCountApplies", () => {
  it("does not apply to link-header fixtures", () => {
    expect(requestCountApplies(["pagination", "link-header"])).toBe(false);
  });

  it("applies to every other fixture", () => {
    for (const tags of [undefined, [], ["pagination"], ["retry", "idempotent"]]) {
      expect(requestCountApplies(tags)).toBe(true);
    }
  });

  it("suppresses one assertion, not the whole case", () => {
    const tags = ["pagination", "link-header"];
    const assertions = [
      { type: "requestCount", expected: 1 },
      { type: "statusCode", expected: 200 },
      { type: "noError" },
    ];
    const live = assertions.filter(
      (a) => !(a.type === "requestCount" && !requestCountApplies(tags)),
    );
    expect(live.map((a) => a.type)).toEqual(["statusCode", "noError"]);
  });
});
