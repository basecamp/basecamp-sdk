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
@Suppress("DEPRECATION")
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
    @SerialName("start_date") val startDate: String? = null,
    @SerialName("end_date") val endDate: String? = null,
    @SerialName("clients_enabled") val clientsEnabled: Boolean? = null,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    @SerialName("star_url") val starUrl: String? = null,
    val dock: List<DockItem>? = null,
    val bookmarked: Boolean? = null,
    val starred: Boolean? = null,
    @SerialName("client_company") val clientCompany: ClientCompany? = null,
    @Deprecated("This shape is deprecated since 2024-01: Use Client Visibility feature instead")
    val clientside: ClientSide? = null
)
