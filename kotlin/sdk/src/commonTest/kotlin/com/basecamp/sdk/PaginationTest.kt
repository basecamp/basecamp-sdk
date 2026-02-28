package com.basecamp.sdk

import com.basecamp.sdk.generated.projects
import io.ktor.client.engine.mock.*
import io.ktor.http.*
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

class PaginationTest {

    @Test
    fun listResultDelegatesToList() {
        val items = listOf("a", "b", "c")
        val result = ListResult(items, ListMeta(totalCount = 10, truncated = false))

        assertEquals(3, result.size)
        assertEquals("a", result[0])
        assertEquals("c", result[2])
        assertEquals(10, result.meta.totalCount)
        assertFalse(result.meta.truncated)
    }

    @Test
    fun listResultWorksWithCollectionOperations() {
        val items = listOf(1, 2, 3, 4, 5)
        val result = ListResult(items, ListMeta(totalCount = 100, truncated = true))

        // map returns plain List
        val doubled = result.map { it * 2 }
        assertEquals(listOf(2, 4, 6, 8, 10), doubled)

        // filter
        val even = result.filter { it % 2 == 0 }
        assertEquals(listOf(2, 4), even)

        // forEach
        var sum = 0
        result.forEach { sum += it }
        assertEquals(15, sum)

        // spread into another list
        val spread = listOf(0) + result
        assertEquals(listOf(0, 1, 2, 3, 4, 5), spread)
    }

    @Test
    fun listResultEmptyCase() {
        val result = ListResult(emptyList<String>(), ListMeta(totalCount = 0, truncated = false))
        assertEquals(0, result.size)
        assertTrue(result.isEmpty())
    }

    @Test
    fun listResultEqualityIncludesMeta() {
        val a = ListResult(listOf(1, 2), ListMeta(10, false))
        val b = ListResult(listOf(1, 2), ListMeta(10, false))
        val c = ListResult(listOf(1, 2), ListMeta(20, true))

        assertEquals(a, b)
        assertFalse(a == c)
    }

    // =========================================================================
    // parseNextLink
    // =========================================================================

    @Test
    fun parseNextLinkExtractsUrl() {
        val header = """<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next""""
        assertEquals("https://3.basecampapi.com/12345/projects.json?page=2", parseNextLink(header))
    }

    @Test
    fun parseNextLinkHandlesMultipleRels() {
        val header = """<https://example.com?page=1>; rel="prev", <https://example.com?page=3>; rel="next""""
        assertEquals("https://example.com?page=3", parseNextLink(header))
    }

    @Test
    fun parseNextLinkReturnsNullWhenNoNext() {
        assertNull(parseNextLink("""<https://example.com?page=1>; rel="prev""""))
        assertNull(parseNextLink(null))
        assertNull(parseNextLink(""))
    }

    // =========================================================================
    // isSameOrigin
    // =========================================================================

    @Test
    fun sameOriginMatchesExactly() {
        assertTrue(isSameOrigin(
            "https://3.basecampapi.com/12345/projects.json",
            "https://3.basecampapi.com/12345/todos.json",
        ))
    }

    @Test
    fun sameOriginRejectsDifferentHosts() {
        assertFalse(isSameOrigin(
            "https://3.basecampapi.com/12345/projects.json",
            "https://evil.com/12345/projects.json",
        ))
    }

    @Test
    fun sameOriginRejectsDifferentSchemes() {
        assertFalse(isSameOrigin(
            "https://example.com/path",
            "http://example.com/path",
        ))
    }

    @Test
    fun sameOriginRejectsDifferentPorts() {
        assertFalse(isSameOrigin(
            "https://example.com:443/path",
            "https://example.com:8443/path",
        ))
    }

    // =========================================================================
    // parseRetryAfter
    // =========================================================================

    @Test
    fun parseRetryAfterParsesSeconds() {
        assertEquals(30, parseRetryAfter("30"))
        assertEquals(1, parseRetryAfter("1"))
    }

    @Test
    fun parseRetryAfterReturnsNullForInvalid() {
        assertNull(parseRetryAfter(null))
        assertNull(parseRetryAfter(""))
        assertNull(parseRetryAfter("0"))
        assertNull(parseRetryAfter("-1"))
        assertNull(parseRetryAfter("not-a-number"))
    }

    // =========================================================================
    // parseTotalCount
    // =========================================================================

    @Test
    fun parseTotalCountExtractsValue() {
        val headers = mapOf("X-Total-Count" to listOf("42"))
        assertEquals(42, parseTotalCount(headers))
    }

    @Test
    fun parseTotalCountReturnsZeroForMissing() {
        assertEquals(0, parseTotalCount(emptyMap()))
    }

    @Test
    fun parseTotalCountReturnsZeroForInvalid() {
        val headers = mapOf("X-Total-Count" to listOf("not-a-number"))
        assertEquals(0, parseTotalCount(headers))
    }

    // =========================================================================
    // Paginated request integration tests
    // =========================================================================

    private fun projectJson(id: Long, name: String) = """{
        "id": $id, "status": "active", "name": "$name",
        "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
        "url": "https://3.basecampapi.com/12345/projects/$id.json",
        "app_url": "https://3.basecamp.com/12345/projects/$id",
        "dock": []
    }"""

    private fun mockClient(handler: MockRequestHandler): BasecampClient {
        val engine = MockEngine(handler)
        return BasecampClient {
            accessToken("test-token")
            this.engine = engine
        }
    }

    @Test
    fun ssrfRejectionWhenLinkRedirectsToDifferentOrigin() = runTest {
        val client = mockClient { request ->
            respond(
                content = """[${projectJson(1, "Project 1")}]""",
                status = HttpStatusCode.OK,
                headers = headersOf(
                    HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    "Link" to listOf("""<https://evil.com/12345/projects.json?page=2>; rel="next""""),
                    "X-Total-Count" to listOf("2"),
                ),
            )
        }

        val account = client.forAccount("12345")
        try {
            account.projects.list()
            assertTrue(false, "Should have thrown for SSRF")
        } catch (e: BasecampException.Api) {
            assertTrue(e.message!!.contains("different origin"))
        }
        client.close()
    }

    @Test
    fun emptyResultNoItemsNoLinkHeader() = runTest {
        val client = mockClient { _ ->
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
        val projects = account.projects.list()

        assertEquals(0, projects.size)
        assertTrue(projects.isEmpty())
        assertEquals(0L, projects.meta.totalCount)
        assertFalse(projects.meta.truncated)
        client.close()
    }

    @Test
    fun paginationFollowsMultiplePages() = runTest {
        var requestCount = 0
        val client = mockClient { request ->
            requestCount++
            val page = request.url.parameters["page"]?.toIntOrNull() ?: 1
            when (page) {
                1 -> respond(
                    content = """[${projectJson(1, "Project 1")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=2>; rel="next""""),
                        "X-Total-Count" to listOf("3"),
                    ),
                )
                2 -> respond(
                    content = """[${projectJson(2, "Project 2")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                        "Link" to listOf("""<https://3.basecampapi.com/12345/projects.json?page=3>; rel="next""""),
                    ),
                )
                else -> respond(
                    content = """[${projectJson(3, "Project 3")}]""",
                    status = HttpStatusCode.OK,
                    headers = headersOf(
                        HttpHeaders.ContentType to listOf(ContentType.Application.Json.toString()),
                    ),
                )
            }
        }

        val account = client.forAccount("12345")
        val projects = account.projects.list()

        assertEquals(3, projects.size)
        assertEquals(1L, projects[0].id)
        assertEquals(2L, projects[1].id)
        assertEquals(3L, projects[2].id)
        assertEquals(3L, projects.meta.totalCount)
        assertFalse(projects.meta.truncated)
        client.close()
    }
}
