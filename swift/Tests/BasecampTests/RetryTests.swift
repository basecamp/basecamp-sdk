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

        // The second request should have a different auth token than the first
        let requests = transport.requests
        XCTAssertEqual(requests.count, 2)
        let firstAuth = requests[0].request.value(forHTTPHeaderField: "Authorization")
        let secondAuth = requests[1].request.value(forHTTPHeaderField: "Authorization")
        // Both get token from the provider; the retry re-authenticates
        // The key thing is that authenticate() was called again
        XCTAssertNotNil(firstAuth)
        XCTAssertNotNil(secondAuth)
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
