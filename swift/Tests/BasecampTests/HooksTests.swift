import XCTest
@testable import Basecamp

final class HooksTests: XCTestCase {

    func testNoopHooksDoNothing() {
        let hooks = NoopHooks()
        let info = OperationInfo(
            service: "Test", operation: "Get",
            resourceType: "test", isMutation: false
        )

        // Should not crash
        hooks.onOperationStart(info)
        hooks.onOperationEnd(info, result: OperationResult(durationMs: 0))
        hooks.onRequestStart(RequestInfo(method: "GET", url: "https://example.com", attempt: 1))
        hooks.onRequestEnd(
            RequestInfo(method: "GET", url: "https://example.com", attempt: 1),
            result: RequestResult(statusCode: 200, durationMs: 0)
        )
    }

    func testCustomHooksReceiveCallbacks() async throws {
        let spy = SpyHooks()

        let transport = MockTransport { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("{}".utf8), response)
        }

        _ = makeTestClient(transport: transport, hooks: spy)

        XCTAssertTrue(spy.operationStarts.isEmpty)

        // Hooks are invoked through BaseService operations, so we test
        // the OperationInfo structure directly here.
        let info = OperationInfo(
            service: "Todos", operation: "List",
            resourceType: "todo", isMutation: false,
            projectId: 123
        )
        XCTAssertEqual(info.service, "Todos")
        XCTAssertEqual(info.operation, "List")
        XCTAssertEqual(info.projectId, 123)
        XCTAssertFalse(info.isMutation)
    }

    // MARK: - P0 Regression: Single onOperationEnd on HTTP errors

    func testOnOperationEndFiresExactlyOnceOnHTTPError() async throws {
        let spy = SpyHooks()

        let transport = MockTransport(statusCode: 404, data: Data())
        let account = makeTestAccountClient(transport: transport, hooks: spy)

        do {
            let _: Todo = try await account.todos.get(projectId: 1, todoId: 2)
            XCTFail("Expected error")
        } catch {
            // Expected
        }

        XCTAssertEqual(spy.operationStarts.count, 1, "onOperationStart should fire once")
        XCTAssertEqual(spy.operationEnds.count, 1, "onOperationEnd should fire exactly once on HTTP error")
        XCTAssertNotNil(spy.operationEnds.first?.1.error, "onOperationEnd should include the error")
    }

    func testOnOperationEndFiresExactlyOnceOnHTTPErrorVoid() async throws {
        let spy = SpyHooks()

        let transport = MockTransport(statusCode: 500, data: Data())
        let account = makeTestAccountClient(transport: transport, hooks: spy)

        do {
            try await account.todos.complete(projectId: 1, todoId: 2)
            XCTFail("Expected error")
        } catch {
            // Expected
        }

        XCTAssertEqual(spy.operationStarts.count, 1, "onOperationStart should fire once")
        XCTAssertEqual(spy.operationEnds.count, 1, "onOperationEnd should fire exactly once on HTTP error (void)")
        XCTAssertNotNil(spy.operationEnds.first?.1.error)
    }

    func testOnOperationEndFiresExactlyOnceOnHTTPErrorPaginated() async throws {
        let spy = SpyHooks()

        let transport = MockTransport(statusCode: 403, data: Data())
        let account = makeTestAccountClient(transport: transport, hooks: spy)

        do {
            let _: ListResult<Todo> = try await account.todos.list(projectId: 1, todolistId: 2)
            XCTFail("Expected error")
        } catch {
            // Expected
        }

        XCTAssertEqual(spy.operationStarts.count, 1, "onOperationStart should fire once")
        XCTAssertEqual(spy.operationEnds.count, 1, "onOperationEnd should fire exactly once on HTTP error (paginated)")
        XCTAssertNotNil(spy.operationEnds.first?.1.error)
    }

    func testOnOperationEndFiresOnceOnSuccess() async throws {
        let spy = SpyHooks()

        let todo: [String: Any] = ["id": 1, "content": "Test"]
        let data = try JSONSerialization.data(withJSONObject: todo)
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport, hooks: spy)

        let _: Todo = try await account.todos.get(projectId: 1, todoId: 1)

        XCTAssertEqual(spy.operationStarts.count, 1)
        XCTAssertEqual(spy.operationEnds.count, 1, "onOperationEnd should fire exactly once on success")
        XCTAssertNil(spy.operationEnds.first?.1.error, "onOperationEnd should have no error on success")
    }

    // MARK: - P0 Regression: ETag 304 cache returns success

    func testETagCacheHitReturnsSuccessfully() async throws {
        let spy = SpyHooks()
        let todo: [String: Any] = ["id": 1, "content": "Cached"]
        let cachedData = try JSONSerialization.data(withJSONObject: todo)

        let counter = RequestCounter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                // First request: 200 with ETag
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 200,
                    httpVersion: "HTTP/1.1",
                    headerFields: ["ETag": "\"abc123\""]
                )!
                return (cachedData, response)
            } else {
                // Second request: 304 Not Modified
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 304,
                    httpVersion: "HTTP/1.1", headerFields: [:]
                )!
                return (Data(), response)
            }
        }

        let client = makeTestClient(transport: transport, enableCache: true, hooks: spy)
        let account = client.forAccount("999999999")

        // First request: populate cache
        let first: Todo = try await account.todos.get(projectId: 1, todoId: 1)
        XCTAssertEqual(first.id, 1)
        XCTAssertEqual(first.content, "Cached")

        // Second request: should use cache, not throw a 304 error
        let second: Todo = try await account.todos.get(projectId: 1, todoId: 1)
        XCTAssertEqual(second.id, 1)
        XCTAssertEqual(second.content, "Cached")

        // Both operations should succeed with exactly one start/end each
        XCTAssertEqual(spy.operationStarts.count, 2)
        XCTAssertEqual(spy.operationEnds.count, 2)
        XCTAssertNil(spy.operationEnds[0].1.error, "First operation should succeed")
        XCTAssertNil(spy.operationEnds[1].1.error, "Cached operation should succeed, not throw 304 error")

        // The second request-level hook should report fromCache
        let cachedRequestEnd = spy.requestEnds.last!
        XCTAssertTrue(cachedRequestEnd.1.fromCache, "Request result should indicate cache hit")
    }
}

/// A spy hooks implementation that records all callbacks.
private final class SpyHooks: BasecampHooks, @unchecked Sendable {
    private let lock = NSLock()
    private var _operationStarts: [OperationInfo] = []
    private var _operationEnds: [(OperationInfo, OperationResult)] = []
    private var _requestStarts: [RequestInfo] = []
    private var _requestEnds: [(RequestInfo, RequestResult)] = []

    var operationStarts: [OperationInfo] {
        lock.withLock { _operationStarts }
    }

    var operationEnds: [(OperationInfo, OperationResult)] {
        lock.withLock { _operationEnds }
    }

    var requestStarts: [RequestInfo] {
        lock.withLock { _requestStarts }
    }

    var requestEnds: [(RequestInfo, RequestResult)] {
        lock.withLock { _requestEnds }
    }

    func onOperationStart(_ info: OperationInfo) {
        lock.withLock { _operationStarts.append(info) }
    }

    func onOperationEnd(_ info: OperationInfo, result: OperationResult) {
        lock.withLock { _operationEnds.append((info, result)) }
    }

    func onRequestStart(_ info: RequestInfo) {
        lock.withLock { _requestStarts.append(info) }
    }

    func onRequestEnd(_ info: RequestInfo, result: RequestResult) {
        lock.withLock { _requestEnds.append((info, result)) }
    }
}

/// Thread-safe counter for use in @Sendable closures.
private final class RequestCounter: Sendable {
    private let lock = NSLock()
    nonisolated(unsafe) private var _count = 0

    @discardableResult
    func increment() -> Int {
        lock.withLock {
            _count += 1
            return _count
        }
    }
}
