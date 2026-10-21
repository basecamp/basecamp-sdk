package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for Embeds operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class EmbedsService(client: AccountClient) : BaseService(client) {

    /**
     * Create an embed for a URL, or return the one Basecamp already has for it.
     * @param body Request body
     */
    suspend fun create(body: CreateEmbedBody): Embed {
        val info = OperationInfo(
            service = "Embeds",
            operation = "CreateEmbed",
            resourceType = "embed",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPost("/embeds.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("url", kotlinx.serialization.json.JsonPrimitive(body.url))
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<Embed>(body)
        }
    }

    /**
     * Get an embed by its sgid.
     * @param sgid An embed's signed global id, an opaque string.
     */
    suspend fun get(sgid: String): Embed {
        val info = OperationInfo(
            service = "Embeds",
            operation = "GetEmbed",
            resourceType = "embed",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/embeds/${pathSegment(sgid)}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Embed>(body)
        }
    }
}
