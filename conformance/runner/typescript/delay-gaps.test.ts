/**
 * Bounds and every-gap contract for the delayBetweenRequests assertion.
 *
 * Each case names a behavior that regressed on #563/#568: an assertion that
 * looked like it covered a timing gap and did not.
 */
import { describe, it, expect } from "vitest";
import { checkDelayGaps } from "./delay-gaps.js";

/** Builds request timestamps from successive gaps in milliseconds. */
function times(...gapsMs: number[]): number[] {
  const out = [0];
  for (const ms of gapsMs) out.push(out[out.length - 1]! + ms);
  return out;
}

describe("checkDelayGaps", () => {
  it("catches a later failing gap when the index is omitted", () => {
    // Three runners measured gap 0 and stopped, so a second backoff that never
    // happened passed unnoticed.
    expect(checkDelayGaps(times(1000, 5), 500, undefined)).toContain("at gap 1");
  });

  it("passes when every gap clears the minimum", () => {
    expect(checkDelayGaps(times(1000, 2000, 800), 500, undefined)).toBeUndefined();
  });

  it("fails an omitted index when there are no gaps at all", () => {
    // An "every gap" rule with no gaps left must not wave the run through: a
    // fully dropped retry lands exactly here.
    expect(checkDelayGaps(times(), 500, undefined)).toBe(
      "expected a delay between requests, but only 1 request(s) were made",
    );
  });

  it("fails a named gap the run never produced", () => {
    expect(checkDelayGaps(times(1000), 500, 1)).toBe(
      "expected a delay at gap 1, but only 2 request(s) were made",
    );
  });

  it("fails a named gap on a single-request run", () => {
    expect(checkDelayGaps(times(), 500, 0)).toBe(
      "expected a delay at gap 0, but only 1 request(s) were made",
    );
  });

  it("rejects a negative gap index rather than wrapping to the end", () => {
    // The pre-fix guard was `toBeGreaterThan(index + 1)`, trivially satisfied
    // by a negative index, which then read times[-1] and reported NaN.
    expect(checkDelayGaps(times(1000, 2000), 500, -1)).toBe(
      "delayBetweenRequests gap index must be non-negative, got -1",
    );
  });

  it("fails an enormous gap index without arithmetic overflow", () => {
    expect(checkDelayGaps(times(1000, 2000), 500, Number.MAX_SAFE_INTEGER)).toContain(
      "expected a delay at gap",
    );
  });

  it("passes a named gap that clears the minimum", () => {
    expect(checkDelayGaps(times(5, 2000), 500, 1)).toBeUndefined();
  });

  it("fails a named gap below the minimum", () => {
    expect(checkDelayGaps(times(2000, 5), 500, 1)).toContain("at gap 1");
  });

  it("still asserts the gap exists when the minimum is zero", () => {
    // A zero minimum is trivially met; the EXISTENCE requirement is not.
    for (const missing of [0, undefined]) {
      expect(checkDelayGaps(times(), missing, undefined)).toBe(
        "expected a delay between requests, but only 1 request(s) were made",
      );
      expect(checkDelayGaps(times(), missing, 0)).toBe(
        "expected a delay at gap 0, but only 1 request(s) were made",
      );
      expect(checkDelayGaps(times(5), missing, undefined)).toBeUndefined();
    }
  });

  it("allows timer slack but not a real miss", () => {
    // libuv can fire a 2000ms sleep at 1999.87ms; that must not fail. A
    // dropped Retry-After (1000ms instead of 2000ms) still must.
    expect(checkDelayGaps(times(1999), 2000, undefined, 2)).toBeUndefined();
    expect(checkDelayGaps(times(1000), 2000, undefined, 2)).toContain("at gap 0");
  });
});
