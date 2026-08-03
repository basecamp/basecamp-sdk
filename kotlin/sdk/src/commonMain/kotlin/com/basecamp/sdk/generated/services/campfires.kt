package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Campfires operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class CampfiresService(client: AccountClient) : BaseService(client) {

    /**
     * List all chatbots for a campfire
     * @param bucketId The bucket ID
     * @param campfireId The campfire ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listChatbots(bucketId: Long, campfireId: Long, options: PaginationOptions? = null): ListResult<Chatbot> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListChatbots",
            resourceType = "chatbot",
            isMutation = false,
            projectId = bucketId,
            resourceId = campfireId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${bucketId}/chats/${campfireId}/integrations.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Chatbot>>(body)
        }
    }

    /**
     * Create a new chatbot for a campfire
     * @param bucketId The bucket ID
     * @param campfireId The campfire ID
     * @param body Request body
     */
    suspend fun createChatbot(bucketId: Long, campfireId: Long, body: CreateChatbotBody): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "CreateChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = bucketId,
            resourceId = campfireId,
        )
        return request(info, {
            httpPost("/buckets/${bucketId}/chats/${campfireId}/integrations.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("service_name", kotlinx.serialization.json.JsonPrimitive(body.serviceName))
                body.commandUrl?.let { put("command_url", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Get a chatbot by ID
     * @param bucketId The bucket ID
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     */
    suspend fun getChatbot(bucketId: Long, campfireId: Long, chatbotId: Long): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "GetChatbot",
            resourceType = "chatbot",
            isMutation = false,
            projectId = bucketId,
            resourceId = chatbotId,
        )
        return request(info, {
            httpGet("/buckets/${bucketId}/chats/${campfireId}/integrations/${chatbotId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Update an existing chatbot
     * @param bucketId The bucket ID
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     * @param body Request body
     */
    suspend fun updateChatbot(bucketId: Long, campfireId: Long, chatbotId: Long, body: UpdateChatbotBody): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "UpdateChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = bucketId,
            resourceId = chatbotId,
        )
        return request(info, {
            httpPut("/buckets/${bucketId}/chats/${campfireId}/integrations/${chatbotId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("service_name", kotlinx.serialization.json.JsonPrimitive(body.serviceName))
                body.commandUrl?.let { put("command_url", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Delete a chatbot
     * @param bucketId The bucket ID
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     */
    suspend fun deleteChatbot(bucketId: Long, campfireId: Long, chatbotId: Long): Unit {
        val info = OperationInfo(
            service = "Campfires",
            operation = "DeleteChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = bucketId,
            resourceId = chatbotId,
        )
        request(info, {
            httpDelete("/buckets/${bucketId}/chats/${campfireId}/integrations/${chatbotId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * List all campfires across the account
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: ListCampfiresOptions): ListResult<Campfire> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListCampfires",
            resourceType = "campfire",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/chats.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Campfire>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [ListCampfiresOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [list] needs an expected type to disambiguate.
     */
    suspend fun list(options: PaginationOptions? = null): ListResult<Campfire> =
        list(ListCampfiresOptions(maxItems = options?.maxItems))

    /**
     * Get a campfire by ID
     * @param campfireId The campfire ID
     */
    suspend fun get(campfireId: Long): Campfire {
        val info = OperationInfo(
            service = "Campfires",
            operation = "GetCampfire",
            resourceType = "campfire",
            isMutation = false,
            projectId = null,
            resourceId = campfireId,
        )
        return request(info, {
            httpGet("/chats/${campfireId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Campfire>(body)
        }
    }

    /**
     * List all lines (messages) in a campfire
     * @param campfireId The campfire ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listLines(campfireId: Long, options: ListCampfireLinesOptions? = null): ListResult<CampfireLine> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListCampfireLines",
            resourceType = "campfire_line",
            isMutation = false,
            projectId = null,
            resourceId = campfireId,
        )
        val qs = buildQueryString(
            "sort" to options?.sort,
            "direction" to options?.direction,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/chats/${campfireId}/lines.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<CampfireLine>>(body)
        }
    }

    /**
     * Create a new line (message) in a campfire
     * @param campfireId The campfire ID
     * @param body Request body
     */
    suspend fun createLine(campfireId: Long, body: CreateCampfireLineBody): CampfireLine {
        val info = OperationInfo(
            service = "Campfires",
            operation = "CreateCampfireLine",
            resourceType = "campfire_line",
            isMutation = true,
            projectId = null,
            resourceId = campfireId,
        )
        return request(info, {
            httpPost("/chats/${campfireId}/lines.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
                body.contentType?.let { put("content_type", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CampfireLine>(body)
        }
    }

    /**
     * Get a campfire line by ID
     * @param campfireId The campfire ID
     * @param lineId The line ID
     */
    suspend fun getLine(campfireId: Long, lineId: Long): CampfireLine {
        val info = OperationInfo(
            service = "Campfires",
            operation = "GetCampfireLine",
            resourceType = "campfire_line",
            isMutation = false,
            projectId = null,
            resourceId = lineId,
        )
        return request(info, {
            httpGet("/chats/${campfireId}/lines/${lineId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CampfireLine>(body)
        }
    }

    /**
     * Update an existing campfire line; the content is always treated as rich text (HTML).
     * @param campfireId The campfire ID
     * @param lineId The line ID
     * @param body Request body
     */
    suspend fun updateLine(campfireId: Long, lineId: Long, body: UpdateCampfireLineBody): Unit {
        val info = OperationInfo(
            service = "Campfires",
            operation = "UpdateCampfireLine",
            resourceType = "campfire_line",
            isMutation = true,
            projectId = null,
            resourceId = lineId,
        )
        request(info, {
            httpPut("/chats/${campfireId}/lines/${lineId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * Delete a campfire line; allowed for the line's creator or an admin.
     * @param campfireId The campfire ID
     * @param lineId The line ID
     */
    suspend fun deleteLine(campfireId: Long, lineId: Long): Unit {
        val info = OperationInfo(
            service = "Campfires",
            operation = "DeleteCampfireLine",
            resourceType = "campfire_line",
            isMutation = true,
            projectId = null,
            resourceId = lineId,
        )
        request(info, {
            httpDelete("/chats/${campfireId}/lines/${lineId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * List uploaded files in a campfire
     * @param campfireId The campfire ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listUploads(campfireId: Long, options: ListCampfireUploadsOptions? = null): ListResult<CampfireLine> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListCampfireUploads",
            resourceType = "campfire_upload",
            isMutation = false,
            projectId = null,
            resourceId = campfireId,
        )
        val qs = buildQueryString(
            "sort" to options?.sort,
            "direction" to options?.direction,
            "page" to options?.page,
        )
        return requestPaginated(info, options?.toPaginationOptions(), {
            httpGet("/chats/${campfireId}/uploads.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<CampfireLine>>(body)
        }
    }

    /**
     * Upload a file to a campfire
     * @param campfireId The campfire ID
     * @param data Binary file data to upload
     * @param contentType MIME type of the file
     * @param name Filename for the uploaded file (e.g. "report.pdf").
     */
    suspend fun createUpload(campfireId: Long, data: ByteArray, contentType: String, name: String): CampfireLine {
        val info = OperationInfo(
            service = "Campfires",
            operation = "CreateCampfireUpload",
            resourceType = "campfire_upload",
            isMutation = true,
            projectId = null,
            resourceId = campfireId,
        )
        val qs = buildQueryString(
            "name" to name,
        )
        return request(info, {
            httpPostBinary("/chats/${campfireId}/uploads.json" + qs, data, contentType)
        }) { body ->
            json.decodeFromString<CampfireLine>(body)
        }
    }
}
