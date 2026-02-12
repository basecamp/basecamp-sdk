package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * WebhookDelivery entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class WebhookDelivery(
    val id: Long = 0L,
    @SerialName("created_at") val createdAt: String? = null,
    val request: JsonObject? = null,
    val response: JsonObject? = null
)
