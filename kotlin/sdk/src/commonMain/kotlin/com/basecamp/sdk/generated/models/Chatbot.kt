package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Chatbot entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Chatbot(
    val id: Long,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    @SerialName("service_name") val serviceName: String,
    @SerialName("command_url") val commandUrl: String? = null,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    @SerialName("lines_url") val linesUrl: String? = null
)
