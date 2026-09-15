package com.basecamp.sdk

import kotlin.test.Test
import kotlin.test.assertEquals
import com.basecamp.sdk.generated.models.Person
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The character-reference decoder and the trim that follows it, pinned against
 * the reference implementation row by row.
 *
 * Every row here was MEASURED — a scratch harness fed 185 crafted sgid attribute
 * values through the reference read path and through this one and diffed the
 * verdicts, and the rules below are read off the boundary rather than off a
 * spec. That mattered: the boundary is not where anyone would guess, and each
 * family below was a live divergence before it was measured.
 *
 * Why an unescape bug is worth this much: the decoder sits one layer above the
 * base64 one, on the read path a consumer uses for admission decisions. A
 * reference this decoder fails to resolve leaves a `&` in the value, the base64
 * decode then fails, and a mention the reference implementation reports simply
 * is not there — silently, with no error anywhere.
 */
class EntityDecodingTest {

    private val good =
        "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJIg9h" +
            "dHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102"

    private val plusSeed = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNSIsInB1ciI6ImF0dGFjaGFibGUiLCJ4Ijoi77+9In19"
    private val solSeed = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNSIsInB1ciI6ImF0dGFjaGFibGUiLCJ4Ijoi77+/In19"

    /**
     * An envelope whose base64 contains `fj`. CONSTRUCTED, not searched: `f` is
     * sextet 31 and `j` is 35, which needs the byte `~` at an offset divisible
     * by three followed by one in 0x30..0x3F. A random search over alphanumeric
     * filler runs forever and looks broken — 200,000 trials found none — because
     * the one byte it needs is never in the alphabet.
     */
    private val fjSeed =
        "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNSIsInB1ciI6ImF0dGFjaGFibGUiLCJ4IjoifjAifX0="

    private fun idFor(value: String): Long? =
        mentionedPersonIds("<bc-attachment sgid=\"$value\"></bc-attachment>").firstOrNull()

    @Test
    fun aNamedWhitespaceReferenceLeadingThePayloadIsTrimmedAwayAndTheMentionSurvives() {
        // Each of these expands to whitespace, so the trim removes it and the
        // payload behind it decodes. A decoder that does not know the name leaves
        // the `&` in place and loses the mention instead.
        for (entity in listOf(
            "&nbsp;", "&NonBreakingSpace;", "&ensp;", "&emsp;", "&emsp13;", "&emsp14;",
            "&numsp;", "&puncsp;", "&thinsp;", "&ThinSpace;", "&hairsp;", "&VeryThinSpace;",
            "&MediumSpace;", "&ThickSpace;", "&NewLine;", "&Tab;",
        )) {
            assertEquals(1049715915L, idFor(entity + good), "leading $entity")
        }
    }

    @Test
    fun theNameIsMatchedAgainstTheTableNotByConsumingNameCharacters() {
        // `&nbsp` is the one whitespace reference the reference decoder resolves
        // WITHOUT its semicolon. The greedy reading — consume the longest run of
        // name characters — sees `nbspBAh7…` as one name, matches nothing, and
        // loses the mention. That is the reading anyone would write.
        assertEquals(1049715915L, idFor("&nbsp$good"))
        // A name outside the table stays literal, so the payload does not decode.
        assertNull(idFor("&notaname;$good"))
        assertNull(idFor("&ensp$good"), "only nbsp is semicolon-optional")
        // Zero-width space is NOT whitespace, so it is not trimmed there either.
        assertNull(idFor("&ZeroWidthSpace;$good"))
    }

    @Test
    fun theNumericBoundaryIsWhereTheReferenceImplementationPutsIt() {
        // Decimal with no semicolon needs TWO digits; hex needs one.
        assertNull(idFor("&#9$good"), "one decimal digit, no semicolon: literal")
        assertEquals(1049715915L, idFor("&#32$good"), "two decimal digits, no semicolon: resolves")
        assertEquals(1049715915L, idFor("&#160$good"))
        assertEquals(1049715915L, idFor("&#10$good"))
        assertEquals(1049715915L, idFor("&#13$good"))
        // A semicolon lets a single decimal digit through.
        assertEquals(1049715915L, idFor("&#9;$good"))
        // Hex resolves on one digit, and a trailing letter is taken as a digit.
        assertEquals(1049715915L, idFor("&#x20;$good"))
        assertEquals(1049715915L, idFor("&#x0A;$good"))
        // Nothing at all after the marker stays literal.
        assertNull(idFor("&#;$good"))
        assertNull(idFor("&#$good"))
    }

    @Test
    fun aNumericReferenceInTheWindows1252RangeIsRemappedNotTakenAtFaceValue() {
        // `&#133;` is an ellipsis, not U+0085. Taken at face value it would be
        // NEL — whitespace, therefore trimmed — and this would report a mention
        // the reference implementation does not.
        assertNull(idFor("&#133;$good"))
        assertNull(idFor("&#128;$good"), "an ellipsis and a euro sign are not whitespace")
        // The hex spelling of the same range is remapped identically.
        assertNull(idFor("&#x85;$good"))
    }

    @Test
    fun lineFeedAndCarriageReturnSurviveMidPayloadBecauseTheBase64DecoderSkipsThem() {
        // These two are the reason a whitespace expansion cannot simply be folded
        // to a space: mid-payload, a space fails and these do not.
        assertEquals(1049715915L, idFor(good.substring(0, 20) + "&#10;" + good.substring(20)))
        assertEquals(1049715915L, idFor(good.substring(0, 20) + "&#13;" + good.substring(20)))
        assertNull(idFor(good.substring(0, 20) + "&#32;" + good.substring(20)), "a space mid-payload does not")
    }

    @Test
    fun theTrimIsTheReferenceSpaceSetNotKotlinsWhitespace() {
        // U+0085 (NEL) is trimmed by the reference and is NOT
        // Char.isWhitespace() on the JVM, so delegating to the stdlib loses this
        // one leading character and the whole mention with it.
        val referenceSpaces = listOf(
            '\u0009', '\u000A', '\u000B', '\u000C', '\u000D', '\u0020', '\u0085', '\u00A0',
            '\u1680', '\u2000', '\u2003', '\u2007', '\u2009', '\u200A', '\u2028', '\u2029',
            '\u202F', '\u205F', '\u3000',
        )
        for (space in referenceSpaces) {
            assertEquals(1049715915L, idFor(space + good), "U+%04X".format(space.code))
        }
        // Zero-width space is not in that set.
        assertNull(idFor('\u200B' + good))
    }

    @Test
    fun anAttackerCannotSUPPRESSAMentionByAppendingSomethingThatUnescapesAway() {
        // The other direction from every finding so far. The write side
        // deduplicates against the EXACT attachable_sgid the people read
        // returned — the rule that stops a forged tag standing in for a real
        // mention — and that rule is what makes this reachable: anything in
        // caller-supplied content that unescapes to exactly the authoritative
        // string suppresses the tag, and nothing is written.
        //
        // HTML5 drops a numeric reference naming a C0 control to the EMPTY
        // string; the reference implementation emits the character. A decoder
        // that took the HTML5 reading would see `REAL&#1;` unescape to exactly
        // `REAL`, match, and skip the mention the caller asked for. No fixture
        // covers this, and the failure is silent: the comment posts, without the
        // mention in it.
        val person = Person(id = 1049715915, name = "Annie", attachableSgid = good)
        for (suffix in listOf(
            "&#1;", "&#x1;", "&#0;", "&#8;", "&#11;", "&#31;", "&#127;", "&#x7F;",
            "&ZeroWidthSpace;", "&shy;", "&#8203;", "&#65279;",
        )) {
            val content = "<div><bc-attachment sgid=\"$good$suffix\"></bc-attachment> hi</div>"
            val out = withMentions(content, listOf(person))
            assertTrue(
                out.contains("<bc-attachment sgid=\"$good\"></bc-attachment>"),
                "appending $suffix must not suppress the authorized mention: $out",
            )
        }
        // The control, without which every row above would pass an
        // implementation that never deduplicates at all: with NO suffix the
        // content already carries the authoritative sgid exactly, and the tag
        // must NOT be added a second time.
        val exact = "<div><bc-attachment sgid=\"$good\"></bc-attachment> hi</div>"
        assertEquals(exact, withMentions(exact, listOf(person)), "the dedupe still fires when it should")
    }

    @Test
    fun theTrimmedAndUntrimmedReferencesAreBothPinned() {
        // Both halves of the boundary, each row taken from a run against the
        // reference implementation rather than written by hand. Asserting only
        // "a control character breaks the payload" would be confidently wrong
        // about VT: U+000B IS in the reference's space set, so it is trimmed and
        // the mention survives. A test that pins the convenient half is a
        // tripwire aimed at the wrong thing — the next person to touch the
        // decoder gets a red build for being right.
        for (prefix in listOf("&#9;", "&#10;", "&#11;", "&#12;", "&#13;", "&#32;", "&#160;", "&nbsp;")) {
            assertEquals(1049715915L, idFor(prefix + good), "$prefix expands to whitespace and is trimmed")
        }
        for (prefix in listOf(
            "&#1;", "&#x1;", "&#0;", "&#x0;", "&#8;", "&#31;", "&#127;", "&#x7F;",
            "&ZeroWidthSpace;", "&shy;", "&#8203;", "&#65279;",
        )) {
            assertNull(idFor(prefix + good), "$prefix is not whitespace, so the payload does not decode")
        }
    }

    @Test
    fun anEntityExpandingToABase64CharacterIsResolvedBackIntoThePayload() {
        // The second family that can change a verdict, and the one that hides:
        // these sit INSIDE the payload rather than leading it. A real sgid's
        // base64 can carry `+`, `/` or `=`, and written as `&plus;`, `&sol;` or
        // `&equals;` the reference resolves them back and reads the mention,
        // while a decoder that leaves them literal sees a `&` and finds nothing.
        //
        // The two seeds below are NOT arbitrary. A Marshal envelope is structured
        // ASCII and essentially never reaches the sextets that encode `+` and
        // `/`, so searching real payloads for one looks like a broken search —
        // these were built with non-ASCII filler on purpose.
        val plusSeed = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNSIsInB1ciI6ImF0dGFjaGFibGUiLCJ4Ijoi77+9In19"
        val solSeed = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vMTA0OTcxNTkxNSIsInB1ciI6ImF0dGFjaGFibGUiLCJ4Ijoi77+/In19"
        assertEquals(1049715915L, idFor(plusSeed), "the seed itself decodes")
        assertEquals(1049715915L, idFor(plusSeed.replace("+", "&plus;")))
        assertEquals(1049715915L, idFor(solSeed.replace("/", "&sol;")))
        // `=` is the one a REAL BC3 sgid reaches, since its payload is padded.
        assertEquals(1049715915L, idFor(good.replace("=", "&equals;")))
    }

    @Test
    fun aLeadingNonAsciiSpaceStillTrimsWhenTheDIGESTHalfIsCorrupt() {
        // The trim must decide the boundary characters on their own, not from a
        // property of the whole value. A port that first asks "is this value
        // well-formed?" and picks an ASCII-only alphabet when it is not will
        // leave a non-ASCII space in place and lose the mention — and the byte
        // that made it decide is in the DIGEST half, the part the separator
        // throws away, so it cannot affect the answer at all.
        //
        // On this platform the transport hands over an already-decoded string,
        // so a malformed byte arrives as U+FFFD rather than as itself; that is
        // the shape to pin here. Verified against the reference over the same
        // grid of six leading spaces and five digest corruptions: all thirty
        // resolve there, and all thirty resolve here.
        val payload = good.substringBefore("--")
        for (space in listOf("\u00A0", "\u2002", "\u2028", "\u3000", "\u0085", " ")) {
            for (corruption in listOf("", "\uFFFD", "\uFFFD\uFFFD", "\u0000", "\u00FF")) {
                val sgid = space + payload + "--919d2c8b" + corruption + "11ff403e"
                assertEquals(
                    1049715915L,
                    idFor(sgid),
                    "space U+%04X with %d corrupt char(s) in the digest".format(space[0].code, corruption.length),
                )
            }
        }
    }

    @Test
    fun leadingZerosAreAnOrdinaryWayToWriteACodePoint() {
        // No cap on the digit run, because there cannot be one: `&#00000000065;`
        // is an ordinary spelling of "A", and a length cap justified by "longer
        // than this is out of range anyway" is false the moment a reference is
        // padded. No generated corpus produces leading zeros by accident.
        assertEquals(1049715915L, idFor("&#00000000032;$good"))
        assertEquals(1049715915L, idFor("&#000000000000000000032;$good"))
        assertEquals(1049715915L, idFor("&#x0000000020;$good"))
        assertNull(idFor("&#0000000000065;$good"), "a padded 'A' is still not whitespace")
    }

    @Test
    fun aNumericReferencePastTheRuneRangeWrapsRatherThanSaturating() {
        // The reference accumulates into a rune and signed overflow wraps there,
        // so 2^32 + 65 is "A". Saturating at the rune maximum instead would
        // refuse a base64 character the reference resolves.
        assertEquals(1049715915L, idFor(good.replace("=", "&#4294967357;")), "2^32+61 wraps to '='")
        assertNull(idFor("&#4294967296;$good"), "2^32 wraps to 0, which is the replacement character")
    }

    @Test
    fun everyBase64ExpandingReferenceIsResolvedBackIntoThePayload() {
        // The headline fix of the round that added `fjlig` had NO test. A table
        // is not tested by a row asserting "no mention": a hundred different
        // failures satisfy that, including a decoder that does nothing at all.
        // Each row here asserts a POSITIVE id, so it can only pass if that
        // specific expansion happened.
        //
        // `fjlig` is the one the reference keeps in its two-rune table, which is
        // why a sweep of the single-rune table reported five names where there
        // are six.
        val seeds = mapOf(
            "&equals;" to good.replace("=", "&equals;"),
            "&plus;" to plusSeed.replace("+", "&plus;"),
            "&sol;" to solSeed.replace("/", "&sol;"),
        )
        for ((name, value) in seeds) {
            assertEquals(1049715915L, idFor(value), "$name must resolve back into the payload")
        }
        // fj appears in this payload's base64; written as the reference spells
        // it, the mention must still be found.
        assertEquals(1049715915L, idFor(fjSeed), "the fj seed itself decodes")
        assertEquals(1049715915L, idFor(fjSeed.replace("fj", "&fjlig;")), "&fjlig; expands to fj")
    }

    @Test
    fun theTableTestsCanTellTheFixFromItsAbsence() {
        // A mutation check on the assertions above, done in the test rather than
        // by editing the decoder: each expansion, left literal, must NOT resolve.
        // If these passed, the positive rows above would be satisfied by a
        // decoder that expanded nothing.
        assertNull(idFor(good.replace("=", "&equalsX;")), "a near-miss name must not resolve")
        assertNull(idFor(fjSeed.replace("fj", "&fjligX;")), "a near-miss fjlig must not resolve")
        assertNull(idFor(fjSeed.replace("fj", "&amp;")), "a different expansion must not resolve")
        // And the whitespace family, same check.
        assertNull(idFor("&nbspX;$good"), "a near-miss whitespace name must not resolve")
    }

    @Test
    fun theMarkupPunctuationNamesCarryTheReferencesOwnCasingAndValues() {
        // `Lt` and `Gt` are NOT the markup punctuation in the reference's table:
        // they are the much-less-than and much-greater-than signs. Mapping them
        // to `<` and `>` was the KDoc's "exact casing" claim being false about
        // its own table, and nothing asserted it either way.
        assertEquals("\u226A", unescapeForTest("&Lt;"))
        assertEquals("\u226B", unescapeForTest("&Gt;"))
        assertEquals("<", unescapeForTest("&lt;"))
        assertEquals("<", unescapeForTest("&LT;"))
        assertEquals(">", unescapeForTest("&gt;"))
        assertEquals(">", unescapeForTest("&GT;"))
        assertEquals("&", unescapeForTest("&amp;"))
        assertEquals("&", unescapeForTest("&AMP;"))
        assertEquals("'", unescapeForTest("&apos;"))
        // The casings the reference does NOT carry stay literal.
        for (absent in listOf("&Amp;", "&APOS;", "&Apos;", "&QuOt;", "&Quot;")) {
            assertEquals(absent, unescapeForTest(absent), "$absent is not in the reference's table")
        }
    }

    /**
     * Reads one reference back through the scanner, by putting it in an sgid
     * attribute and taking what the scanner reports. The decoder is internal to
     * the markup walk, so this is the seam a test can reach it through.
     */
    private fun unescapeForTest(reference: String): String =
        bcAttachmentSgids("<bc-attachment sgid=\"$reference\"></bc-attachment>").single()
}
