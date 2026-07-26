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
    val id: Long? = null,
    @SerialName("created_at") val createdAt: String? = null,
    val request: WebhookDeliveryRequest? = null,
    val response: WebhookDeliveryResponse? = null
)
