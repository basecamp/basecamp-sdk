package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * WebhookCopy entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class WebhookCopy(
    val id: Long = 0L,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    val bucket: WebhookCopyBucket? = null
)
