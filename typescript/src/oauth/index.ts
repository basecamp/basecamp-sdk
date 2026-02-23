/**
 * OAuth 2.0 module for Basecamp SDK.
 *
 * Provides OAuth discovery, token exchange, and token refresh functionality.
 * Supports both standard OAuth 2.0 and Basecamp's Launchpad legacy format.
 *
 * @example
 * ```ts
 * import { discover, exchangeCode, refreshToken, isTokenExpired } from "@basecamp/sdk/oauth";
 *
 * // 1. Discover OAuth configuration
 * const config = await discover("https://launchpad.37signals.com");
 *
 * // 2. Exchange authorization code for tokens
 * const token = await exchangeCode({
 *   tokenEndpoint: config.tokenEndpoint,
 *   code: "auth_code_from_callback",
 *   redirectUri: "https://myapp.com/callback",
 *   clientId: process.env.BASECAMP_CLIENT_ID!,
 *   clientSecret: process.env.BASECAMP_CLIENT_SECRET!,
 *   useLegacyFormat: true, // Required for Launchpad
 * });
 *
 * // 3. Use the token
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: token.accessToken,
 * });
 *
 * // 4. Refresh when needed
 * if (isTokenExpired(token)) {
 *   const newToken = await refreshToken({
 *     tokenEndpoint: config.tokenEndpoint,
 *     refreshToken: token.refreshToken!,
 *     useLegacyFormat: true,
 *   });
 * }
 * ```
 */

// Types
export type {
  OAuthConfig,
  OAuthToken,
  ExchangeRequest,
  RefreshRequest,
  RawTokenResponse,
  OAuthErrorResponse,
} from "./types.js";

// Discovery
export {
  discover,
  discoverLaunchpad,
  LAUNCHPAD_BASE_URL,
  type DiscoverOptions,
} from "./discovery.js";

// Token exchange
export {
  exchangeCode,
  refreshToken,
  isTokenExpired,
  type TokenOptions,
} from "./exchange.js";

// PKCE utilities
export {
  generatePKCE,
  generateState,
  type PKCE,
} from "./pkce.js";

// Authorization URL
export {
  buildAuthorizationUrl,
  type AuthorizeParams,
} from "./authorize.js";

// Token store
export {
  FileTokenStore,
  type TokenStore,
} from "./token-store.js";

// Token manager
export {
  TokenManager,
  type TokenManagerOptions,
} from "./token-manager.js";

// Callback server
export {
  startCallbackServer,
  type CallbackResult,
  type CallbackServerOptions,
} from "./callback-server.js";

// Interactive login
export {
  performInteractiveLogin,
  type InteractiveLoginOptions,
} from "./interactive-login.js";

// Identity
export {
  discoverIdentity,
} from "./identity.js";
