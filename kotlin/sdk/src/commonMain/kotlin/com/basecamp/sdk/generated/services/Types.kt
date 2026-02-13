package com.basecamp.sdk.generated.services

import com.basecamp.sdk.PaginationOptions
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject

/**
 * Request body and options classes for generated service methods.
 *
 * @generated from OpenAPI spec — do not edit directly
 */

/** Request body for CreateRecordingBoost. */
data class CreateRecordingBoostBody(
    val content: String
)

/** Request body for CreateEventBoost. */
data class CreateEventBoostBody(
    val content: String
)

/** Request body for CreateChatbot. */
data class CreateChatbotBody(
    val serviceName: String,
    val commandUrl: String? = null
)

/** Request body for UpdateChatbot. */
data class UpdateChatbotBody(
    val serviceName: String,
    val commandUrl: String? = null
)

/** Request body for CreateCampfireLine. */
data class CreateCampfireLineBody(
    val content: String,
    val contentType: String? = null
)

/** Request body for UpdateCardColumn. */
data class UpdateCardColumnBody(
    val title: String? = null,
    val description: String? = null
)

/** Request body for SetCardColumnColor. */
data class SetCardColumnColorBody(
    val color: String
)

/** Request body for CreateCardColumn. */
data class CreateCardColumnBody(
    val title: String,
    val description: String? = null
)

/** Request body for MoveCardColumn. */
data class MoveCardColumnBody(
    val sourceId: Long,
    val targetId: Long,
    val position: Int? = null
)

/** Request body for RepositionCardStep. */
data class RepositionCardStepBody(
    val sourceId: Long,
    val position: Int
)

/** Request body for CreateCardStep. */
data class CreateCardStepBody(
    val title: String,
    val dueOn: String? = null,
    val assignees: List<Long>? = null
)

/** Request body for UpdateCardStep. */
data class UpdateCardStepBody(
    val title: String? = null,
    val dueOn: String? = null,
    val assignees: List<Long>? = null
)

/** Request body for SetCardStepCompletion. */
data class SetCardStepCompletionBody(
    val completion: String
)

/** Request body for UpdateCard. */
data class UpdateCardBody(
    val title: String? = null,
    val content: String? = null,
    val dueOn: String? = null,
    val assigneeIds: List<Long>? = null
)

/** Request body for MoveCard. */
data class MoveCardBody(
    val columnId: Long
)

/** Request body for CreateCard. */
data class CreateCardBody(
    val title: String,
    val content: String? = null,
    val dueOn: String? = null,
    val notify: Boolean? = null
)

/** Request body for UpdateAnswer. */
data class UpdateAnswerBody(
    val content: String
)

/** Request body for CreateQuestion. */
data class CreateQuestionBody(
    val title: String,
    val schedule: JsonObject
)

/** Request body for UpdateQuestion. */
data class UpdateQuestionBody(
    val title: String? = null,
    val schedule: JsonObject? = null,
    val paused: Boolean? = null
)

/** Request body for CreateAnswer. */
data class CreateAnswerBody(
    val content: String,
    val groupOn: String? = null
)

/** Request body for UpdateQuestionNotificationSettings. */
data class UpdateQuestionNotificationSettingsBody(
    val notifyOnAnswer: Boolean? = null,
    val digestIncludeUnanswered: Boolean? = null
)

/** Request body for SetClientVisibility. */
data class SetClientVisibilityBody(
    val visibleToClients: Boolean
)

/** Request body for UpdateComment. */
data class UpdateCommentBody(
    val content: String
)

/** Request body for CreateComment. */
data class CreateCommentBody(
    val content: String
)

/** Request body for UpdateDocument. */
data class UpdateDocumentBody(
    val title: String? = null,
    val content: String? = null
)

/** Request body for CreateDocument. */
data class CreateDocumentBody(
    val title: String,
    val content: String? = null,
    val status: String? = null
)

/** Request body for CreateForwardReply. */
data class CreateForwardReplyBody(
    val content: String
)

/** Request body for CreateLineupMarker. */
data class CreateLineupMarkerBody(
    val name: String,
    val date: String
)

/** Request body for UpdateLineupMarker. */
data class UpdateLineupMarkerBody(
    val name: String? = null,
    val date: String? = null
)

/** Request body for CreateMessageType. */
data class CreateMessageTypeBody(
    val name: String,
    val icon: String
)

/** Request body for UpdateMessageType. */
data class UpdateMessageTypeBody(
    val name: String? = null,
    val icon: String? = null
)

/** Request body for CreateMessage. */
data class CreateMessageBody(
    val subject: String,
    val content: String? = null,
    val status: String? = null,
    val categoryId: Long? = null
)

/** Request body for UpdateMessage. */
data class UpdateMessageBody(
    val subject: String? = null,
    val content: String? = null,
    val status: String? = null,
    val categoryId: Long? = null
)

/** Request body for UpdateProjectAccess. */
data class UpdateProjectAccessBody(
    val grant: List<Long>? = null,
    val revoke: List<Long>? = null,
    val create: List<JsonObject>? = null
)

/** Options for ListProjects. */
data class ListProjectsOptions(
    val status: String? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems)
}

/** Request body for CreateProject. */
data class CreateProjectBody(
    val name: String,
    val description: String? = null
)

/** Request body for UpdateProject. */
data class UpdateProjectBody(
    val name: String,
    val description: String? = null,
    val admissions: String? = null,
    val scheduleAttributes: JsonObject? = null
)

/** Options for ListRecordings. */
data class ListRecordingsOptions(
    val bucket: String? = null,
    val status: String? = null,
    val sort: String? = null,
    val direction: String? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems)
}

/** Options for GetUpcomingSchedule. */
data class GetUpcomingScheduleOptions(
    val windowStartsOn: String? = null,
    val windowEndsOn: String? = null
) {
}

/** Options for GetAssignedTodos. */
data class GetAssignedTodosOptions(
    val groupBy: String? = null
) {
}

/** Request body for UpdateScheduleEntry. */
data class UpdateScheduleEntryBody(
    val summary: String? = null,
    val startsAt: String? = null,
    val endsAt: String? = null,
    val description: String? = null,
    val participantIds: List<Long>? = null,
    val allDay: Boolean? = null,
    val notify: Boolean? = null
)

/** Request body for UpdateScheduleSettings. */
data class UpdateScheduleSettingsBody(
    val includeDueAssignments: Boolean
)

/** Options for ListScheduleEntries. */
data class ListScheduleEntriesOptions(
    val status: String? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems)
}

/** Request body for CreateScheduleEntry. */
data class CreateScheduleEntryBody(
    val summary: String,
    val startsAt: String,
    val endsAt: String,
    val description: String? = null,
    val participantIds: List<Long>? = null,
    val allDay: Boolean? = null,
    val notify: Boolean? = null
)

/** Options for Search. */
data class SearchOptions(
    val sort: String? = null,
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems)
}

/** Request body for UpdateSubscription. */
data class UpdateSubscriptionBody(
    val subscriptions: List<Long>? = null,
    val unsubscriptions: List<Long>? = null
)

/** Options for ListTemplates. */
data class ListTemplatesOptions(
    val status: String? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems)
}

/** Request body for CreateTemplate. */
data class CreateTemplateBody(
    val name: String,
    val description: String? = null
)

/** Request body for UpdateTemplate. */
data class UpdateTemplateBody(
    val name: String? = null,
    val description: String? = null
)

/** Request body for CreateProjectFromTemplate. */
data class CreateProjectFromTemplateBody(
    val name: String,
    val description: String? = null
)

/** Request body for CreateTimesheetEntry. */
data class CreateTimesheetEntryBody(
    val date: String,
    val hours: String,
    val description: String? = null,
    val personId: Long? = null
)

/** Options for GetRecordingTimesheet. */
data class GetRecordingTimesheetOptions(
    val from: String? = null,
    val to: String? = null,
    val personId: Long? = null
) {
}

/** Options for GetProjectTimesheet. */
data class GetProjectTimesheetOptions(
    val from: String? = null,
    val to: String? = null,
    val personId: Long? = null
) {
}

/** Request body for UpdateTimesheetEntry. */
data class UpdateTimesheetEntryBody(
    val date: String? = null,
    val hours: String? = null,
    val description: String? = null,
    val personId: Long? = null
)

/** Options for GetTimesheetReport. */
data class GetTimesheetReportOptions(
    val from: String? = null,
    val to: String? = null,
    val personId: Long? = null
) {
}

/** Request body for RepositionTodolistGroup. */
data class RepositionTodolistGroupBody(
    val position: Int
)

/** Request body for CreateTodolistGroup. */
data class CreateTodolistGroupBody(
    val name: String
)

/** Request body for UpdateTodolistOrGroup. */
data class UpdateTodolistOrGroupBody(
    val name: String? = null,
    val description: String? = null
)

/** Options for ListTodolists. */
data class ListTodolistsOptions(
    val status: String? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems)
}

/** Request body for CreateTodolist. */
data class CreateTodolistBody(
    val name: String,
    val description: String? = null
)

/** Options for ListTodos. */
data class ListTodosOptions(
    val status: String? = null,
    val completed: Boolean? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems)
}

/** Request body for CreateTodo. */
data class CreateTodoBody(
    val content: String,
    val description: String? = null,
    val assigneeIds: List<Long>? = null,
    val completionSubscriberIds: List<Long>? = null,
    val notify: Boolean? = null,
    val dueOn: String? = null,
    val startsOn: String? = null
)

/** Request body for UpdateTodo. */
data class UpdateTodoBody(
    val content: String? = null,
    val description: String? = null,
    val assigneeIds: List<Long>? = null,
    val completionSubscriberIds: List<Long>? = null,
    val notify: Boolean? = null,
    val dueOn: String? = null,
    val startsOn: String? = null
)

/** Request body for RepositionTodo. */
data class RepositionTodoBody(
    val position: Int,
    val parentId: Long? = null
)

/** Request body for CloneTool. */
data class CloneToolBody(
    val sourceRecordingId: Long
)

/** Request body for UpdateTool. */
data class UpdateToolBody(
    val title: String
)

/** Request body for RepositionTool. */
data class RepositionToolBody(
    val position: Int
)

/** Request body for UpdateUpload. */
data class UpdateUploadBody(
    val description: String? = null,
    val baseName: String? = null
)

/** Request body for CreateUpload. */
data class CreateUploadBody(
    val attachableSgid: String,
    val description: String? = null,
    val baseName: String? = null
)

/** Request body for UpdateVault. */
data class UpdateVaultBody(
    val title: String? = null
)

/** Request body for CreateVault. */
data class CreateVaultBody(
    val title: String
)

/** Request body for CreateWebhook. */
data class CreateWebhookBody(
    val payloadUrl: String,
    val types: List<String>,
    val active: Boolean? = null
)

/** Request body for UpdateWebhook. */
data class UpdateWebhookBody(
    val payloadUrl: String? = null,
    val types: List<String>? = null,
    val active: Boolean? = null
)

