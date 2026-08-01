package com.basecamp.sdk.conformance

import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * Bounds and every-gap contract for the `delayBetweenRequests` assertion.
 *
 * Each case names a behavior that regressed on #563/#568: an assertion that
 * looked like it covered a timing gap and did not.
 */
class DelayGapsTest {
    /** Builds request timestamps from successive gaps in milliseconds. */
    private fun times(vararg gapsMs: Long): List<Long> {
        val out = mutableListOf(0L)
        for (ms in gapsMs) out.add(out.last() + ms)
        return out
    }

    @Test
    fun `omitted index catches a later failing gap`() {
        // Three runners measured gap 0 and stopped, so a second backoff that
        // never happened passed unnoticed.
        assertContains(checkDelayGaps(times(1000, 5), 500, null)!!, "at gap 1")
    }

    @Test
    fun `omitted index passes when every gap clears the minimum`() {
        assertNull(checkDelayGaps(times(1000, 2000, 800), 500, null))
    }

    @Test
    fun `omitted index fails when there are no gaps at all`() {
        // An "every gap" rule with no gaps left must not wave the run through:
        // a fully dropped retry lands exactly here.
        assertEquals(
            "Expected a delay between requests, but only 1 request(s) were made",
            checkDelayGaps(times(), 500, null),
        )
    }

    @Test
    fun `named gap fails when the run never produced it`() {
        assertEquals(
            "Expected a delay at gap 1, but only 2 request(s) were made",
            checkDelayGaps(times(1000), 500, 1),
        )
    }

    @Test
    fun `named gap fails on a single-request run`() {
        assertEquals(
            "Expected a delay at gap 0, but only 1 request(s) were made",
            checkDelayGaps(times(), 500, 0),
        )
    }

    @Test
    fun `negative gap index is rejected`() {
        // Rejected categorically, not wrapped to the end the way
        // headerPresent's index is.
        assertEquals(
            "delayBetweenRequests gap index must be non-negative, got -1",
            checkDelayGaps(times(1000, 2000), 500, -1),
        )
    }

    @Test
    fun `Int MAX_VALUE gap index fails without overflowing`() {
        // `gap + 1 >= size` computes the addition first, so Int.MAX_VALUE wraps
        // negative and sails through the guard into an out-of-range read.
        assertContains(
            checkDelayGaps(times(1000, 2000), 500, Int.MAX_VALUE)!!,
            "Expected a delay at gap",
        )
    }

    @Test
    fun `zero minimum still asserts that the gap exists`() {
        // A zero minimum is trivially met; the EXISTENCE requirement is not.
        assertEquals(
            "Expected a delay between requests, but only 1 request(s) were made",
            checkDelayGaps(times(), 0, null),
        )
        assertEquals(
            "Expected a delay at gap 0, but only 1 request(s) were made",
            checkDelayGaps(times(), 0, 0),
        )
        assertNull(checkDelayGaps(times(5), 0, null))
    }

    @Test
    fun `named gap passes when it clears the minimum`() {
        assertNull(checkDelayGaps(times(5, 2000), 500, 1))
    }

    @Test
    fun `named gap fails when it is below the minimum`() {
        assertContains(checkDelayGaps(times(2000, 5), 500, 1)!!, "at gap 1")
    }
}
