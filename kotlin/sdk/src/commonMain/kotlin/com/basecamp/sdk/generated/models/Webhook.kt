package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Webhook entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Webhook(
    val id: Long = 0L,
    val active: Boolean = false,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("updated_at") val updatedAt: String? = null,
    @SerialName("payload_url") val payloadUrl: String? = null,
    val types: List<String> = emptyList(),
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    @SerialName("recent_deliveries") val recentDeliveries: List<JsonObject> = emptyList()
)
