package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Card entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Card(
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
    val parent: RecordingParent,
    val bucket: TodoBucket,
    val creator: Person,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    @SerialName("subscription_url") val subscriptionUrl: String? = null,
    val position: Int = 0,
    val content: String? = null,
    val description: String? = null,
    @SerialName("due_on") val dueOn: String? = null,
    val completed: Boolean = false,
    @SerialName("completed_at") val completedAt: String? = null,
    @SerialName("comments_count") val commentsCount: Int = 0,
    @SerialName("comments_url") val commentsUrl: String? = null,
    @SerialName("completion_url") val completionUrl: String? = null,
    val completer: Person? = null,
    val assignees: List<Person> = emptyList(),
    @SerialName("completion_subscribers") val completionSubscribers: List<Person> = emptyList(),
    val steps: List<CardStep> = emptyList(),
    @SerialName("boosts_count") val boostsCount: Int = 0,
    @SerialName("boosts_url") val boostsUrl: String? = null
)
