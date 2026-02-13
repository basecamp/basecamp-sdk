package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * WebhookEvent entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class WebhookEvent(
    val id: Long = 0L,
    val kind: String? = null,
    val details: JsonElement? = null,
    @SerialName("created_at") val createdAt: String? = null,
    val recording: Recording? = null,
    val creator: Person? = null,
    val copy: WebhookCopy? = null
)
