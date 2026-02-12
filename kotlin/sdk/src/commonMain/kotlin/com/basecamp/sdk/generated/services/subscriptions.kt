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
     * @param projectId The project ID
     * @param recordingId The recording ID
     */
    suspend fun get(projectId: Long, recordingId: Long): Subscription {
        val info = OperationInfo(
            service = "Subscriptions",
            operation = "GetSubscription",
            resourceType = "subscription",
            isMutation = false,
            projectId = projectId,
            resourceId = recordingId,
        )
        return request(info, {
            httpGet("/buckets/${projectId}/recordings/${recordingId}/subscription.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Subscription>(body)
        }
    }

    /**
     * Subscribe the current user to a recording
     * @param projectId The project ID
     * @param recordingId The recording ID
     */
    suspend fun subscribe(projectId: Long, recordingId: Long): Subscription {
        val info = OperationInfo(
            service = "Subscriptions",
            operation = "Subscribe",
            resourceType = "resource",
            isMutation = true,
            projectId = projectId,
            resourceId = recordingId,
        )
        return request(info, {
            httpPost("/buckets/${projectId}/recordings/${recordingId}/subscription.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Subscription>(body)
        }
    }

    /**
     * Update subscriptions by adding or removing specific users
     * @param projectId The project ID
     * @param recordingId The recording ID
     * @param body Request body
     */
    suspend fun update(projectId: Long, recordingId: Long, body: UpdateSubscriptionBody): Subscription {
        val info = OperationInfo(
            service = "Subscriptions",
            operation = "UpdateSubscription",
            resourceType = "subscription",
            isMutation = true,
            projectId = projectId,
            resourceId = recordingId,
        )
        return request(info, {
            httpPut("/buckets/${projectId}/recordings/${recordingId}/subscription.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.subscriptions?.let { put("subscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.unsubscriptions?.let { put("unsubscriptions", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Subscription>(body)
        }
    }

    /**
     * Unsubscribe the current user from a recording
     * @param projectId The project ID
     * @param recordingId The recording ID
     */
    suspend fun unsubscribe(projectId: Long, recordingId: Long): Unit {
        val info = OperationInfo(
            service = "Subscriptions",
            operation = "Unsubscribe",
            resourceType = "resource",
            isMutation = true,
            projectId = projectId,
            resourceId = recordingId,
        )
        request(info, {
            httpDelete("/buckets/${projectId}/recordings/${recordingId}/subscription.json", operationName = info.operation)
        }) { Unit }
    }
}
