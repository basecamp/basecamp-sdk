package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * TemplateLibrary entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class TemplateLibrary(
    val bucket: RecordingBucket,
    val todoset: RecordingParent,
    val todolists: List<Todolist>
)
