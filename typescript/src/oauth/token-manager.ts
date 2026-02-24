/**
 * OAuth 2.0 token lifecycle management.
 *
 * Handles automatic token refresh, expiry detection, and refresh deduplication.
 */

import { isTokenExpired } from "./exchange.js";
import type { TokenStore } from "./token-store.js";
import type { RefreshRequest, OAuthToken } from "./types.js";

/**
 * Options for creating a TokenManager.
 */
export interface TokenManagerOptions {
  /** Token persistence store */
  store: TokenStore;
  /** Function to perform the token refresh HTTP call */
  refreshToken: (req: RefreshRequest) => Promise<OAuthToken>;
  /** URL of the token endpoint */
  tokenEndpoint: string;
  /** The client identifier (optional) */
  clientId?: string;
  /** The client secret (optional) */
  clientSecret?: string;
  /** Use Launchpad's non-standard token format (optional) */
  useLegacyFormat?: boolean;
  /** Seconds before expiry to trigger refresh (default: 120) */
  bufferSeconds?: number;
}

/**
 * Manages OAuth token lifecycle with automatic refresh and persistence.
 *
 * Loads tokens from a store on first access, refreshes when expired,
 * and deduplicates concurrent refresh requests.
 *
 * @example
 * ```ts
 * const manager = new TokenManager({
 *   store: new FileTokenStore("~/.config/basecamp/tokens.json"),
 *   refreshToken,
 *   tokenEndpoint: config.tokenEndpoint,
 *   clientId: "my_client_id",
 *   clientSecret: "my_client_secret",
 *   useLegacyFormat: true,
 * });
 *
 * // Use as a TokenProvider
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: () => manager.getToken(),
 * });
 * ```
 */
export class TokenManager {
  private readonly store: TokenStore;
  private readonly doRefresh: (req: RefreshRequest) => Promise<OAuthToken>;
  private readonly tokenEndpoint: string;
  private readonly clientId?: string;
  private readonly clientSecret?: string;
  private readonly useLegacyFormat?: boolean;
  private readonly bufferSeconds: number;

  private token: OAuthToken | null = null;
  private loaded = false;
  private refreshPromise: Promise<OAuthToken> | null = null;

  constructor(options: TokenManagerOptions) {
    this.store = options.store;
    this.doRefresh = options.refreshToken;
    this.tokenEndpoint = options.tokenEndpoint;
    this.clientId = options.clientId;
    this.clientSecret = options.clientSecret;
    this.useLegacyFormat = options.useLegacyFormat;
    this.bufferSeconds = options.bufferSeconds ?? 120;
  }

  /**
   * Returns a valid access token string, refreshing if necessary.
   *
   * On first call, loads from store. If expired, refreshes and saves.
   * Concurrent calls during refresh share the same promise.
   *
   * @throws if no token is available and refresh fails
   */
  async getToken(): Promise<string> {
    if (!this.loaded) {
      this.token = await this.store.load();
      this.loaded = true;
    }

    if (this.token && !isTokenExpired(this.token, this.bufferSeconds)) {
      return this.token.accessToken;
    }

    const refreshed = await this.forceRefresh();
    return refreshed.accessToken;
  }

  /**
   * Forces a token refresh regardless of expiry.
   *
   * Concurrent calls share the same in-flight request.
   *
   * @throws if no refresh token is available or refresh fails
   */
  async forceRefresh(): Promise<OAuthToken> {
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    this.refreshPromise = this.executeRefresh();

    try {
      return await this.refreshPromise;
    } finally {
      this.refreshPromise = null;
    }
  }

  /**
   * Checks if the current token is expired.
   *
   * Returns true if no token is loaded or the token has expired.
   */
  isExpired(): boolean {
    if (!this.token) return true;
    return isTokenExpired(this.token, this.bufferSeconds);
  }

  /**
   * The currently loaded token, or null if none.
   */
  get currentToken(): OAuthToken | null {
    return this.token;
  }

  private async executeRefresh(): Promise<OAuthToken> {
    const refreshTokenValue = this.token?.refreshToken;
    if (!refreshTokenValue) {
      throw new Error("No refresh token available");
    }

    const newToken = await this.doRefresh({
      tokenEndpoint: this.tokenEndpoint,
      refreshToken: refreshTokenValue,
      clientId: this.clientId,
      clientSecret: this.clientSecret,
      useLegacyFormat: this.useLegacyFormat,
    });

    // Preserve the previous refresh token when the server omits one
    const merged: OAuthToken = newToken.refreshToken
      ? newToken
      : { ...newToken, refreshToken: refreshTokenValue };

    this.token = merged;
    await this.store.save(merged);
    return merged;
  }
}
