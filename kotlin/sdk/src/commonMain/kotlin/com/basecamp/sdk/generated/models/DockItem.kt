package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * DockItem entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class DockItem(
    val id: Long,
    val title: String,
    val name: String,
    val enabled: Boolean,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    val position: Int = 0
)
