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
