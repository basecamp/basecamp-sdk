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
}
