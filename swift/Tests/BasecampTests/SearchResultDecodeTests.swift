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

    // MARK: - The four special-cased branches (#651)

    /// The shared, coverage-guarded body: eight hits covering the generic
    /// recording envelope and all four branches
    /// `api_search_result_template_path` special-cases. Read relative to this
    /// source file, the way `UpcomingScheduleDecodeTests` does, so it cannot
    /// drift from the copy the other five SDKs and the conformance runners
    /// assert against; `make check-fixture-coverage` validates it against the
    /// generated schema.
    private func sharedResults() throws -> [SearchResult] {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()  // BasecampTests
            .deletingLastPathComponent()  // Tests
            .deletingLastPathComponent()  // swift
            .deletingLastPathComponent()  // <repo root>
        let data = try Data(
            contentsOf: repoRoot
                .appendingPathComponent("spec/fixtures/search/results.json")
        )
        return try makeDecoder().decode([SearchResult].self, from: data)
    }

    func testSharedFixtureDecodesEveryBranch() throws {
        let results = try sharedResults()
        XCTAssertEqual(results.count, 8)

        // bubble_up_url rides the polymorphic projection: todolists/_todolist is
        // the only partial that passes `bubbleupable: true`.
        XCTAssertEqual(
            results.filter { $0.bubbleUpUrl != nil }.map(\.type),
            ["Todolist"]
        )
    }

    /// `searches/_attachment.json.jbuilder` writes its own projection instead of
    /// decorating the recording envelope, so it emits NONE of
    /// `id`/`title`/`type`/`url`/`app_url`. Before #651 those five were
    /// non-optional here and this hit could not decode at all.
    func testFileAttachmentBranchOmitsTheFiveEnvelopeKeys() throws {
        let hit = try XCTUnwrap(try sharedResults().first { $0.type == nil })

        XCTAssertNil(hit.id)
        XCTAssertNil(hit.title)
        XCTAssertNil(hit.type)
        XCTAssertNil(hit.url)
        XCTAssertNil(hit.appUrl)

        XCTAssertEqual(hit.filename, "leto-hero.jpg")
        XCTAssertEqual(hit.contentType, "image/jpeg")
        XCTAssertEqual(hit.byteSize, 512000)
        XCTAssertEqual(hit.previewable, true)
        // Float-spelled on the wire (`1920.0`) against an `Int32?` member.
        // Foundation's JSONDecoder accepts an integral-valued JSON float into an
        // integer field, so no FlexibleInt wrapper is needed — this test is what
        // says so, rather than the claim being assumed.
        XCTAssertEqual(hit.width, 1920)
        XCTAssertEqual(hit.height, 1080)
        XCTAssertNotNil(hit.previewUrl)
        XCTAssertNotNil(hit.thumbnailUrl)
        XCTAssertNotNil(hit.downloadUrl)
        XCTAssertNotNil(hit.appDownloadUrl)
        // Present-and-null holds on this branch too — the pair the tests above
        // pin is required even where the envelope keys are not.
        XCTAssertNil(hit.content)
        XCTAssertNil(hit.description)
        XCTAssertEqual(hit.parent?.type, "Message")
    }

    /// A chat upload line's `attachments` is a BESPOKE six-key aggregate the
    /// line builds inline — not a `RichTextAttachment`. `SearchResultAttachment`
    /// exists because of this: it is the optional-field superset of both wire
    /// variants, with only the four keys both always emit non-optional.
    func testChatUploadLineCarriesTheBespokeAttachmentVariant() throws {
        let hit = try XCTUnwrap(
            try sharedResults().first { $0.type == "Chat::Lines::Upload" }
        )

        // Chat lines pass `boostable`, so the envelope emits the boost pair.
        XCTAssertEqual(hit.boostsCount, 1)
        XCTAssertNotNil(hit.boostsUrl)

        let attachment = try XCTUnwrap(hit.attachments?.first)
        XCTAssertEqual(attachment.title, "leto-benchmarks.pdf")
        XCTAssertNotNil(attachment.url)
        XCTAssertEqual(attachment.filename, "leto-benchmarks.pdf")
        XCTAssertEqual(attachment.contentType, "application/pdf")
        XCTAssertEqual(attachment.byteSize, 1_048_576)
        XCTAssertNotNil(attachment.downloadUrl)
        // The rich-text variant's keys are absent, which is the whole point.
        XCTAssertNil(attachment.id)
        XCTAssertNil(attachment.sgid)
        XCTAssertNil(attachment.previewable)
        XCTAssertNil(attachment.width)
    }

    /// A kanban (card table) list layers the list partial's keys over the
    /// recording envelope. `color` is emitted unconditionally with a null value
    /// when unset, so it must stay optional rather than being required.
    func testKanbanListCarriesListKeysAndANullColor() throws {
        let hit = try XCTUnwrap(
            try sharedResults().first { $0.type == "Kanban::Column" }
        )

        XCTAssertEqual(hit.cardsCount, 4)
        XCTAssertEqual(hit.commentCount, 1)
        XCTAssertNotNil(hit.cardsUrl)
        XCTAssertNil(hit.color)
        // Envelope keys the list branch reaches: subscribable and positioned.
        XCTAssertNotNil(hit.subscriptionUrl)
        XCTAssertEqual(hit.position, 2)
        XCTAssertEqual(hit.subscribers?.map(\.name), ["Victor Cooper"])
        // on_hold is a whole nested list, not a flag.
        let onHold = try XCTUnwrap(hit.onHold)
        XCTAssertEqual(onHold.cardsCount, 0)
        XCTAssertFalse(onHold.cardsUrl.isEmpty)
    }

    /// A gauge needle is both commentable and boostable, so it carries BOTH
    /// count pairs — plus the branch partial's own singular `comment_count`, a
    /// distinct key from the envelope's `comments_count`. Its `attachments` is
    /// the OTHER variant: the rich-text one, the mirror of the upload line.
    func testGaugeNeedleCarriesBothCountPairsAndTheRichTextVariant() throws {
        let hit = try XCTUnwrap(
            try sharedResults().first { $0.type == "Gauge::Needle" }
        )

        XCTAssertEqual(hit.commentsCount, 2)
        XCTAssertEqual(hit.commentCount, 2)
        XCTAssertEqual(hit.boostsCount, 3)
        XCTAssertEqual(hit.color, "green")
        XCTAssertEqual(hit.position, 72)
        // description is nil-overwritten; its companion array survives.
        XCTAssertNil(hit.description)
        XCTAssertEqual(hit.descriptionAttachments?.count, 1)

        let attachment = try XCTUnwrap(hit.attachments?.first)
        XCTAssertEqual(attachment.id, 1069479631)
        XCTAssertEqual(attachment.sgid?.hasSuffix("--srchndl1"), true)
        XCTAssertEqual(attachment.width, 1024)
        XCTAssertEqual(attachment.previewable, true)
    }
}
