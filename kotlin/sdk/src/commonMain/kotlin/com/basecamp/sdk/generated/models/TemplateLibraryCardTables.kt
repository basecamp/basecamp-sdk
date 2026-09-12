package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * TemplateLibraryCardTables entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class TemplateLibraryCardTables(
    val bucket: RecordingBucket,
    @SerialName("kanban_boardset") val kanbanBoardset: RecordingParent?,
    @SerialName("card_tables") val cardTables: List<Recording>
)
