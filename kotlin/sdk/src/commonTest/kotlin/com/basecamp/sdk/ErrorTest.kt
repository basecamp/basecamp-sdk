package com.basecamp.sdk

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

class ErrorTest {

    @Test
    fun authErrorHasCorrectExitCode() {
        val e = BasecampException.Auth()
        assertEquals(3, e.exitCode)
        assertEquals("auth_required", e.code)
        assertEquals(401, e.httpStatus)
        assertFalse(e.retryable)
    }

    @Test
    fun forbiddenErrorHasCorrectExitCode() {
        val e = BasecampException.Forbidden()
        assertEquals(4, e.exitCode)
        assertEquals("forbidden", e.code)
        assertEquals(403, e.httpStatus)
    }

    @Test
    fun notFoundErrorHasCorrectExitCode() {
        val e = BasecampException.NotFound("Todo 123 not found")
        assertEquals(2, e.exitCode)
        assertEquals("not_found", e.code)
        assertEquals(404, e.httpStatus)
    }

    @Test
    fun rateLimitErrorIsRetryable() {
        val e = BasecampException.RateLimit(retryAfterSeconds = 30)
        assertEquals(5, e.exitCode)
        assertTrue(e.retryable)
        assertEquals(429, e.httpStatus)
        assertEquals(30, e.retryAfterSeconds)
    }

    @Test
    fun networkErrorIsRetryable() {
        val cause = RuntimeException("Connection refused")
        val e = BasecampException.Network(cause = cause)
        assertEquals(6, e.exitCode)
        assertTrue(e.retryable)
        assertNull(e.httpStatus)
        assertEquals(cause, e.cause)
    }

    @Test
    fun apiErrorRetryableFor5xx() {
        val e500 = BasecampException.Api("Server error", 500)
        assertTrue(e500.retryable)
        assertEquals(7, e500.exitCode)

        val e503 = BasecampException.Api("Service unavailable", 503)
        assertTrue(e503.retryable)

        val e400 = BasecampException.Api("Bad request", 400, retryable = false)
        assertFalse(e400.retryable)
    }

    @Test
    fun validationErrorExitCode() {
        val e = BasecampException.Validation("Name is required")
        assertEquals(9, e.exitCode)
        assertEquals("validation", e.code)
    }

    @Test
    fun ambiguousErrorExitCode() {
        val e = BasecampException.Ambiguous("project", listOf("Project A", "Project B"))
        assertEquals(8, e.exitCode)
        assertEquals("ambiguous", e.code)
        assertEquals("Ambiguous project", e.message)
        assertEquals("Did you mean: Project A, Project B", e.hint)
    }

    @Test
    fun usageErrorExitCode() {
        val e = BasecampException.Usage("Invalid argument")
        assertEquals(1, e.exitCode)
        assertEquals("usage", e.code)
    }

    @Test
    fun fromHttpStatusMaps401ToAuth() {
        val e = BasecampException.fromHttpStatus(401, "Unauthorized")
        assertIs<BasecampException.Auth>(e)
    }

    @Test
    fun fromHttpStatusMaps403ToForbidden() {
        val e = BasecampException.fromHttpStatus(403, "Forbidden")
        assertIs<BasecampException.Forbidden>(e)
    }

    @Test
    fun fromHttpStatusMaps404ToNotFound() {
        val e = BasecampException.fromHttpStatus(404, "Not found")
        assertIs<BasecampException.NotFound>(e)
    }

    @Test
    fun fromHttpStatusMaps429ToRateLimit() {
        val e = BasecampException.fromHttpStatus(429, "Too many requests", retryAfterSeconds = 10)
        assertIs<BasecampException.RateLimit>(e)
        assertEquals(10, e.retryAfterSeconds)
    }

    @Test
    fun fromHttpStatusMaps422ToValidation() {
        val e = BasecampException.fromHttpStatus(422, "Invalid data")
        assertIs<BasecampException.Validation>(e)
    }

    @Test
    fun fromHttpStatusMaps500ToApi() {
        val e = BasecampException.fromHttpStatus(500, "Internal Server Error")
        assertIs<BasecampException.Api>(e)
        assertTrue(e.retryable)
    }

    @Test
    fun truncateMessageTruncatesLongMessages() {
        val longMessage = "x".repeat(1000)
        val truncated = BasecampException.truncateMessage(longMessage)
        assertEquals(500, truncated.length)
        assertTrue(truncated.endsWith("..."))
    }

    @Test
    fun truncateMessagePreservesShortMessages() {
        val short = "Short message"
        assertEquals(short, BasecampException.truncateMessage(short))
    }

    @Test
    fun exhaustiveWhenMatching() {
        val errors: List<BasecampException> = listOf(
            BasecampException.Auth(),
            BasecampException.Forbidden(),
            BasecampException.NotFound(),
            BasecampException.RateLimit(),
            BasecampException.Network(),
            BasecampException.Api("error", 500),
            BasecampException.Ambiguous("project"),
            BasecampException.Validation("invalid"),
            BasecampException.Usage("bad arg"),
        )

        for (e in errors) {
            // This ensures all branches compile (exhaustive when)
            val code = when (e) {
                is BasecampException.Auth -> "auth"
                is BasecampException.Forbidden -> "forbidden"
                is BasecampException.NotFound -> "not_found"
                is BasecampException.RateLimit -> "rate_limit"
                is BasecampException.Network -> "network"
                is BasecampException.Api -> "api"
                is BasecampException.Ambiguous -> "ambiguous"
                is BasecampException.Validation -> "validation"
                is BasecampException.Usage -> "usage"
            }
            assertTrue(code.isNotEmpty())
        }
    }
}
