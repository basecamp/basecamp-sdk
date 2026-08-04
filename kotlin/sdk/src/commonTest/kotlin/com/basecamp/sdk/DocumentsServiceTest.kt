package com.basecamp.sdk

import com.basecamp.sdk.generated.documents
import com.basecamp.sdk.generated.services.ReplaceDocumentBody
import com.basecamp.sdk.generated.services.UpdateDocumentBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertTrue

/**
 * The merge-safe `update` / read-modify-write `edit` composites and the raw
 * `replace` they are built on.
 *
 * `PUT /documents/{id}` is a full replace: BC3 rebuilds the Document from only
 * the permitted params, so a sparse PUT that omits `content` erases it and one
 * that omits `title` leaves the document reading back as "Untitled". Neither
 * omission is a 422 — both are a 200 that quietly clears. That is why the
 * composites always send BOTH writable fields, empties included: on this
 * endpoint `""` is how a clear is expressed, and omission is indistinguishable
 * from an accident.
 */
class DocumentsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    // -- Merge-safe update / edit / replace --

    private fun fullDocumentJson(id: Long = 42) = """{
        "id": $id, "status": "active", "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
        "title": "Kickoff notes", "inherits_status": true, "type": "Document",
        "url": "https://3.basecampapi.com/12345/buckets/1/documents/$id.json",
        "app_url": "https://3.basecamp.com/12345/buckets/1/documents/$id",
        "parent": {"id": 2, "title": "Docs & Files", "type": "Vault", "url": "https://3.basecampapi.com/12345/buckets/1/vaults/2.json", "app_url": "https://3.basecamp.com/12345/buckets/1/vaults/2"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "creator": {"id": 1, "name": "Test", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
        "content_attachments": [],
        "content": "<p>From the kickoff</p>",
        "position": 1
    }"""

    private class WriteCapture {
        val methods = mutableListOf<String>()
        var putBody: kotlinx.serialization.json.JsonObject? = null
    }

    private fun captureClient(capture: WriteCapture): BasecampClient = mockClient { request ->
        capture.methods.add(request.method.value)
        if (request.method == HttpMethod.Put) {
            capture.putBody = json.parseToJsonElement(
                (request.body as io.ktor.http.content.TextContent).text
            ).jsonObject
        }
        respond(
            content = fullDocumentJson(),
            status = HttpStatusCode.OK,
            headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
        )
    }

    // kotlinx.serialization is Kotlin's answer to the hand-written type guards
    // the dynamic SDKs carry, and it does refuse a structurally wrong-typed
    // field before the composite can write it back. But it reports that as a
    // raw SerializationException, which is not the shape SPEC 6 defines for a
    // malformed 2xx body: a caller catching BasecampException would miss it
    // entirely. The composite normalizes it, so a malformed response looks the
    // same in every SDK.
    //
    // An ARRAY here, but a bare scalar would be refused too since #598 dropped
    // the client-wide `isLenient`, which used to render a JSON number or
    // boolean as a String instead of rejecting it. See `DecoderStrictnessTest`.
    @Test
    fun updateNormalizesADecodeFailure() = runTest {
        val capture = WriteCapture()
        val client = mockClient { request ->
            capture.methods.add(request.method.value)
            respond(
                content = fullDocumentJson().replace(
                    "\"title\": \"Kickoff notes\"", "\"title\": [\"nope\"]"
                ),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").documents
                .update(42, UpdateDocumentBody(content = "<p>New body.</p>"))
        }

        // Statusless and non-retryable: the transport succeeded, and
        // re-requesting cannot repair a malformed body.
        assertEquals(null, error.httpStatus)
        assertEquals(false, error.retryable)
        assertTrue(error.hint != null, "expected a hint naming the escape hatch")
        // The ordering is what matters: no PUT. A guard that fires after the
        // PUT has already lost the field.
        assertEquals(listOf("GET"), capture.methods)

        client.close()
    }

    // BC3 can never render a blank title (Document#title is
    // super.presence || "Untitled"), so "" on a 2xx read is malformed. The
    // model's non-null String already refuses absent/null; "" decodes fine and
    // needs the hand-written check. The ordering is what matters: no PUT.
    @Test
    fun updateRefusesABlankTitle() = runTest {
        val capture = WriteCapture()
        val client = mockClient { request ->
            capture.methods.add(request.method.value)
            respond(
                content = fullDocumentJson().replace("\"title\": \"Kickoff notes\"", "\"title\": \"   \""),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").documents
                .update(42, UpdateDocumentBody(content = "<p>New body.</p>"))
        }

        assertEquals(null, error.httpStatus)
        assertEquals(false, error.retryable)
        assertEquals(listOf("GET"), capture.methods)

        client.close()
    }

    @Test
    fun updateMergesUnsetFields() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val document = client.forAccount("12345").documents
            .update(42, UpdateDocumentBody(title = "Kickoff notes, revised"))

        assertEquals(42L, document.id)
        assertEquals(listOf("GET", "PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals("Kickoff notes, revised", body["title"]?.jsonPrimitive?.content)
        // content was never named, so the GET's value is written straight back
        // rather than left to the server's clear-by-default.
        assertEquals("<p>From the kickoff</p>", body["content"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun updateExplicitEmptyStringClears() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").documents.update(42, UpdateDocumentBody(content = ""))

        val body = capture.putBody!!
        // An explicitly-passed "" is a set, not an unset: present and empty.
        assertTrue("content" in body, "an explicit clear must be sent, not omitted")
        assertEquals("", body["content"]?.jsonPrimitive?.content)
        assertEquals("Kickoff notes", body["title"]?.jsonPrimitive?.content)

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
                content = fullDocumentJson(),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            this.hooks = hooks
        }

        client.forAccount("12345").documents.update(42, UpdateDocumentBody(title = "observed"))

        // The composite is built from the public get/replace, so hooks see the
        // two wire operations, not a synthetic composite.
        assertEquals(listOf("GetDocument", "ReplaceDocument"), operations)

        client.close()
    }

    @Test
    fun editPutsFullStateBack() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val document = client.forAccount("12345").documents.edit(42) {
            assertEquals("Kickoff notes", title)
            assertEquals("<p>From the kickoff</p>", content)
            title = "🚨 $title"
        }

        assertEquals(42L, document.id)
        assertEquals(listOf("GET", "PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals("🚨 Kickoff notes", body["title"]?.jsonPrimitive?.content)
        assertEquals("<p>From the kickoff</p>", body["content"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun editClearsContentPresentAndEmpty() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").documents.edit(42) {
            content = ""
        }

        val body = capture.putBody!!
        // Clearing on a full-replace endpoint is an explicit "": never JSON
        // null (SPEC §18 body compaction), and never by omission, which would
        // leave the clear to the server and read as an accident.
        assertTrue("content" in body, "a cleared content must be sent present-and-empty")
        assertEquals("", body["content"]?.jsonPrimitive?.content)
        assertEquals("Kickoff notes", body["title"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun editBlockErrorAbortsWithoutPut() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        try {
            client.forAccount("12345").documents.edit(42) {
                title = "never written"
                error("abort")
            }
            kotlin.test.fail("expected the block error to propagate")
        } catch (e: IllegalStateException) {
            assertEquals("abort", e.message)
        }

        assertEquals(listOf("GET"), capture.methods)

        client.close()
    }

    @Test
    fun replaceSendsSparseVerbatimWithNoGet() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val document = client.forAccount("12345").documents
            .replace(42, ReplaceDocumentBody(title = "the whole new document"))

        assertEquals(42L, document.id)
        assertEquals(listOf("PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals("the whole new document", body["title"]?.jsonPrimitive?.content)
        // The raw path is destructive by design: what the caller left out stays
        // out, and the server clears it.
        assertTrue("content" !in body, "content must be omitted from a sparse replace")

        client.close()
    }
}
