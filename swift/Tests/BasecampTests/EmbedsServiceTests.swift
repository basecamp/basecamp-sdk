import XCTest

@testable import Basecamp

final class EmbedsServiceTests: XCTestCase {
    private let youtubeURL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

    private func fixture(_ name: String) throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        return try Data(
            contentsOf: repoRoot.appendingPathComponent("spec/fixtures/embeds/\(name).json"))
    }

    private func pendingSgid() throws -> String {
        let object = try JSONSerialization.jsonObject(with: try fixture("pending")) as? [String: Any]
        return try XCTUnwrap(object?["sgid"] as? String)
    }

    func testCreateSendsTheURLAndDecodesThePendingEmbed() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("pending"))
        let account = makeTestAccountClient(transport: transport)

        let embed = try await account.embeds.create(req: CreateEmbedRequest(url: youtubeURL))

        XCTAssertEqual(embed.status, "pending")
        XCTAssertEqual(embed.sgid, try pendingSgid())
        XCTAssertNil(embed.contentType)
        let body = try XCTUnwrap(transport.lastRequest?.request.httpBody)
        let sent = try XCTUnwrap(JSONSerialization.jsonObject(with: body) as? [String: Any])
        XCTAssertEqual(sent as? [String: String], ["url": youtubeURL])
        XCTAssertEqual(transport.lastRequest?.request.httpMethod, "POST")
        XCTAssertEqual(transport.lastRequest?.request.url?.path, "/999999999/embeds.json")
    }

    func testCreateReturnsAnExistingActiveEmbed() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("active"))
        let account = makeTestAccountClient(transport: transport)

        let embed = try await account.embeds.create(req: CreateEmbedRequest(url: youtubeURL))

        XCTAssertEqual(embed.status, "active")
        XCTAssertEqual(embed.contentType, "embed/youtube-video")
        XCTAssertNil(embed.imageUrl)
    }

    func testCreateSurfacesA422ForAURLNoProviderEmbeds() async throws {
        let transport = MockTransport(
            statusCode: 422,
            data: Data(#"{"error":"Basecamp can't embed this URL. Link to it instead."}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.embeds.create(req: CreateEmbedRequest(url: "https://example.com/page"))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .validation = error else { return XCTFail("Expected .validation, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 422)
            XCTAssertEqual(error.message, "Basecamp can't embed this URL. Link to it instead.")
        }
    }

    func testGetEscapesTheSgidAsOnePathSegment() async throws {
        let sgid = try pendingSgid()
        let transport = MockTransport(statusCode: 200, data: try fixture("active"))
        let account = makeTestAccountClient(transport: transport)

        let embed = try await account.embeds.get(sgid: sgid)

        XCTAssertEqual(embed.status, "active")
        let escaped = sgid.replacingOccurrences(of: "/", with: "%2F").replacingOccurrences(of: "=", with: "%3D")
        XCTAssertEqual(
            transport.lastRequest?.request.url?.absoluteString.hasSuffix("/999999999/embeds/\(escaped)"), true,
            transport.lastRequest?.request.url?.absoluteString ?? "")
    }

    func testGetNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data(#"{"error":"Not found"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.embeds.get(sgid: "missing")
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 404)
        }
    }
}
