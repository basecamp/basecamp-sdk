package com.basecamp.sdk.services

import com.basecamp.sdk.*
import com.basecamp.sdk.http.currentTimeMillis
import com.basecamp.sdk.http.millisToDuration
import io.ktor.client.statement.*
import io.ktor.http.*
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.coroutines.cancellation.CancellationException

/**
 * Upload or replace the account logo.
 *
 * Sends a multipart/form-data PUT to /account/logo.json with a "logo" field.
 * Accepted formats: PNG, JPEG, GIF, WebP, AVIF, HEIC. Maximum 5 MB.
 *
 * @param data Raw bytes of the image file.
 * @param filename Display name for the uploaded file (e.g., "logo.png").
 * @param contentType MIME type of the image (e.g., "image/png").
 * @return Parsed JSON response from the API.
 * @throws BasecampException on API or network errors.
 */
suspend fun AccountClient.updateAccountLogo(
    data: ByteArray,
    filename: String,
    contentType: String,
): JsonElement {
    val op = OperationInfo(
        service = "Account",
        operation = "UpdateAccountLogo",
        resourceType = "account_logo",
        isMutation = true,
        projectId = null,
        resourceId = null,
    )
    val opStart = currentTimeMillis()
    parent.hooks.safeOnOperationStart(op)

    var operationError: Throwable? = null
    return try {
        // Build the multipart/form-data body manually for KMP compatibility.
        // Sanitize inputs to prevent CRLF injection in multipart headers.
        val safeFilename = filename.replace("\r", "").replace("\n", "").replace("\"", "\\\"")
        val safeContentType = contentType.replace("\r", "").replace("\n", "")
        val boundary = "----BasecampSDK${currentTimeMillis()}"
        val preamble = buildString {
            append("--$boundary\r\n")
            append("Content-Disposition: form-data; name=\"logo\"; filename=\"$safeFilename\"\r\n")
            append("Content-Type: $safeContentType\r\n")
            append("\r\n")
        }
        val epilogue = "\r\n--$boundary--\r\n"

        val preambleBytes = preamble.encodeToByteArray()
        val epilogueBytes = epilogue.encodeToByteArray()
        val body = ByteArray(preambleBytes.size + data.size + epilogueBytes.size)
        preambleBytes.copyInto(body, 0)
        data.copyInto(body, preambleBytes.size)
        epilogueBytes.copyInto(body, preambleBytes.size + data.size)

        val multipartContentType = "multipart/form-data; boundary=$boundary"

        val url = accountUrl(parent.config.baseUrl, accountId, "/account/logo.json")

        val response = httpClient.requestBinaryWithRetry(
            method = HttpMethod.Put,
            url = url,
            data = body,
            contentType = multipartContentType,
        )

        if (!response.status.isSuccess()) {
            throw errorFromResponse(response)
        }

        val bodyText = response.bodyAsText()
        parent.json.decodeFromString<JsonElement>(bodyText)
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

/** Builds the full API URL for a path relative to the account. */
private fun accountUrl(baseUrl: String, accountId: String, path: String): String {
    val base = baseUrl.trimEnd('/')
    val normalizedPath = if (path.startsWith("/")) path else "/$path"
    return "$base/$accountId$normalizedPath"
}

/** Converts an HTTP error response to a [BasecampException]. */
private suspend fun AccountClient.errorFromResponse(response: HttpResponse): BasecampException {
    val status = response.status.value
    val requestId = response.headers["X-Request-Id"]
    val retryAfter = parseRetryAfter(response.headers["Retry-After"])

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

    return BasecampException.fromHttpStatus(status, message, hint, requestId, retryAfter)
}

/** Safely call onOperationStart, catching hook exceptions. */
private fun BasecampHooks.safeOnOperationStart(info: OperationInfo) {
    runCatching { onOperationStart(info) }
}

/** Safely call onOperationEnd, catching hook exceptions. */
private fun BasecampHooks.safeOnOperationEnd(info: OperationInfo, result: OperationResult) {
    runCatching { onOperationEnd(info, result) }
}
