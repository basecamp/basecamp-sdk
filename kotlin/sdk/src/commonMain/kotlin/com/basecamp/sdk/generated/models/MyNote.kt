package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * MyNote entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class MyNote(
    val id: Long?,
    val type: String,
    @SerialName("created_at") val createdAt: String?,
    @SerialName("updated_at") val updatedAt: String?,
    val content: String,
    @SerialName("content_attachments") val contentAttachments: List<RichTextAttachment>,
    val url: String,
    @SerialName("app_url") val appUrl: String
)
