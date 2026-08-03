package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Folder entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Folder(
    val id: Long,
    val name: String,
    val type: String,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    @SerialName("bucket_ids") val bucketIds: List<Long>,
    @SerialName("is_emoji_only_name") val isEmojiOnlyName: Boolean,
    @SerialName("star_url") val starUrl: String,
    @SerialName("gauges_url") val gaugesUrl: String?,
    val color: String?,
    @SerialName("image_url") val imageUrl: String?,
    val url: String
)
