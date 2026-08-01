import Basecamp
import Foundation

enum RunnerError: Error, CustomStringConvertible {
    case unknownOperation(String)

    var description: String {
        switch self {
        case .unknownOperation(let op): "Unknown operation: \(op)"
        }
    }
}

// MARK: - Fixture parameter helpers

extension Optional where Wrapped == [String: JSON] {
    func longParam(_ key: String) -> Int {
        (self?[key]?.intValue).map(Int.init) ?? 0
    }

    func stringParam(_ key: String) -> String {
        self?[key]?.stringValue ?? ""
    }

    func optString(_ key: String) -> String? {
        self?[key]?.stringValue
    }

    func optBool(_ key: String) -> Bool? {
        self?[key]?.boolValue
    }

    func intArray(_ key: String) -> [Int]? {
        self?[key]?.arrayValue?.compactMap { $0.intValue.map(Int.init) }
    }
}

/// Re-serializes a decoded SDK model through the SDK's own encoder (snake_case
/// keys) so responseBody assertions see wire-shaped field names.
private func resultJSON<T: Encodable>(_ value: T) throws -> JSON? {
    JSON.parse(try BaseService.encoder.encode(value))
}

/// Dispatches the test operation against the SDK and returns observed metadata.
/// Direct port of the Kotlin dispatch table.
func dispatchOperation(_ tc: TestCase, _ account: AccountClient) async throws -> DispatchResult {
    let pathParams = tc.pathParams
    let rb = tc.requestBody

    switch tc.operation {
    case "ListProjects":
        let maxItems = tc.configOverrides?.maxItems
        let options = (maxItems ?? 0) > 0 ? ListProjectOptions(maxItems: maxItems) : nil
        let result = try await account.projects.list(options: options)
        return DispatchResult(totalCount: result.meta.totalCount, truncated: result.meta.truncated)

    case "GetProject":
        let project = try await account.projects.get(projectId: pathParams.longParam("projectId"))
        return DispatchResult(resultJSON: try resultJSON(project))

    case "CreateProject":
        _ = try await account.projects.create(req: CreateProjectRequest(name: rb.stringParam("name")))
        return DispatchResult()

    case "UpdateProject":
        _ = try await account.projects.update(
            projectId: pathParams.longParam("projectId"),
            req: UpdateProjectRequest(name: rb.stringParam("name")))
        return DispatchResult()

    case "TrashProject":
        try await account.projects.trash(projectId: pathParams.longParam("projectId"))
        return DispatchResult()

    case "ListTodos":
        let result = try await account.todos.list(todolistId: pathParams.longParam("todolistId"))
        return DispatchResult(totalCount: result.meta.totalCount, truncated: result.meta.truncated)

    case "UpdateTodo":
        _ = try await account.todos.update(
            todoId: pathParams.longParam("todoId"),
            req: UpdateTodoRequest(
                assigneeIds: rb.intArray("assignee_ids"),
                completionSubscriberIds: rb.intArray("completion_subscriber_ids"),
                content: rb.optString("content"),
                description: rb.optString("description"),
                dueOn: rb.optString("due_on"),
                notify: rb.optBool("notify"),
                startsOn: rb.optString("starts_on")))
        return DispatchResult()

    // Participants are presence-bearing: an absent key must not become an
    // empty list on the wire, or BC3 clears the participants.
    case "UpdateScheduleEntry":
        _ = try await account.schedules.updateEntry(
            entryId: pathParams.longParam("entryId"),
            req: UpdateScheduleEntryRequest(
                endsAt: rb.optString("ends_at"),
                participantIds: rb.intArray("participant_ids"),
                startsAt: rb.optString("starts_at"),
                summary: rb.optString("summary")))
        return DispatchResult()

    // Merge-safe composite: GET then PUT, resending the fetched due_on.
    // An explicit empty due_on means clear (single PUT, no GET); an absent
    // key means preserve (GET first).
    case "UpdateCard":
        let dueOn: CardsService.DueDate = if let raw = rb.optString("due_on") {
            raw.isEmpty ? .clear : .on(raw)
        } else {
            .preserve
        }
        _ = try await account.cards.update(
            cardId: pathParams.longParam("cardId"),
            title: rb.optString("title"),
            content: rb.optString("content"),
            dueOn: dueOn,
            assigneeIds: rb.intArray("assignee_ids"))
        return DispatchResult()

    // Raw single PUT, no read-before-write.
    case "UpdateCardVerbatim":
        _ = try await account.cards.updateVerbatim(
            cardId: pathParams.longParam("cardId"),
            req: UpdateCardRequest(
                assigneeIds: rb.intArray("assignee_ids"),
                content: rb.optString("content"),
                dueOn: rb.optString("due_on"),
                title: rb.optString("title")))
        return DispatchResult()

    // Synthetic scenario key (not a wire operation): exercises the
    // read-modify-write edit closure by assigning each fixture key
    // onto the corresponding TodoFields member.
    case "EditTodo":
        _ = try await account.todos.edit(todoId: pathParams.longParam("todoId")) { fields in
            if let content = rb.optString("content") { fields.content = content }
            if let description = rb.optString("description") { fields.description = description }
            if let assigneeIds = rb.intArray("assignee_ids") { fields.assigneeIds = assigneeIds }
            if let subscriberIds = rb.intArray("completion_subscriber_ids") { fields.completionSubscriberIds = subscriberIds }
            if let dueOn = rb.optString("due_on") { fields.dueOn = dueOn }
            if let startsOn = rb.optString("starts_on") { fields.startsOn = startsOn }
            if let notify = rb.optBool("notify") { fields.notify = notify }
        }
        return DispatchResult()

    case "ReplaceTodo":
        _ = try await account.todos.replace(
            todoId: pathParams.longParam("todoId"),
            req: ReplaceTodoRequest(
                assigneeIds: rb.intArray("assignee_ids"),
                completionSubscriberIds: rb.intArray("completion_subscriber_ids"),
                content: rb.stringParam("content"),
                description: rb.optString("description"),
                dueOn: rb.optString("due_on"),
                notify: rb.optBool("notify"),
                startsOn: rb.optString("starts_on")))
        return DispatchResult()

    case "CreateTodo":
        _ = try await account.todos.create(
            todolistId: pathParams.longParam("todolistId"),
            req: CreateTodoRequest(content: rb.stringParam("content")))
        return DispatchResult()

    case "CreateTodosetTodo":
        _ = try await account.todos.createTodosetTodo(
            bucketId: pathParams.longParam("bucketId"),
            todosetId: pathParams.longParam("todosetId"),
            req: CreateTodosetTodoRequest(content: rb.stringParam("content")))
        return DispatchResult()

    case "CompleteTodo":
        try await account.todos.complete(todoId: pathParams.longParam("todoId"))
        return DispatchResult()

    case "Subscribe":
        _ = try await account.subscriptions.subscribe(recordingId: pathParams.longParam("recordingId"))
        return DispatchResult()

    case "ListMyBookmarks":
        _ = try await account.bookmarks.listMyBookmarks()
        return DispatchResult()

    case "ListMyDrafts":
        _ = try await account.drafts.listMyDrafts()
        return DispatchResult()

    case "GetMyNote":
        _ = try await account.myNotes.getMyNote()
        return DispatchResult()

    case "PrioritizeAssignment":
        try await account.myAssignments.prioritizeAssignment(
            req: PrioritizeAssignmentRequest(id: rb.longParam("id")))
        return DispatchResult()

    case "DeprioritizeAssignment":
        try await account.myAssignments.deprioritizeAssignment(recordingId: pathParams.longParam("recordingId"))
        return DispatchResult()

    case "ReorderUpNext":
        try await account.myAssignments.reorderUpNext(
            req: ReorderUpNextRequest(
                position: Int32(rb.longParam("position")),
                sourceId: rb.longParam("source_id")))
        return DispatchResult()

    case "GetCalendar":
        _ = try await account.calendars.getCalendar(calendarId: pathParams.longParam("calendarId"))
        return DispatchResult()

    case "UpdateCalendar":
        let calendar = rb?["calendar"]?.objectValue
        _ = try await account.calendars.updateCalendar(
            calendarId: pathParams.longParam("calendarId"),
            req: UpdateCalendarRequest(calendar: CalendarAttributes(color: calendar.stringParam("color"))))
        return DispatchResult()

    case "UpdateMyNote":
        let note = rb?["note"]?.objectValue
        _ = try await account.myNotes.updateMyNote(
            req: UpdateMyNoteRequest(note: MyNoteAttributes(content: note.stringParam("content"))))
        return DispatchResult()

    case "GetBookmark":
        _ = try await account.bookmarks.getBookmark(recordingId: pathParams.longParam("recordingId"))
        return DispatchResult()

    case "CreateBookmark":
        _ = try await account.bookmarks.createBookmark(recordingId: pathParams.longParam("recordingId"))
        return DispatchResult()

    case "DeleteBookmark":
        try await account.bookmarks.deleteBookmark(recordingId: pathParams.longParam("recordingId"))
        return DispatchResult()

    case "GetTimesheetEntry":
        var entryId = pathParams.longParam("timesheetEntryId")
        if entryId == 0 { entryId = pathParams.longParam("entryId") }
        _ = try await account.timesheets.get(entryId: entryId)
        return DispatchResult()

    case "CreateTimesheetEntry":
        _ = try await account.timesheets.create(
            recordingId: pathParams.longParam("recordingId"),
            req: CreateTimesheetEntryRequest(
                date: rb.stringParam("date"),
                description: rb.optString("description"),
                hours: rb.stringParam("hours")))
        return DispatchResult()

    case "UpdateTimesheetEntry":
        var entryId = pathParams.longParam("entryId")
        if entryId == 0 { entryId = pathParams.longParam("timesheetEntryId") }
        _ = try await account.timesheets.update(
            entryId: entryId,
            req: UpdateTimesheetEntryRequest(
                date: rb.optString("date"),
                description: rb.optString("description"),
                hours: rb.optString("hours")))
        return DispatchResult()

    case "GetProjectTimeline":
        _ = try await account.timeline.projectTimeline(projectId: pathParams.longParam("projectId"))
        return DispatchResult()

    case "GetProgressReport":
        _ = try await account.reports.progress()
        return DispatchResult()

    case "GetPersonProgress":
        _ = try await account.reports.personProgress(personId: pathParams.longParam("personId"))
        return DispatchResult()

    case "GetProjectTimesheet":
        _ = try await account.timesheets.forProject(projectId: pathParams.longParam("projectId"))
        return DispatchResult()

    case "ListWebhooks":
        _ = try await account.webhooks.list(bucketId: pathParams.longParam("bucketId"))
        return DispatchResult()

    case "CreateWebhook":
        let types = rb?["types"]?.arrayValue?.compactMap(\.stringValue) ?? []
        _ = try await account.webhooks.create(
            bucketId: pathParams.longParam("bucketId"),
            req: CreateWebhookRequest(payloadUrl: rb.stringParam("payload_url"), types: types))
        return DispatchResult()

    case "GetTool":
        _ = try await account.tools.get(toolId: pathParams.longParam("toolId"))
        return DispatchResult()

    case "CreateTool":
        _ = try await account.tools.create(
            bucketId: pathParams.longParam("bucketId"),
            req: CreateToolRequest(title: rb.optString("title"), toolType: rb.stringParam("tool_type")))
        return DispatchResult()

    case "EnableTool":
        try await account.tools.enable(toolId: pathParams.longParam("toolId"))
        return DispatchResult()

    case "GetEverythingMessages":
        _ = try await account.everything.everythingMessages()
        return DispatchResult()

    case "GetEverythingComments":
        _ = try await account.everything.everythingComments()
        return DispatchResult()

    case "GetEverythingCheckins":
        _ = try await account.everything.everythingCheckins()
        return DispatchResult()

    case "GetEverythingForwards":
        _ = try await account.everything.everythingForwards()
        return DispatchResult()

    case "GetEverythingFiles":
        _ = try await account.everything.everythingFiles()
        return DispatchResult()

    case "GetEverythingOverdueTodos":
        _ = try await account.everything.everythingOverdueTodos()
        return DispatchResult()

    case "GetEverythingOverdueCards":
        _ = try await account.everything.everythingOverdueCards()
        return DispatchResult()

    case "GetEverythingOpenTodos":
        _ = try await account.everything.everythingOpenTodos()
        return DispatchResult()

    case "GetEverythingCompletedTodos":
        _ = try await account.everything.everythingCompletedTodos()
        return DispatchResult()

    case "GetEverythingUnassignedTodos":
        _ = try await account.everything.everythingUnassignedTodos()
        return DispatchResult()

    case "GetEverythingNoDueDateTodos":
        _ = try await account.everything.everythingNoDueDateTodos()
        return DispatchResult()

    case "GetEverythingOpenCards":
        _ = try await account.everything.everythingOpenCards()
        return DispatchResult()

    case "GetEverythingCompletedCards":
        _ = try await account.everything.everythingCompletedCards()
        return DispatchResult()

    case "GetEverythingUnassignedCards":
        _ = try await account.everything.everythingUnassignedCards()
        return DispatchResult()

    case "GetEverythingNoDueDateCards":
        _ = try await account.everything.everythingNoDueDateCards()
        return DispatchResult()

    case "GetEverythingNotNowCards":
        _ = try await account.everything.everythingNotNowCards()
        return DispatchResult()

    case "DownloadURL":
        // Construct an absolute URL the SDK will accept. downloadURL rewrites
        // the scheme+host to the configured baseURL, so the synthetic host here
        // is never actually hit — only tc.path matters for mock routing. Same
        // shape as the Go and Kotlin runners.
        _ = try await account.downloadURL("https://storage.3.basecamp.com" + tc.fixturePath)
        return DispatchResult()

    case "UploadsDownload":
        _ = try await account.uploads.download(uploadId: pathParams.longParam("uploadId"))
        return DispatchResult()

    default:
        throw RunnerError.unknownOperation(tc.operation)
    }
}
