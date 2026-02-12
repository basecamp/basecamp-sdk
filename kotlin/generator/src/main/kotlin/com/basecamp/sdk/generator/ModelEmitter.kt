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

        val propLines = mutableListOf<String>()
        for ((propName, propSchema) in properties) {
            val propObj = propSchema.jsonObject
            val kotlinType = resolvePropertyType(propObj)
            val camelName = propName.snakeToCamelCase()
            val needsSerialName = camelName != propName

            val propLine = buildString {
                if (needsSerialName) {
                    append("    @SerialName(\"$propName\") ")
                } else {
                    append("    ")
                }
                append("val $camelName: $kotlinType = ${defaultValue(kotlinType)}")
            }
            propLines += propLine
        }

        lines += propLines.joinToString(",\n")
        lines += ")"

        return lines.joinToString("\n") + "\n"
    }

    /**
     * Resolves a property schema to the appropriate Kotlin type.
     * All fields default to optional (nullable with defaults) since the
     * Basecamp API doesn't guarantee all fields are always present.
     */
    private fun resolvePropertyType(schema: JsonObject): String {
        val ref = schema["\$ref"]?.jsonPrimitive?.content
        if (ref != null) {
            val refName = api.resolveRef(ref)
            // Use the entity type if it's one we generate, otherwise JsonObject
            val typeName = TYPE_ALIASES[refName] ?: return "JsonObject?"
            return "$typeName?"
        }

        return when (schema["type"]?.jsonPrimitive?.content) {
            "integer" -> when (schema["format"]?.jsonPrimitive?.content) {
                "int64" -> "Long"
                else -> "Int"
            }
            "boolean" -> "Boolean"
            "number" -> "Double"
            "string" -> {
                // Password-marked fields are still strings
                "String?"
            }
            "array" -> {
                val itemType = resolveArrayItemType(schema["items"]?.jsonObject)
                "List<$itemType>"
            }
            "object" -> "JsonObject?"
            else -> "JsonElement?"
        }
    }

    private fun resolveArrayItemType(items: JsonObject?): String {
        if (items == null) return "JsonElement"
        val ref = items["\$ref"]?.jsonPrimitive?.content
        if (ref != null) {
            val refName = api.resolveRef(ref)
            return TYPE_ALIASES[refName] ?: "JsonObject"
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
