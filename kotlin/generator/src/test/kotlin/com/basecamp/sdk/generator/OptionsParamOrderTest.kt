package com.basecamp.sdk.generator

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Pins the append-only constructor-order rule for generated options classes.
 *
 * Options classes are data classes with defaults, so callers may construct them
 * positionally: constructor position is public API, and the pre-1.0 policy in
 * kotlin/README.md is append-only. Without the pin, the natural order (optional
 * query params in spec order, then the synthetic `maxItems`) silently displaces
 * `maxItems` every time an operation gains a query param — which is exactly what
 * wiring `page` did to sixteen shipped classes.
 */
class OptionsParamOrderTest {

    @Test
    fun `a newly added parameter is appended after the pinned ones`() {
        val pinned = listOf("sort", "direction", "maxItems")
        val natural = listOf("sort", "direction", "page", "maxItems")

        assertEquals(listOf("sort", "direction", "maxItems", "page"), orderOptionsParams(pinned, natural))
    }

    @Test
    fun `several new parameters append in natural order, after the pinned ones`() {
        val pinned = listOf("status", "maxItems")
        val natural = listOf("status", "page", "since", "maxItems")

        assertEquals(listOf("status", "maxItems", "page", "since"), orderOptionsParams(pinned, natural))
    }

    @Test
    fun `the pin wins over spec order for parameters it already covers`() {
        // The spec reordering its own parameters must not move a shipped one.
        val pinned = listOf("page", "maxItems")
        val natural = listOf("maxItems", "page")

        assertEquals(listOf("page", "maxItems"), orderOptionsParams(pinned, natural))
    }

    @Test
    fun `a class with no pin is emitted in natural order`() {
        val natural = listOf("page", "maxItems")

        assertEquals(natural, orderOptionsParams(emptyList(), natural))
    }

    @Test
    fun `a pinned parameter the spec has dropped falls out without shifting the rest`() {
        val pinned = listOf("status", "legacyFilter", "maxItems")
        val natural = listOf("status", "maxItems", "page")

        assertEquals(listOf("status", "maxItems", "page"), orderOptionsParams(pinned, natural))
    }

    @Test
    fun `ordering never drops or duplicates a parameter the spec declares`() {
        val pinned = listOf("b", "gone", "a")
        val natural = listOf("a", "b", "c", "d")

        val ordered = orderOptionsParams(pinned, natural)

        assertEquals(natural.toSet(), ordered.toSet())
        assertEquals(natural.size, ordered.size)
        assertTrue(ordered.indexOf("b") < ordered.indexOf("a"), "pinned order must survive")
        assertTrue(ordered.indexOf("a") < ordered.indexOf("c"), "unpinned params come last")
    }

    /**
     * The compatibility bridge is a one-time migration aid, so its roster is a
     * frozen list rather than a predicate over the current spec: a predicate
     * would start emitting bridges for operations that never had a
     * `PaginationOptions` signature, making untyped callable references
     * ambiguous, and would stop emitting them for operations that gain a second
     * query parameter, breaking the call sites the bridge exists for.
     */
    @Test
    fun `the PaginationOptions bridge roster is frozen and non-empty`() {
        assertEquals(22, PAGINATION_OPTIONS_COMPAT_OVERLOADS.size)
        assertTrue("ListComments" in PAGINATION_OPTIONS_COMPAT_OVERLOADS, "an operation that gained its first optional query param")
        assertTrue("ListMyBookmarks" !in PAGINATION_OPTIONS_COMPAT_OVERLOADS, "an operation whose options type never changed")
        assertTrue("Search" !in PAGINATION_OPTIONS_COMPAT_OVERLOADS, "an operation whose options type never changed")
    }
}
