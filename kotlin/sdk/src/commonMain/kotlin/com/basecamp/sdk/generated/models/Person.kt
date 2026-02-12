package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * Person entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class Person(
    val id: Long = 0L,
    @SerialName("attachable_sgid") val attachableSgid: String? = null,
    val name: String? = null,
    @SerialName("email_address") val emailAddress: String? = null,
    @SerialName("personable_type") val personableType: String? = null,
    val title: String? = null,
    val bio: String? = null,
    val location: String? = null,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("updated_at") val updatedAt: String? = null,
    val admin: Boolean = false,
    val owner: Boolean = false,
    val client: Boolean = false,
    val employee: Boolean = false,
    @SerialName("time_zone") val timeZone: String? = null,
    @SerialName("avatar_url") val avatarUrl: String? = null,
    val company: JsonObject? = null,
    @SerialName("can_manage_projects") val canManageProjects: Boolean = false,
    @SerialName("can_manage_people") val canManagePeople: Boolean = false,
    @SerialName("can_ping") val canPing: Boolean = false,
    @SerialName("can_access_timesheet") val canAccessTimesheet: Boolean = false,
    @SerialName("can_access_hill_charts") val canAccessHillCharts: Boolean = false
)
