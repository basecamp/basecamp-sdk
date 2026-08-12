package com.basecamp.sdk

import com.basecamp.sdk.generated.models.SearchResult
import com.basecamp.sdk.generated.search
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.test.runTest
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `search` returns `ListResult<SearchResult>` rather than
 * `ListResult<JsonElement>` (#717). Until then Kotlin was the one tier where
 * the special-branch modelling done in #651 enforced nothing at all: an
 * untyped `JsonElement` decodes any body whatsoever, so a spec that lied about
 * the search projection could not fail here.
 *
 * These tests drive the real generated service through the real client decoder
 * against the shared `spec/fixtures/search/results.json` body — the same file
 * `go/pkg/basecamp/search_test.go` reads, validated against the generated
 * schema by `make check-fixture-coverage`. Reading it from disk rather than
 * restating it inline is deliberate: the point is that Kotlin now decodes the
 * *same bytes* the other five SDKs do, which an invented body cannot show.
 *
 * This is a JVM-only test because `commonTest` has no filesystem. The decoder
 * under test is `commonMain`, so nothing platform-specific is being asserted.
 */
class SearchResultDecodeTest {

    /** Walk up from the working directory to the repo root (the tree holding `openapi.json`). */
    private fun repoRoot(): File {
        var dir: File? = File(".").absoluteFile
        while (dir != null && !File(dir, "openapi.json").isFile) {
            dir = dir.parentFile
        }
        return requireNotNull(dir) { "could not locate repo root from ${File(".").absolutePath}" }
    }

    private fun sharedFixture(): String =
        File(repoRoot(), "spec/fixtures/search/results.json").readText()

    private suspend fun search(body: String): List<SearchResult> {
        val client = testBasecampClient {
            accessToken("test-token")
            engine = MockEngine {
                respond(
                    content = body,
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
        }
        try {
            return client.forAccount("195539477").search.search(q = "Leto")
        } finally {
            client.close()
        }
    }

    @Test
    fun decodesEveryBranchOfTheSharedFixture() = runTest {
        val results = search(sharedFixture())
        assertEquals(8, results.size)

        // /0 — Message. content/description are required-and-nullable: the show
        // template nil-overwrites both on every branch, so a decoder that made
        // them non-nullable would throw here.
        val message = results[0]
        assertEquals(1069479351L, message.id)
        assertEquals("Message", message.type)
        assertEquals("We won Leto!", message.title)
        assertEquals("We won Leto!", message.subject)
        assertNull(message.content)
        assertNull(message.description)
        assertTrue(message.plainTextContent!!.contains("""<mark class="circled-text">"""))
        assertNull(message.plainTextDescription)
        assertEquals("Message::Board", message.parent?.type)
        assertEquals("The Leto Laptop", message.bucket?.name)
        assertEquals("Victor Cooper", message.creator?.name)

        // bubble_up_url rides the polymorphic projection: only Todolist hits
        // reach a partial that passes `bubbleupable: true`.
        for (r in results) {
            if (r.type == "Todolist") {
                assertNotNull(r.bubbleUpUrl, "Todolist result ${r.id}: expected bubbleUpUrl")
            } else {
                assertNull(r.bubbleUpUrl, "${r.type} result ${r.id}: expected no bubbleUpUrl")
            }
        }

        // /4 — file-attachment branch. `searches/_attachment.json.jbuilder`
        // writes its own projection with NONE of the five envelope keys, so a
        // model declaring them required would throw. The branch discriminator
        // is that absence plus `filename`.
        val attachment = results[4]
        assertNull(attachment.id)
        assertNull(attachment.title)
        assertNull(attachment.type)
        assertNull(attachment.url)
        assertNull(attachment.appUrl)
        assertEquals("leto-hero.jpg", attachment.filename)
        assertEquals("image/jpeg", attachment.contentType)
        assertEquals(512000L, attachment.byteSize)
        assertEquals(true, attachment.previewable)
        // Float-spelled on the wire (`1920.0`) — FlexibleIntSerializer narrows
        // it. A plain Int field throws here.
        assertEquals(1920, attachment.width)
        assertEquals(1080, attachment.height)
        assertNotNull(attachment.downloadUrl)
        assertNotNull(attachment.appDownloadUrl)
        assertNotNull(attachment.previewUrl)
        assertNotNull(attachment.thumbnailUrl)
        assertNull(attachment.content)
        assertNull(attachment.description)
        assertEquals("Message", attachment.parent?.type)

        // /5 — chat upload line: a bespoke six-key `attachments` aggregate that
        // does NOT match RichTextAttachment (no id, no sgid, no preview keys),
        // plus the boostable envelope keys.
        val uploadLine = results[5]
        assertEquals("Chat::Lines::Upload", uploadLine.type)
        assertEquals(1, uploadLine.boostsCount)
        assertNotNull(uploadLine.boostsUrl)
        val bespoke = assertNotNull(uploadLine.attachments).single()
        assertEquals("leto-benchmarks.pdf", bespoke.title)
        assertNotNull(bespoke.url)
        assertEquals("leto-benchmarks.pdf", bespoke.filename)
        assertEquals("application/pdf", bespoke.contentType)
        assertEquals(1048576L, bespoke.byteSize)
        assertNotNull(bespoke.downloadUrl)
        assertNull(bespoke.id)
        assertNull(bespoke.sgid)
        assertNull(bespoke.previewable)
        assertNull(bespoke.width)

        // /6 — kanban list: list-partial keys over the envelope. `color` is
        // emitted unconditionally with a null value, so it must stay nullable
        // rather than coerce to "".
        val kanban = results[6]
        assertEquals("Kanban::Column", kanban.type)
        assertNotNull(kanban.subscriptionUrl)
        assertEquals(2, kanban.position)
        assertNull(kanban.color)
        assertEquals(4, kanban.cardsCount)
        assertEquals(1, kanban.commentCount)
        assertNotNull(kanban.cardsUrl)
        assertEquals("Victor Cooper", assertNotNull(kanban.subscribers).single().name)
        val onHold = assertNotNull(kanban.onHold)
        assertEquals(0, onHold.cardsCount)
        assertNotNull(onHold.cardsUrl)

        // /7 — gauge needle: commentable + boostable envelope, the needle's own
        // keys, and the rich-text description companion array surviving the
        // nil-overwrite of `description` itself.
        val needle = results[7]
        assertEquals("Gauge::Needle", needle.type)
        assertEquals(2, needle.commentsCount)
        assertEquals(3, needle.boostsCount)
        assertEquals(2, needle.commentCount)
        assertEquals("green", needle.color)
        assertEquals(72, needle.position)
        assertNull(needle.description)
        assertEquals(1, assertNotNull(needle.descriptionAttachments).size)
        // The generic `attachments` key repeats the companion array through the
        // same partial, so here the rich-text-variant keys ARE populated —
        // the mirror image of the chat upload line above.
        val richText = assertNotNull(needle.attachments).single()
        assertNotNull(richText.id)
        assertNotNull(richText.sgid)
        assertEquals(1024, richText.width)
    }

    /**
     * The regression proof for the retype itself. Against `JsonElement` this
     * body decodes happily; against `SearchResult` it throws, because
     * `content` and `description` have no default and the projection always
     * emits them.
     */
    @Test
    fun missingRequiredNullableMembersThrow() = runTest {
        val e = runCatching { search("""[{"id": 1, "type": "Message"}]""") }.exceptionOrNull()
        assertNotNull(e, "a hit missing content/description must not decode")
    }
}
