package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Schedule entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Schedule(
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
    @SerialName("include_due_assignments") val includeDueAssignments: Boolean = false,
    @SerialName("entries_count") val entriesCount: Int = 0,
    @SerialName("entries_url") val entriesUrl: String? = null
)
