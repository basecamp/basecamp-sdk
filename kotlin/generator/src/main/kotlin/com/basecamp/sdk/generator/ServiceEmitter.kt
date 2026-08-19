package com.basecamp.sdk.generator

import kotlinx.serialization.json.*

/**
 * Generates Kotlin service classes from parsed operation data.
 */
class ServiceEmitter(private val api: OpenApiParser) {

    fun generateService(service: ServiceDefinition): String {
        val sb = StringBuilder()

        // Header
        sb.appendLine("package com.basecamp.sdk.generated.services")
        sb.appendLine()
        val needsWrappedPagination = service.operations.any {
            it.hasPagination && it.paginationKey != null
        }

        sb.appendLine("import com.basecamp.sdk.*")
        sb.appendLine("import com.basecamp.sdk.generated.models.*")
        if (needsWrappedPagination) {
            // `requiredMember` and `decodeFromJsonElement` between them raise a
            // SerializationException for every way a wrapper can be wrong, which
            // is the one type BaseService maps to the statusless api_error
            // (#728). The `jsonObject`/`jsonArray` casts this replaces raised
            // IllegalArgumentException, and `["key"]!!` a NullPointerException —
            // neither mapped, and neither safe to map.
            sb.appendLine("import com.basecamp.sdk.serialization.requiredMember")
        }
        sb.appendLine("import com.basecamp.sdk.services.BaseService")
        if (service.operations.any { arrayEnvelopeMembers(it) != null }) {
            sb.appendLine("import kotlinx.serialization.SerialName")
            sb.appendLine("import kotlinx.serialization.Serializable")
        }
        sb.appendLine("import kotlinx.serialization.json.JsonElement")
        if (needsWrappedPagination) {
            sb.appendLine("import kotlinx.serialization.json.JsonObject")
            sb.appendLine("import kotlinx.serialization.json.decodeFromJsonElement")
        }

        val needsPagination = service.operations.any { it.hasPagination && (it.returnsArray || it.paginationKey != null) }
        if (needsPagination) {
            // ListResult and PaginationOptions are in the sdk package already
        }

        sb.appendLine()

        // Generate result data classes for wrapped pagination operations
        for (op in service.operations) {
            if (op.hasPagination && op.paginationKey != null && !op.returnsArray) {
                sb.append(generateWrappedResultClass(op))
            }
        }

        // Generate result data classes for typed array-envelope operations
        for (op in service.operations) {
            arrayEnvelopeMembers(op)?.let { sb.append(generateArrayEnvelopeResultClass(op, it)) }
        }

        sb.appendLine("/**")
        sb.appendLine(" * Service for ${service.name} operations.")
        sb.appendLine(" *")
        sb.appendLine(" * @generated from OpenAPI spec — do not edit directly")
        sb.appendLine(" */")
        val classKeyword = if (service.name in EXTENSIBLE_SERVICES) "open class" else "class"
        sb.appendLine("$classKeyword ${service.className}(client: AccountClient) : BaseService(client) {")

        for (op in service.operations) {
            sb.appendLine()
            sb.append(generateMethod(op, service.name))
        }

        sb.appendLine("}")

        return sb.toString()
    }

    private fun generateMethod(op: ParsedOperation, serviceName: String): String {
        val sb = StringBuilder()

        val returnType = buildReturnType(op)
        val params = buildParams(op)
        val description = enrichDescription(op.description.lines().first())

        // KDoc
        sb.appendLine("    /**")
        sb.appendLine("     * $description")
        for (p in op.pathParams) {
            sb.appendLine("     * @param ${p.name.snakeToCamelCase()} ${p.description ?: "The ${p.name.toHumanReadable()}"}")
        }
        if (op.bodyProperties.isNotEmpty() && op.bodyContentType == "json") {
            sb.appendLine("     * @param body Request body")
        }
        if (op.bodyContentType == "octet-stream") {
            sb.appendLine("     * @param data Binary file data to upload")
            sb.appendLine("     * @param contentType MIME type of the file")
        }
        if (op.bodyContentType == "multipart") {
            sb.appendLine("     * @param data Raw bytes of the file to upload")
            sb.appendLine("     * @param filename Display name for the uploaded file")
            sb.appendLine("     * @param contentType MIME type of the file (e.g., \"image/png\")")
        }
        for (q in op.queryParams.filter { it.required }) {
            sb.appendLine("     * @param ${q.name.snakeToCamelCase()} ${q.description ?: q.name.toHumanReadable()}")
        }
        if (op.queryParams.any { !it.required } || (op.hasPagination && (op.returnsArray || op.paginationKey != null))) {
            sb.appendLine("     * @param options Optional query parameters and pagination control")
        }
        sb.appendLine("     */")

        // A function that reads a deprecated query param (e.g. search reads
        // options?.type/bucketId/creatorId) would itself emit DEPRECATION
        // warnings on the generated SDK's own code. Suppress at the function
        // level, keyed on "operation has >=1 deprecated param" rather than the
        // literal operation name, to hold the zero-generator-warning invariant.
        // See #406.
        if (op.queryParams.any { it.deprecated }) {
            sb.appendLine("    @Suppress(\"DEPRECATION\")")
        }

        // Method signature
        sb.appendLine("    suspend fun ${op.methodName}($params): $returnType {")

        // Build OperationInfo
        val projectParam = op.pathParams.find { it.name == "projectId" || it.name == "bucketId" }
        val resourceParam = op.pathParams.findLast { it.name != "projectId" && it.name != "bucketId" && (it.name.endsWith("Id") || it.name == "id") }
        val projectArg = if (projectParam != null) projectParam.name.snakeToCamelCase() else "null"
        val resourceArg = if (resourceParam != null) resourceParam.name.snakeToCamelCase() else "null"

        sb.appendLine("        val info = OperationInfo(")
        sb.appendLine("            service = \"$serviceName\",")
        sb.appendLine("            operation = \"${op.operationId}\",")
        sb.appendLine("            resourceType = \"${op.resourceType}\",")
        sb.appendLine("            isMutation = ${op.isMutation},")
        sb.appendLine("            projectId = $projectArg,")
        sb.appendLine("            resourceId = $resourceArg,")
        sb.appendLine("        )")

        // Build path with interpolated params
        val pathExpr = buildPathExpression(op)

        // Emit query string building if the operation has query params
        val hasQueryParams = op.queryParams.isNotEmpty()
        if (hasQueryParams) {
            sb.append(generateQueryBuilding(op))
        }
        val pathWithQuery = if (hasQueryParams) "$pathExpr + qs" else pathExpr

        val isPaginated = op.hasPagination && op.returnsArray
        val isWrappedPaginated = op.hasPagination && op.paginationKey != null && !op.returnsArray

        if (isWrappedPaginated) {
            val entitySchema = op.responseSchemaRef?.let { api.findUnderlyingEntitySchema(it, op.paginationKey) }
            val entityType = entitySchema?.let { TYPE_ALIASES[it] } ?: "JsonElement"
            val resultClassName = buildWrappedResultClassName(op)

            // Convert custom options to PaginationOptions
            val wrappedHasOptionalQuery = op.queryParams.any { !it.required }
            val wrappedOptionsArg = if (wrappedHasOptionalQuery) "${optionsAccess(op)}toPaginationOptions()" else "options"

            // Both halves of a wrapped response — the items array on every page,
            // and the first page's remaining members — are decoded INSIDE
            // `requestPaginatedWrapped`, the second through its `buildResult`
            // lambda. Every accessor below therefore has to fail as a
            // SerializationException, the one type the primitive maps to SPEC §6's
            // statusless `api_error`: `decodeFromString<JsonObject>` for a body
            // that is not an object, `requiredMember` for an absent member, and
            // `decodeFromJsonElement` for a wrong-typed one. The `!!` and
            // `.jsonArray` this replaces raised NullPointerException and
            // IllegalArgumentException, which surfaced raw (#728).
            val schema = api.getSchema(op.responseSchemaRef!!)
            val properties = schema?.get("properties")?.jsonObject ?: JsonObject(emptyMap())

            sb.appendLine("        return requestPaginatedWrapped<$entityType, $resultClassName>(info, $wrappedOptionsArg, {")
            sb.appendLine("            httpGet($pathWithQuery, operationName = info.operation)")
            sb.appendLine("        }, { body ->")
            sb.appendLine("            json.decodeFromJsonElement<List<$entityType>>(")
            sb.appendLine("                json.decodeFromString<JsonObject>(body).requiredMember(\"${op.paginationKey}\")")
            sb.appendLine("            )")
            val hasWrapperMembers = properties.keys.any { it != op.paginationKey }
            sb.appendLine("        }) { ${if (hasWrapperMembers) "firstPageBody" else "_"}, items ->")
            if (hasWrapperMembers) {
                sb.appendLine("            val wrapper = json.decodeFromString<JsonObject>(firstPageBody)")
            }

            val constructorArgs = mutableListOf<String>()
            for (propName in properties.keys.sorted()) {
                val camelName = propName.snakeToCamelCase()
                if (propName == op.paginationKey) {
                    constructorArgs.add("$camelName = items")
                } else {
                    val propType = resolveWrapperPropertyType(op.responseSchemaRef!!, propName)
                    constructorArgs.add("$camelName = json.decodeFromJsonElement<$propType>(wrapper.requiredMember(\"$propName\"))")
                }
            }

            sb.appendLine("            $resultClassName(")
            for ((i, arg) in constructorArgs.withIndex()) {
                val comma = if (i < constructorArgs.size - 1) "," else ""
                sb.appendLine("                $arg$comma")
            }
            sb.appendLine("            )")
            sb.appendLine("        }")
        } else if (isPaginated) {
            val entitySchema = op.responseSchemaRef?.let { api.findUnderlyingEntitySchema(it, op.paginationKey) }
            val entityType = entitySchema?.let { TYPE_ALIASES[it] } ?: "JsonElement"

            // Convert custom options to PaginationOptions
            val hasOptionalQuery = op.queryParams.any { !it.required }
            val optionsArg = if (hasOptionalQuery) "${optionsAccess(op)}toPaginationOptions()" else "options"

            sb.appendLine("        return requestPaginated(info, $optionsArg, {")
            sb.appendLine("            httpGet($pathWithQuery, operationName = info.operation)")
            sb.appendLine("        }) { body ->")
            sb.appendLine("            json.decodeFromString<List<$entityType>>(body)")
            sb.appendLine("        }")
        } else if (op.returnsVoid) {
            sb.appendLine("        request(info, {")
            sb.append(generateHttpCall(op, pathWithQuery))
            sb.appendLine("        }) { Unit }")
        } else {
            val entitySchema = op.responseSchemaRef?.let { api.findUnderlyingEntitySchema(it, op.paginationKey) }
            val entityType = entitySchema?.let { TYPE_ALIASES[it] }
            val decodeType = when {
                arrayEnvelopeMembers(op) != null -> buildWrappedResultClassName(op)
                entityType != null && op.returnsArray -> "List<$entityType>"
                entityType != null -> entityType
                else -> "JsonElement"
            }

            sb.appendLine("        return request(info, {")
            sb.append(generateHttpCall(op, pathWithQuery))
            sb.appendLine("        }) { body ->")
            sb.appendLine("            json.decodeFromString<$decodeType>(body)")
            sb.appendLine("        }")
        }

        sb.appendLine("    }")

        sb.append(generatePaginationOptionsOverload(op, returnType))

        return sb.toString()
    }

    /**
     * Emits the source-compatibility overload for an operation that gained its
     * first optional query parameter.
     *
     * Such an operation used to take `options: PaginationOptions? = null` and now
     * needs an `<Operation>Options` to carry its query parameters. Replacing the
     * parameter outright would be a source break, which the pre-1.0 policy in
     * kotlin/README.md forbids, so the OLD signature is kept verbatim as the
     * defaulted overload and the new options class arrives beside it. Every call
     * shape that compiled before still compiles: `list(id)`, `list(id, null)`,
     * `list(id, PaginationOptions(maxItems = 3))`, and — the reason the parameter
     * stays nullable — `list(id, aPaginationOptionsVariable)` whatever its
     * nullability.
     *
     * The new options class is therefore non-null and undefaulted: two defaulted
     * one-argument candidates would make a bare `list(id)` ambiguous. A caller
     * wanting "no options" uses the compatibility overload, which is what the
     * default already does.
     *
     * Emitted ONLY for the operations that actually made that move, listed in
     * [PAGINATION_OPTIONS_COMPAT_OVERLOADS]. An operation that already had its
     * own options class needs no bridge, and emitting one anyway would leave two
     * applicable one-argument candidates — enough to make an untyped callable
     * reference like `client.bookmarks::listMyBookmarks` ambiguous.
     */
    private fun generatePaginationOptionsOverload(op: ParsedOperation, returnType: String): String {
        if (op.operationId !in PAGINATION_OPTIONS_COMPAT_OVERLOADS) return ""

        val hasOptionalQuery = op.queryParams.any { !it.required }
        val hasPagination = op.hasPagination && op.returnsArray
        val isWrappedPaginated = op.hasPagination && op.paginationKey != null && !op.returnsArray
        if (!hasOptionalQuery || !(hasPagination || isWrappedPaginated)) return ""

        val optionsClassName = "${op.operationId}Options"
        val leading = mutableListOf<String>()
        for (p in op.pathParams) {
            leading += p.name.snakeToCamelCase()
        }
        for (q in op.queryParams.filter { it.required }) {
            leading += q.name.snakeToCamelCase()
        }
        val declared = buildParams(op).split(", ").filter { it.isNotEmpty() }
        // Reuse the primary signature minus its trailing options parameter.
        val paramDecls = declared.dropLast(1) + "options: PaginationOptions? = null"
        // `page` rides across too when the operation declares one: PaginationOptions
        // gained it in #566, and dropping it here would hand the compat overload
        // the exact bug that issue fixed — a pinned page auto-paginating the whole
        // collection because neither the query string nor the pagination options
        // ever saw it.
        val pageArg = if (op.queryParams.any { !it.required && it.name == "page" }) ", page = options?.page" else ""
        val forwarded = (leading + "$optionsClassName(maxItems = options?.maxItems$pageArg)").joinToString(", ")

        val sb = StringBuilder()
        sb.appendLine()
        sb.appendLine("    /**")
        sb.appendLine("     * Source-compatibility overload: the signature this operation had before")
        sb.appendLine("     * it gained query parameters of its own.")
        sb.appendLine("     *")
        sb.appendLine("     * Prefer [$optionsClassName], which also carries this operation's query")
        sb.appendLine("     * parameters. This overload forwards maxItems and leaves them unset.")
        sb.appendLine("     *")
        sb.appendLine("     * Because two candidates now apply, an *untyped* callable reference to")
        sb.appendLine("     * [${op.methodName}] needs an expected type to disambiguate.")
        sb.appendLine("     */")
        sb.appendLine("    suspend fun ${op.methodName}(${paramDecls.joinToString(", ")}): $returnType =")
        sb.appendLine("        ${op.methodName}($forwarded)")
        return sb.toString()
    }

    /**
     * How the generated body reaches into `options`: safely for the usual
     * nullable parameter, directly for an operation carrying a
     * PaginationOptions bridge, whose own options class is non-null. A safe call
     * on a non-null receiver is a warning, and the SDK builds with
     * allWarningsAsErrors.
     */
    private fun optionsAccess(op: ParsedOperation): String =
        if (op.operationId in PAGINATION_OPTIONS_COMPAT_OVERLOADS) "options." else "options?."

    /**
     * Generates query string building code that calls BaseService.buildQueryString().
     * E.g.:
     *     val qs = buildQueryString(
     *         "query" to query,
     *         "sort" to options?.sort,
     *     )
     */
    private fun generateQueryBuilding(op: ParsedOperation): String {
        val sb = StringBuilder()
        sb.appendLine("        val qs = buildQueryString(")
        for (q in op.queryParams) {
            val camelName = q.name.snakeToCamelCase()
            val accessor = if (q.required) camelName else "${optionsAccess(op)}$camelName"
            sb.appendLine("            \"${q.name}\" to $accessor,")
        }
        sb.appendLine("        )")
        return sb.toString()
    }

    private fun generateHttpCall(op: ParsedOperation, pathWithQuery: String): String {
        val sb = StringBuilder()

        when (op.httpMethod) {
            "GET" -> sb.appendLine("            httpGet($pathWithQuery, operationName = info.operation)")
            "POST" -> {
                if (op.bodyContentType == "octet-stream") {
                    sb.appendLine("            httpPostBinary($pathWithQuery, data, contentType)")
                } else if (op.bodyContentType == "json" && op.bodyProperties.isNotEmpty()) {
                    sb.appendLine("            httpPost($pathWithQuery, json.encodeToString(${buildBodySerializer(op)}), operationName = info.operation)")
                } else {
                    sb.appendLine("            httpPost($pathWithQuery, operationName = info.operation)")
                }
            }
            "PUT" -> {
                if (op.bodyContentType == "multipart") {
                    val field = op.multipartFieldName ?: "file"
                    sb.appendLine("            httpPutMultipart($pathWithQuery, \"$field\", data, filename, contentType)")
                } else {
                    val bodyArg = if (op.bodyContentType == "json" && op.bodyProperties.isNotEmpty()) {
                        ", json.encodeToString(${buildBodySerializer(op)})"
                    } else {
                        ""
                    }
                    sb.appendLine("            httpPut($pathWithQuery$bodyArg, operationName = info.operation)")
                }
            }
            "DELETE" -> sb.appendLine("            httpDelete($pathWithQuery, operationName = info.operation)")
            "PATCH" -> {
                val bodyArg = if (op.bodyContentType == "json" && op.bodyProperties.isNotEmpty()) {
                    ", json.encodeToString(${buildBodySerializer(op)})"
                } else {
                    ""
                }
                sb.appendLine("            httpPut($pathWithQuery$bodyArg, operationName = info.operation)")
            }
        }

        return sb.toString()
    }

    private fun buildPathExpression(op: ParsedOperation): String {
        // Replace path params like {projectId} with $projectId
        var path = op.path
        for (p in op.pathParams) {
            path = path.replace("{${p.name}}", "\${${p.name.snakeToCamelCase()}}")
        }
        return "\"$path\""
    }

    private fun buildBodySerializer(op: ParsedOperation): String {
        // Build a JsonObject from the body properties
        val props = op.bodyProperties
        if (props.isEmpty()) return "kotlinx.serialization.json.JsonObject(emptyMap())"

        val sb = StringBuilder()
        sb.appendLine("kotlinx.serialization.json.buildJsonObject {")
        for (p in props) {
            val camelName = p.name.snakeToCamelCase()
            val accessor = "body.$camelName"
            when {
                !p.required -> {
                    sb.appendLine("                $accessor?.let { put(\"${p.name}\", ${jsonPutExpression(p.type, "it")}) }")
                }
                else -> {
                    sb.appendLine("                put(\"${p.name}\", ${jsonPutExpression(p.type, accessor)})")
                }
            }
        }
        sb.append("            }")
        return sb.toString()
    }

    private fun jsonPutExpression(type: String, accessor: String): String = when (type) {
        "String" -> "kotlinx.serialization.json.JsonPrimitive($accessor)"
        "Int", "Long" -> "kotlinx.serialization.json.JsonPrimitive($accessor)"
        "Boolean" -> "kotlinx.serialization.json.JsonPrimitive($accessor)"
        "Double" -> "kotlinx.serialization.json.JsonPrimitive($accessor)"
        "JsonObject" -> "$accessor"
        else -> {
            if (type == "List<JsonObject>") {
                "kotlinx.serialization.json.JsonArray($accessor)"
            } else if (type.startsWith("List<")) {
                "kotlinx.serialization.json.JsonArray($accessor.map { kotlinx.serialization.json.JsonPrimitive(it) })"
            } else {
                "kotlinx.serialization.json.JsonPrimitive($accessor.toString())"
            }
        }
    }

    private fun buildReturnType(op: ParsedOperation): String {
        if (op.returnsVoid) return "Unit"

        // Wrapped pagination returns a result data class
        if (op.hasPagination && op.paginationKey != null && !op.returnsArray) {
            return buildWrappedResultClassName(op)
        }

        // An object-of-arrays envelope returns its own result data class
        if (arrayEnvelopeMembers(op) != null) {
            return buildWrappedResultClassName(op)
        }

        val entitySchema = op.responseSchemaRef?.let { api.findUnderlyingEntitySchema(it, op.paginationKey) }
        val entityType = entitySchema?.let { TYPE_ALIASES[it] }

        return when {
            entityType != null && op.returnsArray && op.hasPagination -> "ListResult<$entityType>"
            entityType != null && op.returnsArray -> "List<$entityType>"
            op.returnsArray && op.hasPagination -> "ListResult<JsonElement>"
            entityType != null -> entityType
            else -> "JsonElement"
        }
    }

    private fun buildParams(op: ParsedOperation): String {
        val parts = mutableListOf<String>()

        // Path params
        for (p in op.pathParams) {
            parts += "${p.name.snakeToCamelCase()}: ${p.type}"
        }

        // Body param
        if (op.bodyContentType == "json" && op.bodyProperties.isNotEmpty()) {
            val bodyClassName = buildBodyClassName(op)
            parts += "body: $bodyClassName"
        }

        // Binary upload
        if (op.bodyContentType == "octet-stream") {
            parts += "data: ByteArray"
            parts += "contentType: String"
        }

        // Multipart file upload
        if (op.bodyContentType == "multipart") {
            parts += "data: ByteArray"
            parts += "filename: String"
            parts += "contentType: String"
        }

        // Required query params
        for (q in op.queryParams.filter { it.required }) {
            parts += "${q.name.snakeToCamelCase()}: ${q.type}"
        }

        // Optional: query params + pagination
        val hasOptionalQuery = op.queryParams.any { !it.required }
        val hasPagination = op.hasPagination && op.returnsArray
        val isWrappedPaginated = op.hasPagination && op.paginationKey != null && !op.returnsArray
        if (hasOptionalQuery || hasPagination || isWrappedPaginated) {
            val optionsClassName = buildOptionsClassName(op, hasPagination || isWrappedPaginated, hasOptionalQuery)
            // An operation that kept a PaginationOptions bridge keeps the OLD
            // signature — `options: PaginationOptions? = null` — as the defaulted
            // one, so every call shape that compiled before still does, `null`
            // literal and nullable variable alike. Its own options class then has
            // to arrive non-null and undefaulted, or a bare `list(id)` would have
            // two applicable candidates. See generatePaginationOptionsOverload.
            parts += if (op.operationId in PAGINATION_OPTIONS_COMPAT_OVERLOADS) {
                "options: $optionsClassName"
            } else {
                "options: $optionsClassName? = null"
            }
        }

        return parts.joinToString(", ")
    }

    private fun buildBodyClassName(op: ParsedOperation): String =
        "${op.operationId}Body"

    private fun buildOptionsClassName(op: ParsedOperation, hasPagination: Boolean, hasOptionalQuery: Boolean): String =
        when {
            hasPagination && !hasOptionalQuery -> "PaginationOptions"
            else -> "${op.operationId}Options"
        }

    private fun enrichDescription(desc: String): String {
        var result = desc.replace(Regex("""\s*\(returns \d+ [^)]+\)"""), "")
        if (result.startsWith("Trash ", ignoreCase = true) && !result.contains("can be recovered", ignoreCase = true)) {
            result += ". Trashed items can be recovered."
        }
        return result
    }

    /**
     * Builds a result class name for wrapped pagination operations.
     * E.g., "GetPersonProgress" → "PersonProgressResult"
     */
    private fun buildWrappedResultClassName(op: ParsedOperation): String {
        val base = op.operationId
            .removePrefix("Get")
            .removePrefix("List")
        return "${base}Result"
    }

    /**
     * Generates a data class for wrapped pagination results.
     * Wrapper fields get their resolved types; the pagination key gets ListResult<EntityType>.
     */
    private fun generateWrappedResultClass(op: ParsedOperation): String {
        val sb = StringBuilder()
        val className = buildWrappedResultClassName(op)
        val schema = api.getSchema(op.responseSchemaRef!!) ?: return ""
        val properties = schema["properties"]?.jsonObject ?: return ""

        val entitySchema = api.findUnderlyingEntitySchema(op.responseSchemaRef, op.paginationKey)
        val entityType = entitySchema?.let { TYPE_ALIASES[it] } ?: "JsonElement"

        sb.appendLine("data class $className(")
        val propNames = properties.keys.sorted()
        for ((i, propName) in propNames.withIndex()) {
            val camelName = propName.snakeToCamelCase()
            val comma = if (i < propNames.size - 1) "," else ""
            if (propName == op.paginationKey) {
                sb.appendLine("    val $camelName: ListResult<$entityType>$comma")
            } else {
                val propType = resolveWrapperPropertyType(op.responseSchemaRef!!, propName)
                sb.appendLine("    val $camelName: $propType$comma")
            }
        }
        sb.appendLine(")")
        sb.appendLine()

        return sb.toString()
    }

    /**
     * Members of an object-of-arrays response envelope, in wire order, mapped to
     * their Kotlin element types — or null when this operation is not one.
     *
     * Requires all four of: the operation is opted in via
     * [TYPED_ARRAY_ENVELOPE_OPERATIONS]; it is not already served by the
     * pagination path; its response schema is an object with at least one
     * property; and EVERY property is an array whose items `$ref` a schema in
     * [TYPE_ALIASES]. Anything less falls back to the JsonElement path rather
     * than emitting a class that cannot decode the body.
     */
    private fun arrayEnvelopeMembers(op: ParsedOperation): Map<String, String>? {
        if (op.operationId !in TYPED_ARRAY_ENVELOPE_OPERATIONS) return null
        if (op.returnsVoid || op.returnsArray || op.hasPagination) return null

        val schema = op.responseSchemaRef?.let { api.getSchema(it) } ?: return null
        if (schema["type"]?.jsonPrimitive?.contentOrNull != "object") return null
        val properties = schema["properties"]?.jsonObject ?: return null
        if (properties.isEmpty()) return null

        val members = LinkedHashMap<String, String>()
        for ((propName, propValue) in properties) {
            val prop = propValue as? JsonObject ?: return null
            if (prop["type"]?.jsonPrimitive?.contentOrNull != "array") return null
            val itemsRef = prop["items"]?.jsonObject?.get("\$ref")?.jsonPrimitive?.contentOrNull ?: return null
            val entityType = TYPE_ALIASES[api.resolveRef(itemsRef)] ?: return null
            members[propName] = entityType
        }
        return members
    }

    /**
     * Generates the `@Serializable` data class an object-of-arrays envelope
     * decodes into. Every member is required and non-null: the arrays this
     * covers are `@required` in the spec because BC3 writes all of them
     * unconditionally, so an absent key is a contract violation worth failing on
     * rather than a shape to paper over with a default.
     */
    private fun generateArrayEnvelopeResultClass(op: ParsedOperation, members: Map<String, String>): String {
        val sb = StringBuilder()
        sb.appendLine("@Serializable")
        sb.appendLine("data class ${buildWrappedResultClassName(op)}(")
        val entries = members.entries.toList()
        for ((i, entry) in entries.withIndex()) {
            val camelName = entry.key.snakeToCamelCase()
            val serialName = if (camelName == entry.key) "" else "@SerialName(\"${entry.key}\") "
            val comma = if (i < entries.size - 1) "," else ""
            sb.appendLine("    ${serialName}val $camelName: List<${entry.value}>$comma")
        }
        sb.appendLine(")")
        sb.appendLine()
        return sb.toString()
    }

    /**
     * Resolves a wrapper property's type from the response schema.
     * Uses TYPE_ALIASES for known entity $refs, falls back to JsonElement.
     */
    private fun resolveWrapperPropertyType(schemaRef: String, propName: String): String {
        val schema = api.getSchema(schemaRef) ?: return "JsonElement"
        val propObj = schema["properties"]?.jsonObject?.get(propName)?.jsonObject ?: return "JsonElement"

        // Direct $ref to a known entity
        val ref = propObj["\$ref"]?.jsonPrimitive?.contentOrNull
        if (ref != null) {
            val refName = api.resolveRef(ref)
            return TYPE_ALIASES[refName] ?: "JsonElement"
        }

        // Primitive types
        return api.schemaToKotlinType(propObj)
    }
}

private fun String.toHumanReadable(): String {
    if (endsWith("Id")) {
        return removeSuffix("Id")
            .replace(Regex("([a-z])([A-Z])"), "$1 $2")
            .lowercase() + " ID"
    }
    return replace("_", " ")
        .replace(Regex("([a-z])([A-Z])"), "$1 $2")
        .lowercase()
}
