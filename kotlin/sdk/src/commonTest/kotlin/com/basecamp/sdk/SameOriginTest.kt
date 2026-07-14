package com.basecamp.sdk

import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNull
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

    // Hosts the token may legitimately reach: the configured base origin plus
    // the localhost carve-out.
    private fun tokenMayReach(host: String): Boolean {
        val h = host.lowercase().removePrefix("[").removeSuffix("]")
        return h == "3.basecampapi.com" || h == "localhost" || h == "127.0.0.1" ||
            h == "::1" || h.endsWith(".localhost")
    }

    /**
     * End-to-end parser-differential regression: every adversarial URL, driven
     * through the real token-attach path, must either be rejected by the guard
     * or egress only to a host the token may reach — NEVER to a foreign host
     * carrying Authorization. The backslash vector caught a live leak: the old
     * hand-rolled guard read the host of `http://evil.example\.localhost/x` as
     * `evil.example\.localhost` (passes the .localhost carve-out) while Ktor
     * treats `\` as a path separator and dials `evil.example`.
     */
    @Test fun adversarialUrlsNeverEgressTokenToForeignHost() = runTest {
        val corpus = listOf(
            """http://evil.example\.localhost/x""",
            "http://localhost@evil.example/x",
            "http://evil.example#foo.localhost",
            "http://evil.example?x=.localhost",
            "http://localhost:80@evil.example/x",
            "https://3.basecampapi.com:443@evil.example/x",
            "http://[::1]/x",
            "HTTPS://localhost/x",
            "https://3.basecampapi.com:443/x",
            "http://localhost.evil.example/x",
        )
        val egress = mutableListOf<Pair<String, String?>>() // host -> Authorization
        val client = clientWith(MockEngine { req ->
            egress += req.url.host to req.headers["Authorization"]
            respondOk("[]")
        })
        val account = client.forAccount("12345")
        for (url in corpus) {
            try {
                account.httpClient.requestWithRetry(HttpMethod.Get, url)
            } catch (_: BasecampException) {
                // Rejection before egress is a passing outcome.
            }
        }
        for ((host, auth) in egress) {
            assertTrue(
                tokenMayReach(host) || auth == null,
                "Bearer token egressed to foreign host $host",
            )
        }
        client.close()
    }

    /**
     * Pins Ktor's cross-origin redirect behavior: the client follows redirects
     * by default, and Ktor strips Authorization when the redirect leaves the
     * origin. If a Ktor upgrade ever regresses this, the token would silently
     * leak to the Location target — this test turns that into a CI failure.
     */
    @Test fun crossOriginRedirectDropsAuthorization() = runTest {
        val egress = mutableListOf<Pair<String, String?>>() // host -> Authorization
        val client = clientWith(MockEngine { req ->
            egress += req.url.host to req.headers["Authorization"]
            if (req.url.host == "3.basecampapi.com") {
                respond(
                    content = "",
                    status = HttpStatusCode.Found,
                    headers = headersOf(HttpHeaders.Location, "https://evil.example/stolen"),
                )
            } else {
                respondOk("[]")
            }
        })
        val account = client.forAccount("12345")
        account.httpClient.requestWithRetry(HttpMethod.Get, "https://3.basecampapi.com/12345/projects.json")
        assertEquals(2, egress.size)
        assertEquals("Bearer secret-token", egress[0].second)
        assertEquals("evil.example", egress[1].first)
        assertNull(egress[1].second, "Authorization must be stripped on a cross-origin redirect")
        client.close()
    }
}
