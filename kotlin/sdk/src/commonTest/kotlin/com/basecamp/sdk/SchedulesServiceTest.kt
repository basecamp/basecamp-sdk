package com.basecamp.sdk

import com.basecamp.sdk.generated.schedules
import com.basecamp.sdk.generated.services.ReplaceScheduleEntryBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertSame
import kotlin.test.assertTrue

/**
 * The merge-safe `updateEntry` / read-modify-write `editEntry` composites and
 * the raw `replaceEntry` they are built on.
 *
 * `PUT /schedule_entries/{id}` is a full replace: BC3 rebuilds the entry from
 * only the permitted params, so a sparse PUT that omits `description` erases
 * it, one that omits `summary` leaves the entry reading back as "Untitled", and
 * one that omits `all_day` turns an all-day event into a midnight-to-midnight
 * timed one. None of those is a 422 — every one is a 200 that quietly clears.
 *
 * The writable set splits in two, and the split is the whole point:
 *
 * - **Full state** (`summary`, `starts_at`, `ends_at`, `description`,
 *   `all_day`) is always resent, empties included.
 * - **Addressed-only** (`participant_ids`, `url`, `highlighted`, `notify`) is
 *   sent only when the caller addressed it, and is never seeded onto the wire
 *   from the read-back — BC3 preserves those three server-side, and the join
 *   link comes back as `join_url` rather than `url`.
 */
class SchedulesServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    /**
     * A populated read-back: a join link, a highlight and two participants are
     * all present, so any test that asserts they stay off the wire is asserting
     * against something the composite could have echoed.
     */
    private fun fullEntryJson(id: Long = 1069479523) = """{
        "id": $id, "status": "active", "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
        "title": "Team Meeting", "inherits_status": true, "type": "Schedule::Entry",
        "url": "https://3.basecampapi.com/999/buckets/1/schedule_entries/$id.json",
        "app_url": "https://3.basecamp.com/999/buckets/1/schedule_entries/$id",
        "parent": {"id": 1069479521, "title": "Schedule", "type": "Schedule", "url": "https://3.basecampapi.com/999/buckets/1/schedules/1069479521.json", "app_url": "https://3.basecamp.com/999/buckets/1/schedules/1069479521"},
        "bucket": {"id": 1, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1049715914, "name": "Victor Cooper", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
        "summary": "Team Meeting",
        "description": "<div>Agenda in the doc.</div>",
        "description_attachments": [],
        "all_day": false,
        "starts_at": "2026-06-05T06:00:00Z",
        "ends_at": "2026-06-05T08:30:00Z",
        "join_url": "https://meet.example.com/team",
        "highlighted": true,
        "participants": [
            {"id": 1049715914, "name": "Victor Cooper"},
            {"id": 1049715915, "name": "Annie Bryan"}
        ]
    }"""

    private class WriteCapture {
        val methods = mutableListOf<String>()
        var putBody: kotlinx.serialization.json.JsonObject? = null
    }

    private fun captureClient(
        capture: WriteCapture,
        body: String = fullEntryJson(),
    ): BasecampClient = mockClient { request ->
        capture.methods.add(request.method.value)
        if (request.method == HttpMethod.Put) {
            capture.putBody = json.parseToJsonElement(
                (request.body as io.ktor.http.content.TextContent).text
            ).jsonObject
        }
        respond(
            content = body,
            status = HttpStatusCode.OK,
            headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
        )
    }

    /** GETs a mangled entry, so the guard has to fire before anything is written. */
    private suspend fun assertRefusesRead(
        mangled: String,
        block: suspend (AccountClient) -> Unit,
    ): BasecampException.Api {
        val capture = WriteCapture()
        val client = mockClient { request ->
            capture.methods.add(request.method.value)
            respond(
                content = mangled,
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val error = assertFailsWith<BasecampException.Api> { block(client.forAccount("999")) }

        // Statusless and non-retryable: the transport succeeded, and
        // re-requesting cannot repair a malformed body. The ordering is what
        // matters — no PUT. A guard that fires after the PUT has already lost
        // the field.
        assertEquals(null, error.httpStatus)
        assertEquals(false, error.retryable)
        assertTrue(error.hint != null, "expected a hint naming the escape hatch")
        assertEquals(listOf("GET"), capture.methods)

        client.close()
        return error
    }

    // -- Merge-safe update --

    @Test
    fun updateMergesUnmentionedFullStateFields() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val entry = client.forAccount("999").schedules
            .updateEntry(1069479523, summary = "Team Meeting & Kickoff")

        assertEquals(1069479523L, entry.id)
        assertEquals(listOf("GET", "PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals("Team Meeting & Kickoff", body["summary"]?.jsonPrimitive?.content)
        // The four the caller never mentioned are written straight back rather
        // than left to the server's clear-by-default.
        assertEquals("2026-06-05T06:00:00Z", body["starts_at"]?.jsonPrimitive?.content)
        assertEquals("2026-06-05T08:30:00Z", body["ends_at"]?.jsonPrimitive?.content)
        assertEquals("<div>Agenda in the doc.</div>", body["description"]?.jsonPrimitive?.content)
        assertEquals("false", body["all_day"]?.jsonPrimitive?.content)

        client.close()
    }

    // The read-back carries a join link, a highlight and two participants. None
    // may be echoed: BC3 preserves all three server-side, so resending them is
    // redundant at best and wrong if the read raced a concurrent change — and
    // the response's `url` is the entry's own API URL, so echoing THAT into the
    // request's `url` would store the API URL as the join link.
    @Test
    fun updateNeverEchoesTheCarveOutsFromTheReadBack() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("999").schedules.updateEntry(1069479523, summary = "Team Sync")

        val body = capture.putBody!!
        assertTrue("participant_ids" !in body, "an unaddressed participant list must stay off the wire")
        assertTrue("url" !in body, "an unaddressed join link must stay off the wire")
        assertTrue("highlighted" !in body, "an unaddressed highlight must stay off the wire")
        assertTrue("notify" !in body, "an unaddressed notify directive must stay off the wire")

        client.close()
    }

    // The falsey-value trap. "", [] and false are addresses, not absences, and
    // must survive body compaction: BC3 preserves what the request does not
    // address, so an omitted clear is not a clear at all.
    @Test
    fun updateExplicitEmptyCarveOutsReachTheWire() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("999").schedules.updateEntry(
            1069479523,
            url = "",
            highlighted = false,
            participantIds = emptyList(),
        )

        val body = capture.putBody!!
        assertTrue("url" in body, "an explicit clear must be sent, not omitted")
        assertEquals("", body["url"]?.jsonPrimitive?.content)
        assertTrue("highlighted" in body, "an explicit false must be sent, not omitted")
        assertEquals("false", body["highlighted"]?.jsonPrimitive?.content)
        assertTrue("participant_ids" in body, "an explicit empty list must be sent, not omitted")
        assertEquals(JsonArray(emptyList()), body["participant_ids"])

        client.close()
    }

    @Test
    fun updateAddressedCarveOutsAreIndependent() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("999").schedules.updateEntry(
            1069479523,
            url = "https://meet.example.com/new-room",
            highlighted = true,
        )

        val body = capture.putBody!!
        // The caller's join link goes on the wire under `url` — the request
        // spelling — even though the response returns it as `join_url`.
        assertEquals("https://meet.example.com/new-room", body["url"]?.jsonPrimitive?.content)
        assertEquals("true", body["highlighted"]?.jsonPrimitive?.content)
        // ...and participants stay off it, so the carve-outs are independent
        // rather than all-or-nothing.
        assertTrue("participant_ids" !in body, "an unaddressed participant list must stay off the wire")

        client.close()
    }

    // An all-day entry's bounds come back as bare dates. Round-tripping them
    // verbatim is the contract: parsing into a date/time type and re-rendering
    // would rewrite a value the caller never mentioned.
    @Test
    fun updateRoundTripsAnAllDayEntrysBareDatesVerbatim() = runTest {
        val capture = WriteCapture()
        val client = captureClient(
            capture,
            body = fullEntryJson()
                .replace("\"all_day\": false", "\"all_day\": true")
                .replace("\"starts_at\": \"2026-06-05T06:00:00Z\"", "\"starts_at\": \"2016-06-01\"")
                .replace("\"ends_at\": \"2026-06-05T08:30:00Z\"", "\"ends_at\": \"2016-06-02\""),
        )

        client.forAccount("999").schedules.updateEntry(1069479523, summary = "Offsite")

        val body = capture.putBody!!
        assertEquals("2016-06-01", body["starts_at"]?.jsonPrimitive?.content)
        assertEquals("2016-06-02", body["ends_at"]?.jsonPrimitive?.content)
        assertEquals("true", body["all_day"]?.jsonPrimitive?.content)

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
                content = fullEntryJson(),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            this.hooks = hooks
        }

        client.forAccount("999").schedules.updateEntry(1069479523, summary = "observed")

        // The composite is built from the public getEntry/replaceEntry, so
        // hooks see the two wire operations, not a synthetic composite.
        assertEquals(listOf("GetScheduleEntry", "ReplaceScheduleEntry"), operations)

        client.close()
    }

    // -- Read-modify-write edit --

    @Test
    fun editPutsFullStateBack() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val entry = client.forAccount("999").schedules.editEntry(1069479523) {
            assertEquals("Team Meeting", summary)
            assertEquals("2026-06-05T06:00:00Z", startsAt)
            assertEquals("<div>Agenda in the doc.</div>", description)
            assertEquals(false, allDay)
            // The carve-outs are seeded for reading: `url` from join_url, and
            // participant_ids from the participants' ids.
            assertEquals("https://meet.example.com/team", url)
            assertEquals(true, highlighted)
            assertEquals(listOf(1049715914L, 1049715915L), participantIds)
            description = ""
        }

        assertEquals(1069479523L, entry.id)
        assertEquals(listOf("GET", "PUT"), capture.methods)
        val body = capture.putBody!!
        // Clearing on a full-replace endpoint is an explicit "": never JSON
        // null (SPEC §18 body compaction), and never by omission.
        assertTrue("description" in body, "a cleared description must be sent present-and-empty")
        assertEquals("", body["description"]?.jsonPrimitive?.content)
        assertEquals("Team Meeting", body["summary"]?.jsonPrimitive?.content)
        assertEquals("2026-06-05T06:00:00Z", body["starts_at"]?.jsonPrimitive?.content)
        assertEquals("2026-06-05T08:30:00Z", body["ends_at"]?.jsonPrimitive?.content)
        assertEquals("false", body["all_day"]?.jsonPrimitive?.content)

        client.close()
    }

    // Reading a carve-out is not addressing it: the block inspects all three
    // and assigns none, so the edit view cannot simply serialize what it was
    // seeded with.
    @Test
    fun editUntouchedCarveOutsStayOffTheWire() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("999").schedules.editEntry(1069479523) {
            assertEquals("https://meet.example.com/team", url)
            assertEquals(true, highlighted)
            assertEquals(listOf(1049715914L, 1049715915L), participantIds)
            summary = "Team Sync"
        }

        val body = capture.putBody!!
        assertEquals("Team Sync", body["summary"]?.jsonPrimitive?.content)
        assertTrue("participant_ids" !in body, "an untouched participant list must stay off the wire")
        assertTrue("url" !in body, "an untouched join link must stay off the wire")
        assertTrue("highlighted" !in body, "an untouched highlight must stay off the wire")

        client.close()
    }

    // The reason the contract is setter-invocation dirty tracking and not a
    // snapshot diff: the block assigns exactly what the GET returned, so a
    // value-comparison implementation would conclude nothing changed and omit
    // both — handing the write back to BC3's preserve-on-omission. Intent is
    // not recoverable from the value.
    @Test
    fun editAssigningTheReadBacksOwnValueStillSendsIt() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("999").schedules.editEntry(1069479523) {
            url = url
            highlighted = highlighted
            participantIds = participantIds
        }

        val body = capture.putBody!!
        assertEquals("https://meet.example.com/team", body["url"]?.jsonPrimitive?.content)
        assertEquals("true", body["highlighted"]?.jsonPrimitive?.content)
        assertEquals(
            listOf(1049715914L, 1049715915L),
            body["participant_ids"]?.jsonArray?.map { it.jsonPrimitive.long },
        )

        client.close()
    }

    // notify is a directive, not state: nothing in the read-back seeds it, and
    // it rides the wire only on an explicit assignment.
    @Test
    fun editAddressesNotifyOnlyWhenAssigned() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("999").schedules.editEntry(1069479523) {
            assertEquals(null, notify)
            notify = true
        }

        assertEquals("true", capture.putBody!!["notify"]?.jsonPrimitive?.content)

        client.close()
    }

    // A null assigned to a carve-out whose type has no empty spelling would be
    // stripped by body compaction, turning a stated address back into an
    // omission — exactly the defect this composite exists to prevent. Refuse it
    // instead of dropping it.
    @Test
    fun editRefusesANullBooleanCarveOut() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        assertFailsWith<BasecampException.Usage> {
            client.forAccount("999").schedules.editEntry(1069479523) {
                highlighted = null
            }
        }

        assertEquals(listOf("GET"), capture.methods)

        client.close()
    }

    @Test
    fun editBlockErrorAbortsWithoutPut() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        try {
            client.forAccount("999").schedules.editEntry(1069479523) {
                summary = "never written"
                error("abort")
            }
            kotlin.test.fail("expected the block error to propagate")
        } catch (e: IllegalStateException) {
            assertEquals("abort", e.message)
        }

        assertEquals(listOf("GET"), capture.methods)

        client.close()
    }

    // -- Raw replace --

    @Test
    fun replaceSendsSparseVerbatimWithNoGet() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val entry = client.forAccount("999").schedules.replaceEntry(
            1069479523,
            ReplaceScheduleEntryBody(
                summary = "Team Meeting",
                startsAt = "2026-06-05T06:00:00Z",
                endsAt = "2026-06-05T08:30:00Z",
            ),
        )

        assertEquals(1069479523L, entry.id)
        assertEquals(listOf("PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals("Team Meeting", body["summary"]?.jsonPrimitive?.content)
        // The raw path is destructive by design for the rebuilt fields and
        // silent for the carve-outs: what the caller left out stays out.
        assertTrue("description" !in body, "description must be omitted from a sparse replace")
        assertTrue("participant_ids" !in body, "participant_ids must be omitted from a sparse replace")
        assertTrue("url" !in body, "url must be omitted from a sparse replace")
        assertTrue("highlighted" !in body, "highlighted must be omitted from a sparse replace")

        client.close()
    }

    // -- Malformed-read refusal --

    // BC3 can never render a blank summary (Schedule::Entry#summary is
    // super.presence || "Untitled"), so "" on a 2xx read is malformed. The
    // model's non-null String already refuses absent/null; "" decodes fine and
    // needs the hand-written check.
    @Test
    fun updateRefusesABlankSummary() = runTest {
        assertRefusesRead(
            fullEntryJson().replace("\"summary\": \"Team Meeting\"", "\"summary\": \"   \"")
        ) { account -> account.schedules.updateEntry(1069479523, description = "<div>New.</div>") }
    }

    // summary is @required and non-null on the model, so kotlinx.serialization
    // refuses the absent case with a MissingFieldException. The composite
    // normalizes it into the SPEC §6 shape rather than letting a raw
    // SerializationException escape past callers catching BasecampException.
    @Test
    fun updateRefusesAnAbsentSummary() = runTest {
        assertRefusesRead(
            fullEntryJson().replace("\"summary\": \"Team Meeting\",", "")
        ) { account -> account.schedules.updateEntry(1069479523, description = "<div>New.</div>") }
    }

    // An ARRAY here, but a bare scalar would be refused too since #598 dropped
    // the client-wide `isLenient`, which used to render a JSON number or
    // boolean as a String instead of rejecting it. See `DecoderStrictnessTest`.
    @Test
    fun updateRefusesAWrongTypedSummary() = runTest {
        val error = assertRefusesRead(
            fullEntryJson().replace("\"summary\": \"Team Meeting\"", "\"summary\": [\"nope\"]")
        ) { account -> account.schedules.updateEntry(1069479523, description = "<div>New.</div>") }

        // The composite's own account of the failure, not the base layer's:
        // which record failed to decode, and the escape hatch. This is the half
        // of the discriminator that must keep firing — see
        // updateDoesNotRelabelAnAuthStrategyFailure for the half that must not.
        assertTrue(
            error.message!!.contains(
                "GetScheduleEntry returned a body that does not decode as a schedule entry"
            ),
            "expected the composite's message, got: ${error.message}",
        )
        // The restatement is still a malformed body, so it carries the marker
        // forward: rebuilding through the public constructor would drop it and
        // tell a caller — and the conformance runner — that this was not a
        // decode failure (#750).
        assertNotNull(error.decodeFailure, "the restatement must keep the decode-failure marker")
        assertTrue(
            error.hint?.contains("Use replaceEntry to write the record deliberately") == true,
            "expected a hint naming the escape hatch, got: ${error.hint}",
        )
    }

    /**
     * An auth strategy's already-classified failure is not this GET's decode
     * failure (#730).
     *
     * `BasecampHttpClient` propagates a `BasecampException` thrown by the
     * strategy untouched, deliberately: a token provider's own classification
     * is not the SDK's to overwrite. So a provider that decodes a JSON token
     * response and classifies its own decode failure as
     * `Api(cause = SerializationException(...))` lands in this composite's
     * catch — and while the discriminator was `cause is SerializationException`
     * it matched, and the caller was told "GetScheduleEntry returned a body
     * that does not decode as a schedule entry", with the merge-safe hint
     * attached, for a request that was never sent.
     *
     * The discriminator is now the internal slot only the response decoder
     * fills, so the strategy's exception arrives as itself.
     */
    @Test
    fun updateDoesNotRelabelAnAuthStrategyFailure() = runTest {
        var requests = 0
        val thrown = BasecampException.Api(
            message = "the token endpoint returned a body that does not decode",
            cause = SerializationException("Unexpected JSON token at offset 0"),
        )
        val client = testBasecampClient {
            auth(AuthStrategy { throw thrown })
            enableRetry = false
            engine = MockEngine {
                requests += 1
                respond(
                    content = fullEntryJson(),
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
                )
            }
        }

        val error = assertFailsWith<BasecampException.Api> {
            client.forAccount("999").schedules
                .updateEntry(1069479523, description = "<div>New.</div>")
        }

        assertSame(thrown, error, "the strategy's own exception must reach the caller unchanged")
        assertEquals(
            "the token endpoint returned a body that does not decode", error.message,
            "an auth failure must not be restated as a malformed schedule entry",
        )
        assertNull(error.hint, "the merge-safe escape hatch does not answer an auth failure")
        assertEquals(0, requests, "auth failed, so no request was ever sent")

        client.close()
    }

    // all_day is NOT NULL DEFAULT false and every partial emits it. Defaulting
    // a missing one to false would silently convert an all-day event into a
    // midnight-to-midnight timed one on a call that only changed the summary.
    @Test
    fun updateRefusesAMissingAllDay() = runTest {
        assertRefusesRead(
            fullEntryJson().replace("\"all_day\": false,", "")
        ) { account -> account.schedules.updateEntry(1069479523, summary = "Team Sync") }
    }

    // starts_at/ends_at are NOT NULL columns every partial emits, and both ride
    // back in the full-replace PUT.
    @Test
    fun updateRefusesAMissingStartsAt() = runTest {
        assertRefusesRead(
            fullEntryJson().replace("\"starts_at\": \"2026-06-05T06:00:00Z\",", "")
        ) { account -> account.schedules.updateEntry(1069479523, summary = "Team Sync") }
    }

    // description is optional and nullable — absent or null is genuinely empty
    // — but a structurally wrong-typed one is still malformed.
    @Test
    fun updateRefusesANonStringDescription() = runTest {
        assertRefusesRead(
            fullEntryJson().replace(
                "\"description\": \"<div>Agenda in the doc.</div>\"",
                "\"description\": [\"nope\"]",
            )
        ) { account -> account.schedules.updateEntry(1069479523, summary = "Team Sync") }
    }

    // The edit closure reads through the same guard, so it refuses the same
    // bodies — before the block runs, not after.
    @Test
    fun editRefusesAMalformedReadBeforeRunningTheBlock() = runTest {
        var blockRan = false
        assertRefusesRead(
            fullEntryJson().replace("\"summary\": \"Team Meeting\"", "\"summary\": \"\"")
        ) { account ->
            account.schedules.editEntry(1069479523) { blockRan = true }
        }
        assertEquals(false, blockRan, "the block must not run against a malformed read")
    }
}
