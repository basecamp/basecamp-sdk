package com.basecamp.sdk

import io.ktor.client.engine.mock.*
import io.ktor.http.*
import io.ktor.utils.io.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotNull

class ClientTest {

    private fun mockClient(
        handler: MockRequestHandler,
    ): BasecampClient {
        val engine = MockEngine(handler)
        return BasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
            enableRetry = false
        }
    }

    @Test
    fun forAccountCreatesAccountClient() = runTest {
        val client = mockClient { respondOk("[]") }
        val account = client.forAccount("12345")
        assertEquals("12345", account.accountId)
        client.close()
    }

    @Test
    fun forAccountRejectsBlankId() {
        val client = mockClient { respondOk("[]") }
        assertFailsWith<IllegalArgumentException> {
            client.forAccount("")
        }
        client.close()
    }

    @Test
    fun forAccountRejectsNonNumericId() {
        val client = mockClient { respondOk("[]") }
        assertFailsWith<IllegalArgumentException> {
            client.forAccount("abc")
        }
        client.close()
    }

    @Test
    fun requestSendsAuthHeaders() = runTest {
        var capturedAuth: String? = null
        var capturedAccept: String? = null
        val client = mockClient { request ->
            capturedAuth = request.headers["Authorization"]
            capturedAccept = request.headers["Accept"]
            respondOk("[]")
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals("Bearer test-token", capturedAuth)
        assertEquals("application/json", capturedAccept)
        client.close()
    }

    @Test
    fun request401ReturnsUnauthorizedStatus() = runTest {
        val client = mockClient {
            respond(
                content = """{"error": "Unauthorized"}""",
                status = HttpStatusCode.Unauthorized,
                headers = headersOf(HttpHeaders.ContentType, "application/json"),
            )
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"

        val response = account.httpClient.requestWithRetry(HttpMethod.Get, url)
        assertEquals(401, response.status.value)
        client.close()
    }

    @Test
    fun builderRequiresAccessToken() {
        assertFailsWith<IllegalArgumentException> {
            BasecampClient {
                baseUrl = "http://localhost:3000"
            }
        }
    }

    @Test
    fun builderRejectsNonHttpsUrl() {
        assertFailsWith<IllegalArgumentException> {
            BasecampClient {
                accessToken("token")
                baseUrl = "http://not-localhost.com"
            }
        }
    }

    @Test
    fun builderAllowsLocalhost() {
        val client = BasecampClient {
            accessToken("token")
            baseUrl = "http://localhost:3000"
        }
        assertEquals("http://localhost:3000", client.config.baseUrl)
        client.close()
    }

    @Test
    fun serviceExtensibilityViaCachePattern() = runTest {
        val client = mockClient { respondOk("{}") }
        val account = client.forAccount("12345")

        // Simulate an extension service using the service cache
        val service1 = account.service("test") { "TestService" }
        val service2 = account.service("test") { "Should not be created" }

        assertEquals("TestService", service1)
        assertEquals("TestService", service2) // Same instance
        client.close()
    }

    @Test
    fun hooksAreCalledOnRequest() = runTest {
        var requestStartCalled = false
        var requestEndCalled = false

        val hooks = object : BasecampHooks {
            override fun onRequestStart(info: RequestInfo) {
                requestStartCalled = true
                assertEquals("GET", info.method)
                assertEquals(1, info.attempt)
            }

            override fun onRequestEnd(info: RequestInfo, result: RequestResult) {
                requestEndCalled = true
                assertEquals(200, result.statusCode)
            }
        }

        val engine = MockEngine { respondOk("[]") }
        val client = BasecampClient {
            accessToken("test-token")
            baseUrl = "http://localhost:3000"
            this.engine = engine
            this.hooks = hooks
        }

        val account = client.forAccount("12345")
        val url = "${client.config.baseUrl}/12345/projects.json"
        account.httpClient.requestWithRetry(HttpMethod.Get, url)

        assertEquals(true, requestStartCalled)
        assertEquals(true, requestEndCalled)
        client.close()
    }
}
