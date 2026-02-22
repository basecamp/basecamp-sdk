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
     * List all campfires across the account
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: PaginationOptions? = null): ListResult<Campfire> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListCampfires",
            resourceType = "campfire",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/chats.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Campfire>>(body)
        }
    }

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
     * List all chatbots for a campfire
     * @param campfireId The campfire ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listChatbots(campfireId: Long, options: PaginationOptions? = null): ListResult<Chatbot> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListChatbots",
            resourceType = "chatbot",
            isMutation = false,
            projectId = null,
            resourceId = campfireId,
        )
        return requestPaginated(info, options, {
            httpGet("/chats/${campfireId}/integrations.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Chatbot>>(body)
        }
    }

    /**
     * Create a new chatbot for a campfire
     * @param campfireId The campfire ID
     * @param body Request body
     */
    suspend fun createChatbot(campfireId: Long, body: CreateChatbotBody): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "CreateChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = null,
            resourceId = campfireId,
        )
        return request(info, {
            httpPost("/chats/${campfireId}/integrations.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("service_name", kotlinx.serialization.json.JsonPrimitive(body.serviceName))
                body.commandUrl?.let { put("command_url", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Get a chatbot by ID
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     */
    suspend fun getChatbot(campfireId: Long, chatbotId: Long): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "GetChatbot",
            resourceType = "chatbot",
            isMutation = false,
            projectId = null,
            resourceId = campfireId,
        )
        return request(info, {
            httpGet("/chats/${campfireId}/integrations/${chatbotId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Update an existing chatbot
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     * @param body Request body
     */
    suspend fun updateChatbot(campfireId: Long, chatbotId: Long, body: UpdateChatbotBody): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "UpdateChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = null,
            resourceId = campfireId,
        )
        return request(info, {
            httpPut("/chats/${campfireId}/integrations/${chatbotId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("service_name", kotlinx.serialization.json.JsonPrimitive(body.serviceName))
                body.commandUrl?.let { put("command_url", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Delete a chatbot
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     */
    suspend fun deleteChatbot(campfireId: Long, chatbotId: Long): Unit {
        val info = OperationInfo(
            service = "Campfires",
            operation = "DeleteChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = null,
            resourceId = campfireId,
        )
        request(info, {
            httpDelete("/chats/${campfireId}/integrations/${chatbotId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * List all lines (messages) in a campfire
     * @param campfireId The campfire ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listLines(campfireId: Long, options: PaginationOptions? = null): ListResult<CampfireLine> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListCampfireLines",
            resourceType = "campfire_line",
            isMutation = false,
            projectId = null,
            resourceId = campfireId,
        )
        return requestPaginated(info, options, {
            httpGet("/chats/${campfireId}/lines.json", operationName = info.operation)
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
            resourceId = campfireId,
        )
        return request(info, {
            httpGet("/chats/${campfireId}/lines/${lineId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CampfireLine>(body)
        }
    }

    /**
     * Delete a campfire line
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
            resourceId = campfireId,
        )
        request(info, {
            httpDelete("/chats/${campfireId}/lines/${lineId}", operationName = info.operation)
        }) { Unit }
    }
}
