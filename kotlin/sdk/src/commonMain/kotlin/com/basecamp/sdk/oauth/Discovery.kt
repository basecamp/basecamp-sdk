package com.basecamp.sdk.oauth

import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.isSameOrigin
import com.basecamp.sdk.requireOriginRoot
import io.ktor.client.HttpClient
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.request.accept
import io.ktor.client.request.prepareGet
import io.ktor.client.statement.HttpResponse
import io.ktor.client.statement.bodyAsChannel
import io.ktor.http.ContentType
import io.ktor.utils.io.readAvailable
import kotlinx.coroutines.CancellationException
import kotlinx.serialization.SerialName
import kotlinx.serialization.SerializationException
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject

/**
 * OAuth 2.0 discovery for the Basecamp SDK.
 *
 * Two composable operations plus an orchestrator (SPEC.md §16, "Resource-First
 * Discovery"):
 *   - [discover] — RFC 8414 AS metadata + issuer binding.
 *   - [discoverProtectedResource] — RFC 9728 resource metadata.
 *   - [discoverFromResource] — resource-first selection + stage-sensitive fallback.
 *
 * All fetches are SSRF-hardened: HTTPS-only origins (localhost exempt), origin
 * parsed/validated with the transport URL parser before any socket opens,
 * redirects suppressed ([HttpClient.followRedirects] = false), timeouts bounded,
 * and bodies read under a bounded/streaming cap that aborts before an oversized
 * body is fully buffered.
 */

/**
 * OAuth 2.0 server configuration from an RFC 8414 discovery endpoint.
 *
 * As of BC5 resource-first discovery, [authorizationEndpoint] is OPTIONAL:
 * device-only authorization servers omit it, so authorization-code consumers
 * MUST assert its presence before use. [tokenEndpoint] remains required.
 */
@Serializable
data class OAuthConfig(
    val issuer: String,
    @SerialName("authorization_endpoint") val authorizationEndpoint: String? = null,
    @SerialName("token_endpoint") val tokenEndpoint: String,
    @SerialName("device_authorization_endpoint") val deviceAuthorizationEndpoint: String? = null,
    @SerialName("registration_endpoint") val registrationEndpoint: String? = null,
    @SerialName("scopes_supported") val scopesSupported: List<String> = emptyList(),
    @SerialName("grant_types_supported") val grantTypesSupported: List<String> = emptyList(),
)

/**
 * RFC 9728 protected-resource metadata (hop 1 of resource-first discovery).
 *
 * [authorizationServers] is a nullable list so "key absent" (`null`, BC5's dark
 * posture) and "present but empty" (`[]`) stay distinguishable at the type
 * level, even though both select Launchpad.
 */
data class ProtectedResourceMetadata(
    val resource: String,
    val authorizationServers: List<String>? = null,
)

/**
 * Soft fallback reasons — the ONLY two outcomes under which
 * [discoverFromResource] yields a [DiscoveryResult.FallBack] (Launchpad) rather
 * than a selected config. Every other failure raises
 * [BasecampException.DiscoverySelection].
 */
enum class FallbackReason(val code: String) {
    RESOURCE_DISCOVERY_FAILED("resource_discovery_failed"),
    NO_AS_ADVERTISED("no_as_advertised"),
}

/**
 * Result of [discoverFromResource]: either a selected AS config, or a soft
 * fallback to Launchpad. Hard failures are thrown
 * ([BasecampException.DiscoverySelection]), never represented here.
 */
sealed interface DiscoveryResult {
    /** A BC5 authorization server was selected and its metadata bound. */
    data class Selected(val config: OAuthConfig, val issuer: String) : DiscoveryResult

    /** No BC5 AS was committed; the caller should fall back to Launchpad. */
    data class FallBack(val reason: FallbackReason) : DiscoveryResult
}

/** Basecamp's Launchpad OAuth server URL (the fallback authorization server). */
const val LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com"

/** Cap on a discovery response body (1 MiB) — discovery docs are tiny. */
private const val MAX_DISCOVERY_BODY_BYTES = 1L * 1024 * 1024

/** Bounded timeout for a single discovery fetch. */
private const val DISCOVERY_TIMEOUT_MS = 10_000L

private val discoveryJson = Json { ignoreUnknownKeys = true }

/** Raw AS metadata (RFC 8414). All fields nullable to validate manually. */
@Serializable
private data class RawDiscoveryResponse(
    val issuer: String? = null,
    @SerialName("authorization_endpoint") val authorizationEndpoint: String? = null,
    @SerialName("token_endpoint") val tokenEndpoint: String? = null,
    @SerialName("device_authorization_endpoint") val deviceAuthorizationEndpoint: String? = null,
    @SerialName("registration_endpoint") val registrationEndpoint: String? = null,
    @SerialName("scopes_supported") val scopesSupported: List<String> = emptyList(),
    @SerialName("grant_types_supported") val grantTypesSupported: List<String> = emptyList(),
)

/** Raw resource metadata (RFC 9728). */
@Serializable
private data class RawResourceResponse(
    val resource: String? = null,
    // Nullable so absent (null) and present-empty ([]) stay distinct.
    @SerialName("authorization_servers") val authorizationServers: List<String>? = null,
)

/**
 * Discovers OAuth 2.0 Authorization Server Metadata (RFC 8414) from
 * `{baseUrl}/.well-known/oauth-authorization-server`, and binds it: the returned
 * `issuer` MUST equal the requested issuer origin by code-point (no
 * normalization beyond origin-root parsing). `token_endpoint` is required;
 * `authorization_endpoint` is optional (device-only servers omit it); any
 * present `*_endpoint` must be non-empty.
 *
 * ```kotlin
 * val config = discover("https://launchpad.37signals.com")
 * val tokenUrl = config.tokenEndpoint
 * ```
 *
 * @param baseUrl The OAuth server's issuer origin.
 * @param client Optional HTTP client (a hardened one is created if not provided).
 * @throws BasecampException.Usage on a malformed origin.
 * @throws BasecampException.Api on invalid/mismatched metadata.
 * @throws BasecampException.Network on transport failure or timeout.
 */
suspend fun discover(baseUrl: String, client: HttpClient? = null): OAuthConfig =
    try {
        fetchAndBindAsMetadata(baseUrl, client)
    } catch (marker: IssuerBindingException) {
        // The binding failure is signalled internally by a module-private marker
        // so discoverFromResource can classify it by type. To external callers it
        // MUST surface as an ordinary api_error — exactly like any other invalid
        // AS metadata — never as this private type.
        throw BasecampException.Api(
            marker.message ?: "OAuth issuer mismatch",
            httpStatus = 200,
            cause = marker,
        )
    }

/**
 * Fetches and binds RFC 8414 AS metadata, letting the module-private
 * [IssuerBindingException] escape on an issuer code-point mismatch. [discover]
 * wraps this to convert the marker into a public [BasecampException.Api], while
 * [discoverFromResource] calls it directly so it can branch on the marker type
 * to classify `issuer_mismatch` vs `as_fetch_failed`.
 */
private suspend fun fetchAndBindAsMetadata(
    baseUrl: String,
    client: HttpClient?,
    bindIssuer: String? = null,
): OAuthConfig {
    val issuerOrigin = requireOriginRoot(baseUrl, "OAuth discovery base URL")
    val url = "$issuerOrigin/.well-known/oauth-authorization-server"

    val body = fetchDiscoveryDocument(url, client)
    // Inspect the raw JSON before decoding so an endpoint key present with an
    // explicit JSON `null` is rejected as invalid metadata rather than collapsed to
    // the same absent-key state a nullable Kotlin field cannot distinguish.
    val obj = try {
        discoveryJson.parseToJsonElement(body) as? JsonObject
    } catch (e: SerializationException) {
        throw BasecampException.Api("Failed to parse OAuth discovery response", httpStatus = 200, cause = e)
    } ?: throw BasecampException.Api("Failed to parse OAuth discovery response", httpStatus = 200)
    rejectExplicitNullEndpoints(obj)
    val raw = try {
        discoveryJson.decodeFromString<RawDiscoveryResponse>(body)
    } catch (e: SerializationException) {
        throw BasecampException.Api("Failed to parse OAuth discovery response", httpStatus = 200, cause = e)
    }
    // Fetch from the normalized origin, but bind the metadata issuer against the
    // exact advertised string when supplied (routing vs binding are distinct);
    // the public discover passes none, so it binds to its own normalized origin.
    return bindAsMetadata(raw, bindIssuer ?: issuerOrigin)
}

/**
 * Rejects any `*_endpoint` key present with an explicit JSON `null`. A nullable
 * Kotlin field collapses "key omitted" (valid: endpoint absent) and "key present
 * but `null`" (invalid metadata) to the same `null`, so the distinction is
 * enforced at the JSON layer. Present-but-empty strings are rejected downstream in
 * [bindAsMetadata]; non-string values fail the structural decode.
 */
private fun rejectExplicitNullEndpoints(obj: JsonObject) {
    for (key in listOf(
        "authorization_endpoint",
        "token_endpoint",
        "device_authorization_endpoint",
        "registration_endpoint",
    )) {
        if (obj[key] is JsonNull) {
            throw BasecampException.Api(
                "Invalid OAuth discovery response: $key must not be null",
                httpStatus = 200,
            )
        }
    }
}

/**
 * Module-private structural marker for an RFC 8414 issuer-binding failure: the AS
 * metadata's `issuer` did not equal the requested issuer origin by code-point.
 *
 * [discoverFromResource] branches on this by type to classify `issuer_mismatch`
 * vs `as_fetch_failed` — a structured tag, never a match on the message text.
 * Kept private to this module and deliberately NOT a [BasecampException] subtype
 * (that sealed type lives in another package, and device flow shares its file):
 * [discover] converts it to a plain [BasecampException.Api] before it can escape,
 * so to any external caller a binding failure is an ordinary api_error.
 */
private class IssuerBindingException(message: String) : Exception(message)

/**
 * Validates AS metadata and binds `issuer` to [expectedIssuerOrigin] by
 * code-point. Universal validation only: `issuer` + `token_endpoint` present and
 * non-empty; any present endpoint field non-empty. Per-grant endpoint checks are
 * the consumer's responsibility.
 */
private fun bindAsMetadata(raw: RawDiscoveryResponse, expectedIssuerOrigin: String): OAuthConfig {
    val issuer = raw.issuer
    if (issuer.isNullOrEmpty()) {
        throw BasecampException.Api(
            "Invalid OAuth discovery response: missing required fields (issuer)",
            httpStatus = 200,
        )
    }
    // RFC 8414 §3.3/§4: issuer identical by code-point. No normalization. Raised
    // as the structural IssuerBindingException so discoverFromResource classifies
    // it by type without matching the message text.
    if (issuer != expectedIssuerOrigin) {
        throw IssuerBindingException(
            "OAuth issuer mismatch: metadata issuer \"$issuer\" does not equal \"$expectedIssuerOrigin\"",
        )
    }
    val token = raw.tokenEndpoint
    if (token.isNullOrEmpty()) {
        throw BasecampException.Api(
            "Invalid OAuth discovery response: missing required fields (token_endpoint)",
            httpStatus = 200,
        )
    }
    // Reject present-but-empty endpoint strings.
    for ((name, value) in listOf(
        "authorization_endpoint" to raw.authorizationEndpoint,
        "device_authorization_endpoint" to raw.deviceAuthorizationEndpoint,
        "registration_endpoint" to raw.registrationEndpoint,
    )) {
        if (value != null && value.isEmpty()) {
            throw BasecampException.Api("Invalid OAuth discovery response: empty $name", httpStatus = 200)
        }
    }

    return OAuthConfig(
        issuer = issuer,
        authorizationEndpoint = raw.authorizationEndpoint,
        tokenEndpoint = token,
        deviceAuthorizationEndpoint = raw.deviceAuthorizationEndpoint,
        registrationEndpoint = raw.registrationEndpoint,
        scopesSupported = raw.scopesSupported,
        grantTypesSupported = raw.grantTypesSupported,
    )
}

/**
 * Discovers RFC 9728 protected-resource metadata from
 * `{resourceOrigin}/.well-known/oauth-protected-resource`. `resource` is
 * required and MUST equal the requested origin by code-point.
 * `authorization_servers` is preserved distinctly as absent (`null`) vs `[]`.
 *
 * @throws BasecampException.Usage on a malformed caller origin.
 * @throws BasecampException.Api on invalid/mismatched metadata.
 * @throws BasecampException.Network on transport failure or timeout.
 */
suspend fun discoverProtectedResource(
    resourceOrigin: String,
    client: HttpClient? = null,
): ProtectedResourceMetadata {
    val origin = requireOriginRoot(resourceOrigin, "resource origin")
    val url = "$origin/.well-known/oauth-protected-resource"

    val body = fetchDiscoveryDocument(url, client)
    val raw = try {
        discoveryJson.decodeFromString<RawResourceResponse>(body)
    } catch (e: SerializationException) {
        throw BasecampException.Api("Failed to parse resource metadata response", httpStatus = 200, cause = e)
    }

    val resource = raw.resource
    if (resource.isNullOrEmpty()) {
        throw BasecampException.Api(
            "Invalid resource metadata: missing required field (resource)",
            httpStatus = 200,
        )
    }
    // Bind resource identifier to the requested origin, code-point exact.
    if (resource != origin) {
        throw BasecampException.Api(
            "Resource identifier mismatch: metadata resource \"$resource\" does not equal \"$origin\"",
            httpStatus = 200,
        )
    }

    return ProtectedResourceMetadata(resource = resource, authorizationServers = raw.authorizationServers)
}

/**
 * Resource-first discovery orchestrator (SPEC.md §16). Composes RFC 9728 + RFC
 * 8414 and applies the stage-sensitive fallback state machine.
 *
 * Returns [DiscoveryResult.Selected] or [DiscoveryResult.FallBack] where the
 * reason is one of [FallbackReason]'s two values ONLY. Every hard failure throws
 * [BasecampException.DiscoverySelection] — callers MUST NOT convert a throw into
 * a Launchpad request. ("BC5 committed" = valid resource metadata advertised a
 * BC5 issuer that was then selected; afterward every failure is fatal.)
 *
 * @param resourceOrigin Caller-supplied API/resource origin (hop 1).
 * @param expectedIssuer Optional authoritative issuer selection. When provided,
 *   the advertised member equal by code-point is selected; if none matches,
 *   `expected_issuer_unavailable` is raised (never falls back). Omit to use the
 *   exactly-one-non-Launchpad exclusion heuristic.
 * @param client Optional HTTP client (a hardened one is created if not provided).
 * @throws BasecampException.Usage on a malformed caller origin.
 * @throws BasecampException.DiscoverySelection on any hard selection failure.
 */
suspend fun discoverFromResource(
    resourceOrigin: String,
    expectedIssuer: String? = null,
    client: HttpClient? = null,
): DiscoveryResult {
    // Origin-root validation of the *caller's* input is a usage error — let it
    // propagate as-is (not a soft fallback).
    val origin = requireOriginRoot(resourceOrigin, "resource origin")

    // --- Hop 1: resource metadata. Failure here is soft (before selection). ---
    val resource: ProtectedResourceMetadata = try {
        discoverProtectedResource(origin, client)
    } catch (e: BasecampException.Usage) {
        throw e
    } catch (e: BasecampException) {
        return DiscoveryResult.FallBack(FallbackReason.RESOURCE_DISCOVERY_FAILED)
    }

    val advertised = resource.authorizationServers ?: emptyList()

    // --- Selection ---
    val selectedIssuer: String
    if (expectedIssuer != null) {
        selectedIssuer = advertised.firstOrNull { it == expectedIssuer }
            ?: throw BasecampException.DiscoverySelection(
                "expected_issuer_unavailable",
                "Expected issuer \"$expectedIssuer\" is not advertised by the resource",
            )
    } else {
        // Dedupe by code-point: the same non-Launchpad issuer advertised more
        // than once is ONE candidate, not an ambiguity.
        val nonLaunchpad = advertised.filterNot { isLaunchpadIssuer(it) }.distinct()
        when {
            nonLaunchpad.size >= 2 -> throw BasecampException.DiscoverySelection(
                "ambiguous_issuers",
                "Multiple non-Launchpad issuers advertised; pass expectedIssuer to disambiguate: " +
                    nonLaunchpad.joinToString(", "),
            )
            // Valid resource metadata omits BC5 — soft fallback (before selection).
            nonLaunchpad.isEmpty() -> return DiscoveryResult.FallBack(FallbackReason.NO_AS_ADVERTISED)
            else -> selectedIssuer = nonLaunchpad[0]
        }
    }

    // --- BC5 is now committed: every subsequent failure is fatal (no Launchpad). ---
    val issuerOrigin = try {
        requireOriginRoot(selectedIssuer, "advertised issuer")
    } catch (e: BasecampException) {
        throw BasecampException.DiscoverySelection(
            "invalid_issuer_origin",
            "Advertised issuer \"$selectedIssuer\" is not a valid origin root",
            cause = e,
        )
    }

    val config = try {
        // Call the binding path directly (not the public discover, which converts
        // the marker to api_error) so the structural marker reaches the branch.
        // Bind against the exact advertised issuer, not the normalized origin.
        fetchAndBindAsMetadata(issuerOrigin, client, bindIssuer = selectedIssuer)
    } catch (e: IssuerBindingException) {
        // Structured marker — never the message text — decides issuer_mismatch.
        throw BasecampException.DiscoverySelection("issuer_mismatch", e.message!!, cause = e)
    } catch (e: BasecampException) {
        // Every other committed-AS fault (5xx, missing token_endpoint, parse
        // failure, transport, …) is a generic fetch failure.
        throw BasecampException.DiscoverySelection(
            "as_fetch_failed",
            "AS metadata fetch failed for committed issuer \"$issuerOrigin\": ${e.message}",
            cause = e,
        )
    }

    return DiscoveryResult.Selected(config, config.issuer)
}

/**
 * Discovers OAuth configuration from Basecamp's Launchpad server.
 */
suspend fun discoverLaunchpad(client: HttpClient? = null): OAuthConfig =
    discover(LAUNCHPAD_BASE_URL, client)

/**
 * True when an issuer string is a valid origin root equal to Launchpad's.
 *
 * Both sides run through [requireOriginRoot], so an advertised look-alike that
 * is not a clean origin root — e.g. `https://launchpad.37signals.com/path`
 * (path), userinfo, or a query — is not treated as Launchpad. It stays a
 * non-Launchpad candidate and later fails hard (`ambiguous_issuers` /
 * `invalid_issuer_origin`) rather than being silently excluded. A
 * trailing-slash-only origin root still matches because [requireOriginRoot]
 * normalizes it away.
 */
private fun isLaunchpadIssuer(issuer: String): Boolean =
    try {
        requireOriginRoot(issuer, "issuer") == requireOriginRoot(LAUNCHPAD_BASE_URL, "issuer")
    } catch (e: BasecampException.Usage) {
        false
    }

/**
 * SSRF-hardened GET of a discovery document. The origin must already be
 * validated (via [requireOriginRoot]). Suppresses redirects, bounds the timeout,
 * reads the body under a bounded/streaming cap, and maps non-2xx → api_error.
 *
 * When [baseClient] is supplied its engine is reused but wrapped in a hardened
 * client so redirect suppression and the timeout apply regardless; the borrowed
 * engine is not closed by the wrapper (Ktor only closes engines it created).
 */
private suspend fun fetchDiscoveryDocument(url: String, baseClient: HttpClient?): String {
    val engine = baseClient?.engine
    val httpClient = if (engine != null) {
        HttpClient(engine) {
            followRedirects = false
            expectSuccess = false
            install(HttpTimeout) { requestTimeoutMillis = DISCOVERY_TIMEOUT_MS }
        }
    } else {
        HttpClient {
            followRedirects = false
            expectSuccess = false
            install(HttpTimeout) { requestTimeoutMillis = DISCOVERY_TIMEOUT_MS }
        }
    }

    try {
        return httpClient.prepareGet(url) {
            accept(ContentType.Application.Json)
        }.execute { response ->
            val status = response.status.value
            if (status < 200 || status >= 300) {
                // Non-2xx (including a suppressed 3xx) surfaces as api_error, not network.
                throw BasecampException.Api("OAuth discovery failed: HTTP $status", httpStatus = status)
            }
            readBoundedText(response, MAX_DISCOVERY_BODY_BYTES)
        }
    } catch (e: CancellationException) {
        // Coroutine cancellation must propagate untouched — never soft-fall-back
        // to Launchpad by masquerading a cancelled fetch as a Network error.
        throw e
    } catch (e: BasecampException) {
        throw e
    } catch (e: Throwable) {
        // Transport failure / timeout.
        throw BasecampException.Network(
            "OAuth discovery failed: ${e.message ?: e::class.simpleName}",
            cause = e,
        )
    } finally {
        httpClient.close()
    }
}

/**
 * Reads a response body under a bounded, streaming cap. Aborts (cancels the
 * channel) the moment the accumulated size exceeds [maxBytes], so an oversized
 * body is never fully buffered — real memory bounding, not a post-hoc check.
 */
internal suspend fun readBoundedText(response: HttpResponse, maxBytes: Long): String {
    val channel = response.bodyAsChannel()
    val chunks = ArrayList<ByteArray>()
    var total = 0L
    val buffer = ByteArray(16 * 1024)

    while (true) {
        val read = channel.readAvailable(buffer, 0, buffer.size)
        if (read == -1) break
        if (read == 0) continue
        total += read
        if (total > maxBytes) {
            channel.cancel(null)
            throw BasecampException.Api(
                "OAuth response exceeds size cap",
                httpStatus = response.status.value,
            )
        }
        chunks.add(buffer.copyOf(read))
    }

    val merged = ByteArray(total.toInt())
    var offset = 0
    for (chunk in chunks) {
        chunk.copyInto(merged, offset)
        offset += chunk.size
    }
    return merged.decodeToString()
}
