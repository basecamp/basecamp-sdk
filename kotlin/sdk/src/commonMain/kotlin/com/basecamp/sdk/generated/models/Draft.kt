package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Draft entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Draft(
    val id: Long,
    @SerialName("app_url") val appUrl: String,
    val title: String,
    val type: String,
    val bucket: DraftBucket,
    val parent: DraftParent?,
    val excerpt: String,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    @SerialName("scheduled_posting_at") val scheduledPostingAt: String?
)
