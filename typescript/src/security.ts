/**
 * Security utilities for the Basecamp SDK.
 *
 * Provides helpers for safely logging HTTP requests without exposing
 * sensitive information like tokens and cookies.
 */

/**
 * Headers that contain sensitive values and should be redacted.
 */
const SENSITIVE_HEADERS = [
  "authorization",
  "cookie",
  "set-cookie",
  "x-csrf-token",
];

/**
 * Returns a copy of the headers with sensitive values replaced by "[REDACTED]".
 *
 * This is useful for safely logging HTTP requests and responses without
 * exposing tokens, cookies, or other credentials.
 *
 * Redacted headers:
 * - Authorization
 * - Cookie
 * - Set-Cookie
 * - X-CSRF-Token
 *
 * @example
 * ```ts
 * const safeHeaders = redactHeaders(response.headers);
 * console.log("Response headers:", safeHeaders);
 * // { "content-type": "application/json", "authorization": "[REDACTED]" }
 * ```
 */
export function redactHeaders(headers: Headers): Record<string, string> {
  const result: Record<string, string> = {};

  headers.forEach((value, key) => {
    const lowerKey = key.toLowerCase();
    if (SENSITIVE_HEADERS.includes(lowerKey)) {
      result[key] = "[REDACTED]";
    } else {
      result[key] = value;
    }
  });

  return result;
}

/**
 * Returns a copy of the header record with sensitive values replaced by "[REDACTED]".
 *
 * Similar to redactHeaders, but works with plain objects instead of Headers instances.
 *
 * @example
 * ```ts
 * const headers = { Authorization: "Bearer token", "Content-Type": "application/json" };
 * const safe = redactHeadersRecord(headers);
 * // { Authorization: "[REDACTED]", "Content-Type": "application/json" }
 * ```
 */
export function redactHeadersRecord(
  headers: Record<string, string>
): Record<string, string> {
  const result: Record<string, string> = {};

  for (const [key, value] of Object.entries(headers)) {
    const lowerKey = key.toLowerCase();
    if (SENSITIVE_HEADERS.includes(lowerKey)) {
      result[key] = "[REDACTED]";
    } else {
      result[key] = value;
    }
  }

  return result;
}

/**
 * Checks if a hostname represents localhost for development/testing purposes.
 *
 * Matches:
 * - "localhost" exactly
 * - Any subdomain of localhost (e.g., "api.localhost", "dev.api.localhost")
 * - IPv4 loopback "127.0.0.1"
 * - IPv6 loopback "::1"
 * - The .localhost TLD (RFC 6761) - any hostname ending in ".localhost"
 *
 * This is used to allow HTTP (non-HTTPS) connections during local development
 * while enforcing HTTPS for all production traffic.
 *
 * @param hostname - The hostname to check (without port)
 * @returns true if the hostname represents localhost
 *
 * @example
 * ```ts
 * isLocalhost("localhost");           // true
 * isLocalhost("api.localhost");       // true
 * isLocalhost("myapp.localhost");     // true
 * isLocalhost("127.0.0.1");           // true
 * isLocalhost("::1");                 // true
 * isLocalhost("example.com");         // false
 * isLocalhost("localhost.example.com"); // false
 * ```
 */
export function isLocalhost(hostname: string): boolean {
  const normalized = hostname.toLowerCase();
  return (
    normalized === "localhost" ||
    normalized === "127.0.0.1" ||
    normalized === "::1" ||
    normalized.endsWith(".localhost")
  );
}
