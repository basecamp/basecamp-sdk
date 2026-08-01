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
/// envelope around it (spec/fixtures/todolists/get.json). The `todolist` /
/// `group` union is a spec-modelling convention — see AGENTS.md, "Smithy Spec
/// vs Actual API Responses".
private func flatTodolistJSON(
    id: Int = 2,
    name: String = "Hardware",
    description: String = "<p>Ship the hardware</p>"
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
    ]
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
        var groupBody = flatTodolistJSON()
        groupBody.removeValue(forKey: "groups_url")
        groupBody["group_position_url"] =
            "https://3.basecampapi.com/999999999/buckets/1/todolists/groups/2/position.json"
        groupBody["parent"] = [
            "id": 9, "title": "Hardware", "type": "Todolist",
            "url": "https://3.basecampapi.com/999999999/buckets/1/todolists/9.json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/todolists/9",
        ] as [String: Any]

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

        let returned = try XCTUnwrap(result.todolist)
        XCTAssertEqual(returned.id, 2)
        XCTAssertEqual(log.methods, ["PUT"], "replace must not GET")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body.count, 1, "sparse in, sparse out")
        XCTAssertEqual(body["name"] as? String, "The whole new list")
        XCTAssertNil(
            body["description"],
            "an omitted description is omitted verbatim — the server clears it")
        XCTAssertEqual(recorder.operations, ["UpdateTodolistOrGroup"])
    }

    // MARK: - TodolistOrGroup union decoding

    /// Regression test for the union decoder. The synthesised `Codable` decoded
    /// BC3's flat todolist body into an all-nil struct and reported SUCCESS, so
    /// `todolists.get()` handed back an empty value with no error. The custom
    /// `init(from:)` now matches the flat body against the arms in order.
    func testUnionDecodesAFlatTodolistBody() async throws {
        let data = try JSONSerialization.data(withJSONObject: flatTodolistJSON())
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        let result = try await account.todolists.get(id: 2)

        let todolist = try XCTUnwrap(
            result.todolist, "the flat body must land in the todolist arm, not nowhere")
        XCTAssertEqual(todolist.id, 2)
        XCTAssertEqual(todolist.name, "Hardware")
        XCTAssertEqual(todolist.description, "<p>Ship the hardware</p>")
        XCTAssertEqual(todolist.descriptionAttachments.count, 0)
        XCTAssertNil(result.group, "only one arm is populated")
    }

    /// A malformed todolist must surface its real decoding error, not be
    /// laundered into the narrower arm.
    ///
    /// TodolistGroup's property set is a strict subset of Todolist's — it has
    /// zero exclusive keys, and `description_attachments` is required on
    /// Todolist alone. So a body that IS a todolist but carries one malformed
    /// todolist-only field fails the todolist arm and then decodes *cleanly* as
    /// a group, which simply ignores the bad key. Without the guard the caller
    /// gets a successful `.group` with the description silently discarded —
    /// the same swallow-and-report-success shape as the original union defect.
    func testUnionSurfacesAMalformedTodolistRatherThanFallingBackToGroup() async throws {
        var malformed = flatTodolistJSON()
        malformed["description_attachments"] = "not-an-array"
        let data = try JSONSerialization.data(withJSONObject: malformed)
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        do {
            let result = try await account.todolists.get(id: 2)
            XCTFail(
                "expected the todolist arm's decoding error to surface, got "
                    + "todolist=\(String(describing: result.todolist)) "
                    + "group=\(String(describing: result.group))")
        } catch let error as DecodingError {
            guard case .typeMismatch(_, let context) = error else {
                return XCTFail("expected DecodingError.typeMismatch, got \(error)")
            }
            // camelCase because BaseService.decoder applies .convertFromSnakeCase.
            XCTAssertEqual(
                context.codingPath.last?.stringValue, "descriptionAttachments",
                "the error must name the field that actually failed")
        }
    }

    /// The masking guard must match on multi-word keys, not just single-word
    /// ones that happen to survive snake_case conversion unchanged.
    ///
    /// `BaseService.decoder` sets `.convertFromSnakeCase`, so the raw wire keys
    /// arrive at the guard already camelCased. An earlier version of this guard
    /// listed only the wire spelling and passed this file solely because
    /// `description` is one word — every multi-word key silently failed to
    /// match. Here the only todolist-exclusive key in the body is `groups_url`,
    /// so the guard has to handle the converted spelling to fire at all.
    func testUnionMaskingGuardMatchesMultiWordKeys() async throws {
        var malformed = flatTodolistJSON()
        // Drop the one single-word exclusive key, so `description_attachments`
        // and `groups_url` — both multi-word, both arriving camelCased — are
        // the only things the guard can match on.
        malformed.removeValue(forKey: "description")
        malformed["groups_url"] = 12345  // wrong type; String expected

        let data = try JSONSerialization.data(withJSONObject: malformed)
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        do {
            let result = try await account.todolists.get(id: 2)
            XCTFail(
                "the group arm must not absorb a todolist carrying groups_url, got "
                    + "group=\(String(describing: result.group))")
        } catch let error as DecodingError {
            guard case .typeMismatch(_, let context) = error else {
                return XCTFail("expected DecodingError.typeMismatch, got \(error)")
            }
            XCTAssertEqual(context.codingPath.last?.stringValue, "groupsUrl")
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
            guard case .api(let message, _, _, _) = error else {
                return XCTFail("expected .api for a malformed response, got \(error)")
            }
            XCTAssertTrue(
                message.contains("name is empty"),
                "the error must name the offending field, got: \(message)")
        }

        XCTAssertEqual(log.methods, ["GET"], "an empty name must never reach the PUT")
    }

    /// The `{"todolist": {...}}` envelope the spec models still decodes, so the
    /// flat-body support is an addition rather than a swap.
    func testUnionDecodesTheEnvelopeShape() throws {
        let data = try JSONSerialization.data(
            withJSONObject: ["todolist": flatTodolistJSON()] as [String: Any])

        let decoded = try BaseService.decoder.decode(TodolistOrGroup.self, from: data)

        let todolist = try XCTUnwrap(decoded.todolist)
        XCTAssertEqual(todolist.name, "Hardware")
        XCTAssertNil(decoded.group)
    }

    /// A body matching no arm is now an error rather than a silent all-nil
    /// value. This is the assertion that would have caught the original defect.
    ///
    /// The surfaced error is the *first arm's* — an empty body fails `Todolist`
    /// on its first required key — rather than a generic "matched no variant".
    /// Reporting the underlying cause is the same principle as the masking
    /// guard above: the decoder never invents an error of its own while it
    /// still holds a real one.
    func testUnionRejectsABodyMatchingNoArm() async throws {
        let data = try JSONSerialization.data(withJSONObject: [:] as [String: Any])
        let transport = MockTransport(
            statusCode: 200, data: data, headers: ["Content-Type": "application/json"])
        let account = makeTestAccountClient(transport: transport)

        do {
            let result = try await account.todolists.get(id: 2)
            XCTFail(
                "expected a decoding error, got todolist=\(String(describing: result.todolist)) "
                    + "group=\(String(describing: result.group))")
        } catch let error as DecodingError {
            guard case .keyNotFound(let key, _) = error else {
                return XCTFail("expected DecodingError.keyNotFound, got \(error)")
            }
            XCTAssertEqual(
                key.stringValue, "appUrl",
                "the empty body must report the todolist arm's first missing required key")
        }
    }

    /// A body that only the `group` projection can decode (no
    /// `description_attachments`, which `Todolist` requires) is refused by the
    /// composite rather than converted: the group arm has no `description`
    /// field, so writing it back would erase the description.
    func testUpdate_refusesAGroupOnlyProjection() async throws {
        // A realistic group-only body carries no todolist-exclusive key at all:
        // BC3 emits group_position_url in place of groups_url, and this
        // projection models neither description nor description_attachments.
        // Leaving groups_url in would make it a malformed *todolist* instead,
        // which the union's masking guard now correctly refuses to launder into
        // the group arm.
        var groupOnly = flatTodolistJSON()
        groupOnly.removeValue(forKey: "description_attachments")
        groupOnly.removeValue(forKey: "description")
        groupOnly.removeValue(forKey: "boosts_count")
        groupOnly.removeValue(forKey: "boosts_url")
        groupOnly.removeValue(forKey: "groups_url")
        groupOnly["group_position_url"] =
            "https://3.basecampapi.com/999999999/buckets/1/todolists/groups/2/position.json"

        let log = TodolistRequestLog()
        let account = try makeTodolistsClient(log: log, getBody: groupOnly)

        do {
            _ = try await account.todolists.update(
                id: 2, req: UpdateTodolistRequest(name: "Renamed"))
            XCTFail("expected the group-only projection to be refused")
        } catch let error as BasecampError {
            guard case .api(let message, let status, let hint, _) = error else {
                return XCTFail("expected BasecampError.api, got \(error)")
            }
            XCTAssertEqual(
                message,
                "GetTodolistOrGroup returned a todolist group projection, which carries no "
                    + "description; writing it back would erase the description")
            XCTAssertNil(status)
            XCTAssertEqual(hint, "Use replace(id:req:) to overwrite this recording deliberately.")
        }

        XCTAssertEqual(log.methods, ["GET"], "no PUT is issued from an unusable read")
    }
}
