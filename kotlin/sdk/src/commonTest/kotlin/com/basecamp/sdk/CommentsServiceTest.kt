package com.basecamp.sdk

import com.basecamp.sdk.generated.comments
import com.basecamp.sdk.generated.services.CreateCommentBody
import com.basecamp.sdk.generated.services.UpdateCommentBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class CommentsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return BasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private fun commentJson(id: Long, content: String) = """{
        "id": $id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2025-01-01T00:00:00Z",
        "updated_at": "2025-01-01T00:00:00Z",
        "title": "Re: Test",
        "inherits_status": true,
        "type": "Comment",
        "url": "https://3.basecampapi.com/12345/buckets/1/comments/$id.json",
        "app_url": "https://3.basecamp.com/12345/buckets/1/comments/$id",
        "content": "$content",
        "parent": {"id": 100, "title": "Parent", "type": "Todo", "url": "https://3.basecampapi.com/12345/buckets/1/todos/100.json", "app_url": "https://3.basecamp.com/12345/buckets/1/todos/100"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "creator": {"id": 1, "name": "Test User", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"}
    }"""

    @Test
    fun listComments() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/recordings/100/comments.json"))
            assertEquals("Bearer test-token", request.headers["Authorization"])

            respond(
                content = """[
                    ${commentJson(10, "First comment")},
                    ${commentJson(11, "Second comment")}
                ]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val comments = account.comments.list(recordingId = 100)

        assertEquals(2, comments.size)
        assertEquals(10L, comments[0].id)
        assertEquals("First comment", comments[0].content)
        assertEquals(11L, comments[1].id)
        assertEquals("Second comment", comments[1].content)
        assertEquals(2L, comments.meta.totalCount)

        client.close()
    }

    @Test
    fun getComment() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/comments/10"))

            respond(
                content = commentJson(10, "A specific comment"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val comment = account.comments.get(commentId = 10)

        assertEquals(10L, comment.id)
        assertEquals("A specific comment", comment.content)
        assertEquals("Comment", comment.type)

        client.close()
    }

    @Test
    fun createComment() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertTrue(request.url.encodedPath.contains("/recordings/100/comments.json"))
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = commentJson(20, "New comment"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val comment = account.comments.create(
            recordingId = 100,
            body = CreateCommentBody(content = "New comment"),
        )

        assertEquals(20L, comment.id)
        assertEquals("New comment", comment.content)

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("New comment", bodyJson["content"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun updateComment() = runTest {
        var capturedMethod: HttpMethod? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedMethod = request.method
            capturedBody = request.body.toByteArray().decodeToString()
            assertTrue(request.url.encodedPath.contains("/comments/10"))

            respond(
                content = commentJson(10, "Updated comment"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val comment = account.comments.update(
            commentId = 10,
            body = UpdateCommentBody(content = "Updated comment"),
        )

        assertEquals(HttpMethod.Put, capturedMethod)
        assertEquals(10L, comment.id)
        assertEquals("Updated comment", comment.content)

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("Updated comment", bodyJson["content"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun commentNotFoundThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Not found"}""",
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.comments.get(commentId = 999)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.NotFound) {
            assertEquals("Not found", e.message)
        }

        client.close()
    }

    @Test
    fun listCommentsPaginated() = runTest {
        val client = mockClient { request ->
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1

            when (page) {
                1 -> respond(
                    content = """[${commentJson(1, "Comment 1")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/recordings/100/comments.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("2"),
                    ),
                )
                else -> respond(
                    content = """[${commentJson(2, "Comment 2")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    ),
                )
            }
        }

        val account = client.forAccount("12345")
        val comments = account.comments.list(recordingId = 100)

        assertEquals(2, comments.size)
        assertEquals("Comment 1", comments[0].content)
        assertEquals("Comment 2", comments[1].content)

        client.close()
    }

    @Test
    fun commentAuthErrorThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Authentication required"}""",
                status = HttpStatusCode.Unauthorized,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.comments.list(recordingId = 100)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.Auth) {
            assertEquals("Authentication required", e.message)
        }

        client.close()
    }
}
