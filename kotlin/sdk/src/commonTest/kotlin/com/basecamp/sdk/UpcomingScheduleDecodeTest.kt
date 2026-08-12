package com.basecamp.sdk

import com.basecamp.sdk.generated.reports
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.MissingFieldException
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * `GetUpcomingSchedule` renders BC3's reduced calendar partials
 * (`app/views/api/schedules/calendar/`), not the per-resource ones.
 *
 * Kotlin's stake in #635 is specific: this operation used to return a bare
 * `JsonElement` and decode with `decodeFromString<JsonElement>`, so no contract
 * was enforced here at all — the spec could say anything and Kotlin would
 * neither confirm nor deny it. It now decodes into `UpcomingScheduleResult`,
 * whose members are `@Serializable` data classes with non-null required
 * properties, which is what these tests pin.
 *
 * The bodies are inline because KMP `commonTest` has no filesystem; the shared
 * `spec/fixtures/schedules/upcoming.json` copy is exercised across all six SDKs
 * by `conformance/tests/upcoming_schedule.json`.
 */
class UpcomingScheduleDecodeTest {

    private fun mockClient(body: String): BasecampClient = testBasecampClient {
        accessToken("test-token")
        engine = MockEngine {
            respond(
                content = body,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, "application/json"),
            )
        }
    }

    /** One timed entry, one all-day recurring occurrence, one to-do, one card. */
    private val populatedWindow = """{
        "schedule_entries": [
            {
                "id": 1069479523, "status": "active", "visible_to_clients": false,
                "url": "https://3.basecampapi.com/999/buckets/2085958499/schedule_entries/1069479523.json",
                "app_url": "https://3.basecamp.com/999/buckets/2085958499/schedule_entries/1069479523",
                "type": "ScheduleEntry", "summary": "Team Meeting",
                "all_day": false, "recurring": false,
                "starts_at": "2026-06-05T06:00:00.000Z", "ends_at": "2026-06-05T08:30:00.000Z",
                "creator": {"id": 1049715914, "name": "Victor Cooper", "avatar_url": "https://3.basecampapi.com/999/people/1049715914/avatar"},
                "participants": [
                    {"id": 1049715914, "name": "Victor Cooper", "avatar_url": "https://3.basecampapi.com/999/people/1049715914/avatar"},
                    {"id": 1049715915, "name": "Steve Marsh", "avatar_url": "https://3.basecampapi.com/999/people/1049715915/avatar"}
                ],
                "bucket": {"id": 2085958499, "name": "The Leto Laptop"},
                "comments_count": 2
            }
        ],
        "recurring_schedule_entry_occurrences": [
            {
                "id": 1069479524, "status": "active", "visible_to_clients": false,
                "url": "https://3.basecampapi.com/999/buckets/2085958499/schedule_entries/1069479524.json",
                "app_url": "https://3.basecamp.com/999/buckets/2085958499/schedule_entries/1069479524",
                "type": "ScheduleEntry", "summary": "All-hands",
                "all_day": true, "recurring": true,
                "starts_at": "2026-06-08", "ends_at": "2026-06-08",
                "creator": {"id": 1049715914, "name": "Victor Cooper", "avatar_url": "https://3.basecampapi.com/999/people/1049715914/avatar"},
                "participants": [],
                "bucket": {"id": 2085958499, "name": "The Leto Laptop"},
                "comments_count": 0
            }
        ],
        "assignables": [
            {
                "id": 1069479525, "status": "active", "visible_to_clients": false,
                "url": "https://3.basecampapi.com/999/buckets/2085958499/todos/1069479525.json",
                "app_url": "https://3.basecamp.com/999/buckets/2085958499/todos/1069479525",
                "starts_on": "2026-06-01", "due_on": "2026-06-10",
                "type": "todo", "content": "Ship the hardware",
                "assignees": [{"id": 1049715915, "name": "Steve Marsh", "avatar_url": "https://3.basecampapi.com/999/people/1049715915/avatar"}],
                "bucket": {"id": 2085958499, "name": "The Leto Laptop"},
                "parent": {"id": 1069479520, "title": "Launch: Hardware"},
                "completion_url": "https://3.basecampapi.com/999/buckets/2085958499/todos/1069479525/completion.json",
                "completed": true, "repeating": false,
                "completion": {
                    "created_at": "2026-06-09T15:04:05.000Z",
                    "creator": {"id": 1049715915, "name": "Steve Marsh", "avatar_url": "https://3.basecampapi.com/999/people/1049715915/avatar"}
                },
                "comments_count": 1
            },
            {
                "id": 1069479526, "status": "active", "visible_to_clients": false,
                "url": "https://3.basecampapi.com/999/buckets/2085958499/card_tables/cards/1069479526.json",
                "app_url": "https://3.basecamp.com/999/buckets/2085958499/card_tables/cards/1069479526",
                "starts_on": null, "due_on": null,
                "type": "card", "content": "Design the enclosure",
                "assignees": [],
                "bucket": {"id": 2085958499, "name": "The Leto Laptop"},
                "parent": {"id": 1069479519, "title": "In Progress"},
                "completion_url": "/999/buckets/2085958499/steps/1069479526/completions.json",
                "completed": false, "repeating": false,
                "comments_count": 0
            }
        ]
    }"""

    private val emptyWindow =
        """{"schedule_entries": [], "recurring_schedule_entry_occurrences": [], "assignables": []}"""

    @Test
    fun populatedWindowDecodesIntoTheReducedProjection() = runTest {
        val client = mockClient(populatedWindow)
        val result = client.forAccount("999").reports.upcoming("2026-06-01", "2026-06-30")

        assertEquals(1, result.scheduleEntries.size)
        assertEquals(1, result.recurringScheduleEntryOccurrences.size)
        assertEquals(2, result.assignables.size)

        val entry = result.scheduleEntries[0]
        assertEquals("Team Meeting", entry.summary)
        // Emitted only by the calendar partial, and the flag that separates the
        // two envelope arrays.
        assertEquals(false, entry.recurring)
        // id + name only: UpcomingScheduleBucket has no `type` member, which is
        // the nested omission that broke a strict decode against TodoBucket.
        assertEquals("The Leto Laptop", entry.bucket.name)
        assertEquals(2, entry.participants.size)
        assertEquals(2, entry.commentsCount)

        val occurrence = result.recurringScheduleEntryOccurrences[0]
        assertTrue(occurrence.recurring)
        assertTrue(occurrence.allDay)
        // An all-day entry reads back as a bare date, not a timestamp.
        assertEquals("2026-06-08", occurrence.startsAt)

        // BC3 spells the item text `content`. The retired schema declared
        // `title`, so the one field callers want was permanently absent.
        val todo = result.assignables[0]
        assertEquals("Ship the hardware", todo.content)
        assertEquals("todo", todo.type)
        assertEquals("Launch: Hardware", todo.parent.title)
        assertTrue(todo.completed)
        assertEquals("Steve Marsh", todo.completion?.creator?.name)

        val card = result.assignables[1]
        assertEquals("card", card.type)
        // Kanban::Card and Step both define starts_on as a literal nil to
        // duck-type Todo, and the partial reads it unconditionally.
        assertNull(card.startsOn)
        assertNull(card.dueOn)
        // The partial's one conditional key: absent, not null.
        assertNull(card.completion)
        // Non-to-dos get a `_path` helper, which emits no host.
        assertEquals("/999/buckets/2085958499/steps/1069479526/completions.json", card.completionUrl)
    }

    /**
     * The half that always worked, pinned so the failure claim stays precise:
     * an empty window is three empty arrays and decodes on any contract. What
     * failed elsewhere was any window carrying one entry, occurrence or
     * assignable.
     */
    @Test
    fun emptyWindowDecodes() = runTest {
        val client = mockClient(emptyWindow)
        val result = client.forAccount("999").reports.upcoming("2026-01-01", "2026-01-31")

        assertTrue(result.scheduleEntries.isEmpty())
        assertTrue(result.recurringScheduleEntryOccurrences.isEmpty())
        assertTrue(result.assignables.isEmpty())
    }

    /**
     * All three arrays are `@required`: BC3's index template writes every key
     * unconditionally. This is the assertion that could not exist while the
     * operation returned `JsonElement`.
     *
     * The decoder's [MissingFieldException] is what does the rejecting, and
     * since #604 the SDK reports it as the SPEC §6 malformed-2xx-body shape —
     * statusless, non-retryable, decoder exception kept as `cause` — rather than
     * letting it out raw.
     */
    @OptIn(ExperimentalSerializationApi::class)
    @Test
    fun envelopeMissingAnArrayIsRejected() = runTest {
        val client = mockClient("""{"schedule_entries": [], "recurring_schedule_entry_occurrences": []}""")

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("999").reports.upcoming("2026-01-01", "2026-01-31")
        }

        assertNull(error.httpStatus, "the transport succeeded, so no status describes this")
        assertFalse(error.retryable, "re-requesting cannot repair a malformed body")
        assertIs<MissingFieldException>(
            error.cause,
            "the absent required array must still be named by the cause, got ${error.cause}",
        )
    }
}
