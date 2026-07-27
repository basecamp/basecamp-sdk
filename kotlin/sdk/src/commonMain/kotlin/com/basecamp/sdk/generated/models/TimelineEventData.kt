package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * TimelineEventData entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class TimelineEventData(
    @SerialName("all_day") val allDay: Boolean,
    @SerialName("starts_at") val startsAt: String,
    @SerialName("ends_at") val endsAt: String
)
