package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Calendar entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Calendar(
    val id: Long,
    val type: String,
    val name: String,
    val color: String,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    @SerialName("schedule_url") val scheduleUrl: String
)
