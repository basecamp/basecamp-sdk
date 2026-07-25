package com.basecamp.sdk

import com.basecamp.sdk.generated.cardTables
import com.basecamp.sdk.generated.services.CreateWormholeBody
import com.basecamp.sdk.generated.services.UpdateWormholeBody
import com.basecamp.sdk.generated.wormholes
import io.ktor.client.engine.mock.*
import io.ktor.client.request.HttpRequestData
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class WormholesServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private fun wormholeJson(id: Long, linked: Boolean = true) = """{
        "id": $id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z",
        "title": "Design → Marketing backlog",
        "inherits_status": true,
        "type": "Kanban::Wormhole",
        "url": "https://3.basecampapi.com/12345/buckets/2085958499/card_tables/wormholes/$id.json",
        "app_url": "https://3.basecamp.com/12345/buckets/2085958499/card_tables/wormholes/$id",
        "parent": { "id": 10, "title": "Development Board", "type": "Kanban::Board", "url": "u", "app_url": "a" },
        "bucket": { "id": 2085958499, "name": "The Leto Laptop", "type": "Project" },
        "creator": { "id": 1, "name": "Victor Cooper" },
        "color": "#f5d76e",
        "linked": $linked,
        "destination_url": ${if (linked) "\"https://3.basecampapi.com/12345/buckets/2085958500/card_tables/columns/1069479500.json\"" else "null"}
    }"""

    @Test
    fun createPostsToBoardScopedWormholesPath() = runTest {
        var capturedRequest: HttpRequestData? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedRequest = request
            capturedBody = request.body.toByteArray().decodeToString()
            respond(
                content = wormholeJson(99),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val wormhole = account.wormholes.create(
            bucketId = 2085958499,
            cardTableId = 1069479345,
            body = CreateWormholeBody(destinationRecordingId = 1069479500),
        )

        assertEquals(99L, wormhole.id)
        assertTrue(wormhole.linked)
        assertNotNull(wormhole.destinationUrl)
        assertEquals(HttpMethod.Post, capturedRequest!!.method)
        assertTrue(capturedRequest!!.url.encodedPath.endsWith("/buckets/2085958499/card_tables/1069479345/wormholes.json"))

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals(1069479500L, bodyJson["destination_recording_id"]!!.jsonPrimitive.content.toLong())

        client.close()
    }

    @Test
    fun createValidationErrorAtLimitThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Limit reached"}""",
                status = HttpStatusCode.UnprocessableEntity,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.wormholes.create(
                bucketId = 2085958499,
                cardTableId = 1069479345,
                body = CreateWormholeBody(destinationRecordingId = 1069479500),
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Validation) {
            assertEquals("Limit reached", e.message)
        }

        client.close()
    }

    @Test
    fun createNotFoundDestinationThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Not found"}""",
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.wormholes.create(
                bucketId = 2085958499,
                cardTableId = 1069479345,
                body = CreateWormholeBody(destinationRecordingId = 999),
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.NotFound) {
            assertEquals("Not found", e.message)
        }

        client.close()
    }

    @Test
    fun updatePutsToWormholeScopedPath() = runTest {
        var capturedRequest: HttpRequestData? = null

        val client = mockClient { request ->
            capturedRequest = request
            respond(
                content = wormholeJson(1069479400),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val wormhole = account.wormholes.update(
            bucketId = 2085958499,
            wormholeId = 1069479400,
            body = UpdateWormholeBody(destinationRecordingId = 1069479501),
        )

        assertEquals(1069479400L, wormhole.id)
        assertEquals(HttpMethod.Put, capturedRequest!!.method)
        assertTrue(capturedRequest!!.url.encodedPath.endsWith("/buckets/2085958499/card_tables/wormholes/1069479400"))

        client.close()
    }

    @Test
    fun updateNotFoundThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Not found"}""",
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.wormholes.update(
                bucketId = 2085958499,
                wormholeId = 999,
                body = UpdateWormholeBody(destinationRecordingId = 1),
            )
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.NotFound) {
            assertEquals("Not found", e.message)
        }

        client.close()
    }

    @Test
    fun deleteSendsDeleteToWormholeScopedPath() = runTest {
        var capturedRequest: HttpRequestData? = null

        val client = mockClient { request ->
            capturedRequest = request
            respond(content = "", status = HttpStatusCode.NoContent)
        }

        val account = client.forAccount("12345")
        account.wormholes.delete(bucketId = 2085958499, wormholeId = 1069479400)

        assertEquals(HttpMethod.Delete, capturedRequest!!.method)
        assertTrue(capturedRequest!!.url.encodedPath.endsWith("/buckets/2085958499/card_tables/wormholes/1069479400"))

        client.close()
    }

    @Test
    fun deleteForbiddenThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Access denied"}""",
                status = HttpStatusCode.Forbidden,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.wormholes.delete(bucketId = 2085958499, wormholeId = 1069479400)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Forbidden) {
            assertEquals("Access denied", e.message)
        }

        client.close()
    }

    @Test
    fun deleteNotFoundThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Not found"}""",
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.wormholes.delete(bucketId = 2085958499, wormholeId = 999)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.NotFound) {
            assertEquals("Not found", e.message)
        }

        client.close()
    }

    @Test
    fun cardTableDecodesLinkedAndUnlinkedWormholes() = runTest {
        val cardTableJson = """{
            "id": 1069479345,
            "status": "active",
            "visible_to_clients": false,
            "created_at": "2025-01-01T00:00:00Z",
            "updated_at": "2025-01-01T00:00:00Z",
            "title": "Development Board",
            "inherits_status": true,
            "type": "Kanban::Board",
            "url": "u",
            "app_url": "a",
            "bucket": { "id": 2085958499, "name": "The Leto Laptop", "type": "Project" },
            "creator": { "id": 1, "name": "Victor Cooper" },
            "wormholes": [
                ${wormholeJson(1069479400, linked = true)},
                ${wormholeJson(1069479401, linked = false)}
            ]
        }"""

        val client = mockClient { _ ->
            respond(
                content = cardTableJson,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val cardTable = account.cardTables.get(cardTableId = 1069479345)

        assertEquals(2, cardTable.wormholes.size)
        assertTrue(cardTable.wormholes[0].linked)
        assertNotNull(cardTable.wormholes[0].destinationUrl)
        assertTrue(!cardTable.wormholes[1].linked)
        assertNull(cardTable.wormholes[1].destinationUrl)

        client.close()
    }
}
