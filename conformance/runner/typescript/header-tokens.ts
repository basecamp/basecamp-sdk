/**
 * The one token a fixture header value may carry, `{{httpdate+Ns}}` (SPEC §19,
 * conformance/schema.json), kept apart from the runner so its arithmetic is
 * unit-testable against a frozen instant (header-tokens.test.ts).
 */

const HEADER_TOKEN = /^\{\{(.*)\}\}$/;
const HTTPDATE_TOKEN = /^httpdate\+(\d{1,9})s$/;

/**
 * Resolves `{{httpdate+Ns}}` at the moment the response is served to the
 * IMF-fixdate of floor(now) + N + 1 seconds: the first whole second strictly
 * more than N seconds after the second the response is served in. A compliant
 * SPEC §6 parser sees a remainder in (N − latency, N + 1] and, rounding up,
 * computes at least N whole seconds, so the fixture pairs it with a
 * `delayBetweenRequests` floor of N × 1000 ms. It exists because a static
 * fixture has no clock: a literal past date pins only the fall-through, and a
 * far-future one is differently behaved per host.
 *
 * N is one to nine digits, so the arithmetic is exact everywhere and every
 * runner's date formatter stays in range; a longer N is an unrecognised token.
 *
 * An unrecognised `{{…}}` throws rather than passing through: a typo'd token
 * served verbatim would be an unparseable header, which the SDK answers with
 * its ordinary backoff — the exact outcome the case exists to distinguish
 * from. Every other value passes through untouched.
 */
export function resolveHeaderValue(value: string, nowMs: number): string {
  const token = HEADER_TOKEN.exec(value);
  if (token === null) return value;
  const inner = HTTPDATE_TOKEN.exec(token[1]!);
  if (inner === null) {
    throw new Error(
      `unrecognised header token ${JSON.stringify(value)}: only {{httpdate+Ns}} is defined (conformance/schema.json)`,
    );
  }
  const seconds = Math.floor(nowMs / 1000) + Number(inner[1]) + 1;
  // Date#toUTCString is specified as the IMF-fixdate shape (ECMA-262 §21.4.4.43).
  return new Date(seconds * 1000).toUTCString();
}
