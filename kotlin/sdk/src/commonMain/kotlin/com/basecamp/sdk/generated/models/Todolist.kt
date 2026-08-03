package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Todolist entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Todolist(
    val id: Long,
    val status: String,
    @SerialName("visible_to_clients") val visibleToClients: Boolean,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    val title: String,
    @SerialName("inherits_status") val inheritsStatus: Boolean,
    val type: String,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    @SerialName("bubble_up_url") val bubbleUpUrl: String,
    val parent: TodoParent,
    val bucket: TodoBucket,
    val creator: Person,
    val description: String,
    @SerialName("description_attachments") val descriptionAttachments: List<RichTextAttachment>,
    val name: String,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    @SerialName("subscription_url") val subscriptionUrl: String? = null,
    @SerialName("comments_count") val commentsCount: Int? = null,
    @SerialName("comments_url") val commentsUrl: String? = null,
    val position: Int? = null,
    val completed: Boolean? = null,
    @SerialName("completed_ratio") val completedRatio: String? = null,
    @SerialName("todos_url") val todosUrl: String? = null,
    @SerialName("groups_url") val groupsUrl: String? = null,
    @SerialName("group_position_url") val groupPositionUrl: String? = null,
    @SerialName("app_todos_url") val appTodosUrl: String? = null,
    val color: String? = null,
    @SerialName("comments_app_url") val commentsAppUrl: String? = null,
    @SerialName("boosts_count") val boostsCount: Int? = null,
    @SerialName("boosts_url") val boostsUrl: String? = null
)
