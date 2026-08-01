import Basecamp
import Foundation

/// One outbound request captured by the scripted transport.
struct CapturedRequest: @unchecked Sendable {
    /// The full URLRequest as handed to the transport (headers, body, method).
    let request: URLRequest
    /// Monotonic capture time in milliseconds (DispatchTime, immune to
    /// wall-clock adjustments), for delayBetweenRequests assertions.
    let monotonicMs: UInt64

    var method: String { request.httpMethod?.uppercased() ?? "" }
    var path: String { request.url?.path ?? "" }
    var bodyJSON: JSON? { request.httpBody.flatMap { JSON.parse($0) } }

    /// Case-insensitive request-header lookup.
    func header(_ name: String) -> String? {
        request.value(forHTTPHeaderField: name)
    }
}

/// Transport that answers each request from the fixture's scripted response
/// queue, in order, recording every request it sees.
///
/// This is the public `Transport` seam — no `@testable` anywhere. Both
/// `data(for:)` and `dataNoRedirect(for:)` draw from the same queue, so
/// multi-hop flows (downloads) consume entries in wire order exactly like the
/// Kotlin MockEngine port model.
final class ScriptedTransport: Transport, @unchecked Sendable {
    private let lock = NSLock()
    private let responses: [MockResponse]
    /// When the fixture advertises Link rel="next" headers the SDK will
    /// auto-paginate past the scripted queue; answer the overflow with an
    /// empty terminal page instead of an error, mirroring the Kotlin runner.
    private let autoPaginates: Bool
    private var _captured: [CapturedRequest] = []
    private var served = 0

    init(responses: [MockResponse], autoPaginates: Bool) {
        self.responses = responses
        self.autoPaginates = autoPaginates
    }

    var captured: [CapturedRequest] {
        lock.withLock { _captured }
    }

    var requestCount: Int {
        lock.withLock { _captured.count }
    }

    /// The fixture index of the last consumed queue entry, or nil when no
    /// entry (or only synthetic overflow pages) served the final request.
    var lastConsumedIndex: Int? {
        lock.withLock {
            let last = served - 1
            return (last >= 0 && last < responses.count) ? last : nil
        }
    }

    func data(for request: URLRequest) async throws -> (Data, URLResponse) {
        try await serve(request)
    }

    func dataNoRedirect(for request: URLRequest) async throws -> (Data, URLResponse) {
        try await serve(request)
    }

    private func serve(_ request: URLRequest) async throws -> (Data, URLResponse) {
        let index: Int? = lock.withLock {
            _captured.append(CapturedRequest(
                request: request,
                monotonicMs: DispatchTime.now().uptimeNanoseconds / 1_000_000
            ))
            let i = served
            served += 1
            return i < responses.count ? i : nil
        }

        let url = request.url ?? URL(string: "http://localhost:3000/")!

        guard let index else {
            if autoPaginates {
                // Terminal empty page: no Link header ends pagination cleanly.
                return (
                    Data("[]".utf8),
                    HTTPURLResponse(
                        url: url, statusCode: 200, httpVersion: "HTTP/1.1",
                        headerFields: ["Content-Type": "application/json"])!
                )
            }
            return (
                Data(#"{"error": "No more mock responses"}"#.utf8),
                HTTPURLResponse(
                    url: url, statusCode: 500, httpVersion: "HTTP/1.1",
                    headerFields: ["Content-Type": "application/json"])!
            )
        }

        let mock = responses[index]

        if mock.delayMs > 0 {
            try await Task.sleep(nanoseconds: UInt64(mock.delayMs) * 1_000_000)
        }

        // Genuine transport failure for this queued entry: throw a plain
        // URLError (NOT a BasecampError — the SDK rethrows those untouched,
        // which would bypass the network-retry path under test).
        if mock.isNetworkError {
            throw URLError(.networkConnectionLost)
        }

        var headerFields = ["Content-Type": "application/json"]
        for (key, value) in mock.allHeaders {
            headerFields[key] = value
        }

        let body: Data
        if let fixtureBody = mock.body {
            body = try Self.normalize(fixtureBody).serialized()
        } else {
            body = Data()
        }

        // The fixture schema guarantees a status on every non-networkError
        // entry; the runner backstop re-checks before dispatch.
        let response = HTTPURLResponse(
            url: url, statusCode: mock.status ?? 200,
            httpVersion: "HTTP/1.1", headerFields: headerFields)!
        return (body, response)
    }

    /// Unwraps `{"key": [...]}` single-key array wrappers: some fixtures wrap
    /// list bodies in an object, but the SDK's list operations decode a raw
    /// JSON array (same normalization as the Kotlin runner).
    private static func normalize(_ body: JSON) -> JSON {
        if let object = body.objectValue, object.count == 1,
           let sole = object.values.first, sole.arrayValue != nil {
            return sole
        }
        return body
    }
}
