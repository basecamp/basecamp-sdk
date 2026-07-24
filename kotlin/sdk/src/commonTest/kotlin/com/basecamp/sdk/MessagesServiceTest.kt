package com.basecamp.sdk

import com.basecamp.sdk.generated.messages
import com.basecamp.sdk.generated.services.CreateMessageBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * visible_to_clients is tri-state: null omits the key, true/false are sent
 * verbatim. An explicit false must reach the wire (?.let fires for false). The
 * shared generator carries this field on all six create ops; this messages
 * coverage stands in for the other five ops.
 */
class MessagesServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun messageJson(id: Long) = """{
        "id": $id, "status": "active", "visible_to_clients": false,
        "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
        "title": "Hello", "inherits_status": true, "type": "Message",
        "url": "https://3.basecampapi.com/1/buckets/1/messages/$id.json",
        "app_url": "https://3.basecamp.com/1/buckets/1/messages/$id",
        "subject": "Hello", "content": "<p>Body</p>",
        "creator": {"id": 1, "name": "Test", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "parent": {"id": 2, "title": "Board", "type": "Message::Board", "url": "https://3.basecampapi.com/1/buckets/1/message_boards/2.json", "app_url": "https://3.basecamp.com/1/buckets/1/message_boards/2"}
    }"""

    @Test
    fun createOmitsVisibleToClientsWhenNull() = runTest {
        var capturedBody: String? = null
        val engine = MockEngine { request ->
            capturedBody = request.body.toByteArray().decodeToString()
            respond(messageJson(99), HttpStatusCode.Created,
                headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()))
        }
        val client = testBasecampClient { accessToken("test-token"); this.engine = engine }
        client.forAccount("12345").messages.create(boardId = 200, body = CreateMessageBody(subject = "Hello"))

        assertFalse(json.parseToJsonElement(capturedBody!!).jsonObject.containsKey("visible_to_clients"))
        client.close()
    }

    @Test
    fun createSendsVisibleToClientsTrue() = runTest {
        var capturedBody: String? = null
        val engine = MockEngine { request ->
            capturedBody = request.body.toByteArray().decodeToString()
            respond(messageJson(99), HttpStatusCode.Created,
                headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()))
        }
        val client = testBasecampClient { accessToken("test-token"); this.engine = engine }
        client.forAccount("12345").messages.create(
            boardId = 200, body = CreateMessageBody(subject = "Hello", visibleToClients = true))

        val obj = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals(true, obj["visible_to_clients"]!!.jsonPrimitive.content.toBoolean())
        client.close()
    }

    @Test
    fun createSendsVisibleToClientsFalse() = runTest {
        var capturedBody: String? = null
        val engine = MockEngine { request ->
            capturedBody = request.body.toByteArray().decodeToString()
            respond(messageJson(99), HttpStatusCode.Created,
                headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()))
        }
        val client = testBasecampClient { accessToken("test-token"); this.engine = engine }
        client.forAccount("12345").messages.create(
            boardId = 200, body = CreateMessageBody(subject = "Hello", visibleToClients = false))

        val obj = json.parseToJsonElement(capturedBody!!).jsonObject
        assertTrue(obj.containsKey("visible_to_clients"))
        assertEquals(false, obj["visible_to_clients"]!!.jsonPrimitive.content.toBoolean())
        client.close()
    }
}
