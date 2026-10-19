import XCTest

@testable import Basecamp

/// Tests for the generated `PingsService`: list, get and create, each with a
/// happy path and a mapped-error path.
///
/// Bodies are the shared fixtures in `spec/fixtures/pings/`, read relative to
/// this source file so they cannot drift from the copy the other SDKs and the
/// conformance runners use. Create's request body is asserted on the wire: a
/// ping is made by `user_ids`, and `new_group` is sent only when it is set.
final class PingsServiceTests: XCTestCase {
    private func fixture(_ name: String) throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()  // BasecampTests
            .deletingLastPathComponent()  // Tests
            .deletingLastPathComponent()  // swift
            .deletingLastPathComponent()  // <repo root>
        return try Data(
            contentsOf: repoRoot.appendingPathComponent("spec/fixtures/pings/\(name).json"))
    }

    private func sentJSON(_ transport: MockTransport) throws -> [String: Any] {
        let body = try XCTUnwrap(transport.lastRequest?.request.httpBody)
        return try XCTUnwrap(JSONSerialization.jsonObject(with: body) as? [String: Any])
    }

    private func path(_ transport: MockTransport) -> String {
        transport.lastRequest?.request.url?.path ?? ""
    }

    // MARK: - List

    func testListDecodesTheSharedFixture() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("list"))
        let account = makeTestAccountClient(transport: transport)

        let pings = try await account.pings.list()

        XCTAssertEqual(pings.count, 2)
        XCTAssertEqual(pings[0].id, 2085958600)
        XCTAssertEqual(pings[0].title, "Launch crew")
        XCTAssertTrue(pings[0].named)
        XCTAssertEqual(pings[0].people.count, 2)
        XCTAssertFalse(pings[1].named, "a direct ping is not a named group")
        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "GET")
        XCTAssertEqual(path(transport), "/999999999/circles.json")
    }

    func testListOfNothingIsEmpty() async throws {
        let transport = MockTransport(statusCode: 200, data: Data("[]".utf8))
        let account = makeTestAccountClient(transport: transport)

        let pings = try await account.pings.list()

        XCTAssertEqual(pings.count, 0)
    }

    func testListSurfacesForbiddenWhenPingsAreDisabled() async throws {
        let transport = MockTransport(
            statusCode: 403, data: Data(#"{"error":"Pings have been disabled"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.pings.list()
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 403)
        }
    }

    // MARK: - Get

    func testGetDecodesTheSharedFixture() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        let ping = try await account.pings.get(pingId: 2085958600)

        XCTAssertEqual(ping.id, 2085958600)
        XCTAssertEqual(ping.title, "Launch crew")
        XCTAssertEqual(
            ping.roomUrl,
            "https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600.json")
        XCTAssertEqual(CampfireIndex.campfireId(fromRoomURL: ping.roomUrl), 1069479600)
        XCTAssertEqual(ping.editTitleUrl, "https://3.basecamp.com/195539477/buckets/2085958600/title/edit")
        XCTAssertEqual(ping.links.lines.hasSuffix("/chats/1069479600/lines.json"), true)
        XCTAssertEqual(ping.people.map(\.name), ["Annie Bryan", "Cheryl Walters"])
        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "GET")
        XCTAssertEqual(path(transport), "/999999999/circles/2085958600", "a Ping read carries no .json")
    }

    func testGetNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data(#"{"error":"Record not found"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.pings.get(pingId: 1)
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 404)
        }
    }

    func testGetForbiddenWhenPingsAreDisabled() async throws {
        let transport = MockTransport(
            statusCode: 403, data: Data(#"{"error":"Pings have been disabled"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.pings.get(pingId: 1)
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
        }
    }

    // MARK: - Create

    func testCreateSendsUserIdsAndOmitsNewGroupWhenUnset() async throws {
        let transport = MockTransport(statusCode: 201, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        let ping = try await account.pings.create(
            req: CreatePingRequest(userIds: [1049715915, 1049715916]))

        XCTAssertEqual(ping.id, 2085958600)
        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "POST")
        XCTAssertEqual(path(transport), "/999999999/circles.json")
        let body = try sentJSON(transport)
        XCTAssertEqual(body["user_ids"] as? [Int], [1049715915, 1049715916])
        XCTAssertNil(body["new_group"], "new_group is sent only when it is set")
        XCTAssertEqual(Set(body.keys), ["user_ids"])
    }

    func testCreateSendsNewGroupWhenSet() async throws {
        let transport = MockTransport(statusCode: 201, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.pings.create(
            req: CreatePingRequest(newGroup: true, userIds: [1049715915, 1049715916]))

        XCTAssertEqual(try sentJSON(transport)["new_group"] as? Bool, true)
    }

    func testCreateValidationError() async throws {
        let transport = MockTransport(
            statusCode: 422, data: Data(#"{"error":"Pick at least one person"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.pings.create(req: CreatePingRequest(userIds: []))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .validation(let message, let status, _, _, _) = error else {
                return XCTFail("Expected .validation, got \(error)")
            }
            XCTAssertEqual(status, 422)
            XCTAssertEqual(message, "Pick at least one person")
        }
    }

    func testCreateForbiddenWhenPingsAreDisabled() async throws {
        let transport = MockTransport(
            statusCode: 403, data: Data(#"{"error":"Pings have been disabled"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.pings.create(req: CreatePingRequest(userIds: [1]))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
        }
    }
}
