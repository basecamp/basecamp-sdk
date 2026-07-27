package com.basecamp.sdk

import com.basecamp.sdk.generated.models.EverythingFile
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull

class EverythingFilesDecodeTest {

    private val json = Json { ignoreUnknownKeys = true }

    // Canonical /files.json "everything" feed: a heterogeneous superset carrying a full
    // Upload recording, a Basecamp Document recording, and a rich-text Attachment envelope
    // in a single array. everythingFiles(options) decodes this as List<EverythingFile>; here
    // we prove the generated EverythingFile superset decodes every variant's real per-variant
    // fields at runtime.
    private val fixtureJson = """
        [
          {
            "id": 900,
            "type": "Upload",
            "status": "active",
            "visible_to_clients": false,
            "title": "logo.png",
            "inherits_status": true,
            "filename": "logo.png",
            "content_type": "image/png",
            "byte_size": 1281,
            "width": 1024.0,
            "height": 768.0,
            "url": "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
            "app_url": "https://3.basecamp.com/1/buckets/2/uploads/900",
            "download_url": "https://3.basecampapi.com/1/buckets/2/uploads/900/download/logo.png",
            "app_download_url": "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png",
            "bucket": { "id": 2, "name": "The Leto Laptop", "type": "Project" },
            "creator": { "id": 1, "name": "Victor Cooper" }
          },
          {
            "id": 901,
            "type": "Document",
            "status": "active",
            "visible_to_clients": false,
            "title": "Spec",
            "inherits_status": true,
            "content_type": "text/html",
            "url": "https://3.basecampapi.com/1/buckets/2/documents/901.json",
            "app_url": "https://3.basecamp.com/1/buckets/2/documents/901",
            "bucket": { "id": 2, "name": "The Leto Laptop", "type": "Project" },
            "creator": { "id": 1, "name": "Victor Cooper" }
          },
          {
            "id": 902,
            "type": "Attachment",
            "attachable_sgid": "sgid-902",
            "filename": "chart.avif",
            "content_type": "image/avif",
            "byte_size": 4096,
            "width": null,
            "height": null,
            "download_url": "https://storage.3.basecamp.com/1/blobs/902/download/chart.avif",
            "parent": {
              "id": 800,
              "title": "A message",
              "type": "Message",
              "url": "https://3.basecampapi.com/1/buckets/2/messages/800.json",
              "app_url": "https://3.basecamp.com/1/buckets/2/messages/800"
            }
          }
        ]
    """.trimIndent()

    @Test
    fun decodesHeterogeneousEverythingFilesFeed() {
        val files = json.decodeFromString<List<EverythingFile>>(fixtureJson)

        assertEquals(3, files.size)

        // Variant 0: full Upload recording; FlexibleIntSerializer collapses 1024.0 -> 1024.
        assertEquals("Upload", files[0].type)
        assertEquals("logo.png", files[0].filename)
        assertNotNull(files[0].appDownloadUrl)
        assertEquals(
            "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png",
            files[0].appDownloadUrl,
        )
        assertEquals(1024, files[0].width)

        // Variant 1: Basecamp Document recording.
        assertEquals("Document", files[1].type)
        assertEquals("Spec", files[1].title)

        // Variant 2: rich-text Attachment envelope; null image dimensions decode to null.
        assertEquals("Attachment", files[2].type)
        assertEquals("sgid-902", files[2].attachableSgid)
        assertEquals("chart.avif", files[2].filename)
        assertNull(files[2].width)
        // The attachment envelope identifies the doc/message the file lives in.
        assertNotNull(files[2].parent)
        assertEquals("Message", files[2].parent?.type)
    }
}
