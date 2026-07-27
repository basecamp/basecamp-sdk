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
    @SerialName("created_at") val createdAt: String,
    val content: String? = null,
    val booster: Person? = null,
    val recording: Recording? = null
)
