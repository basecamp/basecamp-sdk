package com.basecamp.sdk

import io.ktor.client.*
import io.ktor.client.engine.mock.*
import io.ktor.client.plugins.*
import io.ktor.client.request.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class AuthStrategyTest {

    @Test
    fun bearerAuthSetsHeader() = runTest {
        var capturedAuth: String? = null
        val engine = MockEngine { request ->
            capturedAuth = request.headers["Authorization"]
            respondOk("[]")
        }

        val client = BasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
            enableRetry = false
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals("Bearer test-token", capturedAuth)
        client.close()
    }

    @Test
    fun customAuthStrategySetsCustomHeader() = runTest {
        var capturedCookie: String? = null
        var capturedAuth: String? = null
        val engine = MockEngine { request ->
            capturedCookie = request.headers["Cookie"]
            capturedAuth = request.headers["Authorization"]
            respondOk("[]")
        }

        val client = BasecampClient {
            auth(AuthStrategy { request ->
                request.header("Cookie", "session=abc123")
            })
            baseUrl = "http://localhost:3000"
            this.engine = engine
            enableRetry = false
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals("session=abc123", capturedCookie)
        assertEquals(null, capturedAuth)
        client.close()
    }

    @Test
    fun builderRejectsBothAccessTokenAndAuth() {
        assertFailsWith<IllegalArgumentException> {
            BasecampClient {
                accessToken("token")
                auth(BearerAuth(StaticTokenProvider("other-token")))
                baseUrl = "http://localhost:3000"
            }
        }
    }

    @Test
    fun builderRequiresAccessTokenOrAuth() {
        assertFailsWith<IllegalArgumentException> {
            BasecampClient {
                baseUrl = "http://localhost:3000"
            }
        }
    }

    @Test
    fun externalHttpClientIsUsed() = runTest {
        var requestReceived = false
        val engine = MockEngine {
            requestReceived = true
            respondOk("[]")
        }

        val externalClient = HttpClient(engine)

        val client = BasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            httpClient = externalClient
            enableRetry = false
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(true, requestReceived)
        client.close()
    }

    @Test
    fun externalClientWithExpectSuccessReturnsResponse() = runTest {
        val engine = MockEngine {
            respondError(HttpStatusCode.NotFound, "not found")
        }

        val externalClient = HttpClient(engine) {
            expectSuccess = true
        }

        val client = BasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            httpClient = externalClient
            enableRetry = false
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects/999.json"
        // Should return the 404 response instead of throwing ResponseException
        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)
        assertEquals(404, response.status.value)
        client.close()
    }

    @Test
    fun callerOwnedClientSurvivesClose() = runTest {
        var requestCount = 0
        val engine = MockEngine {
            requestCount++
            respondOk("[]")
        }

        val externalClient = HttpClient(engine)

        val client = BasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            httpClient = externalClient
            enableRetry = false
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        account.httpClient.requestWithRetry(HttpMethod.Get, url)
        assertEquals(1, requestCount)

        // Close the SDK client — should NOT close the external client
        client.close()

        // External client should still be usable
        externalClient.get(url)
        assertTrue(requestCount > 1)
        externalClient.close()
    }
}
