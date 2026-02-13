package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Project entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Project(
    val id: Long = 0L,
    val status: String? = null,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("updated_at") val updatedAt: String? = null,
    val name: String? = null,
    val description: String? = null,
    val purpose: String? = null,
    @SerialName("clients_enabled") val clientsEnabled: Boolean = false,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    val dock: List<JsonObject> = emptyList(),
    val bookmarked: Boolean = false,
    @SerialName("client_company") val clientCompany: JsonObject? = null,
    val clientside: JsonObject? = null
)
