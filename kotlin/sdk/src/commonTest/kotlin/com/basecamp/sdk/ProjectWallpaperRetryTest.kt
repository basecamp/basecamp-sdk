package com.basecamp.sdk

import com.basecamp.sdk.generated.account
import com.basecamp.sdk.generated.projects
import com.basecamp.sdk.generated.services.UpdateProjectWallpaperOptions
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.HttpMethod
import io.ktor.http.HttpStatusCode
import io.ktor.http.content.OutgoingContent
import io.ktor.http.headersOf
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class ProjectWallpaperRetryTest {
    @Test
    fun retriesDeclaredStatusesWithTheSameMultipartBodyAndQuery() = runTest {
        for (status in listOf(HttpStatusCode.TooManyRequests, HttpStatusCode.ServiceUnavailable)) {
            val bodies = mutableListOf<List<Byte>>()
            val delays = mutableListOf<Long>()
            val engine = MockEngine { request ->
                assertEquals("tile", request.url.parameters["mode"])
                bodies.add((request.body as OutgoingContent.ByteArrayContent).bytes().toList())
                if (bodies.size == 1) {
                    respond("{}", status, headersOf("Retry-After", "1"))
                } else {
                    respond("", HttpStatusCode.NoContent)
                }
            }
            val client = testBasecampClient {
                accessToken("test-token")
                this.engine = engine
                hooks = object : BasecampHooks {
                    override fun onRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
                        assertEquals(1, info.attempt)
                        assertEquals(2, attempt)
                        delays.add(delayMs)
                    }
                }
            }
            try {
                client.forAccount("12345").projects.updateProjectWallpaper(
                    42, "image bytes".encodeToByteArray(), "wallpaper.png", "image/png",
                    UpdateProjectWallpaperOptions(mode = "tile"),
                )
                assertEquals(2, bodies.size)
                assertEquals(bodies[0], bodies[1])
                assertTrue(bodies[0].toByteArray().decodeToString().contains("name=\"wallpaper\""))
                assertEquals(listOf(1000L), delays)
            } finally {
                client.close()
            }
        }
    }

    @Test
    fun capsAttemptsByBothCallerAndOperationMetadata() = runTest {
        for ((cap, expected) in listOf(0 to 1, 1 to 1, 10 to 3)) {
            var attempts = 0
            val client = testBasecampClient {
                accessToken("test-token")
                maxRetries = cap
                engine = MockEngine {
                    attempts++
                    respond("{}", HttpStatusCode.ServiceUnavailable, headersOf("Retry-After", "1"))
                }
            }
            try {
                val error = assertFailsWith<BasecampException.Api> {
                    client.forAccount("12345").projects.updateProjectWallpaper(
                        42, "data".encodeToByteArray(), "wallpaper.png", "image/png",
                    )
                }
                assertEquals(503, error.httpStatus)
                assertEquals(expected, attempts)
            } finally {
                client.close()
            }
        }
    }

    @Test
    fun nonIdempotentBinaryPostsStillMakeOneAttempt() = runTest {
        var attempts = 0
        val client = testBasecampClient {
            accessToken("test-token")
            engine = MockEngine {
                attempts++
                respond("{}", HttpStatusCode.ServiceUnavailable)
            }
        }
        try {
            val response = client.forAccount("12345").httpClient.requestBinaryWithRetry(
                HttpMethod.Post, "https://3.basecampapi.com/12345/attachments.json",
                "data".encodeToByteArray(), "application/octet-stream",
            )
            assertEquals(HttpStatusCode.ServiceUnavailable, response.status)
            assertEquals(1, attempts)
        } finally {
            client.close()
        }
    }

    @Test
    fun accountLogoUsesItsOwnDeclaredAttemptCap() = runTest {
        var attempts = 0
        val client = testBasecampClient {
            accessToken("test-token")
            maxRetries = 10
            engine = MockEngine { request ->
                attempts++
                assertEquals("/12345/account/logo.json", request.url.encodedPath)
                respond("{}", HttpStatusCode.ServiceUnavailable, headersOf("Retry-After", "1"))
            }
        }
        try {
            assertFailsWith<BasecampException.Api> {
                client.forAccount("12345").account.updateAccountLogo(
                    "data".encodeToByteArray(), "logo.png", "image/png",
                )
            }
            assertEquals(2, attempts)
        } finally {
            client.close()
        }
    }

    @Test
    fun retryOptOutMakesOneAttemptAndEmitsNoRetryHook() = runTest {
        var attempts = 0
        var retries = 0
        val client = testBasecampClient {
            accessToken("test-token")
            enableRetry = false
            engine = MockEngine {
                attempts++
                respond("{}", HttpStatusCode.ServiceUnavailable, headersOf("Retry-After", "1"))
            }
            hooks = object : BasecampHooks {
                override fun onRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
                    retries++
                }
            }
        }
        try {
            assertFailsWith<BasecampException.Api> {
                client.forAccount("12345").projects.updateProjectWallpaper(
                    42, "data".encodeToByteArray(), "wallpaper.png", "image/png",
                )
            }
            assertEquals(1, attempts)
            assertEquals(0, retries)
        } finally {
            client.close()
        }
    }
}
