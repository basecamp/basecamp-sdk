import XCTest
@testable import Basecamp

/// Thread-safe counter for use in @Sendable closures.
private final class Counter: @unchecked Sendable {
    private let lock = NSLock()
    private var _value: Int = 0

    var value: Int {
        lock.withLock { _value }
    }

    @discardableResult
    func increment() -> Int {
        lock.withLock {
            _value += 1
            return _value
        }
    }
}

final class RetryTests: XCTestCase {

    func testNoRetryWhenDisabled() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 429,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data(), response)
        }

        let client = makeTestClient(transport: transport, enableRetry: false)
        let account = client.forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json"
        )

        XCTAssertEqual(response.statusCode, 429)
        XCTAssertEqual(counter.value, 1, "Should not retry when retry is disabled")
    }

    func testRetryOn429() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            let statusCode = count < 3 ? 429 : 200
            let headers: [String: String] = count < 3 ? ["Retry-After": "0"] : [:]
            let response = HTTPURLResponse(
                url: request.url!, statusCode: statusCode,
                httpVersion: "HTTP/1.1", headerFields: headers
            )!
            let data = count >= 3 ? Data("{}".utf8) : Data()
            return (data, response)
        }

        let client = makeTestClient(transport: transport, enableRetry: true)
        let account = client.forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: RetryConfig(
                maxAttempts: 3, baseDelayMs: 1,
                backoff: .constant, retryOn: [429, 503]
            )
        )

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 3, "Should retry twice then succeed on third attempt")
    }

    func testRetryOn503() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            let statusCode = count < 2 ? 503 : 200
            let response = HTTPURLResponse(
                url: request.url!, statusCode: statusCode,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("{}".utf8), response)
        }

        let client = makeTestClient(transport: transport, enableRetry: true)
        let account = client.forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: RetryConfig(
                maxAttempts: 3, baseDelayMs: 1,
                backoff: .constant, retryOn: [429, 503]
            )
        )

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 2)
    }

    func testDoesNotRetryOnNonRetryableStatus() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 404,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data(), response)
        }

        let client = makeTestClient(transport: transport, enableRetry: true)
        let account = client.forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json"
        )

        XCTAssertEqual(response.statusCode, 404)
        XCTAssertEqual(counter.value, 1, "Should not retry on 404")
    }

    // MARK: - maxAttempts: 0 guard (Bug Fix)

    func testMaxAttemptsZeroDoesNotCrash() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("{}".utf8), response)
        }

        let client = makeTestClient(transport: transport, enableRetry: true)
        let account = client.forAccount("999999999")

        // maxAttempts: 0 should not crash — should execute at least 1 attempt
        let (_, response) = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: RetryConfig(
                maxAttempts: 0, baseDelayMs: 1,
                backoff: .constant, retryOn: [429, 503]
            )
        )

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 1, "maxAttempts: 0 should still make one request")
    }

    // MARK: - Auth Re-authentication on Retry

    func testAuthReauthenticatesOnRetry() async throws {
        let requestCounter = Counter()

        let tokenProvider = RotatingTokenProvider(tokens: ["token-1", "token-2", "token-3"])

        let transport = MockTransport { request in
            let count = requestCounter.increment()
            let statusCode = count < 2 ? 429 : 200
            let headers: [String: String] = count < 2 ? ["Retry-After": "0"] : [:]
            let response = HTTPURLResponse(
                url: request.url!, statusCode: statusCode,
                httpVersion: "HTTP/1.1", headerFields: headers
            )!
            return (Data("{}".utf8), response)
        }

        let client = BasecampClient(
            tokenProvider: tokenProvider,
            userAgent: "test-suite",
            config: BasecampConfig(
                baseURL: "https://3.basecampapi.com",
                enableRetry: true,
                enableCache: false
            ),
            transport: transport
        )
        let account = client.forAccount("999999999")

        _ = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: RetryConfig(
                maxAttempts: 3, baseDelayMs: 1,
                backoff: .constant, retryOn: [429, 503]
            )
        )

        // The second request should have a different auth token than the first:
        // the retry re-authenticates and the provider rotates (SPEC §7 step 3.l).
        let requests = transport.requests
        XCTAssertEqual(requests.count, 2)
        let firstAuth = requests[0].request.value(forHTTPHeaderField: "Authorization")
        let secondAuth = requests[1].request.value(forHTTPHeaderField: "Authorization")
        XCTAssertEqual(firstAuth, "Bearer token-1")
        XCTAssertEqual(secondAuth, "Bearer token-2", "The retry after a retryable status must re-authenticate")
    }

    /// Network-path twin of `testAuthReauthenticatesOnRetry`: a retry after a
    /// transport-level failure must also refresh auth before the next attempt
    /// (SPEC §7 step 3.l applies to both retry paths).
    func testAuthReauthenticatesOnRetryAfterNetworkError() async throws {
        let requestCounter = Counter()

        let tokenProvider = RotatingTokenProvider(tokens: ["token-1", "token-2", "token-3"])

        let transport = MockTransport { request in
            let count = requestCounter.increment()
            if count == 1 {
                throw URLError(.networkConnectionLost)
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("{}".utf8), response)
        }

        let client = BasecampClient(
            tokenProvider: tokenProvider,
            userAgent: "test-suite",
            config: BasecampConfig(
                baseURL: "https://3.basecampapi.com",
                enableRetry: true,
                enableCache: false
            ),
            transport: transport
        )
        let account = client.forAccount("999999999")

        _ = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: RetryConfig(
                maxAttempts: 3, baseDelayMs: 1,
                backoff: .constant, retryOn: [429, 503]
            )
        )

        let requests = transport.requests
        XCTAssertEqual(requests.count, 2)
        let firstAuth = requests[0].request.value(forHTTPHeaderField: "Authorization")
        let secondAuth = requests[1].request.value(forHTTPHeaderField: "Authorization")
        XCTAssertEqual(firstAuth, "Bearer token-1")
        XCTAssertEqual(secondAuth, "Bearer token-2", "The retry after a network error must re-authenticate")
    }

    // MARK: - Retry-After Header

    func testRetryAfterHeaderParsedAndUsed() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 429,
                    httpVersion: "HTTP/1.1", headerFields: ["Retry-After": "1"]
                )!
                return (Data(), response)
            } else {
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 200,
                    httpVersion: "HTTP/1.1", headerFields: [:]
                )!
                return (Data("{}".utf8), response)
            }
        }

        let client = makeTestClient(transport: transport, enableRetry: true)
        let account = client.forAccount("999999999")

        let start = CFAbsoluteTimeGetCurrent()
        let (_, response) = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: RetryConfig(
                maxAttempts: 2, baseDelayMs: 10_000,
                backoff: .constant, retryOn: [429]
            )
        )
        let elapsed = CFAbsoluteTimeGetCurrent() - start

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 2)
        // Retry-After: 1 should override the 10s baseDelay
        // Allow some margin for jitter and scheduling
        XCTAssertLessThan(elapsed, 5.0, "Retry-After header should override base delay")
    }

    // MARK: - Network Error Triggers Retry

    func testNetworkErrorTriggersRetry() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                throw URLError(.networkConnectionLost)
            }
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("{}".utf8), response)
        }

        let client = makeTestClient(transport: transport, enableRetry: true)
        let account = client.forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: RetryConfig(
                maxAttempts: 3, baseDelayMs: 1,
                backoff: .constant, retryOn: [429, 503]
            )
        )

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 2, "Should retry after network error")
    }

    func testNetworkErrorExhaustsRetries() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            throw URLError(.timedOut)
        }

        let client = makeTestClient(transport: transport, enableRetry: true)
        let account = client.forAccount("999999999")

        do {
            _ = try await account.httpClient.performRequest(
                method: "GET",
                url: "https://3.basecampapi.com/999999999/projects.json",
                retryConfig: RetryConfig(
                    maxAttempts: 2, baseDelayMs: 1,
                    backoff: .constant, retryOn: [429, 503]
                )
            )
            XCTFail("Expected network error")
        } catch let error as BasecampError {
            if case .network = error {
                // Expected
            } else {
                XCTFail("Expected .network error, got \(error)")
            }
        }

        XCTAssertEqual(counter.value, 2, "Should exhaust all retry attempts")
    }

    // MARK: - Retry Lifecycle (#509): backoff/reauth run outside the attempt's do/catch

    /// A failing re-authentication before a retry must propagate its error raw:
    /// no phantom onRequestEnd(statusCode: 0) for an attempt that already ended,
    /// no second onRetry, and no further attempt with stale headers.
    func testAuthFailureOnRetryPropagatesRaw() async throws {
        let spy = RetrySpyHooks()
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            let statusCode = count == 1 ? 429 : 200
            let response = HTTPURLResponse(
                url: request.url!, statusCode: statusCode,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("{}".utf8), response)
        }

        let client = BasecampClient(
            auth: FailingSecondAuth(),
            userAgent: "test-suite",
            config: BasecampConfig(
                baseURL: "https://3.basecampapi.com",
                enableRetry: true,
                enableCache: false
            ),
            hooks: spy,
            transport: transport
        )
        let account = client.forAccount("999999999")

        do {
            _ = try await account.httpClient.performRequest(
                method: "GET",
                url: "https://3.basecampapi.com/999999999/projects.json",
                retryConfig: RetryConfig(
                    maxAttempts: 3, baseDelayMs: 1,
                    backoff: .constant, retryOn: [429, 503]
                )
            )
            XCTFail("Expected RefreshError")
        } catch is RefreshError {
            // Expected: the authenticate error propagates untouched.
        } catch {
            XCTFail("Expected RefreshError, got \(error)")
        }

        XCTAssertEqual(spy.requestEnds.count, 1, "A failed reauth must not emit a phantom onRequestEnd")
        XCTAssertEqual(spy.retries.count, 1, "A failed reauth must not emit a second onRetry")
    }

    /// Cancelling the task while it sleeps between attempts must propagate
    /// CancellationError raw: no phantom onRequestEnd(statusCode: 0) and no
    /// second onRetry for the attempt that already ended.
    func testCancellationDuringBackoffPropagatesRaw() async throws {
        let spy = RetrySpyHooks()
        let transport = MockTransport { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 503,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data(), response)
        }

        let client = makeTestClient(transport: transport, enableRetry: true, hooks: spy)
        let httpClient = client.forAccount("999999999").httpClient

        let task = Task {
            _ = try await httpClient.performRequest(
                method: "GET",
                url: "https://3.basecampapi.com/999999999/projects.json",
                retryConfig: RetryConfig(
                    maxAttempts: 3, baseDelayMs: 5_000,
                    backoff: .constant, retryOn: [429, 503]
                )
            )
        }

        // Wait until the first attempt's 503 has been served — the client is
        // then headed into (or already inside) its ~5s backoff sleep — and cancel.
        while transport.requests.isEmpty {
            try await Task.sleep(nanoseconds: 1_000_000)
        }
        task.cancel()

        do {
            _ = try await task.value
            XCTFail("Expected CancellationError")
        } catch is CancellationError {
            // Expected: cooperative cancellation propagates raw.
        } catch {
            XCTFail("Expected CancellationError, got \(error)")
        }

        XCTAssertEqual(spy.requestEnds.count, 1, "A cancelled backoff must not emit a phantom onRequestEnd")
        XCTAssertEqual(spy.retries.count, 1, "A cancelled backoff must not emit a second onRetry")
    }

    // MARK: - Idempotency Gate (Layer A: transport)

    /// A retryable config used by the idempotency-gate tests. Fast + deterministic.
    private static let gateConfig = RetryConfig(
        maxAttempts: 3, baseDelayMs: 1, backoff: .constant, retryOn: [429, 503]
    )

    func testNonIdempotentPostNotRetriedOn503() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            return (Data(), HTTPURLResponse(
                url: request.url!, statusCode: 503,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "POST",
            url: "https://3.basecampapi.com/999999999/projects.json",
            body: Data("{}".utf8),
            retryConfig: Self.gateConfig,
            idempotent: false
        )

        XCTAssertEqual(response.statusCode, 503)
        XCTAssertEqual(counter.value, 1, "Non-idempotent POST must not retry on 503")
    }

    func testNonIdempotentPostNotRetriedOn429() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            return (Data(), HTTPURLResponse(
                url: request.url!, statusCode: 429,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "POST",
            url: "https://3.basecampapi.com/999999999/projects.json",
            body: Data("{}".utf8),
            retryConfig: Self.gateConfig,
            idempotent: false
        )

        XCTAssertEqual(response.statusCode, 429)
        XCTAssertEqual(counter.value, 1, "Non-idempotent POST must not retry on 429")
    }

    func testIdempotentPostRetriedOn503() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            let statusCode = count < 2 ? 503 : 200
            return (Data("{}".utf8), HTTPURLResponse(
                url: request.url!, statusCode: statusCode,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "POST",
            url: "https://3.basecampapi.com/999999999/buckets/1/todos/1/completion.json",
            body: Data("{}".utf8),
            retryConfig: Self.gateConfig,
            idempotent: true
        )

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 2, "Idempotent POST must keep retrying on 503")
    }

    func testNonIdempotentPostNotRetriedOnNetworkError() async throws {
        let counter = Counter()
        let transport = MockTransport { _ in
            counter.increment()
            throw URLError(.networkConnectionLost)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        do {
            _ = try await account.httpClient.performRequest(
                method: "POST",
                url: "https://3.basecampapi.com/999999999/projects.json",
                body: Data("{}".utf8),
                retryConfig: Self.gateConfig,
                idempotent: false
            )
            XCTFail("Expected network error")
        } catch let error as BasecampError {
            guard case .network = error else {
                return XCTFail("Expected .network error, got \(error)")
            }
        }

        XCTAssertEqual(counter.value, 1, "Non-idempotent POST must not retry on network error")
    }

    func testUnknownMethodPatchNotRetried() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            return (Data(), HTTPURLResponse(
                url: request.url!, statusCode: 503,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        // PATCH is not in the naturally-idempotent allowlist and the operation
        // is not flagged idempotent, so the method gate must be fail-closed.
        let (_, response) = try await account.httpClient.performRequest(
            method: "PATCH",
            url: "https://3.basecampapi.com/999999999/projects.json",
            body: Data("{}".utf8),
            retryConfig: Self.gateConfig,
            idempotent: false
        )

        XCTAssertEqual(response.statusCode, 503)
        XCTAssertEqual(counter.value, 1, "PATCH must not retry — method gate is fail-closed")
    }

    /// Regression: naturally-idempotent GET still retries regardless of the flag.
    func testGetRetriesOn503RegardlessOfIdempotentFlag() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            let statusCode = count < 2 ? 503 : 200
            return (Data("{}".utf8), HTTPURLResponse(
                url: request.url!, statusCode: statusCode,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        let (_, response) = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json",
            retryConfig: Self.gateConfig,
            idempotent: false
        )

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertEqual(counter.value, 2, "GET retries via the method gate; idempotent flag is irrelevant")
    }

    // MARK: - Idempotency Gate (Layer B: BaseService → Metadata → transport)

    /// Non-idempotent POST driven through a generated service must make a single
    /// attempt — proving `Metadata.isIdempotent` reaches the transport as `false`.
    func testGeneratedNonIdempotentPostNotRetried() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            return (Data(), HTTPURLResponse(
                url: request.url!, statusCode: 503,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        do {
            _ = try await account.projects.create(req: CreateProjectRequest(name: "Test"))
            XCTFail("Expected 503 error")
        } catch let error as BasecampError {
            guard case .api = error else {
                return XCTFail("Expected .api error, got \(error)")
            }
        }

        XCTAssertEqual(transport.requests.count, 1, "CreateProject (non-idempotent POST) must not retry")
    }

    /// Idempotent POST driven through its generated service must retry —
    /// proving `Metadata.isIdempotent` wires `true` through `BaseService`.
    /// Must be an idempotent POST (CompleteTodo), not a PUT: a PUT would only
    /// exercise the method gate and would not prove the metadata lookup.
    func testGeneratedIdempotentPostRetries() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            let statusCode = count < 2 ? 503 : 204
            return (Data(), HTTPURLResponse(
                url: request.url!, statusCode: statusCode,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        try await account.todos.complete(todoId: 1)

        XCTAssertEqual(transport.requests.count, 2, "CompleteTodo (idempotent POST) must retry through BaseService")
    }

    /// Network-error analog of `testGeneratedIdempotentPostRetries`: an
    /// idempotent POST driven through its generated service retries a genuine
    /// transport failure and then succeeds. The existing
    /// `testNetworkErrorTriggersRetry` is GET-only, so it never exercises the
    /// POST metadata gate on the network path. Swift is not in the conformance
    /// matrix, so this unit test is the only coverage of Swift's network
    /// liveness for a metadata-flagged idempotent POST. Refs #439 / #417.
    func testGeneratedIdempotentPostRetriesOnNetworkError() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                throw URLError(.networkConnectionLost)
            }
            return (Data(), HTTPURLResponse(
                url: request.url!, statusCode: 204,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!)
        }

        let account = makeTestClient(transport: transport, enableRetry: true).forAccount("999999999")

        try await account.todos.complete(todoId: 1)

        XCTAssertEqual(transport.requests.count, 2, "CompleteTodo (idempotent POST) must retry a network error through BaseService")
    }

    // MARK: - Auth Header

    func testAuthHeaderIncludesToken() async throws {
        let transport = MockTransport { request in
            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("{}".utf8), response)
        }

        let client = makeTestClient(transport: transport)
        let account = client.forAccount("999999999")

        _ = try await account.httpClient.performRequest(
            method: "GET",
            url: "https://3.basecampapi.com/999999999/projects.json"
        )

        let lastRequest = transport.lastRequest!.request
        XCTAssertEqual(lastRequest.value(forHTTPHeaderField: "Authorization"), "Bearer test-token")
        XCTAssertEqual(lastRequest.value(forHTTPHeaderField: "User-Agent"), "test-suite")
        XCTAssertEqual(lastRequest.value(forHTTPHeaderField: "Accept"), "application/json")
    }
}

// MARK: - Test Helpers

/// Error thrown by `FailingSecondAuth` to simulate a token-refresh failure.
private struct RefreshError: Error {}

/// An auth strategy whose second `authenticate` call throws `RefreshError`,
/// simulating a token refresh that fails between retry attempts.
private final class FailingSecondAuth: AuthStrategy, @unchecked Sendable {
    private let lock = NSLock()
    private var calls = 0

    func authenticate(_ request: inout URLRequest) async throws {
        let call: Int = lock.withLock {
            calls += 1
            return calls
        }
        if call >= 2 {
            throw RefreshError()
        }
        request.setValue("Bearer token-\(call)", forHTTPHeaderField: "Authorization")
    }
}

/// File-local spy recording request-level hook callbacks for the retry
/// lifecycle tests. (HooksTests' SpyHooks is private to that file.)
private final class RetrySpyHooks: BasecampHooks, @unchecked Sendable {
    struct RetryRecord {
        let info: RequestInfo
        let attempt: Int
        let error: any Error
        let delaySeconds: TimeInterval
    }

    private let lock = NSLock()
    private var _requestStarts: [RequestInfo] = []
    private var _requestEnds: [(RequestInfo, RequestResult)] = []
    private var _retries: [RetryRecord] = []

    var requestStarts: [RequestInfo] {
        lock.withLock { _requestStarts }
    }

    var requestEnds: [(RequestInfo, RequestResult)] {
        lock.withLock { _requestEnds }
    }

    var retries: [RetryRecord] {
        lock.withLock { _retries }
    }

    func onRequestStart(_ info: RequestInfo) {
        lock.withLock { _requestStarts.append(info) }
    }

    func onRequestEnd(_ info: RequestInfo, result: RequestResult) {
        lock.withLock { _requestEnds.append((info, result)) }
    }

    func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval) {
        lock.withLock { _retries.append(RetryRecord(info: info, attempt: attempt, error: error, delaySeconds: delaySeconds)) }
    }
}

/// A token provider that returns tokens from a list, cycling through them.
private final class RotatingTokenProvider: TokenProvider, @unchecked Sendable {
    private let lock = NSLock()
    private let tokens: [String]
    private var index = 0

    init(tokens: [String]) {
        self.tokens = tokens
    }

    func accessToken() async throws -> String {
        lock.withLock {
            let token = tokens[index % tokens.count]
            index += 1
            return token
        }
    }
}
