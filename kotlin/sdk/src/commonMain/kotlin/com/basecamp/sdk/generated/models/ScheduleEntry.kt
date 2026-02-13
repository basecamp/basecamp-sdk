package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * ScheduleEntry entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class ScheduleEntry(
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
    val summary: String,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    @SerialName("subscription_url") val subscriptionUrl: String? = null,
    @SerialName("comments_count") val commentsCount: Int = 0,
    @SerialName("comments_url") val commentsUrl: String? = null,
    val description: String? = null,
    @SerialName("all_day") val allDay: Boolean = false,
    @SerialName("starts_at") val startsAt: String? = null,
    @SerialName("ends_at") val endsAt: String? = null,
    val participants: List<Person> = emptyList(),
    @SerialName("boosts_count") val boostsCount: Int = 0,
    @SerialName("boosts_url") val boostsUrl: String? = null
)
