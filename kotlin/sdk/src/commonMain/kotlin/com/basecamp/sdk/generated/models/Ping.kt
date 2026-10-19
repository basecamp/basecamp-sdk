package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Ping entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Ping(
    val id: Long,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    val title: String,
    val named: Boolean,
    @SerialName("avatar_url") val avatarUrl: String,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    @SerialName("bookmark_url") val bookmarkUrl: String,
    @SerialName("room_url") val roomUrl: String,
    val links: PingLinks,
    val people: List<Person>,
    @SerialName("image_url") val imageUrl: String? = null,
    @SerialName("edit_title_url") val editTitleUrl: String? = null
)
