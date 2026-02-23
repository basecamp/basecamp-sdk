/**
 * OAuth 2.0 authorization URL construction for Basecamp SDK.
 *
 * Builds the authorization URL to redirect users to the OAuth provider.
 */

import { BasecampError } from "../errors.js";
import { isLocalhost } from "../security.js";
import type { PKCE } from "./pkce.js";

/**
 * Parameters for building an OAuth authorization URL.
 */
export interface AuthorizeParams {
  /** URL of the authorization endpoint */
  authorizationEndpoint: string;
  /** The client identifier */
  clientId: string;
  /** The redirect URI for the callback */
  redirectUri: string;
  /** CSRF protection state parameter */
  state: string;
  /** PKCE parameters (optional) */
  pkce?: PKCE;
  /** OAuth scope to request (optional) */
  scope?: string;
}

/**
 * Builds an OAuth 2.0 authorization URL.
 *
 * Constructs a URL with the required OAuth parameters for initiating
 * the authorization code flow. Validates that the endpoint uses HTTPS
 * (localhost is allowed for development).
 *
 * @param params - Authorization parameters
 * @returns The complete authorization URL
 * @throws BasecampError on validation errors (non-HTTPS, invalid URL)
 *
 * @example
 * ```ts
 * const url = buildAuthorizationUrl({
 *   authorizationEndpoint: config.authorizationEndpoint,
 *   clientId: "my_client_id",
 *   redirectUri: "http://localhost:14923/callback",
 *   state: generateState(),
 *   pkce: await generatePKCE(),
 * });
 * // Redirect user to url.toString()
 * ```
 */
export function buildAuthorizationUrl(params: AuthorizeParams): URL {
  const { authorizationEndpoint, clientId, redirectUri, state, pkce, scope } = params;

  let url: URL;
  try {
    url = new URL(authorizationEndpoint);
  } catch {
    throw new BasecampError("validation", `Invalid authorization endpoint URL: ${authorizationEndpoint}`);
  }

  if (url.protocol !== "https:" && !isLocalhost(url.hostname)) {
    throw new BasecampError("validation", `Authorization endpoint must use HTTPS: ${authorizationEndpoint}`);
  }

  url.searchParams.set("response_type", "code");
  url.searchParams.set("client_id", clientId);
  url.searchParams.set("redirect_uri", redirectUri);
  url.searchParams.set("state", state);

  if (pkce) {
    url.searchParams.set("code_challenge", pkce.challenge);
    url.searchParams.set("code_challenge_method", "S256");
  }

  if (scope) {
    url.searchParams.set("scope", scope);
  }

  return url;
}
