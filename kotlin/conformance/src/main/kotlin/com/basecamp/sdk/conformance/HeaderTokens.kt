package com.basecamp.sdk.conformance

import io.ktor.http.toHttpDate
import io.ktor.util.date.GMTDate

private val HEADER_TOKEN = Regex("""^\{\{(.*)\}\}$""")
private val HTTPDATE_TOKEN = Regex("""^httpdate\+(\d{1,9})s$""")

/**
 * Substitutes the one token a fixture header value may carry, `{{httpdate+Ns}}`
 * (SPEC §19, conformance/schema.json), at the moment the response is served.
 * Every other value passes through untouched.
 *
 * The token resolves to the IMF-fixdate of floor(now) + N + 1 seconds: the
 * first whole second strictly more than N seconds after the second the response
 * is served in. A compliant SPEC §6 parser sees a remainder in (N − latency,
 * N + 1] and, rounding up, computes at least N whole seconds, so the fixture
 * pairs it with a `delayBetweenRequests` floor of N × 1000 ms. It exists
 * because a static fixture has no clock: a literal past date pins only the
 * fall-through, and a far-future one is differently behaved per host.
 *
 * N is one to nine digits, so the arithmetic is exact everywhere and every
 * runner's date formatter stays in range; a longer N is an unrecognised token.
 *
 * An unrecognised `{{…}}` throws rather than passing through: a typo'd token
 * served verbatim would be an unparseable header, which the SDK answers with
 * its ordinary backoff — the exact outcome the case exists to distinguish from.
 */
fun resolveHeaderValue(value: String, nowMs: Long): String {
    val token = HEADER_TOKEN.matchEntire(value) ?: return value
    val inner = HTTPDATE_TOKEN.matchEntire(token.groupValues[1])
        ?: throw IllegalArgumentException(
            "unrecognised header token \"$value\": only {{httpdate+Ns}} is defined (conformance/schema.json)",
        )
    val seconds = Math.floorDiv(nowMs, 1000L) + inner.groupValues[1].toLong() + 1
    return GMTDate(seconds * 1000).toHttpDate()
}
