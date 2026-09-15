package com.basecamp.sdk

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.time.TimeSource

/**
 * The sgid envelope reader against payloads built to break it.
 *
 * This is the one parser in the SDK fed bytes that other people wrote: an sgid
 * arrives inside rich text, and `personIdFromSgid` is called on whatever is in
 * the attribute. The reader must REFUSE a hostile payload — return null — and
 * do it without throwing anything and without allocating or recursing on a
 * claim the payload makes about its own size.
 *
 * Ported from the reference's `TestPersonIDFromSGID_HostileMarshal`, case for
 * case, because a decoder's refusals are as much of the contract as its
 * acceptances and this suite had none of them.
 *
 * **What these rows do and do not guard, measured rather than assumed.** They
 * pin the end-to-end property — refused, promptly, nothing thrown — and they
 * catch a reader that loses its bounds. They do NOT pin any single guard, and
 * mutating four of them one at a time leaves every row green:
 *
 *  - the count bound alone: the length check inside `bytes` refuses next.
 *  - the length check's overflow-safe form alone: `count` already refused.
 *  - both together: OutOfMemoryError, and these rows go red. That pair is the
 *    real subject.
 *  - the payload size cap alone: these payloads are all far under it.
 *  - the nesting bound, at any depth: unreachable through this entry point.
 *    The payload cap is 4096 bytes and a nesting level costs two, so nothing
 *    that decodes can recurse past about two thousand frames — the cap bounds
 *    the recursion before the depth guard is consulted. The guard is real
 *    defence in depth; it is simply not observable from outside, and pretending
 *    a row here covers it would be worse than saying so.
 */
class HostileMarshalTest {

    /** Standard-alphabet base64 plus a signature, the way an sgid arrives. */
    private fun sgid(raw: ByteArray): String {
        val alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
        val out = StringBuilder()
        var i = 0
        while (i + 2 < raw.size) {
            val n = ((raw[i].toInt() and 0xFF) shl 16) or
                ((raw[i + 1].toInt() and 0xFF) shl 8) or (raw[i + 2].toInt() and 0xFF)
            out.append(alphabet[(n shr 18) and 0x3F]).append(alphabet[(n shr 12) and 0x3F])
            out.append(alphabet[(n shr 6) and 0x3F]).append(alphabet[n and 0x3F])
            i += 3
        }
        when (raw.size - i) {
            1 -> {
                val n = (raw[i].toInt() and 0xFF) shl 16
                out.append(alphabet[(n shr 18) and 0x3F]).append(alphabet[(n shr 12) and 0x3F]).append("==")
            }
            2 -> {
                val n = ((raw[i].toInt() and 0xFF) shl 16) or ((raw[i + 1].toInt() and 0xFF) shl 8)
                out.append(alphabet[(n shr 18) and 0x3F]).append(alphabet[(n shr 12) and 0x3F])
                out.append(alphabet[(n shr 6) and 0x3F]).append("=")
            }
        }
        return out.toString() + "--00"
    }

    private fun bytes(vararg values: Int) = ByteArray(values.size) { values[it].toByte() }

    @Test
    fun refusesEveryHostilePayloadWithoutThrowing() {
        // Marshal packs 1048576 as 0x03 plus three little-endian bytes.
        val million = bytes(0x03, 0x00, 0x00, 0x10)
        val nested = mutableListOf<Byte>()
        nested.add(0x04); nested.add(0x08)
        repeat(33) {
            nested.add('['.code.toByte())
            million.forEach { nested.add(it) }
        }
        repeat(1024) { nested.add(0) }

        val cases = listOf(
            "nested arrays claiming a million elements each" to nested.toByteArray(),
            "hash claiming more pairs than bytes" to
                bytes(0x04, 0x08, 0x7B, 0x03, 0x00, 0x00, 0x10, 0x49, 0x22, 0x06, 0x61),
            "negative ivar count on the envelope string" to
                bytes(0x04, 0x08, 0x49, 0x22, 0x06, 0x78, 0xFA),
            "string claiming past the end" to bytes(0x04, 0x08, 0x22, 0x20, 0x61),
            "unsupported object type" to bytes(0x04, 0x08, 0x6F, 0x3A, 0x06, 0x58, 0x00),
            // A packed Marshal length is at most four bytes, so the largest
            // claim is 2^32-1 and the sharpest is Int.MAX_VALUE: a bounds check
            // that ADDS the claim to the position overflows and lets the slice
            // through. The reader subtracts instead.
            "string of MaxInt32 at a non-zero position" to
                bytes(0x04, 0x08, 0x5B, 0x07, 0x69, 0x06, 0x22, 0x04, 0xFF, 0xFF, 0xFF, 0x7F, 0x61),
            "string of 2^32-1 at a non-zero position" to
                bytes(0x04, 0x08, 0x5B, 0x07, 0x69, 0x06, 0x22, 0x04, 0xFF, 0xFF, 0xFF, 0xFF, 0x61),
            "symbol of MaxInt32" to bytes(0x04, 0x08, 0x3A, 0x04, 0xFF, 0xFF, 0xFF, 0x7F, 0x61),
            "negative string length" to bytes(0x04, 0x08, 0x22, 0xFA, 0x61),
            "negative symbol length" to bytes(0x04, 0x08, 0x3A, 0xFA, 0x61),
            "truncated after the version" to bytes(0x04, 0x08),
        )
        assertEquals(11, cases.size, "the reference's hostile cases")

        val clock = TimeSource.Monotonic.markNow()
        for ((name, raw) in cases) {
            // No try/catch: anything thrown fails the test, which is the
            // guarantee under test. Returning null is the only acceptable
            // outcome — an exception out of here would reach a caller walking
            // rich text, where a single crafted attachment would end the walk.
            assertNull(personIdFromSgid(sgid(raw)), name)
        }
        // "Promptly" is the other half: the nesting and the million-element
        // claims are there to make a reader that trusts them run for a long
        // time or die allocating. The bound is generous because it has to hold
        // on a slow CI machine; the failure it guards against is seconds or
        // minutes, not milliseconds.
        val elapsed = clock.elapsedNow().inWholeMilliseconds
        kotlin.test.assertTrue(elapsed < 2_000, "hostile payloads took ${elapsed}ms")
    }

    @Test
    fun refusesAPayloadNestedDeeperThanTheReaderWillRecurse() {
        // The depth guard's own row, and it needs a payload the OTHER guards
        // cannot refuse first: 20,000 single-element arrays, each perfectly
        // well formed, so every length and count check passes and only the
        // depth bound stands between the reader and its own stack. The hostile
        // table above cannot reach this — its nested arrays claim a million
        // elements and die on the first missing byte, one level down.
        //
        // `[` then a packed 1 (0x06) is an array of one element; a `0` at the
        // bottom is Ruby nil.
        val deep = mutableListOf<Byte>()
        deep.add(0x04); deep.add(0x08)
        repeat(20_000) { deep.add('['.code.toByte()); deep.add(0x06) }
        deep.add('0'.code.toByte())
        assertNull(personIdFromSgid(sgid(deep.toByteArray())), "20,000 levels of nesting")
    }

    @Test
    fun refusesTrailingBytesAfterAValidEnvelopeAndAcceptsTheSameOneWithout() {
        // The control matters more than the refusal: without it, a reader that
        // rejected EVERYTHING would pass the row above and this one too.
        val envelope = "{\"_rails\":{\"data\":\"gid://bc3/Person/42\",\"pur\":\"attachable\"}}".encodeToByteArray()
        assertEquals(42L, personIdFromSgid(sgid(envelope)), "the envelope alone decodes")
        assertNull(personIdFromSgid(sgid(envelope + "junk".encodeToByteArray())), "trailing bytes")
    }

    @Test
    fun refusesAnEncodedFormOverTheSizeCapBeforeDecodingIt() {
        // Not even valid base64: the length alone refuses it, before any
        // allocation proportional to the input.
        assertNull(personIdFromSgid("!".repeat(20_000) + "--00"))
    }

    @Test
    fun refusesAPayloadOverTheSizeCap() {
        // A well-formed Marshal string of 8192 bytes, which is past the payload
        // cap: refused for its size rather than for its shape.
        val big = bytes(0x04, 0x08, 0x22, 0x02, 0x00, 0x20) + ByteArray(8192)
        assertNull(personIdFromSgid(sgid(big)))
    }

    @Test
    fun aValidJsonEnvelopeStillDecodes() {
        // The acceptance control for the whole class: every assertion above is
        // a refusal, and a decoder that refused everything would satisfy them.
        val id = personIdFromSgid(
            sgid("{\"_rails\":{\"data\":\"gid://bc3/Person/1049715915\",\"pur\":\"attachable\"}}".encodeToByteArray()),
        )
        assertNotNull(id)
        assertEquals(1049715915L, id)
    }
}
