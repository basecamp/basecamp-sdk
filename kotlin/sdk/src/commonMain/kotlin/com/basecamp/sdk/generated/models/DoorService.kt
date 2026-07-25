package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * DoorService entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class DoorService(
    val name: String? = null,
    @SerialName("example_url") val exampleUrl: String? = null,
    val code: String? = null,
    @SerialName("valid_patterns") val validPatterns: List<String> = emptyList(),
    @SerialName("supporting_text") val supportingText: String? = null
)
