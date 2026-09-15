package com.basecamp.sdk.services

import com.basecamp.sdk.BasecampClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.recordings
import com.basecamp.sdk.testBasecampClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.ContentType
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class RecordingsSummarizeTest {

    private companion object {
        const val BUCKET = 2085958499L
        const val LINE = 1069479350L

        /** An attachable Person sgid for 1049715915, as BC3 serves it. */
        const val PERSON_SGID =
            "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7AFRJ" +
                "Ig9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102"
    }

    private val recordedPaths = mutableListOf<String>()

    private fun jsonHeaders() = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString())

    /** A client whose mock answers by path, recording every path it is asked for. */
    private fun client(
        maxPages: Int? = null,
        route: (String) -> Pair<HttpStatusCode, String>,
    ): BasecampClient {
        val engine = MockEngine { request ->
            val path = request.url.encodedPath
            recordedPaths.add(path)
            val (status, body) = route(path)
            respond(content = body, status = status, headers = jsonHeaders())
        }
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            maxPages?.let { this.maxPages = it }
        }
    }

    private fun notFound() = HttpStatusCode.NotFound to """{"error":"Record not found"}"""
    private fun ok(body: String) = HttpStatusCode.OK to body

    private fun campfireJson(id: Long, bucketId: Long) = """{
        "id": $id, "status": "active", "visible_to_clients": false,
        "created_at": "2022-10-28T15:25:00.000Z", "updated_at": "2022-10-28T15:25:00.000Z",
        "title": "Campfire", "inherits_status": true, "type": "Chat::Transcript",
        "url": "https://3.basecampapi.com/999/buckets/$bucketId/chats/$id.json",
        "app_url": "https://3.basecamp.com/999/buckets/$bucketId/chats/$id",
        "bucket": {"id": $bucketId, "name": "Project $bucketId", "type": "Project"},
        "creator": {"id": 1049715914, "name": "Victor Cooper"}
    }"""

    /** [content] is inserted verbatim into the JSON string, so it arrives JSON-escaped. */
    private fun lineJson(type: String = "Chat::Lines::Text", content: String = "Hello everyone!") = """{
        "id": $LINE, "status": "active", "visible_to_clients": false,
        "created_at": "2022-10-28T15:25:00.000Z", "updated_at": "2022-10-28T15:25:00.000Z",
        "title": "Hello everyone!", "inherits_status": true, "type": "$type",
        "url": "https://3.basecampapi.com/999/buckets/$BUCKET/chats/1/lines/$LINE.json",
        "app_url": "https://3.basecamp.com/999/buckets/$BUCKET/chats/1#__recording_$LINE",
        "content": "$content",
        "parent": {"id": 1, "title": "Campfire", "type": "Chat::Transcript",
                   "url": "https://3.basecampapi.com/999/buckets/$BUCKET/chats/1.json",
                   "app_url": "https://3.basecamp.com/999/buckets/$BUCKET/chats/1"},
        "bucket": {"id": $BUCKET, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1049715914, "name": "Victor Cooper"}
    }"""

    private fun projectJson(bucketId: Long, campfireIds: List<Long>) = """{
        "id": $bucketId, "status": "active",
        "created_at": "2022-10-28T15:25:00.000Z", "updated_at": "2022-10-28T15:25:00.000Z",
        "name": "The Leto Laptop", "url": "https://3.basecampapi.com/999/projects/$bucketId.json",
        "app_url": "https://3.basecamp.com/999/projects/$bucketId",
        "dock": [${
        campfireIds.joinToString(",") {
            """{"id": $it, "title": "Campfire", "name": "chat", "enabled": true,
                 "url": "https://3.basecampapi.com/999/buckets/$bucketId/chats/$it.json",
                 "app_url": "https://3.basecamp.com/999/buckets/$bucketId/chats/$it"}"""
        }
    }]
    }"""

    private fun commentJson(bucketId: Long) = """{
        "id": 1069479361, "status": "active", "visible_to_clients": false,
        "created_at": "2022-10-30T01:01:58.169Z", "updated_at": "2022-10-30T01:01:58.169Z",
        "title": "Re: We won Leto!", "inherits_status": true, "type": "Comment",
        "url": "https://3.basecampapi.com/999/buckets/$bucketId/comments/1069479361.json",
        "app_url": "https://3.basecamp.com/999/buckets/$bucketId/comments/1069479361",
        "parent": {"id": 1, "title": "We won Leto!", "type": "Message",
                   "url": "https://3.basecampapi.com/999/buckets/$bucketId/messages/1.json",
                   "app_url": "https://3.basecamp.com/999/buckets/$bucketId/messages/1"},
        "bucket": {"id": $bucketId, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1049715915, "name": "Annie Bryan"},
        "content": "<div><bc-attachment sgid=\"$PERSON_SGID\"></bc-attachment> hi</div>",
        "content_attachments": []
    }"""

    // --- Routing --------------------------------------------------------------

    @Test
    fun refusesBoostBeforeAnyRequest() = runTest {
        val client = client { error("no request expected") }
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(bucketId = BUCKET, recordingId = 1, eventType = "boost.created"),
            )
        }
        assertEquals(BasecampException.RECORDING_NO_TYPE, failure.reason)
        assertEquals(BasecampException.RECORDING_NO_TYPE, failure.code)
        assertTrue(recordedPaths.isEmpty(), "routing fails before any request")
        client.close()
    }

    @Test
    fun refusesATypeOutsideTheRoutingTable() = runTest {
        val client = client { error("no request expected") }
        val account = client.forAccount("999")
        for (ref in listOf(
            RecordingRef(BUCKET, 1, recordingType = "Timesheet::Entry"),
            RecordingRef(BUCKET, 1, eventType = "thing.created"),
            // Not a feed type at all: no action segment.
            RecordingRef(BUCKET, 1, eventType = "comment"),
            RecordingRef(BUCKET, 1),
        )) {
            val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> { account.recordings.summarize(ref) }
            assertEquals(BasecampException.RECORDING_UNKNOWN_TYPE, failure.reason, "for $ref")
        }
        assertTrue(recordedPaths.isEmpty())
        client.close()
    }

    @Test
    fun recordingTypeWinsOverEventType() = runTest {
        val client = client { path -> if (path.endsWith("/comments/1069479361")) ok(commentJson(BUCKET)) else notFound() }
        val summary = client.forAccount("999").recordings.summarize(
            // The event type would route to a card; the recording type is exact.
            RecordingRef(BUCKET, 1069479361, eventType = "card.created", recordingType = "Comment"),
        )
        assertEquals("Comment", summary.type)
        assertEquals(listOf("/999/comments/1069479361"), recordedPaths)
        client.close()
    }

    @Test
    fun requiresBothIds() = runTest {
        val client = client { error("no request expected") }
        val account = client.forAccount("999")
        assertFailsWith<BasecampException.Usage> {
            account.recordings.summarize(RecordingRef(0, 1, eventType = "comment.created"))
        }
        assertFailsWith<BasecampException.Usage> {
            account.recordings.summarize(RecordingRef(BUCKET, 0, eventType = "comment.created"))
        }
        client.close()
    }

    @Test
    fun theRoutedSetIsTheDocumentedSet() {
        val types = summarizableRecordingTypes()
        assertEquals(types.sorted(), types, "the list is sorted")
        assertContains(types, "Chat::Lines::*")
        assertContains(types, "Kanban::Card")
        assertTrue("Timesheet::Entry" !in types, "the set is deliberate, not exhaustive")
        assertEquals(
            listOf("card.*", "chat.line.*", "comment.*", "message.*", "todo.*"),
            summarizableEventTypes(),
        )
        assertTrue(summarizableEventTypes().none { it.startsWith("boost") }, "boost names no recording type")
    }

    @Test
    fun refusesARecordingFromAnotherBucket() = runTest {
        val client = client { ok(commentJson(bucketId = 111)) }
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, 1069479361, eventType = "comment.created"),
            )
        }
        assertEquals(BasecampException.RECORDING_BUCKET_MISMATCH, failure.reason)
        client.close()
    }

    @Test
    fun readsTheMentionsOutOfTheProjectedContent() = runTest {
        val client = client { ok(commentJson(BUCKET)) }
        val summary = client.forAccount("999").recordings.summarize(
            RecordingRef(BUCKET, 1069479361, eventType = "comment.created"),
        )
        assertEquals(listOf(1049715915L), summary.mentionedPersonIds)
        assertNull(summary.campfireId, "only a chat line carries one")
        client.close()
    }

    // --- Chat-line discovery --------------------------------------------------

    @Test
    fun readsTheLineUnderTheProjectDockCampfireFirst() = runTest {
        val client = client { path ->
            when {
                path == "/999/projects/$BUCKET" -> ok(projectJson(BUCKET, listOf(77L)))
                path == "/999/chats/77/lines/$LINE" -> ok(lineJson())
                else -> notFound()
            }
        }
        val summary = client.forAccount("999").recordings.summarize(
            RecordingRef(BUCKET, LINE, eventType = "chat.line.created"),
        )
        assertEquals(77L, summary.campfireId)
        assertEquals(
            listOf("/999/projects/$BUCKET", "/999/chats/77/lines/$LINE"),
            recordedPaths,
            "the dock answers without ever fetching the account-wide listing",
        )
        client.close()
    }

    @Test
    fun fallsBackToTheAccountListingFilteredToTheBucket() = runTest {
        val client = client { path ->
            when (path) {
                "/999/chats.json" -> ok(
                    "[${campfireJson(400, bucketId = 999)}," +
                        "${campfireJson(340, BUCKET)},${campfireJson(345, BUCKET)}]",
                )
                "/999/chats/345/lines/$LINE" -> ok(lineJson())
                else -> notFound()
            }
        }
        val summary = client.forAccount("999").recordings.summarize(
            RecordingRef(BUCKET, LINE, eventType = "chat.line.created"),
        )
        assertEquals(345L, summary.campfireId)
        assertEquals(
            listOf(
                "/999/projects/$BUCKET",
                "/999/chats.json",
                "/999/chats/340/lines/$LINE",
                "/999/chats/345/lines/$LINE",
            ),
            recordedPaths,
            "the Campfire in the other bucket is never tried",
        )
        client.close()
    }

    @Test
    fun aPlainTextLineMentionsNobodyEvenWhenItsTextLooksLikeMarkup() = runTest {
        val literalTag = "<bc-attachment sgid=\\\"$PERSON_SGID\\\"></bc-attachment>"
        val client = client { path ->
            when (path) {
                "/999/chats.json" -> ok("[${campfireJson(345, BUCKET)}]")
                "/999/chats/345/lines/$LINE" -> ok(lineJson(type = "Chat::Lines::Text", content = literalTag))
                else -> notFound()
            }
        }
        val summary = client.forAccount("999").recordings.summarize(
            RecordingRef(BUCKET, LINE, recordingType = "Chat::Lines::Text"),
        )
        assertEquals(emptyList<Long>(), summary.mentionedPersonIds, "a text line's content was never read as markup")
        client.close()
    }

    @Test
    fun aRichTextLineReportsItsMentions() = runTest {
        val literalTag = "<bc-attachment sgid=\\\"$PERSON_SGID\\\"></bc-attachment>"
        val client = client { path ->
            when (path) {
                "/999/chats.json" -> ok("[${campfireJson(345, BUCKET)}]")
                "/999/chats/345/lines/$LINE" -> ok(
                    lineJson(type = "Chat::Lines::RichText", content = "$literalTag hi"),
                )
                else -> notFound()
            }
        }
        val summary = client.forAccount("999").recordings.summarize(
            RecordingRef(BUCKET, LINE, recordingType = "Chat::Lines::RichText"),
        )
        assertEquals(listOf(1049715915L), summary.mentionedPersonIds)
        client.close()
    }

    @Test
    fun everyCandidateAnsweringNotFoundIsUnresolvedNotAFailedRead() = runTest {
        val client = client { path ->
            when (path) {
                "/999/chats.json" -> ok("[${campfireJson(340, BUCKET)},${campfireJson(345, BUCKET)}]")
                else -> notFound()
            }
        }
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, LINE, eventType = "chat.line.created"),
            )
        }
        assertEquals(BasecampException.RECORDING_UNRESOLVED, failure.reason)
        assertEquals(listOf(340L, 345L), failure.campfireIds)
        assertEquals(false, failure.refreshed, "both sources were loaded during this call, so nothing was refreshed")
        assertEquals(emptyList<Long>(), failure.staleCampfireIds)
        client.close()
    }

    @Test
    fun aCandidatesForbiddenStopsTheLoopAndIsRaisedAsItself() = runTest {
        val client = client { path ->
            when (path) {
                "/999/chats.json" -> ok("[${campfireJson(340, BUCKET)},${campfireJson(345, BUCKET)}]")
                "/999/chats/340/lines/$LINE" -> HttpStatusCode.Forbidden to """{"error":"Access denied"}"""
                else -> notFound()
            }
        }
        assertFailsWith<BasecampException.Forbidden> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, LINE, eventType = "chat.line.created"),
            )
        }
        assertTrue(
            "/999/chats/345/lines/$LINE" !in recordedPaths,
            "a permission failure never masquerades as \"not here\", and the next candidate is not tried",
        )
        client.close()
    }

    @Test
    fun aBucketPastTheCandidateBudgetIsIncompleteNotUnresolved() = runTest {
        val campfires = (1L..(MAX_CAMPFIRE_CANDIDATES + 1)).joinToString(",") { campfireJson(it, BUCKET) }
        val client = client { path -> if (path == "/999/chats.json") ok("[$campfires]") else notFound() }
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, LINE, eventType = "chat.line.created"),
            )
        }
        assertEquals(BasecampException.CAMPFIRE_DISCOVERY_INCOMPLETE, failure.reason)
        assertEquals(
            MAX_CAMPFIRE_CANDIDATES,
            recordedPaths.count { it.startsWith("/999/chats/") },
            "the budget bounds the reads; nothing unsearched is reported absent",
        )
        client.close()
    }

    @Test
    fun listingOverflowIsReportedAsIncomplete() = runTest {
        var listingCalls = 0
        val engine = MockEngine { request ->
            val path = request.url.encodedPath
            recordedPaths.add(path)
            when (path) {
                "/999/chats.json" -> {
                    listingCalls++
                    respond(
                        content = "[${campfireJson(340, BUCKET)}]",
                        status = HttpStatusCode.OK,
                        headers = headersOf(
                            HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                            HttpHeaders.Link to listOf("<https://3.basecampapi.com/999/chats.json?page=2>; rel=\"next\""),
                        ),
                    )
                }
                else -> respond(
                    content = """{"error":"Record not found"}""",
                    status = HttpStatusCode.NotFound,
                    headers = jsonHeaders(),
                )
            }
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            maxPages = 1
        }
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, LINE, eventType = "chat.line.created"),
            )
        }
        assertEquals(BasecampException.CAMPFIRE_DISCOVERY_INCOMPLETE, failure.reason)
        assertEquals(1, listingCalls, "one listing fetch in this call")
        // The "not cached" half needs a SECOND call to prove: an overflowing
        // listing must not be stored, so the next summarize pays for its own
        // fetch rather than inheriting a snapshot that was never complete.
        assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, LINE, eventType = "chat.line.created"),
            )
        }
        assertEquals(2, listingCalls, "an overflowing listing is not cached, so the next call fetches again")
        client.close()
    }

    // --- Caching --------------------------------------------------------------

    @Test
    fun theDiscoverySourcesAreReadOncePerBucketWithinTheTtl() = runTest {
        val client = client { path ->
            when {
                path == "/999/projects/$BUCKET" -> ok(projectJson(BUCKET, listOf(77L)))
                path.startsWith("/999/chats/77/lines/") -> ok(lineJson())
                else -> notFound()
            }
        }
        val account = client.forAccount("999")
        repeat(3) {
            account.recordings.summarize(RecordingRef(BUCKET, LINE, eventType = "chat.line.created"))
        }
        assertEquals(
            1,
            recordedPaths.count { it == "/999/projects/$BUCKET" },
            "the dock is read once per bucket and reused inside the TTL",
        )
        client.close()
    }

    @Test
    fun aRefreshedSourceReportsWhichCachedCandidatesItNoLongerLists() = runTest {
        var clock = 0L
        val index = CampfireIndex { clock }
        var listingBody = "[${campfireJson(340, BUCKET)}]"
        val client = client { path ->
            when (path) {
                "/999/chats.json" -> ok(listingBody)
                "/999/chats/340/lines/${LINE + 1}" -> ok(lineJson())
                else -> notFound()
            }
        }
        val service = client.forAccount("999").recordings

        // Fills both caches, and finds a different line under Campfire 340.
        service.summarize(RecordingRef(BUCKET, LINE + 1, eventType = "chat.line.created"), index)

        // Past the refresh floor, inside the TTL: the miss re-reads both sources,
        // and the listing no longer shows the Campfire the cache held.
        clock = CAMPFIRE_INDEX_MIN_REFRESH_MILLIS
        listingBody = "[${campfireJson(345, BUCKET)}]"
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            service.summarize(RecordingRef(BUCKET, LINE, eventType = "chat.line.created"), index)
        }
        assertEquals(BasecampException.RECORDING_UNRESOLVED, failure.reason)
        assertTrue(failure.refreshed, "the cached sources were re-read before concluding")
        assertEquals(listOf(340L, 345L), failure.campfireIds)
        assertEquals(
            listOf(340L),
            failure.staleCampfireIds,
            "340 was visible when the cache filled and is not in the refreshed listing",
        )
        client.close()
    }

    @Test
    fun theRefreshFloorDeclinesASecondReadWithinThirtySeconds() = runTest {
        var clock = 0L
        val index = CampfireIndex { clock }
        val client = client { path ->
            when (path) {
                "/999/chats.json" -> ok("[${campfireJson(340, BUCKET)}]")
                "/999/chats/340/lines/${LINE + 1}" -> ok(lineJson())
                else -> notFound()
            }
        }
        val service = client.forAccount("999").recordings
        service.summarize(RecordingRef(BUCKET, LINE + 1, eventType = "chat.line.created"), index)
        val before = recordedPaths.count { it == "/999/chats.json" || it == "/999/projects/$BUCKET" }

        clock = CAMPFIRE_INDEX_MIN_REFRESH_MILLIS - 1
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            service.summarize(RecordingRef(BUCKET, LINE, eventType = "chat.line.created"), index)
        }
        assertEquals(BasecampException.RECORDING_UNRESOLVED, failure.reason)
        assertTrue(!failure.refreshed, "the floor declined the re-read, and the verdict says so")
        assertEquals(
            before,
            recordedPaths.count { it == "/999/chats.json" || it == "/999/projects/$BUCKET" },
            "a run of unresolvable lines cannot become a listing per line",
        )
        client.close()
    }

    @Test
    fun refusesARecordingFromAnotherBucketAndNamesBothBuckets() = runTest {
        val client = client { ok(commentJson(bucketId = 111)) }
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, 1069479361, eventType = "comment.created"),
            )
        }
        // The whole content of this failure is that the two differ, so both are
        // carried: one field cannot say which is which.
        assertEquals(BUCKET, failure.bucketId)
        assertEquals(111L, failure.readBucketId)
        client.close()
    }

    @Test
    fun theRoutingFailureReportsTheKeyTheCallerWrote() = runTest {
        val client = client { error("no request expected") }
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(RecordingRef(BUCKET, 1, recordingType = " Widget "))
        }
        assertTrue(
            "\" Widget \"" in failure.message.orEmpty(),
            "the key is trimmed to route and reported as written: ${failure.message}",
        )
        client.close()
    }

    @Test
    fun aTypeWithNoAssigneesEmitsNoAssigneesKey() = runTest {
        val client = client { ok(commentJson(BUCKET)) }
        val summary = client.forAccount("999").recordings.summarize(
            RecordingRef(BUCKET, 1069479361, eventType = "comment.created"),
        )
        val json = Json.encodeToString(RecordingSummary.serializer(), summary)
        assertTrue("assignees" !in json, "absent and empty are one thing, as Go's omitempty makes them: $json")
        assertTrue("parent" in json && "campfire_id" !in json, json)
        client.close()
    }

    @Test
    fun anUntypedRecordingWithNullNestedIdentitiesProjectsRatherThanRefusing() = runTest {
        // The cloud-storage reads hand back decoded JSON rather than a named
        // type, so this projection has to read a present-but-null key as absent
        // itself: a JSON null is JsonNull, not Kotlin null, and handing it to a
        // non-nullable serializer would throw a raw SerializationException out of
        // a composite that speaks only BasecampException.
        val client = client {
            ok(
                """{"id": 7, "status": "active", "type": "GoogleDocument", "title": "Roadmap",
                    "app_url": "https://3.basecamp.com/999/buckets/$BUCKET/google_documents/7",
                    "updated_at": "2022-11-22T08:30:00.000Z",
                    "parent": null, "bucket": null, "creator": null,
                    "description": "<div>Quarterly roadmap</div>"}""",
            )
        }
        val summary = client.forAccount("999").recordings.summarize(
            RecordingRef(BUCKET, 7, recordingType = "GoogleDocument"),
        )
        assertEquals("GoogleDocument", summary.type)
        assertEquals("<div>Quarterly roadmap</div>", summary.content)
        assertNull(summary.parent)
        assertNull(summary.bucket)
        assertNull(summary.creator)
        client.close()
    }

    @Test
    fun anUntypedRecordingWithAPartialNestedIdentityFailsAsAnApiErrorNotARawDecoderThrow() = runTest {
        // The cloud-storage reads are projected from decoded JSON by hand, and
        // the nested identities' serializers declare required members — so a
        // payload missing one raises a SerializationException. Unwrapped, that
        // escapes summarize through an error contract that promises only
        // BasecampException, while every generated read turns the identical
        // failure into SPEC §6's statusless api_error. Go never reaches this at
        // all: unmarshalling into its pointer structs leaves zero values.
        val client = client {
            ok(
                """{"id": 7, "status": "active", "type": "CloudFile", "title": "Brand book",
                    "app_url": "https://3.basecamp.com/999/buckets/$BUCKET/cloud_files/7",
                    "updated_at": "2022-11-22T08:30:00.000Z",
                    "bucket": {"id": $BUCKET, "name": "The Leto Laptop"},
                    "description": "<div>Draft</div>"}""",
            )
        }
        val failure = assertFailsWith<BasecampException.Api> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, 7, recordingType = "CloudFile"),
            )
        }
        assertNull(failure.httpStatus, "the request succeeded; no status describes this")
        assertNotNull(failure.decodeFailure, "carries the decoder's own refusal, as a generated read's does")
        client.close()
    }

    @Test
    fun aBudgetSpentBeforeTheListingIsIncompleteAndCostsNoListingRequest() = runTest {
        // The dock holds exactly the budget's worth of candidates and they all
        // answer 404. The budget is spent, but nothing was ever left UNTRIED, so
        // the `skipped` flag is false — and gating on `skipped` would let the
        // call go on to fetch the account listing. That request cannot help (no
        // budget remains to try anything it returns) and a failure on it would
        // replace this deterministic verdict with a transient error a consumer
        // retries forever.
        val campfireIds = (1L..MAX_CAMPFIRE_CANDIDATES).toList()
        val client = client { path ->
            when {
                path == "/999/projects/$BUCKET" -> ok(projectJson(BUCKET, campfireIds))
                path.startsWith("/999/chats/") -> notFound()
                else -> error("unexpected request to $path")
            }
        }
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            client.forAccount("999").recordings.summarize(
                RecordingRef(BUCKET, LINE, eventType = "chat.line.created"),
            )
        }
        assertEquals(BasecampException.CAMPFIRE_DISCOVERY_INCOMPLETE, failure.reason)
        assertTrue(
            "before the account listing was consulted" in failure.message.orEmpty(),
            "the reason names WHY it is incomplete: ${failure.message}",
        )
        assertTrue(
            recordedPaths.none { it == "/999/chats.json" },
            "the listing must not be fetched with no budget left to use it: $recordedPaths",
        )
        client.close()
    }

    @Test
    fun aBudgetSpentAfterTheListingWasConsultedIsUnresolvedNotIncomplete() = runTest {
        // The mirror image, and the reason "incomplete on a spent budget" is the
        // wrong fix: here BOTH sources were consulted and every candidate was
        // searched. Nothing is unsearched, so the answer is a settled
        // "unresolved" rather than "incomplete" — and the cached listing is not
        // re-read, because it cannot hand this call a candidate it may try.
        val listingIds = (1L..MAX_CAMPFIRE_CANDIDATES).toList()
        val listing = "[" + listingIds.joinToString(",") { campfireJson(it, BUCKET) } + "]"
        var listingFetches = 0
        val client = client { path ->
            when {
                path == "/999/projects/$BUCKET" -> notFound()
                path == "/999/chats.json" -> { listingFetches++; ok(listing) }
                path.startsWith("/999/chats/") -> notFound()
                else -> error("unexpected request to $path")
            }
        }
        val service = client.forAccount("999").recordings
        // First call fills the listing cache and spends nothing this call keeps.
        assertFailsWith<BasecampException.RecordingSummaryFailure> {
            service.summarize(RecordingRef(BUCKET, LINE, eventType = "chat.line.created"))
        }
        val fetchesAfterFirst = listingFetches
        // Second call: the listing is cached, so it is consulted in pass 1 and
        // the whole budget goes on it.
        val failure = assertFailsWith<BasecampException.RecordingSummaryFailure> {
            service.summarize(RecordingRef(BUCKET, LINE + 1, eventType = "chat.line.created"))
        }
        assertEquals(BasecampException.RECORDING_UNRESOLVED, failure.reason)
        assertEquals(MAX_CAMPFIRE_CANDIDATES, failure.campfireIds.size)
        assertEquals(fetchesAfterFirst, listingFetches, "a consulted source is not re-read with the budget spent")
        client.close()
    }

    @Test
    fun anUntypedRecordingWithAWrongTypedScalarIsRefusedNotProjectedEmpty() = runTest {
        // A present value of the wrong type is a malformed body, not an absent
        // field. Reading the projection key by key coerced one into "" and
        // produced a successful summary with a blank title; the typed reads and
        // the Go reference both refuse such a body, and the whole point of
        // decoding this one through a declared shape is that it refuses it too.
        //
        // `coerceInputValues` is on for this client, so this is measured rather
        // than assumed: it rescues an explicit null for a non-nullable member
        // with a default, and does NOT rescue a type mismatch.
        for (field in listOf("title", "status", "app_url", "updated_at", "description")) {
            val client = client {
                ok(
                    """{"id": 7, "type": "CloudFile", "$field": 42,
                        "app_url": "https://3.basecamp.com/999/x", "updated_at": "2022-11-22T08:30:00.000Z"}"""
                        .replace("\"app_url\": \"https", if (field == "app_url") "\"_unused\": \"https" else "\"app_url\": \"https")
                        .replace("\"updated_at\": \"2022", if (field == "updated_at") "\"_unused2\": \"2022" else "\"updated_at\": \"2022"),
                )
            }
            val failure = assertFailsWith<BasecampException.Api>("$field: a wrong-typed scalar must be refused") {
                client.forAccount("999").recordings.summarize(
                    RecordingRef(BUCKET, 7, recordingType = "CloudFile"),
                )
            }
            assertNull(failure.httpStatus, "$field: the request succeeded, so no status describes this")
            assertNotNull(failure.decodeFailure, "$field: carries the decoder's own refusal")
            client.close()
        }
    }
}
