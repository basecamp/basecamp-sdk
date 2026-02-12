package com.basecamp.sdk

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
}
