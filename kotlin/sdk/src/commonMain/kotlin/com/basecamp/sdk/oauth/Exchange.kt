package com.basecamp.sdk.oauth

import io.ktor.client.*
import io.ktor.client.plugins.*
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

/** Bounded per-request timeout for every token-endpoint POST — the 30 s credential-POST default shared across the SDKs (SPEC §16). */
private const val TOKEN_REQUEST_TIMEOUT_MS = 30_000L

/**
 * The redirects the token endpoint refuses outright (SPEC §16 "Token-Endpoint
 * Transport Policy") — the same set the signed download hop refuses (#809).
 * 304 is deliberately absent: it is a cache validator with no `Location`, and
 * falls through to the generic non-success branch below.
 */
private val REDIRECT_STATUSES = setOf(301, 302, 303, 307, 308)

/**
 * Builds a hardened HTTP client for token-endpoint POSTs: redirects suppressed
 * ([HttpClient.followRedirects] = false, so a 3xx is classified below rather
 * than any engine chasing an attacker-influenced `Location` with the
 * credentials re-POSTed) and a bounded per-request timeout ([HttpTimeout]) so
 * a stalled token request cannot hang an exchange or refresh unbounded.
 *
 * When [baseClient] is supplied its engine is reused but wrapped so the
 * hardening applies regardless — redirect suppression is a security
 * invariant, not a default (the device flow does the same). The returned
 * wrapper is always closed by the caller and, because Ktor only closes
 * engines it created, the borrowed engine survives.
 */
private fun hardenedTokenClient(baseClient: HttpClient?): HttpClient {
    val engine = baseClient?.engine
    return if (engine != null) {
        HttpClient(engine) {
            followRedirects = false
            expectSuccess = false
            install(HttpTimeout) { requestTimeoutMillis = TOKEN_REQUEST_TIMEOUT_MS }
        }
    } else {
        HttpClient {
            followRedirects = false
            expectSuccess = false
            install(HttpTimeout) { requestTimeoutMillis = TOKEN_REQUEST_TIMEOUT_MS }
        }
    }
}

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

    val httpClient = hardenedTokenClient(client)

    try {
        val response = httpClient.submitForm(endpoint, params) {
            accept(ContentType.Application.Json)
        }

        val status = response.status.value

        // Status-first: a refused redirect is classified BEFORE any body read,
        // so a 3xx that drip-feeds its body cannot degrade into a timeout. The
        // hardened client never follows, and the `Location` is never dialled —
        // the endpoint the caller (or discovered metadata) named is the one
        // destination these credentials go to (SPEC §16).
        if (status in REDIRECT_STATUSES) {
            throw BasecampException.Api(
                "redirect $status on the token endpoint is not followed",
                httpStatus = status,
            )
        }

        val body = response.bodyAsText()

        if (body.length > MAX_RESPONSE_SIZE) {
            throw BasecampException.Api(
                "OAuth token response exceeds size limit",
                httpStatus = status,
            )
        }

        if (!response.status.isSuccess()) {
            val errorResp = runCatching { tokenJson.decodeFromString<OAuthErrorResponse>(body) }.getOrNull()
            val message = errorResp?.errorDescription
                ?: errorResp?.error
                ?: "Token request failed: HTTP $status"
            throw BasecampException.Auth(
                message = BasecampException.truncateMessage(message),
            )
        }

        // A token response that fails to decode may still contain credential
        // material, and kotlinx embeds input excerpts in its exception
        // messages — map to a status-only fault (no cause: cause messages
        // surface in stack traces) instead of propagating it.
        val raw = runCatching { tokenJson.decodeFromString<RawTokenResponse>(body) }.getOrElse {
            throw BasecampException.Api("Failed to parse token response", httpStatus = status)
        }
        // resource: absent and JSON null decode to null (unset); when present
        // it must be non-empty (SPEC §16) — an empty binding is not a binding.
        // A non-string resource fails deserialization above.
        if (raw.resource != null && raw.resource.isEmpty()) {
            throw BasecampException.Api(
                "Token response resource must be a non-empty string when present",
                httpStatus = status,
            )
        }
        // A 2xx with an EMPTY access_token is malformed, not a success —
        // matching the device flow and the other SDKs' non-empty contract.
        if (raw.accessToken.isEmpty()) {
            throw BasecampException.Api(
                "Token response missing access_token",
                httpStatus = status,
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
                    httpStatus = status,
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
    } catch (e: HttpRequestTimeoutException) {
        // The wrapper's HttpTimeout fired. Mapped explicitly — it subclasses
        // CancellationException, so left alone it would masquerade as a
        // cooperative cancellation — to the retryable network fault the other
        // SDKs raise here ("Token request timed out", TS/Python/Ruby).
        throw BasecampException.Network("Token request timed out", cause = e)
    } finally {
        // Always ours: hardenedTokenClient built it, an injected client only
        // lent its engine (which close() leaves running).
        httpClient.close()
    }
}
