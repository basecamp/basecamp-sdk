import XCTest

@testable import Basecamp

/// The half of the mention contract the shared fixture cannot reach.
///
/// `conformance/tests/recording_summary.json` pins the round trip — a comment's
/// content yields the right person id, a write places the right tag — with one
/// well-formed sgid. Everything that makes the helpers *safe* is a negative: a
/// gid that merely appears inside another value, an sgid minted for a different
/// purpose, a tag inside an HTML comment, a write that must not be skipped
/// because the content already carries some other tag naming the same person.
/// None of those can be a fixture case without a second wire shape to drive
/// them, so they live here.
final class MentionsTests: XCTestCase {
    /// A real `attachable_sgid` as BC3 serves one: Rails' older Marshal
    /// envelope, `{"gid" => …, "purpose" => "attachable", "expires_at" => nil}`,
    /// signed. Taken from the conformance fixture, so the two cannot drift.
    private let marshalSgid =
        "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102"
    private let marshalSgidPersonId = 1_049_715_915

    /// The current Rails layout in its JSON spelling:
    /// `{"_rails": {"data": "gid://bc3/Person/42", "pur": "attachable"}}`.
    private let railsJSONSgid =
        "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19"
    /// The older layout in JSON:
    /// `{"gid": "gid://bc3/Person/43", "purpose": "attachable", "expires_at": null}`.
    private let legacyJSONSgid =
        "eyJnaWQiOiJnaWQ6Ly9iYzMvUGVyc29uLzQzIiwicHVycG9zZSI6ImF0dGFjaGFibGUiLCJleHBpcmVzX2F0IjpudWxsfQ"
    /// A Person gid minted for a different purpose — valid, and not a mention.
    private let readablePurposeSgid =
        "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDQiLCJwdXIiOiJyZWFkYWJsZSJ9fQ"
    /// A file attachment's sgid: attachable, but it names a blob.
    private let blobSgid =
        "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9BY3RpdmVTdG9yYWdlOjpCbG9iLzU1IiwicHVyIjoiYXR0YWNoYWJsZSJ9fQ"
    /// A Document gid built out of a Person gid — the case a byte search for
    /// "Person/" would call a mention.
    private let nestedGidSgid =
        "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9Eb2N1bWVudC9naWQ6Ly9iYzMvUGVyc29uLzY2IiwicHVyIjoiYXR0YWNoYWJsZSJ9fQ"

    // MARK: - sgid decoding

    func testDecodesBothEnvelopeLayoutsAndBothSerializations() {
        XCTAssertEqual(
            Mentions.personId(fromAttachableSgid: marshalSgid), marshalSgidPersonId,
            "the signed Marshal envelope BC3 actually serves must decode")
        XCTAssertEqual(Mentions.personId(fromAttachableSgid: railsJSONSgid), 42)
        XCTAssertEqual(Mentions.personId(fromAttachableSgid: legacyJSONSgid), 43)
    }

    func testRefusesAnSgidMintedForAnotherPurpose() {
        XCTAssertNil(
            Mentions.personId(fromAttachableSgid: readablePurposeSgid),
            "only an \"attachable\" sgid may be placed in rich text, so no other purpose is a mention")
    }

    func testRefusesAnSgidThatNamesSomethingOtherThanAPerson() {
        XCTAssertNil(Mentions.personId(fromAttachableSgid: blobSgid))
    }

    /// The envelope is read structurally, never searched as bytes.
    func testAPersonGidInsideAnotherGidIsNotAMention() {
        XCTAssertNil(Mentions.personId(fromAttachableSgid: nestedGidSgid))
    }

    func testRefusesUndecodableAndOversizedSgids() {
        XCTAssertNil(Mentions.personId(fromAttachableSgid: ""))
        XCTAssertNil(Mentions.personId(fromAttachableSgid: "not base64 at all !!"))
        XCTAssertNil(Mentions.personId(fromAttachableSgid: "--onlyadigest"))
        XCTAssertNil(
            Mentions.personId(
                fromAttachableSgid: String(repeating: "A", count: Mentions.maxSgidEncodedBytes + 1)),
            "the bound is applied to the encoded form, before anything is allocated")
    }

    func testRefusesAMalformedGlobalIdPath() {
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/12x"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/0"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid:///Person/12"), "a gid needs an app host")
        XCTAssertNil(Mentions.personId(fromGlobalId: "https://bc3/Person/12"))
        XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://bc3/Person/12"), 12)
    }

    // MARK: - Reading mentions out of markup

    func testReadsEveryAttachmentInDocumentOrderWithoutRepeats() {
        let content = """
            <div><bc-attachment sgid="\(railsJSONSgid)"></bc-attachment> and \
            <bc-attachment sgid="\(legacyJSONSgid)"></bc-attachment> and \
            <bc-attachment sgid="\(railsJSONSgid)"></bc-attachment></div>
            """
        XCTAssertEqual(Mentions.personIds(in: content), [42, 43])
    }

    func testCountsAQuotedMentionBecauseBC3NotifiesIt() {
        let content = "<blockquote><bc-attachment sgid=\"\(railsJSONSgid)\"></bc-attachment></blockquote>"
        XCTAssertEqual(Mentions.personIds(in: content), [42])
    }

    func testSkipsAnAttachmentInsideAnHTMLComment() {
        let content = "<div><!-- <bc-attachment sgid=\"\(railsJSONSgid)\"></bc-attachment> --></div>"
        XCTAssertEqual(
            Mentions.personIds(in: content), [],
            "a tag inside a comment is not an element, so it mentions nobody")
    }

    func testSkipsAnAttachmentSpelledInsideAnotherTagsAttributeValue() {
        let content = "<div title='<bc-attachment sgid=\"\(railsJSONSgid)\">'>hello</div>"
        XCTAssertEqual(Mentions.personIds(in: content), [])
    }

    func testATagNameThatMerelyStartsWithTheAttachmentNameIsNotOne() {
        let content = "<bc-attachment-preview sgid=\"\(railsJSONSgid)\"></bc-attachment-preview>"
        XCTAssertEqual(Mentions.personIds(in: content), [])
    }

    func testAttributeOrderCaseAndQuotingAreFree() {
        let content =
            "<BC-ATTACHMENT content-type='application/vnd.basecamp.mention' SGID='\(railsJSONSgid)'></bc-attachment>"
        XCTAssertEqual(Mentions.personIds(in: content), [42])
    }

    func testAGreaterThanInsideAQuotedValueDoesNotEndTheTag() {
        let content =
            "<bc-attachment alt=\"a > b\" sgid=\"\(railsJSONSgid)\"></bc-attachment>"
        XCTAssertEqual(
            Mentions.personIds(in: content), [42],
            "the attributes are tokenized, not pattern-matched, so the sgid after the \">\" is still an attribute")
    }

    func testTheFirstSgidAttributeWinsAsInHTML() {
        let content =
            "<bc-attachment sgid=\"\(railsJSONSgid)\" sgid=\"\(legacyJSONSgid)\"></bc-attachment>"
        XCTAssertEqual(Mentions.personIds(in: content), [42])
    }

    func testDecodesEntityEscapesInTheSgidValue() {
        // The sgid is base64, so an escape in it is corruption — but decoding
        // the value is what makes that decision, rather than the escape quietly
        // becoming part of the payload.
        XCTAssertEqual(
            Mentions.attachmentSgids(in: "<bc-attachment sgid=\"a&amp;b&#65;\"></bc-attachment>"),
            ["a&bA"])
    }

    func testAnUnterminatedTagStopsTheWalk() {
        let content =
            "<bc-attachment sgid=\"\(railsJSONSgid)\"></bc-attachment><div sgid=\"x"
        XCTAssertEqual(
            Mentions.personIds(in: content), [42],
            "the attachment before the unterminated tag still counts; nothing after it is markup")
    }

    // MARK: - Writing mentions

    private func person(_ id: Int, _ sgid: String?) -> Person {
        Person(id: FlexibleInt(id), name: "Person \(id)", attachableSgid: sgid)
    }

    func testMarkupRendersTheWriteSideForm() throws {
        let markup = try Mentions.markup(for: person(42, railsJSONSgid))
        XCTAssertEqual(markup, "<bc-attachment sgid=\"\(railsJSONSgid)\"></bc-attachment>")
    }

    func testMarkupRefusesAPersonWithNoAttachableSgid() {
        XCTAssertThrowsError(try Mentions.markup(for: person(42, nil))) { error in
            guard case BasecampError.usage(let message, _) = error else {
                return XCTFail("expected a usage error, got \(error)")
            }
            XCTAssertTrue(message.contains("no attachable_sgid"), message)
        }
    }

    func testMarkupRefusesAnSgidThatNamesSomebodyElse() {
        // The whole point of the check: the tag would be posted under person 42
        // and mention person 43.
        XCTAssertThrowsError(try Mentions.markup(for: person(42, legacyJSONSgid))) { error in
            guard case BasecampError.usage(let message, _) = error else {
                return XCTFail("expected a usage error, got \(error)")
            }
            XCTAssertTrue(message.contains("does not name that person"), message)
        }
    }

    func testMarkupRefusesAnSgidCarryingMarkup() {
        XCTAssertThrowsError(try Mentions.markup(for: person(42, "abc\"><script>"))) { error in
            guard case BasecampError.usage(let message, _) = error else {
                return XCTFail("expected a usage error, got \(error)")
            }
            XCTAssertTrue(message.contains("malformed"), message)
        }
    }

    /// The guard has to test scalars, not `Character`s: a quote followed by a
    /// combining mark is ONE grapheme cluster that compares unequal to `"`, and
    /// the sgid is written into the attribute verbatim. Nothing else validates
    /// the half after the last `--`, so a Character-based test is an
    /// attribute-escape bypass.
    func testMarkupRefusesAQuoteHiddenInAGraphemeCluster() {
        let payload = railsJSONSgid  // decodes to person 42
        let hidden = "\u{0022}\u{0301}"  // quote + combining acute: one Character
        XCTAssertEqual(hidden.count, 1, "precondition: this is a single grapheme cluster")

        XCTAssertThrowsError(
            try Mentions.markup(for: person(42, "\(payload)--dead\(hidden) onerror=x"))
        ) { error in
            guard case BasecampError.usage(let message, _) = error else {
                return XCTFail("expected a usage error, got \(error)")
            }
            XCTAssertTrue(message.contains("malformed"), message)
        }
    }

    /// Go's base64 decoder ignores CR and LF; Foundation's refuses them. An sgid
    /// a serializer wrapped across lines has to decode to the same person on
    /// both sides, or the mention silently vanishes on one of them.
    func testDecodesAnSgidWrappedAcrossLines() {
        let wrapped = railsJSONSgid.prefix(20) + "\r\n" + railsJSONSgid.dropFirst(20)
        XCTAssertEqual(Mentions.personId(fromAttachableSgid: String(wrapped)), 42)
    }

    /// Parity runs both ways. Go refuses a space inside base64, so accepting one
    /// here would make this the lenient side — a payload Go rejects decoding to
    /// a mention.
    func testRefusesAnSgidWithASpaceInIt() {
        let spaced = railsJSONSgid.prefix(20) + " " + railsJSONSgid.dropFirst(20)
        XCTAssertNil(Mentions.personId(fromAttachableSgid: String(spaced)))
    }

    /// A non-zero final group is ignored by Go's decoder and by Foundation's, so
    /// nothing has to be done about it — but it is pinned, because "handling" it
    /// is the obvious wrong fix.
    func testANonZeroTrailingGroupDecodesTheSameOnBothSides() {
        XCTAssertEqual(Data(base64Encoded: "QR==").map { [UInt8]($0) }, [65])
    }

    func testMentionsArePlacedInsideTheLeadingBlock() throws {
        let content = try Mentions.adding([person(42, railsJSONSgid)], to: "<div>On it.</div>")
        XCTAssertEqual(
            content,
            "<div><bc-attachment sgid=\"\(railsJSONSgid)\"></bc-attachment> On it.</div>",
            "they render on the first line rather than as a block of their own")
    }

    func testMentionsArePrefixedWhenTheContentOpensWithNoBlock() throws {
        let content = try Mentions.adding([person(42, railsJSONSgid)], to: "On it.")
        XCTAssertEqual(
            content, "<bc-attachment sgid=\"\(railsJSONSgid)\"></bc-attachment> On it.")
    }

    func testTheSamePersonTwiceIsMentionedOnce() throws {
        let victor = person(42, railsJSONSgid)
        let content = try Mentions.adding([victor, victor], to: "<p>hi</p>")
        XCTAssertEqual(Mentions.personIds(in: content), [42])
        XCTAssertEqual(Mentions.attachmentSgids(in: content).count, 1)
    }

    func testAnExistingTagWithTheExactSgidSuppressesTheAddition() throws {
        let existing = "<div><bc-attachment sgid=\"\(railsJSONSgid)\"></bc-attachment> hi</div>"
        let content = try Mentions.adding([person(42, railsJSONSgid)], to: existing)
        XCTAssertEqual(content, existing, "the person is already mentioned with that exact sgid")
    }

    /// The trust boundary, stated as a test: dedupe is on the sgid STRING, never
    /// on the person id an existing tag decodes to. A port that deduplicates by
    /// id lets a forged or stale tag suppress the real mention.
    func testADifferentSgidNamingTheSamePersonDoesNotSuppressTheAddition() throws {
        // A second, *unsigned* envelope for the same person — exactly what a
        // forged or expired tag in caller-supplied content looks like.
        let staleSgid = railsJSONSgid + "--deadbeef"
        XCTAssertEqual(
            Mentions.personId(fromAttachableSgid: staleSgid), 42,
            "precondition: the stale tag does decode to the same person")

        let existing = "<div><bc-attachment sgid=\"\(staleSgid)\"></bc-attachment> hi</div>"
        let content = try Mentions.adding([person(42, railsJSONSgid)], to: existing)

        XCTAssertEqual(
            Mentions.attachmentSgids(in: content), [railsJSONSgid, staleSgid],
            "the authoritative sgid is added anyway; only an exact match is a duplicate")
    }

    func testAddingNobodyLeavesTheContentAlone() throws {
        XCTAssertEqual(try Mentions.adding([], to: "<p>hi</p>"), "<p>hi</p>")
    }

    func testAnUnusablePersonFailsTheWholeExpansion() {
        // Even though the first person is fine: nothing is half-written.
        XCTAssertThrowsError(
            try Mentions.adding([person(42, railsJSONSgid), person(99, nil)], to: "<p>hi</p>"))
    }
}
