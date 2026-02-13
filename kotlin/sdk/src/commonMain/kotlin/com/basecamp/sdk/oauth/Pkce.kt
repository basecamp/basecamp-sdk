package com.basecamp.sdk.oauth

/**
 * PKCE (Proof Key for Code Exchange) values for OAuth 2.0.
 */
data class Pkce(
    /** The code verifier — a random string sent during token exchange. */
    val verifier: String,
    /** The code challenge — SHA-256 hash of the verifier, sent during authorization. */
    val challenge: String,
)

/**
 * Generates a PKCE pair (verifier + challenge) for OAuth 2.0.
 *
 * The verifier is 43 characters of base64url-encoded random bytes.
 * The challenge is the SHA-256 hash of the verifier, base64url-encoded.
 *
 * ```kotlin
 * val pkce = generatePkce()
 * // Use pkce.challenge in the authorization URL
 * // Use pkce.verifier when exchanging the code
 * ```
 */
expect fun generatePkce(): Pkce

/**
 * Generates a cryptographically random state token for CSRF protection.
 */
expect fun generateState(): String
