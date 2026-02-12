package com.basecamp.sdk.generator

/**
 * Generates body request classes and options classes for each operation
 * that needs them.
 */
class TypeEmitter {

    /**
     * Generate all body and options classes for a set of services.
     * Returns the Kotlin source for a single file containing all types.
     */
    fun generateTypes(services: Map<String, ServiceDefinition>): String {
        val sb = StringBuilder()

        sb.appendLine("package com.basecamp.sdk.generated.services")
        sb.appendLine()
        sb.appendLine("import com.basecamp.sdk.PaginationOptions")
        sb.appendLine("import kotlinx.serialization.Serializable")
        sb.appendLine("import kotlinx.serialization.json.JsonObject")
        sb.appendLine()
        sb.appendLine("/**")
        sb.appendLine(" * Request body and options classes for generated service methods.")
        sb.appendLine(" *")
        sb.appendLine(" * @generated from OpenAPI spec — do not edit directly")
        sb.appendLine(" */")
        sb.appendLine()

        val generatedBodies = mutableSetOf<String>()
        val generatedOptions = mutableSetOf<String>()

        for ((_, service) in services.entries.sortedBy { it.key }) {
            for (op in service.operations) {
                // Body class
                if (op.bodyContentType == "json" && op.bodyProperties.isNotEmpty()) {
                    val className = "${op.operationId}Body"
                    if (className !in generatedBodies) {
                        generatedBodies += className
                        sb.append(generateBodyClass(op, className))
                        sb.appendLine()
                    }
                }

                // Options class
                val hasOptionalQuery = op.queryParams.any { !it.required }
                val hasPagination = op.hasPagination && op.returnsArray
                if (hasOptionalQuery || hasPagination) {
                    val className = if (hasPagination && !hasOptionalQuery) {
                        // Just uses PaginationOptions directly
                        continue
                    } else {
                        "${op.operationId}Options"
                    }
                    if (className !in generatedOptions) {
                        generatedOptions += className
                        sb.append(generateOptionsClass(op, className, hasPagination))
                        sb.appendLine()
                    }
                }
            }
        }

        return sb.toString()
    }

    private fun generateBodyClass(op: ParsedOperation, className: String): String {
        val sb = StringBuilder()
        sb.appendLine("/** Request body for ${op.operationId}. */")
        sb.appendLine("data class $className(")

        val lines = mutableListOf<String>()
        for (p in op.bodyProperties) {
            val camelName = p.name.snakeToCamelCase()
            val type = mapBodyPropertyType(p)
            val nullable = if (!p.required) "?" else ""
            val default = if (!p.required) " = null" else ""
            lines += "    val $camelName: $type$nullable$default"
        }

        sb.appendLine(lines.joinToString(",\n"))
        sb.appendLine(")")
        return sb.toString()
    }

    private fun generateOptionsClass(op: ParsedOperation, className: String, hasPagination: Boolean): String {
        val sb = StringBuilder()
        val optionalParams = op.queryParams.filter { !it.required }

        sb.appendLine("/** Options for ${op.operationId}. */")
        sb.appendLine("data class $className(")

        val lines = mutableListOf<String>()
        for (q in optionalParams) {
            val camelName = q.name.snakeToCamelCase()
            lines += "    val $camelName: ${q.type}? = null"
        }
        if (hasPagination) {
            lines += "    val maxItems: Int? = null"
        }

        sb.appendLine(lines.joinToString(",\n"))
        sb.appendLine(") {")

        // Convert to PaginationOptions if needed
        if (hasPagination) {
            sb.appendLine("    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems)")
        }

        sb.appendLine("}")
        return sb.toString()
    }

    private fun mapBodyPropertyType(p: BodyProperty): String = when (p.type) {
        "Long" -> "Long"
        "Int" -> "Int"
        "Boolean" -> "Boolean"
        "Double" -> "Double"
        "String" -> "String"
        "JsonObject" -> "JsonObject"
        "List<Long>" -> "List<Long>"
        "List<Int>" -> "List<Int>"
        "List<String>" -> "List<String>"
        "List<JsonObject>" -> "List<JsonObject>"
        else -> {
            if (p.type.startsWith("List<")) p.type
            else "String"
        }
    }
}
