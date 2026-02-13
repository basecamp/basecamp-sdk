package com.basecamp.sdk

import com.basecamp.sdk.http.BasecampHttpClient
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals

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
                    headers = headersOf("Retry-After", "0"),
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
                headers = headersOf("Retry-After", "0"),
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
}
