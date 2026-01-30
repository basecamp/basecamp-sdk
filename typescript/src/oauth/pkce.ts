/**
 * PKCE (Proof Key for Code Exchange) utilities for OAuth 2.0.
 *
 * Provides cryptographically secure code verifier and challenge generation
 * to protect against authorization code interception attacks.
 */

/**
 * PKCE parameters for OAuth 2.0 authorization code flow with PKCE.
 */
export interface PKCE {
  /** The code_verifier to send during token exchange */
  verifier: string;
  /** The code_challenge (SHA256 hash of verifier) to send during authorization */
  challenge: string;
}

/**
 * Base64url encodes a Uint8Array without padding.
 */
function base64UrlEncode(bytes: Uint8Array): string {
  // Convert to base64
  const base64 = btoa(String.fromCharCode(...bytes));
  // Convert to base64url (replace + with -, / with _, remove padding)
  return base64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, "");
}

/**
 * Generates a cryptographically secure PKCE code verifier and challenge.
 *
 * The verifier is 43 characters (32 random bytes, base64url-encoded).
 * The challenge is the base64url-encoded SHA256 hash of the verifier.
 *
 * Use code_challenge_method=S256 with the challenge in the authorization request.
 *
 * @example
 * ```ts
 * const pkce = await generatePKCE();
 *
 * // In authorization request:
 * const authUrl = new URL(authEndpoint);
 * authUrl.searchParams.set("code_challenge", pkce.challenge);
 * authUrl.searchParams.set("code_challenge_method", "S256");
 *
 * // Later, in token exchange:
 * const token = await exchangeCode({
 *   code,
 *   codeVerifier: pkce.verifier,
 *   // ...
 * });
 * ```
 */
export async function generatePKCE(): Promise<PKCE> {
  // Generate 32 random bytes
  const bytes = crypto.getRandomValues(new Uint8Array(32));
  const verifier = base64UrlEncode(bytes);

  // Compute SHA256 hash of the verifier
  const encoder = new TextEncoder();
  const hashBuffer = await crypto.subtle.digest("SHA-256", encoder.encode(verifier));
  const challenge = base64UrlEncode(new Uint8Array(hashBuffer));

  return { verifier, challenge };
}

/**
 * Generates a cryptographically secure OAuth state parameter.
 *
 * The state is 22 characters (16 random bytes, base64url-encoded).
 * Use this to prevent CSRF attacks on the OAuth flow.
 *
 * @example
 * ```ts
 * const state = generateState();
 *
 * // Store state before redirect:
 * sessionStorage.setItem("oauth_state", state);
 *
 * // In authorization request:
 * const authUrl = new URL(authEndpoint);
 * authUrl.searchParams.set("state", state);
 *
 * // In callback handler:
 * const returnedState = new URL(window.location.href).searchParams.get("state");
 * const savedState = sessionStorage.getItem("oauth_state");
 * if (returnedState !== savedState) {
 *   throw new Error("State mismatch - possible CSRF attack");
 * }
 * ```
 */
export function generateState(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return base64UrlEncode(bytes);
}
