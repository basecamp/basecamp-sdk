/**
 * SPEC §6 "Retry-After Parsing Algorithm" — the parser itself, and the two
 * retry loops that must reach it rather than re-deriving it (#564).
 *
 * TypeScript had three copies of this algorithm: the correct one in `errors.ts`,
 * which only the error constructor used, and a `parseInt` in each retry loop.
 * Neither copy had the HTTP-date branch, and both mishandled a non-positive
 * value — `retry.ts` slept for `0` or a NEGATIVE interval, `services/base.ts`
 * guarded with `>= 0` and so admitted zero. The conformance fixture pins the
 * zero and negative cases on the JSON client path
 * (`conformance/tests/retry.json`); what it cannot pin is the HTTP-date branch,
 * because a fixture is a static literal with no clock (#780). That branch is
 * asserted here.
 *
 * The delays involved are real sleeps, so the retry loop's own abort signal is
 * used to settle each one the moment its length has been observed. That keeps a
 * three-minute server-directed delay assertable in a millisecond, and it reads
 * the delay from the same `retrying` hook the SDK's callers see.
 */
import { describe, it, expect } from "vitest";
import { executeWithRetry, type RetryConfig, type RetryEmit } from "../src/retry.js";
import { parseRetryAfter } from "../src/errors.js";

const CONFIG: RetryConfig = {
  maxAttempts: 3,
  baseDelayMs: 1000,
  backoff: "exponential",
  retryOn: [429, 503],
};

/** The backoff term for the first retry, plus the jitter ceiling. */
const BACKOFF_MIN_MS = 1000;
const BACKOFF_MAX_MS = 1100;

/**
 * Drives one 429 carrying `retryAfter` through the retry loop and returns the
 * delay the loop chose, without waiting it out.
 */
async function delayChosenFor(retryAfter: string): Promise<number> {
  const controller = new AbortController();
  let chosen = Number.NaN;

  const emit: RetryEmit = {
    begin: () => {},
    finalize: () => {},
    retrying: (_failedAttempt, _error, delayMs) => {
      chosen = delayMs;
      // The loop sleeps immediately after this hook, and sleep() rejects on
      // abort — so the delay is observed in full and never served.
      controller.abort(new Error("delay captured"));
    },
  };

  await expect(
    executeWithRetry(
      async () => new Response(null, { status: 429, headers: { "Retry-After": retryAfter } }),
      CONFIG,
      emit,
      controller.signal,
    ),
  ).rejects.toThrow("delay captured");

  return chosen;
}

describe("Retry-After parsing", () => {
  it("returns integer seconds only when positive", () => {
    expect(parseRetryAfter("120")).toBe(120);
    expect(parseRetryAfter("1")).toBe(1);
    expect(parseRetryAfter("0")).toBeUndefined();
    expect(parseRetryAfter("-5")).toBeUndefined();
    expect(parseRetryAfter(null)).toBeUndefined();
    expect(parseRetryAfter("")).toBeUndefined();
    expect(parseRetryAfter("soon")).toBeUndefined();
  });

  /**
   * Step 1 must validate the WHOLE value, not read a prefix off the front.
   * RFC 9110 spells `delay-seconds` as `1*DIGIT`, and `parseInt` does not — it
   * stops at the first non-digit and returns what it has, so `120junk` bought a
   * 120-second delay where every other SDK falls through to backoff
   * (`strconv.Atoi`, `int()`, `Integer(exception: false)`, `Int()` and
   * `toIntOrNull()` all reject a trailing-junk value). TypeScript was the lone
   * outlier in step 1 of the algorithm this module exists to make uniform.
   */
  it("rejects a value that is only partly numeric", () => {
    expect(parseRetryAfter("120junk")).toBeUndefined();
    expect(parseRetryAfter("120 junk")).toBeUndefined();
    expect(parseRetryAfter("12.5")).toBeUndefined();
    expect(parseRetryAfter("1e3")).toBeUndefined();
    expect(parseRetryAfter("0x10")).toBeUndefined();
  });

  /**
   * A leading sign is not junk: `Atoi`, `int()`, `Integer()`, `Int()` and
   * `toIntOrNull()` all consume one, so `+120` is 120 everywhere and `-120` is
   * a negative that step 1 then rejects for not being > 0. Surrounding
   * whitespace is likewise tolerated, which is what `parseInt` already did.
   */
  it("accepts a leading sign and surrounding whitespace, as the other five do", () => {
    expect(parseRetryAfter("+120")).toBe(120);
    expect(parseRetryAfter(" 120 ")).toBe(120);
    expect(parseRetryAfter("-120")).toBeUndefined();
  });

  /**
   * A digit string too large for a JS number became `Infinity`, and
   * `setTimeout(Infinity)` does not sleep forever — it CLAMPS TO 1ms, turning
   * the longest possible instruction into a tight retry loop against a server
   * already answering 429. That is the failure SPEC §7's backoff ceiling exists
   * to prevent, and `oauth/device.ts` already guards its own copy against it.
   * Go, Kotlin and Swift reject an out-of-range value outright; this does too.
   */
  it("rejects a value too large to represent rather than yielding Infinity", () => {
    expect(parseRetryAfter("9".repeat(400))).toBeUndefined();
    expect(parseRetryAfter("99999999999999999999")).toBeUndefined();
  });

  it("returns the seconds remaining until a future HTTP-date", () => {
    const threeMinutesOut = new Date(Date.now() + 180_000).toUTCString();
    const seconds = parseRetryAfter(threeMinutesOut);
    expect(seconds).toBeGreaterThanOrEqual(179);
    expect(seconds).toBeLessThanOrEqual(180);
  });

  it("rejects a past HTTP-date rather than returning zero", () => {
    // Step 2 computes max(0, date - now) but returns only a POSITIVE value:
    // handing back 0 would mean "retry immediately", the opposite instruction.
    expect(parseRetryAfter("Wed, 09 Jun 2021 10:18:14 GMT")).toBeUndefined();
  });

  /**
   * Step 2 says RFC 7231 HTTP-date, and `Date.parse` is very much wider than
   * that: it reads `2099-01-01`, `Jan 1 2099`, a bare `3000` and even
   * `3000junk` as real dates, all of them in the future. Tightening step 1
   * without tightening step 2 does not merely leave that standing, it makes it
   * REACHABLE — `3000junk` stops at step 1 today for 3000 seconds, and would
   * otherwise arrive at step 2 as the year 3000 and buy a ~975-year sleep.
   *
   * So the value is shape-checked against IMF-fixdate before `Date.parse` sees
   * it, which is where Ruby (`Time.httpdate`), Swift and Kotlin already are.
   * `Date.parse` still does the calendar arithmetic and still rejects an
   * impossible day or hour, which is why the last two cases hold.
   */
  it("accepts only the IMF-fixdate form, not everything Date.parse tolerates", () => {
    const future = new Date(Date.now() + 180_000);
    expect(parseRetryAfter(future.toUTCString())).toBeGreaterThan(0);

    expect(parseRetryAfter("2099-01-01")).toBeUndefined();
    expect(parseRetryAfter("2099-01-01T00:00:00Z")).toBeUndefined();
    expect(parseRetryAfter("Jan 1 2099")).toBeUndefined();
    expect(parseRetryAfter("3000junk")).toBeUndefined();
    // Not a date: a bare `3000` is a valid `delay-seconds` and step 1 claims it,
    // which is the whole reason the compound case above is the dangerous one —
    // `3000junk` is the value that falls out of step 1 and into step 2's lap.
    expect(parseRetryAfter("3000")).toBe(3000);
    expect(parseRetryAfter("Mon, 32 Jan 2099 00:00:00 GMT")).toBeUndefined();
    expect(parseRetryAfter("Mon, 01 Jan 2099 25:00:00 GMT")).toBeUndefined();
  });
});

describe("the shared retry loop honours the parsed value", () => {
  it("waits the seconds remaining until a future HTTP-date", async () => {
    const threeMinutesOut = new Date(Date.now() + 180_000).toUTCString();
    const delay = await delayChosenFor(threeMinutesOut);

    // Before #564 this loop's parseInt read "Wed" as NaN and fell through to
    // the ~1000ms backoff — the header ignored entirely.
    expect(delay).toBeGreaterThanOrEqual(179_000);
    expect(delay).toBeLessThanOrEqual(180_000);
  });

  it("waits the stated seconds for an integer value", async () => {
    expect(await delayChosenFor("120")).toBe(120_000);
  });

  it("falls through to backoff for a zero, a negative and an unparseable value", async () => {
    for (const header of ["0", "-5", "whenever", "120junk", "2099-01-01", "9".repeat(400)]) {
      const delay = await delayChosenFor(header);
      // Before #564: 0ms for "0" and -5000ms for "-5", both of which retry with
      // no wait at all — the backoff collapsed rather than being replaced.
      expect(delay, `Retry-After: ${header}`).toBeGreaterThanOrEqual(BACKOFF_MIN_MS);
      expect(delay, `Retry-After: ${header}`).toBeLessThanOrEqual(BACKOFF_MAX_MS);
    }
  });

  it("ignores Retry-After on a status that is not 429", async () => {
    // Which statuses honour the header is divergent across the six SDKs and is
    // tracked in #775; this pins TypeScript's current position so a parsing
    // change cannot move it by accident.
    const controller = new AbortController();
    let chosen = Number.NaN;
    const emit: RetryEmit = {
      begin: () => {},
      finalize: () => {},
      retrying: (_failedAttempt, _error, delayMs) => {
        chosen = delayMs;
        controller.abort(new Error("delay captured"));
      },
    };

    await expect(
      executeWithRetry(
        async () => new Response(null, { status: 503, headers: { "Retry-After": "120" } }),
        CONFIG,
        emit,
        controller.signal,
      ),
    ).rejects.toThrow("delay captured");

    expect(chosen).toBeGreaterThanOrEqual(BACKOFF_MIN_MS);
    expect(chosen).toBeLessThanOrEqual(BACKOFF_MAX_MS);
  });
});
