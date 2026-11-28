import XCTest
@testable import Basecamp

/// The current user's avatar: the multipart upload and the removal.
final class MyAvatarTests: XCTestCase {

    private func sentPath(_ transport: MockTransport) -> String {
        URLComponents(string: transport.lastRequest!.request.url!.absoluteString)!.path
    }

    func testUpdateMyAvatarSendsOneAvatarFilePart() async throws {
        let transport = MockTransport(statusCode: 204)
        let account = makeTestAccountClient(transport: transport)

        try await account.people.updateMyAvatar(
            data: Data("avatar-bytes".utf8), filename: "me.png", contentType: "image/png")

        let request = transport.lastRequest!.request
        XCTAssertEqual(request.httpMethod, "PUT")
        XCTAssertEqual(sentPath(transport), "/999999999/my/avatar.json")
        XCTAssertTrue(
            request.value(forHTTPHeaderField: "Content-Type")?.hasPrefix("multipart/form-data; boundary=") ?? false)
        let body = String(decoding: try XCTUnwrap(request.httpBody), as: UTF8.self)
        XCTAssertEqual(body.components(separatedBy: "Content-Disposition: form-data").count - 1, 1, body)
        XCTAssertTrue(
            body.contains(
                "Content-Disposition: form-data; name=\"avatar\"; filename=\"me.png\"\r\n"
                    + "Content-Type: image/png\r\n\r\navatar-bytes\r\n"),
            body)
    }

    func testUpdateMyAvatarRefusedImage() async throws {
        let body = Data(#"{"error":"Avatar is not an allowed image type"}"#.utf8)
        let transport = MockTransport(statusCode: 422, data: body)
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.people.updateMyAvatar(
                data: Data("not an image".utf8), filename: "a.txt", contentType: "text/plain")
            XCTFail("Expected a validation error")
        } catch let error as BasecampError {
            guard case .validation = error else { return XCTFail("Expected .validation, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 422)
        }
        XCTAssertEqual(sentPath(transport), "/999999999/my/avatar.json")
    }

    func testDeleteMyAvatar() async throws {
        let transport = MockTransport(statusCode: 204)
        let account = makeTestAccountClient(transport: transport)

        try await account.people.deleteMyAvatar()

        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "DELETE")
        XCTAssertEqual(sentPath(transport), "/999999999/my/avatar.json")
    }

    func testDeleteMyAvatarForbidden() async throws {
        let transport = MockTransport(statusCode: 403, data: Data(#"{"error":"Access denied"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.people.deleteMyAvatar()
            XCTFail("Expected a forbidden error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
            XCTAssertEqual(error.httpStatusCode, 403)
        }
        XCTAssertEqual(sentPath(transport), "/999999999/my/avatar.json")
    }
}
