package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * EverythingBoost entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class EverythingBoost(
    val id: Long,
    val content: String,
    @SerialName("created_at") val createdAt: String,
    val booster: Person,
    val recording: Recording
)
