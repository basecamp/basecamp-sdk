package com.basecamp.sdk

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

/**
 * Shared non-2xx error-body handling — the single SPEC §6 implementation
 * behind both the service layer and [Download], so every SDK surface parses
 * error bodies identically (scalar-member string rule, `"message"` fallback,
 * and field-keyed validation extraction included).
 *
 * [bodyText] is the response body as read by the caller (already normalized
 * where the caller normalizes); `null`, blank, or non-JSON bodies fall back to
 * [statusDescription].
 */
internal fun exceptionFromErrorBody(
    status: Int,
    statusDescription: String,
    bodyText: String?,
    requestId: String?,
    retryAfter: Int?,
    json: Json,
): BasecampException {
    var message: String = statusDescription.ifEmpty { "Request failed" }
    var serverMessage: String? = null
    var hint: String? = null
    var fieldErrors: Map<String, List<String>>? = null

    try {
        if (!bodyText.isNullOrBlank()) {
            val jsonBody = json.parseToJsonElement(bodyText)
            if (jsonBody is JsonObject) {
                // Safe casts, not .jsonPrimitive: SPEC §6 uses a key only
                // when its value is a string, and a throwing access here
                // would abandon the field-error extraction below for
                // bodies like {"error": {}, "errors": {...}}.
                // "error" wins; "message" is the SPEC §6 step-4 fallback.
                (stringMember(jsonBody, "error") ?: stringMember(jsonBody, "message"))?.let {
                    val truncated = BasecampException.truncateMessage(it)
                    serverMessage = truncated
                    message = truncated
                }
                stringMember(jsonBody, "error_description")?.let {
                    hint = BasecampException.truncateMessage(it)
                }
                if (status == 400 || status == 422) {
                    fieldErrors = parseFieldErrors(jsonBody)
                    fieldErrors?.let { fe ->
                        val flat = BasecampException.flattenFieldErrors(fe)
                        // Appended in parentheses after a top-level message,
                        // standing alone otherwise; fromHttpStatus truncates
                        // the composed result so the tail is capped too.
                        message = serverMessage?.let { "$it ($flat)" } ?: flat
                    }
                }
            }
        }
    } catch (_: Exception) {
        // Body is not JSON — use status text
    }

    return BasecampException.fromHttpStatus(status, message, hint, requestId, retryAfter, fieldErrors)
}

/** Returns a member's string value, or null when absent or not a string. */
private fun stringMember(body: JsonObject, key: String): String? =
    (body[key] as? JsonPrimitive)?.takeIf { it.isString }?.content

/**
 * Extracts the field-keyed validation errors map from a parsed error body —
 * the Rails RecordInvalid rendering `{"errors": {"field": ["msg", ...]}}`.
 * Entries whose value is not an array are skipped, non-string elements are
 * dropped, and a map with no usable entries is treated as absent (null).
 */
private fun parseFieldErrors(body: JsonObject): Map<String, List<String>>? {
    val errors = body["errors"] as? JsonObject ?: return null
    val fieldErrors = mutableMapOf<String, List<String>>()
    for ((field, value) in errors) {
        val values = value as? JsonArray ?: continue
        val messages = values.mapNotNull { element ->
            (element as? JsonPrimitive)?.takeIf { it.isString }?.content
        }
        if (messages.isNotEmpty()) {
            fieldErrors[field] = messages
        }
    }
    return fieldErrors.ifEmpty { null }
}
