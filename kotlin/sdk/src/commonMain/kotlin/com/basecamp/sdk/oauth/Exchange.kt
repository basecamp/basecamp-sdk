package com.basecamp.sdk.oauth

import io.ktor.client.*
import io.ktor.client.request.*
import io.ktor.client.request.forms.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.http.currentTimeMillis

/**
 * OAuth access token response.
 */
data class OAuthToken(
    val accessToken: String,
    val refreshToken: String?,
    val tokenType: String,
    val expiresIn: Long?,
    /** Computed wall-clock expiration time (epoch milliseconds). */
    val expiresAt: Long?,
    val scope: String?,
)

@Serializable
internal data class RawTokenResponse(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String? = null,
    @SerialName("token_type") val tokenType: String = "Bearer",
    @SerialName("expires_in") val expiresIn: Long? = null,
    val scope: String? = null,
)

@Serializable
internal data class OAuthErrorResponse(
    val error: String,
    @SerialName("error_description") val errorDescription: String? = null,
)

private val tokenJson = Json { ignoreUnknownKeys = true }
private const val MAX_RESPONSE_SIZE = 1_048_576L // 1 MB

/**
 * Exchanges an authorization code for tokens.
 *
 * Supports both standard OAuth 2.0 (`grant_type=authorization_code`) and
 * Basecamp's Launchpad legacy format (`type=web_server`).
 *
 * ```kotlin
 * val token = exchangeCode(
 *     tokenEndpoint = config.tokenEndpoint,
 *     code = authorizationCode,
 *     redirectUri = "https://myapp.com/callback",
 *     clientId = clientId,
 *     clientSecret = clientSecret,
 *     codeVerifier = pkce.verifier,
 * )
 * ```
 */
suspend fun exchangeCode(
    tokenEndpoint: String,
    code: String,
    redirectUri: String,
    clientId: String,
    clientSecret: String,
    codeVerifier: String? = null,
    useLegacyFormat: Boolean = false,
    client: HttpClient? = null,
): OAuthToken {
    val params = if (useLegacyFormat) {
        parametersOf(
            "type" to listOf("web_server"),
            "code" to listOf(code),
            "redirect_uri" to listOf(redirectUri),
            "client_id" to listOf(clientId),
            "client_secret" to listOf(clientSecret),
        )
    } else {
        val map = mutableMapOf(
            "grant_type" to listOf("authorization_code"),
            "code" to listOf(code),
            "redirect_uri" to listOf(redirectUri),
            "client_id" to listOf(clientId),
            "client_secret" to listOf(clientSecret),
        )
        if (codeVerifier != null) {
            map["code_verifier"] = listOf(codeVerifier)
        }
        parametersOf(map)
    }

    return postTokenRequest(tokenEndpoint, params, client)
}

/**
 * Refreshes an access token using a refresh token.
 *
 * ```kotlin
 * val newToken = refreshToken(
 *     tokenEndpoint = config.tokenEndpoint,
 *     refreshToken = currentToken.refreshToken!!,
 *     clientId = clientId,
 *     clientSecret = clientSecret,
 * )
 * ```
 */
suspend fun refreshToken(
    tokenEndpoint: String,
    refreshToken: String,
    clientId: String,
    clientSecret: String,
    useLegacyFormat: Boolean = false,
    client: HttpClient? = null,
): OAuthToken {
    val params = if (useLegacyFormat) {
        parametersOf(
            "type" to listOf("refresh"),
            "refresh_token" to listOf(refreshToken),
            "client_id" to listOf(clientId),
            "client_secret" to listOf(clientSecret),
        )
    } else {
        parametersOf(
            "grant_type" to listOf("refresh_token"),
            "refresh_token" to listOf(refreshToken),
            "client_id" to listOf(clientId),
            "client_secret" to listOf(clientSecret),
        )
    }

    return postTokenRequest(tokenEndpoint, params, client)
}

/**
 * Checks whether a token is expired (or within the buffer window).
 *
 * @param bufferSeconds Seconds before actual expiration to consider expired (default: 60).
 */
fun isTokenExpired(token: OAuthToken, bufferSeconds: Long = 60): Boolean {
    val expiresAt = token.expiresAt ?: return false // No expiration info → assume valid
    return currentTimeMillis() >= (expiresAt - bufferSeconds * 1000)
}

private suspend fun postTokenRequest(
    endpoint: String,
    params: Parameters,
    client: HttpClient?,
): OAuthToken {
    val httpClient = client ?: HttpClient()
    val shouldClose = client == null

    try {
        val response = httpClient.submitForm(endpoint, params) {
            accept(ContentType.Application.Json)
        }

        val body = response.bodyAsText()

        if (body.length > MAX_RESPONSE_SIZE) {
            throw BasecampException.Api(
                "OAuth token response exceeds size limit",
                httpStatus = response.status.value,
            )
        }

        if (!response.status.isSuccess()) {
            val errorResp = runCatching { tokenJson.decodeFromString<OAuthErrorResponse>(body) }.getOrNull()
            val message = errorResp?.errorDescription
                ?: errorResp?.error
                ?: "Token request failed: HTTP ${response.status.value}"
            throw BasecampException.Auth(
                message = BasecampException.truncateMessage(message),
            )
        }

        val raw = tokenJson.decodeFromString<RawTokenResponse>(body)
        val now = currentTimeMillis()
        val expiresAt = raw.expiresIn?.let { now + it * 1000 }

        return OAuthToken(
            accessToken = raw.accessToken,
            refreshToken = raw.refreshToken,
            tokenType = raw.tokenType,
            expiresIn = raw.expiresIn,
            expiresAt = expiresAt,
            scope = raw.scope,
        )
    } finally {
        if (shouldClose) httpClient.close()
    }
}
