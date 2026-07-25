package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * HillChart entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class HillChart(
    val enabled: Boolean,
    val stale: Boolean,
    @SerialName("updated_at") val updatedAt: String? = null,
    @SerialName("app_update_url") val appUpdateUrl: String? = null,
    @SerialName("app_versions_url") val appVersionsUrl: String? = null,
    val dots: List<HillChartDot>? = null
)
