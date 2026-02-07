/**
 * Pagination utility functions.
 *
 * Extracted to its own module to avoid circular dependencies between
 * client.ts (which imports generated services) and base.ts (which
 * generated services extend).
 */

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
      const match = trimmed.match(/<([^>]+)>/);
      return match?.[1] ?? null;
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
