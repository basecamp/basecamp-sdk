package com.basecamp.sdk.conformance

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull

/**
 * Bounds contract for the requestCount assertion (#573).
 *
 * Until this commit the five non-Swift runners evaluated requestCount as a
 * LOWER bound whenever any mock response carried `Link: rel="next"`. Every
 * committed fixture passes under both rules, so nothing in the suite could tell
 * them apart — the same shape as the #563 delayBetweenRequests regression these
 * support modules exist to pin. The over-fetch case below is the one that
 * distinguishes them, and it is the case that matters: pagination.json's
 * maxPages and maxItems fixtures each queue three pages and assert two
 * requests, so a lower bound green-passes an SDK that ignored the cap.
 */
class RequestCountTest {
    @Test
    fun `the exact count passes`() {
        assertNull(checkRequestCount(2, 2))
    }

    @Test
    fun `an under-fetch fails`() {
        assertNotNull(checkRequestCount(1, 2))
    }

    @Test
    fun `an over-fetch fails`() {
        // The regression. Under the old lower bound this returned null — an SDK
        // that walked all three queued pages instead of stopping at the
        // maxPages cap reported a clean pass.
        assertNotNull(checkRequestCount(3, 2))
    }

    @Test
    fun `the failure message names both counts`() {
        assertEquals("Expected 2 requests, got 3", checkRequestCount(3, 2))
    }

    @Test
    fun `a zero-request run is not a free pass`() {
        // A test whose operation never reached the wire records zero requests.
        // That must fail an assertion expecting one, not read as "no data, no
        // opinion".
        assertNotNull(checkRequestCount(0, 1))
    }

    @Test
    fun `zero expected requires zero actual`() {
        assertNull(checkRequestCount(0, 0))
        assertNotNull(checkRequestCount(1, 0))
    }
}
