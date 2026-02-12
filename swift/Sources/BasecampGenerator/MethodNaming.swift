import Foundation

// MARK: - Verb Patterns

/// Ordered list of verb prefixes for extracting method names from operationIds.
let verbPatterns: [(prefix: String, method: String)] = [
    ("Subscribe", "subscribe"),
    ("Unsubscribe", "unsubscribe"),
    ("List", "list"),
    ("Get", "get"),
    ("Create", "create"),
    ("Update", "update"),
    ("Delete", "delete"),
    ("Trash", "trash"),
    ("Archive", "archive"),
    ("Unarchive", "unarchive"),
    ("Complete", "complete"),
    ("Uncomplete", "uncomplete"),
    ("Enable", "enable"),
    ("Disable", "disable"),
    ("Reposition", "reposition"),
    ("Move", "move"),
    ("Clone", "clone"),
    ("Set", "set"),
    ("Pin", "pin"),
    ("Unpin", "unpin"),
    ("Pause", "pause"),
    ("Resume", "resume"),
    ("Search", "search"),
]

// MARK: - Method Name Overrides

/// Explicit overrides for method name generation.
let methodNameOverrides: [String: String] = [
    "GetMyProfile": "me",
    "GetTodolistOrGroup": "get",
    "UpdateTodolistOrGroup": "update",
    "SetCardColumnColor": "setColor",
    "EnableCardColumnOnHold": "enableOnHold",
    "DisableCardColumnOnHold": "disableOnHold",
    "RepositionCardStep": "reposition",
    "CreateCardStep": "create",
    "UpdateCardStep": "update",
    "SetCardStepCompletion": "setCompletion",
    "GetQuestionnaire": "getQuestionnaire",
    "GetQuestion": "getQuestion",
    "GetAnswer": "getAnswer",
    "ListQuestions": "listQuestions",
    "ListAnswers": "listAnswers",
    "CreateQuestion": "createQuestion",
    "CreateAnswer": "createAnswer",
    "UpdateQuestion": "updateQuestion",
    "UpdateAnswer": "updateAnswer",
    "GetQuestionReminders": "reminders",
    "GetAnswersByPerson": "byPerson",
    "ListQuestionAnswerers": "answerers",
    "UpdateQuestionNotificationSettings": "updateNotificationSettings",
    "PauseQuestion": "pause",
    "ResumeQuestion": "resume",
    "GetSearchMetadata": "metadata",
    "Search": "search",
    "CreateProjectFromTemplate": "createProject",
    "GetProjectConstruction": "getConstruction",
    "GetRecordingTimesheet": "forRecording",
    "GetProjectTimesheet": "forProject",
    "GetTimesheetReport": "report",
    "GetTimesheetEntry": "get",
    "CreateTimesheetEntry": "create",
    "UpdateTimesheetEntry": "update",
    "GetProgressReport": "progress",
    "GetUpcomingSchedule": "upcoming",
    "GetAssignedTodos": "assigned",
    "GetOverdueTodos": "overdue",
    "GetPersonProgress": "personProgress",
    "SubscribeToCardColumn": "subscribeToColumn",
    "UnsubscribeFromCardColumn": "unsubscribeFromColumn",
    "ListRecordingBoosts": "listForRecording",
    "CreateRecordingBoost": "createForRecording",
    "ListEventBoosts": "listForEvent",
    "CreateEventBoost": "createForEvent",
    "SetClientVisibility": "setVisibility",
    "GetCampfire": "get",
    "ListCampfires": "list",
    "ListChatbots": "listChatbots",
    "CreateChatbot": "createChatbot",
    "GetChatbot": "getChatbot",
    "UpdateChatbot": "updateChatbot",
    "DeleteChatbot": "deleteChatbot",
    "ListCampfireLines": "listLines",
    "CreateCampfireLine": "createLine",
    "GetCampfireLine": "getLine",
    "DeleteCampfireLine": "deleteLine",
    "GetForward": "get",
    "ListForwards": "list",
    "GetForwardReply": "getReply",
    "ListForwardReplies": "listReplies",
    "CreateForwardReply": "createReply",
    "GetInbox": "getInbox",
    "GetUpload": "get",
    "UpdateUpload": "update",
    "ListUploads": "list",
    "CreateUpload": "create",
    "ListUploadVersions": "listVersions",
    "GetMessage": "get",
    "UpdateMessage": "update",
    "CreateMessage": "create",
    "ListMessages": "list",
    "PinMessage": "pin",
    "UnpinMessage": "unpin",
    "GetMessageBoard": "get",
    "GetMessageType": "get",
    "UpdateMessageType": "update",
    "CreateMessageType": "create",
    "ListMessageTypes": "list",
    "DeleteMessageType": "delete",
    "GetComment": "get",
    "UpdateComment": "update",
    "CreateComment": "create",
    "ListComments": "list",
    "ListProjectPeople": "listForProject",
    "ListPingablePeople": "listPingable",
    "ListAssignablePeople": "listAssignable",
    "GetSchedule": "get",
    "UpdateScheduleSettings": "updateSettings",
    "GetScheduleEntry": "getEntry",
    "UpdateScheduleEntry": "updateEntry",
    "CreateScheduleEntry": "createEntry",
    "ListScheduleEntries": "listEntries",
    "GetScheduleEntryOccurrence": "getEntryOccurrence",
]

// MARK: - Simple Resources

/// Resource names that are considered "simple" (verb alone suffices as method name).
private let simpleResources: Set<String> = [
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
]

/// Extracts the method name for an operationId.
func extractMethodName(_ operationId: String) -> String {
    if let override = methodNameOverrides[operationId] {
        return override
    }

    for (prefix, method) in verbPatterns {
        if operationId.hasPrefix(prefix) {
            let remainder = String(operationId.dropFirst(prefix.count))
            if remainder.isEmpty { return method }
            let resource = lowercaseFirst(remainder)
            if simpleResources.contains(resource.lowercased()) { return method }
            return method == "get" ? lowercaseFirst(remainder) : method + remainder
        }
    }

    return lowercaseFirst(operationId)
}
