import XCTest

@testable import Basecamp

/// The completed-subtask body is the shared fixture in `spec/fixtures/subtasks/`,
/// read relative to this source file so it cannot drift from the copy the other
/// SDKs use.
final class SubtasksServiceTests: XCTestCase {
    private func fixture(_ name: String) throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()  // BasecampTests
            .deletingLastPathComponent()  // Tests
            .deletingLastPathComponent()  // swift
            .deletingLastPathComponent()  // <repo root>
        return try Data(
            contentsOf: repoRoot.appendingPathComponent("spec/fixtures/subtasks/\(name).json"))
    }

    func testGetCompletedSubtaskSaysWhoCompletedItAndWhen() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get-completed"))
        let account = makeTestAccountClient(transport: transport)

        let subtask = try await account.subtasks.get(subtaskId: 1069479879)

        XCTAssertEqual(subtask.completed, true)
        let completion = try XCTUnwrap(subtask.completion)
        XCTAssertEqual(completion.createdAt, "2026-07-03T15:42:10.000Z")
        XCTAssertEqual(completion.creator.id, 1049715938)
        XCTAssertEqual(completion.creator.name, "Annie Bryan")
    }

    func testGetIncompleteSubtaskHasNoCompletion() async throws {
        let transport = MockTransport(statusCode: 200, data: try fixture("get"))
        let account = makeTestAccountClient(transport: transport)

        let subtask = try await account.subtasks.get(subtaskId: 1069479879)

        XCTAssertNil(subtask.completion)
    }
}
