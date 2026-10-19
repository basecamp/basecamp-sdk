package com.basecamp.sdk

import com.basecamp.sdk.generated.pings
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
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Drives the Pings service through the shared `spec/fixtures/pings` bodies,
 * the same bytes the other SDKs read. JVM-only because `commonTest` has no
 * filesystem; `PingsServiceTest` covers the wire behavior.
 */
class PingsFixtureTest {

    private fun repoRoot(): File {
        var dir: File? = File(".").absoluteFile
        while (dir != null && !File(dir, "openapi.json").isFile) dir = dir.parentFile
        return requireNotNull(dir) { "could not locate repo root from ${File(".").absolutePath}" }
    }

    private fun fixture(name: String): String = File(repoRoot(), "spec/fixtures/pings/$name").readText()

    private fun client(body: String): BasecampClient = testBasecampClient {
        accessToken("test-token")
        engine = MockEngine {
            respond(
                content = body,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
    }

    @Test
    fun listReadsTheSharedFixture() = runTest {
        val client = client(fixture("list.json"))
        val pings = client.forAccount("195539477").pings.list()
        assertEquals(2, pings.size)
        val group = pings[0]
        assertEquals("Launch crew", group.title)
        assertTrue(group.named)
        assertEquals(2, group.people.size)
        assertEquals(
            "https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600/lines.json",
            group.links.lines,
        )
        assertEquals("https://3.basecamp.com/195539477/buckets/2085958600/title/edit", group.editTitleUrl)
        assertFalse(pings[1].named)
        assertNull(pings[1].editTitleUrl, "a direct ping carries no edit_title_url")
        client.close()
    }

    @Test
    fun getReadsTheSharedFixture() = runTest {
        val client = client(fixture("get.json"))
        val ping = client.forAccount("195539477").pings.get(2085958600)
        assertEquals(
            "https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600.json",
            ping.roomUrl,
        )
        client.close()
    }
}
