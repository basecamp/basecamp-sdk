/**
 * OAuth identity discovery for Basecamp SDK.
 *
 * Fetches authorization information (identity and accounts) from
 * the Launchpad endpoint without requiring a full client instance.
 */

import { BasecampError } from "../errors.js";
import type { TokenProvider } from "../client.js";
import type { AuthorizationInfo } from "../services/authorization.js";

/**
 * Raw authorization response from the Launchpad API.
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
    app_href?: string;
    hidden?: boolean;
    expired?: boolean;
    featured?: boolean;
  }>;
}

const AUTHORIZATION_ENDPOINT = "https://launchpad.37signals.com/authorization.json";

/**
 * Fetches authorization information using an access token.
 *
 * Calls the Launchpad authorization endpoint to retrieve the
 * authenticated user's identity and list of accessible accounts.
 * Does not require a full Basecamp client instance.
 *
 * @param accessToken - A token string or async function returning one
 * @returns Authorization information including identity and accounts
 * @throws BasecampError on network or auth errors
 *
 * @example
 * ```ts
 * const info = await discoverIdentity("my_access_token");
 * console.log(info.identity.firstName, info.identity.lastName);
 * for (const account of info.accounts) {
 *   console.log(`${account.name} (${account.product})`);
 * }
 * ```
 */
export async function discoverIdentity(accessToken: TokenProvider): Promise<AuthorizationInfo> {
  const token = typeof accessToken === "function" ? await accessToken() : accessToken;

  let response: Response;
  try {
    response = await fetch(AUTHORIZATION_ENDPOINT, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/json",
      },
    });
  } catch (err) {
    throw new BasecampError("network", `Identity discovery failed: ${err instanceof Error ? err.message : String(err)}`);
  }

  if (!response.ok) {
    if (response.status === 401) {
      throw new BasecampError("auth_required", "Invalid or expired access token", {
        httpStatus: 401,
        hint: "The access token may need to be refreshed",
      });
    }
    throw new BasecampError("api_error", `Authorization endpoint returned ${response.status}`, {
      httpStatus: response.status,
    });
  }

  let raw: RawAuthorizationResponse;
  try {
    raw = (await response.json()) as RawAuthorizationResponse;
  } catch {
    throw new BasecampError("api_error", "Identity discovery returned invalid JSON");
  }

  return {
    expiresAt: new Date(raw.expires_at),
    identity: {
      id: raw.identity.id,
      firstName: raw.identity.first_name,
      lastName: raw.identity.last_name,
      emailAddress: raw.identity.email_address,
    },
    accounts: raw.accounts.map((a) => ({
      id: a.id,
      name: a.name,
      product: a.product,
      href: a.href,
      appHref: a.app_href ?? "",
      hidden: a.hidden,
      expired: a.expired,
      featured: a.featured,
    })),
  };
}
