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

    /// Go puts no cap on a numeric reference's digit count, and a cap is the
    /// obvious wrong optimisation: `&#00000000065;` is an ordinary way to write
    /// `A`, so "anything longer is out of range anyway" is false. Measured
    /// against `html.UnescapeString`, which gives "A" for all four of these.
    func testLeadingZerosDoNotExhaustTheNumericScanner() {
        for spelling in ["&#65;", "&#065;", "&#00000000065;", "&#x0000000000041;"] {
            XCTAssertEqual(
                Mentions.attachmentSgids(in: "<bc-attachment sgid=\"\(spelling)\"></bc-attachment>"),
                ["A"], spelling)
        }
    }

    /// The gid PATH and SCHEME, against Go, over the whole sweep rather than a
    /// handful of rows — the cross product is built here, and Go's answers to it
    /// are the data below.
    ///
    /// 747 distinct shapes, from nine spellings of the scheme, three prefixes
    /// and twenty-eight paths. Go names a person for 44, obtained by running each through
    /// `PersonIDFromSGID` in `go/pkg/basecamp`. This parser must name a person
    /// for exactly those 44 MINUS the 12 that differ, and for nothing else.
    ///
    /// The 12 are one mechanism, not one shape: Go reads `u.Path`, which is
    /// percent-DECODED, and this reads the path as written — so an escape
    /// spelling the model name, the id, or the separator between them names a
    /// Person there and nobody here. The set difference is what makes the
    /// direction a property rather than a claim: any shape this accepted and Go
    /// did not would appear as an extra element, and there is none.
    func testTheGidPathDisagreesWithGoInOneDirectionOnly() {
        let paths = [
            "Person/1",
            "Person/01",
            "Person/1/extra",
            "Person/",
            "Person",
            "Pers%6Fn/1",
            "Person/%31",
            "person/1",
            "PERSON/1",
            "Person/1?x=y",
            "Person/1#frag",
            "Person/1#x?y",
            "Person/1?y#x",
            "Person//1",
            "/Person/1",
            "Person/1/",
            "Person/-1",
            "Person/+1",
            "Person/ 1",
            "Person/1 ",
            "Person%2F1",
            "Person/9223372036854775807",
            "Person/9223372036854775808",
            "Person/0",
            "Person/00001",
            "Per\u{A7}on/1",
            "Person/\u{FF11}",
            "Person/1\u{301}",
        ]
        let schemes = ["gid", "GID", "Gid", "gID", " gid", "gid ", "xgid", "", "g id"]
        let prefixes = ["%@://bc3/", "%@:/", "%@:"]

        // Every shape Go names a person for. The cross product yields 756
        // constructions but 747 distinct gids — `scheme:` + `/Person/1` and
        // `scheme:/` + `Person/1` spell the same thing — which is why this is a
        // Set and why the count below is of shapes, not of constructions.
        let goAccepts: Set<String> = [
            "GID://bc3/Pers%6Fn/1",
            "GID://bc3/Person%2F1",
            "GID://bc3/Person/%31",
            "GID://bc3/Person/00001",
            "GID://bc3/Person/01",
            "GID://bc3/Person/1",
            "GID://bc3/Person/1#frag",
            "GID://bc3/Person/1#x?y",
            "GID://bc3/Person/1?x=y",
            "GID://bc3/Person/1?y#x",
            "GID://bc3/Person/9223372036854775807",
            "Gid://bc3/Pers%6Fn/1",
            "Gid://bc3/Person%2F1",
            "Gid://bc3/Person/%31",
            "Gid://bc3/Person/00001",
            "Gid://bc3/Person/01",
            "Gid://bc3/Person/1",
            "Gid://bc3/Person/1#frag",
            "Gid://bc3/Person/1#x?y",
            "Gid://bc3/Person/1?x=y",
            "Gid://bc3/Person/1?y#x",
            "Gid://bc3/Person/9223372036854775807",
            "gID://bc3/Pers%6Fn/1",
            "gID://bc3/Person%2F1",
            "gID://bc3/Person/%31",
            "gID://bc3/Person/00001",
            "gID://bc3/Person/01",
            "gID://bc3/Person/1",
            "gID://bc3/Person/1#frag",
            "gID://bc3/Person/1#x?y",
            "gID://bc3/Person/1?x=y",
            "gID://bc3/Person/1?y#x",
            "gID://bc3/Person/9223372036854775807",
            "gid://bc3/Pers%6Fn/1",
            "gid://bc3/Person%2F1",
            "gid://bc3/Person/%31",
            "gid://bc3/Person/00001",
            "gid://bc3/Person/01",
            "gid://bc3/Person/1",
            "gid://bc3/Person/1#frag",
            "gid://bc3/Person/1#x?y",
            "gid://bc3/Person/1?x=y",
            "gid://bc3/Person/1?y#x",
            "gid://bc3/Person/9223372036854775807",
        ]
        // The subset this refuses, deliberately, and the only way the two differ.
        let percentDecodedPath: Set<String> = [
            "GID://bc3/Pers%6Fn/1",
            "GID://bc3/Person%2F1",
            "GID://bc3/Person/%31",
            "Gid://bc3/Pers%6Fn/1",
            "Gid://bc3/Person%2F1",
            "Gid://bc3/Person/%31",
            "gID://bc3/Pers%6Fn/1",
            "gID://bc3/Person%2F1",
            "gID://bc3/Person/%31",
            "gid://bc3/Pers%6Fn/1",
            "gid://bc3/Person%2F1",
            "gid://bc3/Person/%31",
        ]

        var accepted: Set<String> = []
        for scheme in schemes {
            for prefix in prefixes {
                for path in paths {
                    let gid = prefix.replacingOccurrences(of: "%@", with: scheme) + path
                    if Mentions.personId(fromGlobalId: gid) != nil { accepted.insert(gid) }
                }
            }
        }

        XCTAssertEqual(
            accepted, goAccepts.subtracting(percentDecodedPath),
            "this parser must accept exactly what Go accepts, less the percent-decoded-path shapes")
        // Not `accepted.isSubset(of: goAccepts)`: that is implied by the
        // equality above, since `subtracting` can only remove. This pins the
        // PREMISE the equality rests on instead — that the twelve shapes named
        // as the one mechanism are themselves gids Go accepts, so the set
        // difference is narrowing the right set.
        XCTAssertTrue(
            percentDecodedPath.isSubset(of: goAccepts),
            "the documented divergences must be shapes Go accepts, or the difference means nothing")
    }

    /// The fragment and the query are not equivalent, and truncating at the
    /// first of either gets one of them wrong.
    ///
    /// Go splits the fragment off the whole URL and UNESCAPES it, so a malformed
    /// escape there refuses the entire gid. It keeps `RawQuery` raw and
    /// validates nothing. Truncating at `#` without looking accepted a gid Go
    /// refuses — the permissive direction, on a path whose only
    /// authenticity-adjacent gate is "does this sgid name this person".
    ///
    /// Measured over 364 shapes — two paths × seven queries × thirteen
    /// fragments, in both orders — of which Go accepts 160, with zero
    /// mismatches in either direction.
    func testTheFragmentIsValidatedAndTheQueryIsNot() {
        // Go unescapes the fragment, so a malformed escape in it refuses the gid.
        for gid in [
            "gid://bc3/Person/1#%zz", "gid://bc3/Person/1#%", "gid://bc3/Person/1#%2",
            "gid://bc3/Person/1?x=y#%zz", "gid://bc3/Person/1#a%1", "gid://bc3/Person/1#%1g",
        ] {
            XCTAssertNil(Mentions.personId(fromGlobalId: gid), gid)
        }
        // A well-formed escape, or no escape at all, is fine there.
        for gid in [
            "gid://bc3/Person/1#frag", "gid://bc3/Person/1#%41", "gid://bc3/Person/1#%C3%A9",
            "gid://bc3/Person/1#a b", "gid://bc3/Person/1#f%20g", "gid://bc3/Person/1#",
            // And the query is never validated: Go keeps it raw.
            "gid://bc3/Person/1?%zz", "gid://bc3/Person/1?%", "gid://bc3/Person/1?x=%zz",
            "gid://bc3/Person/1?a b", "gid://bc3/Person/1?",
        ] {
            XCTAssertEqual(Mentions.personId(fromGlobalId: gid), 1, gid)
        }
    }

    /// The authority, against `net/url` in BOTH directions — measured over 1,824
    /// generated shapes, because a charset that looked about right was wrong
    /// each way, and a first fix that checked only the host was wrong again.
    func testTheAuthorityMatchesWhatGoAccepts() {
        // Refused there, so refused here. Userinfo leaves the host empty; a
        // malformed escape, a space or a control refuses the gid wherever it
        // sits; an escape naming an ASCII byte is not allowed in a HOST; and a
        // port is digits or nothing.
        for gid in [
            "gid://user@/Person/1", "gid://@/Person/1", "gid:///Person/1",
            "gid://bad%zz/Person/1", "gid://bad%zz@bc3/Person/1",
            "gid://a%41b/Person/1", "gid://a b/Person/1", "gid://a b@bc3/Person/1",
            "gid://us er@bc3/Person/1", "gid://user%zz:pw@bc3/Person/1",
            "gid://bc3:notaport/Person/1", "gid://[::1]:x/Person/1",
        ] {
            XCTAssertNil(Mentions.personId(fromGlobalId: gid), gid)
        }
        // Accepted there, so accepted here — each one a mention Go reads that a
        // conservative allowlist would have thrown away.
        for gid in [
            "gid://bc3/Person/1", "gid://b%C3%A9c3/Person/1", "gid://a%25b/Person/1",
            "gid://a\"b/Person/1", "gid://a<b/Person/1", "gid://a>b/Person/1",
            "gid://a]b/Person/1", "gid://a_b/Person/1",
            "gid://bc3:8080/Person/1", "gid://bc3:/Person/1", "gid://bc3:0/Person/1",
            "gid://bc3:99999999999/Person/1", "gid://bc3:80:80/Person/1",
            // An escape naming an ASCII byte is fine in USERINFO, unlike a host.
            "gid://a%41b@bc3/Person/1", "gid://user:pw@bc3/Person/1",
            // Go's non-empty test is on the host WITH its port.
            "gid://:8080/Person/1", "gid://user@:8080/Person/1",
        ] {
            XCTAssertEqual(Mentions.personId(fromGlobalId: gid), 1, gid)
        }
    }

    /// A Swift `Character` is a grapheme cluster and a URL grammar's character
    /// is a byte, and that gap runs BOTH ways at once.
    ///
    /// `%` followed by a combining acute is one `Character` equal to neither
    /// `"%"` nor anything else, so a `Character`-based escape scan walks past a
    /// malformed escape Go refuses; and `#` followed by one is a `Character`
    /// that is not `"#"`, so a `Character`-based delimiter search never finds
    /// the fragment Go splits off. The first is the accepting direction — a gid
    /// Go rejects, read here as a mention — and no sweep of ASCII shapes can
    /// see either, which is why this table exists next to one.
    ///
    /// Every expectation was produced by running the row through Go's
    /// `url.Parse` and the `PersonIDFromSGID` gid rules, not by reading them.
    func testAGraphemeClusterCannotHideADelimiter() {
        let cases: [(gid: String, expected: Int?)] = [
            // The escape is malformed in Go's eyes wherever the cluster hides it.
            ("gid://bc3/Person/1#%\u{0301}zz", nil),
            ("gid://bc3/Person/1#%\u{0300}", nil),
            ("gid://bc3/Person/1#%2\u{0301}5", nil),
            ("gid://bc3/Person/1#\u{0301}%zz", nil),
            // The `#` is still a `#` to Go, so the fragment is still a fragment.
            ("gid://bc3/Person/1#\u{0301}", 1),
            ("gid://bc3/Person/1#\u{0301}ok", 1),
            ("gid://bc3/Person/1?\u{0301}%zz", 1),
            // A cluster over a structural byte elsewhere is not that byte.
            ("gid://bc3/Person/1\u{0301}", nil),
            ("gid://bc3/Person\u{0301}/1", nil),
            ("gid://bc3/\u{0301}Person/1", nil),
            ("gid:\u{0301}//bc3/Person/1", nil),
            ("gid://bc3\u{0301}/Person/1", 1),
        ]
        for (gid, expected) in cases {
            XCTAssertEqual(Mentions.personId(fromGlobalId: gid), expected, gid.debugDescription)
        }
    }

    /// Only an IPv6 address may be bracketed, and `parseHost` enforces it with
    /// `netip.ParseAddr` — so `[notanip]`, `[]`, `[::1]]` and even the perfectly
    /// good IPv4 literal `[1.2.3.4]` are gids Go reads as nobody. A parser that
    /// treats brackets as decoration accepts all four, which is the accepting
    /// direction on the mention path.
    ///
    /// The cross product below is rebuilt here rather than described: ten group
    /// spellings joined by `:` and by `::` at one, two and three positions,
    /// 2,110 distinct literals, against the 60 Go names a person for. Compared
    /// as a SET, so a literal this accepted and Go did not appears as an extra
    /// element rather than as an unexamined claim.
    ///
    /// The `%25en0` rows are the ones worth reading twice. A zone begins at the
    /// FIRST `%`, not the last, and everything after it is zone however many
    /// more percents or colons it carries — which is why `0::%25en0::g` is a
    /// host Go reads. Only four of the sixty carry a second `%`, so this sweep
    /// is not where that rule was caught; splitting at the last `%` refused 108
    /// shapes of a separate 45,816-gid fuzz. Four rows is enough to hold it once
    /// it is known, which is what a regression test is for.
    func testABracketedHostIsAnIPv6AddressOrNoHostAtAll() {
        let groups = ["", "0", "1", "ffff", "fffff", "g", "1.2.3.4", "01.2.3.4", "%25en0", "%en0"]
        var literals = Set<String>()
        for separator in [":", "::"] {
            for a in groups {
                literals.insert(a)
                for b in groups {
                    literals.insert([a, b].joined(separator: separator))
                    for c in groups {
                        literals.insert([a, b, c].joined(separator: separator))
                    }
                }
            }
        }
        XCTAssertEqual(literals.count, 2110)

        // What `PersonIDFromSGID` names a person for, run rather than reasoned.
        let goAccepts: Set<String> = [
            "0::", "0::%25en0", "0::%25en0::", "0::%25en0::%25en0", "0::%25en0::0",
            "0::%25en0::01.2.3.4", "0::%25en0::1", "0::%25en0::1.2.3.4", "0::%25en0::ffff",
            "0::%25en0::fffff", "0::%25en0::g", "0::0", "0::1", "0::1.2.3.4", "0::ffff",
            "1::", "1::%25en0", "1::%25en0::", "1::%25en0::%25en0", "1::%25en0::0",
            "1::%25en0::01.2.3.4", "1::%25en0::1", "1::%25en0::1.2.3.4", "1::%25en0::ffff",
            "1::%25en0::fffff", "1::%25en0::g", "1::0", "1::1", "1::1.2.3.4", "1::ffff",
            "::", "::%25en0", "::%25en0::", "::%25en0::%25en0", "::%25en0::0",
            "::%25en0::01.2.3.4", "::%25en0::1", "::%25en0::1.2.3.4", "::%25en0::ffff",
            "::%25en0::fffff", "::%25en0::g", "::0", "::1", "::1.2.3.4", "::ffff",
            "ffff::", "ffff::%25en0", "ffff::%25en0::", "ffff::%25en0::%25en0",
            "ffff::%25en0::0", "ffff::%25en0::01.2.3.4", "ffff::%25en0::1",
            "ffff::%25en0::1.2.3.4", "ffff::%25en0::ffff", "ffff::%25en0::fffff",
            "ffff::%25en0::g", "ffff::0", "ffff::1", "ffff::1.2.3.4", "ffff::ffff",
        ]
        XCTAssertEqual(goAccepts.count, 60)
        XCTAssertTrue(goAccepts.isSubset(of: literals))

        var accepted = Set<String>()
        for literal in literals where Mentions.personId(fromGlobalId: "gid://[\(literal)]/Person/1") != nil {
            accepted.insert(literal)
        }
        XCTAssertEqual(accepted, goAccepts)
        // And the port outside the brackets does not change which host it is.
        for literal in goAccepts {
            XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://[\(literal)]:80/Person/1"), 1, literal)
        }
    }

    /// The host and the userinfo have DIFFERENT alphabets, and neither is "any
    /// printable byte". Go runs the host through `unescape(_, encodeHost)`,
    /// which refuses an ASCII byte the host grammar requires to be escaped, and
    /// the userinfo through `validUserinfo`, which is an allowlist over runes —
    /// so every non-ASCII byte fails it, while a non-ASCII byte in the HOST is
    /// fine.
    ///
    /// Both alphabets are swept here rather than asserted: every printable ASCII
    /// byte but `/` — 94 of them, space included, since printable ASCII is
    /// 0x20–0x7E and an off-by-one there would silently drop the one byte most
    /// likely to be mishandled — placed once in each position, compared against
    /// the set Go accepts. Reading `\\`, `^`, a backtick, `{`, `|` and `}` as
    /// host bytes is six mentions Go does not report; the userinfo list is
    /// longer. A third alphabet, the zone's, is swept in its own test.
    func testTheHostAndUserinfoAlphabetsAreGos() {
        let swept = (UInt8(ascii: " ")...UInt8(ascii: "~"))
            .map { Character(UnicodeScalar($0)) }
            .filter { $0 != "/" }
        XCTAssertEqual(swept.count, 94)

        let goAcceptsInHost = Set("!\"$%&'()*+,-.0123456789;<=>@ABCDEFGHIJKLMNOPQRSTUVWXYZ]_abcdefghijklmnopqrstuvwxyz~")
        let goAcceptsInUserinfo = Set("!$%&'()*+,-.0123456789:;=@ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz~")

        XCTAssertEqual(
            Set(swept.filter { Mentions.personId(fromGlobalId: "gid://b\($0)c3/Person/1") != nil }),
            goAcceptsInHost)
        XCTAssertEqual(
            Set(swept.filter { Mentions.personId(fromGlobalId: "gid://b\($0)c3@bc3/Person/1") != nil }),
            goAcceptsInUserinfo)

        // A non-ASCII byte is a host Go reads and a userinfo it refuses.
        XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://b\u{00E9}c3/Person/1"), 1)
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://b\u{00E9}c3@bc3/Person/1"))
    }

    /// A zone identifier's escapes obey the MIRROR of the host's rule, and
    /// reading the second rule as simply absent is permissive across most of the
    /// byte space.
    ///
    /// In a host an escape exists only to spell a byte you could not write, so
    /// `%41` is refused and `%C3` is fine. In a zone Go says the opposite — "you
    /// can use escaping in the zone identifier but not to introduce bytes you
    /// couldn't just write directly" — so `%C3` is refused and `%41` is fine,
    /// with `%25` and `%20` (Windows puts spaces in zone names) excepted. The
    /// two rules are not one rule, and they are not ordered by strictness.
    ///
    /// Both are swept here over all 256 byte values rather than asserted.
    func testAZoneEscapeAndAHostEscapeAllowOppositeBytes() {
        let everyByte = (0...255)

        // A host escape: non-ASCII only, plus `%25`.
        let goAcceptsInHostEscape = Set(everyByte.filter { $0 >= 0x80 || $0 == 0x25 })
        XCTAssertEqual(goAcceptsInHostEscape.count, 129)
        XCTAssertEqual(
            Set(everyByte.filter {
                Mentions.personId(fromGlobalId: String(format: "gid://b%%%02Xc3/Person/1", $0)) != nil
            }),
            goAcceptsInHostEscape)

        // A zone escape: a literal host byte, plus space, plus `%25`. Measured
        // through Go rather than derived from the sentence above.
        let goAcceptsInZoneEscape = Set(
            Array(" !\"$%&'()*+,-.0123456789:;<=>ABCDEFGHIJKLMNOPQRSTUVWXYZ[]_abcdefghijklmnopqrstuvwxyz~"
                .unicodeScalars.map { Int($0.value) }))
        XCTAssertEqual(goAcceptsInZoneEscape.count, 85)
        XCTAssertEqual(
            Set(everyByte.filter {
                Mentions.personId(fromGlobalId: String(format: "gid://[::1%%25%%%02X]/Person/1", $0)) != nil
            }),
            goAcceptsInZoneEscape)

        // With no zone rule at all every one of the 256 would be accepted, so
        // the hole is 171 byte values wide — and the two rules overlap on
        // exactly one byte, `%` itself.
        XCTAssertEqual(256 - goAcceptsInZoneEscape.count, 171)
        XCTAssertEqual(goAcceptsInHostEscape.intersection(goAcceptsInZoneEscape), [0x25])
        XCTAssertNil(Mentions.personId(fromGlobalId: "gid://[::1%25%C3%A9]/Person/1"))
        XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://[::1%25%20]/Person/1"), 1)
        XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://[::1%25%41]/Person/1"), 1)
    }

    /// Go's control-character check runs on the URL with the fragment ALREADY
    /// cut off, and only the fragment — `Parse` splits at the first `#` before
    /// `parse` ever sees the string, while the query is split inside `parse`,
    /// after the check. So a control in a fragment is a gid Go reads and a
    /// control in a query is not.
    ///
    /// Checking the whole string instead reads as the safer choice and is not
    /// one: it guards nothing, because the fragment is discarded either way, and
    /// it loses every mention whose sgid carries a stray control after a `#`.
    func testAControlIsRefusedWhereGoRefusesItAndNotInTheFragment() {
        // 33 controls, plus space — which is not a control and is here to be
        // the one byte of the 34 that the refusals below must skip.
        let controls = (0...0x20).map { $0 } + [0x7F]
        XCTAssertEqual(controls.count, 34)
        let scalars = controls.map { Character(UnicodeScalar(UInt8($0))) }

        // Accepted in a fragment, every one of them, at either position.
        for c in scalars {
            XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://bc3/Person/1#\(c)"), 1, c.debugDescription)
            XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://bc3/Person/1#a\(c)b"), 1, c.debugDescription)
        }
        // Refused everywhere else — in the query, the host and the path tail —
        // with space the one byte that is not a control.
        for c in scalars where c != " " {
            XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/1?\(c)"), c.debugDescription)
            XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3\(c)/Person/1"), c.debugDescription)
            XCTAssertNil(Mentions.personId(fromGlobalId: "gid://bc3/Person/1\(c)"), c.debugDescription)
        }
        XCTAssertEqual(Mentions.personId(fromGlobalId: "gid://bc3/Person/1? "), 1)
        // The cut is at the FIRST `#`, as `strings.Cut` makes it: with a second
        // one, a control before it is still inside the fragment. Cutting at the
        // last `#` instead would refuse these, and every row above carries only
        // one `#`, so nothing above can tell the two apart.
        for c in scalars {
            XCTAssertEqual(
                Mentions.personId(fromGlobalId: "gid://bc3/Person/1#\(c)#y"), 1, c.debugDescription)
            XCTAssertEqual(
                Mentions.personId(fromGlobalId: "gid://bc3/Person/1#a#\(c)"), 1, c.debugDescription)
        }
    }

    /// Go's `encoding/json` substitutes U+FFFD for text it cannot read as UTF-8;
    /// `JSONSerialization` refuses the whole DOCUMENT. So one stray byte
    /// anywhere in the envelope — in a field this code never looks at — loses an
    /// sgid Go decodes.
    ///
    /// Two substitutions, because Go makes two: an invalid UTF-8 sequence, and a
    /// `\uXXXX` escape naming an unpaired surrogate, which is all-ASCII on the
    /// wire and so survives the first.
    ///
    /// Every expectation was produced by running the sgid through
    /// `basecamp.PersonIDFromSGID`.
    func testAnUnreadableByteInTheEnvelopeDoesNotLoseTheMention() {
        func sgid(_ payload: [UInt8]) -> String {
            var encoded = Data(payload).base64EncodedString()
            while encoded.hasSuffix("=") { encoded.removeLast() }
            return encoded
        }
        func envelope(_ extra: [UInt8]) -> [UInt8] {
            Array(#"{"x":""#.utf8) + extra
                + Array(#"","_rails":{"data":"gid://bc3/Person/7","pur":"attachable"}}"#.utf8)
        }

        // Invalid UTF-8 in a field nothing reads.
        for stray: [UInt8] in [[0xFF], [0xC3], [0x80], [0xE2, 0x82], [0xED, 0xA0]] {
            XCTAssertEqual(Mentions.personId(fromAttachableSgid: sgid(envelope(stray))), 7, "\(stray)")
        }
        // An unpaired surrogate escape, high and low.
        for escape in ["\\ud800", "\\udc00", "\\udbff", "\\udfff", "\\ud800\\ud800", "\\udc00\\ud800"] {
            XCTAssertEqual(
                Mentions.personId(fromAttachableSgid: sgid(envelope(Array(escape.utf8)))), 7, escape)
        }
        // A PAIRED surrogate escape is a character, not a substitution, and must
        // still decode; and `\\ud800` is a literal backslash then `ud800`, which
        // is not an escape at all.
        for escape in ["\\ud83d\\ude00", "\\\\ud800", "\\u0041", "ok"] {
            XCTAssertEqual(
                Mentions.personId(fromAttachableSgid: sgid(envelope(Array(escape.utf8)))), 7, escape)
        }
        // The walk is over unicode scalars, not `Character`s, and this is the
        // half no ASCII shape can reach: a combining mark after the opening
        // quote makes `"` + mark ONE grapheme cluster, so a `Character`-level
        // walk never enters the string, never substitutes the surrogate, and
        // loses the document. Every escape above sits directly after a plain
        // quote and so cannot tell the two walks apart.
        for hidden in ["\u{0301}\\ud800", "\u{200D}\\ud800", "\u{FE0F}\\udfff"] {
            XCTAssertEqual(
                Mentions.personId(fromAttachableSgid: sgid(envelope(Array(hidden.utf8)))), 7,
                hidden.debugDescription)
        }
        // And in a KEY, where the opening quote is just as hideable.
        let keyed = Array(
            ("{\"\u{0301}\\ud800\":\"a\",\"_rails\":{\"data\":\"gid://bc3/Person/7\","
                + "\"pur\":\"attachable\"}}").utf8)
        XCTAssertEqual(Mentions.personId(fromAttachableSgid: sgid(keyed)), 7)

        // The substitution does not make a broken document readable.
        for broken in ["{\"_rails\":", "{\"_rails\":{\"data\":\"gid://bc3/Person/7\""] {
            XCTAssertNil(Mentions.personId(fromAttachableSgid: sgid(Array(broken.utf8))), broken)
        }
        // A trailing comma runs the OTHER way: Foundation accepts it and Go's
        // scanner does not, so it is an sgid that would be read here and nowhere
        // else. Refused explicitly, since which Foundation is underneath is not
        // something this can depend on.
        for trailing in [
            #"{"_rails":{"data":"gid://bc3/Person/7","pur":"attachable"},}"#,
            #"{"_rails":{"data":"gid://bc3/Person/7","pur":"attachable",}}"#,
            #"{"_rails":{"data":"gid://bc3/Person/7","pur":"attachable"},"x":[1,]}"#,
        ] {
            XCTAssertNil(Mentions.personId(fromAttachableSgid: sgid(Array(trailing.utf8))), trailing)
        }
        // A comma that is NOT trailing is untouched, including one inside a
        // string, which the scan must not read as structure.
        for fine in [
            #"{"_rails":{"data":"gid://bc3/Person/7","pur":"attachable"},"x":[1,2]}"#,
            #"{"x":",}","_rails":{"data":"gid://bc3/Person/7","pur":"attachable"}}"#,
            #"{"x":", ]","_rails":{"data":"gid://bc3/Person/7","pur":"attachable"}}"#,
        ] {
            XCTAssertEqual(Mentions.personId(fromAttachableSgid: sgid(Array(fine.utf8))), 7, fine)
        }
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

    /// HTML character references, measured against Go and pinned on BOTH what
    /// the decoder produces and what the projection reports.
    ///
    /// Both columns, because the id column alone cannot see this: every row whose
    /// expectation is "no mention" is reached by a hundred routes, so a decoder
    /// that did NOTHING satisfied eleven of the sixteen id-only rows this
    /// replaces — including the row written to guard `&hyphen;`, which did not
    /// fail when that exact regression was reintroduced. The decoded column
    /// discriminates; the id column keeps the rows tied to the contract.
    ///
    /// Checked by mutation, and no row here is vacuous. Replacing the decoder
    /// with the identity function flips 14 of the 18; each of the four that
    /// survive states a rule an identity decoder happens to satisfy, and each is
    /// caught by a mutation aimed at that rule — relaxing the "no characters
    /// matched" guard flips `&#9x`, stopping the `x` counting toward the index
    /// flips `&#x9x`, accepting a fullwidth digit flips its own row, and adding
    /// a name to the table flips the unknown-name row. A survivor is not
    /// evidence of a worthless row until a second mutation says so.
    ///
    /// The id column is `MentionedPersonIDs` from `go/pkg/basecamp`, run on
    /// these exact inputs. The decoded column is this decoder's own output,
    /// and it equals `html.UnescapeString` on every row but the two marked —
    /// where this is deliberately narrower and the verdict is identical anyway,
    /// because the character Go produces is neither base64 nor whitespace.
    ///
    /// The boundaries are not guessable, which is why they are pinned rather
    /// than reasoned about: `&#9x` is literal while `&#x9x` is a tab, because
    /// the `x` counts toward the same index Go tests; `&#133;` is an ellipsis
    /// rather than the NEL that would have been trimmed; `&#8203;` survives the
    /// trim because U+200B is whitespace to Foundation and not to Go; a
    /// fullwidth digit is not a digit; and an overflowing reference wraps onto a
    /// real character, because Go accumulates into an `int32` rune.
    func testEntityDecodingMatchesTheGoImplementationRowForRow() {
        let cases: [(name: String, html: String, decoded: String, ids: [Int])] = [
            ("a named reference is matched longest-first, not by the longest run of name characters",
             "&nbspeyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\u{A0}eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             [42]),
            ("the same name with its semicolon",
             "&nbsp;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\u{A0}eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             [42]),
            ("a single character after &# is not a reference",
             "&#9xeyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "&#9xeyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("...but in hex the x counts toward the same index, so one digit is enough",
             "&#x9xeyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\txeyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("...and a semicolon makes the decimal one a reference too",
             "&#9;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\teyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             [42]),
            ("0x80-0x9F is remapped through Windows-1252, so this is an ellipsis, not the NEL that would be trimmed",
             "&#133;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\u{2026}eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("U+200B is not whitespace to Go, so it is not trimmed away",
             "&#8203;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\u{200B}eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("U+00A0 is",
             "&#160;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\u{A0}eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             [42]),
            ("a two-scalar expansion, all of it whitespace",
             "&ThickSpace;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\u{205F}\u{200A}eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             [42]),
            ("NUL becomes U+FFFD",
             "&#0;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\u{FFFD}eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("a surrogate becomes U+FFFD",
             "&#xD800;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "\u{FFFD}eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("a wrapped overflow lands back on a real character, as Go's int32 rune does",
             "&#x100000042;yJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "ByJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("a fullwidth digit is not a digit",
             "&#x\u{FF14}\u{FF12};yJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "&#x\u{FF14}\u{FF12};yJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("an unknown name is left verbatim",
             "&notaname;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "&notaname;eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),  // narrower than Go here, verdict-identical
            ("a legacy name expands without its semicolon",
             "&ampeyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "&eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("UnderBar spells an underscore",
             "eyJfcmFpbHMiOnsiZGF0&UnderBar;YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "eyJfcmFpbHMiOnsiZGF0_YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
            ("hyphen does NOT spell a hyphen",
             "eyJfcmFpbHMiOnsiZGF0&hyphen;YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "eyJfcmFpbHMiOnsiZGF0&hyphen;YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),  // narrower than Go here, verdict-identical
            ("lowbar inside the payload",
             "eyJfcmFpbHMiOnsiZGF0&lowbar;YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             "eyJfcmFpbHMiOnsiZGF0_YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19--deadbeef",
             []),
        ]
        for row in cases {
            let wrapped = "<div><bc-attachment sgid=\"\(row.html)\"></bc-attachment></div>"
            XCTAssertEqual(
                Mentions.attachmentSgids(in: wrapped), [row.decoded],
                "decoded: \(row.name)")
            XCTAssertEqual(
                Mentions.personIds(in: wrapped), row.ids, "projected: \(row.name)")
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

    /// The `--` separator is found by a BYTE scan, as `strings.LastIndex` finds
    /// it, and this is the last place in the file where it was not.
    ///
    /// `String.range(of:options:.backwards)` searches grapheme clusters, so a
    /// `--` whose second dash carries a combining mark is invisible to it and
    /// plain to Go. The two sides then split the envelope at DIFFERENT places:
    /// in `<payload>--x--\u{0301}` Go splits at the last `--` and reads `…--x`,
    /// which is not base64, while this split at the first and read a person.
    /// That is the accepting direction, and it reaches the WRITE side too —
    /// `markup(for:)`'s gate is "does this sgid name this person", and the sgid
    /// carries no character that gate escapes, so a tag Go refuses to write
    /// would have been rendered.
    ///
    /// The alphabet-mapping step had the same shape:
    /// `replacingOccurrences(of: "-", with: "+")` leaves a `-` that carries a
    /// mark alone, where Go's replacer maps the byte.
    ///
    /// Six marks, each in six positions, plus the four unmarked controls. Every
    /// expectation was produced by running the sgid through
    /// `basecamp.PersonIDFromSGID`.
    func testTheSeparatorIsFoundByByteAsGoFindsIt() {
        let seven = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNyIsInB1ciI6ImF0dGFjaGFibGUifX0"
        let nine = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vOSIsInB1ciI6ImF0dGFjaGFibGUifX0"
        for mark in ["\u{0301}", "\u{0300}", "\u{0308}", "\u{FE0F}", "\u{20E3}", "\u{200D}"] {
            // The mark sits on the LAST separator, so Go splits there and finds
            // a payload that is not base64.
            XCTAssertNil(Mentions.personId(fromAttachableSgid: "\(seven)--x--\(mark)"))
            XCTAssertNil(Mentions.personId(fromAttachableSgid: "\(seven)--x--\(mark)sig"))
            XCTAssertNil(Mentions.personId(fromAttachableSgid: "\(seven)--\(nine)--\(mark)"))
            // The mark sits after the only separator, or inside the digest, or
            // on the payload — none of which moves the split.
            XCTAssertEqual(Mentions.personId(fromAttachableSgid: "\(seven)--\(mark)sig"), 7)
            XCTAssertEqual(Mentions.personId(fromAttachableSgid: "\(seven)--sig\(mark)"), 7)
            XCTAssertNil(Mentions.personId(fromAttachableSgid: "\(seven)\(mark)--sig"))
        }
        // The unmarked controls, so a regression cannot be read as "marks are
        // simply refused".
        XCTAssertEqual(Mentions.personId(fromAttachableSgid: seven), 7)
        XCTAssertEqual(Mentions.personId(fromAttachableSgid: "\(seven)--sig"), 7)
        XCTAssertNil(Mentions.personId(fromAttachableSgid: "\(seven)--x--sig"))
        XCTAssertNil(Mentions.personId(fromAttachableSgid: "\(seven)--\(nine)--sig"))
        // And the same sgid on the write side, which is where it would have
        // rendered a tag Go will not write: the gate is "does this sgid name
        // this person", and the sgid carries no character the markup check
        // escapes, so only the split decides it.
        XCTAssertThrowsError(
            try Mentions.markup(for: person(7, "\(seven)--x--\u{0301}")))
        // Both base64 alphabets, since the mapping step is the other place a
        // grapheme-level search sat: `replacingOccurrences(of: "-", with: "+")`
        // leaves a dash that carries a mark alone where Go's replacer maps the
        // byte. These two payloads are the same envelope written in the standard
        // and the URL-safe alphabet, and the URL-safe one exercises BOTH
        // substitutions — it carries a `-` and a `_`.
        let standard =
            "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNyIsInB1ciI6ImF0dGFjaGFibGUifSwieCI6"
            + "Ij1+Q1soNzhiLTQ/ZlVwcyJ9"
        let urlSafe =
            "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNyIsInB1ciI6ImF0dGFjaGFibGUifSwieCI6"
            + "Ij1-Q1soNzhiLTQ_ZlVwcyJ9"
        XCTAssertTrue(urlSafe.contains("-") && urlSafe.contains("_"), "precondition")
        XCTAssertEqual(Mentions.personId(fromAttachableSgid: standard), 7)
        XCTAssertEqual(Mentions.personId(fromAttachableSgid: urlSafe), 7)
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

        func envelope(_ text: String) -> String? { Mentions.envelopeGlobalId(Array(text.utf8)[...]) }

        XCTAssertEqual(envelope(padded), "gid://bc3/Person/42")
        XCTAssertNil(
            envelope(padded + "\n"),
            "the padding is no longer trailing, so it stays — and an interior = is illegal")
        XCTAssertEqual(
            envelope(String(padded.dropLast()) + "\n="),
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
            Mentions.envelopeGlobalId(Array(payload.utf8)[...]) == "gid://bc3/Person/42"
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

    /// The write-side dedupe turned against itself, which is the direction none
    /// of the other entity findings pointed in.
    ///
    /// HTML5 drops a numeric reference naming a C0 control to the empty string;
    /// Go emits the character. A decoder that followed HTML5 here would unescape
    /// `sgid="<real sgid>&#1;"` to exactly the real sgid, the dedupe would match
    /// what the people read returned, and the mention would be SKIPPED as
    /// already present — an attacker suppressing a real mention through the very
    /// rule that makes the dedupe sound. Go keeps the control, the strings
    /// differ, and the mention is written.
    func testAControlCharacterCannotSuppressAMention() throws {
        let victor = Person(id: 42, name: "Victor", attachableSgid: railsJSONSgid)
        let poisoned = "<div><bc-attachment sgid=\"\(railsJSONSgid)&#1;\"></bc-attachment> hi</div>"

        XCTAssertEqual(
            Mentions.attachmentSgids(in: poisoned), [railsJSONSgid + "\u{01}"],
            "the control survives the unescape, as it does in Go")

        let content = try Mentions.adding([victor], to: poisoned)
        XCTAssertEqual(
            Mentions.attachmentSgids(in: content).count, 2,
            "the authoritative mention is written; the decorated tag is not it")
        XCTAssertTrue(Mentions.attachmentSgids(in: content).contains(railsJSONSgid))
    }

    /// `&fjlig;` expands to two ALPHABET characters, and it lives in Go's second
    /// table — the one for two-rune expansions. A classification of the
    /// single-rune table alone misses it, and then an sgid it spells resolves in
    /// Go and nowhere else.
    func testTheTwoRuneEntitiesAreInTheTable() {
        XCTAssertEqual(
            Mentions.attachmentSgids(in: "<bc-attachment sgid=\"&fjlig;x\"></bc-attachment>"),
            ["fjx"])
        XCTAssertEqual(
            Mentions.attachmentSgids(in: "<bc-attachment sgid=\"&bne;\"></bc-attachment>"),
            ["=\u{20E5}"])
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
