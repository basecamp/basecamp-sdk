/**
 * Shared OAuth limits (SPEC §16), split out so the token-exchange path can
 * import them without pulling the whole device-flow module.
 */

/**
 * Upper bound (seconds) for an OAuth token's `expires_in`: 2_147_483_647 s
 * (~68 years) — cross-runtime safe and vastly beyond any realistic token
 * lifetime. A very large finite value (or a non-finite one from `1e400`)
 * makes `new Date(Date.now() + expires_in * 1000)` an Invalid Date whose
 * `getTime()` is NaN, so downstream expiry checks would treat the token as
 * never expiring. A value past this ceiling is a malformed response. Shared
 * across all five SDKs.
 */
export const MAX_TOKEN_LIFETIME_SECONDS = 2_147_483_647;
