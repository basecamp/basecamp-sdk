package com.basecamp.sdk

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * The sgid decoder's leniency, row for row against Go's
 * `base64.RawStdEncoding.DecodeString` — the reference this port has to match,
 * probed rather than reasoned about.
 *
 * The asymmetry is the point and is easy to get wrong in either direction: Go
 * ignores CR and LF but REFUSES space and tab, so a decoder that treats
 * "whitespace" as one category is wrong whichever way it jumps. Being stricter
 * than Go here does not raise — it makes a real mention silently stop being one.
 */
class Base64LeniencyTest {

    private fun decodes(s: String): String? = decodeBase64(s)?.decodeToString()

    @Test
    fun aWellFormedUnpaddedPayloadDecodes() {
        assertEquals("Hello", decodes("SGVsbG8"))
    }

    @Test
    fun nonZeroDiscardedTrailingBitsAreAccepted() {
        // Go accepts these; only Encoding.Strict() rejects them. "AB" carries a
        // whole byte plus four leftover bits that are not zero.
        assertEquals(1, decodeBase64("AB")?.size)
    }

    @Test
    fun aFinalGroupOfOneCharacterIsRejected() {
        // length % 4 == 1 encodes no whole byte; Go rejects it too.
        assertNull(decodeBase64("A"))
        assertNull(decodeBase64("SGVsbG8AA"))
    }

    @Test
    fun lineFeedAndCarriageReturnAreIgnored() {
        assertEquals("Hello", decodes("SGVs\nbG8"))
        assertEquals("Hello", decodes("SGVs\rbG8"))
        assertEquals("Hello", decodes("SGVs\r\nbG8"))
    }

    @Test
    fun spaceAndTabAreRejected() {
        assertNull(decodeBase64("SGVs bG8"))
        assertNull(decodeBase64("SGVs\tbG8"))
    }
}
