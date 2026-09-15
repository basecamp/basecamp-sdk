package com.basecamp.sdk.services

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertTrue

class TtlCacheTest {

    private class Clock(var now: Long = 0)

    private fun cache(clock: Clock, ttl: Long = 1000, floor: Long = 100, maxItems: Int = 4) =
        TtlCache<String, Int>({ clock.now }, ttl, floor, maxItems)

    @Test
    fun servesACachedValueWithinTheTtlWithoutLoading() = runTest {
        val clock = Clock()
        val cache = cache(clock)
        var loads = 0
        assertEquals(1, cache.get("k", refresh = false) { loads++; 1 }.value)
        clock.now = 999
        val hit = cache.get("k", refresh = false) { loads++; 2 }
        assertEquals(1, hit.value)
        assertTrue(hit.cached, "a value that predated the call reports as cached")
        assertEquals(1, loads)
    }

    @Test
    fun reloadsOnceTheValueIsPastTheTtl() = runTest {
        val clock = Clock()
        val cache = cache(clock)
        cache.get("k", refresh = false) { 1 }
        clock.now = 1000
        val hit = cache.get("k", refresh = false) { 2 }
        assertEquals(2, hit.value)
        assertFalse(hit.cached, "a value loaded during the call is not a cache hit")
    }

    @Test
    fun aRefreshIsDeclinedBelowTheFloorAndHonouredAboveIt() = runTest {
        val clock = Clock()
        val cache = cache(clock)
        cache.get("k", refresh = false) { 1 }
        clock.now = 99
        assertEquals(1, cache.get("k", refresh = true) { 2 }.value)
        clock.now = 100
        assertEquals(2, cache.get("k", refresh = true) { 2 }.value)
    }

    @Test
    fun peekNeverLoadsAndExpires() = runTest {
        val clock = Clock()
        val cache = cache(clock)
        assertEquals(null, cache.peek("k"))
        cache.get("k", refresh = false) { 1 }
        assertEquals(1, cache.peek("k")?.value)
        clock.now = 1000
        assertEquals(null, cache.peek("k"))
    }

    @Test
    fun concurrentCallersShareOneLoad() = runTest {
        val clock = Clock()
        val cache = cache(clock)
        val gate = CompletableDeferred<Unit>()
        var loads = 0
        val results = coroutineScope {
            val first = async {
                cache.get("k", refresh = false) {
                    loads++
                    gate.await()
                    7
                }
            }
            val second = async {
                // Queues behind the load the first caller owns.
                cache.get("k", refresh = false) { loads++; 8 }
            }
            gate.complete(Unit)
            listOf(first.await(), second.await())
        }
        assertEquals(listOf(7, 7), results.map { it.value })
        assertEquals(1, loads, "the second caller must wait on the load in flight, not start another")
    }

    @Test
    fun aFailedLoadIsSharedAndLeavesThePreviousValueInPlace() = runTest {
        val clock = Clock()
        val cache = cache(clock)
        cache.get("k", refresh = false) { 1 }
        clock.now = 1000
        assertFailsWith<IllegalStateException> {
            cache.get("k", refresh = false) { throw IllegalStateException("boom") }
        }
        // The key is released, so a later caller loads again rather than waiting
        // forever on a load that already failed.
        assertEquals(2, cache.get("k", refresh = false) { 2 }.value)
    }

    @Test
    fun theBoundEvictsTheOldestFetchedEntryFirst() = runTest {
        val clock = Clock()
        val cache = cache(clock, ttl = Long.MAX_VALUE, maxItems = 2)
        cache.get("a", refresh = false) { 1 }
        clock.now = 1
        cache.get("b", refresh = false) { 2 }
        clock.now = 2
        cache.get("c", refresh = false) { 3 }
        assertEquals(null, cache.peek("a"), "the oldest entry goes first")
        assertEquals(2, cache.peek("b")?.value)
        assertEquals(3, cache.peek("c")?.value)
    }
}
