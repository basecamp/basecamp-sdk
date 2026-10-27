import XCTest
@testable import Basecamp

/// Project logo: the multipart upload, the
/// removal, and the `logo` object on the project read.
final class ProjectLogoTests: XCTestCase {

    private func sentComponents(_ transport: MockTransport) -> URLComponents {
        URLComponents(string: transport.lastRequest!.request.url!.absoluteString)!
    }

    func testUpdateProjectLogoSendsMultipart() async throws {
        let transport = MockTransport(statusCode: 204)
        let account = makeTestAccountClient(transport: transport)

        try await account.projects.updateProjectLogo(
            projectId: 42,
            data: Data("fake-webp-data".utf8),
            filename: "beach.webp",
            contentType: "image/webp")

        let request = transport.lastRequest!.request
        XCTAssertEqual(request.httpMethod, "PUT")
        XCTAssertEqual(sentComponents(transport).path, "/999999999/projects/42/logo.json")
        XCTAssertTrue(
            request.value(forHTTPHeaderField: "Content-Type")?.hasPrefix("multipart/form-data; boundary=") ?? false)
        let body = String(decoding: try XCTUnwrap(request.httpBody), as: UTF8.self)
        XCTAssertTrue(body.contains(#"name="logo"; filename="beach.webp""#), body)
        XCTAssertTrue(body.contains("fake-webp-data"), body)
    }

    func testUpdateProjectLogoHasNoQueryParameters() async throws {
        let transport = MockTransport(statusCode: 204)
        let account = makeTestAccountClient(transport: transport)

        try await account.projects.updateProjectLogo(
            projectId: 42, data: Data("data".utf8), filename: "beach.png", contentType: "image/png")

        XCTAssertNil(sentComponents(transport).queryItems)
    }

    func testUpdateProjectLogoRefusedFormat() async throws {
        let body = Data(#"{"error":"Logo is not an allowed image type"}"#.utf8)
        let transport = MockTransport(statusCode: 422, data: body)
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.projects.updateProjectLogo(
                projectId: 42, data: Data("not an image".utf8), filename: "a.txt", contentType: "text/plain")
            XCTFail("Expected a validation error")
        } catch let error as BasecampError {
            guard case .validation = error else { return XCTFail("Expected .validation, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 422)
        }
    }

    func testRemoveProjectLogo() async throws {
        let transport = MockTransport(statusCode: 204)
        let account = makeTestAccountClient(transport: transport)

        try await account.projects.removeProjectLogo(projectId: 42)

        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "DELETE")
        XCTAssertEqual(sentComponents(transport).path, "/999999999/projects/42/logo.json")
    }

    func testRemoveProjectLogoForbidden() async throws {
        let transport = MockTransport(statusCode: 403, data: Data(#"{"error":"Access denied"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.projects.removeProjectLogo(projectId: 42)
            XCTFail("Expected a forbidden error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 403)
        }
    }

    func testGetProjectDecodesLogo() async throws {
        let data = try JSONSerialization.data(withJSONObject: [
            "id": 42,
            "status": "active",
            "name": "Walled",
            "created_at": "2024-01-15T10:00:00Z",
            "updated_at": "2024-01-15T10:00:00Z",
            "url": "https://3.basecampapi.com/999999999/projects/42.json",
            "app_url": "https://3.basecamp.com/999999999/projects/42",
            "logo": [
                "url": "https://3.basecampapi.com/999999999/projects/42/logo?v=1669139787",
            ],
        ])
        let account = makeTestAccountClient(transport: MockTransport(statusCode: 200, data: data))

        let project = try await account.projects.get(projectId: 42)

        XCTAssertEqual(
            project.logo?.url, "https://3.basecampapi.com/999999999/projects/42/logo?v=1669139787")
    }
}
