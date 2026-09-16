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

    // MARK: - The person-id grammar corpus

    // 74 rows, every expectation a MEASURED Go verdict: a probe linked against
    // the real `go/pkg/types.FlexibleInt64` and the real
    // `normalizeEmbeddedPeopleJSON` produced each one. They are pinned three
    // times over — at `parsePersonID` itself, at the reader, and at the
    // pre-decode normalizer — because the rule is one rule and the whole defect
    // was two sites drifting from it (and from Go) in opposite directions.
    //
    // Rows that exist to DISCRIMINATE, so do not drop them as redundant:
    // `"+7"`/`"+007"` (the sign a `^-?\d+$` regex refuses); `"007"`, `"010"`,
    // `"0009223372036854775807"` (leading zeros — `"010"` is ten, and Ruby's
    // `Integer()` said eight); the Unicode-digit rows (ICU's `\d` is `\p{Nd}`
    // and matched every one of them, so the regex called them overflow where Go
    // reads the sentinel); the whitespace and underscore rows; and
    // `"18446744073709551615x"` against `"18446744073709551616x"` — one digit
    // apart, opposite refusals, because `ParseUint` checks the magnitude inside
    // the scan and the first disqualifying byte wins.

    enum PersonIDExpectation {
        /// Go read the number: the normalizer writes it with no `system_label`,
        /// the reader returns it.
        case value(Int)
        /// `ErrSyntax`: the normalizer writes id `0` and `system_label`, the
        /// reader returns 0.
        case label
        /// `ErrRange`: the normalizer leaves the string, the reader fails.
        case refuse
    }

    // A function, not a stored property, so there is no global state for strict
    // concurrency to reason about.
    private static func personIDCorpus() -> [(input: String, expected: PersonIDExpectation)] {
        [
            ("7", .value(7)),
            ("0", .value(0)),
            ("-0", .value(0)),
            ("+0", .value(0)),
            ("+7", .value(7)),
            ("-7", .value(-7)),
            ("007", .value(7)),
            ("+007", .value(7)),
            ("-007", .value(-7)),
            ("0009223372036854775807", .value(9223372036854775807)),
            ("0000000000000000000000009", .value(9)),
            ("", .label),
            (" ", .label),
            ("+", .label),
            ("-", .label),
            (" 7", .label),
            ("7 ", .label),
            (" 7 ", .label),
            ("\n7", .label),
            ("7\n", .label),
            ("\t7", .label),
            ("7\t", .label),
            ("1_0", .label),
            ("1_2", .label),
            ("0x10", .label),
            ("0b11", .label),
            ("0o17", .label),
            ("010", .value(10)),
            ("0X1F", .label),
            ("7x", .label),
            ("x7", .label),
            ("12.0", .label),
            ("1e3", .label),
            ("12,3", .label),
            ("basecamp", .label),
            ("campfire", .label),
            ("LocalPerson", .label),
            ("\u{FF11}\u{FF12}\u{FF13}", .label),
            ("\u{FF17}", .label),
            ("\u{0660}\u{0661}\u{0662}", .label),
            ("\u{09ED}", .label),
            ("\u{06F7}", .label),
            ("9223372036854775806", .value(9223372036854775806)),
            ("9223372036854775807", .value(9223372036854775807)),
            ("9223372036854775808", .refuse),
            ("9223372036854775809", .refuse),
            ("-9223372036854775807", .value(-9223372036854775807)),
            ("-9223372036854775808", .value(-9223372036854775808)),
            ("-9223372036854775809", .refuse),
            ("18446744073709551614", .refuse),
            ("18446744073709551615", .refuse),
            ("18446744073709551616", .refuse),
            ("18446744073709551615x", .label),
            ("18446744073709551616x", .refuse),
            ("1844674407370955161x", .label),
            ("-18446744073709551615x", .label),
            ("-18446744073709551616x", .refuse),
            ("99999999999999999999999", .refuse),
            ("99999999999999999999999x", .refuse),
            ("00000000000000000000018446744073709551616", .refuse),
            ("0000000000000000000009223372036854775807", .value(9223372036854775807)),
            ("9007199254740991", .value(9007199254740991)),
            ("9007199254740992", .value(9007199254740992)),
            ("9007199254740993", .value(9007199254740993)),
            ("90071992547409931", .value(90071992547409931)),
            ("-9007199254740993", .value(-9007199254740993)),
            ("+9223372036854775807", .value(9223372036854775807)),
            ("+9223372036854775808", .refuse),
            ("00", .value(0)),
            ("0000", .value(0)),
            ("-00", .value(0)),
            ("\u{0660}", .label),
            ("\u{09ED}7", .label),
            ("7\u{09ED}", .label),
        ]
    }

    func testCorpusIsComplete() {
        XCTAssertEqual(Self.personIDCorpus().count, 74)
    }

    /// `JSONSerialization` hands numbers back as `NSNumber`. `as? Int` is the
    /// Darwin bridge and `int64Value` the fallback, so the rows at the far end
    /// of `Int64` are read exactly rather than through a `Double`.
    private static func asInt(_ value: Any?) -> Int? {
        if let i = value as? Int { return i }
        if let n = value as? NSNumber { return Int(exactly: n.int64Value) }
        return nil
    }

    /// The grammar itself, including the distinction no single "is this a
    /// number?" predicate can make.
    func testParsePersonIDMatchesGo() {
        for (input, expected) in Self.personIDCorpus() {
            let reading = parsePersonID(input)
            switch expected {
            case .value(let n):
                XCTAssertEqual(reading, .value(n), "parsePersonID(\(input.debugDescription))")
            case .label:
                XCTAssertEqual(reading, .syntax, "parsePersonID(\(input.debugDescription))")
            case .refuse:
                XCTAssertEqual(reading, .range, "parsePersonID(\(input.debugDescription))")
            }
        }
    }

    /// The reader: `go/pkg/types/flexible_int64.go:34-47`.
    func testFlexibleIntReadsCorpusLikeGo() throws {
        for (input, expected) in Self.personIDCorpus() {
            // Built through JSONSerialization, not string interpolation: `"\n7"`
            // and `"\t7"` are control characters that have to reach the decoder
            // as escapes or the document itself is malformed and every row would
            // "throw" for the wrong reason.
            let payload = try JSONSerialization.data(withJSONObject: ["id": input])
            let context = "id = \(input.debugDescription)"
            switch expected {
            case .value(let n):
                let decoded = try JSONDecoder().decode(Wrapper.self, from: payload)
                XCTAssertEqual(decoded.id.value, n, context)
            case .label:
                let decoded = try JSONDecoder().decode(Wrapper.self, from: payload)
                XCTAssertEqual(decoded.id.value, 0, context)
            case .refuse:
                XCTAssertThrowsError(try JSONDecoder().decode(Wrapper.self, from: payload), context)
            }
        }
    }

    /// The pre-decode normalizer: `go/pkg/basecamp/normalize.go:40-68`.
    func testNormalizePersonIdsReadsCorpusLikeGo() throws {
        for (input, expected) in Self.personIDCorpus() {
            let body: [String: Any] = [
                "creator": ["id": input, "name": "Row", "personable_type": "User"]
            ]
            let payload = try JSONSerialization.data(withJSONObject: body)
            let normalized = BaseService.normalizePersonIds(in: payload)
            let parsed = try XCTUnwrap(
                JSONSerialization.jsonObject(with: normalized) as? [String: Any])
            let creator = try XCTUnwrap(parsed["creator"] as? [String: Any])
            let context = "id = \(input.debugDescription)"

            switch expected {
            case .value(let n):
                XCTAssertEqual(try XCTUnwrap(Self.asInt(creator["id"])), n, context)
                XCTAssertNil(creator["system_label"], context)
            case .label:
                XCTAssertEqual(try XCTUnwrap(Self.asInt(creator["id"])), 0, context)
                XCTAssertEqual(try XCTUnwrap(creator["system_label"] as? String), input, context)
            case .refuse:
                // Left untouched, so the reader is the one that refuses.
                XCTAssertEqual(try XCTUnwrap(creator["id"] as? String), input, context)
                XCTAssertNil(creator["system_label"], context)
            }
        }
    }

    /// The two sites agree row for row. The defect was not that either was
    /// wrong in isolation — it was that they were wrong in *opposite*
    /// directions, so a body the normalizer waved through the reader then threw
    /// on, and vice versa.
    func testNormalizerAndReaderAgree() throws {
        for (input, _) in Self.personIDCorpus() {
            let body: [String: Any] = [
                "creator": ["id": input, "name": "Row", "personable_type": "User"]
            ]
            let payload = try JSONSerialization.data(withJSONObject: body)
            let normalized = BaseService.normalizePersonIds(in: payload)
            let parsed = try XCTUnwrap(
                JSONSerialization.jsonObject(with: normalized) as? [String: Any])
            let creator = try XCTUnwrap(parsed["creator"] as? [String: Any])
            let context = "id = \(input.debugDescription)"

            // Whatever survived normalization must decode, unless normalization
            // deliberately left the out-of-range string behind for the reader.
            guard let idValue = creator["id"] else {
                XCTFail("normalization dropped the id for \(context)")
                continue
            }
            let idPayload = try JSONSerialization.data(withJSONObject: ["id": idValue])
            if idValue is String {
                XCTAssertThrowsError(
                    try JSONDecoder().decode(Wrapper.self, from: idPayload), context)
            } else {
                XCTAssertNoThrow(
                    try JSONDecoder().decode(Wrapper.self, from: idPayload), context)
            }
        }
    }
}

private struct Wrapper: Codable {
    let id: FlexibleInt
}
