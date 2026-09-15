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

    /// Go parses the gid with `net/url`, which refuses any ASCII control
    /// character. A parser that strips tab, CR and LF first — the WHATWG rule —
    /// reads a crafted gid as a clean person id where Go reads nothing, and the
    /// write side would then render a mention tag Go refuses to write.
    func testRefusesAGlobalIdCarryingAControlCharacter() {
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/104\u{0A}9715915"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/104\u{09}9715915"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc\u{0D}3/Person/42"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Pers\u{00}on/42"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/42\u{7F}"))
    }

    /// Go reads the id with `ParseInt(..., 64)`, which refuses an overflow
    /// rather than wrapping.
    func testRefusesAPersonIdThatOverflows() {
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/9223372036854775808"))
        XCTAssertEqual(
            Mentions.personId(fromGlobalId: "gid://bc3/Person/9223372036854775807"),
            9_223_372_036_854_775_807)
    }

    /// Go's `url.Parse` refuses an empty host behind userinfo and a bad percent
    /// escape in the authority. A split that only looks for the first slash
    /// accepts both — the PERMISSIVE direction, and the one that matters: a gid
    /// Go refuses must not name a person here, or the write side renders a tag
    /// Go would not write.
    /// The ordering axis, measured against the real Go implementation rather
    /// than reasoned about.
    ///
    /// Four ports got this wrong in two different directions, and neither the
    /// fixture nor a leniency table about what the decoder SKIPS can see it: it
    /// is about where the padding trim sits relative to the separator split and
    /// the line-break strip. Go's order is trim the whole value, split on the
    /// LAST `--`, right-trim the padding, then decode with a decoder that has no
    /// padding character — so a `=` the trim could not reach is refused.
    ///
    /// Every expectation below was produced by running these exact inputs
    /// through `basecamp.PersonIDFromSGID` in `go/pkg/basecamp`, not by reading
    /// it. The CRLF rows are here because a CRLF is a single Swift `Character`,
    /// so anything done over a `String` rather than UTF-8 bytes walks past the
    /// pair a line-wrapping serializer emits.
    func testSgidDecodingMatchesTheGoImplementationRowForRow() {
        let cases: [(name: String, sgid: String, expected: Int?)] = [
            ("01 unpadded + sig", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef", 42),
            ("02 unpadded LF before sep", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19\n--deadbeef", 42),
            ("03 unpadded CRLF before sep", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19\r\n--deadbeef", 42),
            ("04 padded + sig", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==--deadbeef", 42),
            ("05 padded LF between padding and sep", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==\n--deadbeef", nil),
            ("06 padded CRLF between padding and sep", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==\r\n--deadbeef", nil),
            ("07 break amid the padding", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ=\n=--deadbeef", nil),
            ("08 break amid the padding CRLF", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ=\r\n=--deadbeef", nil),
            ("09 leading break on whole value", "\neyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==--deadbeef", 42),
            ("10 trailing break on whole value", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==--deadbeef\n", 42),
            ("11 trailing space on whole value", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==--deadbeef ", 42),
            ("12 leading space on whole value", " eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==--deadbeef", 42),
            ("13 padded, no signature", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==", 42),
            ("14 padded, no signature, trailing break", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ==\n", 42),
            ("15 unpadded, space inside", "eyJfcmFpbHMiOnsiZGF0 YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef", nil),
            ("16 unpadded, tab inside", "eyJfcmFpbHMiOnsiZGF0\tYSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef", nil),
            ("17 unpadded, LF inside", "eyJfcmFpbHMiOnsiZGF0\nYSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef", 42),
            ("18 padded, interior = after trim", "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn0sIngiOiIifQ=Q--deadbeef", nil),
        ]
        for row in cases {
            XCTAssertEqual(
                Mentions.personId(fromAttachableSgid: row.sgid), row.expected, row.name)
        }
    }

    /// HTML character references, measured against `html.UnescapeString` through
    /// the same end-to-end diff as the sgid table above — 4,000 fuzzed attribute
    /// values as well as these, zero mismatches.
    ///
    /// The boundaries here are not guessable, which is why they are pinned
    /// rather than reasoned about: `&#9` is literal while `&#x9` is a tab,
    /// because the `x` counts toward the same index Go tests; `&#133;` is an
    /// ellipsis rather than the NEL that would have been trimmed, because
    /// 0x80–0x9F are remapped through Windows-1252; `&#8203;` survives the trim
    /// because U+200B is whitespace to Foundation and not to Go; and `&nbspBAh…`
    /// resolves because a name is matched against the table longest-first rather
    /// than by consuming the longest run of name characters.
    func testEntityDecodingMatchesTheGoImplementationRowForRow() {
        let cases: [(name: String, html: String, expected: [Int])] = [
            ("a named reference matched longest-first, not by the longest run of name characters", "<div><bc-attachment sgid=\"&nbspeyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", [42]),
            ("the same with its semicolon", "<div><bc-attachment sgid=\"&nbsp;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", [42]),
            ("one character after &# is not a reference", "<div><bc-attachment sgid=\"&#9eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("...but a semicolon makes it one", "<div><bc-attachment sgid=\"&#9;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", [42]),
            ("...and in hex the x counts toward the same index", "<div><bc-attachment sgid=\"&#x9eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("0x80-0x9F is remapped through Windows-1252, so this is an ellipsis, not NEL", "<div><bc-attachment sgid=\"&#133;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("U+200B is not whitespace to Go, so it is not trimmed", "<div><bc-attachment sgid=\"&#8203;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("U+00A0 is", "<div><bc-attachment sgid=\"&#160;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", [42]),
            ("a two-scalar expansion, all of it whitespace", "<div><bc-attachment sgid=\"&ThickSpace;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", [42]),
            ("NUL becomes U+FFFD", "<div><bc-attachment sgid=\"&#0;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("a surrogate becomes U+FFFD", "<div><bc-attachment sgid=\"&#xD800;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("an unknown name is left verbatim", "<div><bc-attachment sgid=\"&notaname;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("a legacy name expands without its semicolon", "<div><bc-attachment sgid=\"&ampeyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("UnderBar spells an underscore", "<div><bc-attachment sgid=\"eyJfcmFpbHMiOnsiZGF0&UnderBar;YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("hyphen does NOT spell a hyphen", "<div><bc-attachment sgid=\"eyJfcmFpbHMiOnsiZGF0&hyphen;YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
            ("lowbar inside the payload", "<div><bc-attachment sgid=\"eyJfcmFpbHMiOnsiZGF0&lowbar;YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef\"></bc-attachment></div>", []),
        ]
        for row in cases {
            XCTAssertEqual(Mentions.personIds(in: row.html), row.expected, row.name)
        }
    }

    func testRefusesAMalformedAuthority() {
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://@/Person/1"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bad%zz/Person/1"))
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://a b/Person/1"))
        XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://bc3/Person/1"), 1)
    }

    /// Go decodes a numeric character reference whether or not it carries the
    /// terminating semicolon, and a numeric reference can spell a letter — which
    /// is in the base64 alphabet. Leaving it encoded loses a real mention, and
    /// on the write side makes the authoritative tag look absent.
    func testDecodesNumericReferencesWithAndWithoutTheirSemicolon() {
        let tail = String(railsJSONSgid.dropFirst())  // the payload minus its leading "e"
        for spelling in ["&#101;", "&#101", "&#x65;", "&#x65"] {
            let content = "<bc-attachment sgid=\"\(spelling)\(tail)\"></bc-attachment>"
            XCTAssertEqual(
                Mentions.personIds(in: content), [42],
                "\(spelling) spells \"e\", which is base64")
        }
    }

    /// The named references that matter are the ones producing a character the
    /// base64 alphabet contains; anything else fails the envelope decode on both
    /// sides whether it was decoded or not.
    func testDecodesTheNamedReferencesThatSpellBase64Characters() {
        XCTAssertEqual(
            Mentions.attachmentSgids(
                in: "<bc-attachment sgid=\"a&plus;b&sol;c&equals;\"></bc-attachment>"),
            ["a+b/c="])
        XCTAssertEqual(
            Mentions.attachmentSgids(in: "<bc-attachment sgid=\"a&lowbar;b\"></bc-attachment>"),
            ["a_b"])
        // `&hyphen;` and `&dash;` name U+2010, not ASCII `-` — measured against
        // html.UnescapeString, not assumed. Decoding them to a hyphen would make
        // this the lenient side, so they are left verbatim, which is also what
        // the base64 decode does with them either way.
        XCTAssertEqual(
            Mentions.attachmentSgids(in: "<bc-attachment sgid=\"a&hyphen;b\"></bc-attachment>"),
            ["a&hyphen;b"])
    }

    /// Go trims the trailing `=` BEFORE decoding, and its decoder then refuses
    /// an `=` anywhere. Stripping newlines first instead turns `<payload>=\n`
    /// into `<payload>=`, trims that `=` as trailing, and accepts an envelope Go
    /// rejects — and Foundation's decoder cannot be leaned on either, since it
    /// accepts an interior `=`.
    func testBase64NormalizationHappensInGosOrder() {
        let json = #"{"_rails":{"data":"gid://bc3/Person/42","pur":"attachable"},"x":"y"}"#
        let padded = Data(json.utf8).base64EncodedString()
        XCTAssertTrue(padded.hasSuffix("="), "precondition: this payload is padded")

        XCTAssertEqual(Mentions.envelopeGlobalId(padded), "gid://bc3/Person/42")
        XCTAssertNil(
            Mentions.envelopeGlobalId(padded + "\n"),
            "the padding is no longer trailing, so it stays — and an interior = is illegal")
        XCTAssertEqual(
            Mentions.envelopeGlobalId(String(padded.dropLast()) + "\n="),
            "gid://bc3/Person/42",
            "here the = IS trailing, so it is trimmed and the newline ignored")
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

    /// The whole base64 leniency table, measured against Go's `RawStdEncoding`
    /// rather than reasoned about, because the fixture cannot see any of it and
    /// a divergence in EITHER direction is a silent wrong answer: stricter loses
    /// a real mention, looser accepts a payload Go refuses.
    ///
    /// Foundation matches none of it out of the box — `Data(base64Encoded:)`
    /// rejects CR and LF, and `.ignoreUnknownCharacters` would accept space and
    /// tab as well — which is why the decode normalises the two bytes Go ignores
    /// and nothing else.
    func testBase64LeniencyMatchesGoRowForRow() {
        // A payload for person 42, and the same payload mutilated six ways.
        func decodes(_ payload: String) -> Bool {
            Mentions.envelopeGlobalId(payload) == "gid://bc3/Person/42"
        }
        let base = railsJSONSgid
        let head = String(base.prefix(20))
        let tail = String(base.dropFirst(20))

        XCTAssertTrue(decodes(base), "the control")
        // Non-zero trailing (discarded) bits: Go accepts, only Encoding.Strict()
        // rejects. Checked on the primitive, since a mutated envelope would not
        // decode to the same gid.
        XCTAssertEqual(Data(base64Encoded: "QR==").map { [UInt8]($0) }, [65])
        // length % 4 == 1: rejected on both sides.
        XCTAssertFalse(decodes(base + "A"))
        // Embedded LF and CR: accepted on both sides.
        XCTAssertTrue(decodes(head + "\n" + tail))
        XCTAssertTrue(decodes(head + "\r" + tail))
        XCTAssertTrue(decodes(head + "\r\n" + tail))
        // Embedded space and tab: rejected on both sides.
        XCTAssertFalse(decodes(head + " " + tail))
        XCTAssertFalse(decodes(head + "\t" + tail))
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

    /// "Exact attachable_sgid" is a BYTE rule. Swift string equality is
    /// canonical equivalence, so a combining sequence and its precomposed form
    /// compare equal — and a dedupe keyed on `String` would skip a mention Go
    /// adds, because Go compares map keys byte for byte.
    func testDedupeComparesBytesRatherThanCanonicalEquivalence() throws {
        let decomposed = railsJSONSgid + "--e\u{0301}"
        let precomposed = railsJSONSgid + "--\u{00E9}"
        XCTAssertEqual(decomposed, precomposed, "precondition: Swift calls these equal")
        XCTAssertNotEqual(Array(decomposed.utf8), Array(precomposed.utf8))

        let existing = "<div><bc-attachment sgid=\"\(decomposed)\"></bc-attachment></div>"
        let content = try Mentions.adding(
            [Person(id: 42, name: "Victor", attachableSgid: precomposed)], to: existing)

        XCTAssertEqual(
            Mentions.attachmentSgids(in: content).count, 2,
            "different bytes are different sgids, whatever Unicode says about them")
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
