package com.basecamp.sdk

import com.basecamp.sdk.generated.myAssignments
import com.basecamp.sdk.generated.projects
import com.basecamp.sdk.generated.todos
import com.basecamp.sdk.http.BasecampHttpClient
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

@OptIn(ExperimentalCoroutinesApi::class)

class RetryTest {

    @Test
    fun backoffDelayCalculation() {
        // Exponential backoff: base * 2^(attempt-1)
        // With 1000ms base:
        //   attempt 1: 1000 * 1 = 1000 + jitter(0-100)
        //   attempt 2: 1000 * 2 = 2000 + jitter(0-100)
        //   attempt 3: 1000 * 4 = 4000 + jitter(0-100)
        val base = 1000L

        val delay1 = BasecampHttpClient.calculateBackoffDelay(base, 1)
        assert(delay1 in 1000..1100) { "Expected ~1000, got $delay1" }

        val delay2 = BasecampHttpClient.calculateBackoffDelay(base, 2)
        assert(delay2 in 2000..2100) { "Expected ~2000, got $delay2" }

        val delay3 = BasecampHttpClient.calculateBackoffDelay(base, 3)
        assert(delay3 in 4000..4100) { "Expected ~4000, got $delay3" }
    }

    /**
     * #577: the backoff term saturates at [BasecampHttpClient.MAX_BACKOFF_DELAY_MS]
     * instead of overflowing.
     *
     * Before the fix `baseDelayMs * (1L shl (attempt - 1))` wrapped: `1L shl 63`
     * sets the sign bit, so with the 1000ms default the product went NEGATIVE at
     * attempt 54, and `delay()` returns immediately for a non-positive argument.
     * The client stopped backing off entirely and hammered a server that was
     * already answering 429/503. `1L shl 64` wraps the shift itself back to 1,
     * so past that the delay collapsed to the base value.
     *
     * The builder validates `maxRetries >= 0` with no upper bound, so a caller
     * setting a large cap reaches these attempt numbers on a long failure streak.
     *
     * The bound asserted is two-sided, and that matters: a one-sided "never too
     * long" check would pass on the two worst attempts. `1L shl 63` is
     * `Long.MIN_VALUE`, and `1000 * Long.MIN_VALUE` wraps to exactly 0 — a 0ms
     * backoff that looks perfectly in range while being the tight loop itself.
     * Once the exponential term is past the ceiling the delay must SIT at the
     * ceiling, so 0ms and a collapsed 1000ms both fail.
     */
    @Test
    fun backoffDelaySaturatesInsteadOfOverflowing() {
        val base = 1000L
        val ceiling = BasecampHttpClient.MAX_BACKOFF_DELAY_MS + 100L

        // With base 1000, attempt 6 is the first whose unclamped term (32,000ms)
        // exceeds the 30,000ms ceiling, so every attempt from here on must land
        // in [MAX_BACKOFF_DELAY_MS, MAX_BACKOFF_DELAY_MS + MAX_JITTER_MS].
        // Collected rather than asserted one at a time so a failure names every
        // overflow shape at once.
        val violations = listOf(6, 10, 53, 54, 55, 62, 63, 64, 65, 128, Int.MAX_VALUE)
            .map { it to BasecampHttpClient.calculateBackoffDelay(base, it) }
            .filter { (_, delay) -> delay !in BasecampHttpClient.MAX_BACKOFF_DELAY_MS..ceiling }
        assert(violations.isEmpty()) {
            "expected every delay in ${BasecampHttpClient.MAX_BACKOFF_DELAY_MS}..$ceiling ms, got " +
                violations.joinToString { (attempt, delay) -> "attempt $attempt -> ${delay}ms" }
        }

        // The ceiling binds base_delay_ms itself (SPEC §7 requirement 3).
        val hugeBase = BasecampHttpClient.calculateBackoffDelay(600_000L, 1)
        assert(hugeBase in 0..ceiling) {
            "a base delay above the ceiling must clamp to it, got $hugeBase"
        }

        // A zero/negative base delay stays at zero rather than saturating.
        assert(BasecampHttpClient.calculateBackoffDelay(0L, 90) in 0..100L) {
            "a zero base delay must not produce a ceiling-length sleep"
        }
    }

    @Test
    fun retryOn429ForGet() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            if (requestCount == 1) {
                respond(
                    content = "",
                    status = HttpStatusCode.TooManyRequests,
                    headers = headersOf("Retry-After", "2"),
                )
            } else {
                respondOk("""{"id": 1}""")
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(200, response.status.value)
        assertEquals(2, requestCount)
        client.close()
    }

    @Test
    fun noRetryForPostOn429() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respond(
                content = "",
                status = HttpStatusCode.TooManyRequests,
                headers = headersOf("Retry-After", "1"),
            )
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Post, url, """{"name":"test"}""")

        // POST should not retry on 429
        assertEquals(429, response.status.value)
        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun retryOn503ForPut() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            if (requestCount == 1) {
                respond(
                    content = "",
                    status = HttpStatusCode.ServiceUnavailable,
                )
            } else {
                respond(
                    content = """{"id": 1, "name": "Updated"}""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects/1.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Put, url, """{"name": "test"}""")

        assertEquals(200, response.status.value)
        assertEquals(2, requestCount)
        client.close()
    }

    @Test
    fun retryOn503ForDelete() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            if (requestCount == 1) {
                respond(
                    content = "",
                    status = HttpStatusCode.ServiceUnavailable,
                )
            } else {
                respond(
                    content = "",
                    status = HttpStatusCode.NoContent,
                )
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects/1.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Delete, url)

        assertEquals(204, response.status.value)
        assertEquals(2, requestCount)
        client.close()
    }

    @Test
    fun noRetryForNonIdempotentOperationWithMetadata() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respond(
                content = "",
                status = HttpStatusCode.ServiceUnavailable,
            )
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        // CreateProject has metadata (idempotent=false, retryOn=[429,503])
        // Should NOT retry because idempotent=false
        val response = account.httpClient.requestWithRetry(
            HttpMethod.Post, url, """{"name":"test"}""",
            operationName = "CreateProject",
        )

        assertEquals(503, response.status.value)
        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun retryForIdempotentOperationWithMetadata() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            if (requestCount == 1) {
                respond(content = "", status = HttpStatusCode.ServiceUnavailable)
            } else {
                respond(
                    content = """{"id": 1, "name": "Updated"}""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects/1.json"
        // UpdateProject has metadata (idempotent=true, retryOn=[429,503])
        // Should retry because idempotent=true and 503 is in retryOn
        val response = account.httpClient.requestWithRetry(
            HttpMethod.Put, url, """{"name":"test"}""",
            operationName = "UpdateProject",
        )

        assertEquals(200, response.status.value)
        assertEquals(2, requestCount)
        client.close()
    }

    @Test
    fun maxRetriesRespected() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respond(
                content = "",
                status = HttpStatusCode.TooManyRequests,
                headers = headersOf("Retry-After", "1"),
            )
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)

        // Should stop after maxRetries (3)
        assertEquals(429, response.status.value)
        assertEquals(3, requestCount)
        client.close()
    }

    @Test
    fun callerCapWinsOverOperationMax() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respond(content = "", status = HttpStatusCode.ServiceUnavailable)
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            maxRetries = 1
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects/1.json"
        // GetProject declares max 3, but the caller capped attempts at 1.
        // The operation value is a ceiling, not a replacement: min(1, 3) = 1.
        val response = account.httpClient.requestWithRetry(
            HttpMethod.Get, url,
            operationName = "GetProject",
        )

        assertEquals(503, response.status.value)
        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun operationCeilingBoundsRaisedCap() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respond(content = "", status = HttpStatusCode.ServiceUnavailable)
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            maxRetries = 5
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/my/preferences.json"
        // UpdateMyPreferences declares max 2; a raised caller cap must not
        // push past the operation's ceiling: min(5, 2) = 2.
        val response = account.httpClient.requestWithRetry(
            HttpMethod.Put, url, """{"theme":"dark"}""",
            operationName = "UpdateMyPreferences",
        )

        assertEquals(503, response.status.value)
        assertEquals(2, requestCount)
        client.close()
    }

    @Test
    fun paginationFollowUpPagesKeepTheOperationCeiling() = runTest {
        var page2Requests = 0
        val engine = MockEngine { request ->
            if (request.url.parameters["page"] == "2") {
                page2Requests++
                respond(content = "", status = HttpStatusCode.ServiceUnavailable)
            } else {
                respond(
                    content = """[{
                        "id": 1, "status": "active", "name": "One",
                        "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
                        "url": "https://3.basecampapi.com/12345/projects/1.json",
                        "app_url": "https://3.basecamp.com/12345/projects/1",
                        "dock": []
                    }]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        HttpHeaders.Link to listOf("""<http://localhost:3000/12345/projects.json?page=2>; rel="next""""),
                    ),
                )
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            maxRetries = 5
            this.engine = engine
        }

        // ListProjects declares max 3. The ceiling must survive onto pagination
        // follow-up requests: a raised caller cap of 5 still clamps page 2 to 3
        // attempts, exactly like page 1.
        val account = client.forAccount("12345")
        try {
            account.projects.list()
        } catch (_: BasecampException) {
            // expected: page 2 exhausts its attempts on 503
        }

        assertEquals(3, page2Requests)
        client.close()
    }

    @Test
    fun zeroCapCoercesToOneAttempt() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respond(content = "", status = HttpStatusCode.ServiceUnavailable)
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            maxRetries = 0
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects/1.json"
        // maxRetries counts total attempts, so 0 makes no sense as written;
        // it coerces to a single attempt rather than short-circuiting to none.
        val response = account.httpClient.requestWithRetry(
            HttpMethod.Get, url,
            operationName = "GetProject",
        )

        assertEquals(503, response.status.value)
        assertEquals(1, requestCount)
        client.close()
    }

    /**
     * #661: the attempt floor is a property of the budget expression, not of
     * the loop's shape. The pre-fix expression — `min(max(1, cap), op_max ?:
     * cap)` — computed a budget of 0 for an ungoverned call (no operationName,
     * so no per-op retry block) at `maxRetries = 0`; only the loop's post-check
     * shape (the first request goes out before the budget is consulted) kept
     * observable behavior at one attempt. The old bug is therefore NOT
     * behaviorally red-provable, so this test pins the computed budget value
     * directly: `(0, null)` is the discriminating case — 0 under the old
     * expression, 1 under the fixed one, where an absent op_max means no
     * ceiling rather than a second copy of the cap.
     */
    @Test
    fun attemptBudgetFloorsAtOneWithoutOperationMetadata() {
        // Ungoverned at a zero cap: the floor must come from the expression.
        assertEquals(1, BasecampHttpClient.computeMaxAttempts(0, null))
        // Ungoverned otherwise: the cap passes through untouched.
        assertEquals(3, BasecampHttpClient.computeMaxAttempts(3, null))
        // Governed: the floor still applies before the ceiling clamps...
        assertEquals(1, BasecampHttpClient.computeMaxAttempts(0, 2))
        // ...a raised cap is still clamped to the operation's declared max...
        assertEquals(2, BasecampHttpClient.computeMaxAttempts(5, 2))
        // ...and a caller-lowered cap is still honored.
        assertEquals(1, BasecampHttpClient.computeMaxAttempts(1, 3))
    }

    /**
     * Companion to the expression test above: the observable invariant at a
     * zero cap on the ungoverned path (no operationName). This passes against
     * the pre-#661 expression too — the loop's post-check shape already sent
     * exactly one request on a zero budget — which is exactly why the red
     * proof for #661 pins the expression value rather than behavior. This
     * test guards the invariant against future loop-shape changes, where a
     * pre-check loop reading a zero budget would send nothing at all.
     */
    @Test
    fun zeroCapUngovernedStillSendsOneAttempt() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respond(content = "", status = HttpStatusCode.ServiceUnavailable)
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            maxRetries = 0
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects/1.json"
        // No operationName: the per-op cap lookup finds nothing, and the
        // budget must still floor at one attempt.
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(503, response.status.value)
        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun retryAfterHeaderBasedDelay() = runTest {
        var requestCount = 0
        val requestTimestamps = mutableListOf<Long>()
        val engine = MockEngine { _ ->
            requestCount++
            requestTimestamps.add(testScheduler.currentTime)
            if (requestCount == 1) {
                respond(
                    content = "",
                    status = HttpStatusCode.TooManyRequests,
                    headers = headersOf("Retry-After", "2"),
                )
            } else {
                respondOk("""{"id": 1}""")
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(200, response.status.value)
        assertEquals(2, requestCount)
        // Retry-After: 2 means 2000ms delay
        val elapsed = requestTimestamps[1] - requestTimestamps[0]
        assertTrue(elapsed >= 2000, "Expected delay >= 2000ms from Retry-After: 2, got $elapsed")
        client.close()
    }

    @Test
    fun enableRetryFalseDisablesRetry() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respond(
                content = "",
                status = HttpStatusCode.TooManyRequests,
                headers = headersOf("Retry-After", "1"),
            )
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            enableRetry = false
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(429, response.status.value)
        assertEquals(1, requestCount, "Should not retry when enableRetry=false")
        client.close()
    }

    @Test
    fun networkErrorTriggersRetryForIdempotentOps() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            if (requestCount == 1) {
                throw java.io.IOException("Connection refused")
            } else {
                respondOk("""{"id": 1}""")
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        // GET is naturally idempotent, so a transport-level failure is retried
        // through the same gate as an HTTP-status retry (SPEC §7 Gate 3).
        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(200, response.status.value)
        assertEquals(2, requestCount)
        client.close()
    }

    @Test
    fun networkErrorRetriesExhaustAttempts() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            throw java.io.IOException("Connection refused")
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        // A persistent transport failure consumes the same attempt budget as a
        // persistent 503 (default maxRetries = 3), then surfaces as Network.
        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        try {
            account.httpClient.requestWithRetry(HttpMethod.Get, url)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Network) {
            assertTrue(e.message!!.contains("Network error"))
        }
        assertEquals(3, requestCount)
        client.close()
    }

    @Test
    fun retriesIdempotentPostNetworkErrorWithFullBodyThroughGeneratedService() = runTest {
        var requestCount = 0
        val bodies = mutableListOf<String>()
        val engine = MockEngine { request ->
            requestCount++
            bodies.add((request.body as io.ktor.http.content.TextContent).text)
            if (requestCount == 1) {
                throw java.io.IOException("Connection reset by peer")
            } else {
                respond(content = "", status = HttpStatusCode.NoContent)
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }

        val account = client.forAccount("12345")
        // PrioritizeAssignment is a POST flagged idempotent in metadata that
        // carries a JSON body. A transport failure on attempt 1 must be retried
        // AND the retry must carry the complete body — the body is a String
        // re-set on every attempt, so replay is structural, and this pins it.
        account.myAssignments.prioritizeAssignment(
            com.basecamp.sdk.generated.services.PrioritizeAssignmentBody(id = 123),
        )

        assertEquals(2, requestCount)
        assertEquals(2, bodies.size)
        assertEquals("""{"id":123}""", bodies[0])
        assertEquals(bodies[0], bodies[1], "retry must replay the full request body")
        client.close()
    }

    @Test
    fun nonIdempotentPostNetworkErrorIsSingleAttempt() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            throw java.io.IOException("Connection refused")
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        // CreateProject is a non-idempotent POST: a network blip must surface
        // immediately with no re-send, exactly one attempt.
        try {
            account.httpClient.requestWithRetry(
                HttpMethod.Post, url, """{"name":"test"}""",
                operationName = "CreateProject",
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Network) {
            assertTrue(e.message!!.contains("Network error"))
        }
        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun requestTimeoutExceptionIsNotRetried() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            throw io.ktor.client.plugins.HttpRequestTimeoutException("http://localhost:3000", 1000L)
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        // The request-time budget (HttpTimeout requestTimeoutMillis) applies
        // per attempt: an attempt that consumed its entire budget is a
        // slowness shape a retry tends to repeat, and each retry would burn
        // another full budget — the deliberate carve-out from network-error
        // retry.
        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        try {
            account.httpClient.requestWithRetry(HttpMethod.Get, url)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Network) {
            assertTrue(e.message!!.contains("Network error"))
        }
        assertEquals(1, requestCount)
        client.close()
    }

    @Test
    fun enableRetryFalseDisablesNetworkRetry() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            throw java.io.IOException("Connection refused")
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            enableRetry = false
            this.engine = engine
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        try {
            account.httpClient.requestWithRetry(HttpMethod.Get, url)
            assertTrue(false, "Should have thrown")
        } catch (_: BasecampException.Network) {
            // expected
        }
        assertEquals(1, requestCount, "Should not retry network errors when enableRetry=false")
        client.close()
    }

    @Test
    fun authStrategyFailureIsNotRetried() = runTest {
        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            respondOk("""{"id": 1}""")
        }

        var authCalls = 0
        val failingAuth = AuthStrategy {
            authCalls++
            throw IllegalStateException("credential provider broke")
        }

        val client = testBasecampClient {
            baseUrl = "http://localhost:3000"
            this.engine = engine
            authStrategy = failingAuth
        }

        // An auth-phase failure is not a transport fault: it must surface on
        // the first attempt, raw, without entering the network retry path —
        // the strategy runs exactly once, the wire is never reached, and the
        // caller sees their own exception (matching Swift/TS/Go), not a
        // fabricated network error.
        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val thrown = assertFailsWith<IllegalStateException> {
            account.httpClient.requestWithRetry(HttpMethod.Get, url)
        }
        assertEquals("credential provider broke", thrown.message)
        assertEquals(1, authCalls, "auth strategy must not be re-driven by retries")
        assertEquals(0, requestCount, "the wire must not be reached when auth fails")
        client.close()
    }

    @Test
    fun onRetryPairForNetworkError() = runTest {
        val failedAttempts = mutableListOf<Int>()
        val upcomingAttempts = mutableListOf<Int>()
        val retryErrors = mutableListOf<Throwable>()

        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            if (requestCount == 1) {
                throw java.io.IOException("Connection refused")
            } else {
                respondOk("""{"id": 1}""")
            }
        }

        val hooks = object : BasecampHooks {
            override fun onRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
                failedAttempts.add(info.attempt)
                upcomingAttempts.add(attempt)
                retryErrors.add(error)
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
            this.hooks = hooks
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(200, response.status.value)
        // onRetry carries the (failed, upcoming) attempt pair: info names the
        // attempt that failed, the attempt argument names the one about to run.
        assertEquals(listOf(1), failedAttempts)
        assertEquals(listOf(2), upcomingAttempts)
        assertEquals(1, retryErrors.size)
        assertTrue(retryErrors[0] is BasecampException.Network, "onRetry error should be Network, got ${retryErrors[0]::class.simpleName}")
        assertTrue(retryErrors[0].cause is java.io.IOException, "Network error should carry the transport cause")
        client.close()
    }

    @Test
    fun onRetryHookFiresWithCorrectAttemptNumber() = runTest {
        val retryAttempts = mutableListOf<Int>()
        val retryDelays = mutableListOf<Long>()

        var requestCount = 0
        val engine = MockEngine { _ ->
            requestCount++
            if (requestCount <= 2) {
                respond(
                    content = "",
                    status = HttpStatusCode.ServiceUnavailable,
                )
            } else {
                respondOk("""{"id": 1}""")
            }
        }

        val hooks = object : BasecampHooks {
            override fun onRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
                retryAttempts.add(attempt)
                retryDelays.add(delayMs)
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
            this.hooks = hooks
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(200, response.status.value)
        assertEquals(3, requestCount)
        assertEquals(listOf(2, 3), retryAttempts, "onRetry should fire with attempt 2 and 3")
        assertEquals(2, retryDelays.size)
        assertTrue(retryDelays[0] > 0, "First retry delay should be positive")
        assertTrue(retryDelays[1] > retryDelays[0], "Second retry delay should be larger (exponential)")
        client.close()
    }

    @Test
    fun retriesIdempotentPostThroughGeneratedService() = runTest {
        var requestCount = 0
        val methods = mutableListOf<String>()
        val paths = mutableListOf<String>()
        val engine = MockEngine { request ->
            requestCount++
            methods.add(request.method.value)
            paths.add(request.url.encodedPath)
            if (requestCount == 1) {
                respond(content = "", status = HttpStatusCode.ServiceUnavailable)
            } else {
                respond(content = "", status = HttpStatusCode.NoContent)
            }
        }

        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }

        val account = client.forAccount("12345")
        // CompleteTodo is a POST flagged idempotent in metadata. The existing
        // retryForIdempotentOperationWithMetadata test uses PUT/UpdateProject,
        // which is retried via the method allowlist (method in IDEMPOTENT_METHODS)
        // — it does NOT exercise the POST metadata gate. Driving the generated
        // service (account.todos.complete) does, so this fails if CompleteTodo's
        // metadata idempotent flag is flipped off. Regression guard for #439 / #417.
        account.todos.complete(todoId = 100)

        assertEquals(2, requestCount)
        // Pin the generated CompleteTodo wire shape: both the initial attempt and
        // the retry must be POST to /…/todos/100/completion.json. Without this the
        // test could pass on a wrong route/method as long as something retried once.
        assertTrue(methods.all { it == "POST" }, "expected all POST, got $methods")
        assertTrue(
            paths.all { it.endsWith("/todos/100/completion.json") },
            "expected the CompleteTodo completion path, got $paths",
        )
        client.close()
    }
}
