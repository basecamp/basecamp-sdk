package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * MyAssignment entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class MyAssignment(
    val id: Long,
    @SerialName("app_url") val appUrl: String? = null,
    val content: String? = null,
    @SerialName("starts_on") val startsOn: String? = null,
    @SerialName("due_on") val dueOn: String? = null,
    val completed: Boolean = false,
    val type: String? = null,
    @SerialName("comments_count") val commentsCount: Int = 0,
    @SerialName("has_description") val hasDescription: Boolean = false,
    @SerialName("priority_recording_id") val priorityRecordingId: Long = 0L,
    val bucket: MyAssignmentBucket? = null,
    val parent: MyAssignmentParent? = null,
    val assignees: List<MyAssignmentPerson> = emptyList(),
    val children: List<MyAssignment> = emptyList()
)
