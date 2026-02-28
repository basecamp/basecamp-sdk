import XCTest
@testable import Basecamp

/// Smoke tests exercising actual generated service call paths.
/// Verifies the generator produces correct method signatures, request building,
/// and response decoding through the full BaseService lifecycle.
final class GeneratedServiceTests: XCTestCase {

    // MARK: - request<T> path (GET with JSON decode)

    func testGetProjectDecodesResponse() async throws {
        let json: [String: Any] = [
            "id": 42, "name": "My Project", "status": "active",
            "app_url": "https://3.basecamp.com/1/projects/42", "url": "https://3.basecampapi.com/1/projects/42.json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
        ]
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
        let responseJSON: [String: Any] = [
            "id": 99, "content": "Buy milk", "completed": false,
            "app_url": "https://3.basecamp.com/1/buckets/1/todos/99", "url": "https://3.basecampapi.com/1/buckets/1/todos/99.json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
            "status": "active", "title": "Buy milk", "type": "Todo",
            "inherits_status": false, "visible_to_clients": false,
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "parent": ["id": 2, "title": "Todolist", "type": "Todolist", "app_url": "https://3.basecamp.com/1/buckets/1/todolists/2", "url": "https://3.basecampapi.com/1/buckets/1/todolists/2.json"] as [String: Any],
        ]
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
            ["id": 1, "name": "Project A", "status": "active",
             "app_url": "https://3.basecamp.com/1/projects/1", "url": "https://3.basecampapi.com/1/projects/1.json",
             "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"],
            ["id": 2, "name": "Project B", "status": "active",
             "app_url": "https://3.basecamp.com/1/projects/2", "url": "https://3.basecampapi.com/1/projects/2.json",
             "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"],
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

    // MARK: - PUT/PATCH Through Generated Service (Update)

    func testUpdateWebhookSendsPUT() async throws {
        let responseJSON: [String: Any] = [
            "id": 5, "payload_url": "https://hooks.example.com/updated",
            "app_url": "https://3.basecamp.com/1/buckets/1/webhooks/5",
            "url": "https://3.basecampapi.com/1/buckets/1/webhooks/5.json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
            "active": true,
        ]
        let responseData = try JSONSerialization.data(withJSONObject: responseJSON)

        let transport = MockTransport(statusCode: 200, data: responseData)
        let account = makeTestAccountClient(transport: transport)

        let req = UpdateWebhookRequest(active: true, payloadUrl: "https://hooks.example.com/updated")
        let webhook = try await account.webhooks.update(projectId: 1, webhookId: 5, req: req)

        XCTAssertEqual(webhook.id, 5)
        XCTAssertEqual(webhook.payloadUrl, "https://hooks.example.com/updated")

        let sentReq = transport.lastRequest!.request
        XCTAssertEqual(sentReq.httpMethod, "PUT")
        XCTAssertTrue(sentReq.url!.absoluteString.hasSuffix("/buckets/1/webhooks/5"))
    }

    func testUpdateCommentSendsPUT() async throws {
        let responseJSON: [String: Any] = [
            "id": 10, "content": "Updated comment",
            "app_url": "https://3.basecamp.com/1/buckets/1/comments/10",
            "url": "https://3.basecampapi.com/1/buckets/1/comments/10.json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
            "status": "active", "title": "Updated", "type": "Comment",
            "inherits_status": false, "visible_to_clients": false,
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "parent": ["id": 2, "title": "Todo", "type": "Todo", "app_url": "https://3.basecamp.com/1/buckets/1/todos/2", "url": "https://3.basecampapi.com/1/buckets/1/todos/2.json"] as [String: Any],
        ]
        let responseData = try JSONSerialization.data(withJSONObject: responseJSON)

        let transport = MockTransport(statusCode: 200, data: responseData)
        let account = makeTestAccountClient(transport: transport)

        let req = UpdateCommentRequest(content: "Updated comment")
        let comment = try await account.comments.update(projectId: 1, commentId: 10, req: req)

        XCTAssertEqual(comment.id, 10)
        XCTAssertEqual(comment.content, "Updated comment")

        let sentReq = transport.lastRequest!.request
        XCTAssertEqual(sentReq.httpMethod, "PUT")
    }

    // MARK: - Decode Error (Malformed JSON)

    func testDecodeErrorFromMalformedJSON() async throws {
        let transport = MockTransport(statusCode: 200, data: Data("not-json".utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.projects.get(projectId: 1)
            XCTFail("Expected decode error")
        } catch is DecodingError {
            // Expected — malformed JSON causes DecodingError
        } catch {
            // The error bubbles through the service layer
            // It may be wrapped; just verify it propagates
        }
    }

    // MARK: - HTTP Error Mapping Through Service Layer

    func test401ErrorMapsToAuth() async throws {
        let transport = MockTransport(statusCode: 401, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.projects.get(projectId: 1)
            XCTFail("Expected 401 error")
        } catch let error as BasecampError {
            if case .auth = error {
                XCTAssertEqual(error.httpStatusCode, 401)
            } else {
                XCTFail("Expected .auth error, got \(error)")
            }
        }
    }

    func test403ErrorMapsToForbidden() async throws {
        let transport = MockTransport(statusCode: 403, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.projects.get(projectId: 1)
            XCTFail("Expected 403 error")
        } catch let error as BasecampError {
            if case .forbidden = error {
                XCTAssertEqual(error.httpStatusCode, 403)
            } else {
                XCTFail("Expected .forbidden error, got \(error)")
            }
        }
    }

    func test404ErrorMapsToNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.projects.get(projectId: 999)
            XCTFail("Expected 404 error")
        } catch let error as BasecampError {
            if case .notFound = error {
                XCTAssertEqual(error.httpStatusCode, 404)
            } else {
                XCTFail("Expected .notFound error, got \(error)")
            }
        }
    }

    func test422ErrorMapsToValidation() async throws {
        let errorBody = try JSONSerialization.data(withJSONObject: [
            "error": "Title can't be blank"
        ])
        let transport = MockTransport(statusCode: 422, data: errorBody)
        let account = makeTestAccountClient(transport: transport)

        do {
            let req = CreateTodoRequest(content: "")
            _ = try await account.todos.create(projectId: 1, todolistId: 2, req: req)
            XCTFail("Expected 422 error")
        } catch let error as BasecampError {
            if case .validation(let message, let status, _, _) = error {
                XCTAssertEqual(status, 422)
                XCTAssertEqual(message, "Title can't be blank")
            } else {
                XCTFail("Expected .validation error, got \(error)")
            }
        }
    }

    // MARK: - Service Category Coverage

    func testWebhooksServiceCreate() async throws {
        let responseJSON: [String: Any] = [
            "id": 1, "payload_url": "https://hooks.example.com/bc",
            "app_url": "https://3.basecamp.com/1/buckets/1/webhooks/1",
            "url": "https://3.basecampapi.com/1/buckets/1/webhooks/1.json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
        ]
        let data = try JSONSerialization.data(withJSONObject: responseJSON)
        let transport = MockTransport(statusCode: 201, data: data)
        let account = makeTestAccountClient(transport: transport)

        let req = CreateWebhookRequest(payloadUrl: "https://hooks.example.com/bc", types: ["Comment"])
        let webhook = try await account.webhooks.create(projectId: 1, req: req)
        XCTAssertEqual(webhook.id, 1)
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "POST")
    }

    func testCommentsServiceGet() async throws {
        let responseJSON: [String: Any] = [
            "id": 7, "content": "Great idea!",
            "app_url": "https://3.basecamp.com/1/buckets/1/comments/7",
            "url": "https://3.basecampapi.com/1/buckets/1/comments/7.json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
            "status": "active", "title": "Great idea!", "type": "Comment",
            "inherits_status": false, "visible_to_clients": false,
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "parent": ["id": 2, "title": "Todo", "type": "Todo",
                        "app_url": "https://3.basecamp.com/1/buckets/1/todos/2",
                        "url": "https://3.basecampapi.com/1/buckets/1/todos/2.json"] as [String: Any],
        ]
        let data = try JSONSerialization.data(withJSONObject: responseJSON)
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let comment = try await account.comments.get(projectId: 1, commentId: 7)
        XCTAssertEqual(comment.id, 7)
        XCTAssertEqual(comment.content, "Great idea!")
    }

    func testMessagesServiceGet() async throws {
        let responseJSON: [String: Any] = [
            "id": 3, "subject": "Weekly Update", "content": "Here's what happened...",
            "app_url": "https://3.basecamp.com/1/buckets/1/messages/3",
            "url": "https://3.basecampapi.com/1/buckets/1/messages/3.json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
            "status": "active", "title": "Weekly Update", "type": "Message",
            "inherits_status": false, "visible_to_clients": false,
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "parent": ["id": 4, "title": "Message Board", "type": "Message::Board",
                        "app_url": "https://3.basecamp.com/1/buckets/1/message_boards/4",
                        "url": "https://3.basecampapi.com/1/buckets/1/message_boards/4.json"] as [String: Any],
        ]
        let data = try JSONSerialization.data(withJSONObject: responseJSON)
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let message = try await account.messages.get(projectId: 1, messageId: 3)
        XCTAssertEqual(message.id, 3)
        XCTAssertEqual(message.subject, "Weekly Update")
    }

    func testPeopleServiceGetPerson() async throws {
        let responseJSON: [String: Any] = [
            "id": 42, "name": "Jeremy Sharp",
            "email_address": "jeremy@example.com",
        ]
        let data = try JSONSerialization.data(withJSONObject: responseJSON)
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let person = try await account.people.get(personId: 42)
        XCTAssertEqual(person.id, 42)
        XCTAssertEqual(person.name, "Jeremy Sharp")

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/people/42"))
    }

    func testPeopleServiceMe() async throws {
        let responseJSON: [String: Any] = [
            "id": 1, "name": "Current User",
        ]
        let data = try JSONSerialization.data(withJSONObject: responseJSON)
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let person = try await account.people.me()
        XCTAssertEqual(person.name, "Current User")

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/my/profile.json"))
    }
}
