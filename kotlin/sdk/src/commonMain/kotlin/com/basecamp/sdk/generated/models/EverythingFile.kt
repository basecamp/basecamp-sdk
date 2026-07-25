package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import com.basecamp.sdk.serialization.FlexibleIntSerializer

/**
 * EverythingFile entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class EverythingFile(
    val id: Long = 0L,
    val status: String? = null,
    @SerialName("visible_to_clients") val visibleToClients: Boolean = false,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("updated_at") val updatedAt: String? = null,
    val title: String? = null,
    @SerialName("inherits_status") val inheritsStatus: Boolean = false,
    val type: String? = null,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    @SerialName("subscription_url") val subscriptionUrl: String? = null,
    @SerialName("comments_count") val commentsCount: Int = 0,
    @SerialName("comments_url") val commentsUrl: String? = null,
    @SerialName("boosts_count") val boostsCount: Int = 0,
    @SerialName("boosts_url") val boostsUrl: String? = null,
    val position: Int = 0,
    val parent: RecordingParent? = null,
    val bucket: RecordingBucket? = null,
    val creator: Person? = null,
    @SerialName("attachable_sgid") val attachableSgid: String? = null,
    @SerialName("content_type") val contentType: String? = null,
    @SerialName("byte_size") val byteSize: Long = 0L,
    val filename: String? = null,
    @SerialName("download_url") val downloadUrl: String? = null,
    @SerialName("app_download_url") val appDownloadUrl: String? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val width: Int? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val height: Int? = null,
    val description: String? = null,
    @SerialName("description_attachments") val descriptionAttachments: List<RichTextAttachment>? = null
)
