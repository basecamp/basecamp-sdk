package com.basecamp.sdk

import com.basecamp.sdk.generated.gauges
import com.basecamp.sdk.generated.services.CreateGaugeNeedleBody
import com.basecamp.sdk.generated.services.ListGaugeNeedlesOptions
import com.basecamp.sdk.generated.services.ListGaugesOptions
import com.basecamp.sdk.generated.services.ToggleGaugeBody
import com.basecamp.sdk.generated.services.UpdateGaugeNeedleBody
import io.ktor.client.engine.mock.*
import io.ktor.client.request.HttpRequestData
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * All seven Gauges operations, happy path and error case each.
 *
 * Three route facts this file exists to pin, because none of them is what the
 * neighbouring services do:
 *
 *  - The gauge is a project **singleton**: `/projects/{id}/gauge.json`, with no
 *    gauge id anywhere in the URL and no `/buckets/` form at all.
 *  - `GET /gauge_needles/{id}` and `PUT`/`DELETE` on the same path carry **no
 *    `.json` suffix**. Deliberate in the spec, and asserted here as an exact
 *    path plus an explicit "does not end in .json", so a helpful suffix cannot
 *    be added back unnoticed.
 *  - `ToggleGauge` answers bc3's `head :ok` — a **200 with an empty body**, not
 *    a 204 — so the stubs below respond exactly that.
 *
 * Bodies are inline JSON literals faithful to `spec/fixtures/gauges/get.json`
 * and `spec/fixtures/gauges/needle_get.json` (same keys, same values; the
 * creator block and the attachment list are trimmed, as in ToolsServiceTest).
 * KMP `commonTest` has no filesystem, so no test in this source set reads a
 * shared fixture from disk.
 */
class GaugesServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private val accountId = "999999999"
    private val projectId = 2085958500L
    private val gaugeId = 1069479800L
    private val needleId = 1069479850L

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    /**
     * `spec/fixtures/gauges/get.json`. Note `url`/`app_url`: the gauge is a
     * per-project singleton, so its own id never appears in either.
     */
    private val gaugeJson = """{
        "id": $gaugeId,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2022-11-22T08:40:00.000Z",
        "updated_at": "2022-11-28T14:12:00.000Z",
        "title": "How far along are we?",
        "inherits_status": true,
        "type": "Gauge",
        "url": "https://3.basecampapi.com/$accountId/projects/$projectId/gauge.json",
        "app_url": "https://3.basecamp.com/$accountId/projects/$projectId/gauge",
        "bookmark_url": "https://3.basecampapi.com/$accountId/my/bookmarks/BAh7CEkiCGdpZAY6BkVU--abcd1234.json",
        "bucket": { "id": $projectId, "name": "The Leto Laptop", "type": "Project" },
        "creator": {
            "id": 1049715915,
            "name": "Victor Cooper",
            "email_address": "victor@honchodesign.com",
            "personable_type": "User",
            "title": "Chief Strategist"
        },
        "description": "<div>Shipped the new onboarding flow — see the burndown and the retro notes.</div>",
        "description_attachments": [
            {
                "id": 1069480040,
                "sgid": "BAh7CEkiCGdpZAY6BkVU--gauge001",
                "filename": "burndown.png",
                "content_type": "image/png",
                "byte_size": 40960,
                "download_url": "https://3.basecampapi.com/$accountId/blobs/BAh7CEkiCGdpZAY6BkVU--gauge001/download/burndown.png",
                "width": 1024.0,
                "height": 768,
                "previewable": true,
                "preview_url": "https://3.basecampapi.com/$accountId/blobs/BAh7CEkiCGdpZAY6BkVU--gauge001/previews/burndown.png",
                "thumbnail_url": "https://3.basecampapi.com/$accountId/blobs/BAh7CEkiCGdpZAY6BkVU--gauge001/thumbnails/burndown.png"
            }
        ],
        "enabled": true,
        "last_needle_color": "green",
        "last_needle_position": 72,
        "previous_needle_position": 45
    }"""

    /** `spec/fixtures/gauges/needle_get.json`, id parameterized for list pages. */
    private fun needleJson(id: Long = needleId, position: Int = 72, color: String = "green") = """{
        "id": $id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2022-11-28T14:12:00.000Z",
        "updated_at": "2022-11-28T14:12:00.000Z",
        "title": "Moved the needle",
        "inherits_status": true,
        "type": "Gauge::Needle",
        "url": "https://3.basecampapi.com/$accountId/projects/$projectId/gauge/needles/$id.json",
        "app_url": "https://3.basecamp.com/$accountId/projects/$projectId/gauge/needles/$id",
        "bookmark_url": "https://3.basecampapi.com/$accountId/my/bookmarks/BAh7CEkiCGdpZAY6BkVU--abcd1234.json",
        "subscription_url": "https://3.basecampapi.com/$accountId/buckets/$projectId/recordings/$id/subscription.json",
        "comments_count": 2,
        "comments_url": "https://3.basecampapi.com/$accountId/buckets/$projectId/recordings/$id/comments.json",
        "boosts_count": 3,
        "boosts_url": "https://3.basecampapi.com/$accountId/buckets/$projectId/recordings/$id/boosts.json",
        "parent": {
            "id": $gaugeId,
            "title": "How far along are we?",
            "type": "Gauge",
            "url": "https://3.basecampapi.com/$accountId/projects/$projectId/gauge.json",
            "app_url": "https://3.basecamp.com/$accountId/projects/$projectId/gauge"
        },
        "bucket": { "id": $projectId, "name": "The Leto Laptop", "type": "Project" },
        "creator": {
            "id": 1049715915,
            "name": "Victor Cooper",
            "email_address": "victor@honchodesign.com",
            "personable_type": "User",
            "title": "Chief Strategist"
        },
        "description": "<div>Shipped the new onboarding flow — see the burndown and the retro notes.</div>",
        "description_attachments": [],
        "color": "$color",
        "position": $position
    }"""

    private fun errorClient(status: HttpStatusCode, body: String): BasecampClient = mockClient { _ ->
        respond(
            content = body,
            status = status,
            headers = headersOf(
                HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                "X-Request-Id" to listOf("req-gauge-001"),
            ),
        )
    }

    // =========================================================================
    // ListGauges — GET /reports/gauges.json
    // =========================================================================

    @Test
    fun listGaugesGetsTheAccountWideReportsPathAndDecodesASingletonGauge() = runTest {
        var capturedRequest: HttpRequestData? = null

        val client = mockClient { request ->
            capturedRequest = request
            respond(
                content = "[$gaugeJson]",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("1"),
                ),
            )
        }

        val gauges = client.forAccount(accountId).gauges.listGauges(
            ListGaugesOptions(bucketIds = "$projectId,2085958501"),
        )

        assertEquals(HttpMethod.Get, capturedRequest!!.method)
        assertEquals("/$accountId/reports/gauges.json", capturedRequest!!.url.encodedPath)
        assertEquals("$projectId,2085958501", capturedRequest!!.url.parameters["bucket_ids"])
        assertEquals(null, capturedRequest!!.url.parameters["page"])

        assertEquals(1, gauges.size)
        val gauge = gauges[0].jsonObject
        assertEquals(gaugeId, gauge["id"]!!.jsonPrimitive.content.toLong())
        assertEquals("How far along are we?", gauge["title"]!!.jsonPrimitive.content)
        assertEquals("Gauge", gauge["type"]!!.jsonPrimitive.content)
        assertEquals(JsonPrimitive(true), gauge["enabled"])
        assertEquals(JsonPrimitive(72), gauge["last_needle_position"])
        assertEquals(JsonPrimitive(45), gauge["previous_needle_position"])
        assertEquals("green", gauge["last_needle_color"]!!.jsonPrimitive.content)

        // Singleton: the gauge id appears nowhere in its own URLs, and bc3 has
        // no /buckets/{id}/gauges/... route for it.
        val url = gauge["url"]!!.jsonPrimitive.content
        val appUrl = gauge["app_url"]!!.jsonPrimitive.content
        assertEquals("https://3.basecampapi.com/$accountId/projects/$projectId/gauge.json", url)
        assertEquals("https://3.basecamp.com/$accountId/projects/$projectId/gauge", appUrl)
        assertFalse(url.contains("$gaugeId"), "the gauge singleton URL must not carry the gauge id")
        assertFalse(url.contains("/buckets/"), "bc3 has no /buckets/... gauge route")

        assertEquals(1L, gauges.meta.totalCount)
        assertFalse(gauges.meta.truncated)

        client.close()
    }

    // SPEC §8: a positive page selects exactly that page in exactly ONE request,
    // the rel="next" Link is not followed, and truncated reports that it existed.
    @Test
    fun listGaugesPinnedPageIssuesOneRequestAndReportsUnfollowedTruncation() = runTest {
        var requestCount = 0
        val seenPages = mutableListOf<String?>()

        val client = mockClient { request ->
            requestCount++
            seenPages += request.url.parameters["page"]
            respond(
                content = "[$gaugeJson]",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "Link" to listOf("""<https://3.basecampapi.com/$accountId/reports/gauges.json?page=3>; rel="next""""),
                    "X-Total-Count" to listOf("9"),
                ),
            )
        }

        val gauges = client.forAccount(accountId).gauges.listGauges(ListGaugesOptions(page = 2))

        assertEquals(1, requestCount)
        assertEquals(listOf<String?>("2"), seenPages.toList())
        assertEquals(1, gauges.size)
        assertEquals(9L, gauges.meta.totalCount)
        assertTrue(gauges.meta.truncated, "an unfollowed next link is truncation")

        client.close()
    }

    @Test
    fun listGaugesWithoutAPinnedPageWalksTheLinkChain() = runTest {
        val seenPages = mutableListOf<String?>()

        val client = mockClient { request ->
            seenPages += request.url.parameters["page"]
            when (request.url.parameters["page"]) {
                null -> respond(
                    content = "[$gaugeJson]",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/$accountId/reports/gauges.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("2"),
                    ),
                )
                else -> respond(
                    content = "[$gaugeJson]",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString())),
                )
            }
        }

        val gauges = client.forAccount(accountId).gauges.listGauges()

        assertEquals(listOf<String?>(null, "2"), seenPages.toList())
        assertEquals(2, gauges.size)
        assertEquals(2L, gauges.meta.totalCount)
        assertFalse(gauges.meta.truncated)

        client.close()
    }

    @Test
    fun listGaugesForbiddenThrowsForbiddenWithStatusCodeAndServerMessage() = runTest {
        val client = errorClient(
            HttpStatusCode.Forbidden,
            """{"error": "You are not authorized to view gauges for this account"}""",
        )

        val e = assertFailsWith<BasecampException.Forbidden> {
            client.forAccount(accountId).gauges.listGauges(ListGaugesOptions(bucketIds = "$projectId"))
        }
        assertEquals("You are not authorized to view gauges for this account", e.message)
        assertEquals(BasecampException.CODE_FORBIDDEN, e.code)
        assertEquals(403, e.httpStatus)
        assertEquals("req-gauge-001", e.requestId)
        assertFalse(e.retryable)

        client.close()
    }

    // =========================================================================
    // ListGaugeNeedles — GET /projects/{projectId}/gauge/needles.json
    // =========================================================================

    @Test
    fun listGaugeNeedlesGetsTheProjectScopedNeedlesPathAndDecodesANeedle() = runTest {
        var capturedRequest: HttpRequestData? = null

        val client = mockClient { request ->
            capturedRequest = request
            respond(
                content = "[${needleJson()}]",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("1"),
                ),
            )
        }

        val needles = client.forAccount(accountId).gauges.listGaugeNeedles(
            projectId,
            ListGaugeNeedlesOptions(),
        )

        assertEquals(HttpMethod.Get, capturedRequest!!.method)
        assertEquals("/$accountId/projects/$projectId/gauge/needles.json", capturedRequest!!.url.encodedPath)
        assertEquals(null, capturedRequest!!.url.parameters["page"])

        assertEquals(1, needles.size)
        val needle = needles[0].jsonObject
        assertEquals(needleId, needle["id"]!!.jsonPrimitive.content.toLong())
        assertEquals("Moved the needle", needle["title"]!!.jsonPrimitive.content)
        assertEquals("Gauge::Needle", needle["type"]!!.jsonPrimitive.content)
        assertEquals("green", needle["color"]!!.jsonPrimitive.content)
        assertEquals(JsonPrimitive(72), needle["position"])
        assertEquals(
            "https://3.basecampapi.com/$accountId/projects/$projectId/gauge/needles/$needleId.json",
            needle["url"]!!.jsonPrimitive.content,
        )
        assertEquals("Gauge", needle["parent"]!!.jsonObject["type"]!!.jsonPrimitive.content)

        assertEquals(1L, needles.meta.totalCount)
        assertFalse(needles.meta.truncated)

        client.close()
    }

    @Test
    fun listGaugeNeedlesPinnedPageIssuesOneRequestAndReportsUnfollowedTruncation() = runTest {
        var requestCount = 0
        val seenPages = mutableListOf<String?>()

        val client = mockClient { request ->
            requestCount++
            seenPages += request.url.parameters["page"]
            respond(
                content = "[${needleJson(id = 1069479851, position = 60)}]",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "Link" to listOf(
                        """<https://3.basecampapi.com/$accountId/projects/$projectId/gauge/needles.json?page=3>; rel="next"""",
                    ),
                    "X-Total-Count" to listOf("7"),
                ),
            )
        }

        val needles = client.forAccount(accountId).gauges.listGaugeNeedles(
            projectId,
            ListGaugeNeedlesOptions(page = 2),
        )

        assertEquals(1, requestCount)
        assertEquals(listOf<String?>("2"), seenPages.toList())
        assertEquals(1, needles.size)
        assertEquals(1069479851L, needles[0].jsonObject["id"]!!.jsonPrimitive.content.toLong())
        assertEquals(7L, needles.meta.totalCount)
        assertTrue(needles.meta.truncated, "an unfollowed next link is truncation")

        client.close()
    }

    @Test
    fun listGaugeNeedlesWithoutAPinnedPageWalksTheLinkChain() = runTest {
        val seenPages = mutableListOf<String?>()

        val client = mockClient { request ->
            seenPages += request.url.parameters["page"]
            when (request.url.parameters["page"]) {
                null -> respond(
                    content = "[${needleJson(id = 1069479852, position = 80)}]",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf(
                            """<https://3.basecampapi.com/$accountId/projects/$projectId/gauge/needles.json?page=2>; rel="next"""",
                        ),
                        "X-Total-Count" to listOf("2"),
                    ),
                )
                else -> respond(
                    content = "[${needleJson(id = 1069479851, position = 60)}]",
                    status = HttpStatusCode.OK,
                    headers = headersOf(HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString())),
                )
            }
        }

        val needles = client.forAccount(accountId).gauges.listGaugeNeedles(projectId, ListGaugeNeedlesOptions())

        assertEquals(listOf<String?>(null, "2"), seenPages.toList())
        assertEquals(2, needles.size)
        // Newest first, and the chain is walked in order.
        assertEquals(1069479852L, needles[0].jsonObject["id"]!!.jsonPrimitive.content.toLong())
        assertEquals(1069479851L, needles[1].jsonObject["id"]!!.jsonPrimitive.content.toLong())
        assertFalse(needles.meta.truncated)

        client.close()
    }

    @Test
    fun listGaugeNeedlesNotFoundThrowsNotFoundWithStatusCodeAndServerMessage() = runTest {
        val client = errorClient(HttpStatusCode.NotFound, """{"error": "Project not found"}""")

        val e = assertFailsWith<BasecampException.NotFound> {
            client.forAccount(accountId).gauges.listGaugeNeedles(999, ListGaugeNeedlesOptions())
        }
        assertEquals("Project not found", e.message)
        assertEquals(BasecampException.CODE_NOT_FOUND, e.code)
        assertEquals(404, e.httpStatus)
        assertEquals("req-gauge-001", e.requestId)
        assertFalse(e.retryable)

        client.close()
    }

    // =========================================================================
    // GetGaugeNeedle — GET /gauge_needles/{needleId}  (no .json suffix)
    // =========================================================================

    @Test
    fun gaugeNeedleGetsTheExtensionlessNeedlePath() = runTest {
        var capturedRequest: HttpRequestData? = null

        val client = mockClient { request ->
            capturedRequest = request
            respond(
                content = needleJson(),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val needle = client.forAccount(accountId).gauges.gaugeNeedle(needleId).jsonObject

        assertEquals(HttpMethod.Get, capturedRequest!!.method)
        val path = capturedRequest!!.url.encodedPath
        assertEquals("/$accountId/gauge_needles/$needleId", path)
        // Deliberate in the spec: this route carries no format suffix. Asserted
        // separately so adding ".json" back cannot pass on a prefix match.
        assertFalse(path.endsWith(".json"), "GetGaugeNeedle takes no .json suffix; got $path")

        assertEquals(needleId, needle["id"]!!.jsonPrimitive.content.toLong())
        assertEquals("Moved the needle", needle["title"]!!.jsonPrimitive.content)
        assertEquals("green", needle["color"]!!.jsonPrimitive.content)
        assertEquals(JsonPrimitive(72), needle["position"])

        client.close()
    }

    @Test
    fun gaugeNeedleNotFoundThrowsNotFoundWithStatusCodeAndServerMessage() = runTest {
        val client = errorClient(HttpStatusCode.NotFound, """{"error": "Needle not found"}""")

        val e = assertFailsWith<BasecampException.NotFound> {
            client.forAccount(accountId).gauges.gaugeNeedle(999)
        }
        assertEquals("Needle not found", e.message)
        assertEquals(BasecampException.CODE_NOT_FOUND, e.code)
        assertEquals(404, e.httpStatus)
        assertEquals("req-gauge-001", e.requestId)
        assertFalse(e.retryable)

        client.close()
    }

    // =========================================================================
    // CreateGaugeNeedle — POST /projects/{projectId}/gauge/needles.json
    // =========================================================================

    @Test
    fun createGaugeNeedlePostsTheWrappedBodyAndOmitsNotifyAndSubscriptions() = runTest {
        var capturedRequest: HttpRequestData? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedRequest = request
            capturedBody = request.body.toByteArray().decodeToString()
            respond(
                content = needleJson(),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val needle = client.forAccount(accountId).gauges.createGaugeNeedle(
            projectId,
            CreateGaugeNeedleBody(
                gaugeNeedle = buildJsonObject {
                    put("color", "green")
                    put("position", 72)
                    put("description", "<div>Moved the needle</div>")
                },
            ),
        ).jsonObject

        assertEquals(HttpMethod.Post, capturedRequest!!.method)
        assertEquals("/$accountId/projects/$projectId/gauge/needles.json", capturedRequest!!.url.encodedPath)

        val body = json.parseToJsonElement(capturedBody!!).jsonObject
        // Key set first, so a renamed or extra envelope key reports itself
        // rather than exploding on a null lookup below. Doubles as the
        // omitted-notify/subscriptions assertion: only ever `put` when
        // non-null, so both must be absent from the wire body, not
        // present-and-null.
        assertEquals(setOf("gauge_needle"), body.keys)
        assertFalse(body.containsKey("notify"))
        assertFalse(body.containsKey("subscriptions"))

        val wrapped = body["gauge_needle"]!!.jsonObject
        assertEquals("green", wrapped["color"]!!.jsonPrimitive.content)
        assertEquals(JsonPrimitive(72), wrapped["position"])
        assertEquals("<div>Moved the needle</div>", wrapped["description"]!!.jsonPrimitive.content)

        assertEquals(needleId, needle["id"]!!.jsonPrimitive.content.toLong())

        client.close()
    }

    @Test
    fun createGaugeNeedleSendsNotifyAndSubscriptionsWhenGiven() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedBody = request.body.toByteArray().decodeToString()
            respond(
                content = needleJson(),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        client.forAccount(accountId).gauges.createGaugeNeedle(
            projectId,
            CreateGaugeNeedleBody(
                gaugeNeedle = buildJsonObject {
                    put("color", "yellow")
                    put("position", 40)
                },
                notify = "custom",
                subscriptions = listOf(1049715915L, 1049715916L),
            ),
        )

        val body = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals(setOf("gauge_needle", "notify", "subscriptions"), body.keys)
        assertEquals("yellow", body["gauge_needle"]!!.jsonObject["color"]!!.jsonPrimitive.content)
        assertEquals(JsonPrimitive("custom"), body["notify"])
        assertEquals(
            JsonArray(listOf(JsonPrimitive(1049715915L), JsonPrimitive(1049715916L))),
            body["subscriptions"],
        )

        client.close()
    }

    @Test
    fun createGaugeNeedleValidationErrorSurfacesFieldErrors() = runTest {
        val client = errorClient(
            HttpStatusCode.UnprocessableEntity,
            """{"errors": {"position": ["is not included in the list"], "color": ["is not a valid color"]}}""",
        )

        val e = assertFailsWith<BasecampException.Validation> {
            client.forAccount(accountId).gauges.createGaugeNeedle(
                projectId,
                CreateGaugeNeedleBody(
                    gaugeNeedle = buildJsonObject {
                        put("color", "chartreuse")
                        put("position", 150)
                    },
                ),
            )
        }
        assertEquals(
            "color: is not a valid color, position: is not included in the list",
            e.message,
        )
        assertEquals(
            mapOf(
                "color" to listOf("is not a valid color"),
                "position" to listOf("is not included in the list"),
            ),
            e.fieldErrors,
        )
        assertEquals(BasecampException.CODE_VALIDATION, e.code)
        assertEquals(422, e.httpStatus)
        assertEquals("req-gauge-001", e.requestId)
        assertFalse(e.retryable)

        client.close()
    }

    // =========================================================================
    // UpdateGaugeNeedle — PUT /gauge_needles/{needleId}  (no .json suffix)
    // =========================================================================

    @Test
    fun updateGaugeNeedlePutsTheDescriptionToTheExtensionlessNeedlePath() = runTest {
        var capturedRequest: HttpRequestData? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedRequest = request
            capturedBody = request.body.toByteArray().decodeToString()
            respond(
                content = needleJson(),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val needle = client.forAccount(accountId).gauges.updateGaugeNeedle(
            needleId,
            UpdateGaugeNeedleBody(
                gaugeNeedle = buildJsonObject { put("description", "<div>Revised note</div>") },
            ),
        ).jsonObject

        assertEquals(HttpMethod.Put, capturedRequest!!.method)
        val path = capturedRequest!!.url.encodedPath
        assertEquals("/$accountId/gauge_needles/$needleId", path)
        assertFalse(path.endsWith(".json"), "UpdateGaugeNeedle takes no .json suffix; got $path")

        val body = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals(setOf("gauge_needle"), body.keys)
        val wrapped = body["gauge_needle"]!!.jsonObject
        // Position and color are immutable; only description is sent.
        assertEquals(setOf("description"), wrapped.keys)
        assertEquals("<div>Revised note</div>", wrapped["description"]!!.jsonPrimitive.content)

        assertEquals(needleId, needle["id"]!!.jsonPrimitive.content.toLong())

        client.close()
    }

    @Test
    fun updateGaugeNeedleOmitsTheWrapperEntirelyWhenNoAttributesAreGiven() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedBody = request.body.toByteArray().decodeToString()
            respond(
                content = needleJson(),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        client.forAccount(accountId).gauges.updateGaugeNeedle(needleId, UpdateGaugeNeedleBody())

        val body = json.parseToJsonElement(capturedBody!!).jsonObject
        assertTrue(body.isEmpty(), "a null gaugeNeedle sends no wrapper at all; got $capturedBody")

        client.close()
    }

    @Test
    fun updateGaugeNeedleNotFoundThrowsNotFoundWithStatusCodeAndServerMessage() = runTest {
        val client = errorClient(HttpStatusCode.NotFound, """{"error": "Needle not found"}""")

        val e = assertFailsWith<BasecampException.NotFound> {
            client.forAccount(accountId).gauges.updateGaugeNeedle(
                999,
                UpdateGaugeNeedleBody(gaugeNeedle = buildJsonObject { put("description", "x") }),
            )
        }
        assertEquals("Needle not found", e.message)
        assertEquals(BasecampException.CODE_NOT_FOUND, e.code)
        assertEquals(404, e.httpStatus)
        assertEquals("req-gauge-001", e.requestId)
        assertFalse(e.retryable)

        client.close()
    }

    // =========================================================================
    // DestroyGaugeNeedle — DELETE /gauge_needles/{needleId}  (no .json suffix)
    // =========================================================================

    @Test
    fun destroyGaugeNeedleDeletesTheExtensionlessNeedlePathAndAnswersNoContent() = runTest {
        var capturedRequest: HttpRequestData? = null

        val client = mockClient { request ->
            capturedRequest = request
            respond(content = "", status = HttpStatusCode.NoContent)
        }

        client.forAccount(accountId).gauges.destroyGaugeNeedle(needleId)

        assertEquals(HttpMethod.Delete, capturedRequest!!.method)
        val path = capturedRequest!!.url.encodedPath
        assertEquals("/$accountId/gauge_needles/$needleId", path)
        assertFalse(path.endsWith(".json"), "DestroyGaugeNeedle takes no .json suffix; got $path")

        client.close()
    }

    @Test
    fun destroyGaugeNeedleForbiddenThrowsForbiddenWithStatusCodeAndServerMessage() = runTest {
        val client = errorClient(
            HttpStatusCode.Forbidden,
            """{"error": "Only the creator or a project admin can delete a needle"}""",
        )

        val e = assertFailsWith<BasecampException.Forbidden> {
            client.forAccount(accountId).gauges.destroyGaugeNeedle(needleId)
        }
        assertEquals("Only the creator or a project admin can delete a needle", e.message)
        assertEquals(BasecampException.CODE_FORBIDDEN, e.code)
        assertEquals(403, e.httpStatus)
        assertEquals("req-gauge-001", e.requestId)
        assertFalse(e.retryable)

        client.close()
    }

    // =========================================================================
    // ToggleGauge — PUT /projects/{projectId}/gauge.json
    //
    // bc3 answers `head :ok`: HTTP 200 with an EMPTY body, not 204. The stubs
    // below respond exactly that, so the void parse is exercised against the
    // response the API really sends.
    // =========================================================================

    @Test
    fun toggleGaugePutsTheEnabledFlagToTheProjectScopedGaugePath() = runTest {
        var capturedRequest: HttpRequestData? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedRequest = request
            capturedBody = request.body.toByteArray().decodeToString()
            respond(content = "", status = HttpStatusCode.OK)
        }

        client.forAccount(accountId).gauges.toggleGauge(
            projectId,
            ToggleGaugeBody(gauge = buildJsonObject { put("enabled", true) }),
        )

        assertEquals(HttpMethod.Put, capturedRequest!!.method)
        // Singleton route: /gauge.json, not /gauges.json and not /buckets/...
        assertEquals("/$accountId/projects/$projectId/gauge.json", capturedRequest!!.url.encodedPath)

        val body = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals(setOf("gauge"), body.keys)
        assertEquals(JsonPrimitive(true), body["gauge"]!!.jsonObject["enabled"])

        client.close()
    }

    @Test
    fun toggleGaugeSendsAnExplicitFalseToDisable() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedBody = request.body.toByteArray().decodeToString()
            respond(content = "", status = HttpStatusCode.OK)
        }

        client.forAccount(accountId).gauges.toggleGauge(
            projectId,
            ToggleGaugeBody(gauge = buildJsonObject { put("enabled", false) }),
        )

        val body = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals(JsonPrimitive(false), body["gauge"]!!.jsonObject["enabled"])

        client.close()
    }

    @Test
    fun toggleGaugeForbiddenThrowsForbiddenWithStatusCodeAndServerMessage() = runTest {
        val client = errorClient(
            HttpStatusCode.Forbidden,
            """{"error": "Only project admins can toggle the gauge"}""",
        )

        val e = assertFailsWith<BasecampException.Forbidden> {
            client.forAccount(accountId).gauges.toggleGauge(
                projectId,
                ToggleGaugeBody(gauge = buildJsonObject { put("enabled", true) }),
            )
        }
        assertEquals("Only project admins can toggle the gauge", e.message)
        assertEquals(BasecampException.CODE_FORBIDDEN, e.code)
        assertEquals(403, e.httpStatus)
        assertEquals("req-gauge-001", e.requestId)
        assertFalse(e.retryable)

        client.close()
    }
}
