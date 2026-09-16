import Foundation
import XCTest

@testable import Basecamp

/// Person shapes the reference reads, and so writes back.
///
/// Go decodes every embedded person through `generated.Person`, whose `Id` is
/// `types.FlexibleInt64`. Two shapes that are not id grammar follow from that:
///
/// - an **absent** `id` is the zero value `0`, with no error — the flexible
///   reader is only called for a key that is there — while an explicit
///   `"id": null` reaches the reader and fails the read;
/// - a **`null` element** of a `[]Person` is the zero `Person`, contributing id `0`.
///
/// A merge-safe write reads those lists back, so the reference sends `0` for
/// either. Both are generated-model optionality here (`ModelEmitter.swift`),
/// keyed on the flexible id: a person type whose id is a plain `int64` in Go
/// stays strict.
final class PersonShapesTests: XCTestCase {

    private func todoJSON(assignees: String, completionSubscribers: String = "[]") -> String {
        """
        {"id": 456, "status": "active", "visible_to_clients": false,
         "created_at": "2024-01-15T10:00:00Z", "updated_at": "2024-01-15T10:00:00Z",
         "title": "Buy milk", "inherits_status": true, "type": "Todo",
         "url": "https://3.basecampapi.com/999/buckets/1/todos/456.json",
         "app_url": "https://3.basecamp.com/999/buckets/1/todos/456",
         "bookmark_url": "https://3.basecampapi.com/999/my/bookmarks/abc123.json",
         "content": "Buy milk", "description": "<p>From the store</p>",
         "completed": false, "comments_count": 0, "position": 1,
         "parent": {"id": 2, "title": "Todolist", "type": "Todolist",
                    "url": "https://3.basecampapi.com/999/buckets/1/todolists/2.json",
                    "app_url": "https://3.basecamp.com/999/buckets/1/todolists/2"},
         "bucket": {"id": 1, "name": "Project", "type": "Project"},
         "creator": {"id": 1, "name": "Test User"},
         "assignees": \(assignees),
         "completion_subscribers": \(completionSubscribers),
         "description_attachments": []}
        """
    }

    private func decode<T: Decodable>(_ type: T.Type, _ json: String) throws -> T {
        try BaseService.decoder.decode(type, from: Data(json.utf8))
    }

    private func assigneeIds(_ assignees: String) throws -> [Int] {
        try decode(Todo.self, todoJSON(assignees: assignees)).assignees?.map { $0.id.value } ?? []
    }

    func testAnAbsentPersonIdIsZeroAndANullOneFailsTheRead() throws {
        XCTAssertEqual(try decode(Person.self, #"{"name": "A"}"#).id, 0)
        XCTAssertThrowsError(try decode(Person.self, #"{"id": null, "name": "A"}"#))
        XCTAssertThrowsError(try decode(Person.self, #"{"id": null, "name": "A", "personable_type": "User"}"#))
        // The explicit coding keeps everything the synthesized one read.
        let person = try decode(Person.self, #"{"id": "7", "name": "A", "email_address": "a@example.com"}"#)
        XCTAssertEqual(person.id, 7)
        XCTAssertEqual(person.emailAddress, "a@example.com")
    }

    func testANullElementInAPersonListIsTheZeroPerson() throws {
        XCTAssertEqual(try assigneeIds("[null]"), [0])
        XCTAssertEqual(try assigneeIds(#"[null, {"id": 7, "name": "A"}]"#), [0, 7])
        XCTAssertEqual(try assigneeIds(#"[{"id": 7, "name": "A"}, null]"#), [7, 0])
        XCTAssertEqual(try assigneeIds(#"[{"name": "A"}, {"id": "7", "name": "B"}]"#), [0, 7])
        XCTAssertEqual(try decode(Todo.self, todoJSON(assignees: "[null]")).assignees?.first?.name, "")
    }

    func testOnlyTheElementIsLenient() throws {
        // A null or absent list is still no list, not a list of one zero person.
        XCTAssertNil(try decode(Todo.self, todoJSON(assignees: "null")).assignees)
        let absent = todoJSON(assignees: "[]").replacingOccurrences(of: #""assignees": [],"#, with: "")
        XCTAssertNil(try decode(Todo.self, absent).assignees)

        // An element that is not an object, or whose id is an explicit null, still fails.
        XCTAssertThrowsError(try assigneeIds("[5]"))
        XCTAssertThrowsError(try assigneeIds(#"[{"id": null, "name": "A"}]"#))
        XCTAssertThrowsError(try assigneeIds(#"{"id": 7}"#))
    }

    /// `MyAssignmentAssignee.ID`, `UpcomingSchedulePerson.ID` and
    /// `OutOfOfficePerson.ID` are plain `int64` in Go, not the flexible reader:
    /// the rule above is the flexible id's, and does not reach them.
    func testAPersonTypeWithoutTheFlexibleIdStaysStrict() throws {
        XCTAssertThrowsError(try decode(MyAssignmentAssignee.self, #"{"name": "A", "avatar_url": ""}"#))
        XCTAssertThrowsError(try decode(UpcomingSchedulePerson.self, #"{"name": "A", "avatar_url": ""}"#))
        XCTAssertThrowsError(try decode(OutOfOfficePerson.self, #"{"name": "A", "avatar_url": ""}"#))
        XCTAssertNoThrow(try decode(MyAssignment.self, #"{"id": 1, "assignees": [{"id": 7, "name": "A", "avatar_url": ""}]}"#))
        XCTAssertThrowsError(try decode(MyAssignment.self, #"{"id": 1, "assignees": [null]}"#))

        func entry(participants: String) -> String {
            """
            {"id": 1, "all_day": false, "app_url": "", "bucket": {"id": 1, "name": "P"},
             "comments_count": 0, "creator": {"id": 7, "name": "A", "avatar_url": ""},
             "starts_at": "", "ends_at": "", "recurring": false, "status": "active",
             "summary": "", "type": "Schedule::Entry", "url": "", "visible_to_clients": false,
             "participants": \(participants)}
            """
        }
        XCTAssertNoThrow(try decode(UpcomingScheduleEntry.self, entry(participants: #"[{"id": 7, "name": "A", "avatar_url": ""}]"#)))
        XCTAssertThrowsError(try decode(UpcomingScheduleEntry.self, entry(participants: "[null]")))
    }

    /// Through the merge-safe composite: the reference sends `0` for both shapes.
    func testAMergeSafeUpdateWritesBackZeroForAnAbsentIdAndANullElement() async throws {
        let read = todoJSON(
            assignees: #"[{"name": "A"}, null, {"id": 7, "name": "B"}]"#,
            completionSubscribers: "[null]")
        let written = todoJSON(assignees: "[]")
        let recorder = PutRecorder()
        let transport = MockTransport { request in
            recorder.record(request)
            let body = request.httpMethod == "PUT" ? written : read
            return (
                Data(body.utf8),
                makeHTTPResponse(
                    url: request.url!.absoluteString, statusCode: 200,
                    headers: ["Content-Type": "application/json"])
            )
        }
        let account = makeTestAccountClient(transport: transport)

        _ = try await account.todos.update(todoId: 456, req: UpdateTodoRequest(content: "x"))

        let body = try XCTUnwrap(recorder.body)
        XCTAssertEqual(body["assignee_ids"] as? [Int], [0, 0, 7])
        XCTAssertEqual(body["completion_subscriber_ids"] as? [Int], [0])
    }
}

private final class PutRecorder: @unchecked Sendable {
    private let lock = NSLock()
    private var captured: [String: Any]?

    var body: [String: Any]? {
        lock.lock()
        defer { lock.unlock() }
        return captured
    }

    func record(_ request: URLRequest) {
        guard request.httpMethod == "PUT", let data = request.httpBody,
            let object = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
        else { return }
        lock.lock()
        defer { lock.unlock() }
        captured = object
    }
}
