package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Event entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Event(
    val id: Long,
    @SerialName("recording_id") val recordingId: Long,
    val action: String,
    @SerialName("created_at") val createdAt: String,
    val creator: Person,
    val details: EventDetails? = null,
    @SerialName("boosts_count") val boostsCount: Int = 0,
    @SerialName("boosts_url") val boostsUrl: String? = null
)
