package com.basecamp.sdk

import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * The character-reference scan must be LINEAR in its input.
 *
 * It was not. Looking for the next `;` to delimit a name scans to the next
 * semicolon anywhere in the input — to the end of it when there is none — and a
 * length check applied to the result does not bound the scan that produced it.
 * A run of ampersands is the worst case and is reachable from any attribute an
 * author can write: measured at 35 ms for 80,000 characters, scaling 12x for a
 * 4x input. Bounding the search by the name characters actually present made
 * that 0.3 ms.
 *
 * **This asserts a RATIO, not a wall clock.** An absolute time is a different
 * number in debug, on a shared runner, and on a laptop — a test that pins one is
 * measuring the machine rather than the algorithm, and it will be wrong
 * somewhere. Two input sizes at 4x apart predict ~4x for linear work and ~16x
 * for quadratic, which is a gap wide enough to call without pinning anything
 * machine-specific.
 *
 * Lives in jvmTest because it needs a clock and a warmed JIT.
 */
class EntityScanScalingTest {

    /** The longest name in the decoder's tables; a scan is bounded by it. */
    private val MAX_NAME = 16

    private fun minNanosPer(run: Int, iterations: Int = 10, trials: Int = 5): Long {
        // `&` alone exercises the CHEAPEST path in the bounded scan: the next
        // character is never a name character, so the scan stops immediately.
        // A name-shaped run makes every `&` pay the full bounded scan, which is
        // the work the bound is supposed to keep linear — and it still catches a
        // revert to searching for the next `;`.
        val markup = "<bc-attachment sgid=\"" + ("&" + "a".repeat(MAX_NAME)).repeat(run) + "\"></bc-attachment>"
        // Warm the JIT before measuring anything, or the first trial measures
        // the interpreter and every ratio after it is meaningless.
        repeat(20) { mentionedPersonIds(markup) }
        // The MINIMUM across trials, not the mean: a timing sample is bounded
        // below by the real cost and unbounded above by whatever else the
        // machine was doing, so the minimum is the robust estimator.
        var best = Long.MAX_VALUE
        repeat(trials) {
            val start = System.nanoTime()
            repeat(iterations) { mentionedPersonIds(markup) }
            best = minOf(best, (System.nanoTime() - start) / iterations)
        }
        return best
    }

    @Test
    fun aRunOfNameShapedReferencesCostsLinearTimeNotQuadratic() {
        val small = minNanosPer(20_000)
        val large = minNanosPer(80_000)

        // The floor: if the larger input is too fast to measure, the ratio is
        // noise and a pass would mean nothing. Failing here says "this test
        // stopped being able to tell" rather than quietly agreeing.
        assertTrue(
            large > 10_000,
            "the larger run measured ${large}ns, too small for the ratio below to mean anything",
        )

        val ratio = large.toDouble() / small
        // Linear predicts ~4. Quadratic predicts ~16. Eight sits between them
        // with room for the noise a shared runner adds.
        assertTrue(
            ratio < 8.0,
            "the scan is superlinear: 4x the input cost ${"%.1f".format(ratio)}x the time " +
                "(${small}ns -> ${large}ns). A name search that is not bounded by the name " +
                "characters present scans to the next semicolon anywhere in the input.",
        )
    }
}
