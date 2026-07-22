package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import com.basecamp.sdk.serialization.FlexibleIntSerializer

/**
 * RichTextAttachment entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class RichTextAttachment(
    val id: Long,
    val sgid: String,
    val filename: String,
    @SerialName("content_type") val contentType: String,
    @SerialName("byte_size") val byteSize: Long,
    @SerialName("download_url") val downloadUrl: String,
    val previewable: Boolean,
    @SerialName("preview_url") val previewUrl: String,
    @SerialName("thumbnail_url") val thumbnailUrl: String,
    @Serializable(with = FlexibleIntSerializer::class)
    val width: Int? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val height: Int? = null
)
