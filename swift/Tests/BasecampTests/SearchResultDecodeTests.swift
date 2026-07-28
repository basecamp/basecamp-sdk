import XCTest
@testable import Basecamp

/// `SearchResult.content` and `.description` are **required and nullable**: the
/// search projection in `api/searches/show.json.jbuilder` renders the
/// recording's own partial and then unconditionally overwrites both with `nil`
/// to keep the large HTML body out of the payload. So the keys are guaranteed
/// present and their values guaranteed null.
///
/// That is a different contract from optional-and-nullable, and the difference
/// is observable: a payload that *omits* the keys must fail to decode. These
/// tests pin both halves, so a regeneration that relaxed `@required` — or that
/// made the fields optional — would fail here rather than silently widening the
/// type.
final class SearchResultDecodeTests: XCTestCase {
    /// Mirrors the key strategies `BaseService` configures, so these tests
    /// exercise the same decode path the SDK actually uses.
    private func makeDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return decoder
    }

    private func makeEncoder() -> JSONEncoder {
        let encoder = JSONEncoder()
        encoder.keyEncodingStrategy = .convertToSnakeCase
        return encoder
    }

    /// The minimum required key set, less content/description, so each test can
    /// vary only the fields under test.
    private func payload(extraKeys: String) -> Data {
        """
        {
          "id": 1069479351,
          "title": "We won Leto!",
          "type": "Message",
          "url": "https://3.basecampapi.com/195539477/buckets/1/messages/1069479351.json",
          "app_url": "https://3.basecamp.com/195539477/buckets/1/messages/1069479351"
          \(extraKeys)
        }
        """.data(using: .utf8)!
    }

    func testDecodesNullContentAndDescription() throws {
        let result = try makeDecoder().decode(
            SearchResult.self,
            from: payload(extraKeys: #", "content": null, "description": null"#)
        )

        XCTAssertNil(result.content)
        XCTAssertNil(result.description)
    }

    func testRejectsOmittedContent() {
        // Optional-and-nullable would accept this. Required-and-nullable must not.
        XCTAssertThrowsError(
            try makeDecoder().decode(SearchResult.self, from: payload(extraKeys: #", "description": null"#))
        ) { error in
            guard case DecodingError.keyNotFound(let key, _) = error else {
                return XCTFail("expected a keyNotFound error for `content`, got \(error)")
            }
            XCTAssertEqual(key.stringValue, "content")
        }
    }

    func testRejectsOmittedDescription() {
        XCTAssertThrowsError(
            try makeDecoder().decode(SearchResult.self, from: payload(extraKeys: #", "content": null"#))
        ) { error in
            guard case DecodingError.keyNotFound(let key, _) = error else {
                return XCTFail("expected a keyNotFound error for `description`, got \(error)")
            }
            XCTAssertEqual(key.stringValue, "description")
        }
    }

    /// The plain-text excerpts are the opposite contract: optional and
    /// non-nullable. A Message carries `plain_text_content` and omits
    /// `plain_text_description` entirely rather than sending null.
    func testPlainTextFieldsAreOptionalAndOmittedWhenAbsent() throws {
        let excerpt = #"Hello everyone! We got the <mark class=\"circled-text\"><span></span>Leto</mark> Laptop project!"#
        let result = try makeDecoder().decode(
            SearchResult.self,
            from: payload(extraKeys: #", "content": null, "description": null, "plain_text_content": "\#(excerpt)""#)
        )

        XCTAssertNotNil(result.plainTextContent)
        // Despite the name, this is an HTML fragment: matches are wrapped in
        // <mark class="circled-text"> and the whole thing is truncated to 300
        // characters by BC3.
        XCTAssertTrue(result.plainTextContent?.contains(#"<mark class="circled-text">"#) ?? false)
        XCTAssertNil(result.plainTextDescription)
    }

    /// Re-encoding must keep the nulls on the wire. Dropping them would turn a
    /// required key into an absent one and break round-tripping.
    func testReencodesNullsRatherThanOmittingThem() throws {
        let result = try makeDecoder().decode(
            SearchResult.self,
            from: payload(extraKeys: #", "content": null, "description": null"#)
        )

        let encoded = try makeEncoder().encode(result)
        let object = try XCTUnwrap(
            try JSONSerialization.jsonObject(with: encoded) as? [String: Any]
        )

        XCTAssertTrue(object.keys.contains("content"))
        XCTAssertTrue(object.keys.contains("description"))
        XCTAssertTrue(object["content"] is NSNull)
        XCTAssertTrue(object["description"] is NSNull)
        XCTAssertFalse(object.keys.contains("plain_text_content"))
    }
}
