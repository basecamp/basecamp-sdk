import XCTest
@testable import Basecamp

/// Thread-safe counter for use in @Sendable closures.
private final class Counter: @unchecked Sendable {
    private let lock = NSLock()
    private var _value: Int = 0

    var value: Int { lock.withLock { _value } }

    @discardableResult
    func increment() -> Int {
        lock.withLock {
            _value += 1
            return _value
        }
    }
}

/// Returns a minimally-populated upload metadata JSON object keyed on wire
/// (snake_case) field names. Callers pass overrides for `download_url` and
/// `filename`; the rest are fixed fillers for Upload's required fields.
private func uploadMetadataJSON(
    id: Int = 1069479400,
    downloadURL: String?,
    filename: String?
) -> [String: Any] {
    var json: [String: Any] = [
        "id": id,
        "app_url": "https://3.basecamp.com/999999999/uploads/\(id)",
        "url": "https://3.basecampapi.com/999999999/uploads/\(id).json",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "status": "active",
        "title": "report.pdf",
        "type": "Upload",
        "inherits_status": false,
        "visible_to_clients": false,
        "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
        "creator": ["id": 1, "name": "Test User"] as [String: Any],
        "parent": [
            "id": 2, "title": "Docs", "type": "Vault",
            "app_url": "https://3.basecamp.com/999999999/vaults/2",
            "url": "https://3.basecampapi.com/999999999/vaults/2.json"
        ] as [String: Any]
    ]
    if let downloadURL = downloadURL {
        json["download_url"] = downloadURL
    }
    if let filename = filename {
        json["filename"] = filename
    }
    return json
}

final class UploadsServiceExtensionsTests: XCTestCase {

    // MARK: - Success path

    /// The metadata fetch resolves `download_url` to an API-host URL; the
    /// download primitive then runs the auth'd hop (which 302s) and the signed
    /// hop (no auth). The returned filename comes from the upload metadata.
    func testDownload_delegatesThroughDownloadURL() async throws {
        let metadata = uploadMetadataJSON(
            downloadURL: "https://storage.3.basecamp.com/999999999/blobs/abcd1234/download/logo.png",
            filename: "logo.png"
        )
        let metadataData = try JSONSerialization.data(withJSONObject: metadata)

        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            switch count {
            case 1:
                // Metadata GET /uploads/{id}
                XCTAssertEqual(request.url?.path, "/999999999/uploads/1069479400")
                XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer test-token")
                return (
                    metadataData,
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 200,
                        headers: ["Content-Type": "application/json"]
                    )
                )
            case 2:
                // Hop 1: auth'd API request (origin-rewritten from storage host to base host)
                XCTAssertEqual(request.url?.host, "3.basecampapi.com")
                XCTAssertEqual(request.url?.path, "/999999999/blobs/abcd1234/download/logo.png")
                XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer test-token")
                return (
                    Data(),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 302,
                        headers: ["Location": "https://signed.example/bucket/xyz?sig=abc"]
                    )
                )
            case 3:
                // Hop 2: signed download, no auth header
                XCTAssertNil(request.value(forHTTPHeaderField: "Authorization"))
                return (
                    Data("pixels".utf8),
                    makeHTTPResponse(
                        url: request.url!.absoluteString,
                        statusCode: 200,
                        headers: ["Content-Type": "image/png", "Content-Length": "6"]
                    )
                )
            default:
                XCTFail("Unexpected request #\(count) to \(request.url?.absoluteString ?? "<nil>")")
                return (Data(), makeHTTPResponse(url: request.url!.absoluteString, statusCode: 500))
            }
        }
        let account = makeTestAccountClient(transport: transport)

        let result = try await account.uploads.download(uploadId: 1069479400)

        XCTAssertEqual(String(data: result.body, encoding: .utf8), "pixels")
        XCTAssertEqual(result.contentType, "image/png")
        XCTAssertEqual(result.contentLength, 6)
        // filename from metadata wins over URL-derived filename
        XCTAssertEqual(result.filename, "logo.png")
        XCTAssertEqual(counter.value, 3)
    }

    // MARK: - Missing download_url

    func testDownload_throwsUsageWhenDownloadURLMissing() async throws {
        let metadata = uploadMetadataJSON(downloadURL: nil, filename: "logo.png")
        let metadataData = try JSONSerialization.data(withJSONObject: metadata)

        let counter = Counter()
        let transport = MockTransport { request in
            let count = counter.increment()
            XCTAssertEqual(count, 1, "No second hop should fire when metadata has no download_url")
            return (
                metadataData,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.uploads.download(uploadId: 1069479400)
            XCTFail("Expected BasecampError.usage")
        } catch let BasecampError.usage(message, _) {
            XCTAssertTrue(message.contains("1069479400"), "Message should include upload id, got: \(message)")
            XCTAssertTrue(message.contains("download_url"), "Message should mention download_url, got: \(message)")
        }
        XCTAssertEqual(counter.value, 1)
    }
}
