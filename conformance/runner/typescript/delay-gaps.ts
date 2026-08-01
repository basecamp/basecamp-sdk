/**
 * The `delayBetweenRequests` assertion contract, kept apart from the runner so
 * its bounds branches are unit-testable (delay-gaps.test.ts).
 */

/**
 * Validates one assertion against the recorded request times, returning
 * `undefined` when it holds and a failure message otherwise.
 *
 * Gap i is the interval between request i and request i+1, so N requests yield
 * N-1 gaps. The contract in conformance/schema.json:
 *
 * - A NAMED index selects exactly that gap, bounds-checked unconditionally. A
 *   gap the run never produced is a failure, not a silent pass — the whole
 *   point of a timing pin is to catch a dropped backoff, and a dropped backoff
 *   is precisely what removes the gap.
 * - An OMITTED index requires the minimum on EVERY gap. Zero gaps means
 *   nothing was measured, so that fails too: an "every gap" rule with no gaps
 *   left would otherwise wave through a run that dropped every retry.
 * - Negative indexes are rejected rather than wrapping to the end the way the
 *   per-request assertions do. There is no sensible "last gap" when the point
 *   of naming one is to pin a specific backoff.
 *
 * `slackMs` exists because Node's timers may fire marginally BEFORE the
 * requested delay — libuv rounds the deadline down internally, so a 2000ms
 * sleep can legitimately elapse in 1999.87ms. That is a runtime property, not
 * an SDK behaviour: the SDK asked for the full interval. The Go runner never
 * sees it because Go's timers do not fire early. A sub-millisecond allowance
 * cannot mask a real regression, which would miss by hundreds of milliseconds
 * (a dropped Retry-After means ~1000ms of backoff instead of 2000ms) or by the
 * whole interval (no delay at all).
 */
export function checkDelayGaps(
  requestTimes: number[],
  minDelayMs: number | undefined,
  index: number | undefined,
  slackMs = 0,
): string | undefined {
  // An absent or zero minimum still asserts that the gap EXISTS. The default
  // lands HERE rather than at the call site so a truthiness gate cannot
  // quietly reduce the assertion to nothing.
  const min = minDelayMs ?? 0;
  const gaps = requestTimes.length - 1;
  const floor = min - slackMs;

  const short = (gap: number): string | undefined => {
    const delay = requestTimes[gap + 1]! - requestTimes[gap]!;
    return delay < floor
      ? `expected delay >= ${min}ms at gap ${gap} (allowing ${slackMs}ms timer slack), got ${delay}ms`
      : undefined;
  };

  if (index !== undefined) {
    if (index < 0) {
      return `delayBetweenRequests gap index must be non-negative, got ${index}`;
    }
    if (index >= gaps) {
      return `expected a delay at gap ${index}, but only ${requestTimes.length} request(s) were made`;
    }
    return short(index);
  }

  if (gaps < 1) {
    return `expected a delay between requests, but only ${requestTimes.length} request(s) were made`;
  }
  for (let i = 0; i < gaps; i++) {
    const failure = short(i);
    if (failure !== undefined) return failure;
  }
  return undefined;
}
