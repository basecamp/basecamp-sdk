import XCTest
@testable import Basecamp

/// Thread-safe capture of requests seen by the mock transport.
private final class TodolistRequestLog: @unchecked Sendable {
    private let lock = NSLock()
    private var _methods: [String] = []
    private var _putBody: [String: Any]?

    var methods: [String] { lock.withLock { _methods } }
    var putBody: [String: Any]? { lock.withLock { _putBody } }

    func record(_ request: URLRequest) {
        lock.withLock {
            _methods.append(request.httpMethod ?? "?")
            if request.httpMethod == "PUT",
                let data = request.httpBody ?? request.todolistBodyStreamData()
            {
                _putBody = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
            }
        }
    }
}

extension URLRequest {
    /// URLSession moves httpBody into a stream in some paths; drain it if needed.
    fileprivate func todolistBodyStreamData() -> Data? {
        guard let stream = httpBodyStream else { return nil }
        stream.open()
        defer { stream.close() }
        var data = Data()
        let bufferSize = 4096
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: bufferSize)
        defer { buffer.deallocate() }
        while stream.hasBytesAvailable {
            let read = stream.read(buffer, maxLength: bufferSize)
            if read <= 0 { break }
            data.append(buffer, count: read)
        }
        return data
    }
}

private final class TodolistOperationRecorder: BasecampHooks, @unchecked Sendable {
    private let lock = NSLock()
    private var _operations: [String] = []

    var operations: [String] { lock.withLock { _operations } }

    func onOperationStart(_ info: OperationInfo) {
        lock.withLock { _operations.append(info.operation) }
    }
}

/// A todolist body as BC3 actually renders it: FLAT, with no `todolist`
/// envelope around it (spec/fixtures/todolists/get.json). `groups_url` is
/// present because this recording's parent is a Todoset — see
/// ``flatTodolistGroupJSON`` for the group spelling of the same shape.
/// `color` is `Any` rather than `String?` on purpose: it is required-AND-nullable,
/// so an uncolored list carries an explicit JSON `null` (`NSNull()`) and never a
/// missing key. A `String?` default would omit the key instead, which is a body
/// BC3 never sends.
private func flatTodolistJSON(
    id: Int = 2,
    name: String = "Hardware",
    description: String = "<p>Ship the hardware</p>",
    color: Any = "blue"
) -> [String: Any] {
    [
        "id": id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2024-01-15T10:00:00Z",
        "updated_at": "2024-01-15T10:00:00Z",
        "title": name,
        "inherits_status": true,
        "type": "Todolist",
        "url": "https://3.basecampapi.com/999999999/buckets/1/todolists/\(id).json",
        "app_url": "https://3.basecamp.com/999999999/buckets/1/todolists/\(id)",
        "bookmark_url": "https://3.basecampapi.com/999999999/my/bookmarks/abc123.json",
        "subscription_url":
            "https://3.basecampapi.com/999999999/buckets/1/recordings/\(id)/subscription.json",
        "bubble_up_url":
            "https://3.basecampapi.com/999999999/buckets/1/recordings/\(id)/bubble_up.json",
        "comments_count": 0,
        "comments_url":
            "https://3.basecampapi.com/999999999/buckets/1/recordings/\(id)/comments.json",
        "position": 1,
        "parent": [
            "id": 3, "title": "To-dos", "type": "Todoset",
            "url": "https://3.basecampapi.com/999999999/buckets/1/todosets/3.json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/todosets/3",
        ] as [String: Any],
        "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
        "creator": ["id": 1, "name": "Test User"] as [String: Any],
        "description": description,
        "description_attachments": [],
        "completed": false,
        "completed_ratio": "0/3",
        "name": name,
        "todos_url": "https://3.basecampapi.com/999999999/buckets/1/todolists/\(id)/todos.json",
        "groups_url": "https://3.basecampapi.com/999999999/buckets/1/todolists/\(id)/groups.json",
        "app_todos_url": "https://3.basecamp.com/999999999/buckets/1/todolists/\(id)/todos",
        "color": color,
        "comments_app_url":
            "https://3.basecamp.com/999999999/buckets/1/recordings/\(id)/comments",
    ]
}

/// A to-do list GROUP as BC3 actually renders it
/// (spec/fixtures/todolist_groups/get.json). There is no group model: the group
/// views render `todolists/_todolist.json.jbuilder`, so the body is the very
/// same flat todolist shape — `"type": "Todolist"`, a real `description`, and
/// `description_attachments`. The only structural difference is that
/// `group_position_url` stands in for `groups_url`, because the parent is a
/// Todolist rather than a Todoset. Nothing in the SDK may branch on the `type`
/// string. `color` is an explicit `null` here: `recordings.color` is unset for
/// an uncolored group, which per bc3 is the ordinary case, and the key is still
/// emitted.
private func flatTodolistGroupJSON(
    id: Int = 7,
    name: String = "Phase 1",
    description: String = "<p>Phase one hardware work</p>"
) -> [String: Any] {
    var group = flatTodolistJSON(
        id: id, name: name, description: description, color: NSNull())
    group.removeValue(forKey: "groups_url")
    group["group_position_url"] =
        "https://3.basecampapi.com/999999999/buckets/1/todolists/groups/\(id)/position.json"
    group["parent"] = [
        "id": 2, "title": "Hardware", "type": "Todolist",
        "url": "https://3.basecampapi.com/999999999/buckets/1/todolists/2.json",
        "app_url": "https://3.basecamp.com/999999999/buckets/1/todolists/2",
    ] as [String: Any]
    return group
}

final class TodolistsServiceExtensionsTests: XCTestCase {

    /// Answers GETs with `getBody` and PUTs with `putBody` (defaulting to the
    /// same body), so a composite's return value can be pinned to the PUT
    /// response rather than the one it read.
    private func makeTodolistsClient(
        log: TodolistRequestLog,
        getBody: [String: Any] = flatTodolistJSON(),
        putBody: [String: Any]? = nil,
        hooks: (any BasecampHooks)? = nil
    ) throws -> AccountClient {
        let getData = try JSONSerialization.data(withJSONObject: getBody)
        let putData = try JSONSerialization.data(withJSONObject: putBody ?? getBody)
        let transport = MockTransport { request in
            log.record(request)
            let body = request.httpMethod == "PUT" ? putData : getData
            return (
                body,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        return makeTestAccountClient(transport: transport, hooks: hooks)
    }

    // MARK: - update (merge-safe)

    func testUpdate_nameOnlyPreservesDescription() async throws {
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(
            log: log,
            putBody: flatTodolistJSON(name: "Renamed list")
        )

        let todolist = try await account.todolists.update(
            id: 2, req: UpdateTodolistRequest(name: "Renamed list"))

        XCTAssertEqual(todolist.id, 2)
        XCTAssertEqual(todolist.name, "Renamed list", "the PUT response is what is returned")
        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["name"] as? String, "Renamed list")
        XCTAssertEqual(
            body["description"] as? String, "<p>Ship the hardware</p>",
            "the unmentioned description must be carried over — BC3 rebuilds the recordable "
                + "from the permitted params, so an omitted description is erased")
        XCTAssertEqual(
            Set(body.keys), Set(["description", "name"]),
            "the writable set on this endpoint is exactly {name, description}")
    }

    func testUpdate_descriptionOnlyPreservesName() async throws {
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log)

        _ = try await account.todolists.update(
            id: 2, req: UpdateTodolistRequest(description: "<p>New plan</p>"))

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["description"] as? String, "<p>New plan</p>")
        XCTAssertEqual(body["name"] as? String, "Hardware")
    }

    func testUpdate_explicitEmptyDescriptionClearsPresentAndEmpty() async throws {
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log)

        _ = try await account.todolists.update(id: 2, req: UpdateTodolistRequest(description: ""))

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(
            body["description"] as? String, "",
            "a clear is present-and-empty, never JSON null (SPEC §18 body compaction)")
        XCTAssertNotNil(
            body["description"] as? String, "a JSON null would not cast to String")
        XCTAssertEqual(body["name"] as? String, "Hardware")
    }

    func testUpdate_hooksObserveGetThenReplace() async throws {
        let log = TodolistRequestLog()
        let recorder = TodolistOperationRecorder()
        let account = try makeTodolistsClient(log: log, hooks: recorder)

        _ = try await account.todolists.update(
            id: 2, req: UpdateTodolistRequest(name: "observed"))

        XCTAssertEqual(recorder.operations, ["GetTodolistOrGroup", "UpdateTodolistOrGroup"])
    }

    /// A group is rendered by the same `todolists/_todolist.json.jbuilder`
    /// partial as a list, so it carries `description` and reports
    /// `"type": "Todolist"`; only `group_position_url` (vs `groups_url`)
    /// distinguishes it. The composite must be variant-agnostic — no
    /// type-sniffing, no branching — and preserve the description either way.
    func testUpdate_preservesDescriptionOnAGroupShapedBody() async throws {
        let groupBody = flatTodolistGroupJSON(
            id: 2, name: "Hardware", description: "<p>Ship the hardware</p>")

        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log, getBody: groupBody)

        _ = try await account.todolists.update(
            id: 2, req: UpdateTodolistRequest(name: "Renamed group"))

        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["name"] as? String, "Renamed group")
        XCTAssertEqual(body["description"] as? String, "<p>Ship the hardware</p>")
    }

    // MARK: - edit (read-modify-write closure)

    func testEdit_clearsDescriptionAndKeepsName() async throws {
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(
            log: log,
            putBody: flatTodolistJSON(description: "")
        )

        let todolist = try await account.todolists.edit(id: 2) { fields in
            XCTAssertEqual(fields.name, "Hardware")
            XCTAssertEqual(fields.description, "<p>Ship the hardware</p>")
            fields.description = ""
        }

        XCTAssertEqual(todolist.description, "")
        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["description"] as? String, "")
        XCTAssertEqual(body["name"] as? String, "Hardware")
    }

    func testEdit_renamesAndKeepsDescription() async throws {
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log)

        _ = try await account.todolists.edit(id: 2) { fields in
            fields.name = "🚨 " + fields.name
        }

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["name"] as? String, "🚨 Hardware")
        XCTAssertEqual(body["description"] as? String, "<p>Ship the hardware</p>")
    }

    func testEdit_closureErrorAbortsWithoutPut() async throws {
        struct Abort: Error {}
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log)

        do {
            _ = try await account.todolists.edit(id: 2) { fields in
                fields.name = "never written"
                throw Abort()
            }
            XCTFail("expected the closure error to propagate")
        } catch is Abort {
            // expected
        }

        XCTAssertEqual(log.methods, ["GET"], "no PUT after a closure error")
    }

    func testEdit_hooksObserveGetThenReplace() async throws {
        let log = TodolistRequestLog()
        let recorder = TodolistOperationRecorder()
        let account = try makeTodolistsClient(log: log, hooks: recorder)

        _ = try await account.todolists.edit(id: 2) { fields in
            fields.name = "observed"
        }

        XCTAssertEqual(recorder.operations, ["GetTodolistOrGroup", "UpdateTodolistOrGroup"])
    }

    // MARK: - empty name is caught client-side

    func testUpdate_emptyNameThrowsUsageErrorBeforeThePut() async throws {
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log)

        do {
            _ = try await account.todolists.update(id: 2, req: UpdateTodolistRequest(name: ""))
            XCTFail("expected an empty name to be rejected")
        } catch let error as BasecampError {
            guard case .usage(let message, let hint) = error else {
                return XCTFail("expected BasecampError.usage, got \(error)")
            }
            XCTAssertEqual(message, "Todolist name cannot be empty")
            XCTAssertEqual(
                hint, "The server presence-validates a todolist's name; set a non-empty name.")
            XCTAssertEqual(error.exitCode, 1)
        }

        XCTAssertEqual(log.methods, ["GET"], "the PUT must never be issued")
    }

    func testEdit_emptyNameThrowsUsageErrorBeforeThePut() async throws {
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log)

        do {
            _ = try await account.todolists.edit(id: 2) { fields in fields.name = "" }
            XCTFail("expected an empty name to be rejected")
        } catch let error as BasecampError {
            guard case .usage(let message, _) = error else {
                return XCTFail("expected BasecampError.usage, got \(error)")
            }
            XCTAssertEqual(message, "Todolist name cannot be empty")
        }

        XCTAssertEqual(log.methods, ["GET"], "the PUT must never be issued")
    }

    // MARK: - replace (server-native verbatim PUT)

    func testReplace_sendsSparseVerbatimWithNoGet() async throws {
        let log = TodolistRequestLog()
        let recorder = TodolistOperationRecorder()
        let account = try makeTodolistsClient(log: log, hooks: recorder)

        let result = try await account.todolists.replace(
            id: 2, req: UpdateTodolistOrGroupRequest(name: "The whole new list"))

        XCTAssertEqual(result.id, 2)
        XCTAssertEqual(log.methods, ["PUT"], "replace must not GET")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body.count, 1, "sparse in, sparse out")
        XCTAssertEqual(body["name"] as? String, "The whole new list")
        XCTAssertNil(
            body["description"],
            "an omitted description is omitted verbatim — the server clears it")
        XCTAssertEqual(recorder.operations, ["UpdateTodolistOrGroup"])
    }

    // MARK: - flat Todolist decoding (#544)

    /// Regression test for the read path. `GetTodolistOrGroup` used to be
    /// modelled as a `oneOf` of a `todolist`/`group` envelope, and the
    /// synthesised two-arm `Codable` decoded BC3's flat body into two nil
    /// members and reported SUCCESS — `todolists.get()` handed back an empty
    /// value with no error. The operation now carries a plain `Todolist`, whose
    /// required members make an unmatched body a decode failure instead.
    func testGetDecodesAFlatTodolistBody() async throws {
        let data = try JSONSerialization.data(withJSONObject: flatTodolistJSON())
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        let todolist = try await account.todolists.get(id: 2)

        XCTAssertEqual(todolist.id, 2)
        XCTAssertEqual(todolist.name, "Hardware")
        XCTAssertEqual(todolist.description, "<p>Ship the hardware</p>")
        XCTAssertEqual(todolist.descriptionAttachments.count, 0)
        XCTAssertEqual(todolist.type, "Todolist")
        XCTAssertEqual(
            todolist.groupsUrl,
            "https://3.basecampapi.com/999999999/buckets/1/todolists/2/groups.json",
            "a list's parent is a Todoset, so it carries groups_url")
        XCTAssertNil(todolist.groupPositionUrl, "a list has no position within a group")
    }

    /// The case #544 is actually about: `todolists.get()` on a GROUP id.
    ///
    /// A group is rendered by the very same
    /// `todolists/_todolist.json.jbuilder` partial as a list, so it arrives as
    /// the identical flat shape carrying a real `description` and
    /// `"type": "Todolist"`; only `group_position_url` in place of `groups_url`
    /// tells them apart. The old union expected wrapper keys, matched neither
    /// arm on this body, and returned an all-nil value while reporting success.
    /// The flat model must decode it like any other todolist — with the
    /// description intact, since that is what the merge-safe composites write
    /// back.
    func testGetDecodesAFlatGroupBody() async throws {
        let data = try JSONSerialization.data(withJSONObject: flatTodolistGroupJSON())
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        let group = try await account.todolists.get(id: 7)

        XCTAssertEqual(group.id, 7)
        XCTAssertEqual(group.name, "Phase 1", "the group's name must not come back empty")
        XCTAssertEqual(
            group.description, "<p>Phase one hardware work</p>",
            "a group carries a description like any other todolist")
        XCTAssertEqual(group.descriptionAttachments.count, 0)
        XCTAssertEqual(group.type, "Todolist", "BC3 reports a group as a Todolist")
        XCTAssertEqual(
            group.groupPositionUrl,
            "https://3.basecampapi.com/999999999/buckets/1/todolists/groups/7/position.json",
            "the structural discriminator is group_position_url, never the type string")
        XCTAssertNil(group.groupsUrl, "a group's parent is a Todolist, so it has no groups_url")
        XCTAssertEqual(group.parent.type, "Todolist")
    }

    /// The group LIST decodes into an array of the same flat shape. The old
    /// group projection modelled no `description` at all, so every element came
    /// back with it dropped; `ListTodolistGroups` now carries `Todolist`.
    func testListTodolistGroupsDecodesFlatElements() async throws {
        let body = [
            flatTodolistGroupJSON(),
            flatTodolistGroupJSON(id: 8, name: "Phase 2", description: ""),
        ]
        let data = try JSONSerialization.data(withJSONObject: body)
        let transport = MockTransport(
            statusCode: 200, data: data,
            headers: ["Content-Type": "application/json", "X-Total-Count": "2"])
        let account = makeTestAccountClient(transport: transport)

        let groups = try await account.todolistGroups.list(todolistId: 2)

        XCTAssertEqual(groups.items.count, 2)
        let first = try XCTUnwrap(groups.items.first)
        XCTAssertEqual(first.id, 7)
        XCTAssertEqual(first.name, "Phase 1")
        XCTAssertEqual(
            first.description, "<p>Phase one hardware work</p>",
            "the list elements must keep the description the group projection dropped")
        XCTAssertEqual(
            first.groupPositionUrl,
            "https://3.basecampapi.com/999999999/buckets/1/todolists/groups/7/position.json")
        XCTAssertEqual(
            groups.items.last?.description, "",
            "a blank rich text is \"\" on the wire, never JSON null")
    }

    /// A wrong-typed required field must surface its real decoding error rather
    /// than being laundered into a narrower shape. Under the union, a body that
    /// WAS a todolist but carried one malformed todolist-only field failed the
    /// todolist arm and then decoded cleanly as the group arm, which simply
    /// ignored the bad key — a success with the description silently discarded.
    /// With one flat model there is nowhere to fall back to, and this pins that.
    func testGetSurfacesAMalformedDescriptionAttachments() async throws {
        var malformed = flatTodolistJSON()
        malformed["description_attachments"] = "not-an-array"
        let data = try JSONSerialization.data(withJSONObject: malformed)
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        do {
            let todolist = try await account.todolists.get(id: 2)
            XCTFail("expected a decoding error, got \(todolist)")
        } catch let error as BasecampError {
            // Since #604 the decoder's refusal wears the SPEC §6
            // malformed-2xx-body shape, and since #750 it carries the
            // `DecodingError` itself in `decodeFailure` (asserted by the helper
            // below). Its own description, coding path included, is still
            // interpolated into the message, which is what these assertions
            // read — the message is where a human looks.
            let message = assertStatuslessDecodeFailure(error)
            XCTAssertTrue(message.contains("typeMismatch"), message)
            // camelCase because BaseService.decoder applies .convertFromSnakeCase.
            XCTAssertTrue(
                message.contains("descriptionAttachments"),
                "the error must name the field that actually failed: \(message)")
        }
    }

    /// The same contract for a multi-word OPTIONAL key. `groups_url` is
    /// optional, and `decodeIfPresent` returns nil only for an absent or null
    /// key — a wrong-typed one still throws. This is the case the union's
    /// masking guard existed to protect, and it has to keep holding without it.
    func testGetSurfacesAMalformedGroupsUrl() async throws {
        var malformed = flatTodolistJSON()
        malformed["groups_url"] = 12345  // wrong type; String expected

        let data = try JSONSerialization.data(withJSONObject: malformed)
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        do {
            let todolist = try await account.todolists.get(id: 2)
            XCTFail("expected a decoding error, got \(todolist)")
        } catch let error as BasecampError {
            let message = assertStatuslessDecodeFailure(error)
            XCTAssertTrue(message.contains("typeMismatch"), message)
            XCTAssertTrue(message.contains("groupsUrl"), message)
        }
    }

    /// A body that is not a todolist is an error, never a silently empty value
    /// — the assertion that would have caught the original defect.
    func testGetRejectsABodyThatIsNotATodolist() async throws {
        let data = try JSONSerialization.data(withJSONObject: [:] as [String: Any])
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        do {
            let todolist = try await account.todolists.get(id: 2)
            XCTFail("expected a decoding error, got \(todolist)")
        } catch let error as BasecampError {
            let message = assertStatuslessDecodeFailure(error)
            XCTAssertTrue(message.contains("keyNotFound"), message)
            XCTAssertTrue(
                message.contains("appUrl"),
                "the empty body must report the first missing required key: \(message)")
        }
    }

    /// `description` is required and non-nullable: BC3's `format_api_content`
    /// renders a blank rich text as `""`, never JSON null, so a null one is a
    /// malformed response rather than a value to carry.
    func testGetRejectsANullDescription() async throws {
        var malformed = flatTodolistJSON()
        malformed["description"] = NSNull()
        let data = try JSONSerialization.data(withJSONObject: malformed)
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        do {
            let todolist = try await account.todolists.get(id: 2)
            XCTFail("expected a decoding error, got \(todolist)")
        } catch let error as BasecampError {
            let message = assertStatuslessDecodeFailure(error)
            XCTAssertTrue(message.contains("valueNotFound"), message)
            XCTAssertTrue(message.contains("description"), message)
        }
    }

    /// `name` is presence-validated server-side, so an empty one from the wire
    /// is a malformed response — not a value to preserve, and emphatically not
    /// one to write back on a full-replace endpoint. Classification is by
    /// ORIGIN: this name came off the wire, so it is `.api`; a caller-emptied
    /// name stays `.usage` (covered by testEdit_emptyNameThrowsUsageError...).
    /// Swift's Codable already rejects an absent or null name, since
    /// `Todolist.name` is a non-optional `String`.
    func testUpdate_refusesAnEmptyNameFromTheResponse() async throws {
        var body = flatTodolistJSON()
        body["name"] = ""

        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log, getBody: body)

        do {
            _ = try await account.todolists.update(
                id: 2, req: UpdateTodolistRequest(description: "<p>New</p>"))
            XCTFail("expected an empty name from the wire to be refused")
        } catch let error as BasecampError {
            guard case .api(let message, _, _, _, _) = error else {
                return XCTFail("expected .api for a malformed response, got \(error)")
            }
            XCTAssertTrue(
                message.contains("name is empty"),
                "the error must name the offending field, got: \(message)")
        }

        XCTAssertEqual(log.methods, ["GET"], "an empty name must never reach the PUT")
    }

    /// A body that does not decode must surface as a statusless BasecampError,
    /// not a raw DecodingError. Swift's decoder is the typed guard the dynamic
    /// SDKs write by hand, but its native error is not the SPEC section 6 shape:
    /// callers checking for BasecampError would miss it and it carries no hint.
    func testUpdate_wrapsADecodeFailureAsAStatuslessApiError() async throws {
        var malformed = flatTodolistJSON()
        malformed["id"] = "not-an-int"

        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log, getBody: malformed)

        do {
            _ = try await account.todolists.update(
                id: 2, req: UpdateTodolistRequest(name: "Renamed"))
            XCTFail("expected a decode failure to surface as a BasecampError")
        } catch let error as BasecampError {
            guard case .api(let message, let httpStatus, let hint, _, _) = error else {
                return XCTFail("expected .api for a malformed body, got \(error)")
            }
            XCTAssertNil(httpStatus, "the transport succeeded, so there is no status to report")
            XCTAssertNotNil(hint)
            XCTAssertLessThanOrEqual(message.count, 500, "SPEC section 9 caps the message")
            XCTAssertTrue(message.contains("does not decode"), message)
            // The restatement is still that decode failure, so it carries the
            // marker forward (#750). The message assertion above is a
            // convenience for a human reading a failure; this is the contract.
            XCTAssertNotNil(
                error.decodeFailure, "the composite's restatement must keep the marker")
        } catch {
            XCTFail("expected BasecampError, got a raw \(type(of: error)): \(error)")
        }

        XCTAssertEqual(log.methods, ["GET"], "a malformed body must never reach the PUT")
    }

    /// The merge-safe composite must work on a GROUP id end-to-end, not just
    /// decode one: GET the group, overlay the caller's field, and PUT back the
    /// group's own description. Nothing branches on the variant.
    func testUpdate_onAGroupIdPreservesTheGroupsDescription() async throws {
        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(
            log: log,
            getBody: flatTodolistGroupJSON(),
            putBody: flatTodolistGroupJSON(name: "Phase one"))

        let written = try await account.todolists.update(
            id: 7, req: UpdateTodolistRequest(name: "Phase one"))

        XCTAssertEqual(written.name, "Phase one")
        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["name"] as? String, "Phase one")
        XCTAssertEqual(
            body["description"] as? String, "<p>Phase one hardware work</p>",
            "the group's description must be resent verbatim — the PUT is a full replace")
    }
}

extension XCTestCase {
    /// Asserts the SPEC §6 malformed-2xx-body shape and hands back the message
    /// so the caller can assert what the decoder said about *which* field.
    /// A statusless `.api` is the only shape a plain GET produces without a
    /// status: every other `.api` carries the one it came from.
    func assertStatuslessDecodeFailure(
        _ error: BasecampError, file: StaticString = #filePath, line: UInt = #line
    ) -> String {
        guard case .api(let message, let httpStatus, _, _, _) = error else {
            XCTFail("expected .api for a malformed body, got \(error)", file: file, line: line)
            return ""
        }
        XCTAssertNil(
            httpStatus, "the transport succeeded, so no status describes this", file: file,
            line: line)
        XCTAssertFalse(
            error.isRetryable, "re-requesting cannot repair a malformed body", file: file,
            line: line)
        XCTAssertNotNil(
            error.decodeFailure, "the base layer's producer sets the #750 marker", file: file,
            line: line)
        return message
    }
}
