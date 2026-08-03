package com.basecamp.sdk.conformance

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * Both directions of the errorRaised assertion contract.
 *
 * Only the passing direction ever runs against a committed fixture: every case
 * declaring errorRaised is one the SDK does refuse. A handler that accepted
 * everything would therefore look green in all six runners at once, which is
 * exactly how #563 shipped a vacuous delayBetweenRequests check.
 */
class ErrorRaisedTest {
    companion object {
        /**
         * Asserted verbatim here and in the five sibling runners. A fixture
         * debugged in one language should not read differently in another.
         */
        const val MESSAGE = "Expected the call to fail, but it succeeded"
    }

    @Test
    fun `a failed dispatch satisfies the assertion`() {
        assertNull(errorRaisedFailure(true))
    }

    @Test
    fun `a successful dispatch fails the assertion`() {
        // The branch under test. It is unreachable from conformance/tests/, so
        // without this case the handler could accept everything undetected.
        assertEquals(MESSAGE, errorRaisedFailure(false))
    }
}
