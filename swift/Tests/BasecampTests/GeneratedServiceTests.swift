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
            "start_date": "2024-01-01", "end_date": "2024-03-31",
        ]
        let data = try JSONSerialization.data(withJSONObject: json)

        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let project = try await account.projects.get(projectId: 42)
        XCTAssertEqual(project.id, 42)
        XCTAssertEqual(project.name, "My Project")
        XCTAssertEqual(project.status, "active")
        XCTAssertEqual(project.startDate, "2024-01-01")
        XCTAssertEqual(project.endDate, "2024-03-31")

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
            "description_attachments": [],
            "inherits_status": false, "visible_to_clients": false,
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "parent": ["id": 2, "title": "Todolist", "type": "Todolist", "app_url": "https://3.basecamp.com/1/buckets/1/todolists/2", "url": "https://3.basecampapi.com/1/buckets/1/todolists/2.json"] as [String: Any],
        ]
        let responseData = try JSONSerialization.data(withJSONObject: responseJSON)

        let transport = MockTransport(statusCode: 201, data: responseData)
        let account = makeTestAccountClient(transport: transport)

        let req = CreateTodoRequest(content: "Buy milk", dueOn: "2026-03-01")
        let todo = try await account.todos.create(todolistId: 2, req: req)

        XCTAssertEqual(todo.id, 99)
        XCTAssertEqual(todo.content, "Buy milk")

        // Verify body was JSON-encoded with snake_case
        let sentBody = transport.lastRequest!.request.httpBody!
        let sentJSON = try JSONSerialization.jsonObject(with: sentBody) as! [String: Any]
        XCTAssertEqual(sentJSON["content"] as? String, "Buy milk")
        XCTAssertEqual(sentJSON["due_on"] as? String, "2026-03-01")
    }

    func testCreateProjectFromTemplateNestsBodyUnderProjectEnvelope() async throws {
        let responseJSON: [String: Any] = ["id": 900, "status": "completed"]
        let responseData = try JSONSerialization.data(withJSONObject: responseJSON)

        let transport = MockTransport(statusCode: 201, data: responseData)
        let account = makeTestAccountClient(transport: transport)

        let req = CreateProjectFromTemplateRequest(
            project: ProjectConstructionAttributes(name: "New Project", description: "From template")
        )
        let construction = try await account.templates.createProject(templateId: 456, req: req)
        XCTAssertEqual(construction.id, 900)

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/templates/456/project_constructions.json"), "Got \(sentURL)")

        // Body must nest project attributes under a `project` envelope, not flat.
        let sentBody = transport.lastRequest!.request.httpBody!
        let sentJSON = try JSONSerialization.jsonObject(with: sentBody) as! [String: Any]
        XCTAssertNil(sentJSON["name"], "name must not appear at the top level")
        let project = sentJSON["project"] as! [String: Any]
        XCTAssertEqual(project["name"] as? String, "New Project")
        XCTAssertEqual(project["description"] as? String, "From template")
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
             "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
             "start_date": "2024-01-01", "end_date": "2024-03-31"],
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
        XCTAssertEqual(result[0].startDate, "2024-01-01")
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
        let webhook = try await account.webhooks.update(webhookId: 5, req: req)

        XCTAssertEqual(webhook.id, 5)
        XCTAssertEqual(webhook.payloadUrl, "https://hooks.example.com/updated")

        let sentReq = transport.lastRequest!.request
        XCTAssertEqual(sentReq.httpMethod, "PUT")
        XCTAssertTrue(sentReq.url!.absoluteString.hasSuffix("/webhooks/5"))
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
        let comment = try await account.comments.update(commentId: 10, req: req)

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
            _ = try await account.todos.create(todolistId: 2, req: req)
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
        let webhook = try await account.webhooks.create(bucketId: 1, req: req)
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

        let comment = try await account.comments.get(commentId: 7)
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

        let message = try await account.messages.get(messageId: 3)
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

    func testToolsServiceCreatePostsToBucketScopedPath() async throws {
        let responseJSON: [String: Any] = [
            "id": 800,
            "name": "message_board",
            "title": "Message Board (Copy)",
            "enabled": true,
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
        ]
        let data = try JSONSerialization.data(withJSONObject: responseJSON)
        let transport = MockTransport(statusCode: 201, data: data)
        let account = makeTestAccountClient(transport: transport)

        let tool = try await account.tools.create(
            bucketId: 456,
            req: CreateToolRequest(title: "Message Board (Copy)", toolType: "Message::Board")
        )

        XCTAssertEqual(tool.id, 800)
        XCTAssertEqual(tool.title, "Message Board (Copy)")

        let request = transport.lastRequest!.request
        XCTAssertEqual(request.httpMethod, "POST")
        XCTAssertTrue(request.url!.absoluteString.hasSuffix("/buckets/456/dock/tools.json"))

        let sentJSON = try JSONSerialization.jsonObject(with: request.httpBody!) as! [String: Any]
        XCTAssertEqual(sentJSON["tool_type"] as? String, "Message::Board")
        XCTAssertEqual(sentJSON["title"] as? String, "Message Board (Copy)")
    }

    func testToolsServiceCreateOmitsTitleWhenNotProvided() async throws {
        let responseJSON: [String: Any] = [
            "id": 801,
            "name": "message_board",
            "title": "Message Board",
            "enabled": true,
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
        ]
        let data = try JSONSerialization.data(withJSONObject: responseJSON)
        let transport = MockTransport(statusCode: 201, data: data)
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.tools.create(
            bucketId: 456,
            req: CreateToolRequest(toolType: "Message::Board")
        )

        let sentJSON = try JSONSerialization.jsonObject(with: transport.lastRequest!.request.httpBody!) as! [String: Any]
        XCTAssertEqual(sentJSON["tool_type"] as? String, "Message::Board")
        XCTAssertNil(sentJSON["title"])
    }

    // MARK: - Campfire line operations

    private func campfireLineJSON(id: Int, content: String) -> [String: Any] {
        [
            "id": id, "content": content,
            "app_url": "https://3.basecamp.com/1/buckets/1/chats/42/lines/\(id)",
            "url": "https://3.basecampapi.com/1/buckets/1/chats/42/lines/\(id).json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
            "status": "active", "title": "Test line", "type": "Chat::Lines::Text",
            "inherits_status": true, "visible_to_clients": false,
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "parent": ["id": 42, "title": "Campfire", "type": "Chat::Transcript",
                        "app_url": "https://3.basecamp.com/1/buckets/1/chats/42",
                        "url": "https://3.basecampapi.com/1/buckets/1/chats/42.json"] as [String: Any],
        ]
    }

    func testCampfiresServiceCreateLine() async throws {
        let data = try JSONSerialization.data(withJSONObject: campfireLineJSON(id: 300, content: "Hello everyone!"))
        let transport = MockTransport(statusCode: 201, data: data)
        let account = makeTestAccountClient(transport: transport)

        let req = CreateCampfireLineRequest(content: "Hello everyone!")
        let line = try await account.campfires.createLine(campfireId: 42, req: req)

        XCTAssertEqual(line.id, 300)
        XCTAssertEqual(line.content, "Hello everyone!")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "POST")
        XCTAssertTrue(transport.lastRequest!.request.url!.absoluteString.hasSuffix("/chats/42/lines.json"))
    }

    func testCampfiresServiceGetLine() async throws {
        let data = try JSONSerialization.data(withJSONObject: campfireLineJSON(id: 300, content: "Hello everyone!"))
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let line = try await account.campfires.getLine(campfireId: 42, lineId: 300)

        XCTAssertEqual(line.id, 300)
        XCTAssertEqual(line.type, "Chat::Lines::Text")
        XCTAssertTrue(transport.lastRequest!.request.url!.absoluteString.hasSuffix("/chats/42/lines/300"))
    }

    func testCampfiresServiceUpdateLineSendsPUT() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        let req = UpdateCampfireLineRequest(content: "Edited!")
        try await account.campfires.updateLine(campfireId: 42, lineId: 300, req: req)

        let sent = transport.lastRequest!.request
        XCTAssertEqual(sent.httpMethod, "PUT")
        XCTAssertTrue(sent.url!.absoluteString.hasSuffix("/chats/42/lines/300"))

        let sentJSON = try JSONSerialization.jsonObject(with: sent.httpBody!) as! [String: Any]
        XCTAssertEqual(sentJSON["content"] as? String, "Edited!")
        XCTAssertEqual(sentJSON.count, 1, "Body should carry only content")
    }

    func testCampfiresServiceUpdateLine422MapsToValidation() async throws {
        let errorBody = try JSONSerialization.data(withJSONObject: ["error": "Unprocessable"])
        let transport = MockTransport(statusCode: 422, data: errorBody)
        let account = makeTestAccountClient(transport: transport)

        do {
            let req = UpdateCampfireLineRequest(content: "Edited!")
            try await account.campfires.updateLine(campfireId: 42, lineId: 300, req: req)
            XCTFail("Expected 422 error")
        } catch let error as BasecampError {
            if case .validation(let message, let status, _, _) = error {
                XCTAssertEqual(status, 422)
                XCTAssertEqual(message, "Unprocessable")
            } else {
                XCTFail("Expected .validation error, got \(error)")
            }
        }
    }

    func testCampfiresServiceDeleteLine() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.campfires.deleteLine(campfireId: 42, lineId: 300)

        let sent = transport.lastRequest!.request
        XCTAssertEqual(sent.httpMethod, "DELETE")
        XCTAssertTrue(sent.url!.absoluteString.hasSuffix("/chats/42/lines/300"))
    }

    // MARK: - Search array-filter wire encoding + metadata decode

    func testSearchEncodesArrayFiltersAsBracketedKeys() async throws {
        let data = try JSONSerialization.data(withJSONObject: [] as [Any])
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.search.search(
            q: "hello",
            options: SearchSearchOptions(
                typeNames: ["Message", "Todo"],
                bucketIds: [1, 2],
                creatorIds: [7]
            )
        )

        let url = transport.lastRequest!.request.url!
        let items = URLComponents(url: url, resolvingAgainstBaseURL: false)!.queryItems ?? []
        func values(_ name: String) -> [String] {
            items.filter { $0.name == name }.compactMap { $0.value }
        }
        // Rails' permit(bucket_ids: []) only accepts the bracketed repeated form.
        XCTAssertEqual(values("bucket_ids[]"), ["1", "2"])
        XCTAssertEqual(values("type_names[]"), ["Message", "Todo"])
        XCTAssertEqual(values("creator_ids[]"), ["7"])
        // The bare and double-bracketed forms must be absent.
        XCTAssertTrue(values("bucket_ids").isEmpty)
        XCTAssertTrue(values("bucket_ids[][]").isEmpty)
        XCTAssertEqual(values("q"), ["hello"])
    }

    func testSearchEncodesFullFilterSurface() async throws {
        let data = try JSONSerialization.data(withJSONObject: [] as [Any])
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.search.search(
            q: "hello",
            options: SearchSearchOptions(
                typeNames: ["Message"],
                bucketIds: [1, 2],
                creatorIds: [7],
                fileType: "Image",
                excludeChat: true,
                since: "last_30_days",
                sort: "recency",
                type: "Message",
                bucketId: 9,
                creatorId: 3
            )
        )

        let url = transport.lastRequest!.request.url!
        let items = URLComponents(url: url, resolvingAgainstBaseURL: false)!.queryItems ?? []
        func values(_ name: String) -> [String] {
            items.filter { $0.name == name }.compactMap { $0.value }
        }
        func single(_ name: String) -> String? { values(name).first }

        XCTAssertEqual(values("bucket_ids[]"), ["1", "2"])
        XCTAssertEqual(values("type_names[]"), ["Message"])
        XCTAssertEqual(values("creator_ids[]"), ["7"])
        XCTAssertEqual(single("q"), "hello")
        XCTAssertEqual(single("file_type"), "Image")
        XCTAssertEqual(single("exclude_chat"), "true")
        XCTAssertEqual(single("since"), "last_30_days")
        XCTAssertEqual(single("sort"), "recency")
        XCTAssertEqual(single("type"), "Message")
        XCTAssertEqual(single("bucket_id"), "9")
        XCTAssertEqual(single("creator_id"), "3")
    }

    func testSearchMetadataDecodes() async throws {
        let json: [String: Any] = [
            "recording_search_types": [
                ["key": NSNull(), "value": "Everything"],
                ["key": "Message", "value": "Messages"],
            ],
            "file_search_types": [
                ["key": NSNull(), "value": "All files"],
                ["key": "Image", "value": "Images"],
            ],
            "default_creator_label": "Anyone",
            "default_bucket_label": "All projects",
            "default_circle_label": "All pings",
            "default_file_type_label": "All files",
            "default_type_label": "Everything",
        ]
        let data = try JSONSerialization.data(withJSONObject: json)
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let metadata = try await account.search.metadata()

        XCTAssertEqual(metadata.recordingSearchTypes.count, 2)
        // The default "everything" option carries a null key.
        XCTAssertNil(metadata.recordingSearchTypes[0].key)
        XCTAssertEqual(metadata.recordingSearchTypes[1].value, "Messages")
        XCTAssertEqual(metadata.fileSearchTypes[1].key, "Image")
        XCTAssertEqual(metadata.defaultCreatorLabel, "Anyone")
        XCTAssertEqual(metadata.defaultTypeLabel, "Everything")
    }

    // SearchType.key is required-and-nullable: present on the wire, possibly null.
    // The generated custom Codable must (1) accept explicit null, (2) reject a
    // missing key, and (3) re-encode nil as an explicit `"key": null`.
    func testSearchTypeKeyRequiredNullableRoundTrip() throws {
        let decoder = JSONDecoder()

        // (1) explicit null decodes to nil
        let nullKey = try decoder.decode(SearchType.self, from: Data(#"{"key":null,"value":"Everything"}"#.utf8))
        XCTAssertNil(nullKey.key)
        XCTAssertEqual(nullKey.value, "Everything")

        // present key decodes to the value
        let realKey = try decoder.decode(SearchType.self, from: Data(#"{"key":"Message","value":"Messages"}"#.utf8))
        XCTAssertEqual(realKey.key, "Message")

        // (2) a MISSING key is rejected (required presence)
        XCTAssertThrowsError(
            try decoder.decode(SearchType.self, from: Data(#"{"value":"Everything"}"#.utf8))
        ) { error in
            guard case DecodingError.keyNotFound = error else {
                return XCTFail("expected keyNotFound, got \(error)")
            }
        }

        // (3) nil re-encodes as explicit null, not omitted
        let encoder = JSONEncoder()
        let nilEncoded = try encoder.encode(SearchType(key: nil, value: "Everything"))
        let nilObject = try JSONSerialization.jsonObject(with: nilEncoded) as? [String: Any]
        XCTAssertTrue(nilObject?.keys.contains("key") ?? false, "key must be present")
        XCTAssertTrue(nilObject?["key"] is NSNull, "nil key must encode as JSON null")

        // a present key round-trips as the string
        let realEncoded = try encoder.encode(SearchType(key: "Message", value: "Messages"))
        let realObject = try JSONSerialization.jsonObject(with: realEncoded) as? [String: Any]
        XCTAssertEqual(realObject?["key"] as? String, "Message")
    }

    // visibleToClients is tri-state: nil omits the key (encodeIfPresent), true/false
    // are sent verbatim. An explicit false must reach the wire, not be dropped. The
    // shared generator carries this field on all six create ops; this messages
    // coverage stands in for the other five ops.
    private func messageResponseData() throws -> Data {
        let responseJSON: [String: Any] = [
            "id": 99, "subject": "Hello", "content": "<p>Body</p>",
            "app_url": "https://3.basecamp.com/1/buckets/1/messages/99",
            "url": "https://3.basecampapi.com/1/buckets/1/messages/99.json",
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
            "status": "active", "title": "Hello", "type": "Message",
            "inherits_status": false, "visible_to_clients": false,
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "parent": ["id": 4, "title": "Message Board", "type": "Message::Board",
                        "app_url": "https://3.basecamp.com/1/buckets/1/message_boards/4",
                        "url": "https://3.basecampapi.com/1/buckets/1/message_boards/4.json"] as [String: Any],
        ]
        return try JSONSerialization.data(withJSONObject: responseJSON)
    }

    private func sentMessageBody(visibleToClients: Bool?) async throws -> [String: Any] {
        let transport = MockTransport(statusCode: 201, data: try messageResponseData())
        let account = makeTestAccountClient(transport: transport)
        let req = CreateMessageRequest(subject: "Hello", visibleToClients: visibleToClients)
        _ = try await account.messages.create(boardId: 200, req: req)
        let sentBody = transport.lastRequest!.request.httpBody!
        return try JSONSerialization.jsonObject(with: sentBody) as! [String: Any]
    }

    func testCreateMessageOmitsVisibleToClientsWhenNil() async throws {
        let sentJSON = try await sentMessageBody(visibleToClients: nil)
        XCTAssertNil(sentJSON["visible_to_clients"], "nil must omit the key")
    }

    func testCreateMessageSendsVisibleToClientsTrue() async throws {
        let sentJSON = try await sentMessageBody(visibleToClients: true)
        XCTAssertEqual(sentJSON["visible_to_clients"] as? Bool, true)
    }

    func testCreateMessageSendsVisibleToClientsFalse() async throws {
        let sentJSON = try await sentMessageBody(visibleToClients: false)
        XCTAssertNotNil(sentJSON["visible_to_clients"], "explicit false must be sent, not dropped")
        XCTAssertEqual(sentJSON["visible_to_clients"] as? Bool, false)
    }
}
