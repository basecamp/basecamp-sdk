package com.basecamp.sdk

import com.basecamp.sdk.generated.recordings
import com.basecamp.sdk.generated.services.CopyRecordingBody
import com.basecamp.sdk.generated.services.MoveRecordingBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import io.ktor.http.content.TextContent
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotNull
import kotlin.test.assertNull

class RecordingsServiceTest {

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    // The generic Recording is a polymorphic projection: it carries only the
    // rich-text companion array matching its type, and a webhook-sourced
    // recording (rendered from the base partial) carries neither. Those arrays
    // are therefore OPTIONAL — modeled as a nullable `List<RichTextAttachment>?`
    // rather than the empty-list default used for other optional arrays — so an
    // absent array stays distinct from a present-but-empty one, matching Go,
    // Swift, TypeScript, Python, and Ruby. See SPEC.md §10 / the
    // rich-text-attachments-coverage api-gap entry.
    private fun recordingJson(id: Long, attachments: String) = """{
        "id": $id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z",
        "title": "We won Leto!",
        "inherits_status": true,
        "type": "Message",
        "url": "https://3.basecampapi.com/12345/buckets/1/messages/$id.json",
        "app_url": "https://3.basecamp.com/12345/buckets/1/messages/$id",
        "content": "<div>Big news</div>"$attachments,
        "parent": {"id": 100, "title": "Message Board", "type": "Message::Board", "url": "https://3.basecampapi.com/12345/buckets/1/message_boards/100.json", "app_url": "https://3.basecamp.com/12345/buckets/1/message_boards/100"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "creator": {"id": 1, "name": "Test User", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"}
    }"""

    @Test
    fun absentCompanionArrayDecodesToNull() = runTest {
        // A webhook-sourced recording omits the companion arrays entirely.
        val client = mockClient { _ ->
            respond(
                content = "[" + recordingJson(1, "") + "]",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val recording = client.forAccount("12345").recordings.list(type = "Message").single()
        assertNull(recording.contentAttachments, "absent content_attachments should decode to null")
        assertNull(recording.descriptionAttachments, "absent description_attachments should decode to null")
        client.close()
    }

    @Test
    fun presentEmptyCompanionArrayDecodesToEmptyList() = runTest {
        // A Message recording with no inline files: present, but empty. This must
        // stay distinct from absent — a non-null empty list, not null.
        val client = mockClient { _ ->
            respond(
                content = "[" + recordingJson(2, ""","content_attachments": []""") + "]",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val recording = client.forAccount("12345").recordings.list(type = "Message").single()
        val attachments = assertNotNull(recording.contentAttachments, "present-empty content_attachments must not be null")
        assertEquals(0, attachments.size)
        client.close()
    }

    @Test
    fun populatedCompanionArrayDecodesFaithfully() = runTest {
        val attachment = """{
            "id": 1069480040, "sgid": "BAh-img", "filename": "diagram.png",
            "content_type": "image/png", "byte_size": 204800,
            "download_url": "https://3.basecampapi.com/12345/buckets/1/blobs/img/download/diagram.png",
            "width": 1024.0, "height": 768, "previewable": true,
            "preview_url": "https://3.basecampapi.com/12345/buckets/1/blobs/img/previews/diagram.png",
            "thumbnail_url": "https://3.basecampapi.com/12345/buckets/1/blobs/img/thumbnails/diagram.png"
        }"""
        val client = mockClient { _ ->
            respond(
                content = "[" + recordingJson(3, ""","content_attachments": [$attachment]""") + "]",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val recording = client.forAccount("12345").recordings.list(type = "Message").single()
        val attachments = assertNotNull(recording.contentAttachments)
        assertEquals(1, attachments.size)
        // Float-spelled 1024.0 decodes to the integer 1024 via FlexibleIntSerializer.
        assertEquals(1024, attachments[0].width)
        assertEquals(768, attachments[0].height)
        // The matching-type recording carries only its own array; the other stays null.
        assertNull(recording.descriptionAttachments)
        client.close()
    }

    @Test
    fun spotlightUsesCanonicalFlatRouteAndReturnsRecording() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertEquals("/12345/recordings/456/spotlight.json", request.url.encodedPath)
            respond(
                content = recordingJson(456, ""),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val recording = client.forAccount("12345").recordings.spotlight(456)
        assertEquals(456L, recording.id)
        assertEquals("Message", recording.type)
        client.close()
    }

    @Test
    fun spotlightSurfacesIneligibleRecording() = runTest {
        val client = mockClient {
            respond(
                content = """{"errors":["Recording cannot be spotlighted"]}""",
                status = HttpStatusCode.UnprocessableEntity,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val error = assertFailsWith<BasecampException.Validation> {
            client.forAccount("12345").recordings.spotlight(456)
        }
        assertEquals(422, error.httpStatus)
        client.close()
    }

    @Test
    fun unspotlightUsesCanonicalFlatRoute() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Delete, request.method)
            assertEquals("/12345/recordings/456/spotlight.json", request.url.encodedPath)
            respond(content = "", status = HttpStatusCode.NoContent)
        }

        client.forAccount("12345").recordings.unspotlight(456)
        client.close()
    }

    @Test
    fun unspotlightSurfacesPermissionError() = runTest {
        val client = mockClient { respond(content = "", status = HttpStatusCode.Forbidden) }

        val error = assertFailsWith<BasecampException.Forbidden> {
            client.forAccount("12345").recordings.unspotlight(456)
        }
        assertEquals(403, error.httpStatus)
        client.close()
    }

    // Mirrors spec/fixtures/recordings/filing_pending.json and filing_completed.json.
    private val pendingFilingJson = """{
        "id": 4,
        "status": "pending",
        "type": "Move",
        "source_recording_id": 1069479351,
        "url": "https://3.basecampapi.com/195539477/filings/4.json"
    }"""

    private val completedFilingJson = """{
        "id": 4,
        "status": "completed",
        "type": "Move",
        "source_recording_id": 1069479351,
        "url": "https://3.basecampapi.com/195539477/filings/4.json",
        "destination_recording": {
            "id": 1069480270,
            "type": "Message",
            "url": "https://3.basecampapi.com/195539477/buckets/2085958505/messages/1069480270.json",
            "app_url": "https://3.basecamp.com/195539477/buckets/2085958505/messages/1069480270"
        }
    }"""

    private fun jsonHeaders() = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString())

    @Test
    fun moveSendsTheDestinationProjectAndReadsThePendingFiling() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertEquals("/12345/recordings/1069479351/moves.json", request.url.encodedPath)
            assertEquals("""{"destination_project_id":2085958505}""", (request.body as TextContent).text)
            respond(pendingFilingJson, HttpStatusCode.Created, jsonHeaders())
        }

        val filing = client.forAccount("12345").recordings.move(
            1069479351, MoveRecordingBody(destinationProjectId = 2085958505),
        )

        assertEquals(4L, filing.id)
        assertEquals("pending", filing.status)
        assertEquals("Move", filing.type)
        assertEquals(1069479351L, filing.sourceRecordingId)
        assertEquals("https://3.basecampapi.com/195539477/filings/4.json", filing.url)
        assertNull(filing.destinationRecording)
        client.close()
    }

    @Test
    fun moveSurfacesARefusal() = runTest {
        val client = mockClient {
            respond("""{"status":422,"error":"Unprocessable Entity"}""", HttpStatusCode.UnprocessableEntity, jsonHeaders())
        }

        val error = assertFailsWith<BasecampException.Validation> {
            client.forAccount("12345").recordings.move(1069479351, MoveRecordingBody(filingDestinationId = 4001))
        }
        assertEquals(422, error.httpStatus)
        client.close()
    }

    @Test
    fun copySendsTheDestinationAndIncludeComments() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertEquals("/12345/recordings/1069479351/copies.json", request.url.encodedPath)
            assertEquals(
                """{"destination_project_id":2085958505,"include_comments":true}""",
                (request.body as TextContent).text,
            )
            respond(pendingFilingJson, HttpStatusCode.Created, jsonHeaders())
        }

        val filing = client.forAccount("12345").recordings.copy(
            1069479351, CopyRecordingBody(destinationProjectId = 2085958505, includeComments = true),
        )

        assertEquals("pending", filing.status)
        assertNull(filing.destinationRecording)
        client.close()
    }

    // A copy is not idempotent: a retried 503 could make two copies.
    @Test
    fun copyIsSentOnceOnA503WithRetriesEnabled() = runTest {
        var attempts = 0
        val client = testBasecampClient {
            accessToken("test-token")
            maxRetries = 3
            engine = MockEngine {
                attempts++
                respond("{}", HttpStatusCode.ServiceUnavailable, headersOf("Retry-After", "1"))
            }
        }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("12345").recordings.copy(
                1069479351, CopyRecordingBody(filingDestinationId = 4001, includeComments = true),
            )
        }
        assertEquals(503, error.httpStatus)
        assertEquals(1, attempts)
        client.close()
    }

    // Mirrors spec/fixtures/recordings/get_tool.json: a dock tool has no parent.
    private val toolJson = """{
        "id": 100,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z",
        "title": "Message Board",
        "inherits_status": true,
        "type": "Message::Board",
        "url": "https://3.basecampapi.com/12345/buckets/1/message_boards/100.json",
        "app_url": "https://3.basecamp.com/12345/buckets/1/message_boards/100",
        "position": 1,
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "creator": {"id": 1, "name": "Test User", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"}
    }"""

    @Test
    fun getReadsARecordingWithoutKnowingItsType() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Get, request.method)
            assertEquals("/12345/recordings/456", request.url.encodedPath)
            respond(recordingJson(456, ""), HttpStatusCode.OK, jsonHeaders())
        }

        val recording = client.forAccount("12345").recordings.get(456)

        assertEquals("Message", recording.type)
        val parent = assertNotNull(recording.parent)
        assertEquals(100L, parent.id)
        assertEquals("Message::Board", parent.type)
        assertEquals(1L, recording.bucket.id)
        client.close()
    }

    @Test
    fun getReadsADockToolWithoutAParent() = runTest {
        val client = mockClient { respond(toolJson, HttpStatusCode.OK, jsonHeaders()) }

        val recording = client.forAccount("12345").recordings.get(100)

        assertEquals("Message::Board", recording.type)
        assertNull(recording.parent)
        client.close()
    }

    @Test
    fun getSurfacesNotFound() = runTest {
        val client = mockClient { respond("", HttpStatusCode.NotFound) }

        assertFailsWith<BasecampException.NotFound> {
            client.forAccount("12345").recordings.get(9999)
        }
        client.close()
    }

    @Test
    fun getFilingReadsTheCompletedFilingAndItsDestination() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Get, request.method)
            assertEquals("/12345/filings/4", request.url.encodedPath)
            respond(completedFilingJson, HttpStatusCode.OK, jsonHeaders())
        }

        val filing = client.forAccount("12345").recordings.getFiling(4)

        assertEquals("completed", filing.status)
        val destination = assertNotNull(filing.destinationRecording)
        assertEquals(1069480270L, destination.id)
        assertEquals("Message", destination.type)
        assertEquals("https://3.basecampapi.com/195539477/buckets/2085958505/messages/1069480270.json", destination.url)
        assertEquals("https://3.basecamp.com/195539477/buckets/2085958505/messages/1069480270", destination.appUrl)
        client.close()
    }

    @Test
    fun getFilingSurfacesNotFound() = runTest {
        val client = mockClient {
            respond("""{"status":404,"error":"Not Found"}""", HttpStatusCode.NotFound, jsonHeaders())
        }

        val error = assertFailsWith<BasecampException.NotFound> {
            client.forAccount("12345").recordings.getFiling(4)
        }
        assertEquals(404, error.httpStatus)
        client.close()
    }
}
