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
    val id: Long,
    val status: String,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    val name: String,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    val description: String? = null,
    val purpose: String? = null,
    @SerialName("clients_enabled") val clientsEnabled: Boolean = false,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    val dock: List<DockItem> = emptyList(),
    val bookmarked: Boolean = false,
    @SerialName("client_company") val clientCompany: ClientCompany? = null,
    val clientside: ClientSide? = null
)
