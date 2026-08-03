package com.basecamp.sdk

import com.basecamp.sdk.generated.recordings
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
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
}
