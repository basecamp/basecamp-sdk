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
    val bucket: TodoBucket,
    val creator: Person,
    val name: String,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    val position: Int = 0,
    @SerialName("todolists_count") val todolistsCount: Int = 0,
    @SerialName("todolists_url") val todolistsUrl: String? = null,
    @SerialName("completed_ratio") val completedRatio: String? = null,
    val completed: Boolean = false,
    @SerialName("app_todolists_url") val appTodolistsUrl: String? = null,
    @SerialName("todos_count") val todosCount: Int = 0,
    @SerialName("completed_loose_todos_count") val completedLooseTodosCount: Int = 0,
    @SerialName("todos_url") val todosUrl: String? = null,
    @SerialName("app_todos_url") val appTodosUrl: String? = null
)
