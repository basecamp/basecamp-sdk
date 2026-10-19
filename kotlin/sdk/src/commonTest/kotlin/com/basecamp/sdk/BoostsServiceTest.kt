package com.basecamp.sdk

import com.basecamp.sdk.generated.boosts
import com.basecamp.sdk.generated.services.UpdateBoostBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.jsonObject
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

/** Updating a boost: the creator-only replacement of its content. */
class BoostsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient =
        testBasecampClient {
            accessToken("test-token")
            engine = MockEngine(handler)
        }

    private fun jsonHeaders() = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString())

    private fun boostJson(id: Long, content: String) = """{
        "id": $id,
        "content": "$content",
        "created_at": "2022-11-01T10:00:00.000Z",
        "booster": {"id": 1049715914, "name": "Victor Cooper"}
    }"""

    @Test
    fun updatePutsContentAloneAndAnswersWithTheBoost() = runTest {
        var captured: String? = null
        val client = mockClient { request ->
            assertEquals(HttpMethod.Put, request.method)
            assertEquals("/195539477/boosts/1069479500", request.url.encodedPath)
            assertEquals("Bearer test-token", request.headers["Authorization"])
            captured = request.body.toByteArray().decodeToString()
            respond(
                content = boostJson(1069479500, "Done!"),
                status = HttpStatusCode.OK,
                headers = jsonHeaders(),
            )
        }

        val boost = client.forAccount("195539477").boosts.update(1069479500L, UpdateBoostBody(content = "Done!"))

        val body = json.parseToJsonElement(captured!!).jsonObject
        assertEquals(mapOf("content" to JsonPrimitive("Done!")), body.toMap())
        assertEquals(1069479500L, boost.id)
        assertEquals("Done!", boost.content)
        client.close()
    }

    @Test
    fun updateMapsForbiddenNotFoundAndUnprocessable() = runTest {
        var status = HttpStatusCode.Forbidden
        val client = mockClient { respond(content = "", status = status, headers = jsonHeaders()) }
        val boosts = client.forAccount("195539477").boosts
        val update = UpdateBoostBody(content = "Done!")

        assertFailsWith<BasecampException.Forbidden> { boosts.update(1L, update) }

        status = HttpStatusCode.NotFound
        assertFailsWith<BasecampException.NotFound> { boosts.update(1L, update) }

        status = HttpStatusCode.UnprocessableEntity
        val e = assertFailsWith<BasecampException.Validation> { boosts.update(1L, update) }
        assertEquals(422, e.httpStatus)
        client.close()
    }
}
