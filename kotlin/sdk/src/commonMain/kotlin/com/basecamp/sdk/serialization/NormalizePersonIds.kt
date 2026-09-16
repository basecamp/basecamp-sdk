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
                    } else if (key == "system_label" && parsed == ParsedInt64.Syntax) {
                        // DROPPED, because the sentinel branch above already wrote
                        // this key from the raw id. Copying the incoming value here
                        // would overwrite it whenever the body happens to order
                        // `system_label` AFTER `id` — a JSON object's members are
                        // ordered, and this builder replays them in order, so the
                        // last `put` for a key wins.
                        //
                        // The reference has no such hazard and therefore no such
                        // branch: `coercePersonID` assigns into a map
                        // (`go/pkg/basecamp/normalize.go:66-67`), so the label it
                        // writes always wins however the body was spelled. Matching
                        // that is what this drop is for, and it matters because the
                        // value being overwritten came off the wire: a response
                        // carrying its own `system_label` after a sentinel `id`
                        // could otherwise choose the label this SDK reports for the
                        // system actor.
                        //
                        // Only for the sentinel outcome. A value or a range refusal
                        // leaves `system_label` alone in the reference — it is never
                        // assigned on those paths — so an incoming one is preserved
                        // here too, by falling through to the copy below.
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
