package com.basecamp.sdk.generated.services

import com.basecamp.sdk.PaginationOptions
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject

/**
 * Request body and options classes for generated service methods.
 *
 * @generated from OpenAPI spec — do not edit directly
 */

/** Request body for UpdateAccountName. */
data class UpdateAccountNameBody(
    val name: String
)

/** Options for ListMyBookmarks. */
data class ListMyBookmarksOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for ListRecordingBoosts. */
data class ListRecordingBoostsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateRecordingBoost. */
data class CreateRecordingBoostBody(
    val content: String
)

/** Options for ListEventBoosts. */
data class ListEventBoostsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateEventBoost. */
data class CreateEventBoostBody(
    val content: String
)

/** Request body for UpdateCalendar. */
data class UpdateCalendarBody(
    val calendar: JsonObject
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

/** Options for ListCampfires. */
data class ListCampfiresOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for ListCampfireLines. */
data class ListCampfireLinesOptions(
    /** created_at|updated_at */
    val sort: String? = null,
    /** asc|desc */
    val direction: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateCampfireLine. */
data class CreateCampfireLineBody(
    val content: String,
    val contentType: String? = null
)

/** Request body for UpdateCampfireLine. */
data class UpdateCampfireLineBody(
    val content: String
)

/** Options for ListCampfireUploads. */
data class ListCampfireUploadsOptions(
    /** created_at|updated_at */
    val sort: String? = null,
    /** asc|desc */
    val direction: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for SetCardColumnColor. */
data class SetCardColumnColorBody(
    val color: String
)

/** Request body for UpdateCardColumn. */
data class UpdateCardColumnBody(
    val title: String? = null,
    val description: String? = null
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
    val assigneeIds: List<Long>? = null
)

/** Request body for UpdateCardStep. */
data class UpdateCardStepBody(
    val title: String? = null,
    val dueOn: String? = null,
    val assigneeIds: List<Long>? = null
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
    val columnId: Long,
    val position: Int? = null
)

/** Options for ListCards. */
data class ListCardsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateCard. */
data class CreateCardBody(
    val title: String,
    val content: String? = null,
    val dueOn: String? = null,
    val notify: Boolean? = null
)

/** Options for GetQuestionReminders. */
data class GetQuestionRemindersOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for UpdateAnswer. */
data class UpdateAnswerBody(
    val content: String,
    val groupOn: String? = null
)

/** Options for ListQuestions. */
data class ListQuestionsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateQuestion. */
data class CreateQuestionBody(
    val title: String,
    val schedule: JsonObject,
    val visibleToClients: Boolean? = null
)

/** Request body for UpdateQuestion. */
data class UpdateQuestionBody(
    val title: String? = null,
    val schedule: JsonObject? = null,
    val paused: Boolean? = null
)

/** Options for ListAnswers. */
data class ListAnswersOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateAnswer. */
data class CreateAnswerBody(
    val content: String,
    val groupOn: String? = null
)

/** Options for GetAnswersByPerson. */
data class GetAnswersByPersonOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for UpdateQuestionNotificationSettings. */
data class UpdateQuestionNotificationSettingsBody(
    val notifyOnAnswer: Boolean? = null,
    val digestIncludeUnanswered: Boolean? = null
)

/** Options for ListClientApprovals. */
data class ListClientApprovalsOptions(
    /** created_at|updated_at */
    val sort: String? = null,
    /** asc|desc */
    val direction: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for ListClientCorrespondences. */
data class ListClientCorrespondencesOptions(
    /** created_at|updated_at */
    val sort: String? = null,
    /** asc|desc */
    val direction: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for ListClientReplies. */
data class ListClientRepliesOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for SetClientVisibility. */
data class SetClientVisibilityBody(
    val visibleToClients: Boolean
)

/** Request body for CreateCloudFile. */
data class CreateCloudFileBody(
    val url: String,
    val service: String,
    val title: String? = null,
    val description: String? = null,
    val subscriptions: List<Long>? = null,
    val visibleToClients: Boolean? = null
)

/** Request body for UpdateCloudFile. */
data class UpdateCloudFileBody(
    val url: String,
    val service: String,
    val title: String? = null,
    val description: String? = null,
    val subscriptions: List<Long>? = null
)

/** Request body for UpdateComment. */
data class UpdateCommentBody(
    val content: String
)

/** Options for ListComments. */
data class ListCommentsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateComment. */
data class CreateCommentBody(
    val content: String
)

/** Request body for ReplaceDocument. */
data class ReplaceDocumentBody(
    val title: String? = null,
    val content: String? = null
)

/** Options for ListDocuments. */
data class ListDocumentsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateDocument. */
data class CreateDocumentBody(
    val title: String,
    val content: String? = null,
    val status: String? = null,
    val subscriptions: List<Long>? = null,
    val visibleToClients: Boolean? = null
)

/** Options for ListMyDrafts. */
data class ListMyDraftsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for ListEvents. */
data class ListEventsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingCompletedCards. */
data class GetEverythingCompletedCardsOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingNoDueDateCards. */
data class GetEverythingNoDueDateCardsOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingNotNowCards. */
data class GetEverythingNotNowCardsOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingOpenCards. */
data class GetEverythingOpenCardsOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingOverdueCards. */
data class GetEverythingOverdueCardsOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null
) {
}

/** Options for GetEverythingUnassignedCards. */
data class GetEverythingUnassignedCardsOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingCheckins. */
data class GetEverythingCheckinsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingComments. */
data class GetEverythingCommentsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingFiles. */
data class GetEverythingFilesOptions(
    /** Filter by file kind: all (default), images, pdfs, documents, or videos. */
    val kind: String? = null,
    /** Restrict to files created by the given people (repeatable). */
    val peopleIds: List<Long>? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingForwards. */
data class GetEverythingForwardsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingMessages. */
data class GetEverythingMessagesOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingCompletedTodos. */
data class GetEverythingCompletedTodosOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingNoDueDateTodos. */
data class GetEverythingNoDueDateTodosOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingOpenTodos. */
data class GetEverythingOpenTodosOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetEverythingOverdueTodos. */
data class GetEverythingOverdueTodosOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null
) {
}

/** Options for GetEverythingUnassignedTodos. */
data class GetEverythingUnassignedTodosOptions(
    /** Restrict to tasks assigned to at least one of the given people (repeatable). Assignees on nested steps are not considered. */
    val assigneeIds: List<Long>? = null,
    /** Filter by due date: with, without, or overdue. Unrecognized values are ignored. */
    val due: String? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateFolder. */
data class CreateFolderBody(
    val name: String? = null,
    val projectIds: List<Long>? = null
)

/** Request body for UpdateFolder. */
data class UpdateFolderBody(
    val name: String
)

/** Options for ListForwardReplies. */
data class ListForwardRepliesOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for ListForwards. */
data class ListForwardsOptions(
    /** created_at|updated_at */
    val sort: String? = null,
    /** asc|desc */
    val direction: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for UpdateGaugeNeedle. */
data class UpdateGaugeNeedleBody(
    val gaugeNeedle: JsonObject? = null
)

/** Request body for ToggleGauge. */
data class ToggleGaugeBody(
    val gauge: JsonObject
)

/** Options for ListGaugeNeedles. */
data class ListGaugeNeedlesOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateGaugeNeedle. */
data class CreateGaugeNeedleBody(
    val gaugeNeedle: JsonObject,
    val notify: String? = null,
    val subscriptions: List<Long>? = null
)

/** Options for ListGauges. */
data class ListGaugesOptions(
    /** Comma-separated list of project IDs. When provided, results are returned in the order specified instead of by risk level. */
    val bucketIds: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateGoogleDocument. */
data class CreateGoogleDocumentBody(
    val url: String,
    val documentType: String,
    val title: String? = null,
    val description: String? = null,
    val status: String? = null,
    val subscriptions: List<Long>? = null,
    val visibleToClients: Boolean? = null
)

/** Request body for UpdateGoogleDocument. */
data class UpdateGoogleDocumentBody(
    val url: String,
    val documentType: String,
    val title: String? = null,
    val description: String? = null,
    val status: String? = null,
    val subscriptions: List<Long>? = null
)

/** Request body for UpdateHillChartSettings. */
data class UpdateHillChartSettingsBody(
    val tracked: List<Long>? = null,
    val untracked: List<Long>? = null
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

/** Options for ListMessages. */
data class ListMessagesOptions(
    /** created_at|updated_at */
    val sort: String? = null,
    /** asc|desc */
    val direction: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateMessage. */
data class CreateMessageBody(
    val subject: String,
    val content: String? = null,
    val status: String? = null,
    val categoryId: Long? = null,
    val subscriptions: List<Long>? = null,
    val visibleToClients: Boolean? = null
)

/** Request body for UpdateMessage. */
data class UpdateMessageBody(
    val subject: String? = null,
    val content: String? = null,
    val status: String? = null,
    val categoryId: Long? = null
)

/** Options for GetMyDueAssignments. */
data class GetMyDueAssignmentsOptions(
    /** Filter by due date range: overdue, due_today, due_tomorrow, due_later_this_week, due_next_week, due_later */
    val scope: String? = null
) {
}

/** Request body for PrioritizeAssignment. */
data class PrioritizeAssignmentBody(
    val id: Long
)

/** Request body for ReorderUpNext. */
data class ReorderUpNextBody(
    val sourceId: Long,
    val position: Int
)

/** Request body for UpdateMyNote. */
data class UpdateMyNoteBody(
    val note: JsonObject
)

/** Options for GetMyNotifications. */
data class GetMyNotificationsOptions(
    /** Page number for paginating through read items. Defaults to 1. This operation is not auto-paginated in any SDK, so a page is returned as asked for and later pages are not followed. */
    val page: Long? = null,
    /** Set to true to cap `bubble_ups` at 2 current bubble-ups and omit the `scheduled_bubble_ups` key entirely. Defaults to false. Use the dedicated bubble-ups endpoint (GetBubbleUps) to page through all current and scheduled bubble-ups. */
    val limitBubbleUps: Boolean? = null
) {
}

/** Options for GetBubbleUps. */
data class GetBubbleUpsOptions(
    /** Page number. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for MarkAsRead. */
data class MarkAsReadBody(
    val readables: List<String>
)

/** Request body for UpdateMyPreferences. */
data class UpdateMyPreferencesBody(
    val person: JsonObject
)

/** Request body for UpdateMyProfile. */
data class UpdateMyProfileBody(
    val name: String? = null,
    val emailAddress: String? = null,
    val title: String? = null,
    val bio: String? = null,
    val location: String? = null,
    val timeZoneName: String? = null,
    val firstWeekDay: JsonObject? = null,
    val timeFormat: String? = null
)

/** Options for ListPeople. */
data class ListPeopleOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for EnableOutOfOffice. */
data class EnableOutOfOfficeBody(
    val outOfOffice: JsonObject
)

/** Options for ListProjectPeople. */
data class ListProjectPeopleOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for UpdateProjectAccess. */
data class UpdateProjectAccessBody(
    val grant: List<Long>? = null,
    val revoke: List<Long>? = null,
    val create: List<JsonObject>? = null
)

/** Options for ListProjects. */
data class ListProjectsOptions(
    /** active|archived|trashed */
    val status: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
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
    /** active|archived|trashed */
    val status: String? = null,
    /** created_at|updated_at */
    val sort: String? = null,
    /** asc|desc */
    val direction: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetProgressReport. */
data class GetProgressReportOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetAssignedTodos. */
data class GetAssignedTodosOptions(
    /** Group by "bucket" or "date" */
    val groupBy: String? = null
) {
}

/** Options for GetPersonProgress. */
data class GetPersonProgressOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for ReplaceScheduleEntry. */
data class ReplaceScheduleEntryBody(
    val summary: String? = null,
    val startsAt: String,
    val endsAt: String,
    val description: String? = null,
    val participantIds: List<Long>? = null,
    val allDay: Boolean? = null,
    val notify: Boolean? = null,
    val url: String? = null,
    val highlighted: Boolean? = null
)

/** Request body for UpdateScheduleSettings. */
data class UpdateScheduleSettingsBody(
    val includeDueAssignments: Boolean
)

/** Options for ListScheduleEntries. */
data class ListScheduleEntriesOptions(
    /** active|archived|trashed */
    val status: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateScheduleEntry. */
data class CreateScheduleEntryBody(
    val summary: String,
    val startsAt: String,
    val endsAt: String,
    val description: String? = null,
    val participantIds: List<Long>? = null,
    val allDay: Boolean? = null,
    val notify: Boolean? = null,
    val url: String? = null,
    val highlighted: Boolean? = null,
    val status: String? = null,
    val subscriptions: List<Long>? = null,
    val visibleToClients: Boolean? = null
)

/** Options for Search. */
data class SearchOptions(
    /** Recording types to include. Use `key` values from the metadata endpoint's `recording_search_types`. Available since Basecamp 5. */
    val typeNames: List<String>? = null,
    /** Project IDs to filter by. Available since Basecamp 5. */
    val bucketIds: List<Long>? = null,
    /** Creator person IDs to filter by. Available since Basecamp 5. */
    val creatorIds: List<Long>? = null,
    /** Filter attachments by type. Use `key` values from the metadata endpoint's `file_search_types`. */
    val fileType: String? = null,
    /** Set to true to exclude chat results. */
    val excludeChat: Boolean? = null,
    /** last_7_days|last_30_days|last_90_days|last_12_months|forever */
    val since: String? = null,
    /** best_match|recency */
    val sort: String? = null,
    @Deprecated("prefer type_names[].")
    val type: String? = null,
    @Deprecated("prefer bucket_ids[].")
    val bucketId: Long? = null,
    @Deprecated("prefer creator_ids[].")
    val creatorId: Long? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for UpdateSubscription. */
data class UpdateSubscriptionBody(
    val subscriptions: List<Long>? = null,
    val unsubscriptions: List<Long>? = null
)

/** Options for ListTemplates. */
data class ListTemplatesOptions(
    /** active|archived|trashed */
    val status: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
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
    val project: JsonObject
)

/** Options for GetProjectTimeline. */
data class GetProjectTimelineOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetProjectTimesheet. */
data class GetProjectTimesheetOptions(
    val from: String? = null,
    val to: String? = null,
    val personId: Long? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Options for GetRecordingTimesheet. */
data class GetRecordingTimesheetOptions(
    val from: String? = null,
    val to: String? = null,
    val personId: Long? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateTimesheetEntry. */
data class CreateTimesheetEntryBody(
    val date: String,
    val hours: String,
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

/** Request body for UpdateTimesheetEntry. */
data class UpdateTimesheetEntryBody(
    val date: String? = null,
    val hours: String? = null,
    val description: String? = null,
    val personId: Long? = null
)

/** Request body for RepositionTodolistGroup. */
data class RepositionTodolistGroupBody(
    val position: Int
)

/** Options for ListTodolistGroups. */
data class ListTodolistGroupsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateTodolistGroup. */
data class CreateTodolistGroupBody(
    val name: String
)

/** Request body for UpdateTodolistOrGroup. */
data class UpdateTodolistOrGroupBody(
    val name: String,
    val description: String? = null
)

/** Request body for RepositionTodolist. */
data class RepositionTodolistBody(
    val position: Int
)

/** Options for ListTodolists. */
data class ListTodolistsOptions(
    /** active|archived|trashed */
    val status: String? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateTodolist. */
data class CreateTodolistBody(
    val name: String,
    val description: String? = null,
    val visibleToClients: Boolean? = null
)

/** Request body for CreateTodosetTodo. */
data class CreateTodosetTodoBody(
    val content: String,
    val description: String? = null,
    val assigneeIds: List<Long>? = null,
    val completionSubscriberIds: List<Long>? = null,
    val notify: Boolean? = null,
    val dueOn: String? = null,
    val startsOn: String? = null
)

/** Options for ListTodos. */
data class ListTodosOptions(
    /** active|archived|trashed */
    val status: String? = null,
    val completed: Boolean? = null,
    val maxItems: Int? = null,
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
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

/** Request body for ReplaceTodo. */
data class ReplaceTodoBody(
    val content: String,
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

/** Request body for CreateTool. */
data class CreateToolBody(
    val toolType: String,
    val title: String? = null,
    val visibleToClients: Boolean? = null
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

/** Options for ListUploads. */
data class ListUploadsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

/** Request body for CreateUpload. */
data class CreateUploadBody(
    val attachableSgid: String,
    val description: String? = null,
    val baseName: String? = null,
    val subscriptions: List<Long>? = null,
    val visibleToClients: Boolean? = null
)

/** Request body for UpdateVault. */
data class UpdateVaultBody(
    val title: String? = null
)

/** Options for ListVaults. */
data class ListVaultsOptions(
    /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
    val page: Long? = null,
    val maxItems: Int? = null
) {
    fun toPaginationOptions(): PaginationOptions = PaginationOptions(maxItems = maxItems, page = page)
}

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

/** Request body for UpdateWormhole. */
data class UpdateWormholeBody(
    val destinationRecordingId: Long
)

/** Request body for CreateWormhole. */
data class CreateWormholeBody(
    val destinationRecordingId: Long
)

