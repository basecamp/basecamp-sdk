import XCTest
@testable import Basecamp

/// A request that fails in transport must not put its URL's query into any
/// rendering of the error (SPEC §9). `URLError` carries the failing URL in its
/// `userInfo`, and `String(describing:)` on `.network(message:cause:)` prints
/// it — so a signed query on the request reached the error's description,
/// `debugDescription`, `NSError` bridge, cause chain and the retry hook.
final class TransportErrorProjectionTests: XCTestCase {
    private static let secret = "SECRETVALUE"
    private static let signedURL = "http://127.0.0.1:1/blob?signature=\(secret)"

    private final class RetrySpy: BasecampHooks, @unchecked Sendable {
        private let lock = NSLock()
        private var _errors: [any Error] = []
        var errors: [any Error] { lock.withLock { _errors } }

        func onOperationStart(_ info: OperationInfo) {}
        func onOperationEnd(_ info: OperationInfo, result: OperationResult) {}
        func onRequestStart(_ info: RequestInfo) {}
        func onRequestEnd(_ info: RequestInfo, result: RequestResult) {}
        func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval) {
            lock.withLock { _errors.append(error) }
        }
    }

    /// Every way a caller or a logger can render an error, walking the
    /// `.network` cause chain and the `NSError` underlying chain.
    private static func renderings(of error: any Error, label: String = "error") -> [(String, String)] {
        let nsError = error as NSError
        var out: [(String, String)] = [
            ("\(label) describing", String(describing: error)),
            ("\(label) reflecting", String(reflecting: error)),
            ("\(label) localizedDescription", error.localizedDescription),
            ("\(label) NSError.description", nsError.description),
            ("\(label) NSError.userInfo", String(describing: nsError.userInfo)),
        ]
        if let basecampError = error as? BasecampError, case .network(_, let cause) = basecampError, let cause {
            out += renderings(of: cause, label: "\(label).cause")
        }
        if let underlying = nsError.userInfo[NSUnderlyingErrorKey] as? NSError {
            out += renderings(of: underlying, label: "\(label).underlying")
        }
        return out
    }

    private static func assertNoSecret(_ error: any Error, _ context: String, file: StaticString = #filePath, line: UInt = #line) {
        for (label, text) in renderings(of: error) {
            XCTAssertFalse(text.contains(secret), "\(context): the signed query leaked into \(label): \(text)", file: file, line: line)
        }
    }

    // Dials a closed port through the shipped URLSession transport, with a
    // two-attempt policy so the retry hook sees the failure too.
    func testTransportFailureRendersNoSignedQuery() async throws {
        let spy = RetrySpy()
        let client = BasecampClient(
            auth: BearerAuth(tokenProvider: StaticTokenProvider("token")),
            userAgent: "test/1.0",
            config: BasecampConfig(baseURL: "http://127.0.0.1:1", enableRetry: true),
            hooks: spy
        )
        let policy = RetryConfig(maxAttempts: 2, baseDelayMs: 1, backoff: .constant, retryOn: [])

        do {
            _ = try await client.httpClient.performRequest(method: "GET", url: Self.signedURL, retryConfig: policy)
            XCTFail("expected the dial to a closed port to fail")
        } catch {
            guard let basecampError = error as? BasecampError, case .network(let message, let cause) = basecampError else {
                return XCTFail("expected .network, got \(error)")
            }
            XCTAssertEqual(message, "Network error")
            let urlError = try XCTUnwrap(cause as? URLError, "the cause still classifies as a URLError")
            XCTAssertEqual(urlError.code, .cannotConnectToHost)
            XCTAssertEqual(urlError.failingURL?.absoluteString, "http://127.0.0.1:1/blob", "origin and path survive the projection")
            XCTAssertTrue(String(describing: basecampError).contains("http://127.0.0.1:1/blob"))
            Self.assertNoSecret(basecampError, "thrown error")
        }

        XCTAssertEqual(spy.errors.count, 1, "the failed first attempt announces one retry")
        for error in spy.errors {
            XCTAssertEqual((error as? URLError)?.code, .cannotConnectToHost, "onRetry receives the projected transport error")
            Self.assertNoSecret(error, "onRetry error")
        }
    }

    // A Transport that speaks .network (#567) wraps its own URLSession error;
    // the projection reaches through it and keeps the transport's message.
    func testTransportSpokenNetworkErrorIsProjectedThroughItsCause() throws {
        let raw = URLError(.timedOut, userInfo: [
            NSURLErrorFailingURLErrorKey: URL(string: Self.signedURL)!,
            NSURLErrorFailingURLStringErrorKey: Self.signedURL,
        ])
        let projected = HTTPClient.projectedTransportError(BasecampError.network(message: "custom transport", cause: raw))

        guard let basecampError = projected as? BasecampError, case .network(let message, let cause) = basecampError else {
            return XCTFail("expected .network, got \(projected)")
        }
        XCTAssertEqual(message, "custom transport")
        let urlError = try XCTUnwrap(cause as? URLError)
        XCTAssertEqual(urlError.code, .timedOut)
        XCTAssertEqual(urlError.failingURL?.absoluteString, "http://127.0.0.1:1/blob")
        Self.assertNoSecret(projected, "projected transport error")
    }

    // URLSession reports a cancelled task as URLError(.cancelled) carrying the
    // failing URL; the projection keeps the meaning isCancellation reads.
    func testCancelledURLErrorKeepsItsCodeAndDropsTheQuery() {
        let raw = URLError(.cancelled, userInfo: [NSURLErrorFailingURLStringErrorKey: Self.signedURL])
        let projected = HTTPClient.projectedTransportError(raw)
        XCTAssertEqual((projected as? URLError)?.code, .cancelled)
        Self.assertNoSecret(projected, "projected cancellation")
    }

    // An error that is not a URLError is the transport's own and passes through untouched.
    func testForeignTransportErrorPassesThrough() {
        struct ForeignTransportError: Error, Equatable { let id: Int }
        let raw = ForeignTransportError(id: 7)
        XCTAssertEqual(HTTPClient.projectedTransportError(raw) as? ForeignTransportError, raw)
    }
}
