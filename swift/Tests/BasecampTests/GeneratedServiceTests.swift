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
            "star_url": "https://3.basecampapi.com/1/buckets/42/stars.json",
            "bookmarked": true, "starred": false,
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
        XCTAssertEqual(project.starUrl, "https://3.basecampapi.com/1/buckets/42/stars.json")
        // starred implies bookmarked, never the reverse: pinned but unstarred is the discriminating case.
        XCTAssertEqual(project.bookmarked, true)
        XCTAssertEqual(project.starred, false)

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

    func testListRecentProjectsDecodesBookmarkedOnlyProjection() async throws {
        // The recently-visited list is the standard projection plus bookmarked
        // only — the wire omits starred here (BC3 #13043).
        let projects: [[String: Any]] = [
            ["id": 2, "name": "Visited Last", "status": "active",
             "app_url": "https://3.basecamp.com/1/projects/2", "url": "https://3.basecampapi.com/1/projects/2.json",
             "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
             "bookmarked": true],
            ["id": 1, "name": "Visited First", "status": "active",
             "app_url": "https://3.basecamp.com/1/projects/1", "url": "https://3.basecampapi.com/1/projects/1.json",
             "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
             "bookmarked": false],
        ]
        let data = try JSONSerialization.data(withJSONObject: projects)

        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let result = try await account.projects.listRecentProjects()
        XCTAssertEqual(result.map(\.id), [2, 1])
        XCTAssertEqual(result.map(\.bookmarked), [true, false])
        XCTAssertEqual(result.map(\.starred), [Bool?.none, Bool?.none])

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/my/recent_projects.json"), "Unexpected URL \(sentURL)")
    }

    func testRecordProjectVisitSendsPost() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.projects.recordProjectVisit(projectId: 7)

        let req = transport.lastRequest!.request
        XCTAssertEqual(req.httpMethod, "POST")
        XCTAssertTrue(req.url!.absoluteString.hasSuffix("/projects/7/recent_visit.json"))
    }

    // An inaccessible project answers 404; archived/trashed ones still 204.
    func testRecordProjectVisitSurfacesNotFoundError() async throws {
        let transport = MockTransport(statusCode: 404, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.projects.recordProjectVisit(projectId: 999)
            XCTFail("Expected not found error")
        } catch let error as BasecampError {
            XCTAssertEqual(error.httpStatusCode, 404)
        }
    }

    func testTrashProjectSendsDelete() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.projects.trash(projectId: 7)

        let req = transport.lastRequest!.request
        XCTAssertEqual(req.httpMethod, "DELETE")
        XCTAssertTrue(req.url!.absoluteString.hasSuffix("/projects/7"))
    }

    func testArchiveProjectSendsPut() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.projects.archive(projectId: 7)

        let req = transport.lastRequest!.request
        XCTAssertEqual(req.httpMethod, "PUT")
        XCTAssertTrue(req.url!.absoluteString.hasSuffix("/projects/7/status/archived.json"))
    }

    func testUnarchiveProjectSendsPut() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.projects.unarchive(projectId: 7)

        let req = transport.lastRequest!.request
        XCTAssertEqual(req.httpMethod, "PUT")
        XCTAssertTrue(req.url!.absoluteString.hasSuffix("/projects/7/status/active.json"))
    }

    func testSpotlightRecordingSendsPostAndDecodesRecording() async throws {
        let json: [String: Any] = [
            "id": 456, "status": "active", "visible_to_clients": false,
            "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
            "title": "Launch", "inherits_status": true, "type": "Message",
            "url": "https://3.basecampapi.com/1/buckets/1/messages/456.json",
            "app_url": "https://3.basecamp.com/1/buckets/1/messages/456",
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
        ]
        let data = try JSONSerialization.data(withJSONObject: json)
        let transport = MockTransport(statusCode: 201, data: data)
        let account = makeTestAccountClient(transport: transport)

        let recording = try await account.recordings.spotlight(recordingId: 456)
        XCTAssertEqual(recording.id, 456)
        XCTAssertEqual(recording.type, "Message")

        let request = transport.lastRequest!.request
        XCTAssertEqual(request.httpMethod, "POST")
        XCTAssertTrue(request.url!.absoluteString.hasSuffix("/recordings/456/spotlight.json"))
    }

    func testSpotlightRecordingSurfacesValidationError() async throws {
        let transport = MockTransport(statusCode: 422, data: Data(#"{"errors":["Recording cannot be spotlighted"]}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.recordings.spotlight(recordingId: 456)
            XCTFail("Expected validation error")
        } catch let error as BasecampError {
            XCTAssertEqual(error.httpStatusCode, 422)
        }
    }

    func testUnspotlightRecordingSendsDelete() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.recordings.unspotlight(recordingId: 456)

        let request = transport.lastRequest!.request
        XCTAssertEqual(request.httpMethod, "DELETE")
        XCTAssertTrue(request.url!.absoluteString.hasSuffix("/recordings/456/spotlight.json"))
    }

    func testUnspotlightRecordingSurfacesForbiddenError() async throws {
        let transport = MockTransport(statusCode: 403, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.recordings.unspotlight(recordingId: 456)
            XCTFail("Expected forbidden error")
        } catch let error as BasecampError {
            XCTAssertEqual(error.httpStatusCode, 403)
        }
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
            "id": 10, "content": "Updated comment", "content_attachments": [],
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
        } catch let error as BasecampError {
            // Since #604 a body that does not decode is the SPEC §6
            // malformed-2xx-body shape rather than a raw `DecodingError`. This
            // used to accept any error at all, which asserted nothing.
            let message = assertStatuslessDecodeFailure(error)
            XCTAssertTrue(
                message.contains("GetProject returned a body that does not decode"), message)
        } catch {
            XCTFail("expected BasecampError, got a raw \(type(of: error)): \(error)")
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
            if case .validation(let message, let status, _, _, _) = error {
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

    // ListWebhooks is a bucket-scoped collection op: operation metadata carries
    // projectId == bucket id and NO resourceId (there's no deeper target id).
    func testListWebhooksEmitsProjectScopeWithoutResourceId() async throws {
        let spy = SpyHooks()
        let data = try JSONSerialization.data(withJSONObject: [] as [Any])
        let transport = MockTransport(statusCode: 200, data: data, headers: ["X-Total-Count": "0"])
        let account = makeTestAccountClient(transport: transport, hooks: spy)

        _ = try await account.webhooks.list(bucketId: 7)

        XCTAssertEqual(spy.operationStarts.count, 1)
        let info = spy.operationStarts.first!
        XCTAssertEqual(info.operation, "ListWebhooks")
        XCTAssertEqual(info.projectId, 7, "projectId should be the bucket id")
        XCTAssertNil(info.resourceId, "collection op has no deeper resource id")
    }

    /// A full todolist body as BC3 renders it — FLAT, no `todolist` envelope
    /// (see spec/fixtures/todolists/get.json). An empty `[:]` would no longer
    /// do: `GetTodolistOrGroup` decodes into `Todolist`, whose required members
    /// reject a body that is not one instead of quietly producing an empty
    /// value (#544).
    private func todolistWireJSON(id: Int = 42) -> [String: Any] {
        [
            "id": id,
            "status": "active",
            "visible_to_clients": false,
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
            "title": "Hardware",
            "inherits_status": true,
            "type": "Todolist",
            "url": "https://3.basecampapi.com/999999999/buckets/1/todolists/\(id).json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/todolists/\(id)",
            "bookmark_url": "https://3.basecampapi.com/999999999/my/bookmarks/abc123.json",
            "subscription_url": "https://3.basecampapi.com/999999999/buckets/1/recordings/\(id)/subscription.json",
            "bubble_up_url": "https://3.basecampapi.com/999999999/buckets/1/recordings/\(id)/bubble_up.json",
            "comments_count": 0,
            "comments_url": "https://3.basecampapi.com/999999999/buckets/1/recordings/\(id)/comments.json",
            "position": 1,
            "parent": [
                "id": 3, "title": "To-dos", "type": "Todoset",
                "url": "https://3.basecampapi.com/999999999/buckets/1/todosets/3.json",
                "app_url": "https://3.basecamp.com/999999999/buckets/1/todosets/3",
            ] as [String: Any],
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "description": "<p>Ship the hardware</p>",
            "description_attachments": [],
            "completed": false,
            "completed_ratio": "0/3",
            "name": "Hardware",
            "todos_url": "https://3.basecampapi.com/999999999/buckets/1/todolists/\(id)/todos.json",
            "groups_url": "https://3.basecampapi.com/999999999/buckets/1/todolists/\(id)/groups.json",
            "app_todos_url": "https://3.basecamp.com/999999999/buckets/1/todolists/\(id)/todos",
            // Both required: the jbuilder emits color in both branches of its
            // todolist_group? conditional and comments_app_url from a route
            // helper. color is required-and-nullable — null when unset.
            "color": "blue",
            "comments_app_url": "https://3.basecamp.com/999999999/buckets/1/recordings/\(id)/comments",
        ]
    }

    // GetTodolistOrGroup's path label is the unsuffixed `{id}`; the resource
    // selector must still pick it up (endsWith("Id") OR == "id").
    func testGetTodolistOrGroupEmitsTodolistIdAsResourceId() async throws {
        let spy = SpyHooks()
        let data = try JSONSerialization.data(withJSONObject: todolistWireJSON())
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport, hooks: spy)

        _ = try await account.todolists.get(id: 42)

        XCTAssertEqual(spy.operationStarts.count, 1)
        let info = spy.operationStarts.first!
        XCTAssertEqual(info.operation, "GetTodolistOrGroup")
        XCTAssertEqual(info.resourceId, 42, "resourceId should be the todolist id")
    }

    func testUpdateTodolistOrGroupEmitsTodolistIdAsResourceId() async throws {
        let spy = SpyHooks()
        let data = try JSONSerialization.data(withJSONObject: todolistWireJSON())
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport, hooks: spy)

        _ = try await account.todolists.replace(id: 42, req: UpdateTodolistOrGroupRequest(name: "Updated list"))

        XCTAssertEqual(spy.operationStarts.count, 1)
        let info = spy.operationStarts.first!
        XCTAssertEqual(info.operation, "UpdateTodolistOrGroup")
        XCTAssertEqual(info.resourceId, 42, "resourceId should be the todolist id")
    }

    func testCommentsServiceGet() async throws {
        let responseJSON: [String: Any] = [
            "id": 7, "content": "Great idea!", "content_attachments": [],
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
            "id": 3, "subject": "Weekly Update", "content": "Here's what happened...", "content_attachments": [],
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

    /// A dock tool's projection is the bare recordings/recording partial —
    /// api/docks/tools/show.json.jbuilder renders it and adds nothing — so it
    /// carries no `name` and no `enabled`, and it does carry `type`,
    /// `visible_to_clients`, `inherits_status` and `creator` (#650).
    private func toolResponseJSON(id: Int, title: String, type: String = "Message::Board") -> [String: Any] {
        [
            "id": id,
            "status": "active",
            "visible_to_clients": false,
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
            "title": title,
            "inherits_status": true,
            "type": type,
            "url": "https://3.basecampapi.com/999999999/buckets/456/recordings/\(id).json",
            "app_url": "https://3.basecamp.com/999999999/buckets/456/recordings/\(id)",
            "bookmark_url": "https://3.basecampapi.com/999999999/my/bookmarks/BAh7Bkki--\(id).json",
            "position": 5,
            "bucket": ["id": 456, "name": "The Leto Laptop", "type": "Project"] as [String: Any],
            "creator": ["id": 1049715913, "name": "Victor Cooper"] as [String: Any],
        ]
    }

    func testToolsServiceCreatePostsToBucketScopedPath() async throws {
        // The bare recordings/recording projection a dock tool actually returns:
        // no `name`, no `enabled` (#650).
        let responseJSON: [String: Any] = toolResponseJSON(id: 800, title: "Message Board (Copy)")
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
        let responseJSON: [String: Any] = toolResponseJSON(id: 801, title: "Message Board")
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

    // visibleToClients is tri-state: nil omits the key (encodeIfPresent), true/false
    // are sent verbatim. An explicit false must reach the wire, not be dropped. Only
    // Chat::Transcript and Kanban::Board honor it; all other tool types ignore it.
    private func sentToolBody(visibleToClients: Bool?) async throws -> [String: Any] {
        let responseJSON = toolResponseJSON(id: 802, title: "Campfire", type: "Chat::Transcript")
        let transport = MockTransport(statusCode: 201, data: try JSONSerialization.data(withJSONObject: responseJSON))
        let account = makeTestAccountClient(transport: transport)
        _ = try await account.tools.create(
            bucketId: 456,
            req: CreateToolRequest(toolType: "Chat::Transcript", visibleToClients: visibleToClients)
        )
        return try JSONSerialization.jsonObject(with: transport.lastRequest!.request.httpBody!) as! [String: Any]
    }

    func testToolsServiceCreateOmitsVisibleToClientsWhenNil() async throws {
        let sentJSON = try await sentToolBody(visibleToClients: nil)
        XCTAssertNil(sentJSON["visible_to_clients"], "nil must omit the key")
    }

    func testToolsServiceCreateSendsVisibleToClientsTrue() async throws {
        let sentJSON = try await sentToolBody(visibleToClients: true)
        XCTAssertEqual(sentJSON["visible_to_clients"] as? Bool, true)
    }

    func testToolsServiceCreateSendsVisibleToClientsFalse() async throws {
        let sentJSON = try await sentToolBody(visibleToClients: false)
        XCTAssertNotNil(sentJSON["visible_to_clients"], "explicit false must be sent, not dropped")
        XCTAssertEqual(sentJSON["visible_to_clients"] as? Bool, false)
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

    func testCampfiresServiceGetChatbotNonAdminOmitsBothAdminOnlyURLs() async throws {
        // command_url and lines_url are admin-only in responses: a non-admin
        // requester gets neither key at all (absent, not null).
        let json: [String: Any] = [
            "id": 300,
            "created_at": "2022-11-22T08:25:04.466Z",
            "updated_at": "2022-11-22T08:25:04.466Z",
            "service_name": "Capistrano",
            "url": "https://3.basecampapi.com/12345/buckets/100/chats/200/integrations/300.json",
            "app_url": "https://3.basecamp.com/12345/buckets/100/chats/200/integrations/300",
        ]
        let data = try JSONSerialization.data(withJSONObject: json)
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let chatbot = try await account.campfires.getChatbot(bucketId: 100, campfireId: 200, chatbotId: 300)

        XCTAssertEqual(chatbot.id, 300)
        XCTAssertEqual(chatbot.serviceName, "Capistrano")
        XCTAssertNil(chatbot.commandUrl)
        XCTAssertNil(chatbot.linesUrl)
        XCTAssertTrue(transport.lastRequest!.request.url!.absoluteString.hasSuffix("/buckets/100/chats/200/integrations/300"))
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
            if case .validation(let message, let status, _, _, _) = error {
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
            "id": 99, "subject": "Hello", "content": "<p>Body</p>", "content_attachments": [],
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

    // MARK: - Activity timeline additive fields (avatars_sample, data, heterogeneous attachments)

    // Proves runtime decode of the additive TimelineEvent fields through the full
    // service lifecycle (.convertFromSnakeCase): a populated avatars_sample, a
    // schedule-entry `data` payload with all-day date-only bounds, and BOTH
    // attachment shapes — a full Upload recording and a rich-text blob partial —
    // in a single heterogeneous array, each carrying real per-variant fields.
    func testProjectTimelineDecodesAdditiveFields() async throws {
        let fixture = """
        [
          {
            "id": 1,
            "created_at": "2024-03-15T10:30:00Z",
            "kind": "chat_transcript_rollup",
            "avatars_sample": [
              "https://3.basecampapi.com/1/people/aaa/avatar",
              "https://3.basecampapi.com/1/people/bbb/avatar"
            ]
          },
          {
            "id": 2,
            "created_at": "2024-03-15T10:31:00Z",
            "kind": "schedule_entry_created",
            "avatars_sample": [],
            "data": {
              "all_day": true,
              "starts_at": "2025-10-30",
              "ends_at": "2025-10-30"
            }
          },
          {
            "id": 3,
            "created_at": "2024-03-15T10:32:00Z",
            "kind": "upload_created",
            "avatars_sample": [],
            "attachments": [
              {
                "id": 900,
                "type": "Upload",
                "status": "active",
                "visible_to_clients": false,
                "title": "Diagram",
                "filename": "diagram.png",
                "content_type": "image/png",
                "byte_size": 20480,
                "width": 1024.0,
                "height": 768.0,
                "url": "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
                "app_url": "https://3.basecamp.com/1/buckets/2/uploads/900",
                "download_url": "https://3.basecampapi.com/1/buckets/2/uploads/900/download/diagram.png",
                "app_download_url": "https://3.basecamp.com/1/buckets/2/uploads/900/download"
              }
            ]
          },
          {
            "id": 4,
            "created_at": "2024-03-15T10:33:00Z",
            "kind": "comment_created",
            "avatars_sample": [],
            "attachments": [
              {
                "id": 500,
                "attachable_sgid": "sgid-attachable-500",
                "sgid": "sgid-500",
                "status_url": "https://3.basecampapi.com/1/attachments/sgid-500/status.json",
                "caption": "See attached",
                "filename": "notes.pdf",
                "content_type": "application/pdf",
                "byte_size": 4096,
                "key": "blobkey500",
                "width": null,
                "height": null,
                "previewable": true,
                "download_url": "https://3.basecampapi.com/1/blobs/blobkey500/download/notes.pdf",
                "preview_url": "https://3.basecampapi.com/1/blobs/blobkey500/previews/full",
                "thumbnail_url": "https://3.basecampapi.com/1/blobs/blobkey500/previews/card"
              }
            ]
          },
          {
            "id": 5,
            "created_at": "2024-03-15T10:34:00Z",
            "kind": "schedule_entry_created",
            "avatars_sample": [],
            "data": {
              "all_day": true,
              "starts_at": null,
              "ends_at": null
            }
          }
        ]
        """
        let transport = MockTransport(statusCode: 200, data: Data(fixture.utf8),
                                      headers: ["X-Total-Count": "5"])
        let account = makeTestAccountClient(transport: transport)

        let events = try await account.timeline.projectTimeline(projectId: 2)
        XCTAssertEqual(events.count, 5)

        // events[0]: populated avatars_sample
        XCTAssertEqual(events[0].avatarsSample?.count, 2)

        // events[1]: schedule-entry data payload with all-day date-only bounds
        XCTAssertNotNil(events[1].data)
        XCTAssertEqual(events[1].data?.allDay, true)
        XCTAssertEqual(events[1].data?.startsAt, "2025-10-30")
        XCTAssertEqual(events[1].data?.endsAt, "2025-10-30")

        // events[2]: full Upload recording attachment
        XCTAssertEqual(events[2].attachments?.count, 1)
        let upload = events[2].attachments![0]
        XCTAssertEqual(upload.type, "Upload")
        XCTAssertEqual(upload.filename, "diagram.png")
        XCTAssertNotNil(upload.appDownloadUrl)
        XCTAssertEqual(upload.width, 1024)

        // events[3]: rich-text attachment/blob partial (distinct per-variant fields)
        XCTAssertEqual(events[3].attachments?.count, 1)
        let blob = events[3].attachments![0]
        XCTAssertEqual(blob.attachableSgid, "sgid-attachable-500")
        XCTAssertEqual(blob.caption, "See attached")
        XCTAssertEqual(blob.key, "blobkey500")
        XCTAssertEqual(blob.previewable, true)
        XCTAssertNil(blob.width)

        // events[4]: schedule-entry with JSON null bounds — required-and-nullable,
        // so the event decodes and the bounds are nil (not a decode failure).
        XCTAssertNotNil(events[4].data)
        XCTAssertEqual(events[4].data?.allDay, true)
        XCTAssertNil(events[4].data?.startsAt)
        XCTAssertNil(events[4].data?.endsAt)
    }

    // MARK: - GetBubbleUps (paginated bare-array decode)

    // Bubble-ups return a bare array of Notification. Proves the full decode path
    // (.convertFromSnakeCase) maps bubble_up_at → bubbleUpAt and carries type/title.
    func testBubbleUpsDecodesNotifications() async throws {
        let fixture = """
        [
          {
            "id": 2,
            "created_at": "2026-07-21T00:01:43.009Z",
            "updated_at": "2026-07-21T00:01:43.031Z",
            "section": "bubbles",
            "unread_count": 0,
            "read_at": "2026-07-21T00:01:43.031Z",
            "title": "We won Leto!",
            "type": "Message",
            "bucket_name": "The Leto Laptop"
          },
          {
            "id": 3,
            "created_at": "2026-07-21T00:02:00.000Z",
            "updated_at": "2026-07-21T00:02:00.000Z",
            "section": "bubbles",
            "unread_count": 1,
            "title": "Scheduled follow-up",
            "type": "Todo",
            "bubble_up_at": "2026-08-01T00:00:00Z"
          }
        ]
        """
        let transport = MockTransport(statusCode: 200, data: Data(fixture.utf8),
                                      headers: ["X-Total-Count": "2"])
        let account = makeTestAccountClient(transport: transport)

        let bubbleUps = try await account.myNotifications.bubbleUps()

        XCTAssertEqual(bubbleUps.count, 2)
        XCTAssertEqual(bubbleUps[0].id, 2)
        XCTAssertEqual(bubbleUps[0].title, "We won Leto!")
        XCTAssertEqual(bubbleUps[0].type, "Message")
        XCTAssertEqual(bubbleUps[1].bubbleUpAt, "2026-08-01T00:00:00Z")

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/my/readings/bubble_ups.json"), "Got \(sentURL)")
    }

    // MARK: - Everything /files.json heterogeneous feed

    // Proves runtime decode of the /files.json "everything" feed through the full
    // service lifecycle (.convertFromSnakeCase): a single non-empty array carrying
    // three distinct variants — a full Upload recording, a Basecamp Document
    // recording, and a rich-text Attachment envelope — each asserted on real
    // per-variant fields. The Upload width is float-spelled (1024.0) on the wire;
    // Foundation's JSONDecoder folds that into the model's Int32 field.
    func testEverythingFilesDecodesHeterogeneousFeed() async throws {
        let fixture = """
        [
          {
            "id": 900,
            "type": "Upload",
            "status": "active",
            "visible_to_clients": false,
            "title": "logo.png",
            "inherits_status": true,
            "filename": "logo.png",
            "content_type": "image/png",
            "byte_size": 1281,
            "width": 1024.0,
            "height": 768.0,
            "url": "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
            "app_url": "https://3.basecamp.com/1/buckets/2/uploads/900",
            "download_url": "https://3.basecampapi.com/1/buckets/2/uploads/900/download/logo.png",
            "app_download_url": "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png",
            "bucket": { "id": 2, "name": "The Leto Laptop", "type": "Project" },
            "creator": { "id": 1, "name": "Victor Cooper" }
          },
          {
            "id": 901,
            "type": "Document",
            "status": "active",
            "visible_to_clients": false,
            "title": "Spec",
            "inherits_status": true,
            "content_type": "text/html",
            "url": "https://3.basecampapi.com/1/buckets/2/documents/901.json",
            "app_url": "https://3.basecamp.com/1/buckets/2/documents/901",
            "bucket": { "id": 2, "name": "The Leto Laptop", "type": "Project" },
            "creator": { "id": 1, "name": "Victor Cooper" }
          },
          {
            "id": 902,
            "type": "Attachment",
            "attachable_sgid": "sgid-902",
            "filename": "chart.avif",
            "content_type": "image/avif",
            "byte_size": 4096,
            "width": null,
            "height": null,
            "download_url": "https://storage.3.basecamp.com/1/blobs/902/download/chart.avif",
            "parent": {
              "id": 800,
              "title": "A message",
              "type": "Message",
              "app_url": "https://3.basecamp.com/1/buckets/2/messages/800",
              "url": "https://3.basecampapi.com/1/buckets/2/messages/800.json"
            }
          }
        ]
        """
        let transport = MockTransport(statusCode: 200, data: Data(fixture.utf8),
                                      headers: ["X-Total-Count": "3"])
        let account = makeTestAccountClient(transport: transport)

        let files = try await account.everything.everythingFiles()
        XCTAssertEqual(files.count, 3)

        // [0]: full Upload recording (float-spelled width folds into Int32)
        XCTAssertEqual(files[0].type, "Upload")
        XCTAssertEqual(files[0].filename, "logo.png")
        XCTAssertNotNil(files[0].appDownloadUrl)
        XCTAssertEqual(files[0].width, 1024)

        // [1]: Basecamp Document recording
        XCTAssertEqual(files[1].type, "Document")
        XCTAssertEqual(files[1].title, "Spec")

        // [2]: rich-text Attachment envelope
        XCTAssertEqual(files[2].type, "Attachment")
        XCTAssertEqual(files[2].attachableSgid, "sgid-902")
        XCTAssertNotNil(files[2].parent)
        XCTAssertNil(files[2].width)

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/files.json"), "Got \(sentURL)")
    }
}

/// Records operation-start callbacks so tests can assert emitted metadata.
private final class SpyHooks: BasecampHooks, @unchecked Sendable {
    private let lock = NSLock()
    private var _operationStarts: [OperationInfo] = []

    var operationStarts: [OperationInfo] {
        lock.withLock { _operationStarts }
    }

    func onOperationStart(_ info: OperationInfo) {
        lock.withLock { _operationStarts.append(info) }
    }
}
