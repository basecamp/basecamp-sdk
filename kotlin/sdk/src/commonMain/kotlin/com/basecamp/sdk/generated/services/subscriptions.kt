package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Subscriptions operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class SubscriptionsService(client: AccountClient) : BaseService(client) {

    /**
     * Get subscription information for a recording
     * @param recordingId The recording ID
     */
    suspend fun get(recordingId: Long): Subscription {
        val info = OperationInfo(
            service = "Subscriptions",
            operation = "GetSubscription",
            resourceType = "subscription",
            isMutation = false,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpGet("/recordings/${recordingId}/subscription.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Subscription>(body)
        }
    }

    /**
     * Subscribe the current user to a recording
     * @param recordingId The recording ID
     */
    suspend fun subscribe(recordingId: Long): Subscription {
        val info = OperationInfo(
            service = "Subscriptions",
            operation = "Subscribe",
            resourceType = "resource",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/recordings/${recordingId}/subscription.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Subscription>(body)
        }
    }

    /**
     * Update subscriptions by adding or removing specific users
     * @param recordingId The recording ID
     * @param body Request body
     */
    suspend fun update(recordingId: Long, body: UpdateSubscriptionBody): Subscription {
        val info = OperationInfo(
            service = "Subscriptions",
            operation = "UpdateSubscription",
            resourceType = "subscription",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        return request(info, {
            httpPut("/recordings/${recordingId}/subscription.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.unsubscriptions?.let { put("unsubscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Subscription>(body)
        }
    }

    /**
     * Unsubscribe the current user from a recording
     * @param recordingId The recording ID
     */
    suspend fun unsubscribe(recordingId: Long): Unit {
        val info = OperationInfo(
            service = "Subscriptions",
            operation = "Unsubscribe",
            resourceType = "resource",
            isMutation = true,
            projectId = null,
            resourceId = recordingId,
        )
        request(info, {
            httpDelete("/recordings/${recordingId}/subscription.json", operationName = info.operation)
        }) { Unit }
    }
}
