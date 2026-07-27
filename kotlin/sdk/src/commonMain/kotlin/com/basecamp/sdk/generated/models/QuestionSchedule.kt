package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * QuestionSchedule entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class QuestionSchedule(
    val frequency: String? = null,
    val days: List<Int>? = null,
    val hour: Int? = null,
    val minute: Int? = null,
    @SerialName("week_instance") val weekInstance: Int? = null,
    @SerialName("week_interval") val weekInterval: Int? = null,
    @SerialName("month_interval") val monthInterval: Int? = null,
    @SerialName("start_date") val startDate: String? = null,
    @SerialName("end_date") val endDate: String? = null
)
