package com.basecamp.sdk.serialization

import kotlinx.serialization.json.*

/**
 * Normalizes Person-shaped objects in raw JSON text.
 *
 * The BC3 API conflates real Person records (numeric id) with system
 * actors like LocalPerson (symbolic id: "basecamp", "campfire").
 * For objects with `personable_type` and a string `id`:
 * - Numeric strings: coerced to Long, no system_label
 * - Numeric overflow: left as string for FlexibleLongSerializer to reject
 * - Non-numeric sentinels: id becomes 0, original preserved as system_label
 *
 * "Numeric" is [parseInt64] and nothing looser — the same scan
 * [FlexibleLongSerializer] reads with, so this pass accepts exactly what the
 * decoder behind it accepts and fails the read exactly where that decoder
 * errors. That is the property `coercePersonID` holds in the reference
 * (`go/pkg/basecamp/normalize.go:40-68`, pinned by
 * `TestPersonIDNormalizeMatchesFlexible`); two spellings of "numeric" at these
 * two layers is how a value the reader would have read as a person turns into
 * the system actor on the way in.
 */
fun normalizePersonIds(jsonText: String, json: Json): String {
    if (!jsonText.contains("personable_type")) return jsonText
    val element = try {
        json.parseToJsonElement(jsonText)
    } catch (_: Exception) {
        return jsonText
    }
    val normalized = normalizeElement(element)
    return normalized.toString()
}

private fun normalizeElement(element: JsonElement): JsonElement = when (element) {
    is JsonObject -> {
        val hasPersonableType = "personable_type" in element
        val idValue = element["id"]
        if (hasPersonableType && idValue is JsonPrimitive && idValue.isString) {
            val idStr = idValue.content
            val parsed = parseInt64(idStr)
            val builder = buildJsonObject {
                for ((key, value) in element) {
                    if (key == "id") {
                        when (parsed) {
                            is ParsedInt64.Value -> put("id", JsonPrimitive(parsed.value))
                            // Numeric overflow — leave as string, FlexibleLongSerializer will reject
                            ParsedInt64.Range -> put("id", JsonPrimitive(idStr))
                            // Non-numeric sentinel
                            ParsedInt64.Syntax -> {
                                put("id", JsonPrimitive(0L))
                                put("system_label", JsonPrimitive(idStr))
                            }
                        }
                    } else {
                        put(key, normalizeElement(value))
                    }
                }
            }
            builder
        } else {
            buildJsonObject {
                for ((key, value) in element) {
                    put(key, normalizeElement(value))
                }
            }
        }
    }
    is JsonArray -> JsonArray(element.map { normalizeElement(it) })
    else -> element
}
