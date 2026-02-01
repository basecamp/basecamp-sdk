/**
 * OAuth 2.0 discovery for Basecamp SDK.
 *
 * Fetches OAuth server configuration from the well-known discovery endpoint.
 */

import { BasecampError } from "../errors.js";
import { isLocalhost } from "../security.js";
import type { OAuthConfig } from "./types.js";

/**
 * Raw discovery response from OAuth server.
 */
interface RawDiscoveryResponse {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
  registration_endpoint?: string;
  scopes_supported?: string[];
}

/**
 * Options for OAuth discovery.
 */
export interface DiscoverOptions {
  /** Custom fetch function for testing or custom HTTP handling */
  fetch?: typeof globalThis.fetch;
  /** Request timeout in milliseconds (default: 10000) */
  timeoutMs?: number;
}

/**
 * Discovers OAuth server configuration from the well-known endpoint.
 *
 * Fetches the OAuth 2.0 Authorization Server Metadata from:
 * `{baseUrl}/.well-known/oauth-authorization-server`
 *
 * @param baseUrl - The OAuth server's base URL (e.g., "https://launchpad.37signals.com")
 * @param options - Optional configuration
 * @returns The OAuth server configuration
 * @throws BasecampError on network or parsing errors
 *
 * @example
 * ```ts
 * const config = await discover("https://launchpad.37signals.com");
 * console.log(config.tokenEndpoint);
 * // => "https://launchpad.37signals.com/authorization/token"
 * ```
 */
export async function discover(
  baseUrl: string,
  options: DiscoverOptions = {}
): Promise<OAuthConfig> {
  const { fetch: customFetch = globalThis.fetch, timeoutMs = 10000 } = options;

  // Validate HTTPS before making any network request (allow localhost for testing)
  try {
    const parsed = new URL(baseUrl);
    if (parsed.protocol !== "https:" && !isLocalhost(parsed.hostname)) {
      throw new BasecampError("validation", `OAuth discovery base URL must use HTTPS: ${baseUrl}`);
    }
  } catch (err) {
    if (err instanceof BasecampError) throw err;
    throw new BasecampError("validation", `Invalid OAuth discovery base URL: ${baseUrl}`);
  }

  // Normalize base URL (remove trailing slash)
  const normalizedBase = baseUrl.replace(/\/$/, "");
  const discoveryUrl = `${normalizedBase}/.well-known/oauth-authorization-server`;

  // Create abort controller for timeout
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await customFetch(discoveryUrl, {
      method: "GET",
      headers: {
        Accept: "application/json",
      },
      signal: controller.signal,
    });

    if (!response.ok) {
      const body = await response.text().catch(() => "");
      throw new BasecampError(
        "network",
        `OAuth discovery failed with status ${response.status}: ${body}`,
        { httpStatus: response.status }
      );
    }

    const data = (await response.json()) as RawDiscoveryResponse;

    // Validate required fields
    if (!data.issuer || !data.authorization_endpoint || !data.token_endpoint) {
      throw new BasecampError(
        "api_error",
        "Invalid OAuth discovery response: missing required fields"
      );
    }

    return {
      issuer: data.issuer,
      authorizationEndpoint: data.authorization_endpoint,
      tokenEndpoint: data.token_endpoint,
      registrationEndpoint: data.registration_endpoint,
      scopesSupported: data.scopes_supported,
    };
  } catch (err) {
    if (err instanceof BasecampError) {
      throw err;
    }

    if (err instanceof Error) {
      if (err.name === "AbortError") {
        throw new BasecampError("network", "OAuth discovery request timed out", {
          cause: err,
          retryable: true,
        });
      }

      throw new BasecampError("network", `OAuth discovery failed: ${err.message}`, {
        cause: err,
        retryable: true,
      });
    }

    throw new BasecampError("network", "OAuth discovery failed with unknown error", {
      retryable: true,
    });
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * Default Basecamp/Launchpad OAuth server URL.
 */
export const LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com";

/**
 * Discovers OAuth configuration from Basecamp's Launchpad server.
 *
 * Convenience function that calls discover() with the Launchpad base URL.
 *
 * @param options - Optional configuration
 * @returns The OAuth server configuration
 *
 * @example
 * ```ts
 * const config = await discoverLaunchpad();
 * // Use config.authorizationEndpoint to start OAuth flow
 * ```
 */
export async function discoverLaunchpad(
  options: DiscoverOptions = {}
): Promise<OAuthConfig> {
  return discover(LAUNCHPAD_BASE_URL, options);
}
