package com.basecamp.sdk

import com.basecamp.sdk.generated.embeds
import com.basecamp.sdk.generated.services.CreateEmbedBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import io.ktor.http.content.TextContent
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

class EmbedsServiceTest {

    private val youtubeUrl = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
    private val slashedSgid =
        "BAh7BkkiC19yYWlscwY6BkVUewdJIglkYXRhBjsAVEkiI2dpZDovL2JjMy9FbWJlZC85OTg/ZXhwaXJlc19pbgY7AFRJIghwdXIGOwBUSSIPYXR0YWNoYWJsZQY7AFQ=--5d2b6e1c9a0f4e8b7c3d2a1f0e9d8c7b6a5f4e3d"

    private fun mockClient(handler: MockRequestHandler): BasecampClient =
        testBasecampClient {
            accessToken("test-token")
            engine = MockEngine(handler)
        }

    private fun jsonHeaders() = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString())

    private val pendingJson = """{"url": "$youtubeUrl", "status": "pending", "sgid": "$slashedSgid"}"""

    private val activeJson = """{
        "url": "$youtubeUrl",
        "status": "active",
        "sgid": "$slashedSgid",
        "type": "video",
        "content_type": "embed/youtube-video",
        "image_url": null,
        "content": ""
    }"""

    @Test
    fun createSendsTheUrlAndReadsThePendingEmbed() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertEquals("/195539477/embeds.json", request.url.encodedPath)
            assertEquals("""{"url":"$youtubeUrl"}""", (request.body as TextContent).text)
            respond(pendingJson, HttpStatusCode.OK, jsonHeaders())
        }

        val embed = client.forAccount("195539477").embeds.create(CreateEmbedBody(url = youtubeUrl))

        assertEquals("pending", embed.status)
        assertEquals(slashedSgid, embed.sgid)
        assertNull(embed.contentType)
        client.close()
    }

    @Test
    fun createReturnsAnExistingActiveEmbed() = runTest {
        val client = mockClient { respond(activeJson, HttpStatusCode.OK, jsonHeaders()) }

        val embed = client.forAccount("195539477").embeds.create(CreateEmbedBody(url = youtubeUrl))

        assertEquals("active", embed.status)
        assertEquals("embed/youtube-video", embed.contentType)
        assertNull(embed.imageUrl)
        assertEquals("", embed.content)
        client.close()
    }

    @Test
    fun createSurfacesA422ForAUrlNoProviderEmbeds() = runTest {
        val client = mockClient {
            respond(
                """{"error": "Basecamp can't embed this URL. Link to it instead."}""",
                HttpStatusCode.UnprocessableEntity,
                jsonHeaders(),
            )
        }

        val e = assertFailsWith<BasecampException.Validation> {
            client.forAccount("195539477").embeds.create(CreateEmbedBody(url = "https://example.com/page"))
        }
        assertEquals(422, e.httpStatus)
        assertEquals("Basecamp can't embed this URL. Link to it instead.", e.message)
        client.close()
    }

    @Test
    fun createSurfacesA429OnTheFirstAttempt() = runTest {
        var attempts = 0
        val client = mockClient {
            attempts++
            respond(
                """{"error": "Too many new embeds. Try again later."}""",
                HttpStatusCode.TooManyRequests,
                jsonHeaders(),
            )
        }

        val e = assertFailsWith<BasecampException.RateLimit> {
            client.forAccount("195539477").embeds.create(CreateEmbedBody(url = youtubeUrl))
        }
        assertEquals(429, e.httpStatus)
        assertEquals(1, attempts)
        client.close()
    }

    @Test
    fun getEscapesTheSgidAsOnePathSegment() = runTest {
        val client = mockClient { request ->
            assertEquals(
                "/195539477/embeds/" + slashedSgid.replace("/", "%2F"),
                request.url.encodedPath.replace("%3D", "="),
            )
            respond(activeJson, HttpStatusCode.OK, jsonHeaders())
        }

        val embed = client.forAccount("195539477").embeds.get(slashedSgid)

        assertEquals("active", embed.status)
        client.close()
    }

    @Test
    fun getSurfacesA404ForAnUnknownSgid() = runTest {
        val client = mockClient { respond("""{"error": "Not found"}""", HttpStatusCode.NotFound, jsonHeaders()) }

        val e = assertFailsWith<BasecampException.NotFound> { client.forAccount("195539477").embeds.get("missing") }
        assertEquals(404, e.httpStatus)
        client.close()
    }
}
