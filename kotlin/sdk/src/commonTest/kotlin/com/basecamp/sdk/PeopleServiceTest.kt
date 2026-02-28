package com.basecamp.sdk

import com.basecamp.sdk.generated.people
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class PeopleServiceTest {

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return BasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    private fun personJson(id: Long, name: String, email: String = "test@example.com", admin: Boolean = false) = """{
        "id": $id, "name": "$name",
        "email_address": "$email",
        "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
        "admin": $admin, "owner": false, "client": false, "employee": false,
        "can_manage_projects": false, "can_manage_people": false,
        "can_ping": true, "can_access_timesheet": false, "can_access_hill_charts": false
    }"""

    @Test
    fun listPeople() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/people.json"))
            assertEquals("Bearer test-token", request.headers["Authorization"])

            respond(
                content = """[
                    ${personJson(1, "Alice")},
                    ${personJson(2, "Bob", admin = true)}
                ]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val people = account.people.list()

        assertEquals(2, people.size)
        assertEquals(1L, people[0].id)
        assertEquals("Alice", people[0].name)
        assertEquals(false, people[0].admin)
        assertEquals(2L, people[1].id)
        assertEquals("Bob", people[1].name)
        assertEquals(true, people[1].admin)

        client.close()
    }

    @Test
    fun getPerson() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/people/42"))

            respond(
                content = personJson(42, "Charlie", "charlie@example.com"),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val person = account.people.get(personId = 42)

        assertEquals(42L, person.id)
        assertEquals("Charlie", person.name)
        assertEquals("charlie@example.com", person.emailAddress)

        client.close()
    }

    @Test
    fun getMyProfile() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/my/profile.json"))

            respond(
                content = personJson(1, "Me", "me@example.com", admin = true),
                status = HttpStatusCode.OK,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        val me = account.people.me()

        assertEquals(1L, me.id)
        assertEquals("Me", me.name)
        assertEquals(true, me.admin)

        client.close()
    }

    @Test
    fun listPingablePeople() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/circles/people.json"))

            respond(
                content = """[${personJson(1, "Alice")}]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("1"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val people = account.people.listPingable()

        assertEquals(1, people.size)
        assertEquals("Alice", people[0].name)
        assertEquals(true, people[0].canPing)

        client.close()
    }

    @Test
    fun listPeopleForProject() = runTest {
        val client = mockClient { request ->
            assertTrue(request.url.encodedPath.contains("/projects/5/people.json"))

            respond(
                content = """[
                    ${personJson(1, "Alice")},
                    ${personJson(2, "Bob")}
                ]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        val people = account.people.listForProject(projectId = 5)

        assertEquals(2, people.size)
        assertEquals(1L, people[0].id)
        assertEquals(2L, people[1].id)

        client.close()
    }

    @Test
    fun personNotFoundThrows() = runTest {
        val client = mockClient { _ ->
            respond(
                content = """{"error": "Not found"}""",
                status = HttpStatusCode.NotFound,
                headers = headersOf(HttpHeaders.ContentType, ContentType.Application.Json.toString()),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.people.get(personId = 999)
            assertTrue(false, "Should have thrown")
        } catch (e: BasecampException.NotFound) {
            assertEquals("Not found", e.message)
        }

        client.close()
    }

    @Test
    fun listPeoplePaginated() = runTest {
        val client = mockClient { request ->
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1

            when (page) {
                1 -> respond(
                    content = """[${personJson(1, "Alice")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/people.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("2"),
                    ),
                )
                else -> respond(
                    content = """[${personJson(2, "Bob")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    ),
                )
            }
        }

        val account = client.forAccount("12345")
        val people = account.people.list()

        assertEquals(2, people.size)
        assertEquals("Alice", people[0].name)
        assertEquals("Bob", people[1].name)

        client.close()
    }
}
