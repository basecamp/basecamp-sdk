package com.basecamp.sdk.generated.models

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

/**
 * ClientApproval entity from the Basecamp API.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
@Serializable
data class ClientApproval(
    val id: Long,
    val status: String,
    @SerialName("visible_to_clients") val visibleToClients: Boolean,
    @SerialName("created_at") val createdAt: String,
    @SerialName("updated_at") val updatedAt: String,
    val title: String,
    @SerialName("inherits_status") val inheritsStatus: Boolean,
    val type: String,
    val url: String,
    @SerialName("app_url") val appUrl: String,
    val parent: RecordingParent,
    val bucket: RecordingBucket,
    val creator: Person,
    @SerialName("bookmark_url") val bookmarkUrl: String? = null,
    @SerialName("subscription_url") val subscriptionUrl: String? = null,
    val content: String? = null,
    val subject: String? = null,
    @SerialName("due_on") val dueOn: String? = null,
    @SerialName("replies_count") val repliesCount: Int = 0,
    @SerialName("replies_url") val repliesUrl: String? = null,
    @SerialName("approval_status") val approvalStatus: String? = null,
    val approver: Person? = null,
    val responses: List<ClientApprovalResponse> = emptyList()
)
