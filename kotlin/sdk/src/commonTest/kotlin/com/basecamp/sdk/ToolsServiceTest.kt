package com.basecamp.sdk

import com.basecamp.sdk.generated.services.CreateToolBody
import com.basecamp.sdk.generated.tools
import io.ktor.client.engine.mock.*
import io.ktor.client.request.HttpRequestData
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class ToolsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private fun toolJson(id: Long, title: String) = """{
        "id": $id,
        "name": "message_board",
        "title": "$title",
        "enabled": true,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z"
    }"""

    @Test
    fun createToolPostsToBucketScopedDockPath() = runTest {
        var capturedRequest: HttpRequestData? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedRequest = request
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = toolJson(800, "Message Board (Copy)"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val tool = account.tools.create(
            bucketId = 456,
            body = CreateToolBody(toolType = "Message::Board", title = "Message Board (Copy)"),
        )

        assertEquals(800L, tool.id)
        assertEquals(HttpMethod.Post, capturedRequest!!.method)
        assertTrue(capturedRequest!!.url.encodedPath.endsWith("/buckets/456/dock/tools.json"))

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("Message::Board", bodyJson["tool_type"]!!.jsonPrimitive.content)
        assertEquals("Message Board (Copy)", bodyJson["title"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun createToolOmitsTitleWhenNotProvided() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = toolJson(801, "Message Board"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        account.tools.create(
            bucketId = 456,
            body = CreateToolBody(toolType = "Message::Board"),
        )

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("Message::Board", bodyJson["tool_type"]!!.jsonPrimitive.content)
        assertFalse(bodyJson.containsKey("title"))

        client.close()
    }
}
