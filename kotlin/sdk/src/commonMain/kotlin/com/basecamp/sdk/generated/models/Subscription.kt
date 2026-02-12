package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Subscription entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Subscription(
    val subscribed: Boolean = false,
    val count: Int = 0,
    val url: String? = null,
    val subscribers: List<Person> = emptyList()
)
