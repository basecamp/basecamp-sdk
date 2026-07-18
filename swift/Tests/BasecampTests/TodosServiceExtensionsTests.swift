import XCTest
@testable import Basecamp

/// Thread-safe capture of requests seen by the mock transport.
private final class RequestLog: @unchecked Sendable {
    private let lock = NSLock()
    private var _methods: [String] = []
    private var _putBody: [String: Any]?

    var methods: [String] { lock.withLock { _methods } }
    var putBody: [String: Any]? { lock.withLock { _putBody } }

    func record(_ request: URLRequest) {
        lock.withLock {
            _methods.append(request.httpMethod ?? "?")
            if request.httpMethod == "PUT", let data = request.httpBody ?? request.bodyStreamData() {
                _putBody = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
            }
        }
    }
}

extension URLRequest {
    /// URLSession moves httpBody into a stream in some paths; drain it if needed.
    fileprivate func bodyStreamData() -> Data? {
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

private final class OperationRecorder: BasecampHooks, @unchecked Sendable {
    private let lock = NSLock()
    private var _operations: [String] = []

    var operations: [String] { lock.withLock { _operations } }

    func onOperationStart(_ info: OperationInfo) {
        lock.withLock { _operations.append(info.operation) }
    }
}

/// Full todo JSON on wire (snake_case) keys, with description, dates,
/// an assignee, and a completion subscriber populated.
private func fullTodoJSON(id: Int = 42) -> [String: Any] {
    [
        "id": id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "title": "Buy milk",
        "inherits_status": true,
        "type": "Todo",
        "url": "https://3.basecampapi.com/999999999/buckets/1/todos/\(id).json",
        "app_url": "https://3.basecamp.com/999999999/buckets/1/todos/\(id)",
        "content": "Buy milk",
        "description": "<p>From the store</p>",
        "due_on": "2024-03-01",
        "starts_on": "2024-02-01",
        "assignees": [["id": 100, "name": "Jane Doe"] as [String: Any]],
        "completion_subscribers": [["id": 555, "name": "Sub Scriber"] as [String: Any]],
        "completed": false,
        "parent": [
            "id": 2, "title": "Todolist", "type": "Todolist",
            "url": "https://3.basecampapi.com/999999999/buckets/1/todolists/2.json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/todolists/2"
        ] as [String: Any],
        "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
        "creator": ["id": 1, "name": "Test User"] as [String: Any]
    ]
}

final class TodosServiceExtensionsTests: XCTestCase {

    private func makeTodosClient(
        log: RequestLog,
        hooks: (any BasecampHooks)? = nil
    ) throws -> AccountClient {
        let todoData = try JSONSerialization.data(withJSONObject: fullTodoJSON())
        let transport = MockTransport { request in
            log.record(request)
            return (
                todoData,
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

    func testUpdate_mergesUnsetFields() async throws {
        let log = RequestLog()
        let account = try makeTodosClient(log: log)

        let todo = try await account.todos.update(todoId: 42, req: UpdateTodoRequest(content: "Updated task"))

        XCTAssertEqual(todo.id, 42)
        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["content"] as? String, "Updated task")
        XCTAssertEqual(body["description"] as? String, "<p>From the store</p>")
        XCTAssertEqual(body["due_on"] as? String, "2024-03-01")
        XCTAssertEqual(body["starts_on"] as? String, "2024-02-01")
        XCTAssertEqual(body["assignee_ids"] as? [Int], [100])
        XCTAssertEqual(body["completion_subscriber_ids"] as? [Int], [555])
        XCTAssertNil(body["notify"], "notify must be omitted unless true")
    }

    func testUpdate_explicitEmptyArrayClears() async throws {
        let log = RequestLog()
        let account = try makeTodosClient(log: log)

        _ = try await account.todos.update(todoId: 42, req: UpdateTodoRequest(assigneeIds: []))

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["assignee_ids"] as? [Int], [])
        XCTAssertEqual(body["completion_subscriber_ids"] as? [Int], [555])
        XCTAssertEqual(body["content"] as? String, "Buy milk")
    }

    func testUpdate_notifyOnlyWhenTrue() async throws {
        let log = RequestLog()
        let account = try makeTodosClient(log: log)

        _ = try await account.todos.update(todoId: 42, req: UpdateTodoRequest(content: "ping", notify: true))

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["notify"] as? Bool, true)
    }

    func testUpdate_hooksObserveGetThenReplace() async throws {
        let log = RequestLog()
        let recorder = OperationRecorder()
        let account = try makeTodosClient(log: log, hooks: recorder)

        _ = try await account.todos.update(todoId: 42, req: UpdateTodoRequest(content: "observed"))

        XCTAssertEqual(recorder.operations, ["GetTodo", "ReplaceTodo"])
    }

    // MARK: - edit (read-modify-write closure)

    func testEdit_putsFullStateBack() async throws {
        let log = RequestLog()
        let account = try makeTodosClient(log: log)

        let todo = try await account.todos.edit(todoId: 42) { fields in
            XCTAssertEqual(fields.content, "Buy milk")
            fields.content = "🚨 " + fields.content
        }

        XCTAssertEqual(todo.id, 42)
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["content"] as? String, "🚨 Buy milk")
        XCTAssertEqual(body["description"] as? String, "<p>From the store</p>")
        XCTAssertEqual(body["assignee_ids"] as? [Int], [100])
    }

    func testEdit_clearsDateByOmission() async throws {
        let log = RequestLog()
        let account = try makeTodosClient(log: log)

        _ = try await account.todos.edit(todoId: 42) { fields in
            XCTAssertEqual(fields.dueOn, "2024-03-01")
            fields.dueOn = ""
        }

        let body = try XCTUnwrap(log.putBody)
        XCTAssertNil(body["due_on"], "cleared date must be omitted from the PUT body")
        XCTAssertEqual(body["content"] as? String, "Buy milk")
    }

    func testEdit_clearsDescriptionAndIdsPresentAndEmpty() async throws {
        let log = RequestLog()
        let account = try makeTodosClient(log: log)

        _ = try await account.todos.edit(todoId: 42) { fields in
            fields.description = ""
            fields.assigneeIds = []
            fields.completionSubscriberIds = []
        }

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["description"] as? String, "")
        XCTAssertEqual(body["assignee_ids"] as? [Int], [])
        XCTAssertEqual(body["completion_subscriber_ids"] as? [Int], [])
    }

    func testEdit_closureErrorAbortsWithoutPut() async throws {
        struct Abort: Error {}
        let log = RequestLog()
        let account = try makeTodosClient(log: log)

        do {
            _ = try await account.todos.edit(todoId: 42) { fields in
                fields.content = "never written"
                throw Abort()
            }
            XCTFail("expected the closure error to propagate")
        } catch is Abort {
            // expected
        }

        XCTAssertEqual(log.methods, ["GET"], "no PUT after a closure error")
    }

    func testEdit_hooksObserveGetThenReplace() async throws {
        let log = RequestLog()
        let recorder = OperationRecorder()
        let account = try makeTodosClient(log: log, hooks: recorder)

        _ = try await account.todos.edit(todoId: 42) { fields in
            fields.content = "observed"
        }

        XCTAssertEqual(recorder.operations, ["GetTodo", "ReplaceTodo"])
    }

    // MARK: - replace (server-native verbatim PUT)

    func testReplace_sendsSparseVerbatimWithNoGet() async throws {
        let log = RequestLog()
        let recorder = OperationRecorder()
        let account = try makeTodosClient(log: log, hooks: recorder)

        let todo = try await account.todos.replace(todoId: 42, req: ReplaceTodoRequest(content: "the whole new todo"))

        XCTAssertEqual(todo.id, 42)
        XCTAssertEqual(log.methods, ["PUT"], "replace must not GET")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["content"] as? String, "the whole new todo")
        for field in ["description", "assignee_ids", "completion_subscriber_ids", "notify", "due_on", "starts_on"] {
            XCTAssertNil(body[field], "\(field) must be omitted from a sparse replace")
        }
        XCTAssertEqual(recorder.operations, ["ReplaceTodo"])
    }
}
