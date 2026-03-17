package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * CardColumnOnHold entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class CardColumnOnHold(
    val id: Long,
    val status: String,
    @SerialName("inherits_status") val inheritsStatus: Boolean,
    val title: String,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    @SerialName("cards_count") val cardsCount: Int,
    @SerialName("cards_url") val cardsUrl: String
)
