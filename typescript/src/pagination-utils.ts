/**
 * Pagination utility functions.
 *
 * Extracted to its own module to avoid circular dependencies between
 * client.ts (which imports generated services) and base.ts (which
 * generated services extend).
 */

/**
 * Extracts the contents of the first non-empty `<...>` pair.
 *
 * This is the leftmost-match semantics of `/<([^>]+)>/` in linear time. The
 * regex form is quadratic on a header carrying many `<` with no reachable `>`,
 * because every `<` is retried as a start position and each scans to the end.
 * Searching for `>` from *after* the `<` visits each character once instead.
 *
 * An empty `<>` is skipped rather than returned, because `[^>]+` requires at
 * least one character — the regex would move on to the next `<`, and so does
 * this. Skipping costs nothing: the scan that found `<>` was O(1), so the loop
 * only ever repeats after work it did not do.
 */
function extractAngleBracketed(part: string): string | null {
  let cursor = 0;
  for (;;) {
    const start = part.indexOf("<", cursor);
    if (start < 0) return null;
    const end = part.indexOf(">", start + 1);
    if (end < 0) return null;
    if (end > start + 1) return part.slice(start + 1, end);
    cursor = start + 1;
  }
}

import { BasecampError } from "./errors.js";

/**
 * Rejects any `maxPages` that would defeat or invert the cap.
 *
 * Lives here, beside `DEFAULT_MAX_PAGES` and for the same reason: `client.ts`,
 * `services/base.ts` and this module's own helpers all need it, and this is the
 * only one of them they can all import without a cycle. Three entry points set
 * a cap — `createBasecampClient`, the standalone `fetchAllPages`/`paginateAll`,
 * and `BaseService`'s constructor, which is exported and directly constructible
 * — and a guard on two of the three is not a guard.
 *
 * `Number.isSafeInteger(n) && n > 0` is one predicate covering every value that
 * breaks the bound, each of which passed silently while `maxPages` was
 * unvalidated:
 *
 * - `Infinity` — `page === maxPages` is never true, so the loop is unbounded
 *   and the cap does nothing at all. Precisely the runaway it exists to stop.
 * - Anything ABOVE `2 ** 53`, such as `Number.MAX_VALUE` — the same runaway
 *   through a different door, and the reason this is `isSafeInteger` rather
 *   than `isInteger`. `Number.isInteger(Number.MAX_VALUE)` is `true`, so the
 *   obvious predicate admits it. Past `2 ** 53` the counter stalls: `page++`
 *   on `2 ** 53` yields `2 ** 53` again, because the next integer is not
 *   representable, so a bound of `2 ** 53 + 2` is never reached. `2 ** 53`
 *   ITSELF is fine — the counter arrives there from `2 ** 53 - 1` and breaks
 *   — so rejecting it is deliberately conservative rather than necessary. A
 *   cap must be a number the counter can actually arrive at, and
 *   `MAX_SAFE_INTEGER` is the honest edge of that guarantee.
 * - `2.5` — consumes 2 pages, then fetches a 3rd and discards it: a request to
 *   a URL taken from an attacker-influenceable header, whose response is never
 *   parsed or returned.
 * - `0`, negative, `NaN` — consume ZERO pages, throwing away a response the
 *   caller already fetched and passed in.
 *
 * SPEC.md §2 step 5: "Validate `max_pages > 0`. → `⊥ BasecampError(code:
 * "usage")` otherwise."
 */
export function assertValidMaxPages(maxPages: number): void {
  if (!Number.isSafeInteger(maxPages) || maxPages <= 0) {
    throw new BasecampError(
      "usage",
      `maxPages must be a positive integer no larger than ${Number.MAX_SAFE_INTEGER}, got ${String(maxPages)}`,
      { hint: "Pass a whole number greater than 0, or omit maxPages to use the default cap." }
    );
  }
}

/**
 * Default maximum pages to follow as a safety cap against infinite loops.
 *
 * Lives here rather than in base.ts because both base.ts and client.ts need it
 * and this module is the one they can both import without a cycle. A malformed
 * or hostile Link header can name the page it was served from, so every loop
 * that follows rel="next" needs a bound that does not depend on the server
 * ever stopping.
 */
export const DEFAULT_MAX_PAGES = 10_000;

/**
 * Parses the next URL from a Link header.
 * Looks for rel="next" in the header value.
 *
 * @param linkHeader - The Link header value
 * @returns The URL for the next page, or null if not found
 */
export function parseNextLink(linkHeader: string | null): string | null {
  if (!linkHeader) return null;

  for (const part of linkHeader.split(",")) {
    const trimmed = part.trim();
    if (trimmed.includes('rel="next"')) {
      // Keep scanning if this part is malformed. The previous code returned
      // unconditionally here, so one unparseable `rel="next"` segment
      // short-circuited the whole header to null; the other five SDKs fall
      // through and keep looking.
      const url = extractAngleBracketed(trimmed);
      if (url !== null) return url;
    }
  }

  return null;
}

/**
 * Resolves a possibly-relative URL against a base URL.
 * If target is already absolute, it is returned unchanged.
 */
export function resolveURL(base: string, target: string): string {
  try {
    return new URL(target, base).href;
  } catch {
    return target;
  }
}

/**
 * Checks whether two absolute URLs share the same origin (scheme + host + port).
 * Handles default port normalization (e.g. https://example.com:443 === https://example.com).
 * Relative URLs should be resolved with resolveURL before calling this function.
 */
export function isSameOrigin(a: string, b: string): boolean {
  try {
    const urlA = new URL(a);
    const urlB = new URL(b);
    return urlA.origin === urlB.origin;
  } catch {
    return false;
  }
}
