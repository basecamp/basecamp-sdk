package com.basecamp.sdk.conformance

import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import java.io.File

/**
 * Pure-Kotlin port of `conformance/runner/typescript/schema-validator.ts`.
 *
 * Walks parsed JSON against the OpenAPI response schema, surfacing:
 *   - missing required-field paths (slash-separated, e.g. "owner/id")
 *   - extras-seen field paths (dot-separated for objects, "[]" for arrays,
 *     e.g. "unreads[].new_field")
 *
 * Conventions match the TS / Ruby / Python / Go ports in
 * `conformance/runner/{ruby,python,go}/` so cross-language extras parity diffs
 * (PR 4 §Verification) don't false-fire:
 *   - `additionalProperties: false` is intentionally ignored — extras are
 *     reported but do not fail validation (forward-compat).
 *   - `$ref` chains resolve until a non-ref schema or a cycle. Both
 *     `#/components/schemas/X` and `openapi.json#/components/schemas/X` are
 *     accepted.
 *   - Recursion depth bound 12 as a cycle guard.
 *   - Required walk uses `[i]` element segments; extras walk uses `[]` to
 *     dedupe item-level extras across an array.
 *   - A field whose value is `JsonNull` counts as PRESENT (matches TS
 *     `name in body` semantics) — `nullable` separately governs whether null
 *     is acceptable, but presence-checking only asks "is the key in the
 *     object?".
 *
 * No new dependencies — kotlinx.serialization.json is already on the
 * conformance build classpath via the mock runner.
 */
class SchemaWalker(openapiPath: String) {
    private val doc: JsonObject = Json.parseToJsonElement(File(openapiPath).readText()).jsonObject
    private val schemas: JsonObject = doc["components"]?.jsonObject?.get("schemas")?.jsonObject
        ?: JsonObject(emptyMap())

    /**
     * Returns null when no response schema is found (do not throw).
     * Preference order: 200, then any 2xx, then "default" — matches TS.
     */
    fun findResponseSchema(operationId: String): JsonObject? {
        val paths = doc["paths"]?.jsonObject ?: return null
        for ((_, pathItemEl) in paths) {
            val pathItem = pathItemEl as? JsonObject ?: continue
            for ((_, opEl) in pathItem) {
                val op = opEl as? JsonObject ?: continue
                val opId = op["operationId"]?.jsonPrimitive?.contentOrNull
                if (opId != operationId) continue
                val responses = op["responses"]?.jsonObject ?: continue
                for (code in PREFERRED_CODES) {
                    schemaFor(responses[code])?.let { return it }
                }
                for ((code, response) in responses) {
                    if (code.length == 3 && code[0] == '2' && code[1].isAsciiDigit() && code[2].isAsciiDigit()) {
                        schemaFor(response)?.let { return it }
                    }
                }
                schemaFor(responses["default"])?.let { return it }
            }
        }
        return null
    }

    /** Slash-separated paths for required fields absent from body. */
    fun missingRequired(body: JsonElement, schema: JsonObject): List<String> {
        val out = mutableListOf<String>()
        walkRequired("", body, schema, 0, out)
        return out
    }

    /** Dotted-path strings for fields present on wire but not declared in schema. */
    fun extrasSeen(body: JsonElement, schema: JsonObject): List<String> {
        val out = mutableListOf<String>()
        walkExtras("", body, schema, 0, out)
        return out
    }

    private fun walkRequired(prefix: String, body: JsonElement, schema: JsonElement, depth: Int, out: MutableList<String>) {
        if (depth > MAX_DEPTH || body is JsonNull) return
        val s = resolveRef(schema) as? JsonObject ?: return

        if (body is JsonArray) {
            if (s["type"]?.jsonPrimitive?.contentOrNull != "array") return
            val items = s["items"] ?: return
            for ((i, item) in body.withIndex()) {
                val child = if (prefix.isEmpty()) "[$i]" else "$prefix[$i]"
                walkRequired(child, item, items, depth + 1, out)
            }
            return
        }
        if (body !is JsonObject) return
        val type = s["type"]?.jsonPrimitive?.contentOrNull
        if (type != null && type != "object") return

        val props = s["properties"]?.jsonObject ?: JsonObject(emptyMap())
        // Emit prefix/name for any required key absent from the body. JsonNull
        // values count as PRESENT (TS `name in body`).
        val required = s["required"] as? JsonArray
        if (required != null) {
            for (req in required) {
                val name = req.jsonPrimitive.contentOrNull ?: continue
                if (!body.containsKey(name)) {
                    out += if (prefix.isEmpty()) name else "$prefix/$name"
                }
            }
        }
        for ((name, sub) in props) {
            val value = body[name] ?: continue
            val child = if (prefix.isEmpty()) name else "$prefix/$name"
            walkRequired(child, value, sub, depth + 1, out)
        }
    }

    private fun walkExtras(prefix: String, body: JsonElement, schema: JsonElement, depth: Int, out: MutableList<String>) {
        if (depth > MAX_DEPTH || body is JsonNull) return
        val s = resolveRef(schema) as? JsonObject ?: return

        if (body is JsonArray) {
            if (s["type"]?.jsonPrimitive?.contentOrNull != "array") return
            val items = s["items"] ?: return
            val child = if (prefix.isEmpty()) "[]" else "$prefix[]"
            // Per-array dedup mirrors TS collectExtras's `new Set` for arrays.
            val seen = mutableSetOf<String>()
            for (item in body) {
                val tmp = mutableListOf<String>()
                walkExtras(child, item, items, depth + 1, tmp)
                for (e in tmp) if (seen.add(e)) out += e
            }
            return
        }
        if (body !is JsonObject) return
        val type = s["type"]?.jsonPrimitive?.contentOrNull
        if (type != null && type != "object") return

        val props = s["properties"]?.jsonObject ?: JsonObject(emptyMap())
        for ((key, value) in body) {
            val fieldPath = if (prefix.isEmpty()) key else "$prefix.$key"
            val sub = props[key]
            if (sub == null) {
                out += fieldPath
            } else {
                walkExtras(fieldPath, value, sub, depth + 1, out)
            }
        }
    }

    /** Follow $ref chains until a non-ref schema or a cycle. */
    private fun resolveRef(schema: JsonElement): JsonElement {
        val seen = mutableSetOf<String>()
        var current: JsonElement = schema
        while (current is JsonObject) {
            val ref = current["\$ref"]?.jsonPrimitive?.contentOrNull ?: return current
            if (!seen.add(ref)) return current
            val match = REF_REGEX.matchEntire(ref) ?: return current
            current = schemas[match.groupValues[1]] ?: return current
        }
        return current
    }

    private fun schemaFor(response: JsonElement?): JsonObject? {
        val resp = response as? JsonObject ?: return null
        val content = resp["content"] as? JsonObject ?: return null
        val appJson = content["application/json"] as? JsonObject ?: return null
        return appJson["schema"] as? JsonObject
    }

    private fun Char.isAsciiDigit(): Boolean = this in '0'..'9'

    companion object {
        private const val MAX_DEPTH = 12
        private val PREFERRED_CODES = listOf("200", "201", "202", "203", "204")
        private val REF_REGEX = Regex("^(?:openapi\\.json)?#/components/schemas/(.+)$")
    }
}
