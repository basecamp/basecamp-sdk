package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * CardColumnOnHold represents the on-hold section status of a card column.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class CardColumnOnHold(
    val enabled: Boolean,
    val id: Long? = null,
    @SerialName("cards_count") val cardsCount: Int = 0,
    @SerialName("cards_url") val cardsUrl: String? = null
)
