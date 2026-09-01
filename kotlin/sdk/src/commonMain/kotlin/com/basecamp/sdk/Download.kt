package com.basecamp.sdk

import com.basecamp.sdk.http.AuthPhaseFailure
import com.basecamp.sdk.http.BasecampHttpClient
import com.basecamp.sdk.http.currentTimeMillis
import com.basecamp.sdk.http.millisToDuration
import com.basecamp.sdk.http.safeOnRequestEnd
import com.basecamp.sdk.http.safeOnRequestStart
import com.basecamp.sdk.http.safeOnRetry
import io.ktor.client.*
import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.coroutines.delay

/**
 * SPEC §14's declared hop-1 retry set for downloads — never 500, which stays
 * aligned with the main GET loop's declared-set discipline rather than the
 * error taxonomy's broader "all 5xx retryable" flag.
 */
private val DOWNLOAD_RETRY_ON = setOf(429, 502, 503, 504)

/** The redirects hop 1 dispatches on (SPEC §14 step 3d) and hop 2 refuses. */
private val REDIRECT_STATUSES = setOf(301, 302, 303, 307, 308)

/**
 * Result of downloading file content from a URL.
 *
 * @property body Raw file content.
 * @property contentType MIME type of the file.
 * @property contentLength Size in bytes, or -1 if unknown.
 * @property filename Filename extracted from the URL.
 */
data class DownloadResult(
    val body: ByteArray,
    val contentType: String,
    val contentLength: Long,
    val filename: String,
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is DownloadResult) return false
        return body.contentEquals(other.body) &&
            contentType == other.contentType &&
            contentLength == other.contentLength &&
            filename == other.filename
    }

    override fun hashCode(): Int {
        var result = body.contentHashCode()
        result = 31 * result + contentType.hashCode()
        result = 31 * result + contentLength.hashCode()
        result = 31 * result + filename.hashCode()
        return result
    }
}

/**
 * Extracts a filename from the last path segment of a URL.
 * Falls back to "download" if the URL is unparseable or has no path segments.
 */
fun filenameFromURL(rawURL: String): String {
    if (rawURL.isBlank()) return "download"
    return try {
        val url = Url(rawURL)
        // Use rawSegments to detect trailing slashes (empty last segment)
        val raw = url.rawSegments
        if (raw.isEmpty()) return "download"
        val last = raw.last()
        if (last.isEmpty() || last == "." || last == "/") return "download"
        try {
            last.decodeURLPart()
        } catch (_: Exception) {
            last
        }
    } catch (_: Exception) {
        "download"
    }
}

/**
 * Downloads file content from any API-routable download URL.
 *
 * Handles the full download flow: URL rewriting to the configured API host,
 * authenticated first hop (which typically 302s to a signed download URL),
 * and unauthenticated second hop to fetch the actual file content. Neither
 * hop follows a redirect on its own: hop 1's is the dispatch to hop 2, and a
 * redirect on hop 2 is an error (SPEC §14 "Hop-2 Redirect Policy").
 *
 * The first hop retries under the SPEC §14 policy — network errors plus
 * {429, 502, 503, 504}, never 500 — with exponential backoff (Retry-After
 * honored on 429) under the public maxRetries total-attempt cap coerced to at
 * least one; `enableRetry = false` collapses it to exactly one attempt. The
 * second hop is exempt: no retry, no auth.
 *
 * @param rawURL Absolute download URL (e.g., from bc-attachment elements).
 * @return [DownloadResult] with body, contentType, contentLength, and filename.
 * @throws BasecampException.Usage if rawURL is blank or not absolute.
 * @throws BasecampException.Network on transport failure.
 * @throws BasecampException on API errors.
 */
suspend fun AccountClient.downloadURL(rawURL: String): DownloadResult {
    // Validation
    if (rawURL.isBlank()) {
        throw BasecampException.Usage("download URL is required")
    }
    if (!rawURL.startsWith("http://") && !rawURL.startsWith("https://")) {
        throw BasecampException.Usage("download URL must be an absolute URL")
    }
    try {
        Url(rawURL)
    } catch (_: Exception) {
        throw BasecampException.Usage("download URL must be an absolute URL")
    }

    // Operation hooks
    val op = OperationInfo(
        service = "Account",
        operation = "DownloadURL",
        resourceType = "download",
        isMutation = false,
    )
    val opStart = currentTimeMillis()
    parent.hooks.safeOnOperationStart(op)

    var operationError: Throwable? = null
    return try {
        // URL rewriting: replace origin with config.baseUrl, preserve path+query+fragment
        val rewrittenURL = rewriteOrigin(rawURL, parent.config.baseUrl)

        // Create one-shot client with no redirect following, sharing the engine
        // and applying the SDK's timeout settings. Both hops run on it: hop 1
        // so the SDK reads the redirect itself, hop 2 so the signed host cannot
        // choose a further destination (SPEC §14 "Hop-2 Redirect Policy").
        val timeoutMs = parent.config.timeout.inWholeMilliseconds
        val noRedirectClient = HttpClient(httpClient.httpClient.engine) {
            followRedirects = false
            expectSuccess = false
            install(HttpTimeout) {
                requestTimeoutMillis = timeoutMs
                connectTimeoutMillis = timeoutMs
                socketTimeoutMillis = timeoutMs
            }
        }

        noRedirectClient.use { client ->
            // Hop 1: Authenticated API request (capture redirect) under the
            // SPEC §14 hop-1 retry policy. The budget is the public maxRetries
            // total-attempt cap coerced to at least one (an accepted
            // maxRetries = 0 still sends one attempt), collapsed to exactly
            // one attempt when enableRetry is false.
            val maxAttempts = if (parent.config.enableRetry) parent.config.maxRetries.coerceAtLeast(1) else 1
            val baseDelayMs = parent.config.baseRetryDelay.inWholeMilliseconds
            val response = downloadHop1(client, rewrittenURL, maxAttempts, baseDelayMs)

            val status = response.status.value

            when {
                status in REDIRECT_STATUSES -> {
                    // Redirect — extract Location, proceed to hop 2
                    val location = response.headers[HttpHeaders.Location]
                    if (location.isNullOrEmpty()) {
                        throw BasecampException.Api(
                            "redirect $status with no Location header", status
                        )
                    }

                    // Resolve Location: if absolute use as-is, if relative resolve against rewritten URL
                    val resolvedLocation = resolveLocation(rewrittenURL, location)

                    // Hop 2: fetch from signed URL (no auth, no hooks)
                    val signedResponse: HttpResponse
                    try {
                        signedResponse = client.request(resolvedLocation) {
                            method = HttpMethod.Get
                            // No auth, no User-Agent — bare request
                        }
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        // SPEC §9: the transport error renders the signed URL —
                        // fixed message, no cause chained. Cancellation was
                        // rethrown raw above, so nothing here needs the chain.
                        throw BasecampException.Network(
                            message = "Download failed",
                            cause = null,
                        )
                    }

                    // The client above does not follow, so a redirect lands here: the
                    // signed URL is the one destination the API host named, and
                    // its Location is never dialled (#805).
                    if (signedResponse.status.value in REDIRECT_STATUSES) {
                        throw BasecampException.Api(
                            "redirect ${signedResponse.status.value} on the signed download hop is not followed",
                            signedResponse.status.value,
                        )
                    }

                    if (signedResponse.status.value !in 200..299) {
                        throw BasecampException.Api(
                            "download failed with status ${signedResponse.status.value}",
                            signedResponse.status.value,
                        )
                    }

                    DownloadResult(
                        body = signedResponse.readRawBytes(),
                        contentType = signedResponse.headers[HttpHeaders.ContentType] ?: "",
                        contentLength = parseContentLength(signedResponse.headers[HttpHeaders.ContentLength]),
                        filename = filenameFromURL(rawURL),
                    )
                }

                status in 200..299 -> {
                    // Direct download — no second hop
                    DownloadResult(
                        body = response.readRawBytes(),
                        contentType = response.headers[HttpHeaders.ContentType] ?: "",
                        contentLength = parseContentLength(response.headers[HttpHeaders.ContentLength]),
                        filename = filenameFromURL(rawURL),
                    )
                }

                else -> {
                    // Error response — the shared SPEC §6 parser used by the
                    // service layer, so download failures carry the same
                    // message fallback and field-keyed validation data.
                    val bodyText = try {
                        response.bodyAsText()
                    } catch (e: CancellationException) {
                        throw e
                    } catch (_: Exception) {
                        null
                    }
                    throw exceptionFromErrorBody(
                        status = status,
                        bodyText = bodyText,
                        requestId = response.headers["X-Request-Id"],
                        retryAfter = parseRetryAfter(response.headers["Retry-After"]),
                        json = parent.json,
                    )
                }
            }
        }
    } catch (e: CancellationException) {
        throw e
    } catch (e: Throwable) {
        operationError = e
        throw e
    } finally {
        val opDuration = currentTimeMillis() - opStart
        parent.hooks.safeOnOperationEnd(op, OperationResult(
            duration = opDuration.millisToDuration(),
            error = operationError,
        ))
    }
}

/**
 * Runs the download's authenticated hop 1 under the SPEC §14 retry policy:
 * network errors plus [DOWNLOAD_RETRY_ON] — never 500 — retried with
 * exponential backoff (Retry-After honored on 429) while attempts remain.
 * DownloadURL is deliberately absent from the behavior model, so the policy
 * lives here rather than being looked up by operation.
 *
 * Mirror of [BasecampHttpClient.requestWithRetry]'s directive shape (#517):
 * the catch clauses only classify the attempt's outcome; retry side effects
 * (on_retry, the backoff sleep, the next attempt) run outside any catch, so a
 * CancellationException from the sleep propagates raw and no phantom request
 * events fire for an attempt that already ended.
 *
 * Every attempt re-runs the auth strategy so a rotated token is picked up. A
 * throwing strategy is a configuration or credential-provider fault, not a
 * transport fault: it is tagged with the shared [AuthPhaseFailure], surfaces
 * raw, and spends no retry budget — the same classification
 * [BasecampHttpClient] applies.
 */
private suspend fun AccountClient.downloadHop1(
    client: HttpClient,
    url: String,
    maxAttempts: Int,
    baseDelayMs: Long,
): HttpResponse {
    // Hooks render this flow's URL as origin+path only (SPEC §9): the
    // caller's URL can smuggle a signed query through the rewrite into hop 1.
    // The wire request keeps the query; only the rendering is projected.
    val hookUrl = url.substringBefore('?').substringBefore('#')
    var attempt = 1
    while (true) {
        val requestInfo = RequestInfo(method = "GET", url = hookUrl, attempt = attempt)
        parent.hooks.safeOnRequestStart(requestInfo)
        val reqStart = currentTimeMillis()

        var failure: Exception? = null
        var attemptResponse: HttpResponse? = null
        try {
            attemptResponse = client.request(url) {
                method = HttpMethod.Get
                try {
                    parent.authStrategy.authenticate(this)
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Exception) {
                    throw AuthPhaseFailure(e)
                }
                header(HttpHeaders.UserAgent, parent.config.userAgent)
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: AuthPhaseFailure) {
            val duration = currentTimeMillis() - reqStart
            parent.hooks.safeOnRequestEnd(requestInfo, RequestResult(
                statusCode = 0,
                duration = duration.millisToDuration(),
                error = e.original,
            ))
            throw e.original
        } catch (e: Exception) {
            failure = e
        }

        if (failure != null) {
            // SPEC §9: the transport error renders the hop-1 URL (and any
            // signed query smuggled into it) — fixed message, no cause
            // chained, and constructed BEFORE the request-end hook so hooks
            // and the caller see the same projection. Cancellation was
            // rethrown raw above, so nothing here needs the chain.
            val wrapped = BasecampException.Network(
                message = "Network error",
                cause = null,
            )
            val duration = currentTimeMillis() - reqStart
            parent.hooks.safeOnRequestEnd(requestInfo, RequestResult(
                statusCode = 0,
                duration = duration.millisToDuration(),
                error = wrapped,
            ))
            // Same total-budget carve-out as the client loop: an attempt that
            // consumed its entire per-attempt time budget is a slowness shape
            // a retry tends to repeat, not a transient blip.
            val retryableFailure = failure !is HttpRequestTimeoutException
            if (!retryableFailure || attempt >= maxAttempts) {
                throw wrapped
            }
            val delayMs = BasecampHttpClient.calculateBackoffDelay(baseDelayMs, attempt)
            parent.hooks.safeOnRetry(requestInfo, attempt + 1, wrapped, delayMs)
            delay(delayMs)
            attempt += 1
            continue
        }

        val response = attemptResponse!!
        val duration = currentTimeMillis() - reqStart
        parent.hooks.safeOnRequestEnd(requestInfo, RequestResult(
            statusCode = response.status.value,
            duration = duration.millisToDuration(),
        ))

        val status = response.status.value
        if (status !in DOWNLOAD_RETRY_ON || attempt >= maxAttempts) {
            return response
        }

        val retryAfter = parseRetryAfter(response.headers["Retry-After"])
        val delayMs = if (status == 429 && retryAfter != null) {
            retryAfter.toLong() * 1000
        } else {
            BasecampHttpClient.calculateBackoffDelay(baseDelayMs, attempt)
        }
        parent.hooks.safeOnRetry(requestInfo, attempt + 1, BasecampException.Api("HTTP $status", status), delayMs)
        delay(delayMs)
        attempt += 1
    }
}

/**
 * Rewrites a URL's origin (scheme + host + port) to match the base URL,
 * preserving the path, query, and fragment.
 */
private fun rewriteOrigin(rawURL: String, baseUrl: String): String {
    val schemeEnd = rawURL.indexOf("://")
    if (schemeEnd < 0) return rawURL
    val afterScheme = schemeEnd + 3
    val pathStart = rawURL.indexOf('/', afterScheme)
    val pathAndRest = if (pathStart < 0) "" else rawURL.substring(pathStart)
    val base = baseUrl.trimEnd('/')
    return "$base$pathAndRest"
}

/**
 * Resolves a Location header value against a base URL.
 * If the location is absolute, returns it as-is.
 * If relative, resolves against the origin of the base URL.
 */
private fun resolveLocation(base: String, location: String): String {
    if (location.startsWith("http://") || location.startsWith("https://")) {
        return location
    }
    val schemeEnd = base.indexOf("://")
    if (schemeEnd < 0) return location
    val afterScheme = schemeEnd + 3
    val pathStart = base.indexOf('/', afterScheme)
    val origin = if (pathStart < 0) base else base.substring(0, pathStart)
    val normalizedPath = if (location.startsWith("/")) location else "/$location"
    return "$origin$normalizedPath"
}

/** Parse Content-Length header defensively, returning -1 for missing/invalid values. */
private fun parseContentLength(value: String?): Long {
    if (value.isNullOrEmpty()) return -1
    val parsed = value.toLongOrNull() ?: return -1
    return if (parsed >= 0) parsed else -1
}

/** Safely call onOperationStart, catching hook exceptions. */
private fun BasecampHooks.safeOnOperationStart(info: OperationInfo) {
    runCatching { onOperationStart(info) }
}

/** Safely call onOperationEnd, catching hook exceptions. */
private fun BasecampHooks.safeOnOperationEnd(info: OperationInfo, result: OperationResult) {
    runCatching { onOperationEnd(info, result) }
}
