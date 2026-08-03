package com.basecamp.sdk.generator

/**
 * Orders an options class's constructor parameters append-only.
 *
 * Options classes are data classes with defaults, so callers may construct them
 * positionally. Constructor position is therefore part of the public API, and
 * the pre-1.0 policy in kotlin/README.md says public APIs evolve append-only:
 * new parameters go *after* existing ones.
 *
 * The natural order — optional query params in spec order, then the synthetic
 * `maxItems` — violates that on its own, because `maxItems` is last and every
 * new query param displaces it. So the shipped order is pinned per class in
 * `options-param-order.json` and honored here: pinned parameters keep their
 * position, parameters absent from the pin are appended in natural order, and
 * pinned parameters the spec has since dropped fall out.
 *
 * A class with no pin (one being emitted for the first time) is emitted in
 * natural order and pinned from then on.
 */
internal fun orderOptionsParams(pinned: List<String>, natural: List<String>): List<String> {
    val present = natural.toSet()
    return pinned.filter { it in present } + natural.filter { it !in pinned }
}

/**
 * Generates body request classes and options classes for each operation
 * that needs them.
 *
 * @param paramOrder shipped constructor order per options class, from
 *   `options-param-order.json`. See [orderOptionsParams].
 */
class TypeEmitter(private val paramOrder: Map<String, List<String>> = emptyMap()) {

    private val emittedParamOrder = linkedMapOf<String, List<String>>()

    /**
     * The constructor order actually emitted, per options class. Main writes it
     * back out as the next run's pin.
     */
    fun emittedParamOrder(): Map<String, List<String>> = emittedParamOrder.toSortedMap()

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
                // Wrapped pagination (a paginated array nested under a response
                // key) is as paginated as the bare-array kind: the options class
                // still needs maxItems and toPaginationOptions(), because
                // ServiceEmitter hands the result to requestPaginatedWrapped.
                val hasPagination = op.hasPagination && (op.returnsArray || op.paginationKey != null)
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

        val declarations = linkedMapOf<String, String>()
        for (q in optionalParams) {
            val camelName = q.name.snakeToCamelCase()
            // Compiler-level deprecation (see #406): @Deprecated on the property
            // warns at read sites. It binds to the property (it cannot target a
            // VALUE_PARAMETER), so the named-constructor-arg call site stays
            // unflagged — documented as unsupported, parallel to Swift.
            val annotation = if (q.deprecated) "    @Deprecated(${kotlinStringLiteral(q.deprecationReason ?: "deprecated")})\n" else ""
            // Surface the OpenAPI parameter description as KDoc so IDE
            // QuickHelp shows it — the options class is the only place a
            // caller meets these parameters.
            // Collapse spec line wrapping, and defuse any "*/" so a
            // description cannot terminate the KDoc block early.
            // A deprecated param's description IS its deprecation notice
            // upstream; the @Deprecated annotation already carries it, so
            // emitting KDoc too would just say it twice.
            val doc = q.description?.takeIf { it.isNotBlank() && !q.deprecated }
                ?.replace(Regex("\\s+"), " ")
                ?.replace("*/", "* /")
                ?.let { "    /** $it */\n" } ?: ""
            declarations[camelName] = "$doc$annotation    val $camelName: ${q.type}? = null"
        }
        if (hasPagination) {
            declarations["maxItems"] = "    val maxItems: Int? = null"
        }

        // Constructor position is public API here (see orderOptionsParams).
        val order = orderOptionsParams(paramOrder[className].orEmpty(), declarations.keys.toList())
        emittedParamOrder[className] = order

        sb.appendLine(order.map { declarations.getValue(it) }.joinToString(",\n"))
        sb.appendLine(") {")

        // Convert to PaginationOptions if needed. A `page` query param is
        // carried across too: BaseService needs it to know the caller pinned a
        // single page and must not follow Link headers (SPEC §8).
        if (hasPagination) {
            val pageArg = if (optionalParams.any { it.name == "page" }) ", page = page" else ""
            sb.appendLine(
                "    fun toPaginationOptions(): PaginationOptions = " +
                    "PaginationOptions(maxItems = maxItems$pageArg)"
            )
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
