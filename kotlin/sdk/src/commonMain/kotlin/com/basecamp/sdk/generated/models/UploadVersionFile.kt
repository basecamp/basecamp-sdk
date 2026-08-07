package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * UploadVersionFile entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class UploadVersionFile(
    val filename: String,
    @SerialName("download_url") val downloadUrl: String,
    @SerialName("app_download_url") val appDownloadUrl: String,
    val current: Boolean,
    @SerialName("content_type") val contentType: String? = null,
    @SerialName("byte_size") val byteSize: Long? = null
)
