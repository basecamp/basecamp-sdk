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
