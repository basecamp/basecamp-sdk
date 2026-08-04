package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * UpcomingAssignableCompletion entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class UpcomingAssignableCompletion(
    @SerialName("created_at") val createdAt: String,
    val creator: UpcomingSchedulePerson
)
