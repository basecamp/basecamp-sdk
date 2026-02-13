import XCTest
@testable import Basecamp

final class JSONValueTests: XCTestCase {

    // MARK: - Primitives

    func testDecodesString() throws {
        let value = try decode(#""hello""#)
        XCTAssertEqual(value, .string("hello"))
    }

    func testDecodesNumber() throws {
        let value = try decode("42.5")
        XCTAssertEqual(value, .number(42.5))
    }

    func testDecodesInteger() throws {
        let value = try decode("7")
        XCTAssertEqual(value, .number(7))
    }

    func testDecodesBoolTrue() throws {
        XCTAssertEqual(try decode("true"), .bool(true))
    }

    func testDecodesBoolFalse() throws {
        XCTAssertEqual(try decode("false"), .bool(false))
    }

    func testDecodesNull() throws {
        XCTAssertEqual(try decode("null"), .null)
    }

    // MARK: - Containers

    func testDecodesArray() throws {
        let value = try decode(#"[1, "two", true]"#)
        XCTAssertEqual(value, .array([.number(1), .string("two"), .bool(true)]))
    }

    func testDecodesEmptyArray() throws {
        XCTAssertEqual(try decode("[]"), .array([]))
    }

    func testDecodesObject() throws {
        let value = try decode(#"{"name": "Basecamp", "id": 999}"#)
        XCTAssertEqual(value, .object(["name": .string("Basecamp"), "id": .number(999)]))
    }

    func testDecodesEmptyObject() throws {
        XCTAssertEqual(try decode("{}"), .object([:]))
    }

    // MARK: - Nesting

    func testDecodesNestedStructure() throws {
        let json = #"{"users": [{"name": "DHH", "admin": true}], "count": 1}"#
        let value = try decode(json)

        let expected: JSONValue = .object([
            "users": .array([
                .object(["name": .string("DHH"), "admin": .bool(true)])
            ]),
            "count": .number(1)
        ])
        XCTAssertEqual(value, expected)
    }

    // MARK: - Equatable

    func testDifferentCasesAreNotEqual() {
        XCTAssertNotEqual(JSONValue.string("1"), JSONValue.number(1))
        XCTAssertNotEqual(JSONValue.bool(true), JSONValue.number(1))
        XCTAssertNotEqual(JSONValue.null, JSONValue.bool(false))
    }

    // MARK: - Helpers

    private func decode(_ json: String) throws -> JSONValue {
        try JSONDecoder().decode(JSONValue.self, from: Data(json.utf8))
    }
}
