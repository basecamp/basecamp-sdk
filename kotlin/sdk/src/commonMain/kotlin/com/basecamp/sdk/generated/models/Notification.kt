package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Notification entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Notification(
    val id: Long,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    val section: String? = null,
    @SerialName("unread_count") val unreadCount: Int = 0,
    @SerialName("unread_at") val unreadAt: String? = null,
    @SerialName("read_at") val readAt: String? = null,
    @SerialName("readable_sgid") val readableSgid: String? = null,
    @SerialName("readable_identifier") val readableIdentifier: String? = null,
    val title: String? = null,
    val type: String? = null,
    @SerialName("bucket_name") val bucketName: String? = null,
    val creator: Person? = null,
    @SerialName("content_excerpt") val contentExcerpt: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    @SerialName("unread_url") val unreadUrl: String? = null,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    @SerialName("memory_url") val memoryUrl: String? = null,
    @SerialName("bubble_up_url") val bubbleUpUrl: String? = null,
    @SerialName("bubble_up_at") val bubbleUpAt: String? = null,
    @SerialName("subscription_url") val subscriptionUrl: String? = null,
    val subscribed: Boolean = false,
    @SerialName("previewable_attachments") val previewableAttachments: List<PreviewableAttachment>? = null,
    val participants: List<Person>? = null,
    val named: Boolean = false,
    @SerialName("image_url") val imageUrl: String? = null
)
