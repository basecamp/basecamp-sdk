import XCTest

@testable import Basecamp

/// External links (doors). Bodies are the shared fixtures in `spec/fixtures/doors/`:
/// bc3's documented examples, read relative to this file.
final class DoorTests: XCTestCase {

    private func fixture(_ name: String) throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        return try Data(contentsOf: repoRoot.appendingPathComponent("spec/fixtures/doors/\(name).json"))
    }

    func testListReadsTheBucketScopedPath() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("list"))
        let account = makeTestAccountClient(transport: transport)

        let doors = try await account.doors.list(bucketId: 2085958504)

        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "GET")
        XCTAssertEqual(transport.lastRequest?.request.url?.path, "/999999999/buckets/2085958504/dock/doors.json")
        XCTAssertEqual(doors.count, 1)
        XCTAssertEqual(doors.first?.type, "Door")
    }

    func testGetReadsTheFlatPathAndTheOutsideAddress() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        let door = try await account.doors.get(doorId: 1069480290)

        XCTAssertEqual(transport.lastRequest?.request.url?.path, "/999999999/dock/doors/1069480290")
        XCTAssertEqual(door.url, "https://www.figma.com/file/abc123/Design-system")
        XCTAssertEqual(door.service.code, "figma")
        XCTAssertEqual(door.description, "<div>Shared Figma workspace</div>")
    }

    func testCreateSendsTheDocumentedRequest() async throws {
        let transport = MockTransport(statusCode: 201, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.doors.create(
            bucketId: 2085958504,
            req: CreateDoorRequest(
                door: CreateDoorPayload(
                    service: "github",
                    url: "https://github.com/honcho/leto-laptop",
                    description: "<div>Firmware and tooling</div>",
                    title: "Source code"),
                visibleToClients: false))

        let request = try XCTUnwrap(transport.lastRequest?.request)
        XCTAssertEqual(request.httpMethod, "POST")
        XCTAssertEqual(request.url?.path, "/999999999/buckets/2085958504/dock/doors.json")
        let sent = try JSONSerialization.jsonObject(with: XCTUnwrap(request.httpBody)) as? NSDictionary
        let expected = try JSONSerialization.jsonObject(with: fixture("create-request")) as? NSDictionary
        XCTAssertEqual(sent, expected)
    }

    func testUpdateCanChangeOnlyVisibility() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.doors.update(doorId: 1069480290, req: UpdateDoorRequest(visibleToClients: true))

        let request = try XCTUnwrap(transport.lastRequest?.request)
        XCTAssertEqual(request.httpMethod, "PUT")
        XCTAssertEqual(request.url?.path, "/999999999/dock/doors/1069480290")
        let sent = try JSONSerialization.jsonObject(with: XCTUnwrap(request.httpBody)) as? NSDictionary
        XCTAssertEqual(sent, ["visible_to_clients": true] as NSDictionary)
    }

    func testGetNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data(#"{"error":"Not found"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.doors.get(doorId: 1)
            XCTFail("Expected a not-found error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
        }
    }
}
