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

    // MARK: - No Retry on 429

    func testDownloadURL_noRetryOn429() async throws {
        let counter = Counter()
        let transport = MockTransport { request in
            counter.increment()
            return (
                Data(#"{"error":"Rate limited"}"#.utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 429,
                    headers: ["Content-Type": "application/json", "Retry-After": "30"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport, enableRetry: true)

        do {
            _ = try await account.downloadURL("https://3.basecampapi.com/999999999/attachments/abc/download/file.txt")
            XCTFail("Expected rate limit error")
        } catch let error as BasecampError {
            guard case .rateLimit = error else {
                XCTFail("Expected rateLimit, got \(error)")
                return
            }
        }

        // Only one request — no retry
        XCTAssertEqual(counter.value, 1)
    }
}
