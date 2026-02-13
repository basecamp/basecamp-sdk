package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * EventDetails entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class EventDetails(
    @SerialName("added_person_ids") val addedPersonIds: List<Long> = emptyList(),
    @SerialName("removed_person_ids") val removedPersonIds: List<Long> = emptyList(),
    @SerialName("notified_recipient_ids") val notifiedRecipientIds: List<Long> = emptyList()
)
