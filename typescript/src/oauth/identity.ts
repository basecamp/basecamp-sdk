/**
 * OAuth identity discovery for Basecamp SDK.
 *
 * Fetches authorization information (identity and accounts) from
 * the Launchpad endpoint without requiring a full client instance.
 *
 * The endpoint is fixed to Launchpad here — unlike
 * {@link ../services/authorization.js | AuthorizationService.getInfo}, which
 * accepts an `endpoint:` override and so can be pointed at a BC5 issuer. Both
 * share one mapping regardless; see `./authorization-document.js` for why the two
 * documents differ and how the shared parser reconciles them.
 */

import { BasecampError } from "../errors.js";
import type { TokenProvider } from "../client.js";
import {
  parseAuthorizationDocument,
  type AuthorizationInfo,
  type RawAuthorizationDocument,
} from "./authorization-document.js";

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

  let raw: RawAuthorizationDocument;
  try {
    raw = (await response.json()) as RawAuthorizationDocument;
  } catch {
    throw new BasecampError("api_error", "Identity discovery returned invalid JSON");
  }

  return parseAuthorizationDocument(raw);
}
