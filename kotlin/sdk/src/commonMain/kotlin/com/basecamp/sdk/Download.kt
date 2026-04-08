package com.basecamp.sdk

import com.basecamp.sdk.http.currentTimeMillis
import com.basecamp.sdk.http.millisToDuration
import io.ktor.client.*
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.request.*
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.coroutines.cancellation.CancellationException

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
 * and unauthenticated second hop to fetch the actual file content.
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
        // and applying the SDK's timeout settings
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
            // Hop 1: Authenticated API request (capture redirect)
            val requestInfo = RequestInfo(method = "GET", url = rewrittenURL, attempt = 1)
            parent.hooks.safeOnRequestStart(requestInfo)

            val reqStart = currentTimeMillis()
            val response: HttpResponse
            try {
                response = client.request(rewrittenURL) {
                    method = HttpMethod.Get
                    parent.authStrategy.authenticate(this)
                    header(HttpHeaders.UserAgent, parent.config.userAgent)
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                val duration = currentTimeMillis() - reqStart
                parent.hooks.safeOnRequestEnd(requestInfo, RequestResult(
                    statusCode = 0,
                    duration = duration.millisToDuration(),
                    error = e,
                ))
                throw BasecampException.Network(
                    message = "Network error: ${e.message}",
                    cause = e,
                )
            }

            val duration = currentTimeMillis() - reqStart
            parent.hooks.safeOnRequestEnd(requestInfo, RequestResult(
                statusCode = response.status.value,
                duration = duration.millisToDuration(),
            ))

            val status = response.status.value

            when {
                status in setOf(301, 302, 303, 307, 308) -> {
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
                        throw BasecampException.Network(
                            message = "Download failed: ${e.message}",
                            cause = e,
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
                    // Error response — parse JSON error/hint and Retry-After,
                    // matching BaseService.errorFromResponse
                    val requestId = response.headers["X-Request-Id"]
                    val retryAfter = parseRetryAfter(response.headers["Retry-After"])
                    val reason = response.headers["Reason"]

                    var message: String = response.status.description.ifEmpty { "Request failed" }
                    var hint: String? = null

                    try {
                        val bodyText = response.bodyAsText()
                        if (bodyText.isNotBlank()) {
                            val jsonBody = parent.json.parseToJsonElement(bodyText)
                            if (jsonBody is JsonObject) {
                                jsonBody["error"]?.jsonPrimitive?.content?.let {
                                    message = BasecampException.truncateMessage(it)
                                }
                                jsonBody["error_description"]?.jsonPrimitive?.content?.let {
                                    hint = BasecampException.truncateMessage(it)
                                }
                            }
                        }
                    } catch (e: CancellationException) {
                        throw e
                    } catch (_: Exception) {
                        // Body is not JSON or empty — use status text
                    }

                    throw BasecampException.fromHttpStatus(status, message, hint, requestId, retryAfter, reason)
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

/** Safely call onRequestStart, catching hook exceptions. */
private fun BasecampHooks.safeOnRequestStart(info: RequestInfo) {
    runCatching { onRequestStart(info) }
}

/** Safely call onRequestEnd, catching hook exceptions. */
private fun BasecampHooks.safeOnRequestEnd(info: RequestInfo, result: RequestResult) {
    runCatching { onRequestEnd(info, result) }
}
