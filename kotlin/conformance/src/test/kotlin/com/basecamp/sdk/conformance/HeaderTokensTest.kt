package com.basecamp.sdk.conformance

import kotlin.test.Test
import kotlin.test.assertContains
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith

/**
 * The `{{httpdate+Ns}}` header token (SPEC §19, conformance/schema.json).
 *
 * A static fixture has no clock, so the positive half of SPEC §6's HTTP-date
 * branch was unpinnable until this token (#780). These cases pin the resolver's
 * arithmetic against a frozen instant so the fixture's one-sided timing floor
 * rests on a deterministic contract.
 */
class HeaderTokensTest {
    /** A quarter-second into 10:18:14 UTC, so floor and round-up differ. */
    private val nowMs = 1_623_233_894_250L

    @Test
    fun `plain values pass through`() {
        for (value in listOf("", "2", "Wed, 09 Jun 2021 10:18:14 GMT", "application/json", "{not a token}")) {
            assertEquals(value, resolveHeaderValue(value, nowMs))
        }
    }

    @Test
    fun `httpdate resolves to the whole second past N`() {
        assertEquals("Wed, 09 Jun 2021 10:18:17 GMT", resolveHeaderValue("{{httpdate+2s}}", nowMs))
        assertEquals("Wed, 09 Jun 2021 10:18:15 GMT", resolveHeaderValue("{{httpdate+0s}}", nowMs))
        assertEquals("Wed, 09 Jun 2021 10:18:25 GMT", resolveHeaderValue("{{httpdate+10s}}", nowMs))
    }

    @Test
    fun `unknown tokens are errors not literals`() {
        for (value in listOf("{{httpdate}}", "{{httpdate+2}}", "{{httpdate-2s}}", "{{now}}", "{{}}")) {
            val error = assertFailsWith<IllegalArgumentException> { resolveHeaderValue(value, nowMs) }
            assertContains(error.message ?: "", value)
        }
    }
}
