package com.basecamp.sdk.oauth

import io.ktor.client.*
import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import com.basecamp.sdk.BasecampException

/**
 * OAuth server configuration from the discovery endpoint.
 */
@Serializable
data class OAuthConfig(
    val issuer: String,
    @SerialName("authorization_endpoint") val authorizationEndpoint: String,
    @SerialName("token_endpoint") val tokenEndpoint: String,
    @SerialName("registration_endpoint") val registrationEndpoint: String? = null,
    @SerialName("scopes_supported") val scopesSupported: List<String> = emptyList(),
)

/** Basecamp's Launchpad OAuth server URL. */
const val LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com"

private val discoveryJson = Json { ignoreUnknownKeys = true }

/**
 * Fetches OAuth server metadata from the well-known discovery endpoint.
 *
 * ```kotlin
 * val config = discover("https://launchpad.37signals.com")
 * val authUrl = config.authorizationEndpoint
 * ```
 *
 * @param baseUrl Base URL of the OAuth server.
 * @param client Optional HTTP client (a default one is created if not provided).
 */
suspend fun discover(baseUrl: String, client: HttpClient? = null): OAuthConfig {
    val url = "${baseUrl.trimEnd('/')}/.well-known/oauth-authorization-server"

    val httpClient = client ?: HttpClient()
    val shouldClose = client == null

    try {
        val response = httpClient.get(url) {
            accept(ContentType.Application.Json)
        }

        if (!response.status.isSuccess()) {
            throw BasecampException.Api(
                "OAuth discovery failed: HTTP ${response.status.value}",
                httpStatus = response.status.value,
            )
        }

        val body = response.bodyAsText()
        return discoveryJson.decodeFromString<OAuthConfig>(body)
    } finally {
        if (shouldClose) httpClient.close()
    }
}

/**
 * Discovers OAuth configuration for Basecamp's Launchpad server.
 */
suspend fun discoverLaunchpad(client: HttpClient? = null): OAuthConfig =
    discover(LAUNCHPAD_BASE_URL, client)
