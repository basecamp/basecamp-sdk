package com.basecamp.sdk

import kotlin.test.Test
import kotlin.test.assertEquals
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
 * **What these rows kill and what they do not, measured by mutating the reader
 * and re-running rather than by reading the assertions.** Every row below was
 * run against each mutation with `--rerun-tasks`, because a Gradle task
 * reported UP-TO-DATE reports nothing at all.
 *
 * Killed (each verified by making the change and re-running, not by reading
 * the assertions):
 *
 *  - `readWhole` returning null unconditionally — the acceptance control. An
 *    earlier version of this class built its controls from JSON envelopes, so
 *    the Marshal reader was never constructed and this mutation survived every
 *    row HERE. It did not survive the suite: `MentionsTest` decodes the same
 *    envelope. What was missing was this class being able to see its own
 *    subject disappear, which is why the controls below are Marshal.
 *  - the "a dump is exactly one value" check — the trailing-bytes row. No other
 *    row in the KOTLIN suite reaches it; the reference covers it, which is
 *    where the case came from.
 *  - the count bound and the length bound removed TOGETHER. The first row to
 *    throw is "string claiming past the end", whose `copyOfRange` runs off a
 *    five-byte array long before the MaxInt32 rows can allocate anything.
 *
 * Not killed, and each for a reason worth knowing:
 *
 *  - the count bound alone, and the length bound alone: whichever survives
 *    refuses next. Neither line is pinned; the pair is.
 *  - the payload cap alone: the 4097-byte row falls through to the
 *    trailing-bytes check, which refuses it for a different reason.
 *  - the encoded cap alone: the row that names it is 20,000 `!` characters,
 *    which the base64 decoder refuses on the first byte. Nothing here reaches
 *    the encoded cap as the deciding check.
 *  - the nesting bound. It is REACHED — it trips at depth 33 and 34 nested
 *    single-element arrays is 71 bytes, far under both caps — but reaching it
 *    changes no verdict: a nested array is not a person envelope whether the
 *    reader refuses it or returns it. Nor can removing it crash anything here;
 *    the 4096-byte cap admits about two thousand levels, well under what would
 *    exhaust a stack. The row below walks that path and pins nothing, and its
 *    name says so.
 *  - the negative-ivar row, ported from the reference for completeness. Remove
 *    the `n < 0` half of the count bound and `repeat(-1)` is a no-op, the reader
 *    resumes mid-value and fails on a non-string hash key — still null, still
 *    green. It documents a shape; it does not guard one.
 *
 * An earlier version of this note called the nesting bound unreachable because
 * the cap bounds recursion first. That inverted the arithmetic: 2048 is the
 * cap's limit, 33 is the guard's, and the smaller one binds. The number was
 * right and the conclusion drawn from it was backwards.
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

    private fun hex(s: String) = ByteArray(s.length / 2) { s.substring(it * 2, it * 2 + 2).toInt(16).toByte() }

    /**
     * BC3's own Marshal envelope for person 1049715914 — the older Rails layout
     * `{"gid" => …, "purpose" => "attachable", "expires_at" => nil}`, byte for
     * byte as `MentionsTest.victorSgid` carries it.
     *
     * The refusals here have to be built on a MARSHAL envelope rather than a
     * JSON one, or the reader this class is about is never constructed: an
     * envelope opening `{` takes the JSON branch, and an earlier version of
     * these controls did exactly that — every row green with a reader that
     * returned null unconditionally.
     */
    private val marshalEnvelope = hex(
        "04087b08492208676964063a06455449222b6769643a2f2f6263332f506572736f6e2f31" +
            "3034393731353931343f657870697265735f696e063b005449220c707572706f7365063b" +
            "005449220f61747461636861626c65063b005449220f657870697265735f6174063b0054" +
            "30",
    )

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
    fun walksTheNestingPathOnBothSidesOfTheDepthBoundWithoutPinningIt() {
        // `[` then a packed 1 (0x06) is an array of one element; a `0` at the
        // bottom is Ruby nil. 34 levels is 71 bytes — past the guard's 32 and
        // far under both caps, so this is the depth path rather than the size
        // path. An earlier version used 20,000 levels and was refused for its
        // size a layer earlier, which is what made the guard look unreachable.
        fun nested(levels: Int): ByteArray {
            val out = mutableListOf<Byte>(0x04, 0x08)
            repeat(levels) { out.add('['.code.toByte()); out.add(0x06) }
            out.add('0'.code.toByte())
            return out.toByteArray()
        }
        assertEquals(71, nested(34).size, "under both caps, so the size path is not what refuses it")
        assertNull(personIdFromSgid(sgid(nested(34))), "34 levels")
        // And 32 levels, which the guard admits: still nobody, because a nested
        // array is not a person envelope. That is the honest limit of this row
        // — it walks the path, it cannot tell the guard's two sides apart.
        assertNull(personIdFromSgid(sgid(nested(32))), "32 levels is not a person either")
    }

    @Test
    fun refusesTrailingBytesAfterAValidMarshalEnvelopeAndAcceptsTheSameOneWithout() {
        // The control matters more than the refusal: without it, a reader that
        // rejected EVERYTHING would pass the row above and this one too. And it
        // has to be the Marshal envelope, so that `readWhole`'s "a dump is
        // exactly one value" check is the thing being exercised rather than the
        // JSON parser's own end-of-input strictness.
        assertEquals(1049715914L, personIdFromSgid(sgid(marshalEnvelope)), "the envelope alone decodes")
        assertNull(personIdFromSgid(sgid(marshalEnvelope + "junk".encodeToByteArray())), "trailing bytes")
    }

    @Test
    fun aNegativeIvarCountDoesNotSlipAValidEnvelopeThrough() {
        // Ruby's own envelope with one byte changed: the ivar count that follows
        // the "gid" key string, 0x06 (one), replaced by 0xfa (minus one). The
        // reader must refuse it rather than read zero ivars and carry on.
        val tampered = marshalEnvelope.copyOf()
        assertEquals(0x06, tampered[10].toInt(), "fixture layout changed: byte 10 is the ivar count")
        tampered[10] = 0xFA.toByte()
        assertNull(personIdFromSgid(sgid(tampered)), "a negative ivar count")
    }

    @Test
    fun refusesAnEncodedFormOverTheSizeCapBeforeDecodingIt() {
        // Not even valid base64: the length alone refuses it, before any
        // allocation proportional to the input.
        assertNull(personIdFromSgid("!".repeat(20_000) + "--00"))
    }

    @Test
    fun refusesAPayloadOverTheSizeCapRatherThanOverTheEncodedOne() {
        // There are two caps and the encoded one fires first for anything much
        // too big, so a row has to be sized deliberately to reach the payload
        // check. An 8KB payload — the obvious choice — never gets there.
        //
        // The margin is ZERO, not one: 4097 bytes encode to exactly 5464
        // characters, which is exactly `MAX_SGID_ENCODED_BYTES`, and this row
        // survives that check only because it is written `>` rather than `>=`.
        // So the encoded length is asserted rather than assumed — tighten that
        // comparison and this row fails loudly instead of quietly ceasing to
        // exercise the cap it names.
        val big = bytes(0x04, 0x08, 0x22) + ByteArray(4094)
        assertEquals(4097, big.size)
        assertEquals(5464, sgid(big).length - "--00".length, "exactly at the encoded cap, not under it")
        assertNull(personIdFromSgid(sgid(big)), "one byte past the payload cap")
    }

    @Test
    fun theAcceptanceControlsForBothEnvelopeShapes() {
        // Every other assertion in this class is a refusal, and a decoder that
        // refused everything would satisfy all of them. The Marshal control is
        // the one that matters here, because every refusal in this class is a
        // Marshal refusal; the JSON control is kept beside it so the two
        // branches cannot be confused again, which is what went wrong when
        // these rows were JSON and the hostile table was not.
        assertEquals(1049715914L, personIdFromSgid(sgid(marshalEnvelope)), "marshal")
        val json = "{\"_rails\":{\"data\":\"gid://bc3/Person/1049715915\",\"pur\":\"attachable\"}}"
        assertEquals(1049715915L, personIdFromSgid(sgid(json.encodeToByteArray())), "json")
    }
}
