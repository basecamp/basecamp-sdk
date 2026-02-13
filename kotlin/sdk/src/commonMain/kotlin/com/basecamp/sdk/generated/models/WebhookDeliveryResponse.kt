package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * WebhookDeliveryResponse entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class WebhookDeliveryResponse(
    val headers: JsonObject? = null,
    val code: Int = 0,
    val message: String? = null
)
