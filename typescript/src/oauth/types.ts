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
  /**
   * URL of the authorization endpoint.
   *
   * Optional as of BC5 resource-first discovery: device-only authorization
   * servers omit it. Authorization-code consumers MUST assert its presence
   * before use.
   */
  authorizationEndpoint?: string;
  /** URL of the token endpoint */
  tokenEndpoint: string;
  /** URL of the RFC 8628 device authorization endpoint (optional) */
  deviceAuthorizationEndpoint?: string;
  /** URL of the dynamic client registration endpoint (optional) */
  registrationEndpoint?: string;
  /** List of OAuth 2.0 scopes supported (optional) */
  scopesSupported?: string[];
  /** OAuth 2.0 grant types the server supports (optional) */
  grantTypesSupported?: string[];
  /** PKCE code challenge methods supported by the server (optional) */
  codeChallengeMethodsSupported?: string[];
}

/**
 * RFC 9728 protected-resource metadata (hop 1 of resource-first discovery).
 */
export interface ProtectedResourceMetadata {
  /** The resource identifier; must equal the requested resource origin by code-point. */
  resource: string;
  /**
   * Authorization servers advertised for this resource.
   *
   * Absent (`undefined`) and present-but-empty (`[]`) are preserved distinctly:
   * BC5 omits the key while dark, per RFC 9728 §3.2. Both nonetheless select
   * Launchpad, but the distinction is meaningful to callers inspecting metadata.
   */
  authorizationServers?: string[];
}

/**
 * Soft fallback reasons — the ONLY two outcomes under which
 * {@link DiscoverFromResourceResult} yields a fallback (Launchpad) rather than a
 * selected config. Every other failure raises {@link DiscoverySelectionError}.
 */
export type FallbackReason = "resource_discovery_failed" | "no_as_advertised";

/**
 * Hard selection/validation failures. These are THROWN, never returned as a
 * fallback — no consumer may convert them into a Launchpad request.
 */
export type DiscoverySelectionErrorReason =
  | "ambiguous_issuers"
  | "expected_issuer_unavailable"
  | "invalid_issuer_origin"
  | "as_fetch_failed"
  | "issuer_mismatch"
  | "capability_unavailable";

/**
 * Result of {@link discoverFromResource}: either a selected AS config, or a soft
 * fallback to Launchpad. Hard failures are thrown, not represented here.
 *
 * Note: a malformed caller `resourceOrigin` (not an origin-root URL) is a usage
 * error — it throws `BasecampError("usage")` up front and is never surfaced as a
 * fallback reason here.
 */
export type DiscoverFromResourceResult =
  | { kind: "selected"; config: OAuthConfig; issuer: string }
  | { kind: "fallback"; reason: FallbackReason };

/**
 * RFC 8628 device authorization response.
 */
export interface DeviceAuthorization {
  /** The device verification code. */
  deviceCode: string;
  /** The end-user code shown to the user. */
  userCode: string;
  /** The end-user verification URI. */
  verificationUri: string;
  /** The verification URI with the user code embedded (optional). */
  verificationUriComplete?: string;
  /** Lifetime of the device/user codes in seconds. */
  expiresIn: number;
  /** Minimum polling interval in seconds (defaults to 5 when the server omits it). */
  interval: number;
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
