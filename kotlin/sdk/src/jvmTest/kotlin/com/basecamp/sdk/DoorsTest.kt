package com.basecamp.sdk

import com.basecamp.sdk.generated.doors
import com.basecamp.sdk.generated.services.CreateDoorBody
import com.basecamp.sdk.generated.services.UpdateDoorBody
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.MockRequestHandler
import io.ktor.client.engine.mock.respond
import io.ktor.client.engine.mock.toByteArray
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpMethod
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.put
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

/**
 * External links (doors), driven through the shared `spec/fixtures/doors`
 * bodies: bc3's documented examples. JVM-only because `commonTest` has no
 * filesystem.
 */
class DoorsTest {

    private fun repoRoot(): File {
        var dir: File? = File(".").absoluteFile
        while (dir != null && !File(dir, "openapi.json").isFile) dir = dir.parentFile
        return requireNotNull(dir) { "could not locate repo root from ${File(".").absolutePath}" }
    }

    private fun fixture(name: String): String = File(repoRoot(), "spec/fixtures/doors/$name").readText()

    private fun jsonHeaders() =
        headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString())

    private fun client(handler: MockRequestHandler): BasecampClient = testBasecampClient {
        accessToken("test-token")
        engine = MockEngine(handler)
    }

    @Test
    fun listReadsTheBucketScopedPath() = runTest {
        val client = client { request ->
            assertEquals(HttpMethod.Get, request.method)
            assertEquals("/195539477/buckets/2085958504/dock/doors.json", request.url.encodedPath)
            respond(fixture("list.json"), HttpStatusCode.OK, jsonHeaders())
        }
        val doors = client.forAccount("195539477").doors.list(2085958504)
        assertEquals(1, doors.size)
        assertEquals("Door", doors[0].type)
        client.close()
    }

    @Test
    fun getReadsTheFlatPathAndTheOutsideAddress() = runTest {
        val client = client { request ->
            assertEquals("/195539477/dock/doors/1069480290", request.url.encodedPath)
            respond(fixture("get.json"), HttpStatusCode.OK, jsonHeaders())
        }
        val door = client.forAccount("195539477").doors.get(1069480290)
        assertEquals("https://www.figma.com/file/abc123/Design-system", door.url)
        assertEquals("figma", door.service.code)
        assertEquals("<div>Shared Figma workspace</div>", door.description)
        client.close()
    }

    @Test
    fun createSendsTheDocumentedRequest() = runTest {
        var captured: String? = null
        val client = client { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertEquals("/195539477/buckets/2085958504/dock/doors.json", request.url.encodedPath)
            captured = request.body.toByteArray().decodeToString()
            respond(fixture("get.json"), HttpStatusCode.Created, jsonHeaders())
        }
        val expected = Json.parseToJsonElement(fixture("create-request.json")).jsonObject
        client.forAccount("195539477").doors.create(
            2085958504,
            CreateDoorBody(door = expected.getValue("door").jsonObject, visibleToClients = false),
        )
        assertEquals(expected, Json.parseToJsonElement(captured!!).jsonObject)
        client.close()
    }

    @Test
    fun updateCanChangeOnlyVisibility() = runTest {
        var captured: String? = null
        val client = client { request ->
            assertEquals(HttpMethod.Put, request.method)
            assertEquals("/195539477/dock/doors/1069480290", request.url.encodedPath)
            captured = request.body.toByteArray().decodeToString()
            respond(fixture("get.json"), HttpStatusCode.OK, jsonHeaders())
        }
        client.forAccount("195539477").doors.update(1069480290, UpdateDoorBody(visibleToClients = true))
        assertEquals(buildJsonObject { put("visible_to_clients", true) }, Json.parseToJsonElement(captured!!).jsonObject)
        client.close()
    }

    @Test
    fun deleteAnswers204() = runTest {
        val client = client { request ->
            assertEquals(HttpMethod.Delete, request.method)
            assertEquals("/195539477/dock/doors/1069480290", request.url.encodedPath)
            respond("", HttpStatusCode.NoContent)
        }
        client.forAccount("195539477").doors.delete(1069480290)
        client.close()
    }

    @Test
    fun getMaps404ToNotFound() = runTest {
        val client = client { respond("""{"error":"Not found"}""", HttpStatusCode.NotFound, jsonHeaders()) }
        assertFailsWith<BasecampException.NotFound> {
            client.forAccount("195539477").doors.get(1)
        }
        client.close()
    }
}
