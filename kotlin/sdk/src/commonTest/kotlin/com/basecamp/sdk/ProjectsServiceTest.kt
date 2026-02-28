package com.basecamp.sdk

import com.basecamp.sdk.generated.projects
import com.basecamp.sdk.generated.services.CreateProjectBody
import com.basecamp.sdk.generated.services.ListProjectsOptions
import com.basecamp.sdk.generated.services.UpdateProjectBody
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class ProjectsServiceTest {

    private val json = Json { ignoreUnknownKeys = true }

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return BasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private fun projectJson(id: Long, name: String, description: String? = null) = """{
        "id": $id, "status": "active", "name": "$name",
        "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
        "url": "https://3.basecampapi.com/12345/projects/$id.json",
        "app_url": "https://3.basecamp.com/12345/projects/$id",
        "dock": []
        ${if (description != null) """, "description": "$description"""" else ""}
    }"""

    @Test
    fun listProjects() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/projects.json"))
            assertEquals("Bearer test-token", request.headers["Authorization"])

            respond(
                content = """[
                    ${projectJson(1, "Project Alpha")},
                    ${projectJson(2, "Project Beta")}
                ]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val projects = account.projects.list()

        assertEquals(2, projects.size)
        assertEquals(1L, projects[0].id)
        assertEquals("Project Alpha", projects[0].name)
        assertEquals(2L, projects[1].id)
        assertEquals("Project Beta", projects[1].name)
        assertEquals(2L, projects.meta.totalCount)

        client.close()
    }

    @Test
    fun listProjectsWithStatusFilter() = runTest {
        var capturedUrl: String? = null

        val client = mockClient { request ->
            capturedUrl = request.url.toString()
            respond(
                content = """[]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("0"),
                ),
            )
        }

        val account = client.forAccount("12345")
        account.projects.list(options = ListProjectsOptions(status = "archived"))

        assertTrue(capturedUrl!!.contains("status=archived"), "URL should contain status filter: $capturedUrl")
        client.close()
    }

    @Test
    fun getProject() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/projects/42"))

            respond(
                content = projectJson(42, "My Project", "A test project"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val project = account.projects.get(projectId = 42)

        assertEquals(42L, project.id)
        assertEquals("My Project", project.name)
        assertEquals("A test project", project.description)

        client.close()
    }

    @Test
    fun createProject() = runTest {
        var capturedBody: String? = null

        val client = mockClient { request ->
            assertEquals(HttpMethod.Post, request.method)
            assertTrue(request.url.encodedPath.contains("/projects.json"))
            capturedBody = request.body.toByteArray().decodeToString()

            respond(
                content = projectJson(99, "New Project"),
                status = HttpStatusCode.Created,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val project = account.projects.create(
            body = CreateProjectBody(name = "New Project", description = "Created via SDK"),
        )

        assertEquals(99L, project.id)
        assertEquals("New Project", project.name)

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("New Project", bodyJson["name"]!!.jsonPrimitive.content)
        assertEquals("Created via SDK", bodyJson["description"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun updateProject() = runTest {
        var capturedMethod: HttpMethod? = null
        var capturedBody: String? = null

        val client = mockClient { request ->
            capturedMethod = request.method
            capturedBody = request.body.toByteArray().decodeToString()
            assertTrue(request.url.encodedPath.contains("/projects/42"))

            respond(
                content = projectJson(42, "Updated Name"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val project = account.projects.update(
            projectId = 42,
            body = UpdateProjectBody(name = "Updated Name"),
        )

        assertEquals(HttpMethod.Put, capturedMethod)
        assertEquals(42L, project.id)
        assertEquals("Updated Name", project.name)

        val bodyJson = json.parseToJsonElement(capturedBody!!).jsonObject
        assertEquals("Updated Name", bodyJson["name"]!!.jsonPrimitive.content)

        client.close()
    }

    @Test
    fun trashProject() = runTest {
        var capturedMethod: HttpMethod? = null

        val client = mockClient { request ->
            capturedMethod = request.method
            assertTrue(request.url.encodedPath.contains("/projects/42"))

            respond(
                content = "",
                status = HttpStatusCode.NoContent,
            )
        }

        val account = client.forAccount("12345")
        account.projects.trash(projectId = 42)

        assertEquals(HttpMethod.Delete, capturedMethod)

        client.close()
    }

    @Test
    fun projectNotFoundThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Not found"}""",
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.projects.get(projectId = 999)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.NotFound) {
            assertEquals("Not found", e.message)
        }

        client.close()
    }

    @Test
    fun listProjectsPaginated() = runTest {
        val client = mockClient { request ->
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1

            when (page) {
                1 -> respond(
                    content = """[${projectJson(1, "P1")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("2"),
                    ),
                )
                else -> respond(
                    content = """[${projectJson(2, "P2")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    ),
                )
            }
        }

        val account = client.forAccount("12345")
        val projects = account.projects.list()

        assertEquals(2, projects.size)
        assertEquals(1L, projects[0].id)
        assertEquals(2L, projects[1].id)

        client.close()
    }
}
