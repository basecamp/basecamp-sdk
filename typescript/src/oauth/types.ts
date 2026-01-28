/**
 * OAuth 2.0 type definitions for Basecamp SDK.
 *
 * Provides types for OAuth configuration, tokens, and exchange requests.
 * Supports both standard OAuth 2.0 and Basecamp's Launchpad legacy format.
 */

/**
 * OAuth 2.0 server configuration from discovery endpoint.
 */
export interface OAuthConfig {
  /** The authorization server's issuer identifier */
  issuer: string;
  /** URL of the authorization endpoint */
  authorizationEndpoint: string;
  /** URL of the token endpoint */
  tokenEndpoint: string;
  /** URL of the dynamic client registration endpoint (optional) */
  registrationEndpoint?: string;
  /** List of OAuth 2.0 scopes supported (optional) */
  scopesSupported?: string[];
}

/**
 * OAuth 2.0 access token response.
 */
export interface OAuthToken {
  /** The access token string */
  accessToken: string;
  /** The refresh token string (optional) */
  refreshToken?: string;
  /** Token type (usually "Bearer") */
  tokenType: string;
  /** Lifetime of the access token in seconds (optional) */
  expiresIn?: number;
  /** Calculated expiration date (optional) */
  expiresAt?: Date;
  /** OAuth scope granted (optional) */
  scope?: string;
}

/**
 * Parameters for exchanging an authorization code for tokens.
 */
export interface ExchangeRequest {
  /** URL of the token endpoint */
  tokenEndpoint: string;
  /** The authorization code received from the authorization server */
  code: string;
  /** The redirect URI used in the authorization request */
  redirectUri: string;
  /** The client identifier */
  clientId: string;
  /** The client secret (optional for public clients) */
  clientSecret?: string;
  /** PKCE code verifier (optional) */
  codeVerifier?: string;
  /**
   * Use Launchpad's non-standard token format.
   * When true, uses `type=web_server` instead of `grant_type=authorization_code`.
   */
  useLegacyFormat?: boolean;
}

/**
 * Parameters for refreshing an access token.
 */
export interface RefreshRequest {
  /** URL of the token endpoint */
  tokenEndpoint: string;
  /** The refresh token */
  refreshToken: string;
  /** The client identifier (optional) */
  clientId?: string;
  /** The client secret (optional) */
  clientSecret?: string;
  /**
   * Use Launchpad's non-standard token format.
   * When true, uses `type=refresh` instead of `grant_type=refresh_token`.
   */
  useLegacyFormat?: boolean;
}

/**
 * Raw token response from OAuth server.
 * Used internally for JSON parsing.
 */
export interface RawTokenResponse {
  access_token: string;
  refresh_token?: string;
  token_type: string;
  expires_in?: number;
  scope?: string;
}

/**
 * OAuth error response from server.
 */
export interface OAuthErrorResponse {
  error: string;
  error_description?: string;
  error_uri?: string;
}
