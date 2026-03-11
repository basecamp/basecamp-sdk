package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * TimelineEvent entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class TimelineEvent(
    val id: Long = 0L,
    @SerialName("created_at") val createdAt: String? = null,
    val kind: String? = null,
    @SerialName("parent_recording_id") val parentRecordingId: Long = 0L,
    val url: String? = null,
    @SerialName("app_url") val appUrl: String? = null,
    val creator: Person? = null,
    val action: String? = null,
    val target: String? = null,
    val title: String? = null,
    @SerialName("summary_excerpt") val summaryExcerpt: String? = null,
    val bucket: TodoBucket? = null
)
