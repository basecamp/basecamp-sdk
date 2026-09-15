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
    }

    @Test
    fun aC0ControlReferenceKeepsItsCharacterRatherThanVanishing() {
        // The mechanism behind the test above, pinned on its own so a later
        // "simplification" of the decoder toward the HTML5 reading goes red here
        // rather than silently in the write path.
        assertNull(idFor("&#1;$good"), "a C0 control is emitted, so the payload does not decode")
        assertNull(idFor("&#x1;$good"))
        assertNull(idFor("&#127;$good"))
    }
}
