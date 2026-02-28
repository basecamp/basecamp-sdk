/**
 * Interactive OAuth login flow for CLI and desktop applications.
 *
 * Orchestrates the full OAuth 2.0 authorization code flow:
 * discovery, PKCE, local callback server, browser launch, code exchange.
 */

import { discover, discoverLaunchpad } from "./discovery.js";
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
  /** OAuth server base URL (defaults to Launchpad) */
  baseUrl?: string;
  /** Use Launchpad's non-standard token format (default: true) */
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
    useLegacyFormat = true,
    callbackPort,
    openBrowser,
    promptForManualVisit,
    onStatus,
  } = options;

  // 1. Discover OAuth endpoints
  onStatus?.("Discovering OAuth endpoints...");
  const config = baseUrl ? await discover(baseUrl) : await discoverLaunchpad();

  // 2. Generate PKCE and state
  const state = generateState();
  const pkce = await generatePKCE();

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
      codeVerifier: pkce.verifier,
      useLegacyFormat,
    });

    // 8. Save token
    await store.save(token);
    onStatus?.("Authorization complete.");

    return token;
  } finally {
    close();
  }
}
