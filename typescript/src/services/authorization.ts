/**
 * Authorization service for Basecamp SDK.
 *
 * Provides functionality to fetch authorization information including
 * the authenticated user's identity and list of accessible accounts.
 */

import { BaseService, type RawClient } from "./base.js";
import type { BasecampHooks } from "../hooks.js";
import type { AuthStrategy } from "../auth-strategy.js";
import { requireSecureEndpoint } from "../security.js";
import {
  parseAuthorizationDocument,
  type AuthorizationInfo,
  type RawAuthorizationDocument,
} from "../oauth/authorization-document.js";

// The document types live in the leaf module alongside the parser that produces
// them, and are re-exported here so this remains their public import path.
export type {
  Identity,
  AuthorizedAccount,
  AuthorizationInfo,
} from "../oauth/authorization-document.js";

/**
 * Options for fetching authorization information.
 */
export interface GetAuthorizationInfoOptions {
  /**
   * Override the default authorization endpoint URL.
   * Defaults to "https://launchpad.37signals.com/authorization.json"
   *
   * This is also how you point at a BC5 issuer, whose document is a different
   * shape — see `../oauth/authorization-document.js`.
   */
  endpoint?: string;
  /**
   * Filter accounts by product type.
   * Common values: "bc3" (Basecamp), "bcx" (Basecamp 2), "hey" (HEY)
   *
   * A BC5 document carries no `product` on any account, so the filter cannot
   * apply there. In that case all accounts are returned and
   * `AuthorizationInfo.productFilterApplied` is `false`, rather than the empty
   * list a literal filter would produce.
   */
  filterProduct?: string;
}

const DEFAULT_AUTHORIZATION_ENDPOINT = "https://launchpad.37signals.com/authorization.json";

/**
 * Service for authorization-related operations.
 *
 * This service communicates with an authorization endpoint — Launchpad by
 * default, or a BC5 issuer via `endpoint:` — rather than the standard Basecamp
 * API. The two serve different-shaped documents; see
 * `../oauth/authorization-document.js`.
 *
 * @example
 * ```ts
 * import { createBasecampClient } from "@37signals/basecamp";
 *
 * const client = createBasecampClient({
 *   accountId: "12345",
 *   accessToken: "your-token",
 * });
 *
 * // Get all accounts
 * const info = await client.authorization.getInfo();
 * console.log(`Logged in as identity ${info.identity.id}`);
 *
 * // Filter to only Basecamp accounts. Against a BC5 issuer, whose accounts
 * // carry no `product`, this returns every account with
 * // `productFilterApplied === false` instead of an empty list.
 * const bc3Info = await client.authorization.getInfo({ filterProduct: "bc3" });
 * for (const account of bc3Info.accounts) {
 *   console.log(account.name);
 * }
 * ```
 */
export class AuthorizationService extends BaseService {
  private authStrategy: AuthStrategy;
  private userAgent: string;

  constructor(
    client: RawClient,
    hooks: BasecampHooks | undefined,
    authStrategy: AuthStrategy,
    userAgent: string
  ) {
    super(client, hooks);
    this.authStrategy = authStrategy;
    this.userAgent = userAgent;
  }

  /**
   * Fetches authorization information for the current access token.
   *
   * Returns the authenticated user's identity and list of accounts
   * they have access to.
   *
   * @param options - Optional configuration
   * @returns Authorization information including identity and accounts
   *
   * @example
   * ```ts
   * const info = await authService.getInfo();
   *
   * // A BC5 issuer emits only `identity.id`; the name and email fields are
   * // Launchpad's and are `undefined` there.
   * console.log(`User: ${info.identity.id}`);
   * console.log(`Token expires: ${info.expiresAt}`);
   *
   * for (const account of info.accounts) {
   *   console.log(`${account.name}: ${account.href}`);
   * }
   * ```
   */
  async getInfo(options: GetAuthorizationInfoOptions = {}): Promise<AuthorizationInfo> {
    const endpoint = options.endpoint ?? DEFAULT_AUTHORIZATION_ENDPOINT;
    // Validate caller-supplied endpoint overrides before attaching the token.
    requireSecureEndpoint(endpoint, "authorization endpoint");

    return this.request(
      {
        service: "Authorization",
        operation: "GetInfo",
        resourceType: "authorization",
        isMutation: false,
      },
      async () => {
        // Build headers with auth strategy
        const headers = new Headers({
          "User-Agent": this.userAgent,
          Accept: "application/json",
        });
        await this.authStrategy.authenticate(headers);

        // Make direct fetch request to Launchpad endpoint
        const response = await fetch(endpoint, {
          method: "GET",
          headers,
        });

        if (!response.ok) {
          return { data: undefined, error: undefined, response };
        }

        const raw = (await response.json()) as RawAuthorizationDocument;

        const data = parseAuthorizationDocument(raw, { filterProduct: options.filterProduct });

        return { data, error: undefined, response };
      }
    );
  }
}
