package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Todoset entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Todoset(
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
    val position: Int = 0,
    val bucket: JsonObject? = null,
    val creator: Person? = null,
    val name: String? = null,
    @SerialName("todolists_count") val todolistsCount: Int = 0,
    @SerialName("todolists_url") val todolistsUrl: String? = null,
    @SerialName("completed_ratio") val completedRatio: String? = null,
    val completed: Boolean = false,
    @SerialName("completed_count") val completedCount: Int = 0,
    @SerialName("on_schedule_count") val onScheduleCount: Int = 0,
    @SerialName("over_schedule_count") val overScheduleCount: Int = 0,
    @SerialName("app_todolists_url") val appTodolistsUrl: String? = null
)
