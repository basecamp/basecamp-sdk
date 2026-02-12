package com.basecamp.sdk

import com.basecamp.sdk.generated.services.CreateTodoBody
import com.basecamp.sdk.generated.todos
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class TodosServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return BasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    @Test
    fun listTodos() = runTest {
        val client = mockClient { request ->
            val path = request.url.encodedPath
            assertTrue(path.contains("/buckets/1/todolists/2/todos.json"), "Path: $path")
            assertEquals("Bearer test-token", request.headers["Authorization"])

            respond(
                content = """[
                    {"id": 100, "content": "Buy milk", "completed": false},
                    {"id": 101, "content": "Walk dog", "completed": true}
                ]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val todos = account.todos.list(projectId = 1, todolistId = 2)

        assertEquals(2, todos.size)
        assertEquals(100L, todos[0].id)
        assertEquals("Buy milk", todos[0].content)
        assertEquals(false, todos[0].completed)
        assertEquals(101L, todos[1].id)
        assertEquals(true, todos[1].completed)
        assertEquals(2L, todos.meta.totalCount)

        client.close()
    }

    @Test
    fun getTodo() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/buckets/1/todos/100"))

            respond(
                content = """{"id": 100, "content": "Buy milk", "completed": false, "position": 1}""",
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val todo = account.todos.get(projectId = 1, todoId = 100)

        assertEquals(100L, todo.id)
        assertEquals("Buy milk", todo.content)
        assertEquals(1, todo.position)

        client.close()
    }

    @Test
    fun createTodo() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/buckets/1/todolists/2/todos.json"))
            assertEquals(HttpMethod.Post, request.method)
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = """{"id": 200, "content": "New todo", "completed": false}""",
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val todo = account.todos.create(
            projectId = 1,
            todolistId = 2,
            body = CreateTodoBody(content = "New todo", description = "Details here"),
        )

        assertEquals(200L, todo.id)
        assertEquals("New todo", todo.content)

        // Verify the request body was properly serialized
        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("New todo", bodyJson["content"]!!.jsonPrimitive.content)
        assertEquals("Details here", bodyJson["description"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun completeTodo() = runTest {
        var capturedMethod: HttpMethod? = null
        var capturedPath: String? = null

        val client = mockClient { request ->
            capturedMethod = request.method
            capturedPath = request.url.encodedPath

            respond(
                content = "",
                status = HttpStatusCode.NoContent,
            )
        }

        val account = client.forAccount("12345")
        account.todos.complete(projectId = 1, todoId = 100)

        assertEquals(HttpMethod.Post, capturedMethod)
        assertTrue(capturedPath!!.contains("/buckets/1/todos/100/completion.json"))

        client.close()
    }

    @Test
    fun trashTodoReturnsUnit() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Delete, request.method)
            assertTrue(request.url.encodedPath.contains("/buckets/1/todos/100"))

            respond(
                content = "",
                status = HttpStatusCode.NoContent,
            )
        }

        val account = client.forAccount("12345")
        account.todos.trash(projectId = 1, todoId = 100)
        // No exception = success

        client.close()
    }

    @Test
    fun todoNotFoundThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Not found"}""",
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.todos.get(projectId = 1, todoId = 999)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.NotFound) {
            assertEquals("Not found", e.message)
        }

        client.close()
    }

    @Test
    fun listTodosPaginated() = runTest {
        var requestCount = 0

        val client = mockClient { request ->
            requestCount++
            val page = if (request.url.parameters["page"] == "2") 2 else 1

            when (page) {
                1 -> respond(
                    content = """[{"id": 1, "content": "Todo 1"}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/buckets/1/todolists/2/todos.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("2"),
                    ),
                )
                else -> respond(
                    content = """[{"id": 2, "content": "Todo 2"}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    ),
                )
            }
        }

        val account = client.forAccount("12345")
        val todos = account.todos.list(projectId = 1, todolistId = 2)

        assertEquals(2, todos.size)
        assertEquals(1L, todos[0].id)
        assertEquals(2L, todos[1].id)
        assertEquals(2L, todos.meta.totalCount)
        assertEquals(false, todos.meta.truncated)

        client.close()
    }
}
