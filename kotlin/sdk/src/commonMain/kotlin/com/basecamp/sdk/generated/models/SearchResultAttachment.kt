package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import com.basecamp.sdk.serialization.FlexibleIntSerializer

/**
 * SearchResultAttachment entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class SearchResultAttachment(
    val filename: String,
    @SerialName("content_type") val contentType: String,
    @SerialName("byte_size") val byteSize: Long,
    @SerialName("download_url") val downloadUrl: String,
    val id: Long? = null,
    val sgid: String? = null,
    val previewable: Boolean? = null,
    @SerialName("preview_url") val previewUrl: String? = null,
    @SerialName("thumbnail_url") val thumbnailUrl: String? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val width: Int? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val height: Int? = null,
    val title: String? = null,
    val url: String? = null
)
