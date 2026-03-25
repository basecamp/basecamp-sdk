import XCTest
@testable import Basecamp

final class FlexibleIntTests: XCTestCase {
    func testDecodesJSONNumber() throws {
        let json = #"{"id": 12345}"#.data(using: .utf8)!
        let result = try JSONDecoder().decode(Wrapper.self, from: json)
        XCTAssertEqual(result.id.value, 12345)
    }

    func testDecodesNumericString() throws {
        let json = #"{"id": "12345"}"#.data(using: .utf8)!
        let result = try JSONDecoder().decode(Wrapper.self, from: json)
        XCTAssertEqual(result.id.value, 12345)
    }

    func testDecodesNonNumericSentinelAsZero() throws {
        let json = #"{"id": "basecamp"}"#.data(using: .utf8)!
        let result = try JSONDecoder().decode(Wrapper.self, from: json)
        XCTAssertEqual(result.id.value, 0)
    }

    func testRejectsNumericOverflowString() {
        let json = #"{"id": "9223372036854775808"}"#.data(using: .utf8)!
        XCTAssertThrowsError(try JSONDecoder().decode(Wrapper.self, from: json))
    }

    func testEncodesAsNumber() throws {
        let wrapper = Wrapper(id: FlexibleInt(42))
        let data = try JSONEncoder().encode(wrapper)
        let json = String(data: data, encoding: .utf8)!
        XCTAssertTrue(json.contains("42"))
        XCTAssertFalse(json.contains("\"42\""))
    }

    func testIntegerLiteralConformance() {
        let id: FlexibleInt = 42
        XCTAssertEqual(id.value, 42)
    }

    // MARK: - Response normalization boundary tests

    func testNormalizeSentinelCreatorId() throws {
        let json = #"{"creator":{"id":"basecamp","name":"Basecamp","personable_type":"LocalPerson"}}"#.data(using: .utf8)!
        let normalized = BaseService.normalizePersonIds(in: json)
        let parsed = try JSONSerialization.jsonObject(with: normalized) as! [String: Any]
        let creator = parsed["creator"] as! [String: Any]
        XCTAssertEqual(creator["id"] as? Int, 0)
        XCTAssertEqual(creator["system_label"] as? String, "basecamp")
    }

    func testNormalizeNumericStringCreatorId() throws {
        let json = #"{"creator":{"id":"99999","name":"Real","personable_type":"User"}}"#.data(using: .utf8)!
        let normalized = BaseService.normalizePersonIds(in: json)
        let parsed = try JSONSerialization.jsonObject(with: normalized) as! [String: Any]
        let creator = parsed["creator"] as! [String: Any]
        XCTAssertEqual(creator["id"] as? Int, 99999)
        XCTAssertNil(creator["system_label"])
    }

    func testNormalizeOverflowStringCreatorId() throws {
        let json = #"{"creator":{"id":"9223372036854775808","name":"Overflow","personable_type":"User"}}"#.data(using: .utf8)!
        let normalized = BaseService.normalizePersonIds(in: json)
        let parsed = try JSONSerialization.jsonObject(with: normalized) as! [String: Any]
        let creator = parsed["creator"] as! [String: Any]
        // Overflow left as string for FlexibleInt to reject
        XCTAssertTrue(creator["id"] is String)
    }
}

private struct Wrapper: Codable {
    let id: FlexibleInt
}
