package com.basecamp.sdk

/**
 * Provides access tokens for authenticating with the Basecamp API.
 *
 * Implementations can provide static tokens or dynamic token refresh logic.
 *
 * ```kotlin
 * // Static token
 * val provider = StaticTokenProvider("your-token")
 *
 * // Dynamic token with refresh
 * val provider = TokenProvider { refreshAccessToken() }
 * ```
 */
fun interface TokenProvider {
    /** Returns the current access token. */
    suspend fun accessToken(): String
}

/** A [TokenProvider] that always returns the same token. */
class StaticTokenProvider(private val token: String) : TokenProvider {
    init {
        require(token.isNotBlank()) { "Access token must not be blank" }
    }

    override suspend fun accessToken(): String = token
}
