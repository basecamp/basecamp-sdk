package com.basecamp.sdk.oauth

import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.http.currentTimeMillis
import com.basecamp.sdk.requireSecureEndpoint
import io.ktor.client.HttpClient
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.request.accept
import io.ktor.client.request.forms.FormDataContent
import io.ktor.client.request.preparePost
import io.ktor.client.request.setBody
import io.ktor.http.ContentType
import io.ktor.http.Parameters
import io.ktor.http.parametersOf
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.serialization.SerialName
import kotlinx.serialization.SerializationException
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.TimeMark
import kotlin.time.TimeSource

/**
 * RFC 8628 device authorization grant (SPEC.md §16).
 *
 * Three suspend functions mirror the shipping TypeScript reference in the Kotlin
 * coroutines idiom:
 *   - [requestDeviceAuthorization] — obtain a device/user code pair.
 *   - [pollDeviceToken] — run the §3.5 polling loop against the token endpoint.
 *   - [performDeviceLogin] — orchestrate request → display → poll for an
 *     already-selected [OAuthConfig].
 *
 * Both network-facing functions are TLS-guarded via [requireSecureEndpoint]. The
 * polling clock is a monotonic [TimeSource] (default [TimeSource.Monotonic]),
 * injectable so tests lock it to virtual time. Waiting uses [delay], so `runTest`
 * drives it with virtual time and cancellation is cooperative: a
 * [CancellationException] propagates untouched rather than becoming a
 * [BasecampException.DeviceFlow].
 */

/** URN grant type for the device authorization grant (RFC 8628 §3.4). */
const val DEVICE_CODE_GRANT_TYPE = "urn:ietf:params:oauth:grant-type:device_code"

/** Default polling interval when the server omits `interval` (RFC 8628 §3.2). */
private const val DEFAULT_INTERVAL_SECONDS = 5L

/** slow_down bumps the interval by this many seconds, sustained (RFC 8628 §3.5). */
private const val SLOW_DOWN_INCREMENT_SECONDS = 5L

/** Cap on exponential backoff after connection timeouts. */
private const val MAX_BACKOFF_SECONDS = 60L

/** Bounded per-request timeout for every device-flow HTTP round-trip. */
private const val DEVICE_REQUEST_TIMEOUT_MS = 30_000L

/** Cap on a device-flow response body (1 MiB) — device/token docs are tiny. */
private const val MAX_DEVICE_BODY_BYTES = 1L * 1024 * 1024

/**
 * Ceiling for `expires_in`/`interval`: 2147483 s (~24.8 days) is the largest
 * whole-second duration whose millisecond form fits a 32-bit signed timer.
 * Shared across all five SDKs (SPEC.md) — an unbounded value such as 1e100 is
 * a malformed response, not a schedulable deadline.
 */
private const val MAX_DEVICE_SECONDS = 2_147_483L

/**
 * Ceiling for an OAuth token's `expires_in` (2147483647 s ~= 68 years):
 * cross-runtime safe and vastly beyond any realistic token lifetime. Unlike
 * [MAX_DEVICE_SECONDS] this bounds `expiresAt` arithmetic rather than a timer —
 * a large finite value (e.g. `Long.MAX_VALUE`) would overflow `it * 1000` and
 * yield a garbage deadline, so a value past this ceiling is a malformed
 * response. Shared across all five SDKs.
 */
private const val MAX_TOKEN_LIFETIME_SECONDS = 2_147_483_647L

private val deviceJson = Json { ignoreUnknownKeys = true }

/**
 * Builds an SSRF-hardened HTTP client for device-flow requests: redirects
 * suppressed ([HttpClient.followRedirects] = false, so a 3xx surfaces as a
 * non-2xx rather than the client chasing an attacker-influenced `Location`) and
 * a bounded per-request timeout (HttpTimeout) so a stalled request can never
 * blow past the polling deadline unbounded.
 *
 * When [baseClient] is supplied its engine is reused but wrapped so the hardening
 * applies regardless; the returned wrapper is always closed by the caller and,
 * because Ktor only closes engines it created, the borrowed engine survives.
 */
private fun hardenedDeviceClient(baseClient: HttpClient?): HttpClient {
    val engine = baseClient?.engine
    return if (engine != null) {
        HttpClient(engine) {
            followRedirects = false
            expectSuccess = false
            install(HttpTimeout) { requestTimeoutMillis = DEVICE_REQUEST_TIMEOUT_MS }
        }
    } else {
        HttpClient {
            followRedirects = false
            expectSuccess = false
            install(HttpTimeout) { requestTimeoutMillis = DEVICE_REQUEST_TIMEOUT_MS }
        }
    }
}

/**
 * A device/user code pair from the authorization server (RFC 8628 §3.2).
 *
 * @property verificationUriComplete Optional pre-filled verification URI
 *   (`verification_uri` with the user code embedded).
 * @property expiresIn Lifetime of the codes in seconds.
 * @property interval Minimum seconds between token polls (defaults to 5).
 */
data class DeviceAuthorization(
    val deviceCode: String,
    val userCode: String,
    val verificationUri: String,
    val verificationUriComplete: String?,
    val expiresIn: Long,
    val interval: Long,
)

/**
 * Raw token response for the device path. `expires_in` decodes as [Double] (the
 * shared [RawTokenResponse] uses [Long], which throws on an integer-valued float
 * like 3600.0) so the cross-SDK contract — accept 3600.0, reject fractional —
 * can be enforced in validation rather than by decoder happenstance. `token_type`
 * stays nullable (no decoder default) so an explicit `"token_type": ""` remains
 * distinguishable from an absent field: absent defaults to Bearer, explicit empty
 * is malformed (api_error) — matching the other SDKs.
 */
@Serializable
private data class RawDeviceTokenResponse(
    @SerialName("access_token") val accessToken: String,
    @SerialName("refresh_token") val refreshToken: String? = null,
    @SerialName("token_type") val tokenType: String? = null,
    @SerialName("expires_in") val expiresIn: Double? = null,
    val scope: String? = null,
)

/** Raw RFC 8628 device authorization response; all fields nullable to validate. */
@Serializable
private data class RawDeviceAuthorization(
    @SerialName("device_code") val deviceCode: String? = null,
    @SerialName("user_code") val userCode: String? = null,
    @SerialName("verification_uri") val verificationUri: String? = null,
    @SerialName("verification_uri_complete") val verificationUriComplete: String? = null,
    // Decoded as Double so an integer-valued float (900.0) parses instead of
    // throwing; whole-second enforcement + Long conversion happens in validation,
    // matching the other SDKs (which accept 900.0 but reject fractional 2.5).
    @SerialName("expires_in") val expiresIn: Double? = null,
    val interval: Double? = null,
)

/**
 * RFC 8628 durations are integer seconds. Returns the whole-second [Long] for a
 * positive integer-valued number (900 or 900.0) no greater than
 * [MAX_DEVICE_SECONDS], or null for absent, non-positive, fractional (2.5), or
 * oversized (1e100) values — the caller raises api_error on null.
 */
private fun wholeSecondsOrNull(value: Double?): Long? {
    if (value == null || value <= 0.0 || value > MAX_DEVICE_SECONDS.toDouble() || value % 1.0 != 0.0) return null
    return value.toLong()
}

/**
 * Requests a device/user code pair (RFC 8628 §3.1–3.2).
 *
 * POSTs `client_id` (and `scope` ONLY when set, so the server applies its default
 * `read`) to [deviceAuthorizationEndpoint] as `application/x-www-form-urlencoded`.
 * TLS-guarded before any socket opens.
 *
 * ```kotlin
 * val auth = requestDeviceAuthorization(config.deviceAuthorizationEndpoint!!, "basecamp-cli")
 * println("Visit ${auth.verificationUri} and enter ${auth.userCode}")
 * ```
 *
 * @throws BasecampException.Validation if [clientId] is empty.
 * @throws BasecampException.DeviceFlow (reason `transport`) on a network failure.
 * @throws BasecampException.Api on a non-2xx response or invalid/incomplete body.
 */
suspend fun requestDeviceAuthorization(
    deviceAuthorizationEndpoint: String,
    clientId: String,
    scope: String? = null,
    client: HttpClient? = null,
): DeviceAuthorization {
    requireSecureEndpoint(deviceAuthorizationEndpoint, "device authorization endpoint")
    if (clientId.isEmpty()) {
        throw BasecampException.Validation("Client ID is required for device authorization")
    }

    val params = Parameters.build {
        append("client_id", clientId)
        // Omit scope entirely when unset so the server applies its default (`read`).
        if (!scope.isNullOrEmpty()) append("scope", scope)
    }

    val httpClient = hardenedDeviceClient(client)
    try {
        return httpClient.preparePost(deviceAuthorizationEndpoint) {
            accept(ContentType.Application.Json)
            setBody(FormDataContent(params))
        }.execute { response ->
            val status = response.status.value
            if (status < 200 || status >= 300) {
                // Non-2xx (including a suppressed 3xx) is api_error, not transport.
                // retryable = false even for a 5xx (overriding Api's 5xx default):
                // in the device flow only the transport reason is retryable — a
                // completed API fault ends the flow, matching the other four SDKs.
                throw BasecampException.Api(
                    "Device authorization failed with status $status",
                    httpStatus = status,
                    retryable = false,
                )
            }
            val body = readBoundedText(response, MAX_DEVICE_BODY_BYTES)
            val raw = try {
                deviceJson.decodeFromString<RawDeviceAuthorization>(body)
            } catch (e: SerializationException) {
                throw BasecampException.Api(
                    "Failed to parse device authorization response",
                    httpStatus = status,
                    cause = e,
                )
            }
            validateDeviceAuthorization(raw, status)
        }
    } catch (e: CancellationException) {
        // Cooperative coroutine cancellation — propagate, never wrap.
        throw e
    } catch (e: BasecampException) {
        throw e
    } catch (e: Throwable) {
        throw BasecampException.DeviceFlow(
            BasecampException.DEVICE_TRANSPORT,
            "Device authorization request failed: ${e.message ?: e::class.simpleName}",
            cause = e,
        )
    } finally {
        httpClient.close()
    }
}

private fun validateDeviceAuthorization(raw: RawDeviceAuthorization, status: Int): DeviceAuthorization {
    if (raw.deviceCode.isNullOrEmpty() || raw.userCode.isNullOrEmpty() || raw.verificationUri.isNullOrEmpty()) {
        throw BasecampException.Api("Invalid device authorization response: missing required fields", httpStatus = status)
    }
    val expiresIn = wholeSecondsOrNull(raw.expiresIn)
        ?: throw BasecampException.Api(
            "Invalid device authorization response: expires_in must be a positive integer no greater than $MAX_DEVICE_SECONDS",
            httpStatus = status,
        )
    val interval = when (raw.interval) {
        null -> DEFAULT_INTERVAL_SECONDS
        else -> wholeSecondsOrNull(raw.interval)
            ?: throw BasecampException.Api(
                "Invalid device authorization response: interval must be a positive integer no greater than $MAX_DEVICE_SECONDS",
                httpStatus = status,
            )
    }
    return DeviceAuthorization(
        deviceCode = raw.deviceCode,
        userCode = raw.userCode,
        verificationUri = raw.verificationUri,
        verificationUriComplete = raw.verificationUriComplete,
        expiresIn = expiresIn,
        interval = interval,
    )
}

/**
 * Polls the token endpoint until the user approves, denies, or the codes expire
 * (RFC 8628 §3.4–3.5).
 *
 * The loop waits at least [interval] seconds between polls (via [delay]), honours
 * a sustained `slow_down` (+5s for this and every subsequent poll), enforces a
 * monotonic expiry [deadline][TimeSource] computed from [timeSource], and backs
 * off exponentially on connection timeouts — the backoff is transient, reset to
 * the server-driven interval by the next completed round-trip, so intermittent
 * timeouts never permanently inflate the cadence. Coroutine cancellation propagates as
 * a [CancellationException] — it is never converted to a
 * [BasecampException.DeviceFlow].
 *
 * @param interval Minimum seconds between polls.
 * @param expiresIn Code lifetime in seconds; anchors the default [deadline].
 * @param timeSource Monotonic clock for the deadline; tests inject virtual time.
 * @param deadline Absolute expiry deadline. Defaults to `expiresIn` from now, but
 *   [performDeviceLogin] passes a deadline anchored at code issuance so display
 *   time counts against the lifetime rather than resetting it.
 * @throws BasecampException.DeviceFlow reason `access_denied` on denial,
 *   `expired` on expiry, `transport` on a non-timeout network failure.
 * @throws BasecampException.Api on an unrecognized error code or a parse failure.
 * @throws BasecampException.Usage when [interval] or [expiresIn] is not a
 *   positive number of seconds within the shared device ceiling.
 */
suspend fun pollDeviceToken(
    tokenEndpoint: String,
    clientId: String,
    deviceCode: String,
    interval: Long,
    expiresIn: Long,
    timeSource: TimeSource = TimeSource.Monotonic,
    client: HttpClient? = null,
    deadline: TimeMark = timeSource.markNow() + expiresIn.seconds,
): OAuthToken {
    requireSecureEndpoint(tokenEndpoint, "token endpoint")

    // Caller-input sanity for this exported entry point (usage, not RFC response
    // validation, which performDeviceLogin's path already applies): a non-positive
    // duration builds an already-passed deadline, and an oversized one saturates
    // Duration to infinite so the deadline NEVER passes — an unbounded poll loop.
    // Mirrors the Go/TS/Python/Ruby caller guards.
    for ((name, value) in listOf("expiresIn" to expiresIn, "interval" to interval)) {
        if (value <= 0 || value > MAX_DEVICE_SECONDS) {
            throw BasecampException.Usage(
                "pollDeviceToken: $name must be a positive number of seconds no greater than $MAX_DEVICE_SECONDS",
            )
        }
    }

    // Server-driven cadence: the initial interval plus sustained slow_down bumps.
    var intervalSeconds = interval
    // Transient timeout backoff, tracked SEPARATELY from the server-driven
    // interval so intermittent timeouts can never permanently inflate the
    // cadence: each wait is max(interval, backoff), a timeout doubles the
    // backoff (capped), and any completed round-trip resets it to the interval.
    var backoffSeconds = intervalSeconds

    val params = parametersOf(
        "grant_type" to listOf(DEVICE_CODE_GRANT_TYPE),
        "device_code" to listOf(deviceCode),
        "client_id" to listOf(clientId),
    )

    val httpClient = hardenedDeviceClient(client)
    try {
        while (true) {
            // Re-check the deadline before waiting/polling (after the display hook
            // on the first pass, before every poll thereafter).
            if (deadline.hasPassedNow()) {
                throw BasecampException.DeviceFlow(BasecampException.DEVICE_EXPIRED)
            }
            // Clamp the wait to the time remaining so a long interval or an
            // exponential backoff can never overshoot the monotonic deadline.
            val remaining = -deadline.elapsedNow()
            val wait = minOf(maxOf(intervalSeconds, backoffSeconds).seconds, remaining)
            delay(if (wait > Duration.ZERO) wait else Duration.ZERO)

            if (deadline.hasPassedNow()) {
                throw BasecampException.DeviceFlow(BasecampException.DEVICE_EXPIRED)
            }

            val result = try {
                postDeviceTokenPoll(httpClient, tokenEndpoint, params)
            } catch (e: CancellationException) {
                // Cooperative coroutine cancellation — propagate, never wrap.
                throw e
            } catch (e: BasecampException) {
                throw e
            } catch (e: Throwable) {
                if (isConnectionTimeout(e)) {
                    // Our own connection timeout → back off and keep polling.
                    backoffSeconds = minOf(backoffSeconds * 2, MAX_BACKOFF_SECONDS)
                    continue
                }
                // Any other transport failure ends the flow.
                throw BasecampException.DeviceFlow(
                    BasecampException.DEVICE_TRANSPORT,
                    "Device token poll failed: ${e.message ?: e::class.simpleName}",
                    cause = e,
                )
            }

            // Any completed HTTP round-trip — token, authorization_pending,
            // slow_down, or another OAuth error — resets the timeout backoff to
            // the current server-driven interval.
            backoffSeconds = intervalSeconds

            when (result) {
                is PollResult.Token -> return result.token
                PollResult.Pending -> continue
                PollResult.SlowDown -> {
                    intervalSeconds += SLOW_DOWN_INCREMENT_SECONDS
                    // Re-sync the backoff to the GROWN interval (the reset above used
                    // the pre-increment value) so a later timeout doubles from the new
                    // interval, not the stale one.
                    backoffSeconds = intervalSeconds
                    continue
                }
                PollResult.AccessDenied -> throw BasecampException.DeviceFlow(BasecampException.DEVICE_ACCESS_DENIED)
                PollResult.Expired -> throw BasecampException.DeviceFlow(BasecampException.DEVICE_EXPIRED)
                // retryable = false even for a 5xx (overriding Api's 5xx default):
                // only the transport reason is retryable in the device flow.
                is PollResult.Other -> throw BasecampException.Api(
                    "Device token request failed: ${result.error}",
                    httpStatus = result.status,
                    retryable = false,
                )
            }
        }
    } finally {
        httpClient.close()
    }
    // The while(true) loop only exits via `return` or `throw`, so this point is
    // unreachable and Kotlin requires no trailing return.
}

/** One token-poll outcome (RFC 8628 §3.5). */
private sealed interface PollResult {
    data class Token(val token: OAuthToken) : PollResult
    data object Pending : PollResult
    data object SlowDown : PollResult
    data object AccessDenied : PollResult
    data object Expired : PollResult
    data class Other(val error: String, val status: Int) : PollResult
}

private suspend fun postDeviceTokenPoll(
    client: HttpClient,
    tokenEndpoint: String,
    params: Parameters,
): PollResult = client.preparePost(tokenEndpoint) {
    accept(ContentType.Application.Json)
    setBody(FormDataContent(params))
}.execute { response ->
    val status = response.status.value
    // A suppressed 3xx is an api fault whose body is unused (redirects are off, so a
    // 3xx is a misdirected/attacker-influenced endpoint, never an OAuth poll state).
    // Classify it by status BEFORE reading the body: otherwise a redirect that
    // slowly streams its body could time out mid-read and be retried by the poll
    // loop until expiry, and a body that parrots {"error":"authorization_pending"}
    // must not keep the loop polling. The message names the redirect explicitly.
    if (status in 300..399) {
        // The unread body is NOT leaked by this early return: HttpStatement.execute
        // runs cleanup() in its finally, which completes the response job and
        // cancels the raw content channel, releasing the connection.
        return@execute PollResult.Other("unexpected redirect (HTTP $status)", status)
    }

    // Bounded/streaming read: an oversized device-token body aborts here rather than
    // buffering (readBoundedText throws api_error past the cap). A 4xx body IS read —
    // it carries authorization_pending/slow_down and other OAuth errors.
    val body = readBoundedText(response, MAX_DEVICE_BODY_BYTES)

    if (status in 200..299) {
        val raw = try {
            deviceJson.decodeFromString<RawDeviceTokenResponse>(body)
        } catch (e: SerializationException) {
            // Malformed 2xx token response — api_error, NOT a retryable transport.
            throw BasecampException.Api("Failed to parse device token response", httpStatus = status, cause = e)
        }
        if (raw.accessToken.isBlank()) {
            // A 2xx with an empty/blank access_token is a server/api fault
            // (api_error), never an accepted token nor a retryable transport error.
            throw BasecampException.Api("Device token response missing access_token", httpStatus = status)
        }
        // A non-numeric expires_in (string/bool) already fails deserialization
        // above as api_error. What survives is a Double: per the cross-SDK
        // contract it must be a positive WHOLE number of seconds no greater than
        // the ceiling — an explicit 0, a fractional 3600.5, or an overflowing
        // 1e400 (parses to Infinity) is malformed, while 3600.0 is accepted and
        // converted to whole seconds. The ceiling keeps `* 1000` from wrapping.
        val expiresInSeconds = raw.expiresIn?.let {
            if (it <= 0.0 || it > MAX_TOKEN_LIFETIME_SECONDS.toDouble() || it % 1.0 != 0.0) {
                throw BasecampException.Api(
                    "Device token response expires_in must be a positive whole number of seconds " +
                        "no greater than $MAX_TOKEN_LIFETIME_SECONDS",
                    httpStatus = status,
                )
            }
            it.toLong()
        }
        // token_type, when present, must be non-empty — an explicit "" is
        // malformed token metadata (api_error), while an absent field defaults
        // to Bearer. Uniform with Go/Python/Ruby/TypeScript.
        val tokenType = raw.tokenType?.also {
            if (it.isEmpty()) {
                throw BasecampException.Api(
                    "Device token response token_type must be a non-empty string",
                    httpStatus = status,
                )
            }
        } ?: "Bearer"
        val now = currentTimeMillis()
        val expiresAt = expiresInSeconds?.let { now + it * 1000 }
        PollResult.Token(
            OAuthToken(
                accessToken = raw.accessToken,
                refreshToken = raw.refreshToken,
                tokenType = tokenType,
                expiresIn = expiresInSeconds,
                expiresAt = expiresAt,
                scope = raw.scope,
            ),
        )
    } else {
        // 4xx: the OAuth error is carried in the body (3xx already returned above).
        val error = try {
            // An explicit empty "error" decodes cleanly (the field is a required
            // non-null String, so no SerializationException fires) — normalize it to
            // http_<status> here so a blank error code is never surfaced as a dangling
            // message. Matches Go/TS/Python/Ruby, which all coerce a blank error code.
            deviceJson.decodeFromString<OAuthErrorResponse>(body).error.ifEmpty { "http_$status" }
        } catch (e: SerializationException) {
            "http_$status"
        }
        when (error) {
            "authorization_pending" -> PollResult.Pending
            "slow_down" -> PollResult.SlowDown
            "access_denied" -> PollResult.AccessDenied
            "expired_token" -> PollResult.Expired
            else -> PollResult.Other(error, status)
        }
    }
}

/**
 * Runs the full device authorization grant against an ALREADY-SELECTED [config]
 * (from discovery): capability guard → request → [display] → poll (SPEC.md §16).
 *
 * The capability guard requires BOTH [OAuthConfig.deviceAuthorizationEndpoint]
 * present AND [OAuthConfig.grantTypesSupported] advertising the device_code grant;
 * otherwise it raises [BasecampException.DeviceFlow] reason `unavailable`.
 *
 * @param display Invoked once with the code pair, after it is obtained and before
 *   polling begins — surface `userCode` + `verificationUri` to the user here. The
 *   expiry deadline is anchored at code issuance (BEFORE this hook), so time spent
 *   in `display` counts against the code's lifetime rather than resetting it; a
 *   hook that consumes the whole lifetime yields `expired` without any poll.
 * @throws BasecampException.DeviceFlow reason `unavailable` when [config] cannot
 *   do device flow; `expired` when the display hook outlasts the code; other
 *   reasons on denial/expiry/transport per [pollDeviceToken].
 */
suspend fun performDeviceLogin(
    config: OAuthConfig,
    clientId: String,
    scope: String? = null,
    display: (DeviceAuthorization) -> Unit,
    timeSource: TimeSource = TimeSource.Monotonic,
    client: HttpClient? = null,
): OAuthToken {
    val endpoint = config.deviceAuthorizationEndpoint
    val supportsDeviceGrant = config.grantTypesSupported.contains(DEVICE_CODE_GRANT_TYPE)
    // An empty endpoint string is treated as absent (not device-flow-capable): it
    // must yield `unavailable` here rather than slipping through to fail later with
    // the wrong error category when the empty URL hits the TLS/socket layer.
    if (endpoint.isNullOrEmpty() || !supportsDeviceGrant) {
        throw BasecampException.DeviceFlow(BasecampException.DEVICE_UNAVAILABLE)
    }

    val auth = requestDeviceAuthorization(endpoint, clientId, scope, client)
    // Anchor the code's expiry deadline at issuance — BEFORE the display hook — so
    // a slow display counts against the lifetime instead of resetting it.
    val deadline = timeSource.markNow() + auth.expiresIn.seconds
    display(auth)
    // If the display hook already consumed the whole lifetime, the code is dead on
    // arrival: fail fast rather than open a doomed poll.
    if (deadline.hasPassedNow()) {
        throw BasecampException.DeviceFlow(BasecampException.DEVICE_EXPIRED)
    }
    return pollDeviceToken(
        tokenEndpoint = config.tokenEndpoint,
        clientId = clientId,
        deviceCode = auth.deviceCode,
        interval = auth.interval,
        expiresIn = auth.expiresIn,
        timeSource = timeSource,
        client = client,
        deadline = deadline,
    )
}

/**
 * True when [e] (or any cause in its chain) denotes a connection timeout — a
 * signal to back off and keep polling rather than end the flow. Matched by class
 * name so it stays engine-agnostic across Ktor transports (JVM/Native), while
 * genuine [CancellationException]s are handled separately and never reach here.
 */
private fun isConnectionTimeout(e: Throwable): Boolean {
    var current: Throwable? = e
    while (current != null) {
        if (current::class.simpleName?.contains("Timeout", ignoreCase = true) == true) return true
        current = current.cause
    }
    return false
}
