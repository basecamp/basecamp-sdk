package com.basecamp.sdk.conformance

/**
 * Validates one `delayBetweenRequests` assertion against the recorded request
 * times, returning `null` when it holds and a failure message otherwise.
 *
 * Gap i is the interval between request i and request i+1, so N requests yield
 * N-1 gaps. The contract in conformance/schema.json:
 *
 * - A NAMED index selects exactly that gap, bounds-checked unconditionally. A
 *   gap the run never produced is a failure, not a silent pass — the whole
 *   point of a timing pin is to catch a dropped backoff, and a dropped backoff
 *   is precisely what removes the gap.
 * - An OMITTED index requires the minimum on EVERY gap. Zero gaps means
 *   nothing was measured, so that fails too: an "every gap" rule with no gaps
 *   left would otherwise wave through a run that dropped every retry.
 * - Negative indexes are rejected rather than wrapping to the end the way the
 *   per-request assertions do. There is no sensible "last gap" when the point
 *   of naming one is to pin a specific backoff.
 *
 * The bounds test compares against the gap COUNT rather than adding one to the
 * index: `gap + 1 >= requestTimes.size` overflows for [Int.MAX_VALUE] and wraps
 * negative, sailing through the guard into an out-of-range read.
 */
fun checkDelayGaps(requestTimes: List<Long>, minDelay: Long, index: Int?): String? {
    val gaps = requestTimes.size - 1

    if (index != null) {
        return when {
            index < 0 ->
                "delayBetweenRequests gap index must be non-negative, got $index"
            index >= gaps ->
                "Expected a delay at gap $index, but only ${requestTimes.size} request(s) were made"
            else -> shortfall(requestTimes, minDelay, index)
        }
    }

    if (gaps < 1) {
        return "Expected a delay between requests, but only ${requestTimes.size} request(s) were made"
    }
    for (i in 0 until gaps) {
        shortfall(requestTimes, minDelay, i)?.let { return it }
    }
    return null
}

private fun shortfall(requestTimes: List<Long>, minDelay: Long, gap: Int): String? {
    val delay = requestTimes[gap + 1] - requestTimes[gap]
    return if (delay < minDelay) "Expected delay >= ${minDelay}ms at gap $gap, got ${delay}ms" else null
}
