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
    val id: Long = 0L,
    val title: String? = null,
    val name: String? = null,
    val enabled: Boolean = false,
    val position: Int = 0,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null
)
