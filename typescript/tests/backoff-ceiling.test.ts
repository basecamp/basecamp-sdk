import { describe, it, expect } from "vitest";
import {
  calculateBackoffDelay,
  saturatingBackoff,
  MAX_BACKOFF_DELAY_MS,
  type RetryConfig,
} from "../src/retry.js";

const MAX_JITTER_MS = 100;

function config(overrides: Partial<RetryConfig> = {}): RetryConfig {
  return { maxAttempts: 3, baseDelayMs: 1000, backoff: "exponential", retryOn: [429, 503], ...overrides };
}

/**
 * SPEC §7 "Backoff Ceiling" (#577).
 *
 * `base * Math.pow(2, attempt)` reaches `Infinity` at attempt 1024 for a 1000ms
 * base — and `setTimeout` clamps an out-of-range delay to **1ms**, so the
 * failure mode is not a long sleep but a tight retry loop against a server
 * already answering 429/503. Long before that, the delays are measured in
 * millennia, which is its own kind of broken.
 *
 * The bound asserted is two-sided: a one-sided "never too long" check would
 * pass on exactly the `Infinity` case that tight-loops.
 */
describe("backoff ceiling", () => {
  it("saturates the exponential term instead of running away", () => {
    // With a 1000ms base, attempt index 5 is the first whose unclamped term
    // (32,000ms) exceeds the 30,000ms ceiling. Violations are collected rather
    // than asserted one at a time so a failure names every shape at once —
    // the millennia-long delays and the Infinity that setTimeout turns into 1ms.
    const violations = [5, 10, 40, 100, 1023, 1024, 1025, Number.MAX_SAFE_INTEGER]
      .map((attempt) => [attempt, calculateBackoffDelay(config(), attempt)] as const)
      .filter(([, delay]) => delay < MAX_BACKOFF_DELAY_MS || delay > MAX_BACKOFF_DELAY_MS + MAX_JITTER_MS)
      .map(([attempt, delay]) => `attempt ${attempt} -> ${delay}ms`);

    expect(violations, "every delay must land within the ceiling plus jitter").toEqual([]);
  });

  it("leaves the delays a shipped configuration actually reaches untouched", () => {
    for (const [attempt, want] of [[0, 1000], [1, 2000], [2, 4000]] as const) {
      const delay = calculateBackoffDelay(config(), attempt);
      expect(delay).toBeGreaterThanOrEqual(want);
      expect(delay).toBeLessThanOrEqual(want + MAX_JITTER_MS);
    }
  });

  it("clamps linear backoff and a base delay above the ceiling", () => {
    const linear = calculateBackoffDelay(config({ backoff: "linear" }), Number.MAX_SAFE_INTEGER);
    expect(linear).toBeLessThanOrEqual(MAX_BACKOFF_DELAY_MS + MAX_JITTER_MS);

    // SPEC §7 requirement 3: the ceiling binds base_delay_ms itself, with no
    // carve-out for the first sleep.
    const constant = calculateBackoffDelay(config({ backoff: "constant", baseDelayMs: 600_000 }), 0);
    expect(constant).toBeLessThanOrEqual(MAX_BACKOFF_DELAY_MS + MAX_JITTER_MS);
  });

  /**
   * TypeScript has two retry loops. `retry.ts` serves the middleware-chained
   * JSON path and download hop 1; `services/base.ts` runs its own for the raw
   * multipart transport and computed `baseDelayMs * Math.pow(2, attempt)`
   * inline. Fixing only `calculateBackoffDelay` would leave the multipart path
   * overflowing, so both now go through `saturatingBackoff` — asserted here
   * directly, since that shared function IS the multipart path's backoff.
   */
  it("saturates the shared term both retry loops compute", () => {
    for (const attempt of [5, 1023, 1024, 1025, Number.MAX_SAFE_INTEGER]) {
      expect(saturatingBackoff(1000, "exponential", attempt)).toBe(MAX_BACKOFF_DELAY_MS);
    }
    expect(saturatingBackoff(1000, "exponential", 0)).toBe(1000);
    expect(saturatingBackoff(1000, "exponential", 2)).toBe(4000);
    // A zero base delay stays at zero rather than saturating.
    expect(saturatingBackoff(0, "exponential", 1024)).toBe(0);
  });
});
