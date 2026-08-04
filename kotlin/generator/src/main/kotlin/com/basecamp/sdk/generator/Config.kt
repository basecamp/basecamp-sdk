package com.basecamp.sdk.generator

/**
 * Tag to service name mapping overrides.
 * Ported from typescript/scripts/generate-services.ts
 */
val TAG_TO_SERVICE = mapOf(
    "Card Tables" to "CardTables",
    "Campfire" to "Campfires",
    "Todos" to "Todos",
    "Messages" to "Messages",
    "Files" to "Files",
    "Forwards" to "Forwards",
    "Schedule" to "Schedules",
    "People" to "People",
    "Projects" to "Projects",
    "Automation" to "Automation",
    "ClientFeatures" to "ClientFeatures",
    "Boosts" to "Boosts",
    "Untagged" to "Miscellaneous",
)

/**
 * Service split configuration — some tags map to multiple service classes.
 */
val SERVICE_SPLITS: Map<String, Map<String, List<String>>> = mapOf(
    "Campfire" to mapOf(
        "Campfires" to listOf(
            "GetCampfire", "ListCampfires",
            "ListChatbots", "CreateChatbot", "GetChatbot", "UpdateChatbot", "DeleteChatbot",
            "ListCampfireLines", "CreateCampfireLine", "GetCampfireLine", "UpdateCampfireLine", "DeleteCampfireLine",
            "ListCampfireUploads", "CreateCampfireUpload",
        ),
    ),
    "Card Tables" to mapOf(
        "CardTables" to listOf("GetCardTable"),
        "Cards" to listOf("GetCard", "UpdateCard", "MoveCard", "CreateCard", "ListCards"),
        "CardColumns" to listOf(
            "GetCardColumn", "UpdateCardColumn", "SetCardColumnColor",
            "EnableCardColumnOnHold", "DisableCardColumnOnHold",
            "CreateCardColumn", "MoveCardColumn",
            "SubscribeToCardColumn", "UnsubscribeFromCardColumn",
        ),
        "CardSteps" to listOf(
            "GetCardStep", "CreateCardStep", "UpdateCardStep", "SetCardStepCompletion",
            "RepositionCardStep",
        ),
        "Wormholes" to listOf("CreateWormhole", "UpdateWormhole", "DeleteWormhole"),
    ),
    "Files" to mapOf(
        "Attachments" to listOf("CreateAttachment"),
        "Uploads" to listOf("GetUpload", "UpdateUpload", "ListUploads", "CreateUpload", "ListUploadVersions"),
        "Vaults" to listOf("GetVault", "UpdateVault", "ListVaults", "CreateVault"),
        "Documents" to listOf("GetDocument", "ReplaceDocument", "ListDocuments", "CreateDocument"),
    ),
    "Automation" to mapOf(
        "Tools" to listOf("GetTool", "UpdateTool", "DeleteTool", "CreateTool", "EnableTool", "DisableTool", "RepositionTool"),
        "Recordings" to listOf("ArchiveRecording", "UnarchiveRecording", "TrashRecording", "ListRecordings"),
        "Webhooks" to listOf("ListWebhooks", "CreateWebhook", "GetWebhook", "UpdateWebhook", "DeleteWebhook"),
        "Events" to listOf("ListEvents"),
        "Lineup" to listOf("CreateLineupMarker", "UpdateLineupMarker", "DeleteLineupMarker"),
        "Search" to listOf("Search", "GetSearchMetadata"),
        "Templates" to listOf(
            "ListTemplates", "CreateTemplate", "GetTemplate", "UpdateTemplate",
            "DeleteTemplate", "CreateProjectFromTemplate", "GetProjectConstruction",
        ),
        "Checkins" to listOf(
            "GetQuestionnaire", "ListQuestions", "CreateQuestion", "GetQuestion",
            "UpdateQuestion", "ListAnswers", "CreateAnswer", "GetAnswer", "UpdateAnswer",
        ),
    ),
    "Messages" to mapOf(
        "Messages" to listOf("GetMessage", "UpdateMessage", "CreateMessage", "ListMessages", "PinMessage", "UnpinMessage"),
        "MessageBoards" to listOf("GetMessageBoard"),
        "MessageTypes" to listOf("ListMessageTypes", "CreateMessageType", "GetMessageType", "UpdateMessageType", "DeleteMessageType"),
        "Comments" to listOf("GetComment", "UpdateComment", "ListComments", "CreateComment"),
    ),
    "People" to mapOf(
        "People" to listOf("GetMyProfile", "ListPeople", "GetPerson", "ListProjectPeople", "UpdateProjectAccess", "ListPingablePeople", "ListAssignablePeople"),
        "Subscriptions" to listOf("GetSubscription", "Subscribe", "Unsubscribe", "UpdateSubscription"),
    ),
    "Schedule" to mapOf(
        "Schedules" to listOf(
            "GetSchedule", "UpdateScheduleSettings", "ListScheduleEntries",
            "CreateScheduleEntry", "GetScheduleEntry", "ReplaceScheduleEntry", "GetScheduleEntryOccurrence",
        ),
        "Timesheets" to listOf("GetRecordingTimesheet", "GetProjectTimesheet", "GetTimesheetReport", "GetTimesheetEntry", "CreateTimesheetEntry", "UpdateTimesheetEntry", "DestroyTimesheetEntry"),
    ),
    "ClientFeatures" to mapOf(
        "ClientApprovals" to listOf("ListClientApprovals", "GetClientApproval"),
        "ClientCorrespondences" to listOf("ListClientCorrespondences", "GetClientCorrespondence"),
        "ClientReplies" to listOf("ListClientReplies", "GetClientReply"),
        "ClientVisibility" to listOf("SetClientVisibility"),
    ),
    "Todos" to mapOf(
        "Todos" to listOf("ListTodos", "CreateTodo", "CreateTodosetTodo", "GetTodo", "ReplaceTodo", "CompleteTodo", "UncompleteTodo"),
        "Todolists" to listOf("GetTodolistOrGroup", "UpdateTodolistOrGroup", "ListTodolists", "CreateTodolist", "RepositionTodolist"),
        "Todosets" to listOf("GetTodoset"),
        "HillCharts" to listOf("GetHillChart", "UpdateHillChartSettings"),
        "TodolistGroups" to listOf("ListTodolistGroups", "CreateTodolistGroup", "RepositionTodolistGroup"),
    ),
    "Untagged" to mapOf(
        "Timeline" to listOf("GetProjectTimeline"),
        "Reports" to listOf("GetProgressReport", "GetUpcomingSchedule", "GetAssignedTodos", "GetOverdueTodos", "GetPersonProgress"),
        "Checkins" to listOf(
            "GetQuestionReminders", "ListQuestionAnswerers", "GetAnswersByPerson",
            "UpdateQuestionNotificationSettings", "PauseQuestion", "ResumeQuestion",
        ),
        "Todos" to listOf("RepositionTodo"),
        "People" to listOf("ListAssignablePeople"),
        "CardColumns" to listOf("SubscribeToCardColumn", "UnsubscribeFromCardColumn"),
    ),
)

/**
 * Services emitted as `open class` so a hand-written subclass in
 * com.basecamp.sdk.services can add convenience methods (e.g. Todos
 * gains merge-safe update/edit on top of the generated replace).
 */
val EXTENSIBLE_SERVICES = setOf("Todos", "Todolists", "Cards", "Uploads", "Documents", "Schedules")

/**
 * Services whose accessor constructs and declares a hand-written subclass
 * instead of the generated class, keyed by service name to the subclass's
 * fully-qualified name. The subclass's extra methods become visible on the
 * accessor without any caller imports.
 */
val HAND_WRITTEN_SERVICES = mapOf(
    "Todos" to "com.basecamp.sdk.services.TodosService",
    "Todolists" to "com.basecamp.sdk.services.TodolistsService",
    "Cards" to "com.basecamp.sdk.services.CardsService",
    "Uploads" to "com.basecamp.sdk.services.UploadsService",
    "Documents" to "com.basecamp.sdk.services.DocumentsService",
    "Schedules" to "com.basecamp.sdk.services.SchedulesService",
)

/**
 * Verb extraction patterns for operationId → method name mapping.
 */
val VERB_PATTERNS = listOf(
    "Subscribe" to "subscribe",
    "Unsubscribe" to "unsubscribe",
    "List" to "list",
    "Get" to "get",
    "Create" to "create",
    "Update" to "update",
    "Replace" to "replace",
    "Delete" to "delete",
    "Trash" to "trash",
    "Archive" to "archive",
    "Unarchive" to "unarchive",
    "Complete" to "complete",
    "Uncomplete" to "uncomplete",
    "Enable" to "enable",
    "Disable" to "disable",
    "Reposition" to "reposition",
    "Move" to "move",
    "Clone" to "clone",
    "Set" to "set",
    "Pin" to "pin",
    "Unpin" to "unpin",
    "Pause" to "pause",
    "Resume" to "resume",
    "Search" to "search",
)

/**
 * Method name overrides for specific operationIds.
 */
val METHOD_NAME_OVERRIDES = mapOf(
    "GetMyProfile" to "me",
    // "bookmark(id)" reads as the action; keep the getter explicit.
    "GetBookmark" to "getBookmark",
    // "folder(id)" reads as a noun with no verb; the rest of the family is
    // listFolders/createFolder/updateFolder/deleteFolder, and Ruby and Python
    // already emit get_folder. Keep all six SDKs on one name.
    "GetFolder" to "getFolder",
    // "myNote()" reads oddly; keep the getter explicit.
    "GetMyNote" to "getMyNote",
    // "calendar(id)" is ambiguous with the service noun; keep the getter explicit.
    "GetCalendar" to "getCalendar",
    "GetTodolistOrGroup" to "get",
    // The plain `update` name belongs to the merge-safe composite; the raw
    // single-PUT path keeps a name that says what it does. BC3 rebuilds the
    // todolist from the permitted params, so omission clears. See #374.
    "UpdateTodolistOrGroup" to "replace",
    "SetCardColumnColor" to "setColor",
    "EnableCardColumnOnHold" to "enableOnHold",
    "DisableCardColumnOnHold" to "disableOnHold",
    "RepositionCardStep" to "reposition",
    "CreateCardStep" to "create",
    "UpdateCardStep" to "update",
    // The plain `update` name belongs to the merge-safe composite; the raw
    // single-PUT path keeps a name that says what it does. See #467.
    "UpdateCard" to "updateVerbatim",
    "SetCardStepCompletion" to "setCompletion",
    "GetQuestionnaire" to "getQuestionnaire",
    "GetQuestion" to "getQuestion",
    "GetAnswer" to "getAnswer",
    "ListQuestions" to "listQuestions",
    "ListAnswers" to "listAnswers",
    "CreateQuestion" to "createQuestion",
    "CreateAnswer" to "createAnswer",
    "UpdateQuestion" to "updateQuestion",
    "UpdateAnswer" to "updateAnswer",
    "GetQuestionReminders" to "reminders",
    "GetAnswersByPerson" to "byPerson",
    "ListQuestionAnswerers" to "answerers",
    "UpdateQuestionNotificationSettings" to "updateNotificationSettings",
    "PauseQuestion" to "pause",
    "ResumeQuestion" to "resume",
    "GetSearchMetadata" to "metadata",
    "Search" to "search",
    "CreateProjectFromTemplate" to "createProject",
    "GetProjectConstruction" to "getConstruction",
    "GetRecordingTimesheet" to "forRecording",
    "GetProjectTimesheet" to "forProject",
    "GetTimesheetReport" to "report",
    "GetTimesheetEntry" to "get",
    "CreateTimesheetEntry" to "create",
    "UpdateTimesheetEntry" to "update",
    "DestroyTimesheetEntry" to "destroy",
    "GetProgressReport" to "progress",
    "GetUpcomingSchedule" to "upcoming",
    "GetAssignedTodos" to "assigned",
    "GetOverdueTodos" to "overdue",
    "GetPersonProgress" to "personProgress",
    "SubscribeToCardColumn" to "subscribeToColumn",
    "UnsubscribeFromCardColumn" to "unsubscribeFromColumn",
    "ListRecordingBoosts" to "listForRecording",
    "CreateRecordingBoost" to "createForRecording",
    "ListEventBoosts" to "listForEvent",
    "CreateEventBoost" to "createForEvent",
    "SetClientVisibility" to "setVisibility",
    "GetCampfire" to "get",
    "ListCampfires" to "list",
    "ListChatbots" to "listChatbots",
    "CreateChatbot" to "createChatbot",
    "GetChatbot" to "getChatbot",
    "UpdateChatbot" to "updateChatbot",
    "DeleteChatbot" to "deleteChatbot",
    "ListCampfireLines" to "listLines",
    "CreateCampfireLine" to "createLine",
    "GetCampfireLine" to "getLine",
    "UpdateCampfireLine" to "updateLine",
    "DeleteCampfireLine" to "deleteLine",
    "ListCampfireUploads" to "listUploads",
    "CreateCampfireUpload" to "createUpload",
    "GetForward" to "get",
    "ListForwards" to "list",
    "GetForwardReply" to "getReply",
    "ListForwardReplies" to "listReplies",
    "GetInbox" to "getInbox",
    "GetUpload" to "get",
    "UpdateUpload" to "update",
    "ListUploads" to "list",
    "CreateUpload" to "create",
    "ListUploadVersions" to "listVersions",
    "GetMessage" to "get",
    "UpdateMessage" to "update",
    "CreateMessage" to "create",
    "ListMessages" to "list",
    "PinMessage" to "pin",
    "UnpinMessage" to "unpin",
    "GetMessageBoard" to "get",
    "GetMessageType" to "get",
    "UpdateMessageType" to "update",
    "CreateMessageType" to "create",
    "ListMessageTypes" to "list",
    "DeleteMessageType" to "delete",
    "GetComment" to "get",
    "UpdateComment" to "update",
    "CreateComment" to "create",
    "ListComments" to "list",
    "ListProjectPeople" to "listForProject",
    "ListPingablePeople" to "listPingable",
    "ListAssignablePeople" to "listAssignable",
    "GetSchedule" to "get",
    "UpdateScheduleSettings" to "updateSettings",
    "GetHillChart" to "get",
    "UpdateHillChartSettings" to "updateSettings",
    "GetScheduleEntry" to "getEntry",
    // The plain `updateEntry` name belongs to the merge-safe composite; the raw
    // single-PUT path keeps a name that says what it does. Without the override
    // the algorithm yields a bare `replace` (scheduleentry is a SIMPLE_RESOURCE),
    // which reads as "replace the schedule". See #547.
    "ReplaceScheduleEntry" to "replaceEntry",
    "CreateScheduleEntry" to "createEntry",
    "ListScheduleEntries" to "listEntries",
    "GetScheduleEntryOccurrence" to "getEntryOccurrence",
)

val RESOURCE_TYPE_OVERRIDES = mapOf(
    "UpdateHillChartSettings" to "hill_chart",
    // The whole family reports "bookmark"; the inferred "my_bookmark" would
    // split the list operation into its own telemetry category.
    "ListMyBookmarks" to "bookmark",
    // Creates and returns a Todo; the inferred "todoset_todo" would split
    // loose-to-do operations into their own telemetry category.
    "CreateTodosetTodo" to "todo",
    // "Destroy" is not a verb pattern, so inference falls through to the generic
    // "resource" and would split this delete away from the get/update siblings
    // that report "timesheet_entry".
    "DestroyTimesheetEntry" to "timesheet_entry",
)

/**
 * Maps OpenAPI schema names to friendly Kotlin type names.
 */
val TYPE_ALIASES = mapOf(
    "Todo" to "Todo",
    "Person" to "Person",
    "Project" to "Project",
    "Message" to "Message",
    "Comment" to "Comment",
    "Card" to "Card",
    "CardTable" to "CardTable",
    "CardColumn" to "CardColumn",
    "CardStep" to "CardStep",
    "Wormhole" to "Wormhole",
    "Campfire" to "Campfire",
    "CampfireLine" to "CampfireLine",
    "Chatbot" to "Chatbot",
    "Webhook" to "Webhook",
    "Vault" to "Vault",
    "Document" to "Document",
    "Upload" to "Upload",
    "Schedule" to "Schedule",
    "ScheduleEntry" to "ScheduleEntry",
    "Recording" to "Recording",
    "Template" to "Template",
    "Todolist" to "Todolist",
    "Todoset" to "Todoset",
    "Questionnaire" to "Questionnaire",
    "Question" to "Question",
    "QuestionAnswer" to "Answer",
    "Subscription" to "Subscription",
    "Bookmark" to "Bookmark",
    "BookmarkStatus" to "BookmarkStatus",
    "Folder" to "Folder",
    "FolderWithProjects" to "FolderWithProjects",
    "Draft" to "Draft",
    "MyNote" to "MyNote",
    "Calendar" to "Calendar",
    // DraftParent is referenced only through the nullable anyOf union, which the
    // supporting-type scanner does not traverse — list it explicitly.
    "DraftParent" to "DraftParent",
    "Forward" to "Forward",
    "ForwardReply" to "ForwardReply",
    "Inbox" to "Inbox",
    "MessageBoard" to "MessageBoard",
    "MessageType" to "MessageType",
    "Event" to "Event",
    "Tool" to "Tool",
    "LineupMarker" to "LineupMarker",
    "ClientApproval" to "ClientApproval",
    "ClientCorrespondence" to "ClientCorrespondence",
    "ClientReply" to "ClientReply",
    "Boost" to "Boost",
    "Notification" to "Notification",
    "EverythingFile" to "EverythingFile",
    "BucketTodosGroup" to "BucketTodosGroup",
    "BucketCardsGroup" to "BucketCardsGroup",
    "TimelineEvent" to "TimelineEvent",
    "TimesheetEntry" to "TimesheetEntry",
    "HillChart" to "HillChart",
    "HillChartDot" to "HillChartDot",
)

/**
 * Simple resource names (lowercase) — when a method name strips a verb prefix
 * and what's left is one of these, the method is just the verb (e.g., "list", "get").
 */
val SIMPLE_RESOURCES = setOf(
    "todo", "todos", "todolist", "todolists", "todoset",
    "message", "messages", "comment", "comments",
    "card", "cards", "cardtable", "cardcolumn", "cardstep", "column", "step",
    "project", "projects", "person", "people",
    "campfire", "campfires", "chatbot", "chatbots",
    "webhook", "webhooks", "vault", "vaults", "document", "documents",
    "upload", "uploads", "schedule", "scheduleentry", "scheduleentries",
    "event", "events", "recording", "recordings", "template", "templates",
    "attachment", "question", "questions", "answer", "answers", "questionnaire",
    "subscription", "forward", "forwards", "inbox", "messageboard",
    "messagetype", "messagetypes", "tool", "lineupmarker",
    "clientapproval", "clientapprovals", "clientcorrespondence", "clientcorrespondences",
    "clientreply", "clientreplies", "forwardreply", "forwardreplies",
    "campfireline", "campfirelines", "todolistgroup", "todolistgroups",
    "todolistorgroup", "uploadversions",
    "boost", "boosts",
    "hillchart", "hillcharts",
    "wormhole", "wormholes",
)

/**
 * Operations whose options parameter changed type from bare `PaginationOptions`
 * to an operation-specific `<Operation>Options` when they gained their first
 * optional query parameter — the `page` wiring in #561.
 *
 * Each one gets a source-compatibility overload taking `PaginationOptions`, so
 * a call site written against the old signature still compiles (the pre-1.0
 * policy in kotlin/README.md promises source compatibility).
 *
 * The set is frozen. An entry cannot be removed without breaking the call sites
 * the overload exists for, and nothing new belongs in it: an operation that has
 * always had its own options class never had a `PaginationOptions` signature to
 * stay compatible with, and giving it the overload anyway makes an untyped
 * callable reference — `client.bookmarks::listMyBookmarks` — ambiguous between
 * two applicable one-argument candidates, so it stops compiling.
 */
val PAGINATION_OPTIONS_COMPAT_OVERLOADS = setOf(
    "GetAnswersByPerson",
    "GetPersonProgress",
    "GetProgressReport",
    "GetProjectTimeline",
    "GetQuestionReminders",
    "ListAnswers",
    "ListCampfires",
    "ListCards",
    "ListClientReplies",
    "ListComments",
    "ListDocuments",
    "ListEventBoosts",
    "ListEvents",
    "ListForwardReplies",
    "ListGaugeNeedles",
    "ListPeople",
    "ListProjectPeople",
    "ListQuestions",
    "ListRecordingBoosts",
    "ListTodolistGroups",
    "ListUploads",
    "ListVaults",
)
