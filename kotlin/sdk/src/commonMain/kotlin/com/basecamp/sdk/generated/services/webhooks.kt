package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Webhooks operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class WebhooksService(client: AccountClient) : BaseService(client) {

    /**
     * List all webhooks for a project
     * @param bucketId The bucket ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(bucketId: Long, options: PaginationOptions? = null): ListResult<Webhook> {
        val info = OperationInfo(
            service = "Webhooks",
            operation = "ListWebhooks",
            resourceType = "webhook",
            isMutation = false,
            projectId = null,
            resourceId = bucketId,
        )
        return requestPaginated(info, options, {
            httpGet("/buckets/${bucketId}/webhooks.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Webhook>>(body)
        }
    }

    /**
     * Create a new webhook for a project
     * @param bucketId The bucket ID
     * @param body Request body
     */
    suspend fun create(bucketId: Long, body: CreateWebhookBody): Webhook {
        val info = OperationInfo(
            service = "Webhooks",
            operation = "CreateWebhook",
            resourceType = "webhook",
            isMutation = true,
            projectId = null,
            resourceId = bucketId,
        )
        return request(info, {
            httpPost("/buckets/${bucketId}/webhooks.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("payload_url", kotlinx.serialization.json.JsonPrimitive(body.payloadUrl))
                put("types", kotlinx.serialization.json.JsonArray(body.types.map { kotlinx.serialization.json.JsonPrimitive(it) }))
                body.active?.let { put("active", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Webhook>(body)
        }
    }

    /**
     * Get a single webhook by id
     * @param webhookId The webhook ID
     */
    suspend fun get(webhookId: Long): Webhook {
        val info = OperationInfo(
            service = "Webhooks",
            operation = "GetWebhook",
            resourceType = "webhook",
            isMutation = false,
            projectId = null,
            resourceId = webhookId,
        )
        return request(info, {
            httpGet("/webhooks/${webhookId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Webhook>(body)
        }
    }

    /**
     * Update an existing webhook
     * @param webhookId The webhook ID
     * @param body Request body
     */
    suspend fun update(webhookId: Long, body: UpdateWebhookBody): Webhook {
        val info = OperationInfo(
            service = "Webhooks",
            operation = "UpdateWebhook",
            resourceType = "webhook",
            isMutation = true,
            projectId = null,
            resourceId = webhookId,
        )
        return request(info, {
            httpPut("/webhooks/${webhookId}", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.payloadUrl?.let { put("payload_url", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.types?.let { put("types", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.active?.let { put("active", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Webhook>(body)
        }
    }

    /**
     * Delete a webhook
     * @param webhookId The webhook ID
     */
    suspend fun delete(webhookId: Long): Unit {
        val info = OperationInfo(
            service = "Webhooks",
            operation = "DeleteWebhook",
            resourceType = "webhook",
            isMutation = true,
            projectId = null,
            resourceId = webhookId,
        )
        request(info, {
            httpDelete("/webhooks/${webhookId}", operationName = info.operation)
        }) { Unit }
    }
}
