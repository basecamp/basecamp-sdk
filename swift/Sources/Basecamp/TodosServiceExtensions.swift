// Hand-written merge-safe update / edit surface for TodosService.
import Foundation

/// Request parameters for the merge-safe ``TodosService/update(todoId:req:)``.
/// Every field is optional: a `nil` field is left untouched on the todo,
/// guaranteed. An explicitly-passed empty array is a set (clears the list).
public struct UpdateTodoRequest: Codable, Sendable {
    public var assigneeIds: [Int]?
    public var completionSubscriberIds: [Int]?
    public var content: String?
    public var description: String?
    public var dueOn: String?
    public var notify: Bool?
    public var startsOn: String?

    public init(
        assigneeIds: [Int]? = nil,
        completionSubscriberIds: [Int]? = nil,
        content: String? = nil,
        description: String? = nil,
        dueOn: String? = nil,
        notify: Bool? = nil,
        startsOn: String? = nil
    ) {
        self.assigneeIds = assigneeIds
        self.completionSubscriberIds = completionSubscriberIds
        self.content = content
        self.description = description
        self.dueOn = dueOn
        self.notify = notify
        self.startsOn = startsOn
    }
}

/// A todo's full writable state, handed to the ``TodosService/edit(todoId:_:)``
/// closure. The whole value is PUT back to the server, so clearing a field
/// means setting it empty (`""` for strings and dates, `[]` for ID lists) —
/// there is no third state.
public struct TodoFields: Sendable {
    /// Text content (required; the server rejects an empty one).
    public var content: String
    /// Rich text description (HTML). Set `""` to clear.
    public var description: String
    /// Complete list of assigned person IDs. Set `[]` to clear.
    public var assigneeIds: [Int]
    /// Complete list of person IDs notified on completion. Set `[]` to clear.
    public var completionSubscriberIds: [Int]
    /// Due date (YYYY-MM-DD). Set `""` to clear.
    public var dueOn: String
    /// Start date (YYYY-MM-DD). Set `""` to clear.
    public var startsOn: String
    /// Send directive, not todo state: never populated from the current todo
    /// and sent only when true, asking the server to notify assignees about
    /// this write.
    public var notify: Bool

    init(from todo: Todo) {
        content = todo.content
        description = todo.description ?? ""
        assigneeIds = (todo.assignees ?? []).map { $0.id.value }
        completionSubscriberIds = (todo.completionSubscribers ?? []).map { $0.id.value }
        dueOn = todo.dueOn ?? ""
        startsOn = todo.startsOn ?? ""
        notify = false
    }
}

// Merge-safe `update` and read-modify-write `edit`, composed from the public
// `get` and `replace` methods — hooks observe the two wire operations
// (`GetTodo` then `ReplaceTodo`), not a synthetic composite.
//
// Neither is atomic: there is no conditional-update signal on this endpoint,
// so a concurrent write between the GET and PUT is overwritten — last write
// wins for the whole representation. The window is one round-trip. Use
// `replace` to overwrite deliberately.
extension TodosService {
    /// Sets the given fields on a todo and preserves everything else: GETs
    /// the current todo, overlays the explicitly-set (non-`nil`) request
    /// fields, and PUTs the full representation back. A `nil` field is
    /// untouched, guaranteed; an explicitly-passed empty array clears.
    ///
    /// Not atomic — see the extension docs for the GET→PUT race.
    public func update(todoId: Int, req: UpdateTodoRequest) async throws -> Todo {
        var fields = TodoFields(from: try await get(todoId: todoId))
        if let content = req.content { fields.content = content }
        if let description = req.description { fields.description = description }
        if let assigneeIds = req.assigneeIds { fields.assigneeIds = assigneeIds }
        if let subscriberIds = req.completionSubscriberIds { fields.completionSubscriberIds = subscriberIds }
        if let dueOn = req.dueOn { fields.dueOn = dueOn }
        if let startsOn = req.startsOn { fields.startsOn = startsOn }
        if let notify = req.notify { fields.notify = notify }
        return try await putFields(todoId: todoId, fields: fields)
    }

    /// Applies a read-modify-write closure to a todo: GETs the current todo,
    /// hands the closure the full writable representation (``TodoFields``),
    /// and PUTs the whole thing back. Clearing a field means setting it empty
    /// (`""` / `[]`) — an untouched field keeps its current value. If the
    /// closure throws, the edit aborts and nothing is written.
    ///
    /// ```swift
    /// try await account.todos.edit(todoId: 123) {
    ///     $0.content = "🚨 " + $0.content
    ///     $0.dueOn = "" // clearing = setting empty on a full object
    /// }
    /// ```
    ///
    /// Not atomic — see the extension docs for the GET→PUT race.
    public func edit(todoId: Int, _ mutate: (inout TodoFields) throws -> Void) async throws -> Todo {
        var fields = TodoFields(from: try await get(todoId: todoId))
        try mutate(&fields)
        return try await putFields(todoId: todoId, fields: fields)
    }

    /// PUTs the full writable state via `replace`: content, description, and
    /// both ID lists are always sent (empties included, so clears survive);
    /// dates only when non-empty (the server clears an omitted date, and `""`
    /// is a format error); notify only when true.
    private func putFields(todoId: Int, fields: TodoFields) async throws -> Todo {
        try await replace(todoId: todoId, req: ReplaceTodoRequest(
            assigneeIds: fields.assigneeIds,
            completionSubscriberIds: fields.completionSubscriberIds,
            content: fields.content,
            description: fields.description,
            dueOn: fields.dueOn.isEmpty ? nil : fields.dueOn,
            notify: fields.notify ? true : nil,
            startsOn: fields.startsOn.isEmpty ? nil : fields.startsOn
        ))
    }
}
