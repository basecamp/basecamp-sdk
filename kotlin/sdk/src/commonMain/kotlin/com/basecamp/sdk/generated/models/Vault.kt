package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Vault entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Vault(
    val id: Long = 0L,
    val status: String? = null,
    @SerialName("visible_to_clients") val visibleToClients: Boolean = false,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("updated_at") val updatedAt: String? = null,
    val title: String? = null,
    @SerialName("inherits_status") val inheritsStatus: Boolean = false,
    val type: String? = null,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    val position: Int = 0,
    val parent: JsonObject? = null,
    val bucket: JsonObject? = null,
    val creator: Person? = null,
    @SerialName("documents_count") val documentsCount: Int = 0,
    @SerialName("documents_url") val documentsUrl: String? = null,
    @SerialName("uploads_count") val uploadsCount: Int = 0,
    @SerialName("uploads_url") val uploadsUrl: String? = null,
    @SerialName("vaults_count") val vaultsCount: Int = 0,
    @SerialName("vaults_url") val vaultsUrl: String? = null
)
