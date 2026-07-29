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
import com.basecamp.sdk.requireSecureEndpoint
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
    /**
     * RFC 8707 resource indicator the token is bound to (BC5:
     * `urn:bc:account:<id>`). Echo it as the `resource` parameter when
     * refreshing — BC5 multi-account refresh tokens reject a refresh without
     * it (SPEC §16). Appended last: earlier parameters keep their positions.
     */
    val resource: String? = null,
)

@Serializable
internal data class RawTokenResponse(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String? = null,
    @SerialName("token_type") val tokenType: String? = null,
    @SerialName("expires_in") val expiresIn: Long? = null,
    val scope: String? = null,
    val resource: String? = null,
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
    // Nullable: the public `basecamp-cli` client is a public OAuth client
    // (`token_endpoint_auth_method: none`) and sends no secret.
    clientSecret: String? = null,
    useLegacyFormat: Boolean = false,
    client: HttpClient? = null,
    // RFC 8707 resource indicator, sent only when set. Echo the stored
    // token's resource: BC5 multi-account refresh tokens hard-require it
    // (SPEC §16). Appended last: earlier parameters keep their positions.
    resource: String? = null,
): OAuthToken {
    val params = Parameters.build {
        if (useLegacyFormat) {
            append("type", "refresh")
        } else {
            append("grant_type", "refresh_token")
        }
        append("refresh_token", refreshToken)
        append("client_id", clientId)
        if (!clientSecret.isNullOrEmpty()) append("client_secret", clientSecret)
        if (!resource.isNullOrEmpty()) append("resource", resource)
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
    // Never POST credentials over cleartext (localhost exempt for dev/test).
    requireSecureEndpoint(endpoint, "token endpoint")

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

        // A token response that fails to decode may still contain credential
        // material, and kotlinx embeds input excerpts in its exception
        // messages — map to a status-only fault (no cause: cause messages
        // surface in stack traces) instead of propagating it.
        val raw = runCatching { tokenJson.decodeFromString<RawTokenResponse>(body) }.getOrElse {
            throw BasecampException.Api("Failed to parse token response", httpStatus = response.status.value)
        }
        // resource: absent and JSON null decode to null (unset); when present
        // it must be non-empty (SPEC §16) — an empty binding is not a binding.
        // A non-string resource fails deserialization above.
        if (raw.resource != null && raw.resource.isEmpty()) {
            throw BasecampException.Api(
                "Token response resource must be a non-empty string when present",
                httpStatus = response.status.value,
            )
        }
        // A 2xx with an EMPTY access_token is malformed, not a success —
        // matching the device flow and the other SDKs' non-empty contract.
        if (raw.accessToken.isEmpty()) {
            throw BasecampException.Api(
                "Token response missing access_token",
                httpStatus = response.status.value,
            )
        }

        val now = currentTimeMillis()
        val expiresAt = raw.expiresIn?.let { now + it * 1000 }

        // token_type: absent/JSON-null defaults to Bearer; a present-but-empty
        // value is malformed (SPEC §16) — matching the device-flow parser.
        val tokenType = raw.tokenType?.also {
            if (it.isEmpty()) {
                throw BasecampException.Api(
                    "Token response token_type must be a non-empty string when present",
                    httpStatus = response.status.value,
                )
            }
        } ?: "Bearer"

        return OAuthToken(
            accessToken = raw.accessToken,
            refreshToken = raw.refreshToken,
            tokenType = tokenType,
            expiresIn = raw.expiresIn,
            expiresAt = expiresAt,
            scope = raw.scope,
            resource = raw.resource,
        )
    } finally {
        if (shouldClose) httpClient.close()
    }
}
