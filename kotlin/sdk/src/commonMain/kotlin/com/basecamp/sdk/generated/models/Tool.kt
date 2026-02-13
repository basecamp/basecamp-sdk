package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Tool entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Tool(
    val id: Long,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    val title: String,
    val name: String,
    val enabled: Boolean,
    val status: String? = null,
    val position: Int = 0,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    val bucket: RecordingBucket? = null
)
