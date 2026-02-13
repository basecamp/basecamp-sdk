package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * CardStep entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class CardStep(
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
    val position: Int = 0,
    @SerialName("due_on") val dueOn: String? = null,
    val completed: Boolean = false,
    @SerialName("completed_at") val completedAt: String? = null,
    val completer: Person? = null,
    val assignees: List<Person> = emptyList(),
    @SerialName("completion_url") val completionUrl: String? = null
)
