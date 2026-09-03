package com.basecamp.sdk

import com.basecamp.sdk.generated.models.TemplateLibraryConfirmationPerson
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.decodeFromJsonElement

/**
 * Shared non-2xx error-body handling — the single SPEC §6 implementation
 * behind both the service layer and [Download], so every SDK surface parses
 * error bodies identically (scalar-member string rule, `"message"` fallback,
 * and field-keyed validation extraction included).
 *
 * [bodyText] is the response body as read by the caller (already normalized
 * where the caller normalizes); `null`, blank, or non-JSON bodies fall back to
 * the fixed code-bearing phrase (SPEC §6 step 5) — never the wire reason
 * phrase, which does not exist under HTTP/2 and is empty for an unregistered
 * code.
 */
internal fun exceptionFromErrorBody(
    status: Int,
    bodyText: String?,
    requestId: String?,
    retryAfter: Int?,
    json: Json,
): BasecampException {
    var message = "Request failed (HTTP $status)"
    var serverMessage: String? = null
    var hint: String? = null
    var fieldErrors: Map<String, List<String>>? = null
    var confirmationPeople: List<TemplateLibraryConfirmationPerson>? = null

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
                    if (status == 422) {
                        confirmationPeople = parseTemplateLibraryConfirmationPeople(jsonBody, json)
                    }
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

    return BasecampException.fromHttpStatus(
        status, message, hint, requestId, retryAfter, fieldErrors, confirmationPeople,
    )
}

private fun parseTemplateLibraryConfirmationPeople(
    body: JsonObject,
    json: Json,
): List<TemplateLibraryConfirmationPerson>? {
    val value = body["people"] as? JsonArray ?: return null
    if (value.isEmpty()) return null
    val people = runCatching {
        json.decodeFromJsonElement<List<TemplateLibraryConfirmationPerson>>(value)
    }.getOrNull() ?: return null
    return people.takeIf { entries ->
        entries.all { it.id > 0 && it.name.isNotEmpty() && it.avatarUrl.isNotEmpty() }
    }
}

/** Returns a member's string value, or null when absent or not a string. */
private fun stringMember(body: JsonObject, key: String): String? =
    (body[key] as? JsonPrimitive)?.takeIf { it.isString }?.content

/**
 * Extracts the field-keyed validation errors map from a parsed error body —
 * the Rails RecordInvalid rendering `{"errors": {"field": ["msg", ...]}}`.
 * Entries whose value is not an array are skipped, non-string elements are
 * dropped, and a map with no usable entries is treated as absent (null).
 *
 * A body with no `errors` object falls through to [parseBareFieldErrors] for
 * the unwrapped rendering, so this is the entry point for both shapes.
 */
private fun parseFieldErrors(body: JsonObject): Map<String, List<String>>? {
    val errors = body["errors"] as? JsonObject ?: return parseBareFieldErrors(body)
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

/**
 * Extracts an unwrapped field map — the `render json: @webhook.errors`
 * rendering, where the whole body is `{"field": ["msg", ...]}`. The gate is
 * all-or-nothing by design (SPEC §6 step 2): with no `errors` key to declare
 * intent, only shape distinguishes a field map from any other JSON object, so
 * a single non-conforming member means this is not one. Returns null unless
 * every member is a non-empty array of non-empty strings.
 */
private fun parseBareFieldErrors(body: JsonObject): Map<String, List<String>>? {
    if (body.isEmpty()) return null
    // Only "errors" is structurally reserved (it belongs to the wrapped path).
    // "error" and "message" are not excluded by name: a flat body carries them
    // as strings, which the shape gate below already rejects.
    if (body.containsKey("errors")) return null

    val fieldErrors = mutableMapOf<String, List<String>>()
    for ((field, value) in body) {
        val values = value as? JsonArray ?: return null
        if (values.isEmpty()) return null
        val messages = values.map { element ->
            val message = (element as? JsonPrimitive)?.takeIf { it.isString }?.content
            if (message.isNullOrEmpty()) return null
            message
        }
        fieldErrors[field] = messages
    }
    return fieldErrors
}
