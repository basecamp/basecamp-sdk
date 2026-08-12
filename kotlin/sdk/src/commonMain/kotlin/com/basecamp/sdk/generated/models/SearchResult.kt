package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import com.basecamp.sdk.serialization.FlexibleIntSerializer

/**
 * SearchResult entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class SearchResult(
    val content: String?,
    val description: String?,
    val id: Long? = null,
    val status: String? = null,
    @SerialName("visible_to_clients") val visibleToClients: Boolean? = null,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("updated_at") val updatedAt: String? = null,
    val title: String? = null,
    @SerialName("inherits_status") val inheritsStatus: Boolean? = null,
    val type: String? = null,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    @SerialName("subscription_url") val subscriptionUrl: String? = null,
    @SerialName("bubble_up_url") val bubbleUpUrl: String? = null,
    val parent: RecordingParent? = null,
    val bucket: RecordingBucket? = null,
    val creator: Person? = null,
    @SerialName("plain_text_content") val plainTextContent: String? = null,
    @SerialName("plain_text_description") val plainTextDescription: String? = null,
    @SerialName("content_attachments") val contentAttachments: List<RichTextAttachment>? = null,
    @SerialName("description_attachments") val descriptionAttachments: List<RichTextAttachment>? = null,
    val attachments: List<SearchResultAttachment>? = null,
    val subject: String? = null,
    @SerialName("boosts_count") val boostsCount: Int? = null,
    @SerialName("boosts_url") val boostsUrl: String? = null,
    val language: String? = null,
    @SerialName("image_url") val imageUrl: String? = null,
    @SerialName("sound_url") val soundUrl: String? = null,
    val subscribers: List<Person>? = null,
    val color: String? = null,
    @SerialName("cards_count") val cardsCount: Int? = null,
    @SerialName("comment_count") val commentCount: Int? = null,
    @SerialName("cards_url") val cardsUrl: String? = null,
    @SerialName("on_hold") val onHold: CardColumnOnHold? = null,
    @SerialName("comments_count") val commentsCount: Int? = null,
    @SerialName("comments_url") val commentsUrl: String? = null,
    val position: Int? = null,
    val filename: String? = null,
    @SerialName("content_type") val contentType: String? = null,
    @SerialName("byte_size") val byteSize: Long? = null,
    val previewable: Boolean? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val width: Int? = null,
    @Serializable(with = FlexibleIntSerializer::class)
    val height: Int? = null,
    @SerialName("preview_url") val previewUrl: String? = null,
    @SerialName("thumbnail_url") val thumbnailUrl: String? = null,
    @SerialName("download_url") val downloadUrl: String? = null,
    @SerialName("app_download_url") val appDownloadUrl: String? = null
)
