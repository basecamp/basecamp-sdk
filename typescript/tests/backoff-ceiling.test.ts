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

  /**
   * A fixed exponent cap plus a trailing `Math.min` bounds the intermediate but
   * not the outcome. With a cap of 53 and a base of 1e-30ms, every attempt from
   * the 54th on returned ~9e-15ms forever — a tight retry loop against a server
   * already answering 429/503, which is what SPEC §7 requirement 1 forbids. The
   * bound has to be derived from the base.
   *
   * Go, Kotlin and Swift never had this shape: their bases are integer
   * durations, so the `MAX / base` comparison always fires before their shift
   * cap does. This is what makes the six uniform rather than three-and-three.
   */
  const tinyBaseDelays = [1e-3, 1e-9, 1e-30, 1e-100, 1e-300, 5e-324];

  it("saturates at the ceiling for any positive base, never plateaus below it", () => {
    const violations = tinyBaseDelays.flatMap((base) =>
      [1023, 1100, 5000, Number.MAX_SAFE_INTEGER]
        .map((attempt) => [base, attempt, saturatingBackoff(base, "exponential", attempt)] as const)
        .filter(([, , delay]) => delay !== MAX_BACKOFF_DELAY_MS)
        .map(([b, attempt, delay]) => `base=${b} attempt=${attempt} -> ${delay}ms`),
    );

    expect(violations, "the term must saturate at the ceiling, not plateau below it").toEqual([]);
  });

  it("grows monotonically to the ceiling rather than stalling on the way", () => {
    for (const base of tinyBaseDelays) {
      let previous = 0;
      for (let attempt = 0; attempt <= 1100; attempt++) {
        const delay = saturatingBackoff(base, "exponential", attempt);
        expect(delay, `base=${base} went backwards at attempt ${attempt}`).toBeGreaterThanOrEqual(previous);
        expect(delay, `base=${base} exceeded the ceiling at attempt ${attempt}`).toBeLessThanOrEqual(
          MAX_BACKOFF_DELAY_MS,
        );
        previous = delay;
      }
      expect(previous, `base=${base} never reached the ceiling`).toBe(MAX_BACKOFF_DELAY_MS);
    }
  });

  /**
   * Linear backoff compares its multiplier against `MAX / base` before
   * multiplying, the same as Swift's. Saturation is asserted only for the bases
   * a linear term can actually reach the ceiling from: growth is `base × n`, so
   * it needs `n >= 30000 / base`, and `n` is an attempt count. Below ~3.3e-12ms
   * no finite attempt count gets there — an arithmetic fact about linear growth,
   * not a clamp defect. The ceiling is still never exceeded, which is asserted
   * for every base.
   */
  it("saturates linear backoff at the ceiling wherever a linear term can reach it", () => {
    for (const base of tinyBaseDelays) {
      const delay = saturatingBackoff(base, "linear", Number.MAX_SAFE_INTEGER);
      expect(delay).toBeLessThanOrEqual(MAX_BACKOFF_DELAY_MS);
      if (Number.MAX_SAFE_INTEGER >= MAX_BACKOFF_DELAY_MS / base) {
        expect(delay, `base=${base} should have saturated`).toBe(MAX_BACKOFF_DELAY_MS);
      }
    }
  });
});
