package com.basecamp.sdk

import com.basecamp.sdk.generated.pings
import com.basecamp.sdk.generated.services.CreatePingBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.jsonObject
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * The three Pings operations. Bodies are inline literals with the keys
 * and values of `spec/fixtures/pings/get.json` and `list.json` (the `people`
 * lists are trimmed): `commonTest` has no filesystem. `PingsFixtureTest` in
 * `jvmTest` reads the shared files themselves.
 *
 * The service returns the typed [com.basecamp.sdk.generated.models.Ping].
 */
class PingsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient =
        testBasecampClient {
            accessToken("test-token")
            engine = MockEngine(handler)
        }

    private fun jsonHeaders() = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString())

    private fun pingJson(id: Long, title: String, named: Boolean, campfireId: Long) = """{
        "id": $id,
        "created_at": "2022-11-21T16:30:12.231Z",
        "updated_at": "2022-11-22T09:12:44.118Z",
        "title": "$title",
        "named": $named,
        "avatar_url": "https://3.basecamp-static.com/195539477/circles/$id/avatar",
        "url": "https://3.basecampapi.com/195539477/circles/$id.json",
        "app_url": "https://3.basecamp.com/195539477/circles/$id",
        "bookmark_url": "https://3.basecampapi.com/195539477/my/bookmarks/BAh7--ffeeddcc.json",
        "room_url": "https://3.basecampapi.com/195539477/buckets/$id/chats/$campfireId.json",
        "links": {
            "self": "https://3.basecampapi.com/195539477/circles/$id.json",
            "lines": "https://3.basecampapi.com/195539477/buckets/$id/chats/$campfireId/lines.json",
            "files": "https://3.basecampapi.com/195539477/buckets/$id/chats/$campfireId/uploads.json"
        },
        "people": [{"id": 1049715915, "name": "Annie Bryan"}]
    }"""

    @Test
    fun listReadsTheCallersPings() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Get, request.method)
            assertEquals("/195539477/circles.json", request.url.encodedPath)
            assertEquals("Bearer test-token", request.headers["Authorization"])
            respond(
                content = "[${pingJson(2085958600, "Launch crew", true, 1069479600)}," +
                    "${pingJson(2085958601, "Cheryl Walters", false, 1069479601)}]",
                status = HttpStatusCode.OK,
                headers = jsonHeaders(),
            )
        }

        val pings = client.forAccount("195539477").pings.list()

        assertEquals(2, pings.size)
        val group = pings[0]
        assertEquals(2085958600L, group.id)
        assertEquals("Launch crew", group.title)
        assertTrue(group.named)
        assertEquals(1, group.people.size)
        assertEquals("Annie Bryan", group.people[0].name)
        assertEquals("Cheryl Walters", pings[1].title)
        assertFalse(pings[1].named)
        assertEquals(
            "https://3.basecampapi.com/195539477/buckets/2085958601/chats/1069479601/lines.json",
            pings[1].links.lines,
        )
        client.close()
    }

    @Test
    fun getReadsAPingByItsBucketId() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Get, request.method)
            assertEquals("/195539477/circles/2085958600", request.url.encodedPath)
            respond(
                content = pingJson(2085958600, "Launch crew", true, 1069479600),
                status = HttpStatusCode.OK,
                headers = jsonHeaders(),
            )
        }

        val ping = client.forAccount("195539477").pings.get(2085958600)

        assertEquals(
            "https://3.basecampapi.com/195539477/buckets/2085958600/chats/1069479600.json",
            ping.roomUrl,
        )
        client.close()
    }

    @Test
    fun getMapsNotAPingToNotFoundAndDisabledPingsToForbidden() = runTest {
        for ((status, body) in listOf(
            HttpStatusCode.NotFound to """{"error":"Record not found"}""",
            HttpStatusCode.Forbidden to """{"error":"Pings have been disabled"}""",
        )) {
            val client = mockClient { respond(content = body, status = status, headers = jsonHeaders()) }
            val pings = client.forAccount("195539477").pings
            if (status == HttpStatusCode.NotFound) {
                val e = assertFailsWith<BasecampException.NotFound> { pings.get(1) }
                assertEquals(404, e.httpStatus)
            } else {
                val e = assertFailsWith<BasecampException.Forbidden> { pings.get(1) }
                assertEquals(403, e.httpStatus)
                assertTrue("Pings have been disabled" in e.message.orEmpty(), "the server's message is kept: ${e.message}")
            }
            client.close()
        }
    }

    @Test
    fun listMapsDisabledPingsToForbidden() = runTest {
        val client = mockClient {
            respond(
                content = """{"error":"Pings have been disabled"}""",
                status = HttpStatusCode.Forbidden,
                headers = jsonHeaders(),
            )
        }
        assertFailsWith<BasecampException.Forbidden> { client.forAccount("195539477").pings.list() }
        client.close()
    }

    @Test
    fun createPostsTheUserIdsAndAnswersWithThePing() = runTest {
        var captured: String? = null
        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertEquals("/195539477/circles.json", request.url.encodedPath)
            captured = request.body.toByteArray().decodeToString()
            respond(
                content = pingJson(2085958600, "Launch crew", true, 1069479600),
                status = HttpStatusCode.Created,
                headers = jsonHeaders(),
            )
        }

        val ping = client.forAccount("195539477").pings.create(
            CreatePingBody(userIds = listOf(1049715915L, 1049715916L), newGroup = true),
        )

        val body = json.parseToJsonElement(captured!!).jsonObject
        assertEquals(
            JsonArray(listOf(JsonPrimitive(1049715915L), JsonPrimitive(1049715916L))),
            body["user_ids"],
        )
        assertEquals(JsonPrimitive(true), body["new_group"])
        assertEquals(2085958600L, ping.id)
        client.close()
    }

    @Test
    fun createOmitsNewGroupUnlessSet() = runTest {
        var captured: String? = null
        val client = mockClient { request ->
            captured = request.body.toByteArray().decodeToString()
            respond(
                content = pingJson(2085958601, "Cheryl Walters", false, 1069479601),
                status = HttpStatusCode.Created,
                headers = jsonHeaders(),
            )
        }

        client.forAccount("195539477").pings.create(CreatePingBody(userIds = listOf(1049715915L)))

        val body: JsonElement = json.parseToJsonElement(captured!!)
        assertNotNull(body.jsonObject["user_ids"])
        assertFalse("new_group" in body.jsonObject, "an unset new_group is not sent: $captured")
        client.close()
    }

    @Test
    fun createMapsDisabledPingsToForbiddenAndNobodyPingableToUnprocessable() = runTest {
        var status = HttpStatusCode.Forbidden
        var body = """{"error":"Pings have been disabled"}"""
        val client = mockClient { respond(content = body, status = status, headers = jsonHeaders()) }
        val pings = client.forAccount("195539477").pings

        assertFailsWith<BasecampException.Forbidden> { pings.create(CreatePingBody(userIds = listOf(1L))) }

        status = HttpStatusCode.UnprocessableEntity
        body = ""
        val e = assertFailsWith<BasecampException.Validation> { pings.create(CreatePingBody(userIds = listOf(1L))) }
        assertEquals(422, e.httpStatus)
        client.close()
    }
}
