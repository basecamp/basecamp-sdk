/**
 * Authorization service for Basecamp SDK.
 *
 * Provides functionality to fetch authorization information including
 * the authenticated user's identity and list of accessible accounts.
 */

import { BaseService, type RawClient } from "./base.js";
import type { BasecampHooks } from "../hooks.js";
import type { AuthStrategy } from "../auth-strategy.js";

/**
 * The authenticated user's identity.
 */
export interface Identity {
  /** User's unique identifier */
  id: number;
  /** User's first name */
  firstName: string;
  /** User's last name */
  lastName: string;
  /** User's email address */
  emailAddress: string;
}

/**
 * A Basecamp account the user has access to.
 */
export interface AuthorizedAccount {
  /** Account's unique identifier */
  id: number;
  /** Account name */
  name: string;
  /** Product type (e.g., "bc3" for Basecamp, "hey" for HEY) */
  product: string;
  /** API URL for this account */
  href: string;
  /** Web app URL for this account */
  appHref: string;
  /** Whether the account is hidden from the user's view */
  hidden?: boolean;
  /** Whether the account subscription has expired */
  expired?: boolean;
  /** Whether this is the user's featured/primary account */
  featured?: boolean;
}

/**
 * Authorization information response.
 */
export interface AuthorizationInfo {
  /** Token expiration timestamp */
  expiresAt: Date;
  /** The authenticated user's identity */
  identity: Identity;
  /** List of accounts the user can access */
  accounts: AuthorizedAccount[];
}

/**
 * Options for fetching authorization information.
 */
export interface GetAuthorizationInfoOptions {
  /**
   * Override the default authorization endpoint URL.
   * Defaults to "https://launchpad.37signals.com/authorization.json"
   */
  endpoint?: string;
  /**
   * Filter accounts by product type.
   * Common values: "bc3" (Basecamp 3), "bcx" (Basecamp 2), "hey" (HEY)
   */
  filterProduct?: string;
}

/**
 * Raw authorization response from the API.
 */
interface RawAuthorizationResponse {
  expires_at: string;
  identity: {
    id: number;
    first_name: string;
    last_name: string;
    email_address: string;
  };
  accounts: Array<{
    id: number;
    name: string;
    product: string;
    href: string;
    app_href: string;
    hidden?: boolean;
    expired?: boolean;
    featured?: boolean;
  }>;
}

const DEFAULT_AUTHORIZATION_ENDPOINT = "https://launchpad.37signals.com/authorization.json";

/**
 * Service for authorization-related operations.
 *
 * This service communicates with the Launchpad authorization endpoint
 * rather than the standard Basecamp API.
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
 * console.log(`Logged in as ${info.identity.firstName}`);
 *
 * // Filter to only Basecamp 3 accounts
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
   * console.log(`User: ${info.identity.firstName} ${info.identity.lastName}`);
   * console.log(`Email: ${info.identity.emailAddress}`);
   * console.log(`Token expires: ${info.expiresAt}`);
   *
   * for (const account of info.accounts) {
   *   console.log(`${account.name} (${account.product})`);
   * }
   * ```
   */
  async getInfo(options: GetAuthorizationInfoOptions = {}): Promise<AuthorizationInfo> {
    const endpoint = options.endpoint ?? DEFAULT_AUTHORIZATION_ENDPOINT;

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

        const raw = (await response.json()) as RawAuthorizationResponse;

        // Transform to our clean types
        let accounts = raw.accounts.map((a) => ({
          id: a.id,
          name: a.name,
          product: a.product,
          href: a.href,
          appHref: a.app_href,
          hidden: a.hidden,
          expired: a.expired,
          featured: a.featured,
        }));

        // Filter by product if requested
        if (options.filterProduct) {
          accounts = accounts.filter((a) => a.product === options.filterProduct);
        }

        const data: AuthorizationInfo = {
          expiresAt: new Date(raw.expires_at),
          identity: {
            id: raw.identity.id,
            firstName: raw.identity.first_name,
            lastName: raw.identity.last_name,
            emailAddress: raw.identity.email_address,
          },
          accounts,
        };

        return { data, error: undefined, response };
      }
    );
  }
}
