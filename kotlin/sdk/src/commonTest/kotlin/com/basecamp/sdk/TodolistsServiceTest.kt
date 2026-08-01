package com.basecamp.sdk

import com.basecamp.sdk.generated.services.UpdateTodolistBody
import com.basecamp.sdk.generated.services.UpdateTodolistOrGroupBody
import com.basecamp.sdk.generated.todolists
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue
import kotlin.test.fail

class TodolistsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private val todolistJson = """{
        "id": 42,
        "name": "Launch list",
        "title": "Launch list",
        "description": "<p>Things to do before launch</p>",
        "description_attachments": [],
        "completed": false,
        "completed_ratio": "0/5",
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z"
    }"""

    /**
     * The same URI addresses a todolist group; BC3 renders both through
     * `todolists/_todolist.json.jbuilder`, so the projection differs only in
     * the parent and the group/list URLs — never in `{name, description}`.
     */
    private val todolistGroupJson = """{
        "id": 42,
        "name": "Hardware",
        "title": "Hardware",
        "type": "Todolist",
        "description": "<p>Ship the hardware</p>",
        "description_attachments": [],
        "completed": false,
        "completed_ratio": "0/3",
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z",
        "parent": {"id": 7, "title": "Roadmap", "type": "Todolist"},
        "group_position_url": "https://3.basecampapi.com/999/buckets/1/todolists/groups/42/position.json"
    }"""

    @Test
    fun getEmitsTodolistIdAsResourceId() = runTest {
        var capturedInfo: OperationInfo? = null
        val hooks = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                if (info.operation == "GetTodolistOrGroup") capturedInfo = info
            }
        }
        val engine = MockEngine { _ ->
            respond(
                content = todolistJson,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            this.hooks = hooks
        }

        client.forAccount("12345").todolists.get(id = 42)

        assertNotNull(capturedInfo)
        assertEquals(42L, capturedInfo!!.resourceId)

        client.close()
    }

    @Test
    fun updateEmitsTodolistIdAsResourceId() = runTest {
        var capturedInfo: OperationInfo? = null
        val hooks = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                if (info.operation == "UpdateTodolistOrGroup") capturedInfo = info
            }
        }
        val engine = MockEngine { _ ->
            respond(
                content = todolistJson,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            this.hooks = hooks
        }

        client.forAccount("12345").todolists.update(
            id = 42,
            body = UpdateTodolistBody(name = "Updated list"),
        )

        assertNotNull(capturedInfo)
        assertEquals(42L, capturedInfo!!.resourceId)

        client.close()
    }

    // -- Merge-safe update / edit / replace (SPEC.md §5, §18) --

    private class WriteCapture {
        val methods = mutableListOf<String>()
        var putBody: JsonObject? = null
    }

    private fun captureClient(capture: WriteCapture, getBody: String = todolistJson): BasecampClient {
        val engine = MockEngine { request ->
            capture.methods.add(request.method.value)
            if (request.method == HttpMethod.Put) {
                capture.putBody = json.parseToJsonElement(
                    (request.body as io.ktor.http.content.TextContent).text
                ).jsonObject
            }
            respond(
                content = getBody,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    /**
     * A malformed writable field must abort before the PUT, never be coerced
     * into it.
     *
     * `GetTodolistOrGroup` is modelled as a `oneOf`, so Kotlin's generator
     * hands these composites an untyped `JsonElement` with no decoder to
     * reject a wrong-typed field. Reading it leniently is therefore a wire
     * hazard unique to this service: a JSON number or boolean renders as text,
     * and an array or object is not a `JsonPrimitive` at all so it collapses
     * to `""`. Because this endpoint is full-replace, that value is then
     * written back over the real one — the composite erases the field it
     * exists to preserve, on a call that never mentioned it.
     */
    @Test
    fun updateRefusesAMalformedDescriptionBeforeWriting() = runTest {
        // Arrays, objects, numbers and booleans: everything a lenient reader
        // would coerce to text or silently flatten to "".
        val malformedDescriptions = listOf("[]", """{"a":1}""", "42", "true")

        for (malformed in malformedDescriptions) {
            val capture = WriteCapture()
            val body = todolistJson.replace(
                """"description": "<p>Things to do before launch</p>"""",
                """"description": $malformed""",
            )
            val client = captureClient(capture, getBody = body)

            try {
                client.forAccount("12345").todolists
                    .update(42, UpdateTodolistBody(name = "Renamed list"))
                fail("expected a malformed description ($malformed) to abort the update")
            } catch (e: BasecampException.Api) {
                assertTrue(
                    e.message!!.contains("'description' is not a JSON string"),
                    "error must name the offending field, got: ${e.message}",
                )
            }

            assertEquals(
                listOf("GET"), capture.methods,
                "the PUT must never be issued for a malformed description ($malformed)",
            )
            client.close()
        }
    }

    /** The same guard protects the edit closure, which also resends the field. */
    @Test
    fun editRefusesAMalformedNameBeforeWriting() = runTest {
        val capture = WriteCapture()
        val body = todolistJson.replace(""""name": "Launch list"""", """"name": 42""")
        val client = captureClient(capture, getBody = body)

        try {
            client.forAccount("12345").todolists.edit(42) { description = "<p>New</p>" }
            fail("expected a malformed name to abort the edit")
        } catch (e: BasecampException.Api) {
            assertTrue(e.message!!.contains("'name' is not a JSON string"), e.message!!)
        }

        assertEquals(listOf("GET"), capture.methods, "no PUT may be issued")
        client.close()
    }

    /** An absent or explicitly-null field is genuinely empty, not malformed. */
    @Test
    fun updateTreatsAbsentAndNullDescriptionAsEmpty() = runTest {
        for (body in listOf(
            """{"id": 42, "name": "Launch list"}""",
            """{"id": 42, "name": "Launch list", "description": null}""",
        )) {
            val capture = WriteCapture()
            val client = captureClient(capture, getBody = body)

            client.forAccount("12345").todolists
                .update(42, UpdateTodolistBody(name = "Renamed list"))

            assertEquals(listOf("GET", "PUT"), capture.methods)
            assertEquals("", capture.putBody!!["description"]?.jsonPrimitive?.content)
            client.close()
        }
    }

    @Test
    fun updateMergesUnsetDescription() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val result = client.forAccount("12345").todolists
            .update(42, UpdateTodolistBody(name = "Renamed list"))

        assertEquals(listOf("GET", "PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals(setOf("name", "description"), body.keys)
        assertEquals("Renamed list", body["name"]?.jsonPrimitive?.content)
        // Omitting description would erase it: BC3 rebuilds the recordable
        // from only the permitted params.
        assertEquals("<p>Things to do before launch</p>", body["description"]?.jsonPrimitive?.content)
        assertEquals(42L, result.jsonObject["id"]?.jsonPrimitive?.content?.toLong())

        client.close()
    }

    @Test
    fun updateSetsDescriptionAndCarriesName() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").todolists
            .update(42, UpdateTodolistBody(description = "<p>Rewritten</p>"))

        val body = capture.putBody!!
        assertEquals("<p>Rewritten</p>", body["description"]?.jsonPrimitive?.content)
        assertEquals("Launch list", body["name"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun updateExplicitEmptyDescriptionClears() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").todolists.update(42, UpdateTodolistBody(description = ""))

        val body = capture.putBody!!
        // Present-and-empty, never JSON null (SPEC.md §18 body compaction).
        assertEquals("", body["description"]?.jsonPrimitive?.content)
        assertTrue("description" in body, "a cleared description must be present and empty")
        assertEquals("Launch list", body["name"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun updateIsVariantAgnosticForGroups() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture, getBody = todolistGroupJson)

        client.forAccount("12345").todolists.update(42, UpdateTodolistBody(name = "Renamed group"))

        val body = capture.putBody!!
        assertEquals(setOf("name", "description"), body.keys)
        assertEquals("Renamed group", body["name"]?.jsonPrimitive?.content)
        assertEquals("<p>Ship the hardware</p>", body["description"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun updateReadsTheEnvelopedFormToo() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture, getBody = """{"todolist": $todolistJson}""")

        client.forAccount("12345").todolists.update(42, UpdateTodolistBody(name = "Renamed list"))

        val body = capture.putBody!!
        assertEquals("Renamed list", body["name"]?.jsonPrimitive?.content)
        assertEquals("<p>Things to do before launch</p>", body["description"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun updateHooksObserveGetThenReplace() = runTest {
        val operations = mutableListOf<String>()
        val hooks = object : BasecampHooks {
            override fun onOperationStart(info: OperationInfo) {
                operations.add(info.operation)
            }
        }
        val engine = MockEngine { _ ->
            respond(
                content = todolistJson,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            this.hooks = hooks
        }

        client.forAccount("12345").todolists.update(42, UpdateTodolistBody(name = "observed"))

        assertEquals(listOf("GetTodolistOrGroup", "UpdateTodolistOrGroup"), operations)

        client.close()
    }

    @Test
    fun updateWithEmptyNameThrowsUsageBeforeThePut() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        try {
            client.forAccount("12345").todolists.update(42, UpdateTodolistBody(name = ""))
            fail("expected an empty name to be rejected")
        } catch (e: BasecampException.Usage) {
            assertEquals("todolist name is required", e.message)
            assertEquals(BasecampException.CODE_USAGE, e.code)
        }

        assertEquals(listOf("GET"), capture.methods, "nothing may be written")

        client.close()
    }

    @Test
    fun editClearsDescriptionAndKeepsName() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val result = client.forAccount("12345").todolists.edit(42) {
            assertEquals("Launch list", name)
            assertEquals("<p>Things to do before launch</p>", description)
            description = ""
        }

        assertEquals(listOf("GET", "PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals(setOf("name", "description"), body.keys)
        assertEquals("", body["description"]?.jsonPrimitive?.content)
        assertEquals("Launch list", body["name"]?.jsonPrimitive?.content)
        assertEquals(42L, result.jsonObject["id"]?.jsonPrimitive?.content?.toLong())

        client.close()
    }

    @Test
    fun editRenamesAndPutsFullStateBack() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").todolists.edit(42) { name = "🚨 $name" }

        val body = capture.putBody!!
        assertEquals("🚨 Launch list", body["name"]?.jsonPrimitive?.content)
        assertEquals("<p>Things to do before launch</p>", body["description"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun editBlockErrorAbortsWithoutPut() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        try {
            client.forAccount("12345").todolists.edit(42) {
                name = "never written"
                error("abort")
            }
            fail("expected the block error to propagate")
        } catch (e: IllegalStateException) {
            assertEquals("abort", e.message)
        }

        assertEquals(listOf("GET"), capture.methods)

        client.close()
    }

    @Test
    fun editWithEmptyNameThrowsUsageBeforeThePut() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        try {
            client.forAccount("12345").todolists.edit(42) { name = "" }
            fail("expected an empty name to be rejected")
        } catch (e: BasecampException.Usage) {
            assertEquals("todolist name is required", e.message)
            assertEquals(BasecampException.CODE_USAGE, e.code)
        }

        assertEquals(listOf("GET"), capture.methods, "nothing may be written")

        client.close()
    }

    @Test
    fun replaceSendsSparseVerbatimWithNoGet() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val result = client.forAccount("12345").todolists
            .replace(42, UpdateTodolistOrGroupBody(name = "The whole new list"))

        assertEquals(listOf("PUT"), capture.methods, "the raw path never reads before it writes")
        val body = capture.putBody!!
        assertEquals(setOf("name"), body.keys)
        assertEquals("The whole new list", body["name"]?.jsonPrimitive?.content)
        // Omitted, not null: the server clears what the request leaves out.
        assertTrue("description" !in body, "description must be omitted from a sparse replace")
        assertEquals(42L, result.jsonObject["id"]?.jsonPrimitive?.content?.toLong())

        client.close()
    }
}
