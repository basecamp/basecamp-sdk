import XCTest
@testable import Basecamp

final class ChainHooksTests: XCTestCase {

    func testStartEventsCalledInOrder() {
        let recorder = OrderRecorder()
        let hookA = RecordingHooks(id: "A", recorder: recorder)
        let hookB = RecordingHooks(id: "B", recorder: recorder)
        let chain = ChainHooks(hookA, hookB)

        let info = RequestInfo(method: "GET", url: "https://example.com", attempt: 1)
        chain.onRequestStart(info)

        XCTAssertEqual(recorder.events, ["A:requestStart", "B:requestStart"])
    }

    func testEndEventsCalledInReverseOrder() {
        let recorder = OrderRecorder()
        let hookA = RecordingHooks(id: "A", recorder: recorder)
        let hookB = RecordingHooks(id: "B", recorder: recorder)
        let chain = ChainHooks(hookA, hookB)

        let info = RequestInfo(method: "GET", url: "https://example.com", attempt: 1)
        let result = RequestResult(statusCode: 200, durationMs: 10)
        chain.onRequestEnd(info, result: result)

        XCTAssertEqual(recorder.events, ["B:requestEnd", "A:requestEnd"])
    }

    func testOperationStartInOrderEndReversed() {
        let recorder = OrderRecorder()
        let hookA = RecordingHooks(id: "A", recorder: recorder)
        let hookB = RecordingHooks(id: "B", recorder: recorder)
        let hookC = RecordingHooks(id: "C", recorder: recorder)
        let chain = ChainHooks(hookA, hookB, hookC)

        let info = OperationInfo(
            service: "Test", operation: "Get",
            resourceType: "test", isMutation: false
        )

        chain.onOperationStart(info)
        chain.onOperationEnd(info, result: OperationResult(durationMs: 5))

        XCTAssertEqual(recorder.events, [
            "A:operationStart", "B:operationStart", "C:operationStart",
            "C:operationEnd", "B:operationEnd", "A:operationEnd",
        ])
    }

    func testRetryCalledInOrder() {
        let recorder = OrderRecorder()
        let hookA = RecordingHooks(id: "A", recorder: recorder)
        let hookB = RecordingHooks(id: "B", recorder: recorder)
        let chain = ChainHooks(hookA, hookB)

        let info = RequestInfo(method: "GET", url: "https://example.com", attempt: 1)
        let error = BasecampError.network(message: "timeout", cause: nil)
        chain.onRetry(info, attempt: 1, error: error, delaySeconds: 1.0)

        XCTAssertEqual(recorder.events, ["A:retry", "B:retry"])
    }

    func testEmptyChainDoesNotCrash() {
        let chain = ChainHooks([])

        let info = RequestInfo(method: "GET", url: "https://example.com", attempt: 1)
        chain.onRequestStart(info)
        chain.onRequestEnd(info, result: RequestResult(statusCode: 200, durationMs: 0))
    }

    func testArrayInitializer() {
        let recorder = OrderRecorder()
        let hooks: [any BasecampHooks] = [
            RecordingHooks(id: "X", recorder: recorder),
            RecordingHooks(id: "Y", recorder: recorder),
        ]
        let chain = ChainHooks(hooks)

        let info = RequestInfo(method: "GET", url: "https://example.com", attempt: 1)
        chain.onRequestStart(info)

        XCTAssertEqual(recorder.events, ["X:requestStart", "Y:requestStart"])
    }
}

// MARK: - Test helpers

private final class OrderRecorder: @unchecked Sendable {
    private let lock = NSLock()
    private var _events: [String] = []

    var events: [String] {
        lock.withLock { _events }
    }

    func record(_ event: String) {
        lock.withLock { _events.append(event) }
    }
}

private struct RecordingHooks: BasecampHooks {
    let id: String
    let recorder: OrderRecorder

    func onOperationStart(_ info: OperationInfo) {
        recorder.record("\(id):operationStart")
    }

    func onOperationEnd(_ info: OperationInfo, result: OperationResult) {
        recorder.record("\(id):operationEnd")
    }

    func onRequestStart(_ info: RequestInfo) {
        recorder.record("\(id):requestStart")
    }

    func onRequestEnd(_ info: RequestInfo, result: RequestResult) {
        recorder.record("\(id):requestEnd")
    }

    func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval) {
        recorder.record("\(id):retry")
    }
}
