import Foundation

// MARK: - Tag to Service Mapping

/// Maps OpenAPI tags to service class names.
let tagToService: [String: String] = [
    "Card Tables": "CardTables",
    "Campfire": "Campfires",
    "Todos": "Todos",
    "Messages": "Messages",
    "Files": "Files",
    "Forwards": "Forwards",
    "Schedule": "Schedules",
    "People": "People",
    "Projects": "Projects",
    "Automation": "Automation",
    "ClientFeatures": "ClientFeatures",
    "Boosts": "Boosts",
    "Untagged": "Miscellaneous",
]

// MARK: - Service Splits

/// Routes operations within a tag to sub-services.
let serviceSplits: [String: [String: [String]]] = [
    "Campfire": [
        "Campfires": [
            "GetCampfire", "ListCampfires",
            "ListChatbots", "CreateChatbot", "GetChatbot", "UpdateChatbot", "DeleteChatbot",
            "ListCampfireLines", "CreateCampfireLine", "GetCampfireLine", "DeleteCampfireLine",
        ],
    ],
    "Card Tables": [
        "CardTables": ["GetCardTable"],
        "Cards": ["GetCard", "UpdateCard", "MoveCard", "CreateCard", "ListCards"],
        "CardColumns": [
            "GetCardColumn", "UpdateCardColumn", "SetCardColumnColor",
            "EnableCardColumnOnHold", "DisableCardColumnOnHold",
            "CreateCardColumn", "MoveCardColumn",
            "SubscribeToCardColumn", "UnsubscribeFromCardColumn",
        ],
        "CardSteps": [
            "CreateCardStep", "UpdateCardStep", "SetCardStepCompletion",
            "RepositionCardStep",
        ],
    ],
    "Files": [
        "Attachments": ["CreateAttachment"],
        "Uploads": ["GetUpload", "UpdateUpload", "ListUploads", "CreateUpload", "ListUploadVersions"],
        "Vaults": ["GetVault", "UpdateVault", "ListVaults", "CreateVault"],
        "Documents": ["GetDocument", "UpdateDocument", "ListDocuments", "CreateDocument"],
    ],
    "Automation": [
        "Tools": ["GetTool", "UpdateTool", "DeleteTool", "CloneTool", "EnableTool", "DisableTool", "RepositionTool"],
        "Recordings": ["GetRecording", "ArchiveRecording", "UnarchiveRecording", "TrashRecording", "ListRecordings"],
        "Webhooks": ["ListWebhooks", "CreateWebhook", "GetWebhook", "UpdateWebhook", "DeleteWebhook"],
        "Events": ["ListEvents"],
        "Lineup": ["CreateLineupMarker", "UpdateLineupMarker", "DeleteLineupMarker"],
        "Search": ["Search", "GetSearchMetadata"],
        "Templates": [
            "ListTemplates", "CreateTemplate", "GetTemplate", "UpdateTemplate",
            "DeleteTemplate", "CreateProjectFromTemplate", "GetProjectConstruction",
        ],
        "Checkins": [
            "GetQuestionnaire", "ListQuestions", "CreateQuestion", "GetQuestion",
            "UpdateQuestion", "ListAnswers", "CreateAnswer", "GetAnswer", "UpdateAnswer",
        ],
    ],
    "Messages": [
        "Messages": ["GetMessage", "UpdateMessage", "CreateMessage", "ListMessages", "PinMessage", "UnpinMessage"],
        "MessageBoards": ["GetMessageBoard"],
        "MessageTypes": ["ListMessageTypes", "CreateMessageType", "GetMessageType", "UpdateMessageType", "DeleteMessageType"],
        "Comments": ["GetComment", "UpdateComment", "ListComments", "CreateComment"],
    ],
    "People": [
        "People": ["GetMyProfile", "ListPeople", "GetPerson", "ListProjectPeople", "UpdateProjectAccess", "ListPingablePeople", "ListAssignablePeople"],
        "Subscriptions": ["GetSubscription", "Subscribe", "Unsubscribe", "UpdateSubscription"],
    ],
    "Schedule": [
        "Schedules": [
            "GetSchedule", "UpdateScheduleSettings", "ListScheduleEntries",
            "CreateScheduleEntry", "GetScheduleEntry", "UpdateScheduleEntry", "GetScheduleEntryOccurrence",
        ],
        "Timesheets": ["GetRecordingTimesheet", "GetProjectTimesheet", "GetTimesheetReport", "GetTimesheetEntry", "CreateTimesheetEntry", "UpdateTimesheetEntry"],
    ],
    "ClientFeatures": [
        "ClientApprovals": ["ListClientApprovals", "GetClientApproval"],
        "ClientCorrespondences": ["ListClientCorrespondences", "GetClientCorrespondence"],
        "ClientReplies": ["ListClientReplies", "GetClientReply"],
        "ClientVisibility": ["SetClientVisibility"],
    ],
    "Todos": [
        "Todos": ["ListTodos", "CreateTodo", "GetTodo", "UpdateTodo", "CompleteTodo", "UncompleteTodo", "TrashTodo"],
        "Todolists": ["GetTodolistOrGroup", "UpdateTodolistOrGroup", "ListTodolists", "CreateTodolist"],
        "Todosets": ["GetTodoset"],
        "TodolistGroups": ["ListTodolistGroups", "CreateTodolistGroup", "RepositionTodolistGroup"],
    ],
    "Untagged": [
        "Timeline": ["GetProjectTimeline"],
        "Reports": ["GetProgressReport", "GetUpcomingSchedule", "GetAssignedTodos", "GetOverdueTodos", "GetPersonProgress"],
        "Checkins": [
            "GetQuestionReminders", "ListQuestionAnswerers", "GetAnswersByPerson",
            "UpdateQuestionNotificationSettings", "PauseQuestion", "ResumeQuestion",
        ],
        "Todos": ["RepositionTodo"],
        "People": ["ListAssignablePeople"],
        "CardColumns": ["SubscribeToCardColumn", "UnsubscribeFromCardColumn"],
    ],
]

// MARK: - Service Definition

struct ServiceDefinition {
    let name: String
    var operations: [ParsedOperation] = []
    var entityTypes: Set<String> = []

    var className: String { "\(name)Service" }
}

// MARK: - Grouping

/// Groups parsed operations into services based on tags, splits, and overrides.
func groupOperations(_ operations: [ParsedOperation], schemas: [String: Any]) -> [String: ServiceDefinition] {
    var services: [String: ServiceDefinition] = [:]

    for op in operations {
        let tag = op.tag

        // Determine service name
        let serviceName: String
        if let splits = serviceSplits[tag] {
            var matched: String?
            for svc in splits.keys.sorted() {
                if splits[svc]!.contains(op.operationId) {
                    matched = svc
                    break
                }
            }
            serviceName = matched ?? tagToService[tag] ?? tag.replacingOccurrences(of: " ", with: "")
        } else {
            serviceName = tagToService[tag] ?? tag.replacingOccurrences(of: " ", with: "")
        }

        if services[serviceName] == nil {
            services[serviceName] = ServiceDefinition(name: serviceName)
        }

        services[serviceName]!.operations.append(op)

        // Collect entity types
        if let responseRef = op.responseSchemaRef {
            if let entityName = getEntityTypeName(responseRef, schemas: schemas) {
                services[serviceName]!.entityTypes.insert(entityName)
            }
        }
    }

    return services
}
