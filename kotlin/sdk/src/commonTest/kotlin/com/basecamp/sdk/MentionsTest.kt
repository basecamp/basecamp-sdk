package com.basecamp.sdk

import com.basecamp.sdk.generated.models.Person
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull
import kotlin.test.assertTrue

class MentionsTest {

    // Marshal 4.8, the older Rails layout, exactly as BC3 serves it:
    // {"gid" => "gid://bc3/Person/1049715915?expires_in", "purpose" =>
    // "attachable", "expires_at" => nil}. The query string is why the gid is
    // parsed as a URL rather than split on "/".
    private val victorSgid =
        "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE0P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9h" +
            "dHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--aabbccdd"
    private val annieSgid =
        "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9h" +
            "dHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102"

    // Rails' JSON message serializer emits the same two envelopes as JSON.
    private val railsJsonSgid = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19"
    private val legacyJsonSgid =
        "eyJnaWQiOiJnaWQ6Ly9iYzMvUGVyc29uLzc3IiwicHVycG9zZSI6ImF0dGFjaGFibGUiLCJleHBpcmVzX2F0IjpudWxsfQ"
    private val readablePurposeSgid = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJyZWFkYWJsZSJ9fQ"
    private val blobSgid = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9BY3RpdmVTdG9yYWdlOjpCbG9iLzkiLCJwdXIiOiJhdHRhY2hhYmxlIn19"
    private val personGidInsideAnotherGid =
        "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9Eb2N1bWVudC9naWQ6Ly9iYzMvUGVyc29uLzQyIiwicHVyIjoiYXR0YWNoYWJsZSJ9fQ"

    private fun person(id: Long, sgid: String?) = Person(id = id, name = "Person $id", attachableSgid = sgid)

    @Test
    fun decodesThePersonIdFromBothMarshalAndJsonEnvelopes() {
        assertEquals(1049715914L, personIdFromSgid(victorSgid))
        assertEquals(42L, personIdFromSgid(railsJsonSgid))
        assertEquals(77L, personIdFromSgid(legacyJsonSgid))
    }

    @Test
    fun refusesAnSgidMintedForAnotherPurpose() {
        // BC3 accepts only the "attachable" purpose in rich text, so a Person
        // sgid minted for bookmarking names nobody here however valid its gid.
        assertNull(personIdFromSgid(readablePurposeSgid))
    }

    @Test
    fun refusesAnSgidThatNamesSomethingOtherThanAPerson() {
        assertNull(personIdFromSgid(blobSgid))
    }

    @Test
    fun refusesAPersonGidThatMerelyAppearsInsideAnotherValue() {
        // The envelope is decoded structurally rather than searched as bytes, so
        // a Document gid built out of a Person gid is not a mention.
        assertNull(personIdFromSgid(personGidInsideAnotherGid))
    }

    @Test
    fun refusesUndecodableAndOversizedSgids() {
        assertNull(personIdFromSgid(""))
        assertNull(personIdFromSgid("not base64 at all !!"))
        assertNull(personIdFromSgid("--"))
        assertNull(personIdFromSgid("A".repeat(10_000)))
    }

    @Test
    fun readsEveryBcAttachmentInDocumentOrderWithoutRepeats() {
        val text = "<div><bc-attachment sgid=\"$victorSgid\"></bc-attachment> and " +
            "<bc-attachment sgid=\"$annieSgid\"></bc-attachment> and " +
            "<bc-attachment sgid=\"$victorSgid\"></bc-attachment></div>"
        assertEquals(listOf(1049715914L, 1049715915L), mentionedPersonIds(text))
    }

    @Test
    fun countsAQuotedMentionBecauseBc3NotifiesIt() {
        val text = "<blockquote><bc-attachment sgid=\"$victorSgid\"></bc-attachment></blockquote>"
        assertEquals(listOf(1049715914L), mentionedPersonIds(text))
    }

    @Test
    fun skipsAMentionInsideACommentOrAnotherTagsAttributeValue() {
        val commented = "<!-- <bc-attachment sgid=\"$victorSgid\"></bc-attachment> --><p>hi</p>"
        assertEquals(emptyList<Long>(), mentionedPersonIds(commented))

        val quoted = "<div title=\"<bc-attachment sgid='$victorSgid'>\">hi</div>"
        assertEquals(emptyList<Long>(), mentionedPersonIds(quoted))
    }

    @Test
    fun readsAttributesRatherThanPatternMatchingThem() {
        // A ">" inside a quoted value does not end the tag; either quote style
        // works; attribute order and case are free; the first sgid wins.
        val text = "<BC-Attachment data-x=\"a>b\" SGID='$victorSgid' sgid=\"$annieSgid\"></bc-attachment>"
        assertEquals(listOf(1049715914L), mentionedPersonIds(text))
    }

    @Test
    fun aTagWhoseNameMerelyStartsWithBcAttachmentIsNotOne() {
        val text = "<bc-attachment-preview sgid=\"$victorSgid\"></bc-attachment-preview>"
        assertEquals(emptyList<Long>(), mentionedPersonIds(text))
    }

    @Test
    fun decodesEntityEscapesInTheSgidValue() {
        val text = "<bc-attachment sgid=\"${railsJsonSgid.replace("e", "&#101;")}\"></bc-attachment>"
        assertEquals(listOf(42L), mentionedPersonIds(text))
    }

    @Test
    fun rendersTheWriteSideTagFromAttachableSgid() {
        assertEquals(
            "<bc-attachment sgid=\"$victorSgid\"></bc-attachment>",
            mentionMarkup(person(1049715914, victorSgid)),
        )
    }

    @Test
    fun refusesToMentionAPersonWithoutAnSgidOrWithSomeoneElses() {
        val noSgid = assertFailsWith<BasecampException.Usage> { mentionMarkup(person(7, null)) }
        assertTrue("no attachable_sgid" in noSgid.message.orEmpty())

        val wrongPerson = assertFailsWith<BasecampException.Usage> { mentionMarkup(person(7, victorSgid)) }
        assertTrue("does not name that person" in wrongPerson.message.orEmpty())

        val malformed = assertFailsWith<BasecampException.Usage> { mentionMarkup(person(7, "a\"b")) }
        assertTrue("malformed" in malformed.message.orEmpty())
    }

    @Test
    fun placesMentionsInsideTheLeadingBlock() {
        val out = withMentions("<div>On it.</div>", listOf(person(1049715915, annieSgid)))
        assertEquals("<div><bc-attachment sgid=\"$annieSgid\"></bc-attachment> On it.</div>", out)

        val withAttributes = withMentions("<p class=\"x\">Hi</p>", listOf(person(1049715915, annieSgid)))
        assertEquals("<p class=\"x\"><bc-attachment sgid=\"$annieSgid\"></bc-attachment> Hi</p>", withAttributes)
    }

    @Test
    fun prefixesContentThatDoesNotOpenWithABlock() {
        val out = withMentions("On it.", listOf(person(1049715915, annieSgid)))
        assertEquals("<bc-attachment sgid=\"$annieSgid\"></bc-attachment> On it.", out)
    }

    @Test
    fun addsNothingForAPersonWhoseExactSgidIsAlreadyPresent() {
        val content = "<div><bc-attachment sgid=\"$annieSgid\"></bc-attachment> already</div>"
        assertEquals(content, withMentions(content, listOf(person(1049715915, annieSgid))))
    }

    @Test
    fun dedupesTheSamePersonPassedTwice() {
        val out = withMentions("Hi", listOf(person(1049715915, annieSgid), person(1049715915, annieSgid)))
        assertEquals("<bc-attachment sgid=\"$annieSgid\"></bc-attachment> Hi", out)
    }

    @Test
    fun deduplicatesOnTheSgidStringNeverOnTheDecodedPersonId() {
        // The trust boundary: an sgid already in the content is unsigned, so a
        // DIFFERENT string naming the same person proves nothing and must not
        // suppress the real mention. Here the content carries a JSON-layout sgid
        // for person 42 and the caller passes the same person under a different
        // (also valid) spelling — the mention is still written.
        val otherSpellingForFortyTwo = "$railsJsonSgid=="
        assertEquals(42L, personIdFromSgid(otherSpellingForFortyTwo))
        val content = "<div><bc-attachment sgid=\"$otherSpellingForFortyTwo\"></bc-attachment> hi</div>"
        val out = withMentions(content, listOf(person(42, railsJsonSgid)))
        assertTrue(out.contains("sgid=\"$railsJsonSgid\""), "the API-returned sgid must still be written: $out")
    }

    @Test
    fun theRoundTripReportsEveryPersonWritten() {
        val out = withMentions("Hi", listOf(person(1049715914, victorSgid), person(1049715915, annieSgid)))
        assertEquals(listOf(1049715914L, 1049715915L), mentionedPersonIds(out))
    }

    @Test
    fun readsAnSgidWhoseEncodedFormCarriesALineBreak() {
        // An HTML attribute may hold a newline and Go's base64 decoder ignores
        // one, so a value Go reads as a mention has to stay one here.
        val wrapped = railsJsonSgid.chunked(40).joinToString("\n")
        assertEquals(42L, personIdFromSgid(wrapped))
    }

    @Test
    fun refusesAGidWhoseAuthorityIsNotOneAUrlParserWouldAccept() {
        // `bc3:abc` is an invalid port, and a control character is refused
        // outright; a URL parser fails on both, so neither names a person. This
        // is the write side too — mentionMarkup asks the same question.
        assertNull(personIdFromSgid(jsonSgidFor("gid://bc3:abc/Person/42")))
        assertNull(personIdFromSgid(jsonSgidFor("gid://b\u0001c3/Person/42")))
        // A numeric port is fine, as it is there.
        assertEquals(42L, personIdFromSgid(jsonSgidFor("gid://bc3:3000/Person/42")))
    }

    @Test
    fun theAuthorityIsJudgedTheWayAUrlParserJudgesIt() {
        // Every row swept against the reference parser rather than derived from
        // a rule someone stated: all 95 printable ASCII bytes planted mid-host,
        // plus the percent, userinfo, port and bracket shapes. The two columns
        // agreed on all 111 cases; these are the ones worth keeping.
        //
        // The rejections a port that only checks the port would miss:
        assertNull(personIdFromSgid(jsonSgidFor("gid://b c3/Person/7")), "a space in the host")
        assertNull(personIdFromSgid(jsonSgidFor("gid://b%zzc3/Person/7")), "a malformed percent escape")
        assertNull(personIdFromSgid(jsonSgidFor("gid://bc3:abc/Person/7")), "a non-numeric port")
        assertNull(personIdFromSgid(jsonSgidFor("gid://[::1/Person/7")), "an unclosed bracket")
        assertNull(personIdFromSgid(jsonSgidFor("gid://a^b@bc3/Person/7")), "userinfo the parser refuses")
        assertNull(personIdFromSgid(jsonSgidFor("gid://user@/Person/7")), "an empty host behind userinfo")
        // A `[` anywhere but the start is an INVALID IP-literal, not a name
        // character — even though the character set permits `[` so that a
        // bracketed literal can carry one.
        assertNull(personIdFromSgid(jsonSgidFor("gid://b[c3/Person/7")), "a bracket mid-host")

        // A percent escape is refused only when it names an ASCII byte, and
        // `%25` is exempt. Refusing them all would lose real hosts — the
        // vanishing direction.
        assertNull(personIdFromSgid(jsonSgidFor("gid://b%41c3/Person/7")), "an escape naming an ASCII byte")
        assertEquals(7L, personIdFromSgid(jsonSgidFor("gid://b%C3%A9c3/Person/7")), "a non-ASCII escape is a host")
        assertEquals(7L, personIdFromSgid(jsonSgidFor("gid://b%25c3/Person/7")), "%25 is the exemption")

        // And the rows the reference ACCEPTS must keep working — being stricter
        // than it loses real mentions just as silently.
        assertEquals(7L, personIdFromSgid(jsonSgidFor("gid://user@bc3/Person/7")), "userinfo")
        assertEquals(7L, personIdFromSgid(jsonSgidFor("gid://bc3:3000/Person/7")), "a numeric port")
        assertEquals(7L, personIdFromSgid(jsonSgidFor("gid://[::1]:80/Person/7")), "a bracketed host with a port")
        assertEquals(7L, personIdFromSgid(jsonSgidFor("gid://bc3]/Person/7")), "a closing bracket alone is a name")
        // `"` and `#` are absent deliberately: the envelope carrying these gids
        // is JSON, so it cannot hold a raw quote, and `#` opens a fragment. The
        // sweep could not reach either byte by this route, so neither is claimed.
        for (permitted in listOf('<', '>', ']', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=')) {
            assertEquals(
                7L,
                personIdFromSgid(jsonSgidFor("gid://b${permitted}c3/Person/7")),
                "the reference accepts $permitted in a host",
            )
        }
    }

    /** Builds an unsigned JSON-layout attachable sgid for an arbitrary gid. */
    @Test
    fun theShapesThatOnlyASweepFindsAreJudgedTheWayTheReferenceJudgesThem() {
        // Each row below was read off the reference parser through the same
        // public entry point, not derived from a rule. They are the shapes a
        // port gets wrong while every hand-written row still passes: a second
        // bracket INSIDE a bracketed literal, a malformed escape in the
        // userinfo half, and a digit that is a digit only to Unicode.
        //
        // `expected` is the reference's own answer; null means it names nobody.
        val rows: List<Pair<String, Long?>> = listOf(
            // A `[` past the first index is an invalid IP-literal wherever it
            // sits — including inside one that already opened. Checking only
            // "a bracket mid-host" lets `[::1[]` and `bc3[` through.
            "gid://b[c3/Person/7" to null,
            "gid://bc3[/Person/7" to null,
            "gid://[bc3/Person/7" to null,
            "gid://[::1[]/Person/7" to null,
            "gid://[::1]/Person/7" to 7L,
            "gid://[::1]x/Person/7" to null,
            "gid://[]/Person/7" to null,
            "gid://[zzz]/Person/7" to null,
            "gid://[::1]:80/Person/7" to 7L,
            "gid://[::1]:/Person/7" to 7L,
            "gid://[::1]:8a/Person/7" to null,
            "gid://a:1]/Person/7" to null,
            // A bracketed literal holding no colon is not an address there: a
            // bare IPv4 is not an IP-literal.
            "gid://[192.0.2.1]/Person/7" to null,
            // The literal is PARSED, not shape-checked. These are the rows a
            // shape check gets wrong in both directions at once.
            "gid://[::1::2]/Person/7" to null,
            "gid://[:%25a]/Person/7" to null,
            "gid://[g::1]/Person/7" to null,
            "gid://[1:2:3:4:5:6:7:8:9]/Person/7" to null,
            "gid://[1:2:3:4:5:6:7]/Person/7" to null,
            "gid://[1:2:3:4:5:6:7::8]/Person/7" to null,
            "gid://[::12345:1]/Person/7" to null,
            "gid://[::1.2.3.256]/Person/7" to null,
            "gid://[::01.2.3.4]/Person/7" to null,
            "gid://[::]/Person/7" to 7L,
            "gid://[1:2:3:4:5:6:7:8]/Person/7" to 7L,
            "gid://[::ffff:192.0.2.1]/Person/7" to 7L,
            "gid://[::1.2.3.4]/Person/7" to 7L,
            // A zone id, spelled the way RFC 6874 requires. A shape check that
            // refused the letters in `eth0` dropped a host the reference reads.
            "gid://[fe80::1%25eth0]/Person/7" to 7L,
            "gid://[fe80::1%eth0]/Person/7" to null,
            "gid://[fe80::1%25]/Person/7" to null,
            "gid://[::1%25e%41t]/Person/7" to 7L,
            // A zone may escape, but not so as to smuggle in a byte it could not
            // have written raw — `%2F` is `/`, which no host may carry.
            "gid://[::1%25a%2Fb]/Person/7" to null,
            // The literal ends at the LAST `]`, not the first: a zone id may
            // carry one, so `[::1%25a]b]` is a host with the zone `a]b`. Taking
            // the first closing bracket loses it.
            "gid://[::1%25a]b]/Person/7" to 7L,
            "gid://[::1%25e%zzt]/Person/7" to null,
            "gid://[::1%25e^t]/Person/7" to null,

            // The userinfo half is PARSED, not just character-checked: `%` is a
            // permitted character but only as the head of a well-formed escape.
            "gid://u%zz@bc3/Person/7" to null,
            "gid://u%z@bc3/Person/7" to null,
            "gid://u%@bc3/Person/7" to null,
            "gid://u%2@bc3/Person/7" to null,
            "gid://a%GG@bc3/Person/7" to null,
            "gid://u%41@bc3/Person/7" to 7L,
            "gid://u%2F@bc3/Person/7" to 7L,
            "gid://a^b@bc3/Person/7" to null,
            "gid://user@bc3/Person/7" to 7L,
            "gid://us:pw@bc3/Person/7" to 7L,

            // An EMPTY port is valid; a non-ASCII digit is not. `Char.isDigit()`
            // is the Unicode Nd category, so a port check written with it accepts
            // fullwidth and Arabic-Indic digits the reference refuses.
            "gid://bc3:80/Person/7" to 7L,
            "gid://bc3:/Person/7" to 7L,
            "gid://bc3:abc/Person/7" to null,
            "gid://bc3:\uFF18\uFF10/Person/7" to null,
            "gid://bc3:\u0668\u0660/Person/7" to null,
            "gid://bc3:\u06F8\u06F0/Person/7" to null,
            "gid://bc3:\u2078\u2070/Person/7" to null,
            "gid://bc3:8\u0660/Person/7" to null,

            // The same door on the id itself, plus the shapes a lenient integer
            // parse would swallow.
            "gid://bc3/Person/\uFF17" to null,
            "gid://bc3/Person/\u0667" to null,
            "gid://bc3/Person/\u06F7" to null,
            "gid://bc3/Person/7\uFF17" to null,
            "gid://bc3/Person/7 " to null,
            "gid://bc3/Person/+7" to null,
            "gid://bc3/Person/07" to 7L,
            "gid://bc3/Person/0" to null,
            "gid://bc3/Person/-7" to null,

            // And the host's own escapes, for the ASCII rule and its exemption.
            "gid://bc%zz3/Person/7" to null,
            "gid://bc%413/Person/7" to null,
            "gid://bc%253/Person/7" to 7L,

            // The reference's "is there a host at all" test runs on the host
            // WITH its port, so a bare port is a host there. Stripping the port
            // first and demanding a non-empty remainder refuses these — the
            // losing direction, and one another port shipped.
            "gid://:8080/Person/7" to 7L,
            "gid://:/Person/7" to 7L,
            "gid://user@:80/Person/7" to 7L,
        )
        // Counted so a row lost to an editing slip shows up as a failure rather
        // than as a smaller sweep that still passes.
        assertEquals(67, rows.size, "the swept rows")
        for ((gid, expected) in rows) {
            assertEquals(expected, personIdFromSgid(jsonSgidFor(gid)), gid)
        }
    }

    private fun jsonSgidFor(gid: String): String {
        val json = "{\"_rails\":{\"data\":\"$gid\",\"pur\":\"attachable\"}}"
        val alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
        val bytes = json.encodeToByteArray()
        val out = StringBuilder()
        var buffer = 0
        var bits = 0
        for (b in bytes) {
            buffer = (buffer shl 8) or (b.toInt() and 0xFF)
            bits += 8
            while (bits >= 6) {
                bits -= 6
                out.append(alphabet[(buffer shr bits) and 0x3F])
            }
        }
        if (bits > 0) out.append(alphabet[(buffer shl (6 - bits)) and 0x3F])
        return out.toString()
    }

    @Test
    fun aControlCharacterInTheGidNamesNobody() {
        // The sharpest case across the ports: a WHATWG-conformant URL parser
        // STRIPS tab, CR and LF before parsing, so an sgid carrying one decodes
        // to a clean person id there and to nothing in Go, whose net/url rejects
        // any control character. That asymmetry reaches the WRITE side —
        // mentionMarkup's only authenticity-adjacent check is "does this sgid
        // name this person", so a more forgiving parser renders and posts a tag
        // Go refuses to write. Measured here rather than reasoned about.
        for (injected in listOf("\n", "\r", "\t", "\u0000", "\u007F")) {
            val gid = "gid://bc3/Person/104${injected}9715915"
            assertNull(personIdFromSgid(jsonSgidFor(gid)), "a gid carrying ${injected.toCharArray()[0].code} names nobody")
        }
        // And the same person id, uninjected, still reads.
        assertEquals(1049715915L, personIdFromSgid(jsonSgidFor("gid://bc3/Person/1049715915")))
    }

    @Test
    fun theWriteSideRefusesAControlCharacterGidToo() {
        val crafted = jsonSgidFor("gid://bc3/Person/104\n9715915")
        val failure = assertFailsWith<BasecampException.Usage> {
            mentionMarkup(Person(id = 1049715915, name = "Victor", attachableSgid = crafted))
        }
        assertTrue("does not name that person" in failure.message.orEmpty())
    }

    @Test
    fun aPersonIdPastSixtyFourBitsNamesNobody() {
        // Go's ParseInt(..., 64) bounds the id; a port whose conversion wraps or
        // saturates would report a different person than the gid names.
        assertNull(personIdFromSgid(jsonSgidFor("gid://bc3/Person/99999999999999999999")))
        assertNull(personIdFromSgid(jsonSgidFor("gid://bc3/Person/0")))
        assertNull(personIdFromSgid(jsonSgidFor("gid://bc3/Person/-1")))
        assertEquals(Long.MAX_VALUE, personIdFromSgid(jsonSgidFor("gid://bc3/Person/${Long.MAX_VALUE}")))
    }

    @Test
    fun theModelNameIsReadFromTheDecodedPath() {
        // Go reads url.Path, which is percent-decoded, so %6f resolves inside the
        // model name. A port comparing the raw path would miss this.
        assertEquals(77L, personIdFromSgid(jsonSgidFor("gid://bc3/Pers%6fn/77")))
        assertNull(personIdFromSgid(jsonSgidFor("gid://bc3/Vault/77")))
    }
}
