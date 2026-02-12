package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Boost entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Boost(
    val id: Long = 0L,
    val content: String? = null,
    @SerialName("created_at") val createdAt: String? = null,
    val booster: Person? = null,
    val recording: JsonObject? = null
)
