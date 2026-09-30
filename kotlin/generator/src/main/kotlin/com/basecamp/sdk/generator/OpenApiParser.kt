package com.basecamp.sdk.generator

import kotlinx.serialization.json.*

/**
 * Parses the OpenAPI spec JSON into structured data.
 */
private val DEPRECATED_ALIAS_KEYS = setOf("\$ref", "deprecated", "description", "x-deprecated-reason")

class OpenApiParser(private val root: JsonObject) {
    private val schemas: JsonObject = root["components"]!!
        .jsonObject["schemas"]!!
        .jsonObject

    val paths: JsonObject = root["paths"]!!.jsonObject

    fun resolveRef(ref: String): String = ref.substringAfterLast("/")

    fun getSchema(name: String): JsonObject? = schemas[name]?.jsonObject

    /**
     * A component that is nothing but a deprecated `$ref` (a rename kept for
     * compatibility, e.g. CardStep -> Subtask). `deprecated` must be the JSON
     * boolean true, as in every other generator: `booleanOrNull` alone would
     * also read the STRING "true" as true.
     */
    fun isDeprecatedAlias(schema: JsonObject?): Boolean {
        if (schema == null) return false
        val deprecated = schema["deprecated"] as? JsonPrimitive ?: return false
        if (deprecated.isString || deprecated.booleanOrNull != true) return false
        val ref = schema["\$ref"] as? JsonPrimitive ?: return false
        return ref.isString && DEPRECATED_ALIAS_KEYS.containsAll(schema.keys)
    }

    /** Whether a component is an object model: `type: object` with properties. */
    fun isObjectModel(schema: JsonObject?): Boolean =
        schema != null &&
            (schema["type"] as? JsonPrimitive)?.contentOrNull == "object" &&
            (schema["properties"] as? JsonObject)?.isNotEmpty() == true

    /**
     * Deprecated former names: every deprecated alias component, as alias ->
     * target schema name, sorted by alias (String.compareTo, so no locale).
     */
    fun deprecatedAliasSchemas(): List<Pair<String, String>> =
        schemas.entries.mapNotNull { (name, value) ->
            val schema = value as? JsonObject
            if (!isDeprecatedAlias(schema)) return@mapNotNull null
            name to resolveRef(schema!!["\$ref"]!!.jsonPrimitive.content)
        }.sortedBy { it.first }

    /**
     * Find the underlying entity schema for a ResponseContent type.
     * E.g., "GetTodoResponseContent" → "Todo" (via $ref)
     * E.g., "ListTodosResponseContent" → "Todo" (via array items $ref)
     * E.g., "GetPersonProgressResponseContent" → "TimelineEvent" (via object property "events" when paginationKey="events")
     */
    fun findUnderlyingEntitySchema(schemaRef: String, paginationKey: String? = null): String? {
        val schema = getSchema(schemaRef) ?: return null

        // Direct $ref to a known entity
        val directRef = schema["\$ref"]?.jsonPrimitive?.content
        if (directRef != null) {
            val refName = resolveRef(directRef)
            if (refName in TYPE_ALIASES) return refName
        }

        // Array of entities
        if (schema["type"]?.jsonPrimitive?.content == "array") {
            val itemsRef = schema["items"]?.jsonObject?.get("\$ref")?.jsonPrimitive?.content
            if (itemsRef != null) {
                val refName = resolveRef(itemsRef)
                if (refName in TYPE_ALIASES) return refName
            }
        }

        // Wrapped-pagination object: only when paginationKey is specified,
        // look at properties[key].items.$ref to find the entity type
        if (paginationKey != null && schema["type"]?.jsonPrimitive?.content == "object") {
            val keyProp = schema["properties"]?.jsonObject?.get(paginationKey)?.jsonObject
            if (keyProp != null && keyProp["type"]?.jsonPrimitive?.content == "array") {
                val itemsRef = keyProp["items"]?.jsonObject?.get("\$ref")?.jsonPrimitive?.content
                if (itemsRef != null) {
                    val refName = resolveRef(itemsRef)
                    if (refName in TYPE_ALIASES) return refName
                }
            }
        }

        return null
    }

    /**
     * Resolves a schema property type to a Kotlin type string.
     */
    fun schemaToKotlinType(schema: JsonObject): String {
        val ref = schema["\$ref"]?.jsonPrimitive?.content
        if (ref != null) return "JsonObject"

        return when (schema["type"]?.jsonPrimitive?.content) {
            "integer" -> when (schema["format"]?.jsonPrimitive?.content) {
                "int64" -> "Long"
                else -> "Int"
            }
            "boolean" -> "Boolean"
            "number" -> "Double"
            "array" -> {
                val itemType = schema["items"]?.jsonObject?.let { schemaToKotlinType(it) } ?: "JsonElement"
                "List<$itemType>"
            }
            "object" -> "JsonObject"
            else -> "String"
        }
    }

    /**
     * Gets the Go type hint from x-go-type, if present.
     */
    fun getGoType(schema: JsonObject): String? =
        schema["x-go-type"]?.jsonPrimitive?.content
}
