import XCTest
@testable import Basecamp

/// Smoke tests exercising actual generated service call paths.
/// Verifies the generator produces correct method signatures, request building,
/// and response decoding through the full BaseService lifecycle.
final class GeneratedServiceTests: XCTestCase {

    // MARK: - request<T> path (GET with JSON decode)

    func testGetProjectDecodesResponse() async throws {
        let json: [String: Any] = ["id": 42, "name": "My Project", "status": "active"]
        let data = try JSONSerialization.data(withJSONObject: json)

        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let project = try await account.projects.get(projectId: 42)
        XCTAssertEqual(project.id, 42)
        XCTAssertEqual(project.name, "My Project")
        XCTAssertEqual(project.status, "active")

        // Verify request was sent to the correct path
        let lastURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(lastURL.hasSuffix("/projects/42"), "Expected path /projects/42, got \(lastURL)")
    }

    // MARK: - request<T> path (POST with body)

    func testCreateTodoEncodesBodyAndDecodes() async throws {
        let responseJSON: [String: Any] = ["id": 99, "content": "Buy milk", "completed": false]
        let responseData = try JSONSerialization.data(withJSONObject: responseJSON)

        let transport = MockTransport(statusCode: 201, data: responseData)
        let account = makeTestAccountClient(transport: transport)

        let req = CreateTodoRequest(content: "Buy milk", dueOn: "2026-03-01")
        let todo = try await account.todos.create(projectId: 1, todolistId: 2, req: req)

        XCTAssertEqual(todo.id, 99)
        XCTAssertEqual(todo.content, "Buy milk")

        // Verify body was JSON-encoded with snake_case
        let sentBody = transport.lastRequest!.request.httpBody!
        let sentJSON = try JSONSerialization.jsonObject(with: sentBody) as! [String: Any]
        XCTAssertEqual(sentJSON["content"] as? String, "Buy milk")
        XCTAssertEqual(sentJSON["due_on"] as? String, "2026-03-01")
    }

    // MARK: - requestVoid path (DELETE)

    func testTrashProjectSendsDelete() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.projects.trash(projectId: 7)

        let req = transport.lastRequest!.request
        XCTAssertEqual(req.httpMethod, "DELETE")
        XCTAssertTrue(req.url!.absoluteString.hasSuffix("/projects/7"))
    }

    // MARK: - requestPaginated path

    func testListProjectsReturnsPaginatedResult() async throws {
        let projects: [[String: Any]] = [
            ["id": 1, "name": "Project A"],
            ["id": 2, "name": "Project B"],
        ]
        let data = try JSONSerialization.data(withJSONObject: projects)

        let transport = MockTransport(statusCode: 200, data: data,
                                      headers: ["X-Total-Count": "2"])
        let account = makeTestAccountClient(transport: transport)

        let result = try await account.projects.list()
        XCTAssertEqual(result.count, 2)
        XCTAssertEqual(result[0].id, 1)
        XCTAssertEqual(result[1].name, "Project B")
        XCTAssertEqual(result.meta.totalCount, 2)
    }

    func testListProjectsWithQueryParams() async throws {
        let data = try JSONSerialization.data(withJSONObject: [] as [Any])
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.projects.list(options: ListProjectOptions(status: "archived"))

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.contains("status=archived"), "Expected status query param in \(sentURL)")
    }

    // MARK: - Binary upload (CreateAttachment)

    func testCreateAttachmentSendsBinaryBody() async throws {
        let responseJSON: [String: Any] = ["attachable_sgid": "sgid-123"]
        let responseData = try JSONSerialization.data(withJSONObject: responseJSON)

        let transport = MockTransport(statusCode: 200, data: responseData)
        let account = makeTestAccountClient(transport: transport)

        let imageData = Data([0x89, 0x50, 0x4E, 0x47]) // PNG header bytes
        let result = try await account.attachments.create(
            data: imageData, contentType: "image/png", name: "photo.png"
        )

        XCTAssertEqual(result.attachableSgid, "sgid-123")

        let req = transport.lastRequest!.request
        XCTAssertEqual(req.value(forHTTPHeaderField: "Content-Type"), "image/png")
        XCTAssertEqual(req.httpBody, imageData)
        XCTAssertTrue(req.url!.absoluteString.contains("name=photo.png"))
    }

    // MARK: - HTTP error through generated service

    func testGeneratedServicePropagatesHTTPError() async throws {
        let transport = MockTransport(statusCode: 404, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.projects.get(projectId: 999)
            XCTFail("Expected error")
        } catch let error as BasecampError {
            XCTAssertEqual(error.httpStatusCode, 404)
        }
    }
}
