package com.basecamp.sdk.http

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.Metadata
import io.ktor.client.*
import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.client.plugins.ResponseException
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
    private val authStrategy: AuthStrategy,
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
        requireSameOrigin(url)
        return try {
            httpClient.request(url) {
                this.method = method
                authStrategy.authenticate(this)
                header(HttpHeaders.UserAgent, config.userAgent)
                header(HttpHeaders.Accept, "application/json")
                if (body != null) {
                    header(HttpHeaders.ContentType, "application/json")
                    setBody(body)
                }
            }
        } catch (e: ResponseException) {
            // External HttpClient with expectSuccess=true throws on non-2xx.
            // Return the response so the SDK's error classification runs.
            e.response
        }
    }

    /**
     * Executes an HTTP request, applying retry logic for retryable errors.
     * Safe HTTP methods (GET, PUT, DELETE, HEAD) are always retried.
     * Non-safe methods (POST, PATCH) are retried only when per-operation
     * metadata marks them as idempotent.
     *
     * One eligibility gate covers both failure shapes (SPEC §7 Gate 3):
     * retryable HTTP statuses AND transport-level network errors, so an
     * idempotent operation survives a connection blip while a non-idempotent
     * POST is still attempted exactly once. The whole-request time budget
     * (Ktor's [HttpRequestTimeoutException]) is the deliberate carve-out —
     * once the caller's configured timeout has elapsed, retrying would extend
     * total latency past what they asked for. Auth headers are attached per
     * attempt inside [request], so every retry re-authenticates naturally.
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

        // Retry eligibility (SPEC §7 Gates 1+2), hoisted ahead of the attempt so
        // the HTTP-status and network-error paths share a single gate: safe HTTP
        // methods (GET, PUT, DELETE, HEAD) are always retryable, and the
        // per-operation `idempotent` flag can upgrade others.
        val opConfig = operationName?.let { Metadata.operations[it] }
        val opRetry = opConfig?.retry
        val isRetryable = method in IDEMPOTENT_METHODS || opConfig?.idempotent == true
        // The operation's declared max is a ceiling on the caller's configured
        // attempt count, never a replacement for it (SPEC.md §2): a caller who
        // lowered maxRetries is honored, and a raised cap is still clamped to
        // the operation's declared max.
        val maxAttempts = minOf(
            config.maxRetries.coerceAtLeast(1),
            opRetry?.maxRetries ?: config.maxRetries,
        )
        val baseDelayMs = opRetry?.baseDelayMs ?: config.baseRetryDelay.inWholeMilliseconds

        val startTime = currentTimeMillis()
        // Mirror of the Swift directive loop (#517): the catch clauses only
        // classify the attempt's outcome; retry side effects (on_retry, backoff
        // sleep, the next attempt) run outside any catch, so a
        // CancellationException from the sleep propagates raw and no phantom
        // request events fire for an attempt that already ended.
        val outcome: AttemptOutcome = try {
            AttemptOutcome.Completed(request(method, url, body))
        } catch (e: CancellationException) {
            throw e
        } catch (e: BasecampException) {
            // A deliberate SDK error (e.g. the same-origin credential guard) is
            // already classified — surface it as-is rather than masking it as a
            // retryable Network error.
            val duration = currentTimeMillis() - startTime
            hooks.safeOnRequestEnd(info, RequestResult(
                statusCode = 0,
                duration = duration.millisToDuration(),
                error = e,
            ))
            throw e
        } catch (e: Exception) {
            val duration = currentTimeMillis() - startTime
            hooks.safeOnRequestEnd(info, RequestResult(
                statusCode = 0,
                duration = duration.millisToDuration(),
                error = e,
            ))
            AttemptOutcome.NetworkFailure(e)
        }

        val response: HttpResponse = when (outcome) {
            is AttemptOutcome.Completed -> outcome.response
            is AttemptOutcome.NetworkFailure -> {
                val wrapped = BasecampException.Network(
                    message = "Network error: ${outcome.cause.message}",
                    cause = outcome.cause,
                )
                // Total-budget carve-out: HttpRequestTimeoutException means the
                // caller's whole-request time allowance is already spent, so it
                // never retries. Everything else the transport throws —
                // connect/socket timeouts, connection resets, DNS failures
                // (including CIO's UnresolvedAddressException, which is not an
                // IOException) — stays retryable, matching Swift's broad
                // classification.
                val retryableFailure = outcome.cause !is HttpRequestTimeoutException
                if (config.enableRetry && isRetryable && retryableFailure && attempt < maxAttempts) {
                    val delayMs = calculateBackoffDelay(baseDelayMs, attempt)
                    hooks.safeOnRetry(info, attempt + 1, wrapped, delayMs)
                    kotlinx.coroutines.delay(delayMs)
                    return requestWithRetry(method, url, body, attempt + 1, operationName)
                }
                throw wrapped
            }
        }

        val duration = currentTimeMillis() - startTime
        hooks.safeOnRequestEnd(info, RequestResult(
            statusCode = response.status.value,
            duration = duration.millisToDuration(),
        ))

        val status = response.status.value
        val shouldRetry = config.enableRetry && isRetryable && if (opRetry != null) {
            status in opRetry.retryOn
        } else {
            status in RETRYABLE_STATUS_CODES
        }

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
        requireSameOrigin(url)
        return try {
            httpClient.request(url) {
                this.method = method
                authStrategy.authenticate(this)
                header(HttpHeaders.UserAgent, config.userAgent)
                header(HttpHeaders.Accept, "application/json")
                header(HttpHeaders.ContentType, contentType)
                setBody(data)
            }
        } catch (e: ResponseException) {
            e.response
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
        } catch (e: BasecampException) {
            // A deliberate SDK error (e.g. the same-origin credential guard) is
            // already classified — surface it as-is rather than masking it as a
            // retryable Network error.
            val duration = currentTimeMillis() - startTime
            hooks.safeOnRequestEnd(info, RequestResult(
                statusCode = 0,
                duration = duration.millisToDuration(),
                error = e,
            ))
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

    /**
     * Attach-point backstop: refuse to attach credentials to a foreign origin.
     * Localhost is carved out for dev/test. Mirrors the same-origin guard used
     * for pagination Link headers.
     */
    private fun requireSameOrigin(url: String) {
        if (!isLocalhost(url) && !isSameOrigin(url, config.baseUrl)) {
            throw BasecampException.Usage(
                "Refusing to send credentials to a different origin than base URL: " +
                    BasecampException.truncateMessage(url)
            )
        }
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

/**
 * Outcome of a single transport attempt: a response (any status), or a
 * transport-level failure the loop tail may retry. The catch clause only
 * classifies into this type; acting on it happens outside the catch.
 */
private sealed interface AttemptOutcome {
    class Completed(val response: HttpResponse) : AttemptOutcome
    class NetworkFailure(val cause: Exception) : AttemptOutcome
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
