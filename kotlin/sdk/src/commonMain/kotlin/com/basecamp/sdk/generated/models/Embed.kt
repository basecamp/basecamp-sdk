package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Embed entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Embed(
    val url: String,
    val status: String,
    val sgid: String,
    val type: String? = null,
    @SerialName("content_type") val contentType: String? = null,
    @SerialName("image_url") val imageUrl: String? = null,
    val content: String? = null
)
