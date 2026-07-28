package com.basecamp.sdk

import com.basecamp.sdk.oauth.DEVICE_CODE_GRANT_TYPE
import com.basecamp.sdk.oauth.DeviceAuthorization
import com.basecamp.sdk.oauth.OAuthConfig
import com.basecamp.sdk.oauth.performDeviceLogin
import com.basecamp.sdk.oauth.pollDeviceToken
import com.basecamp.sdk.oauth.requestDeviceAuthorization
import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.engine.mock.toByteArray
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.utils.io.ByteReadChannel
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.withContext
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.testTimeSource
import kotlinx.coroutines.withTimeout
import kotlin.time.Duration.Companion.seconds
import kotlin.time.TestTimeSource
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * RFC 8628 device authorization grant tests (SPEC.md §16).
 *
 * `runTest` supplies virtual time so [kotlinx.coroutines.delay] resolves
 * instantly, `testTimeSource` locks the monotonic deadline to that same virtual
 * clock, and `testScheduler.currentTime` reads the schedule so we can assert the
 * poll cadence (sustained slow_down, timeout backoff) without real delays.
 */
@OptIn(ExperimentalCoroutinesApi::class)
class OAuthDeviceTest {

    private val origin = "https://issuer.device-test.example"
    private val deviceEndpoint = "$origin/oauth/device"
    private val tokenEndpoint = "$origin/oauth/token"

    private val jsonHeaders = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString())

    private val deviceAuthJson = """
        {
          "device_code": "dev-code-123",
          "user_code": "WDJB-MJHT",
          "verification_uri": "$origin/device",
          "verification_uri_complete": "$origin/device?user_code=WDJB-MJHT",
          "expires_in": 900,
          "interval": 5
        }
    """.trimIndent()

    private val tokenJson = """
        {
          "access_token": "device_access_token",
          "refresh_token": "device_refresh_token",
          "token_type": "Bearer",
          "expires_in": 3600
        }
    """.trimIndent()

    private fun errorJson(error: String) = """{"error":"$error"}"""

    /** A non-timeout throwable whose class name does NOT match the timeout heuristic. */
    private class SimulatedTransportException : Exception("connection reset")

    /** A throwable whose class name matches the timeout heuristic (connection timeout). */
    private class SimulatedConnectTimeoutException : Exception("simulated connect timeout")

    // =========================================================================
    // requestDeviceAuthorization
    // =========================================================================

    @Test
    fun requestOmitsScopeWhenUnsetAndValidates() = runTest {
        var sentBody = ""
        val engine = MockEngine { request ->
            sentBody = request.body.toByteArray().decodeToString()
            respond(deviceAuthJson, HttpStatusCode.OK, jsonHeaders)
        }
        val client = HttpClient(engine)

        val auth = requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)

        assertTrue(sentBody.contains("client_id=basecamp-cli"))
        assertFalse(sentBody.contains("scope"), "scope must be omitted so the server applies its default")
        assertEquals("dev-code-123", auth.deviceCode)
        assertEquals("WDJB-MJHT", auth.userCode)
        assertEquals("$origin/device", auth.verificationUri)
        assertEquals(5L, auth.interval)
        client.close()
    }

    @Test
    fun requestSendsScopeWhenSet() = runTest {
        var sentBody = ""
        val engine = MockEngine { request ->
            sentBody = request.body.toByteArray().decodeToString()
            respond(deviceAuthJson, HttpStatusCode.OK, jsonHeaders)
        }
        val client = HttpClient(engine)

        requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", scope = "read write", client = client)

        assertTrue(sentBody.contains("scope=read+write"), "the FULL scope must be sent when set: $sentBody")
        client.close()
    }

    @Test
    fun requestDefaultsIntervalToFiveWhenAbsent() = runTest {
        val body = """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":900}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val auth = requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)

        assertEquals(5L, auth.interval)
        client.close()
    }

    @Test
    fun deviceApiFaultsAreNotRetryable() = runTest {
        // Api's constructor defaults retryable = true for a 5xx, but in the device
        // flow only the transport reason is retryable — a completed 5xx from the
        // device-auth or token endpoint ends the flow non-retryably, matching the
        // other four SDKs' non-retryable api_error.
        val engine = MockEngine { respond("boom", HttpStatusCode.InternalServerError, jsonHeaders) }
        val client = HttpClient(engine)

        val requestError = assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        assertEquals(500, requestError.httpStatus)
        assertFalse(requestError.retryable, "a completed 5xx device-auth response is not retryable")
        client.close()

        val pollEngine = MockEngine { respond("""{"error":"boom"}""", HttpStatusCode.InternalServerError, jsonHeaders) }
        val pollClient = HttpClient(pollEngine)
        val pollError = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, pollClient)
        }
        assertEquals(500, pollError.httpStatus)
        assertFalse(pollError.retryable, "a completed 5xx token response is not retryable")
        pollClient.close()
    }

    @Test
    fun requestRejectsNonPositiveExpiresIn() = runTest {
        val body = """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":0}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        client.close()
    }

    @Test
    fun requestRejectsNonPositiveInterval() = runTest {
        // A present but non-positive `interval` is invalid metadata — a poll cadence
        // of zero/negative seconds is nonsensical, so reject it rather than default.
        val body = """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":900,"interval":0}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        client.close()
    }

    @Test
    fun requestAcceptsIntegerValuedFloatDurations() = runTest {
        // 900.0 / 10.0 carry no fractional part → valid integer seconds. Decoding
        // as Long would throw SerializationException; the other SDKs accept these.
        val body = """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":900.0,"interval":10.0}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val auth = requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        assertEquals(900L, auth.expiresIn)
        assertEquals(10L, auth.interval)
        client.close()
    }

    @Test
    fun requestRejectsFractionalExpiresIn() = runTest {
        val body = """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":0.5}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        client.close()
    }

    @Test
    fun requestRejectsFractionalInterval() = runTest {
        val body = """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":900,"interval":2.5}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        client.close()
    }

    @Test
    fun requestRejectsOversizedDurations() = runTest {
        // 1e100 is integer-valued, so whole-second checking alone would admit it;
        // the shared cross-SDK ceiling (2147483 s) makes it api_error. The first
        // value past the boundary is likewise rejected.
        val bodies = listOf(
            """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":1e100}""",
            """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":900,"interval":1e100}""",
            """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":2147484}""",
        )
        for (body in bodies) {
            val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
            val client = HttpClient(engine)

            assertFailsWith<BasecampException.Api> {
                requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
            }
            client.close()
        }
    }

    @Test
    fun requestAcceptsMaxDuration() = runTest {
        // The 2147483 s ceiling itself is valid — the bound is inclusive.
        val body = """{"device_code":"d","user_code":"u","verification_uri":"$origin/device","expires_in":2147483,"interval":2147483}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val auth = requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        assertEquals(2_147_483L, auth.expiresIn)
        assertEquals(2_147_483L, auth.interval)
        client.close()
    }

    @Test
    fun requestRejectsEmptyDeviceCode() = runTest {
        // A present but empty device_code is as unusable as an absent one — reject it
        // as invalid metadata rather than carry a blank code into the poll loop.
        val body = """{"device_code":"","user_code":"u","verification_uri":"$origin/device","expires_in":900}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        client.close()
    }

    @Test
    fun requestRejectsMissingRequiredField() = runTest {
        // Missing device_code.
        val body = """{"user_code":"u","verification_uri":"$origin/device","expires_in":900}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        client.close()
    }

    @Test
    fun requestReportsActual2xxStatusOnMalformedBody() = runTest {
        // A malformed body returned with a non-200 success status (202) must report
        // the real status, not a hard-coded 200, on the thrown Api error.
        val body = """{"device_code":"d","user_code":"u","verification_uri":"$origin/device"}"""
        val engine = MockEngine { respond(body, HttpStatusCode.Accepted, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        assertEquals(202, e.httpStatus)
        client.close()
    }

    @Test
    fun requestAbortsOversizedBody() = runTest {
        // A well-formed but oversized (> 1 MiB) body: the bounded/streaming read
        // must abort before buffering the whole document.
        val huge = "{\"pad\":\"" + "x".repeat(1_100_000) + "\"}"
        val engine = MockEngine { respond(huge, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        client.close()
    }

    @Test
    fun requestDoesNotFollowRedirect() = runTest {
        var attackerContacted = false
        val engine = MockEngine { request ->
            if (request.url.host.contains("attacker")) {
                attackerContacted = true
                respond(deviceAuthJson, HttpStatusCode.OK, jsonHeaders)
            } else {
                respond(
                    content = ByteReadChannel(""),
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location, "https://attacker.example.com/oauth/device"),
                )
            }
        }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
        }
        assertEquals(302, e.httpStatus, "suppressed 3xx surfaces as api_error")
        assertFalse(attackerContacted, "device POST must not follow the redirect")
        client.close()
    }

    // =========================================================================
    // pollDeviceToken
    // =========================================================================

    @Test
    fun pollSustainsSlowDownIncrement() = runTest {
        val pollTimes = mutableListOf<Long>()
        val responses = listOf(
            HttpStatusCode.BadRequest to errorJson("authorization_pending"),
            HttpStatusCode.BadRequest to errorJson("slow_down"),
            HttpStatusCode.BadRequest to errorJson("authorization_pending"),
            HttpStatusCode.OK to tokenJson,
        )
        var i = 0
        val engine = MockEngine {
            pollTimes.add(testScheduler.currentTime)
            val (status, body) = responses[minOf(i, responses.size - 1)]
            i += 1
            respond(body, status, jsonHeaders)
        }
        val client = HttpClient(engine)

        val token = pollDeviceToken(
            tokenEndpoint = tokenEndpoint,
            clientId = "basecamp-cli",
            deviceCode = "dev-code-123",
            interval = 5,
            expiresIn = 900,
            timeSource = testTimeSource,
            client = client,
        )

        assertEquals("device_access_token", token.accessToken)
        // Waits: 5s, 5s (before slow_down), then sustained +5s → 10s, 10s.
        // Cumulative virtual time at each poll: 5s, 10s, 20s, 30s.
        assertEquals(listOf(5_000L, 10_000L, 20_000L, 30_000L), pollTimes)
        client.close()
    }

    @Test
    fun pollDoublesIntervalAfterConnectionTimeout() = runTest {
        val pollTimes = mutableListOf<Long>()
        var i = 0
        val engine = MockEngine {
            pollTimes.add(testScheduler.currentTime)
            i += 1
            if (i == 1) throw SimulatedConnectTimeoutException()
            respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
        }
        val client = HttpClient(engine)

        val token = pollDeviceToken(
            tokenEndpoint = tokenEndpoint,
            clientId = "basecamp-cli",
            deviceCode = "dev-code-123",
            interval = 5,
            expiresIn = 900,
            timeSource = testTimeSource,
            client = client,
        )

        assertEquals("device_access_token", token.accessToken)
        // First wait 5s; timeout doubles the backoff → next wait 10s (t=15s).
        assertEquals(listOf(5_000L, 15_000L), pollTimes)
        client.close()
    }

    @Test
    fun pollResetsTimeoutBackoffAfterCompletedRoundTrip() = runTest {
        // The timeout backoff is transient: two timeouts inflate the wait to
        // 10s then 20s, but the first completed round-trip (even a mere
        // authorization_pending) resets it to the server interval — later polls
        // must return to the 5s cadence, never stay permanently inflated.
        val pollTimes = mutableListOf<Long>()
        var i = 0
        val engine = MockEngine {
            pollTimes.add(testScheduler.currentTime)
            i += 1
            when (i) {
                1, 2 -> throw SimulatedConnectTimeoutException()
                3, 4 -> respond(errorJson("authorization_pending"), HttpStatusCode.BadRequest, jsonHeaders)
                else -> respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
            }
        }
        val client = HttpClient(engine)

        val token = pollDeviceToken(
            tokenEndpoint = tokenEndpoint,
            clientId = "basecamp-cli",
            deviceCode = "dev-code-123",
            interval = 5,
            expiresIn = 900,
            timeSource = testTimeSource,
            client = client,
        )

        assertEquals("device_access_token", token.accessToken)
        // Waits: 5s, then backoff 10s and 20s after the timeouts, then back to
        // the 5s server interval once a round-trip completes.
        assertEquals(listOf(5_000L, 15_000L, 35_000L, 40_000L, 45_000L), pollTimes)
        client.close()
    }

    @Test
    fun pollExpiresAgainstInjectedMonotonicClock() = runTest {
        // interval (5s) exceeds the code lifetime (3s): the first wait pushes
        // virtual time past the deadline before any poll is issued.
        val engine = MockEngine { respond(errorJson("authorization_pending"), HttpStatusCode.BadRequest, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.DeviceFlow> {
            pollDeviceToken(
                tokenEndpoint = tokenEndpoint,
                clientId = "basecamp-cli",
                deviceCode = "dev-code-123",
                interval = 5,
                expiresIn = 3,
                timeSource = testTimeSource,
                client = client,
            )
        }
        assertEquals(BasecampException.DEVICE_EXPIRED, e.reason)
        assertEquals("auth_required", e.code)
        assertEquals(3, e.exitCode)
        client.close()
    }

    @Test
    fun pollRaisesAccessDenied() = runTest {
        val engine = MockEngine { respond(errorJson("access_denied"), HttpStatusCode.BadRequest, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.DeviceFlow> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals(BasecampException.DEVICE_ACCESS_DENIED, e.reason)
        assertEquals("auth_required", e.code)
        client.close()
    }

    @Test
    fun pollRaisesTransportOnNonTimeoutFailure() = runTest {
        val engine = MockEngine { throw SimulatedTransportException() }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.DeviceFlow> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals(BasecampException.DEVICE_TRANSPORT, e.reason)
        assertEquals("network", e.code)
        assertTrue(e.retryable)
        client.close()
    }

    @Test
    fun pollPropagatesCoroutineCancellation() = runTest {
        // Never approves; the poll parks in delay(5s). A 3s timeout cancels it —
        // the CancellationException must propagate untouched (not become DeviceFlow),
        // so withTimeout surfaces a TimeoutCancellationException.
        val engine = MockEngine { respond(errorJson("authorization_pending"), HttpStatusCode.BadRequest, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<TimeoutCancellationException> {
            withTimeout(3_000) {
                pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
            }
        }
        client.close()
    }

    @Test
    fun performCancelDuringAuthorizationNeverReachesDisplay() = runTest {
        // The mock engine cancels the flow's job as it serves the code pair,
        // modeling an engine that completes while ignoring cancellation — the
        // display hook must never fire for a cancelled flow.
        var displayed = 0
        val job = Job()
        val engine = MockEngine {
            job.cancel()
            respond(deviceAuthJson, HttpStatusCode.OK, jsonHeaders)
        }
        val client = HttpClient(engine)
        try {
            assertFailsWith<CancellationException> {
                withContext(job) {
                    performDeviceLogin(
                        OAuthConfig(
                            issuer = origin,
                            authorizationEndpoint = null,
                            tokenEndpoint = tokenEndpoint,
                            deviceAuthorizationEndpoint = deviceEndpoint,
                            grantTypesSupported = listOf(DEVICE_CODE_GRANT_TYPE, "refresh_token"),
                        ),
                        "basecamp-cli",
                        display = { displayed += 1 },
                        timeSource = testTimeSource,
                        client = client,
                    )
                }
            }
        } finally {
            client.close()
        }
        assertEquals(0, displayed, "display must never fire for a cancelled flow")
    }

    // =========================================================================
    // pollDeviceToken 429 too_many_requests handling (SPEC §16)
    // =========================================================================

    private val tooManyJson = """{"error":"too_many_requests"}"""

    private fun headers429(retryAfter: String? = null) =
        if (retryAfter == null) {
            jsonHeaders
        } else {
            headersOf(
                HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                HttpHeaders.RetryAfter to listOf(retryAfter),
            )
        }

    @Test
    fun pollRetriesAfter429WithRetryAfterOverride() = runTest {
        val pollTimes = mutableListOf<Long>()
        var i = 0
        val engine = MockEngine {
            pollTimes.add(testScheduler.currentTime)
            i += 1
            if (i == 1) {
                respond(tooManyJson, HttpStatusCode.TooManyRequests, headers429("30"))
            } else {
                respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
            }
        }
        val client = HttpClient(engine)

        val token = pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)

        assertEquals("device_access_token", token.accessToken)
        // Initial 5s wait, then the one-shot max(interval, Retry-After) = 30s.
        assertEquals(listOf(5_000L, 35_000L), pollTimes)
        client.close()
    }

    @Test
    fun poll429MalformedRetryAfterFallsBackToInterval() = runTest {
        for (header in listOf(null, "abc", "1.5", "-1", "0", "99999999999999999999", "+30", "\uFF11\uFF12")) {
            val pollTimes = mutableListOf<Long>()
            var i = 0
            val start = testScheduler.currentTime
            val engine = MockEngine {
                pollTimes.add(testScheduler.currentTime - start)
                i += 1
                if (i == 1) {
                    respond(tooManyJson, HttpStatusCode.TooManyRequests, headers429(header))
                } else {
                    respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
                }
            }
            val client = HttpClient(engine)

            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)

            // Fallback: both waits are the plain 5s interval.
            assertEquals(listOf(5_000L, 10_000L), pollTimes, "header=$header")
            client.close()
        }
    }

    @Test
    fun poll429RetryAfterOverrideDecaysAfterOneWait() = runTest {
        val pollTimes = mutableListOf<Long>()
        val responses = listOf(
            Triple(HttpStatusCode.TooManyRequests, tooManyJson, "30"),
            Triple(HttpStatusCode.BadRequest, errorJson("authorization_pending"), null),
            Triple(HttpStatusCode.OK, tokenJson, null),
        )
        var i = 0
        val engine = MockEngine {
            pollTimes.add(testScheduler.currentTime)
            val (status, body, retryAfter) = responses[minOf(i, responses.size - 1)]
            i += 1
            respond(body, status, headers429(retryAfter))
        }
        val client = HttpClient(engine)

        pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)

        // 5s initial, 30s one-shot override, then back to the 5s interval —
        // cumulative poll times 5s, 35s, 40s.
        assertEquals(listOf(5_000L, 35_000L, 40_000L), pollTimes)
        client.close()
    }

    @Test
    fun poll429WrongPairStaysTerminal() = runTest {
        val cases = listOf(
            HttpStatusCode.TooManyRequests to errorJson("rate_limited"),
            HttpStatusCode.BadRequest to tooManyJson,
        )
        for ((status, body) in cases) {
            val engine = MockEngine { respond(body, status, headers429("30")) }
            val client = HttpClient(engine)

            val e = assertFailsWith<BasecampException.Api> {
                pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
            }
            assertEquals("api_error", e.code)
            client.close()
        }
    }

    @Test
    fun poll429WaitClampedToDeadline() = runTest {
        // interval 5s, code lifetime 20s. The 429's huge Retry-After would wait
        // 3600s, but the deadline at t=20s clamps the wait so expiry fires then.
        val pollTimes = mutableListOf<Long>()
        val engine = MockEngine {
            pollTimes.add(testScheduler.currentTime)
            respond(tooManyJson, HttpStatusCode.TooManyRequests, headers429("3600"))
        }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.DeviceFlow> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 20, testTimeSource, client)
        }
        assertEquals(BasecampException.DEVICE_EXPIRED, e.reason)
        // One poll at t=5s; the override wait clamps to the 15s remaining and
        // the loop expires at t=20s without a second poll.
        assertEquals(listOf(5_000L), pollTimes)
        client.close()
    }

    @Test
    fun pollPropagatesCancellationDuring429Wait() = runTest {
        // The 429 override parks the loop in a 30s delay; a 10s timeout cancels
        // mid-override-wait and the CancellationException must propagate.
        val engine = MockEngine { respond(tooManyJson, HttpStatusCode.TooManyRequests, headers429("30")) }
        val client = HttpClient(engine)

        assertFailsWith<TimeoutCancellationException> {
            withTimeout(10_000) {
                pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
            }
        }
        client.close()
    }

    @Test
    fun pollCancelledDuringTokenRoundTripNeverReturnsAToken() = runTest {
        // The mock engine cancels the flow's job as it serves the TOKEN,
        // modeling an engine that completes while ignoring cancellation — the
        // success branch must re-check and never hand back the credential.
        val job = Job()
        val engine = MockEngine {
            job.cancel()
            respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
        }
        val client = HttpClient(engine)
        try {
            assertFailsWith<CancellationException> {
                withContext(job) {
                    pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
                }
            }
        } finally {
            client.close()
        }
    }

    @Test
    fun pollCancelledDuringTokenRoundTripBeatsTerminalError() = runTest {
        // Cancellation must also win over a TERMINAL error completed after the
        // caller cancelled: an engine that ignores cancellation and serves
        // access_denied must surface the native CancellationException, not
        // the DeviceFlow(access_denied) classification.
        val job = Job()
        val engine = MockEngine {
            job.cancel()
            respond(errorJson("access_denied"), HttpStatusCode.BadRequest, jsonHeaders)
        }
        val client = HttpClient(engine)
        try {
            assertFailsWith<CancellationException> {
                withContext(job) {
                    pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
                }
            }
        } finally {
            client.close()
        }
    }

    @Test
    fun requestCancelledDuringRoundTripBeatsAPIError() = runTest {
        // Cancellation must also win over a completed NON-2XX on the
        // authorization request: an engine that cancels and then serves a 500
        // must propagate the native CancellationException, not Api.
        val job = Job()
        val engine = MockEngine {
            job.cancel()
            respond("{}", HttpStatusCode.InternalServerError, jsonHeaders)
        }
        val client = HttpClient(engine)
        try {
            assertFailsWith<CancellationException> {
                withContext(job) {
                    requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
                }
            }
        } finally {
            client.close()
        }
    }

    @Test
    fun requestCancelledDuringRoundTripNeverReturnsACode() = runTest {
        // Same seam on the authorization request: a code pair served by an
        // engine that ignores cancellation must not reach a direct caller.
        val job = Job()
        val engine = MockEngine {
            job.cancel()
            respond(deviceAuthJson, HttpStatusCode.OK, jsonHeaders)
        }
        val client = HttpClient(engine)
        try {
            assertFailsWith<CancellationException> {
                withContext(job) {
                    requestDeviceAuthorization(deviceEndpoint, "basecamp-cli", client = client)
                }
            }
        } finally {
            client.close()
        }
    }

    @Test
    fun pollAcceptsAWhitespaceOnlyAccessToken() = runTest {
        // Tokens are opaque: the cross-SDK contract requires only
        // NON-EMPTINESS — a whitespace token is the server's business.
        val body = """{"access_token":"  ","token_type":"Bearer"}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val token = pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        assertEquals("  ", token.accessToken)
        client.close()
    }

    @Test
    fun pollRejectsEmptyAccessToken() = runTest {
        // A 2xx whose access_token is blank must be an api_error, never an accepted
        // token and never a retryable transport error.
        val body = """{"access_token":"","token_type":"Bearer","expires_in":3600}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals("api_error", e.code)
        client.close()
    }

    @Test
    fun pollAbortsOversizedTokenBody() = runTest {
        val huge = "{\"access_token\":\"" + "x".repeat(1_100_000) + "\"}"
        val engine = MockEngine { respond(huge, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        client.close()
    }

    @Test
    fun pollDoesNotFollowRedirect() = runTest {
        var attackerContacted = false
        val engine = MockEngine { request ->
            if (request.url.host.contains("attacker")) {
                attackerContacted = true
                respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
            } else {
                respond(
                    content = ByteReadChannel(""),
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location, "https://attacker.example.com/oauth/token"),
                )
            }
        }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals("api_error", e.code)
        assertFalse(attackerContacted, "token poll must not follow the redirect")
        client.close()
    }

    @Test
    fun pollTreatsRedirectWithPendingBodyAsApiError() = runTest {
        // A suppressed 3xx is an API fault even when its body parrots a valid
        // OAuth poll state: {"error":"authorization_pending"} in a 302 must NOT
        // keep the loop polling toward an attacker-influenced Location.
        var polls = 0
        val engine = MockEngine {
            polls += 1
            respond(
                content = ByteReadChannel(errorJson("authorization_pending")),
                status = HttpStatusCode.Found,
                headers = headersOf(
                    HttpHeaders.Location to listOf("https://attacker.example.com/oauth/token"),
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                ),
            )
        }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals(302, e.httpStatus, "suppressed 3xx surfaces as api_error before body parsing")
        assertEquals(1, polls, "the loop must stop at the first redirect, not keep polling")
        assertTrue(
            e.message?.contains("redirect", ignoreCase = true) == true,
            "the error must name the redirect explicitly, not a generic http_302",
        )
        client.close()
    }

    @Test
    fun pollRejectsOutOfRangeCallerDurations() = runTest {
        // Caller-input sanity on the exported entry point: an oversized expiresIn
        // saturates Duration to infinite — a deadline that never passes, an
        // unbounded poll loop — and non-positive values are not schedulable.
        // Rejected as usage BEFORE any request (2_147_484 = MAX_DEVICE_SECONDS + 1).
        val engine = MockEngine { respond(tokenJson, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val cases = listOf(5L to 0L, 5L to -1L, 5L to 2_147_484L, 0L to 900L, -1L to 900L, 2_147_484L to 900L)
        for ((interval, expiresIn) in cases) {
            assertFailsWith<BasecampException.Usage>("interval=$interval expiresIn=$expiresIn") {
                pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", interval, expiresIn, testTimeSource, client)
            }
        }
        assertEquals(0, engine.requestHistory.size, "guard must reject before any request")
        client.close()
    }

    @Test
    fun pollRejectsADeadlineBeyondTheCodeLifetime() = runTest {
        // A caller-supplied deadline can only SHORTEN the validated lifetime:
        // one later than expiresIn from now would keep token requests running
        // after the server-issued code expired. Rejected as usage BEFORE any
        // request. The equality edge — exactly expiresIn from now, the default
        // and the issuance-anchored mark at zero elapsed — stays accepted.
        val engine = MockEngine { respond(tokenJson, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val clock = TestTimeSource()
        assertFailsWith<BasecampException.Usage> {
            pollDeviceToken(
                tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, clock, client,
                deadline = clock.markNow() + 901.seconds,
            )
        }
        assertEquals(0, engine.requestHistory.size, "guard must reject before any request")

        val token = pollDeviceToken(
            tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, clock, client,
            deadline = clock.markNow() + 900.seconds,
        )
        assertEquals("device_access_token", token.accessToken)
        client.close()
    }

    @Test
    fun pollNormalizesEmptyTokenErrorToHttpStatus() = runTest {
        // A 4xx token body of {"error":""} decodes cleanly (error is a required
        // non-null String, so no SerializationException fires), so it must be
        // normalized to http_<status> rather than surfacing a dangling, empty error
        // code — matching Go/TS/Python/Ruby, which all coerce a blank error the same way.
        val engine = MockEngine { respond(errorJson(""), HttpStatusCode.BadRequest, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals(400, e.httpStatus)
        assertTrue(
            e.message?.contains("http_400") == true,
            "an empty error code must normalize to http_400, not a blank message: ${e.message}",
        )
        client.close()
    }

    @Test
    fun pollAcceptsIntegerValuedFloatExpiresInOn2xx() = runTest {
        // 3600.0 carries no fractional part — accepted per the cross-SDK contract
        // (the shared Long? decode rejected it, unlike TS/Python/Ruby; the device
        // path now decodes Double? and enforces whole seconds in validation).
        val body = """{"access_token":"tok","token_type":"Bearer","expires_in":3600.0}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val token = pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        assertEquals(3600L, token.expiresIn)
        assertNotNull(token.expiresAt)
        client.close()
    }

    @Test
    fun pollRejectsNonNumericExpiresInOn2xx() = runTest {
        // A 2xx token response whose expires_in is not a number is a malformed
        // body: SerializationException → api_error, never a token nor transport.
        val body = """{"access_token":"tok","token_type":"Bearer","expires_in":"soon"}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals("api_error", e.code)
        client.close()
    }

    @Test
    fun pollDecodeFaultOmitsTheCauseAndBody() = runTest {
        // A malformed 2xx token body can carry the access token: the mapped
        // fault must not chain the SerializationException, whose message
        // embeds JSON input excerpts — a logged exception chain must not
        // disclose the token.
        val secret = "sk-live-SUPERSECRET"
        val body = """{"access_token":"$secret","token_type":7}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertNull(e.cause, "decoder cause must be dropped — its message embeds the body")
        assertFalse(e.message!!.contains(secret))
        client.close()
    }

    @Test
    fun pollRejectsMalformedTokenExpiresInOn2xx() = runTest {
        // A 2xx whose expires_in cannot be a schedulable lifetime is api_error:
        // 1e400 parses to Infinity (past the ceiling), an explicit 0 or negative
        // value violates the positive rule, a past-ceiling value would overflow
        // `it * 1000` in expiresAt, and 3600.5 breaks the whole-second contract.
        val bodies = listOf(
            """{"access_token":"tok","token_type":"Bearer","expires_in":1e400}""",
            """{"access_token":"tok","token_type":"Bearer","expires_in":-1}""",
            """{"access_token":"tok","token_type":"Bearer","expires_in":0}""",
            """{"access_token":"tok","token_type":"Bearer","expires_in":2147483648}""",
            """{"access_token":"tok","token_type":"Bearer","expires_in":3600.5}""",
        )
        for (body in bodies) {
            val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
            val client = HttpClient(engine)
            val e = assertFailsWith<BasecampException.Api>("expected api_error for $body") {
                pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
            }
            assertEquals("api_error", e.code)
            client.close()
        }
    }

    @Test
    fun pollRejectsExplicitEmptyTokenTypeOn2xx() = runTest {
        // An explicit "token_type": "" is malformed token metadata (api_error),
        // distinct from an absent field — uniform with Go/Python/Ruby/TS.
        val body = """{"access_token":"tok","token_type":"","expires_in":3600}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals("api_error", e.code)
        client.close()
    }

    @Test
    fun pollDefaultsAbsentTokenTypeToBearer() = runTest {
        // Absent token_type defaults to Bearer (RFC 6749 responses from
        // first-party servers always send it, but the default keeps the token
        // usable) — only an explicit empty string is rejected.
        val body = """{"access_token":"tok","expires_in":3600}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val token = pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)

        assertEquals("Bearer", token.tokenType)
        client.close()
    }

    @Test
    fun pollAcceptsMaxTokenLifetime() = runTest {
        // The 2147483647 s ceiling itself is valid — the bound is inclusive.
        val body = """{"access_token":"tok","token_type":"Bearer","expires_in":2147483647}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val token = pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)

        assertEquals(2_147_483_647L, token.expiresIn)
        assertNotNull(token.expiresAt)
        client.close()
    }

    @Test
    fun pollProtocolErrorsOnlyOn4xx() = runTest {
        // OAuth protocol states are recognized only on a 4xx: a nonstandard
        // 2xx or a 5xx carrying a crafted authorization_pending body must
        // terminate as api_error, never extend polling.
        for (status in listOf(HttpStatusCode.Created, HttpStatusCode.Accepted, HttpStatusCode.InternalServerError)) {
            var polls = 0
            val engine = MockEngine {
                polls += 1
                respond(errorJson("authorization_pending"), status, jsonHeaders)
            }
            val client = HttpClient(engine)

            val e = assertFailsWith<BasecampException.Api> {
                pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
            }
            assertEquals("api_error", e.code)
            assertEquals(status.value, e.httpStatus)
            assertEquals(1, polls)
            client.close()
        }
    }

    @Test
    fun pollTerminalStatusClassifiedWithoutDrainingBody() = runTest {
        // An oversized body on a terminal non-4xx would trip the size cap if
        // drained — the early status check surfaces the status api_error.
        for (status in listOf(HttpStatusCode.Created, HttpStatusCode.InternalServerError)) {
            val engine = MockEngine { respond("x".repeat(2 * 1024 * 1024), status, jsonHeaders) }
            val client = HttpClient(engine)

            val e = assertFailsWith<BasecampException.Api> {
                pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
            }
            assertEquals("api_error", e.code)
            assertEquals(status.value, e.httpStatus)
            assertTrue(e.message!!.contains("http_${status.value}"), "want a status error, got ${e.message}")
            client.close()
        }
    }

    @Test
    fun pollNon200SuccessIsTerminal() = runTest {
        // RFC 8628/6749 token responses are exactly 200 (SPEC §16): a
        // nonstandard 201/202 carrying an access_token must not complete polling.
        for (status in listOf(HttpStatusCode.Created, HttpStatusCode.Accepted)) {
            val engine = MockEngine { respond(tokenJson, status, jsonHeaders) }
            val client = HttpClient(engine)

            val e = assertFailsWith<BasecampException.Api> {
                pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
            }
            assertEquals("api_error", e.code)
            assertEquals(status.value, e.httpStatus)
            client.close()
        }
    }

    @Test
    fun pollCapturesResourceAndTreatsNullAsAbsent() = runTest {
        // resource (RFC 8707) round-trips onto the token; JSON null is absent.
        val body = """{"access_token":"tok","token_type":"Bearer","resource":"urn:bc:account:42"}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val token = pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        assertEquals("urn:bc:account:42", token.resource)
        client.close()

        val nullBody = """{"access_token":"tok","token_type":"Bearer","resource":null}"""
        val nullEngine = MockEngine { respond(nullBody, HttpStatusCode.OK, jsonHeaders) }
        val nullClient = HttpClient(nullEngine)

        val nullToken = pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, nullClient)
        assertNull(nullToken.resource)
        nullClient.close()
    }

    @Test
    fun pollRejectsEmptyResourceOn2xx() = runTest {
        // A present-but-empty resource is malformed (SPEC §16) — an empty
        // binding is not a binding. Uniform with Go/Python/Ruby/TS.
        val body = """{"access_token":"tok","token_type":"Bearer","resource":""}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.Api> {
            pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)
        }
        assertEquals("api_error", e.code)
        client.close()
    }

    @Test
    fun pollAcceptsTokenWithoutExpiresIn() = runTest {
        // Absent expires_in (RFC 6749 §5.1) is allowed — the token has no expiry.
        val body = """{"access_token":"tok","token_type":"Bearer"}"""
        val engine = MockEngine { respond(body, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val token = pollDeviceToken(tokenEndpoint, "basecamp-cli", "dev-code-123", 5, 900, testTimeSource, client)

        assertNull(token.expiresIn)
        assertNull(token.expiresAt)
        client.close()
    }

    @Test
    fun pollClampsBackoffToDeadline() = runTest {
        // interval 5s, code lifetime 8s. The first poll (t=5s) times out, so the
        // backoff would double the wait to 10s (→ t=15s). The deadline at t=8s must
        // clamp that wait so expiry fires at t=8s instead of overshooting.
        val pollTimes = mutableListOf<Long>()
        val engine = MockEngine {
            pollTimes.add(testScheduler.currentTime)
            throw SimulatedConnectTimeoutException()
        }
        val client = HttpClient(engine)

        val e = assertFailsWith<BasecampException.DeviceFlow> {
            pollDeviceToken(
                tokenEndpoint = tokenEndpoint,
                clientId = "basecamp-cli",
                deviceCode = "dev-code-123",
                interval = 5,
                expiresIn = 8,
                timeSource = testTimeSource,
                client = client,
            )
        }

        assertEquals(BasecampException.DEVICE_EXPIRED, e.reason)
        assertEquals(listOf(5_000L), pollTimes, "only one poll before the clamped wait hits expiry")
        assertEquals(8_000L, testScheduler.currentTime, "clamped wait must not overshoot the deadline")
        client.close()
    }

    // =========================================================================
    // performDeviceLogin
    // =========================================================================

    @Test
    fun performDeviceLoginGuardsCapabilityWithoutPolling() = runTest {
        var polled = false
        val engine = MockEngine {
            polled = true
            respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
        }
        val client = HttpClient(engine)

        // Endpoint present, but the device_code grant is NOT advertised.
        val config = OAuthConfig(
            issuer = origin,
            tokenEndpoint = tokenEndpoint,
            deviceAuthorizationEndpoint = deviceEndpoint,
            grantTypesSupported = listOf("refresh_token"),
        )

        val e = assertFailsWith<BasecampException.DeviceFlow> {
            performDeviceLogin(config, "basecamp-cli", display = {}, timeSource = testTimeSource, client = client)
        }
        assertEquals(BasecampException.DEVICE_UNAVAILABLE, e.reason)
        assertEquals("validation", e.code)
        assertFalse(polled, "capability guard must fail before any network call")
        client.close()
    }

    @Test
    fun performDeviceLoginFiresDisplayThenCompletes() = runTest {
        val engine = MockEngine { request ->
            if (request.url.encodedPath == "/oauth/device") {
                respond(deviceAuthJson, HttpStatusCode.OK, jsonHeaders)
            } else {
                respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
            }
        }
        val client = HttpClient(engine)

        val config = OAuthConfig(
            issuer = origin,
            tokenEndpoint = tokenEndpoint,
            deviceAuthorizationEndpoint = deviceEndpoint,
            grantTypesSupported = listOf(DEVICE_CODE_GRANT_TYPE, "refresh_token"),
        )
        var displayed: DeviceAuthorization? = null

        val token = performDeviceLogin(
            config = config,
            clientId = "basecamp-cli",
            display = { displayed = it },
            timeSource = testTimeSource,
            client = client,
        )

        assertNotNull(displayed)
        assertEquals("WDJB-MJHT", displayed.userCode)
        assertEquals("device_access_token", token.accessToken)
        client.close()
    }

    @Test
    fun performAnchorsExpiryAtResponseReceipt() = runTest {
        // SPEC §16: the deadline is markNow() + expiresIn taken AFTER
        // requestDeviceAuthorization returns — a 6s request leg with
        // expires_in 5 must NOT expire the fresh code client-side; expiry
        // past receipt is arbitrated by the server (expired_token). The
        // MockEngine handler advances the manual clock to model the slow
        // round-trip.
        val clock = TestTimeSource()
        val slowAuthJson = deviceAuthJson.replace("\"expires_in\": 900", "\"expires_in\": 5")
        val engine = MockEngine { request ->
            if (request.url.encodedPath == "/oauth/device") {
                clock += 6.seconds
                respond(slowAuthJson, HttpStatusCode.OK, jsonHeaders)
            } else {
                respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
            }
        }
        val client = HttpClient(engine)

        val config = OAuthConfig(
            issuer = origin,
            tokenEndpoint = tokenEndpoint,
            deviceAuthorizationEndpoint = deviceEndpoint,
            grantTypesSupported = listOf(DEVICE_CODE_GRANT_TYPE, "refresh_token"),
        )

        val token = performDeviceLogin(
            config = config,
            clientId = "basecamp-cli",
            display = { },
            timeSource = clock,
            client = client,
        )

        assertEquals("device_access_token", token.accessToken)
        client.close()
    }

    @Test
    fun performDeviceLoginExpiresWhenDisplayConsumesLifetime() = runTest {
        var polled = false
        val engine = MockEngine { request ->
            if (request.url.encodedPath == "/oauth/device") {
                respond(deviceAuthJson, HttpStatusCode.OK, jsonHeaders)
            } else {
                polled = true
                respond(tokenJson, HttpStatusCode.OK, jsonHeaders)
            }
        }
        val client = HttpClient(engine)

        val config = OAuthConfig(
            issuer = origin,
            tokenEndpoint = tokenEndpoint,
            deviceAuthorizationEndpoint = deviceEndpoint,
            grantTypesSupported = listOf(DEVICE_CODE_GRANT_TYPE, "refresh_token"),
        )

        // A manual TestTimeSource lets the (non-suspend) display hook advance the
        // deadline clock synchronously. The code lives 900s (deviceAuthJson); the
        // hook burns the whole lifetime, so the deadline anchored at issuance —
        // before display — is already past when the hook returns. The flow must
        // fail `expired` WITHOUT ever polling the token endpoint.
        val clock = TestTimeSource()
        val e = assertFailsWith<BasecampException.DeviceFlow> {
            performDeviceLogin(
                config = config,
                clientId = "basecamp-cli",
                display = { clock += 900.seconds },
                timeSource = clock,
                client = client,
            )
        }

        assertEquals(BasecampException.DEVICE_EXPIRED, e.reason)
        assertFalse(polled, "a code that expired during display must not be polled")
        client.close()
    }

    @Test
    fun performCancelDuringDisplayBeatsExpiry() = runTest {
        // A display hook that both cancels the flow and consumes the whole
        // code lifetime (a prompt closing in response to cancellation) must
        // propagate the native CancellationException, not DeviceFlow(expired).
        val engine = MockEngine { respond(deviceAuthJson, HttpStatusCode.OK, jsonHeaders) }
        val client = HttpClient(engine)

        val config = OAuthConfig(
            issuer = origin,
            tokenEndpoint = tokenEndpoint,
            deviceAuthorizationEndpoint = deviceEndpoint,
            grantTypesSupported = listOf(DEVICE_CODE_GRANT_TYPE, "refresh_token"),
        )

        val clock = TestTimeSource()
        val job = Job()
        try {
            assertFailsWith<CancellationException> {
                withContext(job) {
                    performDeviceLogin(
                        config = config,
                        clientId = "basecamp-cli",
                        display = {
                            job.cancel()
                            clock += 900.seconds
                        },
                        timeSource = clock,
                        client = client,
                    )
                }
            }
        } finally {
            client.close()
        }
    }
}
