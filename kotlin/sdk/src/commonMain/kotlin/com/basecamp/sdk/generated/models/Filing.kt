package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Filing entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Filing(
    val id: Long,
    val status: String,
    val type: String,
    @SerialName("source_recording_id") val sourceRecordingId: Long,
    val url: String,
    @SerialName("destination_recording") val destinationRecording: FilingDestinationRecording? = null
)
