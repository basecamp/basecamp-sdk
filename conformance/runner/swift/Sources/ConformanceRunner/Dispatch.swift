import Basecamp
import Foundation

enum RunnerError: Error, CustomStringConvertible {
    case unknownOperation(String)
    /// A fixture parameter the dispatch table cannot use as written.
    case badParameter(String)
    /// A decoded response the dispatch table cannot reduce to a case result —
    /// an empty collection where the fixture's responseBody assertions read a
    /// specific element. Reporting nothing would make those assertions vacuous.
    case emptyResult(String)

    var description: String {
        switch self {
        case .unknownOperation(let op): "Unknown operation: \(op)"
        case .badParameter(let detail): "Fixture parameter: \(detail)"
        case .emptyResult(let detail): "Empty result: \(detail)"
        }
    }
}

// MARK: - Fixture parameter helpers

/// These throw rather than substituting a default. A missing or non-integral
/// `projectId` that quietly became `0` still produced a request the scripted
/// transport answered from the queue — a green test for a call to the wrong
/// resource, which is the exact false-green class this runner exists to catch.
/// A wrong-typed optional is the same fault one step quieter: it drops the
/// field and the requestBody assertion never sees what it was meant to pin.
extension Optional where Wrapped == [String: JSON] {
    func longParam(_ key: String) throws -> Int {
        guard let value = self?[key] else {
            throw RunnerError.badParameter("missing integer parameter \"\(key)\"")
        }
        guard let int = value.intValue, let narrowed = Int(exactly: int) else {
            throw RunnerError.badParameter(
                "parameter \"\(key)\" must be an integer, got \(value.display)")
        }
        return narrowed
    }

    /// Reads the first key that is present, for the operations whose fixtures
    /// spell one path parameter two ways. Replaces a `== 0` sentinel that could
    /// not tell an absent key from an id that legitimately read zero.
    func longParam(anyOf keys: [String]) throws -> Int {
        for key in keys where self?[key] != nil {
            return try longParam(key)
        }
        let tried = keys.map { "\"\($0)\"" }.joined(separator: " or ")
        throw RunnerError.badParameter("missing integer parameter \(tried)")
    }

    func stringParam(_ key: String) throws -> String {
        guard let value = self?[key] else {
            throw RunnerError.badParameter("missing string parameter \"\(key)\"")
        }
        guard let string = value.stringValue else {
            throw RunnerError.badParameter(
                "parameter \"\(key)\" must be a string, got \(value.display)")
        }
        return string
    }

    func optString(_ key: String) throws -> String? {
        guard let value = self?[key] else { return nil }
        guard let string = value.stringValue else {
            throw RunnerError.badParameter(
                "parameter \"\(key)\" must be a string, got \(value.display)")
        }
        return string
    }

    func optBool(_ key: String) throws -> Bool? {
        guard let value = self?[key] else { return nil }
        guard let bool = value.boolValue else {
            throw RunnerError.badParameter(
                "parameter \"\(key)\" must be a boolean, got \(value.display)")
        }
        return bool
    }

    func intArray(_ key: String) throws -> [Int]? {
        guard let value = self?[key] else { return nil }
        guard let array = value.arrayValue else {
            throw RunnerError.badParameter(
                "parameter \"\(key)\" must be an array, got \(value.display)")
        }
        return try array.map { element in
            guard let int = element.intValue, let narrowed = Int(exactly: int) else {
                throw RunnerError.badParameter(
                    "parameter \"\(key)\" must contain only integers, got \(element.display)")
            }
            return narrowed
        }
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
        let page = tc.configOverrides?.page
        let options = (maxItems ?? 0) > 0 || (page ?? 0) > 0
            ? ListProjectOptions(page: page, maxItems: maxItems)
            : nil
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

    // The raw read. BC3 renders a to-do list and a group through the same
    // `todolists/_todolist.json.jbuilder` partial, so both answer with the one
    // flat Todolist shape — nothing here sniffs which came back. The decoded
    // value is returned as the case result so the fixture's responseBody
    // assertions read it back through the SDK's own encoder (snake_case keys,
    // see `resultJSON`): a decoder that yields an empty value fails those
    // assertions instead of silently reporting success (#544).
    case "GetTodolistOrGroup":
        let todolist = try await account.todolists.get(id: pathParams.longParam("id"))
        return DispatchResult(resultJSON: try resultJSON(todolist))

    // The group list answers with an array of that same flat shape. Convention
    // documented in the fixture: the FIRST decoded element is the case result,
    // so the responseBody assertions read element 0. An empty list is a failure
    // rather than a silently absent result — that would make them vacuous.
    case "ListTodolistGroups":
        let groups = try await account.todolistGroups.list(
            todolistId: pathParams.longParam("todolistId"))
        guard let firstGroup = groups.items.first else {
            throw RunnerError.emptyResult(
                "ListTodolistGroups decoded 0 groups; the responseBody assertions read element 0")
        }
        return DispatchResult(
            totalCount: groups.meta.totalCount,
            truncated: groups.meta.truncated,
            resultJSON: try resultJSON(firstGroup))

    // Synthetic scenario key (not a wire operation): the merge-safe composite
    // over `PUT /todolists/{id}`, which is a full replace — BC3's
    // TodolistsController#update rebuilds the recordable from the permitted
    // params, so an omitted description is erased. GET then PUT, resending
    // whatever the caller did not mention. Deliberately variant-agnostic: a
    // group is rendered by the same partial as a list, so nothing here sniffs
    // which one came back.
    case "UpdateTodolist":
        _ = try await account.todolists.update(
            id: pathParams.longParam("id"),
            req: UpdateTodolistRequest(
                description: rb.optString("description"),
                name: rb.optString("name")))
        return DispatchResult()

    // Synthetic scenario key (not a wire operation): exercises the
    // read-modify-write edit closure by assigning each fixture key onto the
    // corresponding TodolistFields member.
    case "EditTodolist":
        // Read every fixture key before the call: the edit closure is
        // non-throwing, and validating up front means a malformed parameter
        // fails the test instead of reaching the wire half-applied.
        let editTodolistName = try rb.optString("name")
        let editTodolistDescription = try rb.optString("description")
        _ = try await account.todolists.edit(id: pathParams.longParam("id")) { fields in
            if let editTodolistName { fields.name = editTodolistName }
            if let editTodolistDescription { fields.description = editTodolistDescription }
        }
        return DispatchResult()

    // Raw single PUT, no read-before-write. `name` is required by the schema.
    case "ReplaceTodolist":
        _ = try await account.todolists.replace(
            id: pathParams.longParam("id"),
            req: UpdateTodolistOrGroupRequest(
                description: rb.optString("description"),
                name: rb.stringParam("name")))
        return DispatchResult()

    // Synthetic scenario key (not a wire operation): the merge-safe composite,
    // GET then a full PUT of {title, content}.
    case "UpdateDocument":
        _ = try await account.documents.update(
            documentId: pathParams.longParam("documentId"),
            req: UpdateDocumentRequest(
                content: rb.optString("content"),
                title: rb.optString("title")))
        return DispatchResult()

    // Synthetic scenario key (not a wire operation): exercises the
    // read-modify-write edit closure by assigning each fixture key onto the
    // corresponding DocumentFields member.
    case "EditDocument":
        // Read every fixture key before the call: the edit closure is
        // non-throwing, and validating up front means a malformed parameter
        // fails the test instead of reaching the wire half-applied.
        let editDocumentTitle = try rb.optString("title")
        let editDocumentContent = try rb.optString("content")
        _ = try await account.documents.edit(documentId: pathParams.longParam("documentId")) {
            fields in
            if let editDocumentTitle { fields.title = editDocumentTitle }
            if let editDocumentContent { fields.content = editDocumentContent }
        }
        return DispatchResult()

    // Raw single PUT, no read-before-write. Neither field is required by the
    // schema, so an omitted one stays omitted and the server clears it.
    case "ReplaceDocument":
        _ = try await account.documents.replace(
            documentId: pathParams.longParam("documentId"),
            req: ReplaceDocumentRequest(
                content: rb.optString("content"),
                title: rb.optString("title")))
        return DispatchResult()

    // Raw single PUT, no read-before-write. Presence-bearing throughout: only
    // the keys the fixture's requestBody carries may reach the wire, because
    // BC3 seeds participant_ids, url and highlighted from the existing
    // recordable whenever the request does not address them. An absent key that
    // became null, [] or false would clear what the server is holding.
    // `starts_at`/`ends_at` are required by the schema.
    case "ReplaceScheduleEntry":
        _ = try await account.schedules.replaceEntry(
            entryId: pathParams.longParam("entryId"),
            req: ReplaceScheduleEntryRequest(
                allDay: rb.optBool("all_day"),
                description: rb.optString("description"),
                endsAt: rb.stringParam("ends_at"),
                highlighted: rb.optBool("highlighted"),
                notify: rb.optBool("notify"),
                participantIds: rb.intArray("participant_ids"),
                startsAt: rb.stringParam("starts_at"),
                summary: rb.optString("summary"),
                url: rb.optString("url")))
        return DispatchResult()

    // Synthetic scenario key (not a wire operation): the merge-safe composite,
    // GET then a full PUT. The five full-state fields are always resent; the
    // four carve-outs reach the wire only when the fixture addressed them.
    case "UpdateScheduleEntry":
        _ = try await account.schedules.updateEntry(
            entryId: pathParams.longParam("entryId"),
            req: UpdateScheduleEntryRequest(
                allDay: rb.optBool("all_day"),
                description: rb.optString("description"),
                endsAt: rb.optString("ends_at"),
                highlighted: rb.optBool("highlighted"),
                notify: rb.optBool("notify"),
                participantIds: rb.intArray("participant_ids"),
                startsAt: rb.optString("starts_at"),
                summary: rb.optString("summary"),
                url: rb.optString("url")))
        return DispatchResult()

    // Synthetic scenario key (not a wire operation): exercises the
    // read-modify-write edit closure by assigning each fixture key onto the
    // corresponding ScheduleEntryFields member. The assignment is what puts a
    // carve-out on the wire — dirty tracking is by setter invocation, so a
    // fixture that assigns exactly the value the GET returned still sends it.
    case "EditScheduleEntry":
        // Read every fixture key before the call: the edit closure is
        // non-throwing, and validating up front means a malformed parameter
        // fails the test instead of reaching the wire half-applied.
        let editEntrySummary = try rb.optString("summary")
        let editEntryStartsAt = try rb.optString("starts_at")
        let editEntryEndsAt = try rb.optString("ends_at")
        let editEntryDescription = try rb.optString("description")
        let editEntryAllDay = try rb.optBool("all_day")
        let editEntryParticipantIds = try rb.intArray("participant_ids")
        let editEntryNotify = try rb.optBool("notify")
        let editEntryURL = try rb.optString("url")
        let editEntryHighlighted = try rb.optBool("highlighted")
        _ = try await account.schedules.editEntry(entryId: pathParams.longParam("entryId")) {
            fields in
            if let editEntrySummary { fields.summary = editEntrySummary }
            if let editEntryStartsAt { fields.startsAt = editEntryStartsAt }
            if let editEntryEndsAt { fields.endsAt = editEntryEndsAt }
            if let editEntryDescription { fields.description = editEntryDescription }
            if let editEntryAllDay { fields.allDay = editEntryAllDay }
            if let editEntryParticipantIds { fields.participantIds = editEntryParticipantIds }
            if let editEntryNotify { fields.notify = editEntryNotify }
            if let editEntryURL { fields.url = editEntryURL }
            if let editEntryHighlighted { fields.highlighted = editEntryHighlighted }
        }
        return DispatchResult()

    // Merge-safe composite: GET then PUT, resending the fetched due_on.
    // An explicit empty due_on means clear (single PUT, no GET); an absent
    // key means preserve (GET first).
    case "UpdateCard":
        let dueOn: CardsService.DueDate = if let raw = try rb.optString("due_on") {
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
        // Read every fixture key before the call: the edit closure is
        // non-throwing, and validating up front means a malformed parameter
        // fails the test instead of reaching the wire half-applied.
        let editContent = try rb.optString("content")
        let editDescription = try rb.optString("description")
        let editAssigneeIds = try rb.intArray("assignee_ids")
        let editSubscriberIds = try rb.intArray("completion_subscriber_ids")
        let editDueOn = try rb.optString("due_on")
        let editStartsOn = try rb.optString("starts_on")
        let editNotify = try rb.optBool("notify")
        _ = try await account.todos.edit(todoId: pathParams.longParam("todoId")) { fields in
            if let editContent { fields.content = editContent }
            if let editDescription { fields.description = editDescription }
            if let editAssigneeIds { fields.assigneeIds = editAssigneeIds }
            if let editSubscriberIds { fields.completionSubscriberIds = editSubscriberIds }
            if let editDueOn { fields.dueOn = editDueOn }
            if let editStartsOn { fields.startsOn = editStartsOn }
            if let editNotify { fields.notify = editNotify }
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

    case "ListFolders":
        _ = try await account.folders.listFolders()
        return DispatchResult()

    case "GetFolder":
        _ = try await account.folders.getFolder(folderId: pathParams.longParam("folderId"))
        return DispatchResult()

    case "CreateFolder":
        _ = try await account.folders.createFolder(
            req: CreateFolderRequest(
                name: try rb.optString("name"),
                projectIds: try rb.intArray("project_ids")))
        return DispatchResult()

    case "UpdateFolder":
        _ = try await account.folders.updateFolder(
            folderId: pathParams.longParam("folderId"),
            req: UpdateFolderRequest(name: try rb.stringParam("name")))
        return DispatchResult()

    case "DeleteFolder":
        try await account.folders.deleteFolder(folderId: pathParams.longParam("folderId"))
        return DispatchResult()

    case "GetTimesheetEntry":
        _ = try await account.timesheets.get(
            entryId: pathParams.longParam(anyOf: ["timesheetEntryId", "entryId"]))
        return DispatchResult()

    case "DestroyTimesheetEntry":
        try await account.timesheets.destroy(
            entryId: pathParams.longParam(anyOf: ["timesheetEntryId", "entryId"]))
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
        _ = try await account.timesheets.update(
            entryId: pathParams.longParam(anyOf: ["entryId", "timesheetEntryId"]),
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

    // Pins the `inbox_forwards` collection segment. The shipped path said
    // `forwards`, which bc3 does not route, so the fixture is a wire assertion
    // on the segment rather than on any response shape.
    case "ListForwards":
        let forwards = try await account.forwards.list(inboxId: pathParams.longParam("inboxId"))
        return DispatchResult(totalCount: forwards.meta.totalCount, truncated: forwards.meta.truncated)

    // #588: nine flat spellings bc3 only draws bucket-scoped. Each of the
    // following pins the bucketId segment on the wire — the segment whose
    // absence made every one of them a live 404.
    case "ListChatbots":
        let chatbots = try await account.campfires.listChatbots(
            bucketId: pathParams.longParam("bucketId"),
            campfireId: pathParams.longParam("campfireId"))
        return DispatchResult(totalCount: chatbots.meta.totalCount, truncated: chatbots.meta.truncated)

    case "GetChatbot":
        _ = try await account.campfires.getChatbot(
            bucketId: pathParams.longParam("bucketId"),
            campfireId: pathParams.longParam("campfireId"),
            chatbotId: pathParams.longParam("chatbotId"))
        return DispatchResult()

    case "CreateChatbot":
        _ = try await account.campfires.createChatbot(
            bucketId: pathParams.longParam("bucketId"),
            campfireId: pathParams.longParam("campfireId"),
            req: CreateChatbotRequest(
                commandUrl: try rb.stringParam("command_url"),
                serviceName: try rb.stringParam("service_name")))
        return DispatchResult()

    case "UpdateChatbot":
        _ = try await account.campfires.updateChatbot(
            bucketId: pathParams.longParam("bucketId"),
            campfireId: pathParams.longParam("campfireId"),
            chatbotId: pathParams.longParam("chatbotId"),
            req: UpdateChatbotRequest(
                commandUrl: try rb.stringParam("command_url"),
                serviceName: try rb.stringParam("service_name")))
        return DispatchResult()

    case "DeleteChatbot":
        try await account.campfires.deleteChatbot(
            bucketId: pathParams.longParam("bucketId"),
            campfireId: pathParams.longParam("campfireId"),
            chatbotId: pathParams.longParam("chatbotId"))
        return DispatchResult()

    case "ListClientApprovals":
        let approvals = try await account.clientApprovals.list(
            bucketId: pathParams.longParam("bucketId"))
        return DispatchResult(totalCount: approvals.meta.totalCount, truncated: approvals.meta.truncated)

    case "ListClientCorrespondences":
        let correspondences = try await account.clientCorrespondences.list(
            bucketId: pathParams.longParam("bucketId"))
        return DispatchResult(totalCount: correspondences.meta.totalCount, truncated: correspondences.meta.truncated)

    case "ListClientReplies":
        let replies = try await account.clientReplies.list(
            bucketId: pathParams.longParam("bucketId"),
            recordingId: pathParams.longParam("recordingId"))
        return DispatchResult(totalCount: replies.meta.totalCount, truncated: replies.meta.truncated)

    case "GetClientReply":
        _ = try await account.clientReplies.get(
            bucketId: pathParams.longParam("bucketId"),
            recordingId: pathParams.longParam("recordingId"),
            replyId: pathParams.longParam("replyId"))
        return DispatchResult()

    // Pins the `todolists/groups` segment: a group repositions through its own
    // collection, not through `/todolists/{id}`. 204-shaped (requestVoid), so
    // there is no result to report.
    case "RepositionTodolistGroup":
        try await account.todolistGroups.reposition(
            groupId: pathParams.longParam("groupId"),
            req: RepositionTodolistGroupRequest(position: Int32(rb.longParam("position"))))
        return DispatchResult()

    default:
        throw RunnerError.unknownOperation(tc.operation)
    }
}
