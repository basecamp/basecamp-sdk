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

final class DownloadTests: XCTestCase {

    // MARK: - filenameFromURL

    func testFilenameFromURL_simple() {
        XCTAssertEqual("report.pdf", filenameFromURL("https://example.com/files/report.pdf"))
    }

    func testFilenameFromURL_encoded() {
        XCTAssertEqual("my report.pdf", filenameFromURL("https://example.com/files/my%20report.pdf"))
    }

    func testFilenameFromURL_trailingSlash() {
        XCTAssertEqual("download", filenameFromURL("https://example.com/files/"))
    }

    func testFilenameFromURL_noPath() {
        XCTAssertEqual("download", filenameFromURL("https://example.com"))
    }

    func testFilenameFromURL_empty() {
        XCTAssertEqual("download", filenameFromURL(""))
    }

    func testFilenameFromURL_deepPath() {
        XCTAssertEqual("notes.txt", filenameFromURL("https://example.com/a/b/c/notes.txt"))
    }

    func testFilenameFromURL_withQuery() {
        XCTAssertEqual("image.png", filenameFromURL("https://example.com/image.png?size=large"))
    }

    func testFilenameFromURL_rootPath() {
        XCTAssertEqual("download", filenameFromURL("https://example.com/"))
    }

    // MARK: - Validation

    func testDownloadURL_emptyThrowsUsage() async throws {
        let transport = MockTransport(statusCode: 200)
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL("")
            XCTFail("Expected usage error")
        } catch let error as BasecampError {
            guard case .usage = error else {
                XCTFail("Expected usage error, got \(error)")
                return
            }
        }
    }

    func testDownloadURL_relativeThrowsUsage() async throws {
        let transport = MockTransport(statusCode: 200)
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL("/just/a/path")
            XCTFail("Expected usage error")
        } catch let error as BasecampError {
            guard case .usage = error else {
                XCTFail("Expected usage error, got \(error)")
                return
            }
        }
    }

    // MARK: - URL Rewriting

    func testDownloadURL_rewritesOrigin() async throws {
        let transport = MockTransport { request in
            XCTAssertEqual(request.url?.host, "3.basecampapi.com")
            XCTAssertEqual(request.url?.path, "/999999999/attachments/abc/download/report.pdf")
            return (
                Data("file-content".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/pdf", "Content-Length": "12"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport)
        let result = try await account.downloadURL("https://other-host.example.com/999999999/attachments/abc/download/report.pdf")
        XCTAssertEqual(String(data: result.body, encoding: .utf8), "file-content")
        XCTAssertEqual(result.contentType, "application/pdf")
    }

    func testDownloadURL_preservesQueryParams() async throws {
        let transport = MockTransport { request in
            let url = request.url!
            XCTAssertTrue(url.query?.contains("token=abc") == true)
            XCTAssertTrue(url.query?.contains("v=2") == true)
            return (
                Data("data".utf8),
                makeHTTPResponse(url: url.absoluteString, statusCode: 200, headers: ["Content-Type": "application/octet-stream"])
            )
        }
        let account = makeTestAccountClient(transport: transport)
        let result = try await account.downloadURL("https://any-host.com/999999999/download?token=abc&v=2")
        XCTAssertEqual(String(data: result.body, encoding: .utf8), "data")
    }

    // MARK: - Redirect Flow

    func testDownloadURL_redirectFlow() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                // Hop 1: API redirect
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "https://s3.amazonaws.com/bucket/signed-file?sig=xyz"]
                    )
                )
            } else {
                // Hop 2: Signed download
                return (
                    Data("pdf-content".utf8),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 200,
                        headers: ["Content-Type": "application/pdf", "Content-Length": "11"]
                    )
                )
            }
        }
        let account = makeTestAccountClient(transport: transport)
        let result = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/report.pdf")
        XCTAssertEqual(String(data: result.body, encoding: .utf8), "pdf-content")
        XCTAssertEqual(result.contentType, "application/pdf")
        XCTAssertEqual(result.contentLength, 11)
        XCTAssertEqual(result.filename, "report.pdf")
    }

    func testDownloadURL_directDownload() async throws {
        let transport = MockTransport { request in
            (
                Data("direct-content".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "text/plain", "Content-Length": "14"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport)
        let result = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
        XCTAssertEqual(String(data: result.body, encoding: .utf8), "direct-content")
        XCTAssertEqual(result.contentType, "text/plain")
        XCTAssertEqual(result.contentLength, 14)
        XCTAssertEqual(result.filename, "file.txt")
    }

    func testDownloadURL_relativeLocation() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "/signed/file.txt"]
                    )
                )
            } else {
                // Relative location should be resolved against the API URL
                XCTAssertTrue(request.url!.absoluteString.contains("/signed/file.txt"))
                return (
                    Data("data".utf8),
                    makeHTTPResponse(url: request.url!.absoluteString, statusCode: 200, headers: ["Content-Type": "text/plain"])
                )
            }
        }
        let account = makeTestAccountClient(transport: transport)
        let result = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
        XCTAssertEqual(String(data: result.body, encoding: .utf8), "data")
    }

    func testDownloadURL_redirectNoLocation() async throws {
        let transport = MockTransport { request in
            (
                Data(),
                makeHTTPResponse(url: request.url!.absoluteString, statusCode: 302, headers: [:])
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .api = error else {
                XCTFail("Expected api error, got \(error)")
                return
            }
        }
    }

    // MARK: - Error Tests

    func testDownloadURL_api404() async throws {
        let transport = MockTransport { request in
            (
                Data(#"{"error":"Not found"}"#.utf8),
                makeHTTPResponse(url: request.url!.absoluteString, statusCode: 404, headers: ["Content-Type": "application/json"])
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/missing/download/file.txt")
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .notFound = error else {
                XCTFail("Expected notFound, got \(error)")
                return
            }
        }
    }

    func testDownloadURL_api403() async throws {
        let transport = MockTransport { request in
            (
                Data(#"{"error":"Forbidden"}"#.utf8),
                makeHTTPResponse(url: request.url!.absoluteString, statusCode: 403, headers: ["Content-Type": "application/json"])
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/secret/download/file.txt")
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .forbidden = error else {
                XCTFail("Expected forbidden, got \(error)")
                return
            }
        }
    }

    func testDownloadURL_s3Error() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "https://s3.amazonaws.com/bucket/file"]
                    )
                )
            } else {
                return (
                    Data("AccessDenied".utf8),
                    makeHTTPResponse(url: request.url!.absoluteString, statusCode: 403)
                )
            }
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .api = error else {
                XCTFail("Expected api error, got \(error)")
                return
            }
        }
    }

    // SPEC §14 "Hop-2 Redirect Policy": the signed URL is the one destination
    // the API host named. A redirect from it surfaces with its status, and the
    // Location it names is never dialled (#805). The mock cannot follow a
    // redirect itself, so the last assertion pins the seam that guarantees
    // that in production: both hops go through the transport's no-redirect
    // entry point, where `data(for:)` would have followed the chain.
    func testDownloadURL_hop2RedirectIsRefusedNotFollowed() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            let location = count == 1
                ? "https://s3.amazonaws.com/bucket/file"
                : "https://elsewhere.example.com/final/file"
            return (
                Data(),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 302,
                    headers: ["Location": location]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
            XCTFail("Expected the signed hop's redirect to be refused")
        } catch let error as BasecampError {
            guard case .api(let message, let httpStatus, _, _, _) = error else {
                XCTFail("Expected api error, got \(error)")
                return
            }
            XCTAssertEqual(httpStatus, 302)
            XCTAssertTrue(message.contains("not followed"), "expected the message to name the refusal, got \(message)")
        }

        XCTAssertEqual(transport.requests.count, 2, "the redirect target must never be dialled")
        XCTAssertEqual(transport.requests.map(\.followsRedirects), [false, false])
    }

    // MARK: - Auth Header Tests

    func testDownloadURL_authOnApiNotOnS3() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                // API leg should have auth
                XCTAssertNotNil(request.value(forHTTPHeaderField: "Authorization"))
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "https://s3.amazonaws.com/bucket/file"]
                    )
                )
            } else {
                // S3 leg should NOT have auth
                XCTAssertNil(request.value(forHTTPHeaderField: "Authorization"))
                return (
                    Data("data".utf8),
                    makeHTTPResponse(url: request.url!.absoluteString, statusCode: 200, headers: ["Content-Type": "application/octet-stream"])
                )
            }
        }
        let account = makeTestAccountClient(transport: transport)
        _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
    }

    // MARK: - Hook Tests

    func testDownloadURL_operationHooks() async throws {
        final class TestHooks: BasecampHooks, @unchecked Sendable {
            var opsStarted: [OperationInfo] = []
            var opsEnded: [(OperationInfo, OperationResult)] = []

            func onOperationStart(_ info: OperationInfo) { opsStarted.append(info) }
            func onOperationEnd(_ info: OperationInfo, result: OperationResult) { opsEnded.append((info, result)) }
        }

        let hooks = TestHooks()
        let transport = MockTransport { request in
            (
                Data("data".utf8),
                makeHTTPResponse(url: request.url!.absoluteString, statusCode: 200, headers: ["Content-Type": "text/plain"])
            )
        }
        let account = makeTestAccountClient(transport: transport, hooks: hooks)
        _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")

        XCTAssertEqual(hooks.opsStarted.count, 1)
        XCTAssertEqual(hooks.opsStarted[0].service, "Account")
        XCTAssertEqual(hooks.opsStarted[0].operation, "DownloadURL")

        XCTAssertEqual(hooks.opsEnded.count, 1)
        XCTAssertNil(hooks.opsEnded[0].1.error)
    }

    func testDownloadURL_requestHooksApiOnly() async throws {
        final class TestHooks: BasecampHooks, @unchecked Sendable {
            var reqStarted: [RequestInfo] = []
            var reqEnded: [(RequestInfo, RequestResult)] = []

            func onRequestStart(_ info: RequestInfo) { reqStarted.append(info) }
            func onRequestEnd(_ info: RequestInfo, result: RequestResult) { reqEnded.append((info, result)) }
        }

        let hooks = TestHooks()
        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "https://s3.amazonaws.com/bucket/file"]
                    )
                )
            } else {
                return (
                    Data("data".utf8),
                    makeHTTPResponse(url: request.url!.absoluteString, statusCode: 200, headers: ["Content-Type": "text/plain"])
                )
            }
        }
        let account = makeTestAccountClient(transport: transport, hooks: hooks)
        _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")

        // Request hooks fire for hop 1 only
        XCTAssertEqual(hooks.reqStarted.count, 1)
        XCTAssertEqual(hooks.reqEnded.count, 1)
        XCTAssertEqual(hooks.reqStarted[0].method, "GET")
    }

    // MARK: - Network Failure Tests

    func testDownloadURL_hop1NetworkFailure() async throws {
        struct TestError: Error {}

        final class TestHooks: BasecampHooks, @unchecked Sendable {
            var reqEnded: [(RequestInfo, RequestResult)] = []

            func onRequestEnd(_ info: RequestInfo, result: RequestResult) { reqEnded.append((info, result)) }
        }

        let hooks = TestHooks()
        let transport = MockTransport { _ in
            throw TestError()
        }
        let account = makeTestAccountClient(transport: transport, hooks: hooks)

        do {
            _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
            XCTFail("Expected network error")
        } catch let error as BasecampError {
            guard case .network = error else {
                XCTFail("Expected network error, got \(error)")
                return
            }
        }

        // on_request_end fires with statusCode 0
        XCTAssertEqual(hooks.reqEnded.count, 1)
        XCTAssertEqual(hooks.reqEnded[0].1.statusCode, 0)
    }

    func testDownloadURL_hop2NetworkFailure() async throws {
        struct TestError: Error {}

        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            if count == 1 {
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "https://s3.amazonaws.com/bucket/file"]
                    )
                )
            } else {
                throw TestError()
            }
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
            XCTFail("Expected network error")
        } catch let error as BasecampError {
            guard case .network = error else {
                XCTFail("Expected network error, got \(error)")
                return
            }
        }
    }

    // MARK: - SPEC §9: Credential-Bearing Values Are Never Rendered

    /// An error whose rendering carries the URL it failed on — the shape a raw
    /// `URLError` takes — so a test can assert the SDK never chains it.
    private struct URLLeakingError: Error, CustomStringConvertible {
        let url: String
        var description: String { "dial \(url) refused" }
    }

    /// Hop-1 exhaustion wraps the transport error with a fixed message and a
    /// severed cause: the transport's rendering carries the hop-1 URL, and the
    /// caller's URL can smuggle a signed query into it through the rewrite.
    func testDownloadURL_hop1NetworkErrorSeversCause() async throws {
        let transport = MockTransport { request in
            throw URLLeakingError(url: request.url!.absoluteString)
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL(
                "https://3.basecampapi.com/999999999/attachments/abc/download/file.txt?verifier=SECRET")
            XCTFail("Expected network error")
        } catch let error as BasecampError {
            guard case .network(let message, let cause) = error else {
                XCTFail("Expected .network, got \(error)")
                return
            }
            XCTAssertEqual(message, "Network error")
            XCTAssertNil(cause)
        }
    }

    /// A retried hop-1 transport failure reaches onRequestEnd/onRetry as the
    /// same projection the caller would get, never the raw URL-bearing error.
    func testDownloadURL_retriedHop1NetworkErrorReachesHooksProjected() async throws {
        final class ErrorSpy: BasecampHooks, @unchecked Sendable {
            private let lock = NSLock()
            private var _retryErrors: [any Error] = []
            var retryErrors: [any Error] { lock.withLock { _retryErrors } }
            func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval) {
                lock.withLock { _retryErrors.append(error) }
            }
        }

        let spy = ErrorSpy()
        let counter = Counter()
        let transport = MockTransport { request in
            if counter.increment() == 1 {
                throw URLLeakingError(url: request.url!.absoluteString)
            }
            return (
                Data("data".utf8),
                makeHTTPResponse(url: request.url!.absoluteString, statusCode: 200, headers: ["Content-Type": "text/plain"])
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true, hooks: spy)
        _ = try await account.downloadURL(
            "https://3.basecampapi.com/999999999/attachments/abc/download/file.txt?verifier=SECRET")

        XCTAssertEqual(spy.retryErrors.count, 1)
        guard let projected = spy.retryErrors.first as? BasecampError,
              case .network(let message, let cause) = projected else {
            XCTFail("Expected a projected .network in onRetry, got \(String(describing: spy.retryErrors.first))")
            return
        }
        XCTAssertEqual(message, "Network error")
        XCTAssertNil(cause)
    }

    /// A transport that speaks `.network` itself is projected like a raw
    /// failure on the download hops: its message can render the URL, so the
    /// #567 message passthrough does not apply here.
    func testDownloadURL_transportNetworkErrorMessageIsProjected() async throws {
        let transport = MockTransport { request in
            throw BasecampError.network(
                message: "dial \(request.url!.absoluteString) refused",
                cause: URLLeakingError(url: request.url!.absoluteString))
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL(
                "https://3.basecampapi.com/999999999/attachments/abc/download/file.txt?verifier=SECRET")
            XCTFail("Expected network error")
        } catch let error as BasecampError {
            guard case .network(let message, let cause) = error else {
                XCTFail("Expected .network, got \(error)")
                return
            }
            XCTAssertEqual(message, "Network error")
            XCTAssertNil(cause)
        }
    }

    /// The hop-2 wrap severs the cause the same way: the signed URL is a
    /// credential, and the transport error renders it.
    func testDownloadURL_hop2NetworkErrorSeversCause() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            if counter.increment() == 1 {
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "https://s3.amazonaws.com/bucket/file?X-Amz-Signature=SECRET"]
                    )
                )
            }
            throw URLLeakingError(url: request.url!.absoluteString)
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL(Self.hop1URL)
            XCTFail("Expected network error")
        } catch let error as BasecampError {
            guard case .network(let message, let cause) = error else {
                XCTFail("Expected .network, got \(error)")
                return
            }
            XCTAssertEqual(message, "Download failed")
            XCTAssertNil(cause)
        }
    }

    /// A cancelled hop 2 keeps its meaning through the projection: the chained
    /// cause is a FRESH cancellation sentinel — the shape `isCancellation`
    /// walks — carrying no rendering of the signed URL.
    func testDownloadURL_hop2CancellationProjectsFreshSentinel() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            if counter.increment() == 1 {
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "https://s3.amazonaws.com/bucket/file?X-Amz-Signature=SECRET"]
                    )
                )
            }
            throw URLError(.cancelled, userInfo: [
                NSURLErrorFailingURLStringErrorKey: request.url!.absoluteString
            ])
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL(Self.hop1URL)
            XCTFail("Expected network error")
        } catch let error as BasecampError {
            guard case .network(let message, let cause) = error else {
                XCTFail("Expected .network, got \(error)")
                return
            }
            XCTAssertEqual(message, "Download failed")
            let urlError = try XCTUnwrap(cause as? URLError)
            XCTAssertEqual(urlError.code, .cancelled)
            XCTAssertFalse(String(describing: cause).contains("SECRET"),
                           "the projected sentinel must not carry the original's URL rendering")
        }
    }

    /// Hop-1 hooks see origin+path only — no query, no fragment — while the
    /// wire request keeps the query.
    func testDownloadURL_hookURLsOmitQueryWhileWireKeepsIt() async throws {
        final class TestHooks: BasecampHooks, @unchecked Sendable {
            var urls: [String] = []
            func onRequestStart(_ info: RequestInfo) { urls.append(info.url) }
            func onRequestEnd(_ info: RequestInfo, result: RequestResult) { urls.append(info.url) }
        }

        let hooks = TestHooks()
        let transport = MockTransport { request in
            (
                Data("data".utf8),
                makeHTTPResponse(url: request.url!.absoluteString, statusCode: 200, headers: ["Content-Type": "text/plain"])
            )
        }
        let account = makeTestAccountClient(transport: transport, hooks: hooks)
        _ = try await account.downloadURL(
            "https://3.basecampapi.com/999999999/attachments/abc/download/file.txt?verifier=SECRET#frag")

        XCTAssertEqual(transport.requests[0].request.url?.query, "verifier=SECRET")
        let projected = "https://3.basecampapi.com/999999999/attachments/abc/download/file.txt"
        XCTAssertEqual(hooks.urls, [projected, projected])
    }

    /// A signed Location that fails URL construction renders its SPEC §9
    /// origin projection — the fixed token when nothing parses — never the
    /// input itself.
    func testDownloadURL_unparsableSignedLocationRendersNoURL() async throws {
        let transport = MockTransport { request in
            (
                Data(),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 302,
                    headers: ["Location": "https://[invalid/file?sig=SECRET"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.downloadURL(Self.hop1URL)
            XCTFail("Expected usage error")
        } catch let error as BasecampError {
            guard case .usage(let message, _) = error else {
                XCTFail("Expected .usage, got \(error)")
                return
            }
            XCTAssertEqual(message, "Invalid URL: unparsable")
        }
    }

    // MARK: - Hop-1 Retry Policy (SPEC §14)

    private static let hop1URL = "https://3.basecampapi.com/999999999/attachments/abc/download/file.txt"

    /// Records the request lifecycle so the hop-1 loop's hook emission can be
    /// asserted attempt by attempt.
    private final class RetryHookSpy: BasecampHooks, @unchecked Sendable {
        private let lock = NSLock()
        private var _starts: [Int] = []
        private var _ends: [Int] = []
        private var _retries: [Int] = []

        var starts: [Int] { lock.withLock { _starts } }
        var ends: [Int] { lock.withLock { _ends } }
        var retries: [Int] { lock.withLock { _retries } }

        func onRequestStart(_ info: RequestInfo) {
            lock.withLock { _starts.append(info.attempt) }
        }

        func onRequestEnd(_ info: RequestInfo, result: RequestResult) {
            lock.withLock { _ends.append(info.attempt) }
        }

        func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval) {
            lock.withLock { _retries.append(attempt) }
        }
    }

    /// The COMPLETE declared retry set, pinned status by status (the shared
    /// conformance fixtures only cover 429 and 503): hop 1 retries
    /// {429, 502, 503, 504} and then follows the redirect to the signed hop.
    func testDownloadURL_retriesDeclaredStatusesThenFollowsRedirect() async throws {
        for status in [429, 502, 503, 504] {
            let hop1Attempts = Counter()
            let hop2Requests = Counter()
            let transport = MockTransport { request in
                if request.url!.path.hasPrefix("/signed/") {
                    hop2Requests.increment()
                    return (
                        Data("data".utf8),
                        makeHTTPResponse(
                            url: request.url!.absoluteString,
                            statusCode: 200,
                            headers: ["Content-Type": "application/octet-stream"]
                        )
                    )
                }
                if hop1Attempts.increment() == 1 {
                    return (
                        Data("{}".utf8),
                        makeHTTPResponse(
                            url: request.url!.absoluteString,
                            statusCode: status,
                            headers: ["Content-Type": "application/json"]
                        )
                    )
                }
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "/signed/file.txt"]
                    )
                )
            }
            let account = makeTestAccountClient(transport: transport, enableRetry: true)

            let result = try await account.downloadURL(Self.hop1URL)

            XCTAssertEqual(String(data: result.body, encoding: .utf8), "data", "status \(status)")
            XCTAssertEqual(hop1Attempts.value, 2, "status \(status)")
            XCTAssertEqual(hop2Requests.value, 1, "status \(status)")
        }
    }

    /// 500 is deliberately outside the declared set: the download hop keeps the
    /// main GET loop's declared-set discipline rather than the error taxonomy's
    /// broader "all 5xx retryable" flag.
    func testDownloadURL_neverRetries500() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            return (
                Data(#"{"error":"Internal server error"}"#.utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 500,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true)

        do {
            _ = try await account.downloadURL(Self.hop1URL)
            XCTFail("Expected api error")
        } catch is BasecampError {
            // Expected.
        }

        XCTAssertEqual(counter.value, 1)
    }

    func testDownloadURL_retriesNetworkErrorThenSucceeds() async throws {
        struct TestError: Error {}

        let counter = Counter()
        let transport = MockTransport { request in
            if counter.increment() == 1 {
                throw TestError()
            }
            return (
                Data("content".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "text/plain"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true)

        let result = try await account.downloadURL(Self.hop1URL)

        XCTAssertEqual(String(data: result.body, encoding: .utf8), "content")
        XCTAssertEqual(counter.value, 2)
    }

    /// The enabled policy is a fixed three attempts — there is no public
    /// numeric knob for the download hop.
    func testDownloadURL_exhaustsThreeAttemptsThenSurfaces() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            return (
                Data("{}".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 503,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true)

        do {
            _ = try await account.downloadURL(Self.hop1URL)
            XCTFail("Expected error after the attempt budget is spent")
        } catch is BasecampError {
            // Expected.
        }

        XCTAssertEqual(counter.value, 3)
    }

    func testDownloadURL_enableRetryFalseSendsExactlyOneAttempt() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            return (
                Data("{}".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 503,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: false)

        do {
            _ = try await account.downloadURL(Self.hop1URL)
            XCTFail("Expected error")
        } catch is BasecampError {
            // Expected.
        }

        XCTAssertEqual(counter.value, 1)
    }

    /// Every hop-1 attempt is authenticated — including the retried one — and
    /// the signed hop never is.
    func testDownloadURL_authOnEveryHop1AttemptNeverOnHop2() async throws {
        let hop1Auth = AuthRecorder()
        let hop2Auth = AuthRecorder()
        let hop1Attempts = Counter()
        let transport = MockTransport { request in
            if request.url!.path.hasPrefix("/signed/") {
                hop2Auth.record(request.value(forHTTPHeaderField: "Authorization"))
                return (
                    Data("data".utf8),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 200,
                        headers: ["Content-Type": "application/octet-stream"]
                    )
                )
            }
            hop1Auth.record(request.value(forHTTPHeaderField: "Authorization"))
            if hop1Attempts.increment() == 1 {
                return (
                    Data("{}".utf8),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 503,
                        headers: ["Content-Type": "application/json"]
                    )
                )
            }
            return (
                Data(),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 302,
                    headers: ["Location": "/signed/file.txt"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true)

        _ = try await account.downloadURL(Self.hop1URL)

        XCTAssertEqual(hop1Auth.values, ["Bearer test-token", "Bearer test-token"])
        XCTAssertEqual(hop2Auth.values.count, 1)
        XCTAssertNil(hop2Auth.values[0])
    }

    func testDownloadURL_balancedHooksAcrossRetries() async throws {
        let spy = RetryHookSpy()
        let counter = Counter()
        let transport = MockTransport { request in
            if counter.increment() < 3 {
                return (
                    Data("{}".utf8),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 503,
                        headers: ["Content-Type": "application/json"]
                    )
                )
            }
            return (
                Data("content".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "text/plain"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true, hooks: spy)

        _ = try await account.downloadURL(Self.hop1URL)

        XCTAssertEqual(spy.starts, [1, 2, 3])
        XCTAssertEqual(spy.ends, [1, 2, 3])
        // onRetry names the UPCOMING attempt (SPEC §7 attempt semantics).
        XCTAssertEqual(spy.retries, [2, 3])
    }

    func testDownloadURL_honorsRetryAfterOn429() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            if counter.increment() == 1 {
                return (
                    Data("{}".utf8),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 429,
                        headers: ["Content-Type": "application/json", "Retry-After": "2"]
                    )
                )
            }
            return (
                Data("content".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "text/plain"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true)

        let start = CFAbsoluteTimeGetCurrent()
        _ = try await account.downloadURL(Self.hop1URL)
        let elapsed = CFAbsoluteTimeGetCurrent() - start

        XCTAssertEqual(counter.value, 2)
        // Retry-After: 2 wins over the 1-second exponential base.
        XCTAssertGreaterThanOrEqual(elapsed, 2.0, "Expected the Retry-After pause, got \(elapsed)s")
    }

    /// Cancelling during the hop-1 backoff propagates CancellationError raw:
    /// no phantom onRequestEnd for the attempt that already ended, no second
    /// onRetry, and no further attempt.
    func testDownloadURL_cancellationDuringBackoffPropagatesRaw() async throws {
        let spy = RetryHookSpy()
        let transport = MockTransport { request in
            (
                Data("{}".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 503,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true, hooks: spy)

        let url = Self.hop1URL
        let task = Task { _ = try await account.downloadURL(url) }

        // Bounded poll: if the first attempt never fires, fail the test rather
        // than spinning here forever.
        var waited = 0
        while transport.requests.isEmpty {
            guard waited < 5_000 else {
                task.cancel()
                XCTFail("First hop-1 attempt never reached the transport")
                return
            }
            try await Task.sleep(nanoseconds: 1_000_000)
            waited += 1
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

        XCTAssertEqual(transport.requests.count, 1, "A cancelled backoff must not start another attempt")
        XCTAssertEqual(spy.ends, [1], "A cancelled backoff must not emit a phantom onRequestEnd")
        XCTAssertEqual(spy.retries, [2], "A cancelled backoff must not emit a second onRetry")
    }

    /// Cancellation raised by the transport itself — mid-flight, before any
    /// response — is terminal too. Classifying it as a transport failure would
    /// announce a retry for an attempt that can never start.
    func testDownloadURL_cancellationInFlightIsTerminalAndSilent() async throws {
        let spy = RetryHookSpy()
        let counter = Counter()
        let transport = MockTransport { _ in
            counter.increment()
            throw CancellationError()
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true, hooks: spy)

        do {
            _ = try await account.downloadURL(Self.hop1URL)
            XCTFail("Expected CancellationError")
        } catch is CancellationError {
            // Expected.
        } catch {
            XCTFail("Expected CancellationError, got \(error)")
        }

        XCTAssertEqual(counter.value, 1, "Cancellation must not spend another attempt")
        XCTAssertEqual(spy.retries, [], "Cancellation must not announce a retry")
        // Hook balance is the actual contract, not merely "no crash": the
        // cancelled attempt is finalized exactly once, so an observer pairing
        // start/end spans never leaks one.
        XCTAssertEqual(spy.starts, [1])
        XCTAssertEqual(spy.ends, [1])
    }

    /// `URLSession` reports a cancelled task as `URLError(.cancelled)`, not
    /// `CancellationError` — so the real-world cancellation shape must be
    /// terminal too, not just the Swift-concurrency one.
    func testDownloadURL_urlErrorCancelledIsTerminal() async throws {
        let spy = RetryHookSpy()
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            throw URLError(.cancelled, userInfo: [NSURLErrorFailingURLStringErrorKey: request.url!.absoluteString])
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true, hooks: spy)

        do {
            _ = try await account.downloadURL(Self.hop1URL)
            XCTFail("Expected the cancellation to surface")
        } catch let error as URLError {
            // Terminal errors propagate unwrapped, so the cancellation shape
            // itself must arrive — not a BasecampError.network wrapping it —
            // as a fresh instance: URLSession's carries the failing URL in
            // its userInfo, which SPEC §9 keeps out of the download path.
            XCTAssertEqual(error.code, .cancelled)
            XCTAssertNil(error.userInfo[NSURLErrorFailingURLStringErrorKey])
        } catch {
            XCTFail("Expected URLError(.cancelled) raw, got \(error)")
        }

        XCTAssertEqual(counter.value, 1, "A cancelled URLSession task must not spend another attempt")
        XCTAssertEqual(spy.retries, [], "A cancelled URLSession task must not announce a retry")
        XCTAssertEqual(spy.starts, [1])
        XCTAssertEqual(spy.ends, [1])
    }

    /// A `Retry-After` large enough to overflow the nanosecond conversion must
    /// not trap the process. `UInt64(_:)` on an out-of-range `Double` is a
    /// runtime trap, so an unclamped conversion crashes here rather than
    /// failing.
    func testDownloadURL_absurdRetryAfterDoesNotTrap() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            if counter.increment() == 1 {
                return (
                    Data("{}".utf8),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 429,
                        headers: ["Retry-After": "99999999999"]
                    )
                )
            }
            return (
                Data("content".utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "text/plain"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true)

        let url = Self.hop1URL
        let task = Task { _ = try await account.downloadURL(url) }

        // The clamped sleep is a day, so the call cannot finish here. Reaching
        // this line at all is the assertion: an unclamped UInt64 conversion
        // would have trapped inside the first backoff.
        var waited = 0
        while counter.value < 1 {
            guard waited < 5_000 else {
                task.cancel()
                XCTFail("First hop-1 attempt never reached the transport")
                return
            }
            try await Task.sleep(nanoseconds: 1_000_000)
            waited += 1
        }
        try await Task.sleep(nanoseconds: 50_000_000)
        task.cancel()

        XCTAssertEqual(counter.value, 1, "The retry must still be waiting out the clamped delay")
    }
}

/// Thread-safe recorder for per-attempt Authorization header values.
private final class AuthRecorder: @unchecked Sendable {
    private let lock = NSLock()
    private var _values: [String?] = []

    var values: [String?] { lock.withLock { _values } }

    func record(_ value: String?) {
        lock.withLock { _values.append(value) }
    }
}
