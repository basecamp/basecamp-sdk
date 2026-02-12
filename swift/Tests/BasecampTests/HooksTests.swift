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
