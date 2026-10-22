package com.basecamp.sdk

import com.basecamp.sdk.generated.subtasks
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
import kotlin.test.assertNotNull
import kotlin.test.assertNull

/**
 * Drives `SubtasksService.get` through the shared `spec/fixtures/subtasks`
 * bodies, the same bytes the other SDKs read. JVM-only because `commonTest`
 * has no filesystem.
 */
class SubtaskCompletionFixtureTest {

    private fun repoRoot(): File {
        var dir: File? = File(".").absoluteFile
        while (dir != null && !File(dir, "openapi.json").isFile) dir = dir.parentFile
        return requireNotNull(dir) { "could not locate repo root from ${File(".").absolutePath}" }
    }

    private fun fixture(name: String): String = File(repoRoot(), "spec/fixtures/subtasks/$name").readText()

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
    fun completedSubtaskSaysWhoCompletedItAndWhen() = runTest {
        val client = client(fixture("get-completed.json"))
        val subtask = client.forAccount("195539477").subtasks.get(1069479879)
        assertEquals(true, subtask.completed)
        val completion = assertNotNull(subtask.completion)
        assertEquals("2026-07-03T15:42:10.000Z", completion.createdAt)
        assertEquals(1049715938L, completion.creator.id)
        assertEquals("Annie Bryan", completion.creator.name)
        client.close()
    }

    @Test
    fun incompleteSubtaskHasNoCompletion() = runTest {
        val client = client(fixture("get.json"))
        val subtask = client.forAccount("195539477").subtasks.get(1069479879)
        assertNull(subtask.completion)
        client.close()
    }
}
