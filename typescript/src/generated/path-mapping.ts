/**
 * Maps HTTP method + path to OpenAPI operationId.
 *
 * @generated from OpenAPI spec - do not edit directly
 * Run `npm run generate` to regenerate.
 */

export const PATH_TO_OPERATION: Record<string, string> = {
  // Attachments
  "POST:/{accountId}/attachments.json": "CreateAttachment",

  // Other
  "DELETE:/{accountId}/buckets/{projectId}/boosts/{boostId}": "DeleteBoost",

  "GET:/{accountId}/buckets/{projectId}/boosts/{boostId}": "GetBoost",

  "POST:/{accountId}/buckets/{projectId}/dock/tools.json": "CloneTool",

  "DELETE:/{accountId}/buckets/{projectId}/dock/tools/{toolId}": "DeleteTool",

  "GET:/{accountId}/buckets/{projectId}/dock/tools/{toolId}": "GetTool",

  "PUT:/{accountId}/buckets/{projectId}/dock/tools/{toolId}": "UpdateTool",

  "GET:/{accountId}/chats.json": "ListCampfires",

  "POST:/{accountId}/lineup/markers.json": "CreateLineupMarker",

  "DELETE:/{accountId}/lineup/markers/{markerId}": "DeleteLineupMarker",

  "PUT:/{accountId}/lineup/markers/{markerId}": "UpdateLineupMarker",

  "GET:/{accountId}/reports/progress.json": "GetProgressReport",

  "GET:/{accountId}/reports/schedules/upcoming.json": "GetUpcomingSchedule",

  "GET:/{accountId}/reports/timesheet.json": "GetTimesheetReport",

  "GET:/{accountId}/reports/todos/assigned.json": "ListAssignablePeople",

  "GET:/{accountId}/reports/todos/assigned/{personId}": "GetAssignedTodos",

  "GET:/{accountId}/reports/todos/overdue.json": "GetOverdueTodos",

  "GET:/{accountId}/reports/users/progress/{personId}": "GetPersonProgress",

  // Card Tables
  "GET:/{accountId}/buckets/{projectId}/card_tables/{cardTableId}": "GetCardTable",

  "POST:/{accountId}/buckets/{projectId}/card_tables/{cardTableId}/columns.json": "CreateCardColumn",

  "POST:/{accountId}/buckets/{projectId}/card_tables/{cardTableId}/moves.json": "MoveCardColumn",

  "GET:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}": "GetCard",

  "PUT:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}": "UpdateCard",

  "POST:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/moves.json": "MoveCard",

  "POST:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/positions.json": "RepositionCardStep",

  "POST:/{accountId}/buckets/{projectId}/card_tables/cards/{cardId}/steps.json": "CreateCardStep",

  "GET:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}": "GetCardColumn",

  "PUT:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}": "UpdateCardColumn",

  "PUT:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/color.json": "SetCardColumnColor",

  "DELETE:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json": "DisableCardColumnOnHold",

  "POST:/{accountId}/buckets/{projectId}/card_tables/columns/{columnId}/on_hold.json": "EnableCardColumnOnHold",

  "GET:/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/cards.json": "ListCards",

  "POST:/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/cards.json": "CreateCard",

  "DELETE:/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/subscription.json": "UnsubscribeFromCardColumn",

  "POST:/{accountId}/buckets/{projectId}/card_tables/lists/{columnId}/subscription.json": "SubscribeToCardColumn",

  "PUT:/{accountId}/buckets/{projectId}/card_tables/steps/{stepId}": "UpdateCardStep",

  "PUT:/{accountId}/buckets/{projectId}/card_tables/steps/{stepId}/completions.json": "SetCardStepCompletion",

  // Message Types
  "GET:/{accountId}/buckets/{projectId}/categories.json": "ListMessageTypes",

  "POST:/{accountId}/buckets/{projectId}/categories.json": "CreateMessageType",

  "DELETE:/{accountId}/buckets/{projectId}/categories/{typeId}": "DeleteMessageType",

  "GET:/{accountId}/buckets/{projectId}/categories/{typeId}": "GetMessageType",

  "PUT:/{accountId}/buckets/{projectId}/categories/{typeId}": "UpdateMessageType",

  // Campfires
  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}": "GetCampfire",

  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations.json": "ListChatbots",

  "POST:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations.json": "CreateChatbot",

  "DELETE:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "DeleteChatbot",

  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "GetChatbot",

  "PUT:/{accountId}/buckets/{projectId}/chats/{campfireId}/integrations/{chatbotId}": "UpdateChatbot",

  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}/lines.json": "ListCampfireLines",

  "POST:/{accountId}/buckets/{projectId}/chats/{campfireId}/lines.json": "CreateCampfireLine",

  "DELETE:/{accountId}/buckets/{projectId}/chats/{campfireId}/lines/{lineId}": "DeleteCampfireLine",

  "GET:/{accountId}/buckets/{projectId}/chats/{campfireId}/lines/{lineId}": "GetCampfireLine",

  // Client Features
  "GET:/{accountId}/buckets/{projectId}/client/approvals.json": "ListClientApprovals",

  "GET:/{accountId}/buckets/{projectId}/client/approvals/{approvalId}": "GetClientApproval",

  "GET:/{accountId}/buckets/{projectId}/client/correspondences.json": "ListClientCorrespondences",

  "GET:/{accountId}/buckets/{projectId}/client/correspondences/{correspondenceId}": "GetClientCorrespondence",

  "GET:/{accountId}/buckets/{projectId}/client/recordings/{recordingId}/replies.json": "ListClientReplies",

  "GET:/{accountId}/buckets/{projectId}/client/recordings/{recordingId}/replies/{replyId}": "GetClientReply",

  // Comments
  "GET:/{accountId}/buckets/{projectId}/comments/{commentId}": "GetComment",

  "PUT:/{accountId}/buckets/{projectId}/comments/{commentId}": "UpdateComment",

  // Documents
  "GET:/{accountId}/buckets/{projectId}/documents/{documentId}": "GetDocument",

  "PUT:/{accountId}/buckets/{projectId}/documents/{documentId}": "UpdateDocument",

  // Inbox
  "GET:/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}": "GetForward",

  "GET:/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json": "ListForwardReplies",

  "POST:/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies.json": "CreateForwardReply",

  "GET:/{accountId}/buckets/{projectId}/inbox_forwards/{forwardId}/replies/{replyId}": "GetForwardReply",

  "GET:/{accountId}/buckets/{projectId}/inboxes/{inboxId}": "GetInbox",

  "GET:/{accountId}/buckets/{projectId}/inboxes/{inboxId}/forwards.json": "ListForwards",

  // Message Boards
  "GET:/{accountId}/buckets/{projectId}/message_boards/{boardId}": "GetMessageBoard",

  "GET:/{accountId}/buckets/{projectId}/message_boards/{boardId}/messages.json": "ListMessages",

  "POST:/{accountId}/buckets/{projectId}/message_boards/{boardId}/messages.json": "CreateMessage",

  // Messages
  "GET:/{accountId}/buckets/{projectId}/messages/{messageId}": "GetMessage",

  "PUT:/{accountId}/buckets/{projectId}/messages/{messageId}": "UpdateMessage",

  // Question Answers
  "GET:/{accountId}/buckets/{projectId}/question_answers/{answerId}": "GetAnswer",

  "PUT:/{accountId}/buckets/{projectId}/question_answers/{answerId}": "UpdateAnswer",

  // Questionnaires
  "GET:/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}": "GetQuestionnaire",

  "GET:/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json": "ListQuestions",

  "POST:/{accountId}/buckets/{projectId}/questionnaires/{questionnaireId}/questions.json": "CreateQuestion",

  // Questions
  "GET:/{accountId}/buckets/{projectId}/questions/{questionId}": "GetQuestion",

  "PUT:/{accountId}/buckets/{projectId}/questions/{questionId}": "UpdateQuestion",

  "GET:/{accountId}/buckets/{projectId}/questions/{questionId}/answers.json": "ListAnswers",

  "POST:/{accountId}/buckets/{projectId}/questions/{questionId}/answers.json": "CreateAnswer",

  "GET:/{accountId}/buckets/{projectId}/questions/{questionId}/answers/by.json": "ListQuestionAnswerers",

  "GET:/{accountId}/buckets/{projectId}/questions/{questionId}/answers/by/{personId}": "GetAnswersByPerson",

  "PUT:/{accountId}/buckets/{projectId}/questions/{questionId}/notification_settings.json": "UpdateQuestionNotificationSettings",

  "DELETE:/{accountId}/buckets/{projectId}/questions/{questionId}/pause.json": "ResumeQuestion",

  "POST:/{accountId}/buckets/{projectId}/questions/{questionId}/pause.json": "PauseQuestion",

  // Recordings
  "DELETE:/{accountId}/buckets/{projectId}/recordings/{messageId}/pin.json": "UnpinMessage",

  "POST:/{accountId}/buckets/{projectId}/recordings/{messageId}/pin.json": "PinMessage",

  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}": "GetRecording",

  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/boosts.json": "ListRecordingBoosts",

  "POST:/{accountId}/buckets/{projectId}/recordings/{recordingId}/boosts.json": "CreateRecordingBoost",

  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/client_visibility.json": "SetClientVisibility",

  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/comments.json": "ListComments",

  "POST:/{accountId}/buckets/{projectId}/recordings/{recordingId}/comments.json": "CreateComment",

  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/events.json": "ListEvents",

  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/events/{eventId}/boosts.json": "ListEventBoosts",

  "POST:/{accountId}/buckets/{projectId}/recordings/{recordingId}/events/{eventId}/boosts.json": "CreateEventBoost",

  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/active.json": "UnarchiveRecording",

  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/archived.json": "ArchiveRecording",

  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/status/trashed.json": "TrashRecording",

  "DELETE:/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json": "Unsubscribe",

  "GET:/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json": "GetSubscription",

  "POST:/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json": "Subscribe",

  "PUT:/{accountId}/buckets/{projectId}/recordings/{recordingId}/subscription.json": "UpdateSubscription",
  "DELETE:/{accountId}/buckets/{projectId}/recordings/{toolId}/position.json": "DisableTool",

  "POST:/{accountId}/buckets/{projectId}/recordings/{toolId}/position.json": "EnableTool",

  "PUT:/{accountId}/buckets/{projectId}/recordings/{toolId}/position.json": "RepositionTool",

  // Schedule Entries
  "GET:/{accountId}/buckets/{projectId}/schedule_entries/{entryId}": "GetScheduleEntry",

  "PUT:/{accountId}/buckets/{projectId}/schedule_entries/{entryId}": "UpdateScheduleEntry",

  "GET:/{accountId}/buckets/{projectId}/schedule_entries/{entryId}/occurrences/{date}": "GetScheduleEntryOccurrence",

  // Schedules
  "GET:/{accountId}/buckets/{projectId}/schedules/{scheduleId}": "GetSchedule",

  "PUT:/{accountId}/buckets/{projectId}/schedules/{scheduleId}": "UpdateScheduleSettings",

  "GET:/{accountId}/buckets/{projectId}/schedules/{scheduleId}/entries.json": "ListScheduleEntries",

  "POST:/{accountId}/buckets/{projectId}/schedules/{scheduleId}/entries.json": "CreateScheduleEntry",

  // Todolists
  "PUT:/{accountId}/buckets/{projectId}/todolists/{groupId}/position.json": "RepositionTodolistGroup",

  "GET:/{accountId}/buckets/{projectId}/todolists/{id}": "GetTodolistOrGroup",

  "PUT:/{accountId}/buckets/{projectId}/todolists/{id}": "UpdateTodolistOrGroup",

  "GET:/{accountId}/buckets/{projectId}/todolists/{todolistId}/groups.json": "ListTodolistGroups",

  "POST:/{accountId}/buckets/{projectId}/todolists/{todolistId}/groups.json": "CreateTodolistGroup",

  "GET:/{accountId}/buckets/{projectId}/todolists/{todolistId}/todos.json": "ListTodos",

  "POST:/{accountId}/buckets/{projectId}/todolists/{todolistId}/todos.json": "CreateTodo",

  // Todos
  "DELETE:/{accountId}/buckets/{projectId}/todos/{todoId}": "TrashTodo",

  "GET:/{accountId}/buckets/{projectId}/todos/{todoId}": "GetTodo",

  "PUT:/{accountId}/buckets/{projectId}/todos/{todoId}": "UpdateTodo",

  "DELETE:/{accountId}/buckets/{projectId}/todos/{todoId}/completion.json": "UncompleteTodo",

  "POST:/{accountId}/buckets/{projectId}/todos/{todoId}/completion.json": "CompleteTodo",

  "PUT:/{accountId}/buckets/{projectId}/todos/{todoId}/position.json": "RepositionTodo",

  // Todosets
  "GET:/{accountId}/buckets/{projectId}/todosets/{todosetId}": "GetTodoset",

  "GET:/{accountId}/buckets/{projectId}/todosets/{todosetId}/todolists.json": "ListTodolists",

  "POST:/{accountId}/buckets/{projectId}/todosets/{todosetId}/todolists.json": "CreateTodolist",

  // Uploads
  "GET:/{accountId}/buckets/{projectId}/uploads/{uploadId}": "GetUpload",

  "PUT:/{accountId}/buckets/{projectId}/uploads/{uploadId}": "UpdateUpload",

  "GET:/{accountId}/buckets/{projectId}/uploads/{uploadId}/versions.json": "ListUploadVersions",

  // Vaults
  "GET:/{accountId}/buckets/{projectId}/vaults/{vaultId}": "GetVault",

  "PUT:/{accountId}/buckets/{projectId}/vaults/{vaultId}": "UpdateVault",

  "GET:/{accountId}/buckets/{projectId}/vaults/{vaultId}/documents.json": "ListDocuments",

  "POST:/{accountId}/buckets/{projectId}/vaults/{vaultId}/documents.json": "CreateDocument",

  "GET:/{accountId}/buckets/{projectId}/vaults/{vaultId}/uploads.json": "ListUploads",

  "POST:/{accountId}/buckets/{projectId}/vaults/{vaultId}/uploads.json": "CreateUpload",

  "GET:/{accountId}/buckets/{projectId}/vaults/{vaultId}/vaults.json": "ListVaults",

  "POST:/{accountId}/buckets/{projectId}/vaults/{vaultId}/vaults.json": "CreateVault",

  // Webhooks
  "GET:/{accountId}/buckets/{projectId}/webhooks.json": "ListWebhooks",

  "POST:/{accountId}/buckets/{projectId}/webhooks.json": "CreateWebhook",

  "DELETE:/{accountId}/buckets/{projectId}/webhooks/{webhookId}": "DeleteWebhook",

  "GET:/{accountId}/buckets/{projectId}/webhooks/{webhookId}": "GetWebhook",

  "PUT:/{accountId}/buckets/{projectId}/webhooks/{webhookId}": "UpdateWebhook",

  // People
  "GET:/{accountId}/circles/people.json": "ListPingablePeople",

  "GET:/{accountId}/people.json": "ListPeople",

  "GET:/{accountId}/people/{personId}": "GetPerson",

  // My Profile
  "GET:/{accountId}/my/profile.json": "GetMyProfile",

  "GET:/{accountId}/my/question_reminders.json": "GetQuestionReminders",

  // Projects
  "GET:/{accountId}/projects.json": "ListProjects",

  "POST:/{accountId}/projects.json": "CreateProject",

  "DELETE:/{accountId}/projects/{projectId}": "TrashProject",

  "GET:/{accountId}/projects/{projectId}": "GetProject",

  "PUT:/{accountId}/projects/{projectId}": "UpdateProject",

  "GET:/{accountId}/projects/{projectId}/people.json": "ListProjectPeople",

  "PUT:/{accountId}/projects/{projectId}/people/users.json": "UpdateProjectAccess",

  "GET:/{accountId}/projects/{projectId}/recordings/{recordingId}/timesheet.json": "GetRecordingTimesheet",

  "POST:/{accountId}/projects/{projectId}/recordings/{recordingId}/timesheet/entries.json": "CreateTimesheetEntry",

  "GET:/{accountId}/projects/{projectId}/timeline.json": "GetProjectTimeline",

  "GET:/{accountId}/projects/{projectId}/timesheet.json": "GetProjectTimesheet",

  "GET:/{accountId}/projects/{projectId}/timesheet/entries/{entryId}": "GetTimesheetEntry",

  "PUT:/{accountId}/projects/{projectId}/timesheet/entries/{entryId}": "UpdateTimesheetEntry",

  "GET:/{accountId}/projects/recordings.json": "ListRecordings",

  // Search
  "GET:/{accountId}/search.json": "Search",

  "GET:/{accountId}/searches/metadata.json": "GetSearchMetadata",

  // Templates
  "GET:/{accountId}/templates.json": "ListTemplates",

  "POST:/{accountId}/templates.json": "CreateTemplate",

  "DELETE:/{accountId}/templates/{templateId}": "DeleteTemplate",

  "GET:/{accountId}/templates/{templateId}": "GetTemplate",

  "PUT:/{accountId}/templates/{templateId}": "UpdateTemplate",

  "POST:/{accountId}/templates/{templateId}/project_constructions.json": "CreateProjectFromTemplate",

  "GET:/{accountId}/templates/{templateId}/project_constructions/{constructionId}": "GetProjectConstruction",

  "GET:/{accountId}/card_tables/{cardTableId}": "GetCardTable",

  "POST:/{accountId}/card_tables/{cardTableId}/columns.json": "CreateCardColumn",

  "POST:/{accountId}/card_tables/{cardTableId}/moves.json": "MoveCardColumn",

  "GET:/{accountId}/card_tables/cards/{cardId}": "GetCard",

  "PUT:/{accountId}/card_tables/cards/{cardId}": "UpdateCard",

  "POST:/{accountId}/card_tables/cards/{cardId}/moves.json": "MoveCard",

  "POST:/{accountId}/card_tables/cards/{cardId}/positions.json": "RepositionCardStep",

  "POST:/{accountId}/card_tables/cards/{cardId}/steps.json": "CreateCardStep",

  "GET:/{accountId}/card_tables/columns/{columnId}": "GetCardColumn",

  "PUT:/{accountId}/card_tables/columns/{columnId}": "UpdateCardColumn",

  "PUT:/{accountId}/card_tables/columns/{columnId}/color.json": "SetCardColumnColor",

  "DELETE:/{accountId}/card_tables/columns/{columnId}/on_hold.json": "DisableCardColumnOnHold",

  "POST:/{accountId}/card_tables/columns/{columnId}/on_hold.json": "EnableCardColumnOnHold",

  "GET:/{accountId}/card_tables/lists/{columnId}/cards.json": "ListCards",

  "POST:/{accountId}/card_tables/lists/{columnId}/cards.json": "CreateCard",

  "DELETE:/{accountId}/card_tables/lists/{columnId}/subscription.json": "UnsubscribeFromCardColumn",

  "POST:/{accountId}/card_tables/lists/{columnId}/subscription.json": "SubscribeToCardColumn",

  "PUT:/{accountId}/card_tables/steps/{stepId}": "UpdateCardStep",

  "PUT:/{accountId}/card_tables/steps/{stepId}/completions.json": "SetCardStepCompletion",

  "GET:/{accountId}/categories.json": "ListMessageTypes",

  "POST:/{accountId}/categories.json": "CreateMessageType",

  "DELETE:/{accountId}/categories/{typeId}": "DeleteMessageType",

  "GET:/{accountId}/categories/{typeId}": "GetMessageType",

  "PUT:/{accountId}/categories/{typeId}": "UpdateMessageType",

  "GET:/{accountId}/chats/{campfireId}": "GetCampfire",

  "GET:/{accountId}/chats/{campfireId}/integrations.json": "ListChatbots",

  "POST:/{accountId}/chats/{campfireId}/integrations.json": "CreateChatbot",

  "DELETE:/{accountId}/chats/{campfireId}/integrations/{chatbotId}": "DeleteChatbot",

  "GET:/{accountId}/chats/{campfireId}/integrations/{chatbotId}": "GetChatbot",

  "PUT:/{accountId}/chats/{campfireId}/integrations/{chatbotId}": "UpdateChatbot",

  "GET:/{accountId}/chats/{campfireId}/lines.json": "ListCampfireLines",

  "POST:/{accountId}/chats/{campfireId}/lines.json": "CreateCampfireLine",

  "DELETE:/{accountId}/chats/{campfireId}/lines/{lineId}": "DeleteCampfireLine",

  "GET:/{accountId}/chats/{campfireId}/lines/{lineId}": "GetCampfireLine",

  "GET:/{accountId}/client/approvals.json": "ListClientApprovals",

  "GET:/{accountId}/client/approvals/{approvalId}": "GetClientApproval",

  "GET:/{accountId}/client/correspondences.json": "ListClientCorrespondences",

  "GET:/{accountId}/client/correspondences/{correspondenceId}": "GetClientCorrespondence",

  "GET:/{accountId}/client/recordings/{recordingId}/replies.json": "ListClientReplies",

  "GET:/{accountId}/client/recordings/{recordingId}/replies/{replyId}": "GetClientReply",

  "GET:/{accountId}/comments/{commentId}": "GetComment",

  "PUT:/{accountId}/comments/{commentId}": "UpdateComment",

  "POST:/{accountId}/dock/tools.json": "CloneTool",

  "DELETE:/{accountId}/dock/tools/{toolId}": "DeleteTool",

  "GET:/{accountId}/dock/tools/{toolId}": "GetTool",

  "PUT:/{accountId}/dock/tools/{toolId}": "UpdateTool",

  "GET:/{accountId}/documents/{documentId}": "GetDocument",

  "PUT:/{accountId}/documents/{documentId}": "UpdateDocument",

  "GET:/{accountId}/inbox_forwards/{forwardId}": "GetForward",

  "GET:/{accountId}/inbox_forwards/{forwardId}/replies.json": "ListForwardReplies",

  "POST:/{accountId}/inbox_forwards/{forwardId}/replies.json": "CreateForwardReply",

  "GET:/{accountId}/inbox_forwards/{forwardId}/replies/{replyId}": "GetForwardReply",

  "GET:/{accountId}/inboxes/{inboxId}": "GetInbox",

  "GET:/{accountId}/inboxes/{inboxId}/forwards.json": "ListForwards",

  "GET:/{accountId}/message_boards/{boardId}": "GetMessageBoard",

  "GET:/{accountId}/message_boards/{boardId}/messages.json": "ListMessages",

  "POST:/{accountId}/message_boards/{boardId}/messages.json": "CreateMessage",

  "GET:/{accountId}/messages/{messageId}": "GetMessage",

  "PUT:/{accountId}/messages/{messageId}": "UpdateMessage",

  "GET:/{accountId}/question_answers/{answerId}": "GetAnswer",

  "PUT:/{accountId}/question_answers/{answerId}": "UpdateAnswer",

  "GET:/{accountId}/questionnaires/{questionnaireId}": "GetQuestionnaire",

  "GET:/{accountId}/questionnaires/{questionnaireId}/questions.json": "ListQuestions",

  "POST:/{accountId}/questionnaires/{questionnaireId}/questions.json": "CreateQuestion",

  "GET:/{accountId}/questions/{questionId}": "GetQuestion",

  "PUT:/{accountId}/questions/{questionId}": "UpdateQuestion",

  "GET:/{accountId}/questions/{questionId}/answers.json": "ListAnswers",

  "POST:/{accountId}/questions/{questionId}/answers.json": "CreateAnswer",

  "GET:/{accountId}/questions/{questionId}/answers/by.json": "ListQuestionAnswerers",

  "GET:/{accountId}/questions/{questionId}/answers/by/{personId}": "GetAnswersByPerson",

  "PUT:/{accountId}/questions/{questionId}/notification_settings.json": "UpdateQuestionNotificationSettings",

  "DELETE:/{accountId}/questions/{questionId}/pause.json": "ResumeQuestion",

  "POST:/{accountId}/questions/{questionId}/pause.json": "PauseQuestion",

  "DELETE:/{accountId}/recordings/{messageId}/pin.json": "UnpinMessage",

  "POST:/{accountId}/recordings/{messageId}/pin.json": "PinMessage",

  "GET:/{accountId}/recordings/{recordingId}": "GetRecording",

  "PUT:/{accountId}/recordings/{recordingId}/client_visibility.json": "SetClientVisibility",

  "GET:/{accountId}/recordings/{recordingId}/comments.json": "ListComments",

  "POST:/{accountId}/recordings/{recordingId}/comments.json": "CreateComment",

  "PUT:/{accountId}/recordings/{recordingId}/status/active.json": "UnarchiveRecording",

  "PUT:/{accountId}/recordings/{recordingId}/status/archived.json": "ArchiveRecording",

  "PUT:/{accountId}/recordings/{recordingId}/status/trashed.json": "TrashRecording",

  "DELETE:/{accountId}/recordings/{recordingId}/subscription.json": "Unsubscribe",

  "GET:/{accountId}/recordings/{recordingId}/subscription.json": "GetSubscription",

  "POST:/{accountId}/recordings/{recordingId}/subscription.json": "Subscribe",

  "PUT:/{accountId}/recordings/{recordingId}/subscription.json": "UpdateSubscription",

  "GET:/{accountId}/recordings/{recordingId}/timesheet.json": "GetRecordingTimesheet",

  "DELETE:/{accountId}/recordings/{toolId}/position.json": "DisableTool",

  "POST:/{accountId}/recordings/{toolId}/position.json": "EnableTool",

  "PUT:/{accountId}/recordings/{toolId}/position.json": "RepositionTool",

  "GET:/{accountId}/schedule_entries/{entryId}": "GetScheduleEntry",

  "PUT:/{accountId}/schedule_entries/{entryId}": "UpdateScheduleEntry",

  "GET:/{accountId}/schedule_entries/{entryId}/occurrences/{date}": "GetScheduleEntryOccurrence",

  "GET:/{accountId}/schedules/{scheduleId}": "GetSchedule",

  "PUT:/{accountId}/schedules/{scheduleId}": "UpdateScheduleSettings",

  "GET:/{accountId}/schedules/{scheduleId}/entries.json": "ListScheduleEntries",

  "POST:/{accountId}/schedules/{scheduleId}/entries.json": "CreateScheduleEntry",

  "GET:/{accountId}/timeline.json": "GetProjectTimeline",

  "GET:/{accountId}/timesheet.json": "GetProjectTimesheet",

  "PUT:/{accountId}/todolists/{groupId}/position.json": "RepositionTodolistGroup",

  "GET:/{accountId}/todolists/{id}": "GetTodolistOrGroup",

  "PUT:/{accountId}/todolists/{id}": "UpdateTodolistOrGroup",

  "GET:/{accountId}/todolists/{todolistId}/groups.json": "ListTodolistGroups",

  "POST:/{accountId}/todolists/{todolistId}/groups.json": "CreateTodolistGroup",

  "GET:/{accountId}/todolists/{todolistId}/todos.json": "ListTodos",

  "POST:/{accountId}/todolists/{todolistId}/todos.json": "CreateTodo",

  "DELETE:/{accountId}/todos/{todoId}": "TrashTodo",

  "GET:/{accountId}/todos/{todoId}": "GetTodo",

  "PUT:/{accountId}/todos/{todoId}": "UpdateTodo",

  "DELETE:/{accountId}/todos/{todoId}/completion.json": "UncompleteTodo",

  "POST:/{accountId}/todos/{todoId}/completion.json": "CompleteTodo",

  "PUT:/{accountId}/todos/{todoId}/position.json": "RepositionTodo",

  "GET:/{accountId}/todosets/{todosetId}": "GetTodoset",

  "GET:/{accountId}/todosets/{todosetId}/todolists.json": "ListTodolists",

  "POST:/{accountId}/todosets/{todosetId}/todolists.json": "CreateTodolist",

  "GET:/{accountId}/uploads/{uploadId}": "GetUpload",

  "PUT:/{accountId}/uploads/{uploadId}": "UpdateUpload",

  "GET:/{accountId}/uploads/{uploadId}/versions.json": "ListUploadVersions",

  "GET:/{accountId}/vaults/{vaultId}": "GetVault",

  "PUT:/{accountId}/vaults/{vaultId}": "UpdateVault",

  "GET:/{accountId}/vaults/{vaultId}/documents.json": "ListDocuments",

  "POST:/{accountId}/vaults/{vaultId}/documents.json": "CreateDocument",

  "GET:/{accountId}/vaults/{vaultId}/uploads.json": "ListUploads",

  "POST:/{accountId}/vaults/{vaultId}/uploads.json": "CreateUpload",

  "GET:/{accountId}/vaults/{vaultId}/vaults.json": "ListVaults",

  "POST:/{accountId}/vaults/{vaultId}/vaults.json": "CreateVault",

  "GET:/{accountId}/webhooks.json": "ListWebhooks",

  "POST:/{accountId}/webhooks.json": "CreateWebhook",

  "DELETE:/{accountId}/webhooks/{webhookId}": "DeleteWebhook",

  "GET:/{accountId}/webhooks/{webhookId}": "GetWebhook",

  "PUT:/{accountId}/webhooks/{webhookId}": "UpdateWebhook",

  // Events
  "GET:/{accountId}/recordings/{recordingId}/events.json": "ListEvents",

};
