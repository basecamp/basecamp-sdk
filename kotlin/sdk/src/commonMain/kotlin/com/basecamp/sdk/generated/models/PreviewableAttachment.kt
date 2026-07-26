package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * PreviewableAttachment entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class PreviewableAttachment(
    val id: Long? = null,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    @SerialName("content_type") val contentType: String? = null,
    val filename: String? = null,
    val filesize: Long? = null,
    val width: Int? = null,
    val height: Int? = null
)
