/**
 * Interactive OAuth login flow for CLI and desktop applications.
 *
 * Orchestrates the full OAuth 2.0 authorization code flow:
 * discovery, PKCE, local callback server, browser launch, code exchange.
 */

import { discover, discoverLaunchpad, discoverFromResource, isLaunchpadIssuer } from "./discovery.js";
import { generateState, generatePKCE } from "./pkce.js";
import { buildAuthorizationUrl } from "./authorize.js";
import { startCallbackServer } from "./callback-server.js";
import { exchangeCode } from "./exchange.js";
import type { TokenStore } from "./token-store.js";
import type { OAuthToken } from "./types.js";
import { BasecampError } from "../errors.js";

/**
 * Options for the interactive login flow.
 */
export interface InteractiveLoginOptions {
  /** The client identifier */
  clientId: string;
  /** The client secret (optional for public clients) */
  clientSecret?: string;
  /** Token store for persisting the resulting token */
  store: TokenStore;
  /**
   * Legacy single-hop AS base URL (RFC 8414). Mutually exclusive with
   * {@link resourceBaseUrl}. Defaults to Launchpad when neither is set.
   */
  baseUrl?: string;
  /**
   * Resource-first (two-hop) discovery origin — the API/resource host (RFC
   * 9728). Mutually exclusive with {@link baseUrl}. Soft failures fall back to
   * Launchpad; hard selection errors propagate.
   */
  resourceBaseUrl?: string;
  /**
   * Explicit issuer selection for resource-first discovery (passed through to
   * `discoverFromResource`).
   */
  expectedIssuer?: string;
  /**
   * Use Launchpad's non-standard token format. When omitted, it is derived from
   * the selected flow (legacy for Launchpad, standard for a first-party issuer);
   * the legacy `baseUrl` path defaults to `true`.
   */
  useLegacyFormat?: boolean;
  /** Port for the local callback server (optional) */
  callbackPort?: number;
  /** Function to open the authorization URL in a browser */
  openBrowser: (url: string) => Promise<void>;
  /** Fallback for when browser launch fails — prompt user to visit URL manually */
  promptForManualVisit?: (authUrl: string) => Promise<void>;
  /** Status callback for progress messages */
  onStatus?: (message: string) => void;
}

/**
 * Performs the full interactive OAuth login flow.
 *
 * Steps:
 * 1. Discover OAuth endpoints
 * 2. Generate PKCE and state parameters
 * 3. Start local callback server
 * 4. Build authorization URL and open browser
 * 5. Wait for callback with authorization code
 * 6. Exchange code for tokens
 * 7. Save token to store
 *
 * @param options - Login flow configuration
 * @returns The resulting OAuth token
 * @throws BasecampError on any flow failure
 *
 * @example
 * ```ts
 * import open from "open"; // or use child_process
 *
 * const token = await performInteractiveLogin({
 *   clientId: process.env.BASECAMP_CLIENT_ID!,
 *   clientSecret: process.env.BASECAMP_CLIENT_SECRET!,
 *   store: new FileTokenStore("~/.config/basecamp/tokens.json"),
 *   openBrowser: (url) => open(url),
 *   onStatus: (msg) => console.log(msg),
 * });
 * ```
 */
export async function performInteractiveLogin(
  options: InteractiveLoginOptions,
): Promise<OAuthToken> {
  const {
    clientId,
    clientSecret,
    store,
    baseUrl,
    resourceBaseUrl,
    expectedIssuer,
    useLegacyFormat,
    callbackPort,
    openBrowser,
    promptForManualVisit,
    onStatus,
  } = options;

  if (baseUrl && resourceBaseUrl) {
    throw new BasecampError(
      "usage",
      "baseUrl and resourceBaseUrl are mutually exclusive discovery modes; supply only one"
    );
  }

  // 1. Discover OAuth endpoints
  onStatus?.("Discovering OAuth endpoints...");
  const { config, legacy } = await discoverEndpoints({
    baseUrl,
    resourceBaseUrl,
    expectedIssuer,
    useLegacyFormat,
    onStatus,
  });

  // Authorization-code flow requires an authorization endpoint. It is optional
  // in discovery now (device-only servers omit it), so assert presence here.
  if (!config.authorizationEndpoint) {
    throw new BasecampError(
      "validation",
      "Selected authorization server does not advertise an authorization_endpoint; " +
        "authorization-code login is unavailable for this issuer"
    );
  }

  // 2. Generate PKCE and state
  const state = generateState();
  const serverSupportsPKCE = config.codeChallengeMethodsSupported?.includes("S256") ?? false;
  const pkce = serverSupportsPKCE ? await generatePKCE() : undefined;

  // 3. Start callback server
  onStatus?.("Starting callback server...");
  const { url: redirectUri, waitForCallback, close } = await startCallbackServer({
    port: callbackPort,
    expectedState: state,
  });

  try {
    // 4. Build authorization URL
    const authUrl = buildAuthorizationUrl({
      authorizationEndpoint: config.authorizationEndpoint,
      clientId,
      redirectUri,
      state,
      pkce,
    });

    // 5. Open browser
    onStatus?.("Opening browser for authorization...");
    let browserOpened = false;
    try {
      await openBrowser(authUrl.toString());
      browserOpened = true;
    } catch {
      // Browser launch failed
    }

    if (!browserOpened) {
      if (promptForManualVisit) {
        await promptForManualVisit(authUrl.toString());
      } else {
        throw new BasecampError("auth_required", "Failed to open browser and no manual visit prompt configured");
      }
    }

    // 6. Wait for callback
    onStatus?.("Waiting for authorization...");
    const { code } = await waitForCallback();

    // 7. Exchange code for tokens
    onStatus?.("Exchanging authorization code for tokens...");
    const token = await exchangeCode({
      tokenEndpoint: config.tokenEndpoint,
      code,
      redirectUri,
      clientId,
      clientSecret,
      codeVerifier: pkce?.verifier,
      useLegacyFormat: legacy,
    });

    // 8. Save token
    await store.save(token);
    onStatus?.("Authorization complete.");

    return token;
  } finally {
    close();
  }
}

/**
 * Resolves the OAuth config for the login flow across the two discovery modes,
 * and decides whether to send Launchpad's legacy token format.
 *
 * Resource-first mode falls back to Launchpad ONLY on the two soft reasons; any
 * hard selection error (issuer mismatch, AS fetch failure, ambiguity, …) throws
 * from `discoverFromResource` and propagates here — never a silent Launchpad
 * request.
 */
async function discoverEndpoints(opts: {
  baseUrl?: string;
  resourceBaseUrl?: string;
  expectedIssuer?: string;
  useLegacyFormat?: boolean;
  onStatus?: (message: string) => void;
}): Promise<{ config: import("./types.js").OAuthConfig; legacy: boolean }> {
  const { baseUrl, resourceBaseUrl, expectedIssuer, useLegacyFormat, onStatus } = opts;

  if (resourceBaseUrl) {
    const result = await discoverFromResource(resourceBaseUrl, { expectedIssuer });
    if (result.kind === "selected") {
      // Derive the token format from the selected issuer unless the caller
      // pinned it explicitly.
      const legacy = useLegacyFormat ?? isLaunchpadIssuer(result.config.issuer);
      return { config: result.config, legacy };
    }
    // Soft fallback (resource_discovery_failed | no_as_advertised) → Launchpad.
    onStatus?.(`Resource discovery fell back to Launchpad (${result.reason}).`);
    const config = await discoverLaunchpad();
    return { config, legacy: useLegacyFormat ?? true };
  }

  const config = baseUrl ? await discover(baseUrl) : await discoverLaunchpad();
  // Legacy single-hop path keeps its historical default of legacy=true.
  return { config, legacy: useLegacyFormat ?? true };
}
