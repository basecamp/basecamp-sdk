package com.basecamp.sdk

import com.basecamp.sdk.generated.webhooks
import com.basecamp.sdk.generated.services.CreateWebhookBody
import com.basecamp.sdk.generated.services.UpdateWebhookBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class WebhooksServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return BasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private fun webhookJson(id: Long, payloadUrl: String, active: Boolean = true) = """{
        "id": $id,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z",
        "payload_url": "$payloadUrl",
        "url": "https://3.basecampapi.com/12345/buckets/1/webhooks/$id.json",
        "app_url": "https://3.basecamp.com/12345/buckets/1/webhooks/$id",
        "active": $active,
        "types": ["Todo", "Comment"],
        "recent_deliveries": []
    }"""

    @Test
    fun listWebhooks() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/buckets/1/webhooks.json"))
            assertEquals("Bearer test-token", request.headers["Authorization"])

            respond(
                content = """[
                    ${webhookJson(10, "https://example.com/hook1")},
                    ${webhookJson(11, "https://example.com/hook2", active = false)}
                ]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val webhooks = account.webhooks.list(bucketId = 1)

        assertEquals(2, webhooks.size)
        assertEquals(10L, webhooks[0].id)
        assertEquals("https://example.com/hook1", webhooks[0].payloadUrl)
        assertEquals(true, webhooks[0].active)
        assertEquals(11L, webhooks[1].id)
        assertEquals(false, webhooks[1].active)

        client.close()
    }

    @Test
    fun getWebhook() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/webhooks/10"))

            respond(
                content = webhookJson(10, "https://example.com/hook1"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val webhook = account.webhooks.get(webhookId = 10)

        assertEquals(10L, webhook.id)
        assertEquals("https://example.com/hook1", webhook.payloadUrl)
        assertEquals(true, webhook.active)
        assertEquals(listOf("Todo", "Comment"), webhook.types)

        client.close()
    }

    @Test
    fun createWebhook() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertTrue(request.url.encodedPath.contains("/buckets/1/webhooks.json"))
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = webhookJson(20, "https://example.com/new-hook"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val webhook = account.webhooks.create(
            bucketId = 1,
            body = CreateWebhookBody(
                payloadUrl = "https://example.com/new-hook",
                types = listOf("Todo", "Comment"),
                active = true,
            ),
        )

        assertEquals(20L, webhook.id)
        assertEquals("https://example.com/new-hook", webhook.payloadUrl)

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("https://example.com/new-hook", bodyJson["payload_url"]!!.jsonPrimitive.content)
        assertEquals(2, bodyJson["types"]!!.jsonArray.size)

        client.close()
    }

    @Test
    fun updateWebhook() = runTest {
        var capturedMethod: HttpMethod? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedMethod = request.method
            capturedBody = request.body.toByteArray().decodeToString()
            assertTrue(request.url.encodedPath.contains("/webhooks/10"))

            respond(
                content = webhookJson(10, "https://example.com/updated-hook"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val webhook = account.webhooks.update(
            webhookId = 10,
            body = UpdateWebhookBody(payloadUrl = "https://example.com/updated-hook"),
        )

        assertEquals(HttpMethod.Put, capturedMethod)
        assertEquals(10L, webhook.id)
        assertEquals("https://example.com/updated-hook", webhook.payloadUrl)

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("https://example.com/updated-hook", bodyJson["payload_url"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun deleteWebhook() = runTest {
        var capturedMethod: HttpMethod? = null

        val client = mockClient { request ->
            capturedMethod = request.method
            assertTrue(request.url.encodedPath.contains("/webhooks/10"))

            respond(
                content = "",
                status = HttpStatusCode.NoContent,
            )
        }

        val account = client.forAccount("12345")
        account.webhooks.delete(webhookId = 10)

        assertEquals(HttpMethod.Delete, capturedMethod)

        client.close()
    }

    @Test
    fun webhookNotFoundThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Not found"}""",
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.webhooks.get(webhookId = 999)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.NotFound) {
            assertEquals("Not found", e.message)
        }

        client.close()
    }

    @Test
    fun webhookForbiddenThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Access denied"}""",
                status = HttpStatusCode.Forbidden,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.webhooks.list(bucketId = 1)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Forbidden) {
            assertEquals("Access denied", e.message)
        }

        client.close()
    }
}
