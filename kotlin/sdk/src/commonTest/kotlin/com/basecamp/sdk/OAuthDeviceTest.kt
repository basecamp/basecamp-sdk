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
import kotlinx.coroutines.TimeoutCancellationException
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

        assertTrue(sentBody.contains("scope=read"), "scope must be sent when set")
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
}
