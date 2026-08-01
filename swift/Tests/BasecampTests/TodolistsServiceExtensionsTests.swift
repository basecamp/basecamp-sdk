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
            guard case .dataCorrupted(let context) = error else {
                return XCTFail("expected DecodingError.dataCorrupted, got \(error)")
            }
            XCTAssertEqual(context.debugDescription, "TodolistOrGroup: body matched no variant")
        }
    }

    /// A body that only the `group` projection can decode (no
    /// `description_attachments`, which `Todolist` requires) is refused by the
    /// composite rather than converted: the group arm has no `description`
    /// field, so writing it back would erase the description.
    func testUpdate_refusesAGroupOnlyProjection() async throws {
        var groupOnly = flatTodolistJSON()
        groupOnly.removeValue(forKey: "description_attachments")
        groupOnly.removeValue(forKey: "description")

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
