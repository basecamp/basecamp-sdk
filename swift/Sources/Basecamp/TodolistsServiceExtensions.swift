// Hand-written merge-safe update / edit surface for TodolistsService.
import Foundation

/// Request parameters for the merge-safe ``TodolistsService/update(id:req:)``.
/// Every field is optional: a `nil` field is left untouched on the todolist,
/// guaranteed. An explicitly-passed empty string is a set (clears the field).
public struct UpdateTodolistRequest: Codable, Sendable {
    public var description: String?
    public var name: String?

    public init(
        description: String? = nil,
        name: String? = nil
    ) {
        self.description = description
        self.name = name
    }
}

/// A todolist's full writable state, handed to the
/// ``TodolistsService/edit(id:_:)`` closure. The whole value is PUT back to the
/// server, so clearing a field means setting it empty (`""`) — there is no
/// third state. BC3's writable set on this endpoint is exactly
/// `{name, description}`; everything else on a todolist is server-owned or has
/// its own endpoint (position, status, subscriptions).
public struct TodolistFields: Sendable {
    /// The list's name (required; the server presence-validates it, so an
    /// empty one is a 422 rather than a clear).
    public var name: String
    /// Rich text description (HTML). Set `""` to clear.
    public var description: String

    init(from todolist: Todolist) {
        name = todolist.name
        description = todolist.description
    }
}

// Merge-safe `update` and read-modify-write `edit`, composed from the public
// `get` and `replace` methods — hooks observe the two wire operations
// (`GetTodolistOrGroup` then `UpdateTodolistOrGroup`), not a synthetic
// composite.
//
// Both wire operations carry a plain ``Todolist``. BC3 has no separate group
// model: `todolists/groups/{index,show}.json.jbuilder` render
// `todolists/_todolist.json.jbuilder`, so a group IS a todolist — it reports
// `"type": "Todolist"` and carries `description`/`description_attachments` like
// any list. Only the structural pair `groups_url` (parent is a Todoset) versus
// `group_position_url` (parent is a Todolist) tells them apart, and nothing
// here branches on either: the composites are variant-agnostic by
// construction.
//
// `PUT /{accountId}/todolists/{id}` is a full replace: BC3's
// `TodolistsController#update` rebuilds the recordable from only the permitted
// params, so a PUT that omits `description` ERASES it. That makes the natural
// sparse write destructive on the raw endpoint, which stays available as
// `replace`.
//
// Neither composite is atomic: there is no conditional-update signal on this
// endpoint, so a concurrent write between the GET and PUT is overwritten — last
// write wins for the whole representation. The window is one round-trip. Use
// `replace` to overwrite deliberately.
extension TodolistsService {
    /// Sets the given fields on a todolist and preserves everything else: GETs
    /// the current todolist, overlays the explicitly-set (non-`nil`) request
    /// fields, and PUTs the full representation back. A `nil` field is
    /// untouched, guaranteed; an explicitly-passed `""` clears.
    ///
    /// Not atomic — see the extension docs for the GET→PUT race.
    public func update(id: Int, req: UpdateTodolistRequest) async throws -> Todolist {
        var fields = TodolistFields(from: try await fetchTodolist(id: id))
        if let name = req.name { fields.name = name }
        if let description = req.description { fields.description = description }
        return try await putFields(id: id, fields: fields)
    }

    /// Applies a read-modify-write closure to a todolist: GETs the current
    /// todolist, hands the closure the full writable representation
    /// (``TodolistFields``), and PUTs the whole thing back. Clearing a field
    /// means setting it empty (`""`) — an untouched field keeps its current
    /// value. If the closure throws, the edit aborts and nothing is written.
    ///
    /// ```swift
    /// try await account.todolists.edit(id: 123) {
    ///     $0.name = "🚨 " + $0.name
    ///     $0.description = "" // clearing = setting empty on a full object
    /// }
    /// ```
    ///
    /// Not atomic — see the extension docs for the GET→PUT race.
    public func edit(id: Int, _ mutate: (inout TodolistFields) throws -> Void) async throws -> Todolist {
        var fields = TodolistFields(from: try await fetchTodolist(id: id))
        try mutate(&fields)
        return try await putFields(id: id, fields: fields)
    }

    /// PUTs the full writable state via `replace`. Both fields are ALWAYS sent,
    /// the description included when empty: `""` is how a clear is expressed on
    /// a full-replace endpoint, and an explicit JSON null is never sent (SPEC
    /// §18 body compaction). `name` is presence-validated server-side, so an
    /// empty one is caught here rather than spent on a 422 round-trip.
    private func putFields(id: Int, fields: TodolistFields) async throws -> Todolist {
        guard !fields.name.isEmpty else {
            throw BasecampError.usage(
                message: "Todolist name cannot be empty",
                hint: "The server presence-validates a todolist's name; set a non-empty name."
            )
        }
        return try await replace(
            id: id,
            req: UpdateTodolistOrGroupRequest(
                description: fields.description,
                name: fields.name
            )
        )
    }

    /// GETs the todolist the composites read from, rendering a decoder failure
    /// as the SDK's own error shape.
    private func fetchTodolist(id: Int) async throws -> Todolist {
        // Swift's decoder is the typed guard the dynamic SDKs have to write by
        // hand, and it rejects a wrong-typed field before this composite ever
        // sees it. `BaseService.decoding(_:_:)` now renders that as the SPEC §6
        // malformed-2xx-body shape for every operation (#604) — statusless,
        // non-retryable `api_error` — so what is left to add here is the part
        // the base layer cannot know: the composite's escape hatch.
        // `BaseService.malformedBodyMessage` is what recognizes that one
        // failure — statuslessness alone would also match the pagination
        // same-origin guard — so every other error passes through untouched.
        let todolist: Todolist
        do {
            todolist = try await get(id: id)
        } catch let error as BasecampError {
            guard let message = BaseService.malformedBodyMessage(error) else { throw error }
            throw BasecampError.api(
                message: message,
                httpStatus: nil,
                hint: "The merge-safe update/edit resend this record's fields verbatim, so a "
                    + "malformed response cannot be written back safely. Use replace(id:req:) "
                    + "to write the record deliberately.",
                requestId: nil
            )
        }

        // Classification is by origin, not by value. The same empty name is a
        // caller error when the caller set it and a malformed response when it
        // came off the wire, so each provenance is checked where it is
        // unambiguous: here for the response, `putFields` for the caller.
        // `Todolist.name` is a non-optional `String`, so Codable already rejects
        // an absent or null one — an empty one is the case that still gets
        // through, and BC3 presence-validates the attribute, so no real todolist
        // has it.
        guard !todolist.name.isEmpty else {
            throw BasecampError.api(
                message: "GetTodolistOrGroup returned a todolist whose name is empty",
                httpStatus: nil,
                hint: "The name is presence-validated server-side, so an empty one is a "
                    + "malformed response. The caller did not ask to clear it.",
                requestId: nil
            )
        }
        return todolist
    }
}
