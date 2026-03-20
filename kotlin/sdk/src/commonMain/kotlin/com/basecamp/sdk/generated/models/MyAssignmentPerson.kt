package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * MyAssignmentPerson entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class MyAssignmentPerson(
    val id: Long,
    val name: String,
    @SerialName("avatar_url") val avatarUrl: String? = null
)
