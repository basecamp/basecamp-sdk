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
    val id: Long,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    @SerialName("payload_url") val payloadUrl: String,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    val active: Boolean = false,
    val types: List<String> = emptyList(),
    @SerialName("recent_deliveries") val recentDeliveries: List<WebhookDelivery> = emptyList()
)
