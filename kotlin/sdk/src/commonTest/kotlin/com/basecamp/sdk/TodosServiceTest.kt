package com.basecamp.sdk

import com.basecamp.sdk.generated.services.CreateTodoBody
import com.basecamp.sdk.generated.services.ReplaceTodoBody
import com.basecamp.sdk.generated.services.UpdateTodoBody
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
        return testBasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private fun todoJson(id: Long, content: String, completed: Boolean = false, extras: String = "") = """{
        "id": $id, "content": "$content", "completed": $completed,
        "status": "active", "title": "$content", "type": "Todo",
        "visible_to_clients": false, "inherits_status": true,
        "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
        "url": "https://3.basecampapi.com/1/buckets/1/todos/$id.json",
        "app_url": "https://3.basecamp.com/1/buckets/1/todos/$id",
        "creator": {"id": 1, "name": "Test", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"},
        "bucket": {"id": 1, "name": "Project", "type": "Project"},
        "parent": {"id": 2, "title": "Todolist", "type": "Todolist", "url": "https://3.basecampapi.com/1/buckets/1/todolists/2.json", "app_url": "https://3.basecamp.com/1/buckets/1/todolists/2"}
        ${if (extras.isNotEmpty()) ", $extras" else ""}
    }"""

    @Test
    fun listTodos() = runTest {
        val client = mockClient { request ->
            val path = request.url.encodedPath
            assertTrue(path.contains("/todolists/2/todos.json"), "Path: $path")
            assertEquals("Bearer test-token", request.headers["Authorization"])

            respond(
                content = """[
                    ${todoJson(100, "Buy milk")},
                    ${todoJson(101, "Walk dog", completed = true)}
                ]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val todos = account.todos.list(todolistId = 2)

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
            assertTrue(request.url.encodedPath.contains("/todos/100"))

            respond(
                content = todoJson(100, "Buy milk", extras = """"position": 1"""),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val todo = account.todos.get(todoId = 100)

        assertEquals(100L, todo.id)
        assertEquals("Buy milk", todo.content)
        assertEquals(1, todo.position)

        client.close()
    }

    @Test
    fun createTodo() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/todolists/2/todos.json"))
            assertEquals(HttpMethod.Post, request.method)
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = todoJson(200, "New todo"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val todo = account.todos.create(
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
        account.todos.complete(todoId = 100)

        assertEquals(HttpMethod.Post, capturedMethod)
        assertTrue(capturedPath!!.contains("/todos/100/completion.json"))

        client.close()
    }

    @Test
    fun trashTodoReturnsUnit() = runTest {
        val client = mockClient { request ->
            assertEquals(HttpMethod.Delete, request.method)
            assertTrue(request.url.encodedPath.contains("/todos/100"))

            respond(
                content = "",
                status = HttpStatusCode.NoContent,
            )
        }

        val account = client.forAccount("12345")
        account.todos.trash(todoId = 100)
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
            account.todos.get(todoId = 999)
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
                    content = """[${todoJson(1, "Todo 1")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/todolists/2/todos.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("2"),
                    ),
                )
                else -> respond(
                    content = """[${todoJson(2, "Todo 2")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    ),
                )
            }
        }

        val account = client.forAccount("12345")
        val todos = account.todos.list(todolistId = 2)

        assertEquals(2, todos.size)
        assertEquals(1L, todos[0].id)
        assertEquals(2L, todos[1].id)
        assertEquals(2L, todos.meta.totalCount)
        assertEquals(false, todos.meta.truncated)

        client.close()
    }

    // -- Merge-safe update / edit / replace --

    private fun fullTodoJson(id: Long = 42) = todoJson(
        id, "Buy milk",
        extras = """
            "description": "<p>From the store</p>",
            "due_on": "2024-03-01",
            "starts_on": "2024-02-01",
            "assignees": [{"id": 100, "name": "Jane Doe", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"}],
            "completion_subscribers": [{"id": 555, "name": "Sub Scriber", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z"}]
        """.trimIndent(),
    )

    private class WriteCapture {
        val methods = mutableListOf<String>()
        var putBody: kotlinx.serialization.json.JsonObject? = null
    }

    private fun captureClient(capture: WriteCapture): BasecampClient = mockClient { request ->
        capture.methods.add(request.method.value)
        if (request.method == HttpMethod.Put) {
            capture.putBody = json.parseToJsonElement(
                (request.body as io.ktor.http.content.TextContent).text
            ).jsonObject
        }
        respond(
            content = fullTodoJson(),
            status = HttpStatusCode.OK,
            headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
        )
    }

    @Test
    fun updateMergesUnsetFields() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val todo = client.forAccount("12345").todos
            .update(42, UpdateTodoBody(content = "Updated task"))

        assertEquals(42L, todo.id)
        assertEquals(listOf("GET", "PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals("Updated task", body["content"]?.jsonPrimitive?.content)
        assertEquals("<p>From the store</p>", body["description"]?.jsonPrimitive?.content)
        assertEquals("2024-03-01", body["due_on"]?.jsonPrimitive?.content)
        assertEquals("2024-02-01", body["starts_on"]?.jsonPrimitive?.content)
        assertEquals(listOf(100L), body["assignee_ids"]?.jsonArray?.map { it.jsonPrimitive.content.toLong() })
        assertEquals(listOf(555L), body["completion_subscriber_ids"]?.jsonArray?.map { it.jsonPrimitive.content.toLong() })
        assertTrue("notify" !in body, "notify must be omitted unless true")

        client.close()
    }

    @Test
    fun updateExplicitEmptyListClears() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").todos.update(42, UpdateTodoBody(assigneeIds = emptyList()))

        val body = capture.putBody!!
        assertEquals(0, body["assignee_ids"]?.jsonArray?.size)
        assertEquals(1, body["completion_subscriber_ids"]?.jsonArray?.size)
        assertEquals("Buy milk", body["content"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun updateNotifyOnlyWhenTrue() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").todos.update(42, UpdateTodoBody(content = "ping", notify = true))

        assertEquals(true, capture.putBody!!["notify"]?.jsonPrimitive?.content?.toBoolean())

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
        val engine = MockEngine { request ->
            respond(
                content = fullTodoJson(),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }
        val client = testBasecampClient {
            accessToken("test-token")
            this.engine = engine
            this.hooks = hooks
        }

        client.forAccount("12345").todos.update(42, UpdateTodoBody(content = "observed"))

        assertEquals(listOf("GetTodo", "ReplaceTodo"), operations)

        client.close()
    }

    @Test
    fun editPutsFullStateBack() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val todo = client.forAccount("12345").todos.edit(42) {
            assertEquals("Buy milk", content)
            content = "🚨 $content"
        }

        assertEquals(42L, todo.id)
        val body = capture.putBody!!
        assertEquals("🚨 Buy milk", body["content"]?.jsonPrimitive?.content)
        assertEquals("<p>From the store</p>", body["description"]?.jsonPrimitive?.content)
        assertEquals(1, body["assignee_ids"]?.jsonArray?.size)

        client.close()
    }

    @Test
    fun editClearsDateByOmission() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").todos.edit(42) {
            assertEquals("2024-03-01", dueOn)
            dueOn = ""
        }

        val body = capture.putBody!!
        assertTrue("due_on" !in body, "cleared date must be omitted from the PUT body")
        assertEquals("Buy milk", body["content"]?.jsonPrimitive?.content)

        client.close()
    }

    @Test
    fun editClearsDescriptionAndIdsPresentAndEmpty() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        client.forAccount("12345").todos.edit(42) {
            description = ""
            assigneeIds = emptyList()
            completionSubscriberIds = emptyList()
        }

        val body = capture.putBody!!
        assertEquals("", body["description"]?.jsonPrimitive?.content)
        assertEquals(0, body["assignee_ids"]?.jsonArray?.size)
        assertEquals(0, body["completion_subscriber_ids"]?.jsonArray?.size)

        client.close()
    }

    @Test
    fun editBlockErrorAbortsWithoutPut() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        try {
            client.forAccount("12345").todos.edit(42) {
                content = "never written"
                error("abort")
            }
            kotlin.test.fail("expected the block error to propagate")
        } catch (e: IllegalStateException) {
            assertEquals("abort", e.message)
        }

        assertEquals(listOf("GET"), capture.methods)

        client.close()
    }

    @Test
    fun replaceSendsSparseVerbatimWithNoGet() = runTest {
        val capture = WriteCapture()
        val client = captureClient(capture)

        val todo = client.forAccount("12345").todos
            .replace(42, ReplaceTodoBody(content = "the whole new todo"))

        assertEquals(42L, todo.id)
        assertEquals(listOf("PUT"), capture.methods)
        val body = capture.putBody!!
        assertEquals("the whole new todo", body["content"]?.jsonPrimitive?.content)
        for (field in listOf("description", "assignee_ids", "completion_subscriber_ids", "notify", "due_on", "starts_on")) {
            assertTrue(field !in body, "$field must be omitted from a sparse replace")
        }

        client.close()
    }
}
