import XCTest

@testable import Basecamp

/// Reading any recording by id. Bodies are the shared fixtures in
/// `spec/fixtures/recordings/`, read relative to this file.
final class RecordingGetTests: XCTestCase {

    private func fixture(_ name: String) throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        return try Data(
            contentsOf: repoRoot.appendingPathComponent("spec/fixtures/recordings/\(name).json"))
    }

    func testGetDecodesTypeParentAndBucket() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        let recording = try await account.recordings.get(recordingId: 1069479351)

        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "GET")
        XCTAssertEqual(transport.lastRequest?.request.url?.path, "/999999999/recordings/1069479351")
        XCTAssertEqual(recording.type, "Message")
        let parent = try XCTUnwrap(recording.parent)
        XCTAssertEqual(parent.id, 1069479338)
        XCTAssertEqual(parent.type, "Message::Board")
        XCTAssertEqual(recording.bucket.id, 2085958499)
    }

    func testGetDecodesADockToolWithoutAParent() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get_tool"))
        let account = makeTestAccountClient(transport: transport)

        let recording = try await account.recordings.get(recordingId: 1069479338)

        XCTAssertEqual(recording.type, "Message::Board")
        XCTAssertNil(recording.parent)
        XCTAssertEqual(recording.bucket.id, 2085958499)
    }

    func testGetNotFound() async throws {
        let transport = MockTransport(statusCode: 404)
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.recordings.get(recordingId: 9999)
            XCTFail("Expected a not-found error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 404)
        }
    }
}
