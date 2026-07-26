package com.basecamp.sdk

import com.basecamp.sdk.generated.models.TimelineEvent
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class TimelineDecodeTest {

    private val json = Json { ignoreUnknownKeys = true }

    private val fixtureJson = """
        [
          {
            "id": 1,
            "created_at": "2024-03-15T10:30:00Z",
            "kind": "chat_transcript_rollup",
            "avatars_sample": [
              "https://3.basecampapi.com/1/people/aaa/avatar",
              "https://3.basecampapi.com/1/people/bbb/avatar"
            ]
          },
          {
            "id": 2,
            "created_at": "2024-03-15T10:31:00Z",
            "kind": "schedule_entry_created",
            "avatars_sample": [],
            "data": {
              "all_day": true,
              "starts_at": "2025-10-30",
              "ends_at": "2025-10-30"
            }
          },
          {
            "id": 3,
            "created_at": "2024-03-15T10:32:00Z",
            "kind": "upload_created",
            "avatars_sample": [],
            "attachments": [
              {
                "id": 900,
                "type": "Upload",
                "status": "active",
                "visible_to_clients": false,
                "title": "Diagram",
                "filename": "diagram.png",
                "content_type": "image/png",
                "byte_size": 20480,
                "width": 1024.0,
                "height": 768.0,
                "url": "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
                "app_url": "https://3.basecamp.com/1/buckets/2/uploads/900",
                "download_url": "https://3.basecampapi.com/1/buckets/2/uploads/900/download/diagram.png",
                "app_download_url": "https://3.basecamp.com/1/buckets/2/uploads/900/download"
              }
            ]
          },
          {
            "id": 4,
            "created_at": "2024-03-15T10:33:00Z",
            "kind": "comment_created",
            "avatars_sample": [],
            "attachments": [
              {
                "id": 500,
                "attachable_sgid": "sgid-attachable-500",
                "sgid": "sgid-500",
                "status_url": "https://3.basecampapi.com/1/attachments/sgid-500/status.json",
                "caption": "See attached",
                "filename": "notes.pdf",
                "content_type": "application/pdf",
                "byte_size": 4096,
                "key": "blobkey500",
                "width": null,
                "height": null,
                "previewable": true,
                "download_url": "https://3.basecampapi.com/1/blobs/blobkey500/download/notes.pdf",
                "preview_url": "https://3.basecampapi.com/1/blobs/blobkey500/previews/full",
                "thumbnail_url": "https://3.basecampapi.com/1/blobs/blobkey500/previews/card"
              }
            ]
          }
        ]
    """.trimIndent()

    @Test
    fun decodesAdditiveTimelineFields() {
        val events = json.decodeFromString<List<TimelineEvent>>(fixtureJson)

        assertEquals(4, events.size)

        // Event 0: non-empty avatars_sample. The optional array is nullable
        // (absent decodes to null); this fixture populates it.
        assertEquals(2, events[0].avatarsSample!!.size)

        // Event 1: schedule-entry timing payload, all-day date-only bounds.
        val data = events[1].data
        assertNotNull(data)
        assertEquals(true, data.allDay)
        assertEquals("2025-10-30", data.startsAt)
        assertEquals("2025-10-30", data.endsAt)

        // Event 2: full Upload recording attachment; FlexibleIntSerializer decodes 1024.0 -> 1024.
        val upload = events[2].attachments!!
        assertEquals(1, upload.size)
        assertEquals("Upload", upload[0].type)
        assertEquals("diagram.png", upload[0].filename)
        assertEquals("https://3.basecamp.com/1/buckets/2/uploads/900/download", upload[0].appDownloadUrl)
        assertEquals(1024, upload[0].width)

        // Event 3: rich-text attachment/blob partial variant.
        val blob = events[3].attachments!!
        assertEquals(1, blob.size)
        assertEquals("sgid-attachable-500", blob[0].attachableSgid)
        assertEquals("See attached", blob[0].caption)
        assertEquals("blobkey500", blob[0].key)
        // previewable is now nullable (optional boolean, presence-faithful);
        // this fixture sets it true, so assert the explicit value.
        assertEquals(true, blob[0].previewable)
        assertNull(blob[0].width)
    }
}
