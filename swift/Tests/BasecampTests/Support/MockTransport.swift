import Foundation
@testable import Basecamp

/// A mock transport for testing that records requests and returns predefined responses.
///
/// Matches the iOS team's `NothingTransporter`/spy pattern from core-ios-networking.
///
/// ```swift
/// let transport = MockTransport { request in
///     (try! JSONEncoder().encode(["id": 1, "title": "Test"]), HTTPURLResponse(...))
/// }
/// let client = BasecampClient(
///     accessToken: "test-token",
///     userAgent: "test",
///     transport: transport
/// )
/// ```
final class MockTransport: Transport, @unchecked Sendable {
    struct RecordedRequest: Sendable {
        let request: URLRequest
        /// Which `Transport` entry point carried it: false for
        /// `dataNoRedirect(for:)`, the one both download hops must use. The
        /// mock cannot follow a redirect itself, so this is how a test pins
        /// that production will not either.
        let followsRedirects: Bool
    }

    private let handler: @Sendable (URLRequest) async throws -> (Data, URLResponse)
    private let lock = NSLock()
    private var _requests: [RecordedRequest] = []

    /// All requests that have been made through this transport.
    var requests: [RecordedRequest] {
        lock.withLock { _requests }
    }

    /// The most recent request.
    var lastRequest: RecordedRequest? {
        lock.withLock { _requests.last }
    }

    /// Creates a mock transport with a handler that processes each request.
    init(handler: @escaping @Sendable (URLRequest) async throws -> (Data, URLResponse)) {
        self.handler = handler
    }

    /// Creates a mock transport that returns the same response for all requests.
    convenience init(
        statusCode: Int = 200,
        data: Data = Data(),
        headers: [String: String] = [:]
    ) {
        self.init { request in
            let url = request.url ?? URL(string: "https://3.basecampapi.com")!
            let response = HTTPURLResponse(
                url: url, statusCode: statusCode,
                httpVersion: "HTTP/1.1", headerFields: headers
            )!
            return (data, response)
        }
    }

    func data(for request: URLRequest) async throws -> (Data, URLResponse) {
        try await serve(request, followsRedirects: true)
    }

    func dataNoRedirect(for request: URLRequest) async throws -> (Data, URLResponse) {
        try await serve(request, followsRedirects: false)
    }

    private func serve(_ request: URLRequest, followsRedirects: Bool) async throws -> (Data, URLResponse) {
        lock.withLock {
            _requests.append(RecordedRequest(request: request, followsRedirects: followsRedirects))
        }
        return try await handler(request)
    }

    /// Resets the recorded requests.
    func reset() {
        lock.withLock {
            _requests.removeAll()
        }
    }
}
