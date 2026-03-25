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
            val numericId = idStr.toLongOrNull()
            val looksNumeric = Regex("^-?\\d+$").matches(idStr)
            val builder = buildJsonObject {
                for ((key, value) in element) {
                    if (key == "id") {
                        if (numericId != null) {
                            put("id", JsonPrimitive(numericId))
                        } else if (looksNumeric) {
                            // Numeric overflow — leave as string, FlexibleLongSerializer will reject
                            put("id", JsonPrimitive(idStr))
                        } else {
                            // Non-numeric sentinel
                            put("id", JsonPrimitive(0L))
                            put("system_label", JsonPrimitive(idStr))
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
