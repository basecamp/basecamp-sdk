package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * UpcomingScheduleEntry entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class UpcomingScheduleEntry(
    val id: Long,
    val status: String,
    @SerialName("visible_to_clients") val visibleToClients: Boolean,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    val type: String,
    val summary: String,
    @SerialName("all_day") val allDay: Boolean,
    val recurring: Boolean,
    @SerialName("starts_at") val startsAt: String,
    @SerialName("ends_at") val endsAt: String,
    val creator: UpcomingSchedulePerson,
    val participants: List<UpcomingSchedulePerson>,
    val bucket: UpcomingScheduleBucket,
    @SerialName("comments_count") val commentsCount: Int
)
