package com.basecamp.sdk

import com.basecamp.sdk.generated.models.Notification
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

class BubbleUpsDecodeTest {

    private val json = Json { ignoreUnknownKeys = true }

    private val fixtureJson = """
        [
          {
            "id": 2,
            "created_at": "2026-07-21T00:01:43.009Z",
            "updated_at": "2026-07-21T00:01:43.031Z",
            "section": "bubbles",
            "unread_count": 0,
            "read_at": "2026-07-21T00:01:43.031Z",
            "title": "We won Leto!",
            "type": "Message",
            "bucket_name": "The Leto Laptop"
          },
          {
            "id": 3,
            "created_at": "2026-07-21T00:02:00.000Z",
            "updated_at": "2026-07-21T00:02:00.000Z",
            "section": "bubbles",
            "unread_count": 1,
            "title": "Scheduled follow-up",
            "type": "Todo",
            "bubble_up_at": "2026-08-01T00:00:00Z"
          }
        ]
    """.trimIndent()

    @Test
    fun decodesBubbleUpsNotifications() {
        val notifications = json.decodeFromString<List<Notification>>(fixtureJson)

        assertEquals(2, notifications.size)

        // Notification 0: current bubble-up (no scheduled bubble_up_at).
        assertEquals(2L, notifications[0].id)
        assertEquals("We won Leto!", notifications[0].title)
        assertEquals("Message", notifications[0].type)
        assertNull(notifications[0].bubbleUpAt)

        // Notification 1: scheduled bubble-up carries bubble_up_at.
        assertEquals("2026-08-01T00:00:00Z", notifications[1].bubbleUpAt)
    }
}
