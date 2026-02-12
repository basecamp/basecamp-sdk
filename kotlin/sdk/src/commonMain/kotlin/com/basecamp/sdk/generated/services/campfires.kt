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
     * Get a campfire by ID
     * @param projectId The project ID
     * @param campfireId The campfire ID
     */
    suspend fun get(projectId: Long, campfireId: Long): Campfire {
        val info = OperationInfo(
            service = "Campfires",
            operation = "GetCampfire",
            resourceType = "campfire",
            isMutation = false,
            projectId = projectId,
            resourceId = campfireId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/chats/${campfireId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Campfire>(body)
        }
    }

    /**
     * List all chatbots for a campfire
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listChatbots(projectId: Long, campfireId: Long, options: PaginationOptions? = null): ListResult<Chatbot> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListChatbots",
            resourceType = "chatbot",
            isMutation = false,
            projectId = projectId,
            resourceId = campfireId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/chats/${campfireId}/integrations.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Chatbot>>(body)
        }
    }

    /**
     * Create a new chatbot for a campfire
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param body Request body
     */
    suspend fun createChatbot(projectId: Long, campfireId: Long, body: CreateChatbotBody): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "CreateChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = projectId,
            resourceId = campfireId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/chats/${campfireId}/integrations.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("service_name", kotlinx.serialization.json.JsonPrimitive(body.serviceName))
                body.commandUrl?.let { put("command_url", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Get a chatbot by ID
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     */
    suspend fun getChatbot(projectId: Long, campfireId: Long, chatbotId: Long): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "GetChatbot",
            resourceType = "chatbot",
            isMutation = false,
            projectId = projectId,
            resourceId = campfireId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/chats/${campfireId}/integrations/${chatbotId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Update an existing chatbot
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     * @param body Request body
     */
    suspend fun updateChatbot(projectId: Long, campfireId: Long, chatbotId: Long, body: UpdateChatbotBody): Chatbot {
        val info = OperationInfo(
            service = "Campfires",
            operation = "UpdateChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = projectId,
            resourceId = campfireId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/chats/${campfireId}/integrations/${chatbotId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("service_name", kotlinx.serialization.json.JsonPrimitive(body.serviceName))
                body.commandUrl?.let { put("command_url", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Chatbot>(body)
        }
    }

    /**
     * Delete a chatbot
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param chatbotId The chatbot ID
     */
    suspend fun deleteChatbot(projectId: Long, campfireId: Long, chatbotId: Long): Unit {
        val info = OperationInfo(
            service = "Campfires",
            operation = "DeleteChatbot",
            resourceType = "chatbot",
            isMutation = true,
            projectId = projectId,
            resourceId = campfireId,
        )
        request(info, {
            httpDelete("/buckets/${projectId}/chats/${campfireId}/integrations/${chatbotId}", operationName = info.operation)
        }) { Unit }
    }

    /**
     * List all lines (messages) in a campfire
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listLines(projectId: Long, campfireId: Long, options: PaginationOptions? = null): ListResult<CampfireLine> {
        val info = OperationInfo(
            service = "Campfires",
            operation = "ListCampfireLines",
            resourceType = "campfire_line",
            isMutation = false,
            projectId = projectId,
            resourceId = campfireId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${projectId}/chats/${campfireId}/lines.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<CampfireLine>>(body)
        }
    }

    /**
     * Create a new line (message) in a campfire
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param body Request body
     */
    suspend fun createLine(projectId: Long, campfireId: Long, body: CreateCampfireLineBody): CampfireLine {
        val info = OperationInfo(
            service = "Campfires",
            operation = "CreateCampfireLine",
            resourceType = "campfire_line",
            isMutation = true,
            projectId = projectId,
            resourceId = campfireId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/chats/${campfireId}/lines.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("content", kotlinx.serialization.json.JsonPrimitive(body.content))
                body.contentType?.let { put("content_type", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<CampfireLine>(body)
        }
    }

    /**
     * Get a campfire line by ID
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param lineId The line ID
     */
    suspend fun getLine(projectId: Long, campfireId: Long, lineId: Long): CampfireLine {
        val info = OperationInfo(
            service = "Campfires",
            operation = "GetCampfireLine",
            resourceType = "campfire_line",
            isMutation = false,
            projectId = projectId,
            resourceId = campfireId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/chats/${campfireId}/lines/${lineId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<CampfireLine>(body)
        }
    }

    /**
     * Delete a campfire line
     * @param projectId The project ID
     * @param campfireId The campfire ID
     * @param lineId The line ID
     */
    suspend fun deleteLine(projectId: Long, campfireId: Long, lineId: Long): Unit {
        val info = OperationInfo(
            service = "Campfires",
            operation = "DeleteCampfireLine",
            resourceType = "campfire_line",
            isMutation = true,
            projectId = projectId,
            resourceId = campfireId,
        )
        request(info, {
            httpDelete("/buckets/${projectId}/chats/${campfireId}/lines/${lineId}", operationName = info.operation)
        }) { Unit }
    }

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
}
