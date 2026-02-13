package com.basecamp.sdk.generator

import kotlinx.serialization.json.*

/**
 * Generates @Serializable data classes from OpenAPI entity schemas.
 */
class ModelEmitter(private val api: OpenApiParser) {

    /**
     * Generate a Kotlin model file for a given entity schema.
     * Returns null if the schema can't be generated (e.g., not an object type).
     */
    fun generateModel(schemaName: String, typeName: String): String? {
        val schema = api.getSchema(schemaName) ?: return null
        val type = schema["type"]?.jsonPrimitive?.content

        // Skip non-object schemas
        if (type != "object") return null

        val properties = schema["properties"]?.jsonObject ?: return null
        if (properties.isEmpty()) return null

        val requiredFields = schema["required"]?.jsonArray
            ?.map { it.jsonPrimitive.content }
            ?.toSet()
            ?: emptySet()

        val lines = mutableListOf<String>()
        lines += "package com.basecamp.sdk.generated.models"
        lines += ""
        lines += "import kotlinx.serialization.SerialName"
        lines += "import kotlinx.serialization.Serializable"
        lines += "import kotlinx.serialization.json.JsonElement"
        lines += "import kotlinx.serialization.json.JsonObject"
        lines += ""
        lines += "/**"
        lines += " * $typeName entity from the Basecamp API."
        lines += " *"
        lines += " * @generated from OpenAPI spec — do not edit directly"
        lines += " */"
        lines += "@Serializable"
        lines += "data class $typeName("

        // Required fields first (no defaults), then optional fields (with defaults)
        val requiredProps = mutableListOf<Pair<String, JsonObject>>()
        val optionalProps = mutableListOf<Pair<String, JsonObject>>()
        for ((propName, propSchema) in properties) {
            if (propName in requiredFields) {
                requiredProps += propName to propSchema.jsonObject
            } else {
                optionalProps += propName to propSchema.jsonObject
            }
        }

        val propLines = mutableListOf<String>()
        for ((propName, propObj) in requiredProps + optionalProps) {
            val isRequired = propName in requiredFields
            val kotlinType = resolvePropertyType(propObj, isRequired)
            val camelName = propName.snakeToCamelCase()
            val needsSerialName = camelName != propName

            val propLine = buildString {
                if (needsSerialName) {
                    append("    @SerialName(\"$propName\") ")
                } else {
                    append("    ")
                }
                append("val $camelName: $kotlinType")
                if (!isRequired) {
                    append(" = ${defaultValue(kotlinType)}")
                }
            }
            propLines += propLine
        }

        lines += propLines.joinToString(",\n")
        lines += ")"

        return lines.joinToString("\n") + "\n"
    }

    /**
     * Resolves a property schema to the appropriate Kotlin type.
     * Required fields are non-nullable; optional fields are nullable with defaults.
     */
    private fun resolvePropertyType(schema: JsonObject, isRequired: Boolean = false): String {
        val ref = schema["\$ref"]?.jsonPrimitive?.content
        if (ref != null) {
            val refName = api.resolveRef(ref)
            // Use TYPE_ALIASES for mapped names, fall back to schema name for supporting types.
            // Only use the schema name if it's a generatable object (has properties).
            val typeName = TYPE_ALIASES[refName] ?: run {
                val refSchema = api.getSchema(refName)
                val hasProperties = refSchema?.get("properties")?.jsonObject?.isNotEmpty() == true
                if (hasProperties) refName else return if (isRequired) "JsonObject" else "JsonObject?"
            }
            return if (isRequired) typeName else "$typeName?"
        }

        return when (schema["type"]?.jsonPrimitive?.content) {
            "integer" -> when (schema["format"]?.jsonPrimitive?.content) {
                "int64" -> "Long"
                else -> "Int"
            }
            "boolean" -> "Boolean"
            "number" -> "Double"
            "string" -> if (isRequired) "String" else "String?"
            "array" -> {
                val itemType = resolveArrayItemType(schema["items"]?.jsonObject)
                "List<$itemType>"
            }
            "object" -> if (isRequired) "JsonObject" else "JsonObject?"
            else -> if (isRequired) "JsonElement" else "JsonElement?"
        }
    }

    private fun resolveArrayItemType(items: JsonObject?): String {
        if (items == null) return "JsonElement"
        val ref = items["\$ref"]?.jsonPrimitive?.content
        if (ref != null) {
            val refName = api.resolveRef(ref)
            return TYPE_ALIASES[refName] ?: run {
                val refSchema = api.getSchema(refName)
                val hasProperties = refSchema?.get("properties")?.jsonObject?.isNotEmpty() == true
                if (hasProperties) refName else "JsonObject"
            }
        }
        return when (items["type"]?.jsonPrimitive?.content) {
            "integer" -> when (items["format"]?.jsonPrimitive?.content) {
                "int64" -> "Long"
                else -> "Int"
            }
            "boolean" -> "Boolean"
            "string" -> "String"
            else -> "JsonElement"
        }
    }

    private fun defaultValue(type: String): String = when {
        type == "Boolean" -> "false"
        type == "Int" -> "0"
        type == "Long" -> "0L"
        type == "Double" -> "0.0"
        type.startsWith("List<") -> "emptyList()"
        type.endsWith("?") -> "null"
        else -> "null"
    }
}
