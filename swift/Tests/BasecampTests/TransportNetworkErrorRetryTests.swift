import XCTest
@testable import Basecamp

/// Thread-safe counter for use in `@Sendable` closures.
private final class AttemptCounter: @unchecked Sendable {
    private let lock = NSLock()
    private var _value = 0

    var value: Int { lock.withLock { _value } }

    @discardableResult
    func increment() -> Int {
        lock.withLock {
            _value += 1
            return _value
        }
    }
}

/// A transport that reports connectivity failure as the SDK's own error type.
///
/// This is the natural implementation, not a contrived one: `Transport` is
/// `public`, and an app wiring its existing networking stack into the seam
/// reaches for `BasecampError.network` because that is exactly the
/// classification §6 defines for the condition — `retryable: true`.
private final class BasecampErrorTransport: Transport, @unchecked Sendable {
    private let counter: AttemptCounter
    private let failuresBeforeSuccess: Int
    private let error: BasecampError

    init(counter: AttemptCounter, failuresBeforeSuccess: Int, error: BasecampError) {
        self.counter = counter
        self.failuresBeforeSuccess = failuresBeforeSuccess
        self.error = error
    }

    private func respond(to request: URLRequest) throws -> (Data, URLResponse) {
        if counter.increment() <= failuresBeforeSuccess {
            throw error
        }
        let response = HTTPURLResponse(
            url: request.url!, statusCode: 200,
            httpVersion: "HTTP/1.1", headerFields: [:]
        )!
        return (Data("{}".utf8), response)
    }

    func data(for request: URLRequest) async throws -> (Data, URLResponse) {
        try respond(to: request)
    }

    func dataNoRedirect(for request: URLRequest) async throws -> (Data, URLResponse) {
        try respond(to: request)
    }
}

/// A transport whose failures are its own error type — the shape that already
/// retried, kept here as the control the `BasecampError.network` cases are
/// compared against.
private struct ForeignTransportError: Error {}

/// #567: a `Transport` that throws `BasecampError.network` must be retried.
///
/// Both retry loops classified **every** `BasecampError` out of the transport as
/// terminal, so a transport that normalized connectivity failures into the SDK's
/// own retryable classification got exactly one attempt with retries enabled,
/// while a transport throwing an arbitrary foreign error retried normally. The
/// behavior was inverted from what an implementer would predict, and the more
/// carefully the implementer read §6, the more likely they were to hit it.
///
/// The catch could not simply be deleted: it also swallowed the response-type
/// guard, which throws `BasecampError.network("Invalid response type")` for a
/// non-HTTP `URLResponse`. That is a deterministic programming error — retrying
/// it three times just repeats it — so it now raises a distinct internal error
/// and is converted back at the boundary, keeping the public error contract.
final class TransportNetworkErrorRetryTests: XCTestCase {

    private var retryConfig: RetryConfig {
        RetryConfig(maxAttempts: 3, baseDelayMs: 1, backoff: .constant, retryOn: [429, 503])
    }

    // MARK: - performRequest

    func testTransportBasecampNetworkErrorIsRetried() async throws {
        let counter = AttemptCounter()
        let transport = BasecampErrorTransport(
            counter: counter, failuresBeforeSuccess: 1,
            error: .network(message: "Connection reset", cause: nil))

        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("test-token"),
            userAgent: "test-suite",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com", enableRetry: true),
            transport: transport
        )

        let (_, response) = try await client.forAccount("999999999").httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: retryConfig
        )

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 2, "a transport-thrown .network must reach the retry branch")
    }

    /// On exhaustion the transport's own error surfaces rather than being
    /// re-wrapped in a second, less informative `.network`.
    func testTransportBasecampNetworkErrorSurfacesUnwrappedOnExhaustion() async throws {
        let counter = AttemptCounter()
        let transport = BasecampErrorTransport(
            counter: counter, failuresBeforeSuccess: .max,
            error: .network(message: "Connection reset", cause: nil))

        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("test-token"),
            userAgent: "test-suite",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com", enableRetry: true),
            transport: transport
        )

        do {
            _ = try await client.forAccount("999999999").httpClient.performRequest(
                method: "GET",
                url: "https://3.basecampapi.com/999999999/projects.json",
                retryConfig: retryConfig
            )
            XCTFail("Expected the transport's error")
        } catch let error as BasecampError {
            XCTAssertEqual(error.message, "Connection reset")
        }

        XCTAssertEqual(counter.value, 3, "should have spent the whole retry budget")
    }

    /// The gate is idempotency, not error type: SPEC §7's network-error retry
    /// runs behind the same three-gate check as the status retry, so a
    /// non-idempotent POST is still attempted exactly once.
    func testNonIdempotentPostIsNotRetriedOnBasecampNetworkError() async {
        let counter = AttemptCounter()
        let transport = BasecampErrorTransport(
            counter: counter, failuresBeforeSuccess: .max,
            error: .network(message: "Connection reset", cause: nil))

        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("test-token"),
            userAgent: "test-suite",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com", enableRetry: true),
            transport: transport
        )

        _ = try? await client.forAccount("999999999").httpClient.performRequest(
            method: "POST",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: retryConfig,
            idempotent: false
        )

        XCTAssertEqual(counter.value, 1, "a non-idempotent POST is attempted exactly once")
    }

    /// Only `.network` moves. Every other `BasecampError` out of the transport
    /// is the transport's own final verdict and stays terminal on sight.
    func testNonNetworkBasecampErrorsStayTerminal() async {
        for error in [
            BasecampError.auth(message: "nope", hint: nil, requestId: nil),
            BasecampError.usage(message: "bad config", hint: nil),
            BasecampError.api(message: "boom", httpStatus: 500, hint: nil, requestId: nil),
        ] {
            let counter = AttemptCounter()
            let transport = BasecampErrorTransport(
                counter: counter, failuresBeforeSuccess: .max, error: error)

            let client = BasecampClient(
                tokenProvider: StaticTokenProvider("test-token"),
                userAgent: "test-suite",
                config: BasecampConfig(baseURL: "https://3.basecampapi.com", enableRetry: true),
                transport: transport
            )

            _ = try? await client.forAccount("999999999").httpClient.performRequest(
                method: "GET",
                url: "https://3.basecampapi.com/999999999/projects.json",
                retryConfig: retryConfig
            )

            XCTAssertEqual(counter.value, 1, "\(error) must not consume retry budget")
        }
    }

    // MARK: - Download hop 1

    /// Both loops move together. Fixing only one would leave two classifications
    /// of the same error inside one SDK, which is worse than the gap.
    func testDownloadHopRetriesTransportBasecampNetworkError() async throws {
        let counter = AttemptCounter()
        let transport = BasecampErrorTransport(
            counter: counter, failuresBeforeSuccess: 1,
            error: .network(message: "Connection reset", cause: nil))

        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("test-token"),
            userAgent: "test-suite",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com", enableRetry: true),
            transport: transport
        )

        let (_, response) = try await client.httpClient.performDownloadRequest(
            url: "https://3.basecampapi.com/999999999/attachment.json")

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 2, "the download hop must retry a transport-thrown .network")
    }

    // MARK: - The response-type guard

    /// A `Transport` that returns a non-HTTP `URLResponse` is a deterministic
    /// programming error, not a connectivity blip: it must fail on the first
    /// attempt, and it must still surface as the same `BasecampError.network`
    /// with the same message it always did.
    func testInvalidResponseTypeIsNotRetriedAndKeepsItsPublicShape() async {
        let counter = AttemptCounter()
        let transport = NonHTTPResponseTransport(counter: counter)

        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("test-token"),
            userAgent: "test-suite",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com", enableRetry: true),
            transport: transport
        )

        do {
            _ = try await client.forAccount("999999999").httpClient.performRequest(
                method: "GET",
                url: "https://3.basecampapi.com/999999999/projects.json",
                retryConfig: retryConfig
            )
            XCTFail("Expected the response-type guard to fire")
        } catch let error as BasecampError {
            guard case .network(let message, _) = error else {
                return XCTFail("Expected .network, got \(error)")
            }
            XCTAssertEqual(message, "Invalid response type")
        } catch {
            XCTFail("Expected a BasecampError, got \(error)")
        }

        XCTAssertEqual(counter.value, 1, "a deterministic guard must not consume retry budget")
    }

    func testInvalidResponseTypeOnDownloadHopIsNotRetried() async {
        let counter = AttemptCounter()
        let transport = NonHTTPResponseTransport(counter: counter)

        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("test-token"),
            userAgent: "test-suite",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com", enableRetry: true),
            transport: transport
        )

        do {
            _ = try await client.httpClient.performDownloadRequest(
                url: "https://3.basecampapi.com/999999999/attachment.json")
            XCTFail("Expected the response-type guard to fire")
        } catch let error as BasecampError {
            guard case .network(let message, _) = error else {
                return XCTFail("Expected .network, got \(error)")
            }
            XCTAssertEqual(message, "Invalid response type")
        } catch {
            XCTFail("Expected a BasecampError, got \(error)")
        }

        XCTAssertEqual(counter.value, 1, "a deterministic guard must not consume retry budget")
    }

    // MARK: - Control

    /// The pre-existing contract: a transport throwing its own error type
    /// retries. Pinned so the fix is shown to have converged the two shapes
    /// rather than swapped which one is broken.
    func testForeignTransportErrorStillRetries() async throws {
        let counter = AttemptCounter()
        let transport = MockTransport { request in
            if counter.increment() == 1 {
                throw ForeignTransportError()
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("{}".utf8), response)
        }

        let client = makeTestClient(transport: transport, enableRetry: true)

        let (_, response) = try await client.forAccount("999999999").httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: retryConfig
        )

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 2)
    }
}

/// A transport that hands back a bare `URLResponse`, tripping the SDK's
/// response-type guard.
private final class NonHTTPResponseTransport: Transport, @unchecked Sendable {
    private let counter: AttemptCounter

    init(counter: AttemptCounter) {
        self.counter = counter
    }

    private func respond(to request: URLRequest) -> (Data, URLResponse) {
        counter.increment()
        return (Data(), URLResponse(
            url: request.url!, mimeType: nil,
            expectedContentLength: 0, textEncodingName: nil))
    }

    func data(for request: URLRequest) async throws -> (Data, URLResponse) {
        respond(to: request)
    }

    func dataNoRedirect(for request: URLRequest) async throws -> (Data, URLResponse) {
        respond(to: request)
    }
}
