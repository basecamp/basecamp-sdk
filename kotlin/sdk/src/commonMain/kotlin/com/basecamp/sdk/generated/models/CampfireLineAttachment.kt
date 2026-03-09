package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * CampfireLineAttachment entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class CampfireLineAttachment(
    val title: String? = null,
    val url: String? = null,
    val filename: String? = null,
    @SerialName("content_type") val contentType: String? = null,
    @SerialName("byte_size") val byteSize: Long = 0L,
    @SerialName("download_url") val downloadUrl: String? = null
)
