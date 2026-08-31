/**
 * OAuth 2.0 token exchange and refresh for Basecamp SDK.
 *
 * Handles authorization code exchange and token refresh operations.
 * Supports both standard OAuth 2.0 and Basecamp's Launchpad legacy format.
 */

import { MAX_TOKEN_LIFETIME_SECONDS, raceAbort, resolveRequestTimeoutMs } from "./limits.js";
import { BasecampError } from "../errors.js";
import { isLocalhost } from "../security.js";
import type {
  ExchangeRequest,
  RefreshRequest,
  OAuthToken,
  RawTokenResponse,
  OAuthErrorResponse,
} from "./types.js";

/** Default per-request timeout (ms) for a token exchange/refresh round trip. */
const DEFAULT_TOKEN_TIMEOUT_MS = 30_000;

/**
 * The redirects the token endpoint is refused (SPEC §16 "Token-Endpoint
 * Transport Policy") — the same set the signed download hop refuses (§14).
 * 304 is not in the set: it is a cache validator, not a redirect-with-
 * Location, and falls through to the generic non-ok handling below.
 */
const REDIRECT_STATUSES = [301, 302, 303, 307, 308];

/**
 * Options for token exchange/refresh operations.
 */
export interface TokenOptions {
  /** Custom fetch function for testing or custom HTTP handling */
  fetch?: typeof globalThis.fetch;
  /** Request timeout in milliseconds (default: 30000) */
  timeoutMs?: number;
}

/**
 * Exchanges an authorization code for access and refresh tokens.
 *
 * Supports both standard OAuth 2.0 and Basecamp's Launchpad legacy format.
 * Use `useLegacyFormat: true` for Launchpad compatibility.
 *
 * @param request - Exchange request parameters
 * @param options - Optional configuration
 * @returns The token response
 * @throws BasecampError on validation, network, or authentication errors
 *
 * @example
 * ```ts
 * // Standard OAuth 2.0
 * const token = await exchangeCode({
 *   tokenEndpoint: config.tokenEndpoint,
 *   code: "auth_code_from_callback",
 *   redirectUri: "https://myapp.com/callback",
 *   clientId: "my_client_id",
 *   clientSecret: "my_client_secret",
 * });
 *
 * // Launchpad legacy format
 * const token = await exchangeCode({
 *   tokenEndpoint: "https://launchpad.37signals.com/authorization/token",
 *   code: "auth_code",
 *   redirectUri: "https://myapp.com/callback",
 *   clientId: "my_client_id",
 *   clientSecret: "my_client_secret",
 *   useLegacyFormat: true,
 * });
 * ```
 */
export async function exchangeCode(
  request: ExchangeRequest,
  options: TokenOptions = {}
): Promise<OAuthToken> {
  // Validate required fields
  if (!request.tokenEndpoint) {
    throw new BasecampError("validation", "Token endpoint is required");
  }
  if (!request.code) {
    throw new BasecampError("validation", "Authorization code is required");
  }
  if (!request.redirectUri) {
    throw new BasecampError("validation", "Redirect URI is required");
  }
  if (!request.clientId) {
    throw new BasecampError("validation", "Client ID is required");
  }

  // Build request body
  const body = new URLSearchParams();

  if (request.useLegacyFormat) {
    // Launchpad uses non-standard "type" parameter
    body.set("type", "web_server");
  } else {
    // Standard OAuth 2.0
    body.set("grant_type", "authorization_code");
  }

  body.set("code", request.code);
  body.set("redirect_uri", request.redirectUri);
  body.set("client_id", request.clientId);

  if (request.clientSecret) {
    body.set("client_secret", request.clientSecret);
  }
  if (request.codeVerifier) {
    body.set("code_verifier", request.codeVerifier);
  }

  return doTokenRequest(request.tokenEndpoint, body, options);
}

/**
 * Refreshes an access token using a refresh token.
 *
 * Supports both standard OAuth 2.0 and Basecamp's Launchpad legacy format.
 * Use `useLegacyFormat: true` for Launchpad compatibility.
 *
 * @param request - Refresh request parameters
 * @param options - Optional configuration
 * @returns The new token response
 * @throws BasecampError on validation, network, or authentication errors
 *
 * @example
 * ```ts
 * // Standard OAuth 2.0
 * const newToken = await refreshToken({
 *   tokenEndpoint: config.tokenEndpoint,
 *   refreshToken: oldToken.refreshToken,
 *   clientId: "my_client_id",
 *   clientSecret: "my_client_secret",
 * });
 *
 * // Launchpad legacy format
 * const newToken = await refreshToken({
 *   tokenEndpoint: "https://launchpad.37signals.com/authorization/token",
 *   refreshToken: oldToken.refreshToken,
 *   useLegacyFormat: true,
 * });
 * ```
 */
export async function refreshToken(
  request: RefreshRequest,
  options: TokenOptions = {}
): Promise<OAuthToken> {
  // Validate required fields
  if (!request.tokenEndpoint) {
    throw new BasecampError("validation", "Token endpoint is required");
  }
  if (!request.refreshToken) {
    throw new BasecampError("validation", "Refresh token is required");
  }

  // Build request body
  const body = new URLSearchParams();

  if (request.useLegacyFormat) {
    // Launchpad uses non-standard "type" parameter
    body.set("type", "refresh");
  } else {
    // Standard OAuth 2.0
    body.set("grant_type", "refresh_token");
  }

  body.set("refresh_token", request.refreshToken);

  if (request.clientId) {
    body.set("client_id", request.clientId);
  }
  if (request.clientSecret) {
    body.set("client_secret", request.clientSecret);
  }
  if (request.resource) {
    body.set("resource", request.resource);
  }

  return doTokenRequest(request.tokenEndpoint, body, options);
}

/**
 * Validates that a URL uses the HTTPS scheme (allows localhost for testing).
 */
function requireHTTPS(url: string, label: string): void {
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== "https:" && !isLocalhost(parsed.hostname)) {
      throw new BasecampError("validation", `${label} must use HTTPS: ${url}`);
    }
  } catch (err) {
    if (err instanceof BasecampError) throw err;
    throw new BasecampError("validation", `Invalid ${label}: ${url}`);
  }
}

/**
 * Parses Content-Length header, returning null for missing/invalid values.
 * Treats non-numeric or negative values as invalid (fail closed).
 */
function parseContentLength(header: string | null): number | null {
  if (!header) return null;
  const parsed = parseInt(header, 10);
  // NaN, negative, or non-integer values are invalid
  if (!Number.isInteger(parsed) || parsed < 0) return null;
  // Reject if the header contains non-digit characters (e.g., "123abc")
  if (!/^\d+$/.test(header.trim())) return null;
  return parsed;
}

/**
 * Reads a response body with streaming byte limit.
 * Uses true byte counting even when Content-Length is absent or inaccurate.
 */
async function readResponseWithByteLimit(
  response: Response,
  maxBytes: number,
  httpStatus: number
): Promise<string> {
  // Parse Content-Length, treating invalid values as missing (fail closed)
  const contentLengthBytes = parseContentLength(
    response.headers.get("Content-Length")
  );

  // Fast path: reject if Content-Length exceeds limit
  if (contentLengthBytes !== null && contentLengthBytes > maxBytes) {
    throw new BasecampError(
      "api_error",
      `Token response too large (Content-Length: ${contentLengthBytes} bytes, max ${maxBytes})`,
      { httpStatus }
    );
  }

  // If no body or body isn't streamable, require valid Content-Length
  if (!response.body) {
    // If Content-Length is missing or invalid, we cannot protect against DoS - fail closed.
    // If it was valid and within limits, we can safely read.
    if (contentLengthBytes === null) {
      throw new BasecampError(
        "api_error",
        "Cannot safely read token response: no valid Content-Length header and streaming unavailable",
        { httpStatus }
      );
    }
    // Content-Length was valid and within limits, safe to read
    return response.text();
  }

  // Stream with byte counting
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let totalBytes = 0;

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      totalBytes += value.length;
      if (totalBytes > maxBytes) {
        reader.cancel();
        throw new BasecampError(
          "api_error",
          `Token response too large (${totalBytes}+ bytes, max ${maxBytes})`,
          { httpStatus }
        );
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }

  // Concatenate and decode
  const combined = new Uint8Array(totalBytes);
  let offset = 0;
  for (const chunk of chunks) {
    combined.set(chunk, offset);
    offset += chunk.length;
  }

  return new TextDecoder().decode(combined);
}

/**
 * Performs the actual HTTP token request.
 */
async function doTokenRequest(
  tokenEndpoint: string,
  body: URLSearchParams,
  options: TokenOptions
): Promise<OAuthToken> {
  requireHTTPS(tokenEndpoint, "token endpoint");

  const { fetch: customFetch = globalThis.fetch, timeoutMs = DEFAULT_TOKEN_TIMEOUT_MS } = options;

  // Create abort controller for timeout. The clamp closes the unbounded
  // hole an unvalidated timeoutMs left open: NaN/Infinity made setTimeout
  // fire at ~1 ms (an instant abort masquerading as a network failure), and
  // an oversized finite value could hold a stalled request open for weeks.
  const controller = new AbortController();
  const timeoutId = setTimeout(
    () => controller.abort(),
    resolveRequestTimeoutMs(timeoutMs, DEFAULT_TOKEN_TIMEOUT_MS)
  );

  try {
    // The whole round trip runs INSIDE a race against the controller's
    // signal, like the device flow's POSTs: a cooperative fetch rejects on
    // abort anyway, but a custom fetch that ignores its AbortSignal must not
    // hold the exchange past its timeout — the race rejects the moment the
    // timeout fires, and a late settlement is discarded.
    const { response, responseText } = await raceAbort(controller.signal, async () => {
      const raced = await customFetch(tokenEndpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          Accept: "application/json",
        },
        body: body.toString(),
        signal: controller.signal,
        // Never chase an attacker-influenced Location: the token endpoint may
        // come from discovered metadata, and a followed 307/308 would re-POST
        // the credentials wherever it points (SPEC §16).
        redirect: "manual",
      });

      // A browser fetch answers redirect: "manual" with an opaqueredirect —
      // type "opaqueredirect", status 0, no headers — rather than the 3xx
      // Node hands back, so the status table below cannot see it. Refuse it
      // by type; the real status is hidden, so the error carries none.
      if (raced.type === "opaqueredirect") {
        void raced.body?.cancel().catch(() => {});
        throw new BasecampError(
          "api_error",
          "redirect on the token endpoint is not followed (status hidden by the browser's opaque redirect)"
        );
      }

      // A suppressed redirect is refused by status BEFORE any body read, so a
      // 3xx whose body stalls forever cannot degrade into a timeout. Release
      // the unread stream (non-blocking) so refusals don't retain sockets.
      if (REDIRECT_STATUSES.includes(raced.status)) {
        void raced.body?.cancel().catch(() => {});
        throw new BasecampError(
          "api_error",
          `redirect ${raced.status} on the token endpoint is not followed`,
          { httpStatus: raced.status }
        );
      }

      const MAX_TOKEN_RESPONSE_BYTES = 1 * 1024 * 1024; // 1 MB

      // Use streaming reader with true byte-level limit enforcement.
      // This handles cases where Content-Length is absent or inaccurate.
      const racedText = await readResponseWithByteLimit(
        raced,
        MAX_TOKEN_RESPONSE_BYTES,
        raced.status
      );
      return { response: raced, responseText: racedText };
    });

    let data: RawTokenResponse | OAuthErrorResponse;

    try {
      data = JSON.parse(responseText);
    } catch {
      // A token response that fails to parse may still contain credential
      // material (a syntactically-broken body carrying an access_token) —
      // never echo ANY of it into an error message, where it would reach
      // logs and exception telemetry. The status is diagnosis enough.
      throw new BasecampError("api_error", "Failed to parse token response", {
        httpStatus: response.status,
      });
    }

    // A valid-JSON-but-non-object body (null, array, number, string) is a
    // malformed response on EVERY status — the error branch below would
    // otherwise deref null and surface a raw TypeError misclassified as
    // retryable network. Fail as api_error carrying the HTTP status.
    if (typeof data !== "object" || data === null || Array.isArray(data)) {
      throw new BasecampError("api_error", "Token response is not a JSON object", {
        httpStatus: response.status,
      });
    }

    // Check for error response
    if (!response.ok) {
      const errorData = data as OAuthErrorResponse;
      // Non-string error/error_description (numbers, objects) must not throw
      // a raw TypeError below — that would be misclassified as retryable
      // network, losing the api_error status context.
      const rawMessage =
        (typeof errorData.error_description === "string" && errorData.error_description) ||
        (typeof errorData.error === "string" && errorData.error) ||
        "Token request failed";
      const message = rawMessage.length > 500 ? rawMessage.slice(0, 497) + "..." : rawMessage;

      if (response.status === 401 || errorData.error === "invalid_grant") {
        throw new BasecampError("auth_required", message, {
          httpStatus: response.status,
          hint: "The authorization code or refresh token may be invalid or expired",
        });
      }

      throw new BasecampError("api_error", message, {
        httpStatus: response.status,
      });
    }

    // Parse successful response
    const tokenData = data as RawTokenResponse;

    // Non-empty STRING, not merely truthy: a numeric access_token is not a
    // usable credential. Carry the HTTP status like every other malformed-
    // response raise so failures are diagnosable.
    if (typeof tokenData.access_token !== "string" || tokenData.access_token === "") {
      throw new BasecampError("api_error", "Token response missing or non-string access_token", {
        httpStatus: response.status,
      });
    }

    // Absent/null token_type defaults to Bearer, but a present-but-empty (or
    // non-string) one is a malformed response — matching the stricter
    // device-flow validation rather than silently coercing "" to Bearer.
    if (tokenData.token_type != null && (typeof tokenData.token_type !== "string" || tokenData.token_type === "")) {
      throw new BasecampError("api_error", "Token response token_type must be a non-empty string when present", {
        httpStatus: response.status,
      });
    }

    // The remaining optional fields get the device-flow strictness: a
    // non-string refresh_token/scope or a non-finite/fractional/oversized
    // expires_in would leak malformed values through the public OAuthToken
    // type (or build an Invalid Date expiry).
    if (tokenData.refresh_token != null && typeof tokenData.refresh_token !== "string") {
      throw new BasecampError("api_error", "Token response refresh_token must be a string", {
        httpStatus: response.status,
      });
    }
    if (tokenData.scope != null && typeof tokenData.scope !== "string") {
      throw new BasecampError("api_error", "Token response scope must be a string", {
        httpStatus: response.status,
      });
    }
    if (
      tokenData.expires_in != null &&
      (typeof tokenData.expires_in !== "number" ||
        !Number.isInteger(tokenData.expires_in) ||
        tokenData.expires_in <= 0 ||
        tokenData.expires_in > MAX_TOKEN_LIFETIME_SECONDS)
    ) {
      throw new BasecampError(
        "api_error",
        `Token response expires_in must be a finite positive whole number no greater than ${MAX_TOKEN_LIFETIME_SECONDS} seconds`,
        { httpStatus: response.status }
      );
    }

    // resource: absent and JSON null are unset; when present it must be a
    // non-empty string (SPEC §16) — an empty binding is not a binding.
    if (tokenData.resource != null && (typeof tokenData.resource !== "string" || tokenData.resource === "")) {
      throw new BasecampError("api_error", "Token response resource must be a non-empty string when present", {
        httpStatus: response.status,
      });
    }

    return {
      accessToken: tokenData.access_token,
      // `?? undefined`: JSON null is legal on the wire for the optional
      // fields (absent per SPEC) — never leak null through the public type.
      refreshToken: tokenData.refresh_token ?? undefined,
      tokenType: tokenData.token_type ?? "Bearer",
      expiresIn: tokenData.expires_in,
      expiresAt: tokenData.expires_in
        ? new Date(Date.now() + tokenData.expires_in * 1000)
        : undefined,
      scope: tokenData.scope ?? undefined,
      resource: tokenData.resource ?? undefined,
    };
  } catch (err) {
    if (err instanceof BasecampError) {
      throw err;
    }

    if (err instanceof Error) {
      if (err.name === "AbortError") {
        throw new BasecampError("network", "Token request timed out", {
          cause: err,
          retryable: true,
        });
      }

      throw new BasecampError("network", `Token request failed: ${err.message}`, {
        cause: err,
        retryable: true,
      });
    }

    throw new BasecampError("network", "Token request failed with unknown error", {
      retryable: true,
    });
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * Checks if a token is expired or about to expire.
 *
 * @param token - The token to check
 * @param bufferSeconds - Buffer time before actual expiration (default: 60)
 * @returns true if the token is expired or will expire within the buffer time
 *
 * @example
 * ```ts
 * if (isTokenExpired(token)) {
 *   token = await refreshToken({ ... });
 * }
 * ```
 */
export function isTokenExpired(token: OAuthToken, bufferSeconds = 60): boolean {
  if (!token.expiresAt) {
    // No expiration info - assume not expired
    return false;
  }

  const bufferMs = bufferSeconds * 1000;
  return Date.now() + bufferMs >= token.expiresAt.getTime();
}
