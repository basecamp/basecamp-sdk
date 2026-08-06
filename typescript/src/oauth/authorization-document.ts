/**
 * The authorization document, as served by both issuers.
 *
 * Two different services fetch this document — {@link ../services/authorization.js | AuthorizationService}
 * and {@link ./identity.js | discoverIdentity} — and they used to carry a copy of
 * the mapping each. The copies had already drifted apart (`app_href` was required
 * in one and optional in the other), which is exactly how a shape fix gets applied
 * to one caller and missed on the other. The mapping lives here once.
 *
 * This module is a leaf on purpose: it imports nothing from the SDK, so either
 * caller can take a *value* dependency on it without adding a runtime edge to the
 * `base.ts` / `client.ts` cycle.
 *
 * ## Two issuers, two shapes
 *
 * Launchpad and a BC5 (bc3) issuer both serve `GET /authorization.json`, and the
 * documents are not the same shape. bc3 renders its own from
 * `app/views/api/authorizations/show.json.jbuilder`:
 *
 * | Field | Launchpad | BC5 |
 * |---|---|---|
 * | `identity.first_name` / `last_name` / `email_address` | present | absent |
 * | `accounts[].product` / `app_href` | present | absent |
 * | `accounts[].resource` | absent | present (RFC 8707 indicator) |
 * | `scope` | absent | present, BC3-issued tokens only |
 * | `expires_at` | ISO-8601 string | integer epoch **seconds** |
 *
 * Every field either issuer omits is typed optional here. The union is modelled
 * rather than either shape alone, because a consumer reaches a BC5 issuer just by
 * passing `endpoint:`, and BC5 issuers serving their own document is the intended
 * end state.
 *
 * @see spec/api-gaps/bc5-authorization-document-shape.md
 */

/**
 * The authenticated user's identity.
 *
 * Only `id` is guaranteed. A BC5 issuer emits nothing but the identity id — it
 * deliberately drops the PII that the API docs already say not to use for
 * identifying users — so the name and email fields are absent there.
 */
export interface Identity {
  /** User's unique identifier. Emitted by both issuers. */
  id: number;
  /** User's first name. Launchpad only — `undefined` from a BC5 issuer. */
  firstName?: string;
  /** User's last name. Launchpad only — `undefined` from a BC5 issuer. */
  lastName?: string;
  /** User's email address. Launchpad only — `undefined` from a BC5 issuer. */
  emailAddress?: string;
}

/**
 * A Basecamp account the user has access to.
 */
export interface AuthorizedAccount {
  /** Account's unique identifier. */
  id: number;
  /** Account name. */
  name: string;
  /**
   * Product type (e.g. `"bc3"` for Basecamp, `"hey"` for HEY).
   *
   * Launchpad only. A BC5 issuer serves one product by construction and omits
   * this, which is why {@link filterAccountsByProduct} treats a document with no
   * `product` anywhere as one the filter cannot apply to.
   */
  product?: string;
  /** API URL for this account. Emitted by both issuers. */
  href: string;
  /** Web app URL for this account. Launchpad only. */
  appHref?: string;
  /**
   * RFC 8707 resource indicator for this account (`urn:bc:account:<id>`).
   *
   * BC5 issuers only. Pass it as the `resource` parameter when requesting a
   * token scoped to this account.
   */
  resource?: string;
  /** Whether the account is hidden from the user's view. */
  hidden?: boolean;
  /** Whether the account subscription has expired. */
  expired?: boolean;
  /** Whether this is the user's featured/primary account. */
  featured?: boolean;
}

/**
 * Authorization information response.
 */
export interface AuthorizationInfo {
  /**
   * Token expiration timestamp.
   *
   * Parsed from either issuer's spelling — see {@link parseExpiresAt}.
   */
  expiresAt: Date;
  /** The authenticated user's identity. */
  identity: Identity;
  /** List of accounts the user can access. */
  accounts: AuthorizedAccount[];
  /**
   * The token's granted scope.
   *
   * BC5 issuers only, and only for BC3-issued tokens — legacy Signal tokens
   * predate scopes, so its absence is not an error.
   */
  scope?: string;
  /**
   * Whether a requested `product` filter was actually applied.
   *
   * Only meaningful when a product filter was requested. `false` means the
   * document carried no `product` on any account — a BC5 document — so the
   * filter was inapplicable and `accounts` is unfiltered rather than empty.
   * See {@link filterAccountsByProduct}.
   */
  productFilterApplied?: boolean;
}

/**
 * The raw wire document, covering both issuers.
 *
 * Every field that *either* issuer omits is optional, so this type describes what
 * can actually arrive rather than what one issuer happens to send. `identity` and
 * `accounts` are the two that stay required: both issuers emit both, so a document
 * missing either is malformed, and throwing on it beats inventing an empty account
 * list that a caller cannot distinguish from a real one.
 */
export interface RawAuthorizationDocument {
  /** ISO-8601 string (Launchpad) or integer epoch seconds (BC5). */
  expires_at?: string | number | null;
  /** Both issuers always emit this, so it stays required — see the note below. */
  identity: {
    id: number;
    first_name?: string;
    last_name?: string;
    email_address?: string;
  };
  /** Both issuers always emit this, so it stays required — see the note below. */
  accounts: Array<{
    id: number;
    name: string;
    product?: string;
    href: string;
    app_href?: string;
    resource?: string;
    hidden?: boolean;
    expired?: boolean;
    featured?: boolean;
  }>;
  scope?: string;
}

/**
 * Parses `expires_at` from either issuer's spelling.
 *
 * Launchpad sends an ISO-8601 string; bc3 sends `@token.expires_at.to_i`, an
 * integer epoch **seconds**. The distinction is the whole point of branching on
 * `typeof`: `new Date(2085213356)` treats the number as *milliseconds* and yields
 * a date in 1970 — a wrong answer rather than an exception, which then reads as
 * an expired credential rather than a schema mismatch.
 *
 * bc3 renders `.to_i`, so a nil expiry arrives as `0`, never `null`. A `0`, a
 * `null` and an absent field are all "no expiry known" and produce an Invalid
 * Date, which is what a `Date`-typed field can say about the absence.
 */
export function parseExpiresAt(value: string | number | null | undefined): Date {
  if (typeof value === "number") {
    // Epoch seconds, not milliseconds. `0` is bc3's rendering of a nil expiry.
    return value === 0 ? new Date(NaN) : new Date(value * 1000);
  }
  if (typeof value === "string" && value !== "") {
    return new Date(value);
  }
  return new Date(NaN);
}

/**
 * Filters accounts by product, or reports that it could not.
 *
 * A BC5 document carries no `product` on any account, so filtering it by product
 * matches nothing. Returning `[]` there is silently wrong — the accounts exist,
 * and the caller's next step is to pick a `href` out of a list the SDK just
 * emptied. Instead: when *no* account in the document carries a `product`, the
 * filter is inapplicable, and all accounts are returned with `applied: false` so
 * a caller that cares can tell the two situations apart.
 *
 * When at least one account carries a `product`, the filter is meaningful and is
 * applied normally — an empty result then genuinely means "no account matched".
 */
export function filterAccountsByProduct(
  accounts: AuthorizedAccount[],
  product: string
): { accounts: AuthorizedAccount[]; applied: boolean } {
  const filterable = accounts.some((a) => a.product !== undefined);
  if (!filterable) {
    return { accounts, applied: false };
  }
  return { accounts: accounts.filter((a) => a.product === product), applied: true };
}

/**
 * Maps a raw authorization document to {@link AuthorizationInfo}.
 *
 * Optionally applies a product filter; see {@link filterAccountsByProduct} for
 * what happens when the document cannot be filtered by product.
 */
export function parseAuthorizationDocument(
  raw: RawAuthorizationDocument,
  options: { filterProduct?: string } = {}
): AuthorizationInfo {
  let accounts: AuthorizedAccount[] = raw.accounts.map((a) => ({
    id: a.id,
    name: a.name,
    product: a.product,
    href: a.href,
    appHref: a.app_href,
    resource: a.resource,
    hidden: a.hidden,
    expired: a.expired,
    featured: a.featured,
  }));

  let productFilterApplied: boolean | undefined;
  if (options.filterProduct !== undefined && options.filterProduct !== "") {
    const filtered = filterAccountsByProduct(accounts, options.filterProduct);
    accounts = filtered.accounts;
    productFilterApplied = filtered.applied;
  }

  const info: AuthorizationInfo = {
    expiresAt: parseExpiresAt(raw.expires_at),
    identity: {
      id: raw.identity.id,
      firstName: raw.identity.first_name,
      lastName: raw.identity.last_name,
      emailAddress: raw.identity.email_address,
    },
    accounts,
  };
  if (raw.scope !== undefined) {
    info.scope = raw.scope;
  }
  if (productFilterApplied !== undefined) {
    info.productFilterApplied = productFilterApplied;
  }
  return info;
}
