package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * UpcomingAssignable entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class UpcomingAssignable(
    val id: Long,
    val status: String,
    @SerialName("visible_to_clients") val visibleToClients: Boolean,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    val type: String,
    val content: String,
    val assignees: List<UpcomingSchedulePerson>,
    val bucket: UpcomingScheduleBucket,
    val parent: UpcomingAssignableParent,
    @SerialName("completion_url") val completionUrl: String,
    val completed: Boolean,
    val repeating: Boolean,
    @SerialName("comments_count") val commentsCount: Int,
    @SerialName("starts_on") val startsOn: String? = null,
    @SerialName("due_on") val dueOn: String? = null,
    val completion: UpcomingAssignableCompletion? = null
)
