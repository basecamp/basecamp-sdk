package com.basecamp.sdk.generator

import kotlinx.serialization.json.*

/**
 * A parsed API operation with all the information needed for code generation.
 */
data class ParsedOperation(
    val operationId: String,
    val methodName: String,
    val httpMethod: String,
    val path: String,
    val description: String,
    val pathParams: List<PathParam>,
    val queryParams: List<QueryParam>,
    val bodySchemaRef: String?,
    val bodyProperties: List<BodyProperty>,
    val bodyRequired: Boolean,
    val bodyContentType: String?,
    val responseSchemaRef: String?,
    val returnsArray: Boolean,
    val returnsVoid: Boolean,
    val isMutation: Boolean,
    val resourceType: String,
    val hasPagination: Boolean,
)

data class PathParam(val name: String, val type: String, val description: String?)
data class QueryParam(val name: String, val type: String, val required: Boolean, val description: String?)
data class BodyProperty(val name: String, val type: String, val required: Boolean, val description: String?, val formatHint: String?)

data class ServiceDefinition(
    val name: String,
    val className: String,
    val operations: MutableList<ParsedOperation>,
    val entityTypes: MutableSet<String>,
)

/**
 * Parses operations from the OpenAPI spec.
 */
class OperationParser(private val api: OpenApiParser) {

    fun extractMethodName(operationId: String): String {
        METHOD_NAME_OVERRIDES[operationId]?.let { return it }

        for ((prefix, method) in VERB_PATTERNS) {
            if (operationId.startsWith(prefix)) {
                val remainder = operationId.removePrefix(prefix)
                if (remainder.isEmpty()) return method
                val resource = remainder[0].lowercase() + remainder.substring(1)
                if (resource.lowercase() in SIMPLE_RESOURCES) return method
                return if (method == "get") resource[0].lowercase() + resource.substring(1) else method + remainder
            }
        }

        return operationId[0].lowercase() + operationId.substring(1)
    }

    fun extractResourceType(operationId: String): String {
        for ((prefix, _) in VERB_PATTERNS) {
            if (operationId.startsWith(prefix)) {
                val remainder = operationId.removePrefix(prefix)
                if (remainder.isEmpty()) return "resource"
                return remainder.toSnakeCase().singularize()
            }
        }
        return "resource"
    }

    fun parseOperation(path: String, method: String, operation: JsonObject): ParsedOperation {
        val httpMethod = method.uppercase()
        val operationId = operation["operationId"]!!.jsonPrimitive.content
        val methodName = extractMethodName(operationId)
        val description = operation["description"]?.jsonPrimitive?.content
            ?: operation["summary"]?.jsonPrimitive?.content
            ?: "$methodName operation"

        // Path parameters (skip accountId)
        val pathParams = (operation["parameters"]?.jsonArray ?: emptyList())
            .map { it.jsonObject }
            .filter { it["in"]!!.jsonPrimitive.content == "path" && it["name"]!!.jsonPrimitive.content != "accountId" }
            .map { param ->
                val name = param["name"]!!.jsonPrimitive.content
                val schema = param["schema"]!!.jsonObject
                val type = when (schema["type"]?.jsonPrimitive?.content) {
                    "integer" -> "Long"
                    else -> "String"
                }
                PathParam(name, type, param["description"]?.jsonPrimitive?.content)
            }

        // Query parameters
        val queryParams = (operation["parameters"]?.jsonArray ?: emptyList())
            .map { it.jsonObject }
            .filter { it["in"]!!.jsonPrimitive.content == "query" }
            .map { param ->
                val name = param["name"]!!.jsonPrimitive.content
                val schema = param["schema"]!!.jsonObject
                val type = when (schema["type"]?.jsonPrimitive?.content) {
                    "integer" -> "Long"
                    "boolean" -> "Boolean"
                    else -> "String"
                }
                QueryParam(
                    name,
                    type,
                    param["required"]?.jsonPrimitive?.boolean ?: false,
                    param["description"]?.jsonPrimitive?.content,
                )
            }

        // Request body
        var bodySchemaRef: String? = null
        var bodyProperties = emptyList<BodyProperty>()
        var bodyRequired = false
        var bodyContentType: String? = null

        val requestBody = operation["requestBody"]?.jsonObject
        val jsonContent = requestBody?.get("content")?.jsonObject?.get("application/json")?.jsonObject
        val octetContent = requestBody?.get("content")?.jsonObject?.get("application/octet-stream")?.jsonObject

        if (jsonContent != null) {
            val schema = jsonContent["schema"]!!.jsonObject
            bodyRequired = requestBody["required"]?.jsonPrimitive?.boolean ?: false
            bodyContentType = "json"
            val ref = schema["\$ref"]?.jsonPrimitive?.content
            if (ref != null) {
                bodySchemaRef = api.resolveRef(ref)
                bodyProperties = parseBodyProperties(bodySchemaRef)
            }
        } else if (octetContent != null) {
            bodyRequired = requestBody?.get("required")?.jsonPrimitive?.boolean ?: false
            bodyContentType = "octet-stream"
        }

        // Response
        var responseSchemaRef: String? = null
        var returnsArray = false
        val responses = operation["responses"]?.jsonObject
        val successResponse = responses?.get("200")?.jsonObject ?: responses?.get("201")?.jsonObject
        val responseSchema = successResponse?.get("content")?.jsonObject
            ?.get("application/json")?.jsonObject
            ?.get("schema")?.jsonObject

        if (responseSchema != null) {
            val ref = responseSchema["\$ref"]?.jsonPrimitive?.content
            if (ref != null) {
                responseSchemaRef = api.resolveRef(ref)
                // Check if the referenced schema is an array type
                val resolvedSchema = api.getSchema(responseSchemaRef)
                if (resolvedSchema?.get("type")?.jsonPrimitive?.content == "array") {
                    returnsArray = true
                }
            }
            if (responseSchema["type"]?.jsonPrimitive?.content == "array") {
                returnsArray = true
            }
        }

        val returnsVoid = isVoidResponse(responses)
        val isMutation = httpMethod != "GET"
        val resourceType = extractResourceType(operationId)
        val hasPagination = operation.containsKey("x-basecamp-pagination")

        val convertedPath = path.replace(Regex("^/\\{accountId}"), "")

        return ParsedOperation(
            operationId = operationId,
            methodName = methodName,
            httpMethod = httpMethod,
            path = convertedPath,
            description = description,
            pathParams = pathParams,
            queryParams = queryParams,
            bodySchemaRef = bodySchemaRef,
            bodyProperties = bodyProperties,
            bodyRequired = bodyRequired,
            bodyContentType = bodyContentType,
            responseSchemaRef = responseSchemaRef,
            returnsArray = returnsArray,
            returnsVoid = returnsVoid,
            isMutation = isMutation,
            resourceType = resourceType,
            hasPagination = hasPagination,
        )
    }

    private fun parseBodyProperties(schemaRef: String): List<BodyProperty> {
        val schema = api.getSchema(schemaRef) ?: return emptyList()
        val properties = schema["properties"]?.jsonObject ?: return emptyList()
        val required = schema["required"]?.jsonArray?.map { it.jsonPrimitive.content } ?: emptyList()

        return properties.entries.map { (name, prop) ->
            val propObj = prop.jsonObject
            val type = api.schemaToKotlinType(propObj)
            val desc = propObj["description"]?.jsonPrimitive?.content
            val goType = api.getGoType(propObj)
            val formatHint = when {
                goType == "types.Date" -> "YYYY-MM-DD"
                goType == "time.Time" || goType == "types.DateTime" -> "ISO 8601"
                propObj["format"]?.jsonPrimitive?.content == "date" -> "YYYY-MM-DD"
                propObj["format"]?.jsonPrimitive?.content == "date-time" -> "ISO 8601"
                else -> null
            }
            BodyProperty(name, type, name in required, desc, formatHint)
        }
    }

    private fun isVoidResponse(responses: JsonObject?): Boolean {
        if (responses == null) return true
        val successResponse = responses["200"]?.jsonObject
            ?: responses["201"]?.jsonObject
            ?: responses["204"]?.jsonObject
            ?: return true
        return successResponse["content"]?.jsonObject?.get("application/json") == null
    }

    /**
     * Groups operations into service definitions.
     */
    fun groupOperations(): Map<String, ServiceDefinition> {
        val services = mutableMapOf<String, ServiceDefinition>()

        for ((path, pathItem) in api.paths) {
            for (method in listOf("get", "post", "put", "patch", "delete")) {
                val operation = pathItem.jsonObject[method]?.jsonObject ?: continue
                val operationId = operation["operationId"]!!.jsonPrimitive.content
                val tag = operation["tags"]?.jsonArray?.firstOrNull()?.jsonPrimitive?.content ?: "Untagged"

                val parsed = parseOperation(path, method, operation)

                // Determine service name
                val serviceName = if (tag in SERVICE_SPLITS) {
                    var found: String? = null
                    for ((svc, opIds) in SERVICE_SPLITS[tag]!!) {
                        if (operationId in opIds) {
                            found = svc
                            break
                        }
                    }
                    found ?: (TAG_TO_SERVICE[tag] ?: tag.replace("\\s+".toRegex(), ""))
                } else {
                    TAG_TO_SERVICE[tag] ?: tag.replace("\\s+".toRegex(), "")
                }

                val service = services.getOrPut(serviceName) {
                    ServiceDefinition(
                        name = serviceName,
                        className = "${serviceName}Service",
                        operations = mutableListOf(),
                        entityTypes = mutableSetOf(),
                    )
                }

                service.operations.add(parsed)

                // Track entity types used
                if (parsed.responseSchemaRef != null) {
                    val entitySchema = api.findUnderlyingEntitySchema(parsed.responseSchemaRef)
                    if (entitySchema != null && entitySchema in TYPE_ALIASES) {
                        service.entityTypes.add(entitySchema)
                    }
                }
            }
        }

        return services
    }
}

// =============================================================================
// Utility extensions
// =============================================================================

/** "CardColumn" → "card_column" */
fun String.toSnakeCase(): String =
    replace(Regex("([A-Z])")) { "_${it.value.lowercase()}" }
        .removePrefix("_")

/** "card_columns" → "card_column", "entries" → "entry" */
fun String.singularize(): String = when {
    endsWith("ss") -> this  // "progress", "address", "access"
    endsWith("ies") -> removeSuffix("ies") + "y"
    endsWith("ses") -> removeSuffix("es")
    endsWith("s") -> removeSuffix("s")
    else -> this
}

/** "CardColumn" → "card-column" */
fun String.toKebabCase(): String =
    replace(Regex("([a-z])([A-Z])")) { "${it.groupValues[1]}-${it.groupValues[2]}" }
        .replace(Regex("([A-Z]+)([A-Z][a-z])")) { "${it.groupValues[1]}-${it.groupValues[2]}" }
        .lowercase()

/** "snake_case" → "camelCase" */
fun String.snakeToCamelCase(): String =
    replace(Regex("_([a-z])")) { it.groupValues[1].uppercase() }

fun String.capitalize(): String =
    replaceFirstChar { it.uppercase() }
