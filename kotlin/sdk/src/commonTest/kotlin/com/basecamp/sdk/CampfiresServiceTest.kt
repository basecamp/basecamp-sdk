package com.basecamp.sdk

import com.basecamp.sdk.generated.campfires
import com.basecamp.sdk.generated.services.CreateCampfireLineBody
import com.basecamp.sdk.generated.services.UpdateCampfireLineBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class CampfiresServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private fun lineJson(id: Long, content: String) = """{
        "id": $id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z",
        "title": "Test line",
        "inherits_status": true,
        "type": "Chat::Lines::Text",
        "url": "https://3.basecampapi.com/12345/buckets/1/chats/42/lines/$id.json",
        "app_url": "https://3.basecamp.com/12345/buckets/1/chats/42/lines/$id",
        "content": "$content",
        "parent": {"id": 42, "title": "Campfire", "type": "Chat::Transcript", "url": "https://3.basecampapi.com/12345/buckets/1/chats/42.json", "app_url": "https://3.basecamp.com/12345/buckets/1/chats/42"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "creator": {"id": 1, "name": "Test User", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"}
    }"""

    @Test
    fun createLine() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertTrue(request.url.encodedPath.contains("/chats/42/lines.json"))
            assertEquals("Bearer test-token", request.headers["Authorization"])
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = lineJson(300, "Hello everyone!"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val line = account.campfires.createLine(
            campfireId = 42,
            body = CreateCampfireLineBody(content = "Hello everyone!"),
        )

        assertEquals(300L, line.id)
        assertEquals("Hello everyone!", line.content)

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("Hello everyone!", bodyJson["content"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun getLine() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/chats/42/lines/300"))

            respond(
                content = lineJson(300, "Hello everyone!"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val line = account.campfires.getLine(campfireId = 42, lineId = 300)

        assertEquals(300L, line.id)
        assertEquals("Chat::Lines::Text", line.type)

        client.close()
    }

    @Test
    fun updateLine() = runTest {
        var capturedMethod: HttpMethod? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedMethod = request.method
            capturedBody = request.body.toByteArray().decodeToString()
            assertTrue(request.url.encodedPath.contains("/chats/42/lines/300"))

            respond(
                content = "",
                status = HttpStatusCode.NoContent,
            )
        }

        val account = client.forAccount("12345")
        account.campfires.updateLine(
            campfireId = 42,
            lineId = 300,
            body = UpdateCampfireLineBody(content = "Edited!"),
        )

        assertEquals(HttpMethod.Put, capturedMethod)

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("Edited!", bodyJson["content"]!!.jsonPrimitive.content)
        assertEquals(setOf("content"), bodyJson.keys)

        client.close()
    }

    @Test
    fun updateLineValidationThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Unprocessable"}""",
                status = HttpStatusCode.UnprocessableEntity,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.campfires.updateLine(
                campfireId = 42,
                lineId = 300,
                body = UpdateCampfireLineBody(content = "Edited!"),
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Validation) {
            assertEquals("Unprocessable", e.message)
        }

        client.close()
    }

    @Test
    fun deleteLine() = runTest {
        var capturedMethod: HttpMethod? = null

        val client = mockClient { request ->
            capturedMethod = request.method
            assertTrue(request.url.encodedPath.contains("/chats/42/lines/300"))

            respond(
                content = "",
                status = HttpStatusCode.NoContent,
            )
        }

        val account = client.forAccount("12345")
        account.campfires.deleteLine(campfireId = 42, lineId = 300)

        assertEquals(HttpMethod.Delete, capturedMethod)

        client.close()
    }
}
