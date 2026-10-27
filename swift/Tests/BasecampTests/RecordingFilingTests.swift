import XCTest

@testable import Basecamp

/// Moving and copying a recording to another project, and reading back the
/// filing either one starts. Filing bodies are the shared fixtures in
/// `spec/fixtures/recordings/`, read relative to this file.
final class RecordingFilingTests: XCTestCase {

    private func fixture(_ name: String) throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        return try Data(
            contentsOf: repoRoot.appendingPathComponent("spec/fixtures/recordings/\(name).json"))
    }

    private func sentBody(_ transport: MockTransport) throws -> [String: Any] {
        let body = try XCTUnwrap(transport.lastRequest?.request.httpBody)
        return try XCTUnwrap(JSONSerialization.jsonObject(with: body) as? [String: Any])
    }

    func testMoveSendsTheDestinationProjectAndDecodesThePendingFiling() async throws {
        let transport = MockTransport(
            statusCode: 201, data: try fixture("filing_pending"),
            headers: ["Location": "https://3.basecampapi.com/999999999/filings/4.json"])
        let account = makeTestAccountClient(transport: transport)

        let filing = try await account.recordings.move(
            recordingId: 1069479351, req: MoveRecordingRequest(destinationProjectId: 2085958505))

        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "POST")
        XCTAssertEqual(transport.lastRequest?.request.url?.path, "/999999999/recordings/1069479351/moves.json")
        XCTAssertEqual(try sentBody(transport) as? [String: Int], ["destination_project_id": 2085958505])
        XCTAssertEqual(filing.id, 4)
        XCTAssertEqual(filing.status, "pending")
        XCTAssertEqual(filing.type, "Move")
        XCTAssertEqual(filing.sourceRecordingId, 1069479351)
        XCTAssertEqual(filing.url, "https://3.basecampapi.com/195539477/filings/4.json")
        XCTAssertNil(filing.destinationRecording)
    }

    func testMoveSurfacesARefusal() async throws {
        let transport = MockTransport(
            statusCode: 422, data: Data(#"{"status":422,"error":"Unprocessable Entity"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.recordings.move(
                recordingId: 1069479351, req: MoveRecordingRequest(filingDestinationId: 4001))
            XCTFail("Expected a validation error")
        } catch let error as BasecampError {
            guard case .validation = error else { return XCTFail("Expected .validation, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 422)
        }
    }

    func testCopySendsTheDestinationAndIncludeComments() async throws {
        let transport = MockTransport(statusCode: 201, data: try fixture("filing_pending"))
        let account = makeTestAccountClient(transport: transport)

        let filing = try await account.recordings.copy(
            recordingId: 1069479351,
            req: CopyRecordingRequest(destinationProjectId: 2085958505, includeComments: true))

        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "POST")
        XCTAssertEqual(transport.lastRequest?.request.url?.path, "/999999999/recordings/1069479351/copies.json")
        let sent = try sentBody(transport)
        XCTAssertEqual(sent.keys.sorted(), ["destination_project_id", "include_comments"])
        XCTAssertEqual(sent["destination_project_id"] as? Int, 2085958505)
        XCTAssertEqual(sent["include_comments"] as? Bool, true)
        XCTAssertEqual(filing.status, "pending")
        XCTAssertNil(filing.destinationRecording)
    }

    /// A copy is not idempotent: a retried 503 could make two copies, so it is
    /// sent once even with retries enabled and 503 in its retryOn list.
    func testCopyIsSentOnceOnA503WithRetriesEnabled() async throws {
        let transport = MockTransport(statusCode: 503)
        let account = makeTestAccountClient(transport: transport, enableRetry: true)

        do {
            _ = try await account.recordings.copy(
                recordingId: 1069479351, req: CopyRecordingRequest(filingDestinationId: 4001, includeComments: true))
            XCTFail("Expected a 503 error")
        } catch let error as BasecampError {
            XCTAssertEqual(error.httpStatusCode, 503)
        }

        XCTAssertEqual(transport.requests.count, 1, "CopyRecording must not retry")
    }

    func testGetFilingDecodesTheCompletedFilingAndItsDestination() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("filing_completed"))
        let account = makeTestAccountClient(transport: transport)

        let filing = try await account.recordings.getFiling(filingId: 4)

        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "GET")
        XCTAssertEqual(transport.lastRequest?.request.url?.path, "/999999999/filings/4")
        XCTAssertEqual(filing.status, "completed")
        XCTAssertEqual(filing.type, "Move")
        let destination = try XCTUnwrap(filing.destinationRecording)
        XCTAssertEqual(destination.id, 1069480270)
        XCTAssertEqual(destination.type, "Message")
        XCTAssertEqual(
            destination.url, "https://3.basecampapi.com/195539477/buckets/2085958505/messages/1069480270.json")
        XCTAssertEqual(destination.appUrl, "https://3.basecamp.com/195539477/buckets/2085958505/messages/1069480270")
    }

    func testGetFilingNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data(#"{"status":404,"error":"Not Found"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.recordings.getFiling(filingId: 4)
            XCTFail("Expected a not-found error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 404)
        }
    }
}
