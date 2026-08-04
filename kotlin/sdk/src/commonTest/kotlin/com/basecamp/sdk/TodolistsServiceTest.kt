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
import kotlin.test.assertFailsWith
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlin.test.fail

/**
 * The merge-safe `update` / read-modify-write `edit` composites, the raw
 * `replace` they are built on, and the decode of the one shape this route
 * answers with.
 *
 * `GET /todolists/{id}` is polymorphic in the resource it addresses but not in
 * the JSON it returns: BC3 has no group model, and
 * `todolists/groups/{index,show}.json.jbuilder` render
 * `todolists/_todolist.json.jbuilder`, so a group is a `Todolist` — same flat
 * body, same `"type": "Todolist"`, same `description`. The structural
 * discriminator is `group_position_url` in place of `groups_url`, never the
 * type string, and these composites never look at either.
 */
class TodolistsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private val api = "https://3.basecampapi.com/12345/buckets/1"

    /** A to-do list's parent is the project's Todoset. */
    private val todosetParent = """{"id": 3, "title": "To-dos", "type": "Todoset",
        "url": "$api/todosets/3.json", "app_url": "https://3.basecamp.com/12345/buckets/1/todosets/3"}"""

    /** A GROUP's parent is a Todolist. That, not `type`, is what differs. */
    private val todolistParent = """{"id": 7, "title": "Roadmap", "type": "Todolist",
        "url": "$api/todolists/7.json", "app_url": "https://3.basecamp.com/12345/buckets/1/todolists/7"}"""

    /**
     * A complete flat to-do list body, assembled member by member so a test can
     * OMIT a key rather than string-surgery it out.
     *
     * @param name raw JSON for the `name` member, or null to omit the key
     * @param description raw JSON for `description`, or null to omit the key
     * @param color raw JSON for `color` — `null` for an uncolored list or group,
     *   which is the ordinary case for a group. The key itself is always emitted:
     *   it is required-and-nullable, so this takes raw JSON rather than a String?
     *   that would omit it.
     * @param trailing the variant-specific members (`groups_url` for a list,
     *   `group_position_url` for a group)
     */
    private fun todolistBody(
        id: Long = 42,
        name: String? = "\"Launch list\"",
        description: String? = "\"<p>Things to do before launch</p>\"",
        descriptionAttachments: String = "[]",
        parent: String = todosetParent,
        color: String = "\"blue\"",
        trailing: List<String> = listOf("\"groups_url\": \"$api/todolists/$id/groups.json\""),
    ): String {
        val members = mutableListOf(
            "\"id\": $id",
            "\"status\": \"active\"",
            "\"visible_to_clients\": false",
            "\"created_at\": \"2026-01-01T00:00:00Z\"",
            "\"updated_at\": \"2026-01-01T00:00:00Z\"",
            "\"title\": \"Launch list\"",
            "\"inherits_status\": true",
            "\"type\": \"Todolist\"",
            "\"url\": \"$api/todolists/$id.json\"",
            "\"app_url\": \"https://3.basecamp.com/12345/buckets/1/todolists/$id\"",
            "\"bubble_up_url\": \"$api/recordings/$id/bubble_up.json\"",
            "\"parent\": $parent",
            "\"bucket\": {\"id\": 1, \"name\": \"Project\", \"type\": \"Project\"}",
            "\"creator\": {\"id\": 1, \"name\": \"Test User\"}",
            "\"description_attachments\": $descriptionAttachments",
            "\"completed\": false",
            "\"completed_ratio\": \"0/5\"",
            "\"position\": 1",
            "\"todos_url\": \"$api/todolists/$id/todos.json\"",
            // Both keys are @required in the published contract: the jbuilder
            // emits color in both branches of its todolist_group? conditional and
            // comments_app_url from a route helper. Neither is ever absent.
            "\"color\": $color",
            "\"comments_app_url\": \"https://3.basecamp.com/12345/buckets/1/recordings/$id/comments\"",
        )
        name?.let { members += "\"name\": $it" }
        description?.let { members += "\"description\": $it" }
        members += trailing
        return members.joinToString(",\n", "{\n", "\n}")
    }

    private val todolistJson = todolistBody()

    /**
     * The same URI addresses a to-do list group. BC3 renders it through
     * `todolists/_todolist.json.jbuilder`, so it carries `description` and
     * `description_attachments` like any list; only the parent and the
     * `group_position_url`/`groups_url` pair differ.
     */
    private val todolistGroupJson = todolistBody(
        id = 42,
        name = "\"Hardware\"",
        description = "\"<p>Ship the hardware</p>\"",
        parent = todolistParent,
        color = "null",
        trailing = listOf(
            "\"group_position_url\": \"$api/todolists/groups/42/position.json\"",
        ),
    )

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

    // -- Decode: one flat shape for both variants (#544) --

    /**
     * The route is polymorphic, the payload is not. `get` returns a decoded
     * `Todolist` for a GROUP too: same required `description` and
     * `description_attachments`, `group_position_url` where a list carries
     * `groups_url`.
     *
     * Before #544 the spec modelled this response as a `oneOf`, so Kotlin's
     * generator handed back an untyped `JsonElement` and no decoder ever ran —
     * a group's description was reachable only by hand-walking the tree. This
     * is the test that would have caught that.
     */
    @Test
    fun getDecodesAGroupIntoTheTodolistShape() = runTest {
        val attachment = """[{
            "id": 9, "sgid": "sgid-9", "filename": "spec.pdf",
            "content_type": "application/pdf", "byte_size": 1024,
            "download_url": "$api/attachments/9/spec.pdf",
            "previewable": true,
            "preview_url": "$api/attachments/9/preview",
            "thumbnail_url": "$api/attachments/9/thumb"
        }]"""
        val body = todolistBody(
            id = 7,
            name = "\"Phase 1\"",
            description = "\"<div>Phase one hardware work</div>\"",
            descriptionAttachments = attachment,
            parent = todolistParent,
            color = "null",
            trailing = listOf(
                "\"group_position_url\": \"$api/todolists/groups/7/position.json\"",
            ),
        )
        val engine = MockEngine { _ ->
            respond(
                content = body,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }

        val group = client.forAccount("12345").todolists.get(id = 7)

        assertEquals("Phase 1", group.name)
        // A group DOES carry a description. The pre-#544 group projection
        // modelled none, so this is the field the flat shape recovers.
        assertEquals("<div>Phase one hardware work</div>", group.description)
        assertEquals(1, group.descriptionAttachments.size)
        assertEquals("spec.pdf", group.descriptionAttachments[0].filename)
        // Structural discrimination: group_position_url XOR groups_url. The
        // `type` string is "Todolist" for both, which is why nothing branches
        // on it.
        assertEquals("Todolist", group.type)
        assertEquals("$api/todolists/groups/7/position.json", group.groupPositionUrl)
        assertNull(group.groupsUrl, "a group's parent is a Todolist, so it has no groups_url")
        assertNull(group.color)
        assertEquals(
            "https://3.basecamp.com/12345/buckets/1/recordings/7/comments",
            group.commentsAppUrl,
        )

        client.close()
    }

    /** The list variant is the mirror image: groups_url present, no group_position_url. */
    @Test
    fun getDecodesAListIntoTheTodolistShape() = runTest {
        val engine = MockEngine { _ ->
            respond(
                content = todolistBody(trailing = listOf(
                    "\"groups_url\": \"$api/todolists/42/groups.json\"",
                )),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }

        val list = client.forAccount("12345").todolists.get(id = 42)

        assertEquals("Launch list", list.name)
        assertEquals("<p>Things to do before launch</p>", list.description)
        assertEquals("Todolist", list.type)
        assertEquals("$api/todolists/42/groups.json", list.groupsUrl)
        assertNull(list.groupPositionUrl, "a list's parent is a Todoset, so it has no group_position_url")
        assertEquals("blue", list.color)

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
     * Since #544 `get` returns a decoded `Todolist`, so kotlinx.serialization
     * is the guard the dynamic SDKs write by hand: it refuses a structurally
     * wrong-typed field before this composite can write it back. What the
     * composite still owns is the SPEC §6 shape — a raw `SerializationException`
     * is not it, and a caller catching `BasecampException` would miss it.
     *
     * Arrays and objects here; bare scalars in the sibling below. Both are
     * refused now. Structural mismatches always were, but a bare JSON scalar
     * used to be coerced into a String by the client-wide `isLenient`, which
     * #598 removed — see `updateRefusesABareScalarDescriptionBeforeWriting` for
     * that half and `DecoderStrictnessTest` for which flag did what.
     */
    @Test
    fun updateRefusesAMalformedDescriptionBeforeWriting() = runTest {
        for (malformed in listOf("[]", """{"a":1}""")) {
            val capture = WriteCapture()
            val client = captureClient(capture, getBody = todolistBody(description = malformed))

            val error = assertFailsWith<BasecampException.Api> {
                client.forAccount("12345").todolists
                    .update(42, UpdateTodolistBody(name = "Renamed list"))
            }

            // Statusless and non-retryable: the transport succeeded, and
            // re-requesting cannot repair a malformed body.
            assertEquals(null, error.httpStatus)
            assertEquals(false, error.retryable)
            assertTrue(error.hint != null, "expected a hint naming the escape hatch")
            // The ordering is what matters: no PUT. A guard that fires after
            // the PUT has already lost the field.
            assertEquals(
                listOf("GET"), capture.methods,
                "the PUT must never be issued for a malformed description ($malformed)",
            )
            client.close()
        }
    }

    /**
     * The bare-scalar half of the case above — now refused, not coerced (#598).
     *
     * This test used to assert the opposite, and said so: it pinned the coerced
     * PUT "so closing #576 flips it visibly." Dropping the client-wide
     * `isLenient` flipped it. A JSON number or boolean where a String belongs no
     * longer renders as text; it is refused before the PUT, exactly like the
     * array and object cases above.
     *
     * Booleans as well as numbers, because `isLenient` accepted both, and each
     * produced a differently-shaped fabrication ("42" vs "false").
     */
    @Test
    fun updateRefusesABareScalarDescriptionBeforeWriting() = runTest {
        for (malformed in listOf("42", "false")) {
            val capture = WriteCapture()
            val client = captureClient(capture, getBody = todolistBody(description = malformed))

            val error = assertFailsWith<BasecampException.Api> {
                client.forAccount("12345").todolists
                    .update(42, UpdateTodolistBody(name = "Renamed list"))
            }

            assertEquals(null, error.httpStatus)
            assertEquals(false, error.retryable)
            assertTrue(error.hint != null, "expected a hint naming the escape hatch")
            assertEquals(
                listOf("GET"), capture.methods,
                "the PUT must never be issued for a bare-scalar description ($malformed)",
            )
            client.close()
        }
    }

    /** The same guard protects the edit closure, which also resends the field. */
    @Test
    fun editRefusesAMalformedNameBeforeWriting() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture, getBody = todolistBody(name = """["nope"]"""))

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").todolists.edit(42) { description = "<p>New</p>" }
        }

        assertEquals(null, error.httpStatus)
        assertEquals(false, error.retryable)
        assertEquals(listOf("GET"), capture.methods, "no PUT may be issued")

        client.close()
    }

    /**
     * `name` is required and presence-validated, so absent, null and `""` off
     * the wire are all malformed — but two different mechanisms now say so.
     * Absent and null are refused by the decoder (the field is required and
     * non-nullable on the model) and normalized into the SPEC §6 shape; `""`
     * decodes fine and needs the composite's own check. All three must land on
     * the same outcome: `BasecampException.Api`, and no PUT.
     *
     * Classification is by ORIGIN: this name came off the wire, so it is
     * `BasecampException.Api`. The caller supplying an empty name stays
     * `BasecampException.Usage`, asserted separately.
     */
    @Test
    fun updateRefusesAnAbsentNullOrEmptyNameFromTheResponse() = runTest {
        val cases = listOf(
            null to "does not decode as a todolist",
            "null" to "does not decode as a todolist",
            "\"\"" to "empty \"name\"",
        )

        for ((nameJson, expectedFragment) in cases) {
            val capture = WriteCapture()
            val client = captureClient(capture, getBody = todolistBody(name = nameJson))

            val error = assertFailsWith<BasecampException.Api> {
                client.forAccount("12345").todolists
                    .update(42, UpdateTodolistBody(description = "<p>New</p>"))
            }

            assertTrue(
                error.message!!.contains(expectedFragment),
                "expected a message mentioning \"$expectedFragment\" for name=$nameJson, " +
                    "got: ${error.message}",
            )
            assertTrue(error.hint != null, "expected a hint naming the escape hatch")
            assertEquals(
                listOf("GET"), capture.methods,
                "a malformed name ($nameJson) must never reach the PUT",
            )
            client.close()
        }
    }

    /** The mirror case: same value, caller origin, so Usage not Api. */
    @Test
    fun callerSuppliedEmptyNameIsAUsageError() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        try {
            client.forAccount("12345").todolists.edit(42) { name = "" }
            fail("expected a caller-emptied name to raise a usage error")
        } catch (e: BasecampException.Usage) {
            assertTrue(e.message!!.contains("name"), e.message!!)
        }

        assertEquals(listOf("GET"), capture.methods)
        client.close()
    }

    /**
     * An absent or explicitly-null description is now malformed, not "empty".
     *
     * This inverts the pre-#544 behavior, and the spec change is why: BC3's
     * `format_api_content` renders a blank rich text as `""` and never as JSON
     * null, so `description` is modelled required and non-nullable. A body
     * without one did not come from BC3, and carrying the implied `""` into
     * this full-replace PUT would erase a description the caller never
     * mentioned. The decoder refuses it; the composite normalizes the refusal.
     */
    @Test
    fun updateRefusesAnAbsentOrNullDescriptionFromTheResponse() = runTest {
        for (descriptionJson in listOf(null, "null")) {
            val capture = WriteCapture()
            val client = captureClient(capture, getBody = todolistBody(description = descriptionJson))

            val error = assertFailsWith<BasecampException.Api> {
                client.forAccount("12345").todolists
                    .update(42, UpdateTodolistBody(name = "Renamed list"))
            }

            assertTrue(
                error.message!!.contains("does not decode as a todolist"),
                "expected the decode-failure shape for description=$descriptionJson, " +
                    "got: ${error.message}",
            )
            assertEquals(
                listOf("GET"), capture.methods,
                "a body with no description ($descriptionJson) must never reach the PUT",
            )
            client.close()
        }
    }

    /** An empty description IS a value: BC3 renders a blank rich text as "". */
    @Test
    fun updatePreservesAnEmptyDescription() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture, getBody = todolistBody(description = "\"\""))

        client.forAccount("12345").todolists.update(42, UpdateTodolistBody(name = "Renamed list"))

        assertEquals(listOf("GET", "PUT"), capture.methods)
        assertEquals("", capture.putBody!!["description"]?.jsonPrimitive?.content)

        client.close()
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
        assertEquals(42L, result.id)

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

    /**
     * The composite is variant-agnostic: fed a GROUP — `group_position_url`
     * rather than `groups_url`, parent a Todolist — it preserves the
     * description exactly as it does for a list, with no type sniffing.
     */
    @Test
    fun updateIsVariantAgnosticForGroups() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture, getBody = todolistGroupJson)

        client.forAccount("12345").todolists.update(42, UpdateTodolistBody(name = "Renamed group"))

        assertEquals(listOf("GET", "PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals(setOf("name", "description"), body.keys)
        assertEquals("Renamed group", body["name"]?.jsonPrimitive?.content)
        assertEquals("<p>Ship the hardware</p>", body["description"]?.jsonPrimitive?.content)

        client.close()
    }

    /**
     * The wire body is FLAT — there is no `{"todolist": ...}` envelope to
     * unwrap.
     *
     * The envelope was a Smithy modelling convention (AGENTS.md, "Smithy Spec
     * vs Actual API Responses") that the pre-#544 `JsonElement` reader
     * tolerated with a lookup. #544 removed the `oneOf` and with it the
     * tolerance: an enveloped body carries none of the required members at the
     * root, so it is refused before the PUT rather than half-read.
     */
    @Test
    fun updateRefusesAnEnvelopedBody() = runTest {
        for (key in listOf("todolist", "group")) {
            val capture = WriteCapture()
            val client = captureClient(capture, getBody = """{"$key": $todolistJson}""")

            val error = assertFailsWith<BasecampException.Api> {
                client.forAccount("12345").todolists
                    .update(42, UpdateTodolistBody(name = "Renamed list"))
            }

            assertTrue(
                error.message!!.contains("does not decode as a todolist"),
                "expected the decode-failure shape for a {\"$key\": ...} envelope, " +
                    "got: ${error.message}",
            )
            assertEquals(listOf("GET"), capture.methods, "no PUT may be issued")
            client.close()
        }
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
        assertEquals(42L, result.id)

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
        assertEquals(42L, result.id)

        client.close()
    }
}
