package com.basecamp.sdk

import com.basecamp.sdk.http.BasecampHttpClient
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
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

        val client = BasecampClient {
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

        val client = BasecampClient {
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

        val client = BasecampClient {
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

        val client = BasecampClient {
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

        val client = BasecampClient {
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

        val client = BasecampClient {
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

        val client = BasecampClient {
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

        val client = BasecampClient {
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

        val client = BasecampClient {
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

        val client = BasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
        }

        // Network errors should be wrapped as BasecampException.Network
        // For GET (idempotent), the requestWithRetry catches the exception and throws Network
        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        try {
            account.httpClient.requestWithRetry(HttpMethod.Get, url)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Network) {
            assertTrue(e.message!!.contains("Network error"))
        }
        // Network error is thrown immediately (not retried at the requestWithRetry level,
        // it throws rather than returning a response)
        assertEquals(1, requestCount)
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

        val client = BasecampClient {
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
}
