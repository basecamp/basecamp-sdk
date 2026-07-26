package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import com.basecamp.sdk.serialization.FlexibleIntSerializer

/**
 * TimelineAttachment entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class TimelineAttachment(
    val id: Long? = null,
    @SerialName("content_type") val contentType: String? = null,
    @SerialName("byte_size") val byteSize: Long? = null,
    val filename: String? = null,
    @SerialName("download_url") val downloadUrl: String? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val width: Int? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val height: Int? = null,
    val type: String? = null,
    val title: String? = null,
    val status: String? = null,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("updated_at") val updatedAt: String? = null,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    @SerialName("app_download_url") val appDownloadUrl: String? = null,
    @SerialName("visible_to_clients") val visibleToClients: Boolean? = null,
    @SerialName("attachable_sgid") val attachableSgid: String? = null,
    val sgid: String? = null,
    @SerialName("status_url") val statusUrl: String? = null,
    val caption: String? = null,
    val key: String? = null,
    val previewable: Boolean? = null,
    @SerialName("preview_url") val previewUrl: String? = null,
    @SerialName("thumbnail_url") val thumbnailUrl: String? = null
)
