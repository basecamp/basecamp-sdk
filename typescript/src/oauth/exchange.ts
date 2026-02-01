/**
 * OAuth 2.0 token exchange and refresh for Basecamp SDK.
 *
 * Handles authorization code exchange and token refresh operations.
 * Supports both standard OAuth 2.0 and Basecamp's Launchpad legacy format.
 */

import { BasecampError } from "../errors.js";
import { isLocalhost } from "../security.js";
import type {
  ExchangeRequest,
  RefreshRequest,
  OAuthToken,
  RawTokenResponse,
  OAuthErrorResponse,
} from "./types.js";

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

  const { fetch: customFetch = globalThis.fetch, timeoutMs = 30000 } = options;

  // Create abort controller for timeout
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await customFetch(tokenEndpoint, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        Accept: "application/json",
      },
      body: body.toString(),
      signal: controller.signal,
    });

    const MAX_TOKEN_RESPONSE_BYTES = 1 * 1024 * 1024; // 1 MB

    // Use streaming reader with true byte-level limit enforcement.
    // This handles cases where Content-Length is absent or inaccurate.
    const responseText = await readResponseWithByteLimit(
      response,
      MAX_TOKEN_RESPONSE_BYTES,
      response.status
    );

    let data: RawTokenResponse | OAuthErrorResponse;

    try {
      data = JSON.parse(responseText);
    } catch {
      // Truncate response text to avoid leaking sensitive data in error messages
      const truncated = responseText.length > 500 ? responseText.slice(0, 497) + "..." : responseText;
      throw new BasecampError(
        "api_error",
        `Failed to parse token response: ${truncated}`,
        { httpStatus: response.status }
      );
    }

    // Check for error response
    if (!response.ok) {
      const errorData = data as OAuthErrorResponse;
      const rawMessage = errorData.error_description || errorData.error || "Token request failed";
      const message = rawMessage.length > 500 ? rawMessage.slice(0, 497) + "..." : rawMessage;

      if (response.status === 401 || errorData.error === "invalid_grant") {
        throw new BasecampError("auth", message, {
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

    if (!tokenData.access_token) {
      throw new BasecampError("api_error", "Token response missing access_token");
    }

    return {
      accessToken: tokenData.access_token,
      refreshToken: tokenData.refresh_token,
      tokenType: tokenData.token_type || "Bearer",
      expiresIn: tokenData.expires_in,
      expiresAt: tokenData.expires_in
        ? new Date(Date.now() + tokenData.expires_in * 1000)
        : undefined,
      scope: tokenData.scope,
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
