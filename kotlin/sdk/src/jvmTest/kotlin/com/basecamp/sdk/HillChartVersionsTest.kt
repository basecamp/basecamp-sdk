package com.basecamp.sdk

import com.basecamp.sdk.generated.hillCharts
import com.basecamp.sdk.generated.services.CreateHillChartVersionBody
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
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.put
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

/**
 * A hill chart's progress updates (BC3 #13620, and posting one, BC3 #14214), driven through the shared
 * `spec/fixtures/hill_charts` bodies: bc3's documented examples, plus one dot
 * whose todolist is gone. JVM-only because `commonTest` has no filesystem.
 */
class HillChartVersionsTest {

    private fun repoRoot(): File {
        var dir: File? = File(".").absoluteFile
        while (dir != null && !File(dir, "openapi.json").isFile) dir = dir.parentFile
        return requireNotNull(dir) { "could not locate repo root from ${File(".").absolutePath}" }
    }

    private fun fixture(name: String): String = File(repoRoot(), "spec/fixtures/hill_charts/$name").readText()

    private fun jsonHeaders(vararg extra: Pair<String, String>) =
        headersOf(*(listOf(HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString())) +
            extra.map { it.first to listOf(it.second) }).toTypedArray())

    private fun client(handler: MockRequestHandler): BasecampClient = testBasecampClient {
        accessToken("test-token")
        engine = MockEngine(handler)
    }

    @Test
    fun getCarriesVersionsUrl() = runTest {
        val client = client { respond(fixture("get.json"), HttpStatusCode.OK, jsonHeaders()) }
        val chart = client.forAccount("195539477").hillCharts.get(1069479829)
        assertEquals(
            "https://3.basecampapi.com/195539477/buckets/2085958505/todosets/1069479829/hills/versions.json",
            chart.versionsUrl,
        )
        client.close()
    }

    @Test
    fun listVersionsReadsTheSharedFixture() = runTest {
        val client = client { request ->
            assertEquals(HttpMethod.Get, request.method)
            assertEquals("/195539477/todosets/1069479829/hills/versions.json", request.url.encodedPath)
            respond(fixture("versions.json"), HttpStatusCode.OK, jsonHeaders("X-Total-Count" to "1"))
        }
        val versions = client.forAccount("195539477").hillCharts.listVersions(1069479829)
        assertEquals(1, versions.size)
        assertEquals(1L, versions.meta.totalCount)
        val version = versions[0]
        assertEquals(1069480194L, version.id)
        assertEquals("Hill::Version", version.type)
        assertEquals(emptyList(), version.descriptionAttachments)
        assertEquals(1069479863L, version.dots[0].id)
        assertEquals(40, version.dots[0].position)
        client.close()
    }

    @Test
    fun getVersionReadsADotWhoseTodolistIsGone() = runTest {
        val client = client { request ->
            assertEquals("/195539477/todosets/1069479829/hills/versions/1069480194", request.url.encodedPath)
            respond(fixture("version_todolist_gone.json"), HttpStatusCode.OK, jsonHeaders())
        }
        val version = client.forAccount("195539477").hillCharts.getVersion(1069479829, 1069480194)
        val gone = version.dots[1]
        assertNull(gone.id)
        assertNull(gone.url)
        assertEquals("Old research list", gone.label)
        assertEquals("Victor Cooper", version.creator.name)
        client.close()
    }

    @Test
    fun getVersionMaps404ToNotFound() = runTest {
        val client = client { respond("""{"error":"Not found"}""", HttpStatusCode.NotFound, jsonHeaders()) }
        val e = assertFailsWith<BasecampException.NotFound> {
            client.forAccount("195539477").hillCharts.getVersion(1, 999)
        }
        assertEquals(404, e.httpStatus)
        client.close()
    }

    @Test
    fun listVersionsMaps404ToNotFound() = runTest {
        val client = client { respond("""{"error":"Not found"}""", HttpStatusCode.NotFound, jsonHeaders()) }
        assertFailsWith<BasecampException.NotFound> {
            client.forAccount("195539477").hillCharts.listVersions(999)
        }
        client.close()
    }

    @Test
    fun createVersionPostsTheDocumentedRequest() = runTest {
        var captured: String? = null
        val client = client { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertEquals("/195539477/todosets/1069479829/hills/versions.json", request.url.encodedPath)
            captured = request.body.toByteArray().decodeToString()
            respond(fixture("create-version.json"), HttpStatusCode.Created, jsonHeaders())
        }
        val version = client.forAccount("195539477").hillCharts.createVersion(
            1069479829,
            CreateHillChartVersionBody(
                description = "<div>Research is done. Building starts Monday.</div>",
                dots = listOf(buildJsonObject {
                    put("id", 1069479863L)
                    put("position", 55)
                }),
                subscriptions = listOf(1049715913L),
            ),
        )
        assertEquals(
            Json.parseToJsonElement(fixture("create-version-request.json")).jsonObject,
            Json.parseToJsonElement(captured!!).jsonObject,
        )
        assertEquals(1069480194L, version.id)
        assertEquals(55, version.dots[0].position)
        client.close()
    }

    @Test
    fun createVersionWithNothingPostsAnEmptyObject() = runTest {
        var captured: String? = null
        val client = client { request ->
            captured = request.body.toByteArray().decodeToString()
            respond(fixture("create-version.json"), HttpStatusCode.Created, jsonHeaders())
        }
        client.forAccount("195539477").hillCharts.createVersion(1069479829, CreateHillChartVersionBody())
        assertEquals(JsonObject(emptyMap()), Json.parseToJsonElement(captured!!).jsonObject)
        client.close()
    }

    @Test
    fun createVersionMaps422ToValidation() = runTest {
        val client = client {
            respond("""{"error":"Not tracked on the hill chart: 1069479511"}""", HttpStatusCode.UnprocessableEntity, jsonHeaders())
        }
        val e = assertFailsWith<BasecampException.Validation> {
            client.forAccount("195539477").hillCharts.createVersion(1069479829, CreateHillChartVersionBody())
        }
        assertEquals(422, e.httpStatus)
        assertEquals("Not tracked on the hill chart: 1069479511", e.message)
        client.close()
    }
}
