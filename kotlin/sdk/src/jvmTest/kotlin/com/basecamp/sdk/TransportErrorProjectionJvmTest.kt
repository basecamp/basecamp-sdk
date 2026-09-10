package com.basecamp.sdk

import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.http.*
import kotlinx.coroutines.runBlocking
import java.net.InetAddress
import java.net.ServerSocket
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertTrue
import kotlin.time.Duration.Companion.milliseconds

/**
 * The shipped engine's own timeout, not a mock's: a listener that accepts and
 * never answers drives the HttpTimeout plugin to throw the exception whose
 * message renders the request URL. On `main` this fails with the signed query
 * in the SDK error's message.
 */
class TransportErrorProjectionJvmTest {
    private val secret = "SECRETVALUE"

    private class EndSpy : BasecampHooks {
        val errors = mutableListOf<Throwable?>()
        override fun onRequestEnd(info: RequestInfo, result: RequestResult) { errors += result.error }
    }

    private fun renderings(e: Throwable?, label: String = "error", depth: Int = 0): List<Pair<String, String>> {
        if (e == null || depth > 8) return emptyList()
        val own = listOf(
            "$label.message" to (e.message ?: ""),
            "$label.toString" to e.toString(),
            "$label.stackTrace" to e.stackTraceToString(),
            "$label.hint" to ((e as? BasecampException)?.hint ?: ""),
        )
        return own + renderings(e.cause, "$label.cause", depth + 1)
    }

    @Test
    fun requestTimeoutThroughTheShippedEngineRendersNoSignedQuery() = runBlocking {
        ServerSocket(0, 1, InetAddress.getByName("127.0.0.1")).use { silent ->
            val base = "http://127.0.0.1:${silent.localPort}"
            val spy = EndSpy()
            val client = BasecampClient {
                accessToken("test-token")
                baseUrl = base
                timeout = 300.milliseconds
                enableRetry = false
                hooks = spy
            }

            val e = assertFailsWith<BasecampException.Network> {
                client.forAccount("12345").httpClient.requestWithRetry(HttpMethod.Get, "$base/12345/blob?signature=$secret")
            }
            client.close()

            assertIs<HttpRequestTimeoutException>(e.cause)
            assertEquals("Network error: Request timeout has expired [url=$base/12345/blob, request_timeout=300 ms]", e.message)
            for ((label, text) in renderings(e) + renderings(spy.errors.single(), "RequestResult.error")) {
                assertFalse(text.contains(secret), "the signed query leaked into $label: $text")
            }
            assertTrue(spy.errors.single() is HttpRequestTimeoutException)
        }
    }
}
