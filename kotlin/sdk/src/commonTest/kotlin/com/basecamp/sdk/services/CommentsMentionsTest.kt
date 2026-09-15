package com.basecamp.sdk.services

import com.basecamp.sdk.BasecampClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.comments
import com.basecamp.sdk.testBasecampClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.request.HttpRequestData
import io.ktor.content.TextContent
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

class CommentsMentionsTest {

    private companion object {
        /** An attachable Person sgid for 1049715915, as BC3 serves it. */
        const val ANNIE_SGID =
            "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJ" +
                "Ig9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102"

        /** And one for 1049715914. */
        const val VICTOR_SGID =
            "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE0P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJ" +
                "Ig9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--aabbccdd"
    }

    private val paths = mutableListOf<String>()
    private val bodies = mutableListOf<String?>()

    private fun personJson(id: Long, sgid: String) =
        """{"id": $id, "name": "Person $id", "attachable_sgid": "$sgid"}"""

    private fun commentJson(content: String) = """{
        "id": 1069479370, "status": "active", "visible_to_clients": false,
        "created_at": "2022-10-31T14:22:33.169Z", "updated_at": "2022-10-31T14:22:33.169Z",
        "title": "Re: We won Leto!", "inherits_status": true, "type": "Comment",
        "url": "https://3.basecampapi.com/999/buckets/1/comments/1069479370.json",
        "app_url": "https://3.basecamp.com/999/buckets/1/comments/1069479370",
        "parent": {"id": 1, "title": "We won Leto!", "type": "Message",
                   "url": "https://3.basecampapi.com/999/buckets/1/messages/1.json",
                   "app_url": "https://3.basecamp.com/999/buckets/1/messages/1"},
        "bucket": {"id": 1, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1049715915, "name": "Annie Bryan"},
        "content": "$content",
        "content_attachments": []
    }"""

    private fun client(route: (HttpRequestData) -> Pair<HttpStatusCode, String>): BasecampClient {
        val engine = MockEngine { request ->
            paths.add(request.url.encodedPath)
            bodies.add((request.body as? TextContent)?.text)
            val (status, body) = route(request)
            respond(
                content = body,
                status = status,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    @Test
    fun resolvesEveryPersonBeforePostingAndWritesTheMentionFromAttachableSgid() = runTest {
        val client = client { request ->
            if (request.url.encodedPath.startsWith("/999/people/")) {
                HttpStatusCode.OK to personJson(1049715915, ANNIE_SGID)
            } else {
                HttpStatusCode.Created to commentJson("posted")
            }
        }
        client.forAccount("999").comments.createWithMentions(1, "<div>On it.</div>", listOf(1049715915))
        assertEquals(listOf("/999/people/1049715915", "/999/recordings/1/comments.json"), paths)
        assertTrue(
            bodies.last().orEmpty().contains("<bc-attachment sgid=\\\"$ANNIE_SGID\\\"></bc-attachment> On it."),
            "the mention is placed inside the content's first block: ${bodies.last()}",
        )
        client.close()
    }

    @Test
    fun readsEachDistinctIdOnceAndKeepsTheRequestedOrder() = runTest {
        val client = client { request ->
            val path = request.url.encodedPath
            when {
                path.endsWith("/1049715915") -> HttpStatusCode.OK to personJson(1049715915, ANNIE_SGID)
                path.endsWith("/1049715914") -> HttpStatusCode.OK to personJson(1049715914, VICTOR_SGID)
                else -> HttpStatusCode.Created to commentJson("posted")
            }
        }
        val expanded = client.forAccount("999").comments.expandMentions(
            "Hi",
            listOf(1049715915, 1049715914, 1049715915),
        )
        assertEquals(listOf("/999/people/1049715915", "/999/people/1049715914"), paths)
        assertEquals(
            "<bc-attachment sgid=\"$ANNIE_SGID\"></bc-attachment> " +
                "<bc-attachment sgid=\"$VICTOR_SGID\"></bc-attachment> Hi",
            expanded,
        )
        client.close()
    }

    @Test
    fun readsThePersonEvenWhenTheContentAlreadyCarriesAMatchingTag() = runTest {
        // The trust boundary: an sgid in caller-supplied content is unsigned, so
        // it can never stand in for the authoritative people read. The read still
        // happens; withMentions then adds nothing, because the sgid the read
        // returned is the exact string already present.
        val client = client { HttpStatusCode.OK to personJson(1049715915, ANNIE_SGID) }
        val content = "<div><bc-attachment sgid=\"$ANNIE_SGID\"></bc-attachment> already</div>"
        val expanded = client.forAccount("999").comments.expandMentions(content, listOf(1049715915))
        assertEquals(listOf("/999/people/1049715915"), paths, "the people read is never skipped")
        assertEquals(content, expanded)
        client.close()
    }

    @Test
    fun aFailedPersonReadPostsNothing() = runTest {
        val client = client { HttpStatusCode.NotFound to """{"error":"Not found"}""" }
        assertFailsWith<BasecampException.NotFound> {
            client.forAccount("999").comments.createWithMentions(1, "<div>Hi</div>", listOf(1049715915))
        }
        assertEquals(listOf("/999/people/1049715915"), paths, "nothing is posted on a partial mention list")
        client.close()
    }

    @Test
    fun refusesAnInvalidMentionIdAndEmptyContentBeforeAnyRequest() = runTest {
        val client = client { error("no request expected") }
        val service = client.forAccount("999").comments
        assertFailsWith<BasecampException.Usage> { service.expandMentions("Hi", listOf(0)) }
        assertFailsWith<BasecampException.Usage> { service.createWithMentions(1, "", listOf(1049715915)) }
        assertTrue(paths.isEmpty())
        client.close()
    }

    @Test
    fun noMentionsMeansNoPeopleReads() = runTest {
        val client = client { HttpStatusCode.Created to commentJson("posted") }
        client.forAccount("999").comments.createWithMentions(1, "<div>Hi</div>", emptyList())
        assertEquals(listOf("/999/recordings/1/comments.json"), paths)
        client.close()
    }
}
