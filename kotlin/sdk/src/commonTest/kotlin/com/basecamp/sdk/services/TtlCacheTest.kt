package com.basecamp.sdk.services

import com.basecamp.sdk.BasecampException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancel
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.yield
import kotlin.coroutines.cancellation.CancellationException
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
        val loading = CompletableDeferred<Unit>()
        var loads = 0
        // The handshake matters: the second caller has to reach the cache while
        // the first caller's load is STILL in flight. Completing the gate before
        // either coroutine runs would let the first finish and the second take an
        // ordinary cache hit — one load either way, and the single-flight path
        // never exercised.
        val first = async {
            cache.get("k", refresh = false) {
                loads++
                loading.complete(Unit)
                gate.await()
                7
            }
        }
        loading.await()
        val second = async { cache.get("k", refresh = false) { loads++; 8 } }
        yield()
        gate.complete(Unit)
        val results = listOf(first.await(), second.await())
        assertEquals(listOf(7, 7), results.map { it.value })
        assertFalse(results[1].cached, "the value came from the load this call waited on, not from an entry")
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

    @Test
    fun aLoadCancelledByItsOwnTimeoutIsSharedRatherThanRerunPerWaiter() = runTest {
        // Ktor spells a request timeout as a CancellationException with the
        // calling job still active. That is the LOAD's failure, not the caller's,
        // so every waiter takes it — otherwise one timed-out listing becomes one
        // listing per waiting call. The Go cache tests the same two halves
        // (`callerDone`): context done AND the error being that context's.
        val cache = cache(Clock())
        val gate = CompletableDeferred<Unit>()
        val loading = CompletableDeferred<Unit>()
        var loads = 0
        val owner = async {
            runCatching {
                cache.get("k", refresh = false) {
                    loads++
                    loading.complete(Unit)
                    gate.await()
                    throw CancellationException("simulated transport timeout")
                }
            }
        }
        loading.await()
        val waiter = async { runCatching { cache.get("k", refresh = false) { loads++; 9 } } }
        yield()
        gate.complete(Unit)
        val outcomes = listOf(owner.await(), waiter.await())
        assertEquals(1, loads, "the failed load is shared, not re-run behind each waiter")
        assertTrue(outcomes.all { it.isFailure }, "both callers see the load's own failure")
    }

    @Test
    fun aCancelledOwnerReleasesTheKeyAndALiveWaiterLoadsForItself() = runTest {
        // The owner's cancellation is the owner's. A waiter whose own job is live
        // goes round once and loads for itself — and can only do that because the
        // key was released, which is what makes publication uncancellable. Without
        // that, this test hangs on a deferred nobody completes.
        val cache = cache(Clock())
        val gate = CompletableDeferred<Unit>()
        val ownerStarted = CompletableDeferred<Unit>()
        var loads = 0
        val ownerScope = CoroutineScope(coroutineContext + Job())
        ownerScope.launch {
            cache.get("k", refresh = false) {
                loads++
                ownerStarted.complete(Unit)
                gate.await()
                1
            }
        }
        ownerStarted.await()
        val waiter = async { cache.get("k", refresh = false) { loads++; 7 } }
        yield()
        ownerScope.cancel()
        assertEquals(7, waiter.await().value)
        assertEquals(2, loads, "the waiter loaded for itself rather than inheriting the cancellation")
    }

    @Test
    fun anErrorNotJustAnExceptionStillReleasesTheKey() = runTest {
        // The release path is a `catch (Throwable)`, and this pins the breadth of
        // it: an Error is the shape a `catch (Exception)` would step over. It
        // matters because waiters wait with no timeout, so a key left in flight
        // is a permanent hang for every later caller — and the listing's key is a
        // whole account.
        val cache = cache(Clock())
        assertFailsWith<AssertionError>("the failure reaches the caller") {
            cache.get("k", refresh = false) { throw AssertionError("loader died") }
        }
        assertEquals(5, cache.get("k", refresh = false) { 5 }.value, "the key was released, not poisoned")
    }

    @Test
    fun aWaiterPastItsOneRetryIsNotEndedByAnotherJobsCancellation() = runTest {
        // Reaching the latched arm takes two cancelled owners in a row. First A
        // owns and is cancelled, which sends both waiters round once; the first
        // of them (W) becomes the new owner, so the second (B) is now latched
        // behind W. Cancelling W then lands B on the branch under test.
        //
        // What that branch must NOT do is relay W's CancellationException. It
        // belongs to W's job, not B's, and Kotlin reads a CancellationException
        // structurally: JobSupport.childCancelled short-circuits on one, so no
        // CoroutineExceptionHandler runs and a `launch { }` caller would see its
        // coroutine end as "cancelled" with the failure lost. Go hands its waiter
        // an ordinary error value.
        val cache = cache(Clock())
        val aLoading = CompletableDeferred<Unit>()
        val wLoading = CompletableDeferred<Unit>()
        var loads = 0

        val aScope = CoroutineScope(coroutineContext + Job())
        aScope.launch {
            cache.get("k", refresh = false) { loads++; aLoading.complete(Unit); CompletableDeferred<Unit>().await(); 1 }
        }
        aLoading.await()

        val wScope = CoroutineScope(coroutineContext + Job())
        wScope.launch {
            cache.get("k", refresh = false) { loads++; wLoading.complete(Unit); CompletableDeferred<Unit>().await(); 2 }
        }
        yield()
        val b = async { runCatching { cache.get("k", refresh = false) { loads++; 3 } } }
        yield()

        aScope.cancel()
        wLoading.await()
        wScope.cancel()

        val failure = b.await().exceptionOrNull()
        assertTrue(failure !is CancellationException, "got ${failure?.let { it::class.simpleName }}")
        assertTrue(failure is BasecampException.Api, "got ${failure?.let { it::class.simpleName }}")
        assertEquals(2, loads, "B never loaded for itself past its one retry")
    }
}
