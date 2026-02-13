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
    val id: Long = 0L,
    @SerialName("recording_id") val recordingId: Long = 0L,
    val action: String? = null,
    val details: JsonObject? = null,
    @SerialName("created_at") val createdAt: String? = null,
    val creator: Person? = null,
    @SerialName("boosts_count") val boostsCount: Int = 0,
    @SerialName("boosts_url") val boostsUrl: String? = null
)
