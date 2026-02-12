package com.basecamp.sdk.http

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.Metadata
import io.ktor.client.*
import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.serialization.json.Json

/**
 * Wraps [HttpClient] with Basecamp-specific behavior: authentication,
 * hooks, retry, and error mapping.
 *
 * This is an internal implementation detail. SDK consumers interact with
 * [BasecampClient] and service classes, not this wrapper.
 */
internal class BasecampHttpClient(
    val httpClient: HttpClient,
    private val tokenProvider: TokenProvider,
    private val config: BasecampConfig,
    private val hooks: BasecampHooks,
    internal val json: Json,
) {
    /**
     * Executes an HTTP request with authentication, returning the raw [HttpResponse].
     *
     * Auth headers and User-Agent are injected automatically.
     */
    suspend fun request(
        method: HttpMethod,
        url: String,
        body: String? = null,
    ): HttpResponse {
        val token = tokenProvider.accessToken()
        return httpClient.request(url) {
            this.method = method
            header(HttpHeaders.Authorization, "Bearer $token")
            header(HttpHeaders.UserAgent, config.userAgent)
            header(HttpHeaders.Accept, "application/json")
            if (body != null) {
                header(HttpHeaders.ContentType, "application/json")
                setBody(body)
            }
        }
    }

    /**
     * Executes an HTTP request, applying retry logic for retryable errors.
     * Idempotent methods (GET, PUT, DELETE, HEAD) are retried on 429/503.
     * Non-idempotent methods (POST, PATCH) are not retried.
     */
    suspend fun requestWithRetry(
        method: HttpMethod,
        url: String,
        body: String? = null,
        attempt: Int = 1,
        operationName: String? = null,
    ): HttpResponse {
        val info = RequestInfo(method = method.value, url = url, attempt = attempt)
        hooks.safeOnRequestStart(info)

        val startTime = currentTimeMillis()
        val response: HttpResponse
        try {
            response = request(method, url, body)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            val duration = currentTimeMillis() - startTime
            hooks.safeOnRequestEnd(info, RequestResult(
                statusCode = 0,
                duration = duration.millisToDuration(),
                error = e,
            ))
            throw BasecampException.Network(
                message = "Network error: ${e.message}",
                cause = e,
            )
        }

        val duration = currentTimeMillis() - startTime
        hooks.safeOnRequestEnd(info, RequestResult(
            statusCode = response.status.value,
            duration = duration.millisToDuration(),
        ))

        val status = response.status.value

        // Determine retry eligibility from per-operation metadata when available,
        // falling back to HTTP-method-based heuristic otherwise.
        val opRetry = operationName?.let { Metadata.operations[it]?.retry }
        val shouldRetry = config.enableRetry && if (opRetry != null) {
            status in opRetry.retryOn
        } else {
            status in RETRYABLE_STATUS_CODES && method in IDEMPOTENT_METHODS
        }
        val maxAttempts = opRetry?.maxRetries ?: config.maxRetries
        val baseDelayMs = opRetry?.baseDelayMs ?: config.baseRetryDelay.inWholeMilliseconds

        if (shouldRetry && attempt < maxAttempts) {
            val retryAfter = parseRetryAfter(response.headers["Retry-After"])
            val delayMs = if (status == 429 && retryAfter != null) {
                retryAfter.toLong() * 1000
            } else {
                calculateBackoffDelay(baseDelayMs, attempt)
            }

            hooks.safeOnRetry(info, attempt + 1, BasecampException.Api(
                "HTTP $status", status
            ), delayMs)

            kotlinx.coroutines.delay(delayMs)
            return requestWithRetry(method, url, body, attempt + 1, operationName)
        }

        return response
    }

    /**
     * Executes an HTTP request with a binary body and explicit Content-Type.
     *
     * Auth headers and User-Agent are injected automatically.
     */
    suspend fun requestBinary(
        method: HttpMethod,
        url: String,
        data: ByteArray,
        contentType: String,
    ): HttpResponse {
        val token = tokenProvider.accessToken()
        return httpClient.request(url) {
            this.method = method
            header(HttpHeaders.Authorization, "Bearer $token")
            header(HttpHeaders.UserAgent, config.userAgent)
            header(HttpHeaders.Accept, "application/json")
            header(HttpHeaders.ContentType, contentType)
            setBody(data)
        }
    }

    /**
     * Executes a binary upload request with hooks but no retry (POST is not idempotent).
     */
    suspend fun requestBinaryWithRetry(
        method: HttpMethod,
        url: String,
        data: ByteArray,
        contentType: String,
        attempt: Int = 1,
    ): HttpResponse {
        val info = RequestInfo(method = method.value, url = url, attempt = attempt)
        hooks.safeOnRequestStart(info)

        val startTime = currentTimeMillis()
        val response: HttpResponse
        try {
            response = requestBinary(method, url, data, contentType)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            val duration = currentTimeMillis() - startTime
            hooks.safeOnRequestEnd(info, RequestResult(
                statusCode = 0,
                duration = duration.millisToDuration(),
                error = e,
            ))
            throw BasecampException.Network(
                message = "Network error: ${e.message}",
                cause = e,
            )
        }

        val duration = currentTimeMillis() - startTime
        hooks.safeOnRequestEnd(info, RequestResult(
            statusCode = response.status.value,
            duration = duration.millisToDuration(),
        ))

        // POST is not idempotent, so no retry for binary uploads
        return response
    }

    companion object {
        /** Status codes that trigger automatic retry. */
        val RETRYABLE_STATUS_CODES = setOf(429, 503)

        /** HTTP methods that are safe to retry (idempotent). */
        val IDEMPOTENT_METHODS = setOf(HttpMethod.Get, HttpMethod.Put, HttpMethod.Delete, HttpMethod.Head)

        private const val MAX_JITTER_MS = 100L

        /** Exponential backoff: base * 2^(attempt-1) + jitter. */
        internal fun calculateBackoffDelay(baseDelayMs: Long, attempt: Int): Long {
            val delay = baseDelayMs * (1L shl (attempt - 1))
            val jitter = (kotlin.random.Random.nextLong(MAX_JITTER_MS))
            return delay + jitter
        }
    }
}

/** Safely call onRequestStart, catching hook exceptions. */
private fun BasecampHooks.safeOnRequestStart(info: RequestInfo) {
    runCatching { onRequestStart(info) }
}

/** Safely call onRequestEnd, catching hook exceptions. */
private fun BasecampHooks.safeOnRequestEnd(info: RequestInfo, result: RequestResult) {
    runCatching { onRequestEnd(info, result) }
}

/** Safely call onRetry, catching hook exceptions. */
private fun BasecampHooks.safeOnRetry(info: RequestInfo, attempt: Int, error: Throwable, delayMs: Long) {
    runCatching { onRetry(info, attempt, error, delayMs) }
}

/** Platform-compatible current time in millis. */
internal expect fun currentTimeMillis(): Long

/** Convert millis to Duration. */
@Suppress("NOTHING_TO_INLINE")
internal inline fun Long.millisToDuration(): kotlin.time.Duration {
    val ms = this
    return with(kotlin.time.Duration) { ms.milliseconds }
}
