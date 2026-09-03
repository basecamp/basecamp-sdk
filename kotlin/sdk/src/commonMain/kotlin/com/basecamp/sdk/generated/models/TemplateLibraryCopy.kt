package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * TemplateLibraryCopy entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class TemplateLibraryCopy(
    val id: Long,
    val status: String,
    @SerialName("source_recording_id") val sourceRecordingId: Long,
    @SerialName("destination_parent_id") val destinationParentId: Long,
    val url: String,
    @SerialName("destination_todolist") val destinationTodolist: Todolist? = null
)
