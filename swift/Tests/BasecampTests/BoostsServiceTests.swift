import XCTest

@testable import Basecamp

/// Tests for the generated `BoostsService.update`: the creator-only replacement
/// of a boost's content, with a happy path and a mapped-error path per status.
///
/// The success body is the shared fixture in `spec/fixtures/boosts/`, read
/// relative to this source file so it cannot drift from the copy the other SDKs
/// use. The request body is asserted on the wire: `content` and nothing else.
final class BoostsServiceTests: XCTestCase {
    private func fixture(_ name: String) throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()  // BasecampTests
            .deletingLastPathComponent()  // Tests
            .deletingLastPathComponent()  // swift
            .deletingLastPathComponent()  // <repo root>
        return try Data(
            contentsOf: repoRoot.appendingPathComponent("spec/fixtures/boosts/\(name).json"))
    }

    func testUpdateSendsContentAloneAndDecodesTheBoost() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        let boost = try await account.boosts.update(
            boostId: 500, req: UpdateBoostRequest(content: "Done!"))

        XCTAssertEqual(boost.id, 1069479500)
        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "PUT")
        XCTAssertEqual(transport.lastRequest?.request.url?.path, "/999999999/boosts/500")
        let sent = try XCTUnwrap(transport.lastRequest?.request.httpBody)
        let body = try XCTUnwrap(JSONSerialization.jsonObject(with: sent) as? [String: Any])
        XCTAssertEqual(body["content"] as? String, "Done!")
        XCTAssertEqual(Set(body.keys), ["content"])
    }

    func testUpdateForbidden() async throws {
        let transport = MockTransport(statusCode: 403, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.boosts.update(boostId: 500, req: UpdateBoostRequest(content: "Done!"))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
        }
    }

    func testUpdateNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.boosts.update(boostId: 500, req: UpdateBoostRequest(content: "Done!"))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
        }
    }

    func testUpdateUnprocessable() async throws {
        let transport = MockTransport(statusCode: 422, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.boosts.update(boostId: 500, req: UpdateBoostRequest(content: "Done!"))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .validation(_, let status, _, _, _) = error else {
                return XCTFail("Expected .validation, got \(error)")
            }
            XCTAssertEqual(status, 422)
        }
    }
}
