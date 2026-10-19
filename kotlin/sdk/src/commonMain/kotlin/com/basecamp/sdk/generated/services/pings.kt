package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Pings operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class PingsService(client: AccountClient) : BaseService(client) {

    /**
     * Get Pings: the caller's active Pings that have a line in them, most
     */
    suspend fun list(): List<Ping> {
        val info = OperationInfo(
            service = "Pings",
            operation = "ListPings",
            resourceType = "ping",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/circles.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Ping>>(body)
        }
    }

    /**
     * Create a Ping with people, or return the one that already holds exactly
     * @param body Request body
     */
    suspend fun create(body: CreatePingBody): Ping {
        val info = OperationInfo(
            service = "Pings",
            operation = "CreatePing",
            resourceType = "ping",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/circles.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("user_ids", kotlinx.serialization.json.JsonArray(body.userIds.map { kotlinx.serialization.json.JsonPrimitive(it) }))
                body.newGroup?.let { put("new_group", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Ping>(body)
        }
    }

    /**
     * Get a Ping by its id, which is its bucket's id.
     * @param pingId A Ping's id is the id of the bucket that holds it (the bucket type is
"Circle", which is also the wire path), so the `bucket.id` a Ping's
recordings carry is the id GetPing takes.
     */
    suspend fun get(pingId: Long): Ping {
        val info = OperationInfo(
            service = "Pings",
            operation = "GetPing",
            resourceType = "ping",
            isMutation = false,
            projectId = null,
            resourceId = pingId,
        )
        return request(info, {
            httpGet("/circles/${pingId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Ping>(body)
        }
    }
}
