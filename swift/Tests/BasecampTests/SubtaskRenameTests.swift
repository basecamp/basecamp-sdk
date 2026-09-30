import Foundation
import XCTest

@testable import Basecamp

/// `CardStep` is the deprecated former name of `Subtask`: a typealias (with a
/// documentation-only deprecation, Swift's signal class), so the old spelling
/// names the same type, decodes the same wire shape, and passes wherever a
/// `Subtask` is taken.
final class SubtaskRenameTests: XCTestCase {
    private let wire = """
        {"id": 100, "status": "active", "visible_to_clients": false,
         "created_at": "2026-07-02T00:23:00Z", "updated_at": "2026-07-02T00:23:00Z",
         "title": "Hero shot on the desk", "inherits_status": true, "type": "Kanban::Step",
         "url": "https://3.basecampapi.com/999/buckets/1/subtasks/100.json",
         "app_url": "https://3.basecamp.com/999/buckets/1/todos/200#__recording_100",
         "parent": {"id": 200, "title": "Shot list", "type": "Todo", "url": "u", "app_url": "a"},
         "bucket": {"id": 1, "name": "The Leto Laptop", "type": "Project"},
         "creator": {"id": 7, "name": "Matt Donahue"}}
        """

    private func idOf(_ subtask: Subtask) -> Int { subtask.id }

    func testCardStepIsTheSameTypeAsSubtask() throws {
        let step = try BaseService.decoder.decode(CardStep.self, from: Data(wire.utf8))
        XCTAssertEqual(step.type, "Kanban::Step")
        XCTAssertEqual(idOf(step), 100)
        XCTAssertTrue(CardStep.self == Subtask.self)
    }
}
