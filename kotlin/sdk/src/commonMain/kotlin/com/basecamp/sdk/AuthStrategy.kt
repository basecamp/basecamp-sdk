package com.basecamp.sdk

import io.ktor.client.request.*

/**
 * Controls how authentication is applied to HTTP requests.
 * The default strategy is [BearerAuth], which uses a [TokenProvider] to set
 * the Authorization header with a Bearer token.
 *
 * Custom strategies can implement alternative auth schemes such as
 * cookie-based auth, API keys, or mutual TLS.
 *
 * Example - Cookie auth:
 * ```kotlin
 * class CookieAuth(private val sessionToken: String) : AuthStrategy {
 *     override suspend fun authenticate(request: HttpRequestBuilder) {
 *         request.header("Cookie", "session=$sessionToken")
 *     }
 * }
 * ```
 */
fun interface AuthStrategy {
    /**
     * Apply authentication to the given request builder.
     * Called before every HTTP request.
     */
    suspend fun authenticate(request: HttpRequestBuilder)
}

/**
 * Bearer token authentication strategy (default).
 * Sets the Authorization header with "Bearer {token}" from a [TokenProvider].
 */
class BearerAuth(private val tokenProvider: TokenProvider) : AuthStrategy {
    override suspend fun authenticate(request: HttpRequestBuilder) {
        val token = tokenProvider.accessToken()
        request.header("Authorization", "Bearer $token")
    }
}
