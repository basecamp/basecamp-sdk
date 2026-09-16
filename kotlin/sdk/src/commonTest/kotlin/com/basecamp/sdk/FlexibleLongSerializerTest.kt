package com.basecamp.sdk

import com.basecamp.sdk.generated.models.Person
import com.basecamp.sdk.serialization.FlexibleLongSerializer
import com.basecamp.sdk.serialization.normalizePersonIds
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonPrimitive
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertContains
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlin.test.fail

class FlexibleLongSerializerTest {
    private val json = Json { ignoreUnknownKeys = true }

    @Serializable
    data class Wrapper(
        @Serializable(with = FlexibleLongSerializer::class)
        val id: Long
    )

    @Test
    fun decodesJsonNumber() {
        val result = json.decodeFromString<Wrapper>("""{"id": 12345}""")
        assertEquals(12345L, result.id)
    }

    @Test
    fun decodesNumericString() {
        val result = json.decodeFromString<Wrapper>("""{"id": "12345"}""")
        assertEquals(12345L, result.id)
    }

    @Test
    fun decodesNonNumericSentinelAsZero() {
        val result = json.decodeFromString<Wrapper>("""{"id": "basecamp"}""")
        assertEquals(0L, result.id)
    }

    /**
     * An UNQUOTED bad number, which takes the other branch, and the TYPE of the
     * refusal, which is what this test is for. A `KSerializer` that reports a
     * decode failure in a type kotlinx does not use for decode failures escapes
     * everything downstream that recognizes one — the SDK's SPEC §6 mapping
     * (#604), the §18 composites' re-hint, and the conformance runner's
     * fixture-body policy, all of which read that type.
     *
     * It used to be possible to break: the branch read the literal through
     * `JsonPrimitive.long` and caught the [NumberFormatException] that raises,
     * so removing the catch leaked the numeric type. The catch is gone because
     * the call is: the path now runs Go's own scan, which returns a verdict
     * rather than throwing one, so there is no foreign exception left to convert.
     * The guarantee is therefore structural — but it is still the guarantee, so
     * it stays pinned here rather than being assumed from the shape of the code.
     */
    @Test
    fun rejectsUnquotedFractionalAndOverflowNumbersAsSerializationFailures() {
        for (bad in listOf("1.5", "9223372036854775808", "1e100")) {
            val error = assertFailsWith<SerializationException>(
                "an unquoted $bad must be refused as a serialization failure",
            ) {
                json.decodeFromString<Wrapper>("{\"id\": $bad}")
            }
            assertContains(error.message!!, bad)
        }
    }

    @Test
    fun rejectsNumericOverflowString() {
        assertFailsWith<SerializationException> {
            json.decodeFromString<Wrapper>("""{"id": "9223372036854775808"}""")
        }
    }

    @Test
    fun encodesAsNumber() {
        val encoded = json.encodeToString(Wrapper.serializer(), Wrapper(id = 42L))
        assertEquals("""{"id":42}""", encoded)
    }

    // Response normalization boundary tests

    @Test
    fun normalizeSentinelCreatorId() {
        val input = """{"creator":{"id":"basecamp","name":"Basecamp","personable_type":"LocalPerson"}}"""
        val output = normalizePersonIds(input, json)
        assertContains(output, """"id":0""")
        assertContains(output, """"system_label":"basecamp"""")
    }

    @Test
    fun normalizeNumericStringCreatorId() {
        val input = """{"creator":{"id":"99999","name":"Real","personable_type":"User"}}"""
        val output = normalizePersonIds(input, json)
        assertContains(output, """"id":99999""")
        assertFalse(output.contains("system_label"))
    }

    @Test
    fun normalizeOverflowStringCreatorId() {
        val input = """{"creator":{"id":"9223372036854775808","name":"Overflow","personable_type":"User"}}"""
        val output = normalizePersonIds(input, json)
        // Overflow left as string for FlexibleLongSerializer to reject
        assertContains(output, """"id":"9223372036854775808"""")
    }

    // The person-id grammar corpus, at both sites.
    //
    // Every row below is a MEASURED Go verdict, not one read off the docs: they
    // come from `go/pkg/basecamp/person_id_grammar_test.go`, which is the oracle
    // for this SDK and the six ports held to it, and which can re-derive them
    // (`ORACLE_OUT=... go test ./pkg/basecamp/ -run TestPersonIDOracleDump`).
    //
    // Two of the three id rules in this SDK are supposed to agree and are pinned
    // here: `FlexibleLongSerializer` (the reader) and `normalizePersonIds` (the
    // pre-decode normalizer), both of which are `strconv.ParseInt(s, 10, 64)`.
    // The third — `personIdFromSgid`, which walks the bytes before parsing and
    // so refuses a leading "+" — is deliberately different, and MentionsTest
    // pins that disagreement rather than this file resolving it.
    //
    // Before this corpus existed both sites shared two wrong pieces and diverged
    // in both directions at once, on 12 of the 74 rows each:
    //
    //   - `String.toLongOrNull` goes through `digitOf`, which on JVM is
    //     `Character.digit` and accepts Unicode decimal digits. "１２３" read as
    //     123, "٠١٢" as 12, "৭" as 7, "৭7" and "7৭" as 77, "٠" as 0 — seven rows
    //     where a wire value Go reads as its non-numeric SENTINEL named a real
    //     person instead. The accepting direction, and the worse one.
    //   - `Regex("^-?\\d+$")` refuses the leading "+" ParseInt takes, so
    //     "+9223372036854775808" collapsed to the sentinel (id 0, the SYSTEM
    //     ACTOR) where Go raises a range error; and it gets the scan order
    //     backwards, so "18446744073709551616x", "-18446744073709551616x" and
    //     "99999999999999999999999x" — where Go's in-loop UInt64 overflow fires
    //     before the scan reaches the junk byte — also became the system actor
    //     instead of failing the read.
    private enum class IdOutcome {
        /** ParseInt accepted; the id is that number. */
        VALUE,

        /**
         * ParseInt refused on syntax, so the id is Go's non-numeric sentinel 0
         * and the original string is kept as system_label. 0 is the SYSTEM ACTOR
         * (LocalPerson, "basecamp", "campfire"), which is why a port that lands
         * here for a value Go reads as a number names the wrong actor.
         */
        SENTINEL,

        /**
         * ParseInt refused on range, so the string is left untouched and the
         * read fails rather than a sentinel being substituted.
         */
        REFUSED,
    }

    private data class PersonIdCase(val raw: String, val outcome: IdOutcome, val value: Long = 0L)

    private val personIdCorpus = listOf(
        PersonIdCase("7", IdOutcome.VALUE, 7),
        PersonIdCase("0", IdOutcome.VALUE, 0),
        PersonIdCase("-0", IdOutcome.VALUE, 0),
        PersonIdCase("+0", IdOutcome.VALUE, 0),
        PersonIdCase("+7", IdOutcome.VALUE, 7),
        PersonIdCase("-7", IdOutcome.VALUE, -7),
        PersonIdCase("007", IdOutcome.VALUE, 7),
        PersonIdCase("+007", IdOutcome.VALUE, 7),
        PersonIdCase("-007", IdOutcome.VALUE, -7),
        PersonIdCase("0009223372036854775807", IdOutcome.VALUE, 9223372036854775807),
        PersonIdCase("0000000000000000000000009", IdOutcome.VALUE, 9),
        PersonIdCase("", IdOutcome.SENTINEL),
        PersonIdCase(" ", IdOutcome.SENTINEL),
        PersonIdCase("+", IdOutcome.SENTINEL),
        PersonIdCase("-", IdOutcome.SENTINEL),
        PersonIdCase(" 7", IdOutcome.SENTINEL),
        PersonIdCase("7 ", IdOutcome.SENTINEL),
        PersonIdCase(" 7 ", IdOutcome.SENTINEL),
        PersonIdCase("\n7", IdOutcome.SENTINEL),
        PersonIdCase("7\n", IdOutcome.SENTINEL),
        PersonIdCase("\t7", IdOutcome.SENTINEL),
        PersonIdCase("7\t", IdOutcome.SENTINEL),
        PersonIdCase("1_0", IdOutcome.SENTINEL),
        PersonIdCase("1_2", IdOutcome.SENTINEL),
        PersonIdCase("0x10", IdOutcome.SENTINEL),
        PersonIdCase("0b11", IdOutcome.SENTINEL),
        PersonIdCase("0o17", IdOutcome.SENTINEL),
        // TEN, not eight. Ruby's Integer() detected octal and said eight — not a
        // refusal against an acceptance, but two different people from one wire
        // value, either of which a caller could then mention.
        PersonIdCase("010", IdOutcome.VALUE, 10),
        PersonIdCase("0X1F", IdOutcome.SENTINEL),
        PersonIdCase("7x", IdOutcome.SENTINEL),
        PersonIdCase("x7", IdOutcome.SENTINEL),
        PersonIdCase("12.0", IdOutcome.SENTINEL),
        PersonIdCase("1e3", IdOutcome.SENTINEL),
        PersonIdCase("12,3", IdOutcome.SENTINEL),
        PersonIdCase("basecamp", IdOutcome.SENTINEL),
        PersonIdCase("campfire", IdOutcome.SENTINEL),
        PersonIdCase("LocalPerson", IdOutcome.SENTINEL),
        // Unicode decimal digits: ParseInt takes ASCII 0-9 and nothing else.
        PersonIdCase("１２３", IdOutcome.SENTINEL),
        PersonIdCase("７", IdOutcome.SENTINEL),
        PersonIdCase("٠١٢", IdOutcome.SENTINEL),
        PersonIdCase("৭", IdOutcome.SENTINEL),
        PersonIdCase("۷", IdOutcome.SENTINEL),
        PersonIdCase("9223372036854775806", IdOutcome.VALUE, 9223372036854775806),
        PersonIdCase("9223372036854775807", IdOutcome.VALUE, 9223372036854775807),
        PersonIdCase("9223372036854775808", IdOutcome.REFUSED),
        PersonIdCase("9223372036854775809", IdOutcome.REFUSED),
        PersonIdCase("-9223372036854775807", IdOutcome.VALUE, -9223372036854775807),
        PersonIdCase("-9223372036854775808", IdOutcome.VALUE, Long.MIN_VALUE),
        PersonIdCase("-9223372036854775809", IdOutcome.REFUSED),
        PersonIdCase("18446744073709551614", IdOutcome.REFUSED),
        PersonIdCase("18446744073709551615", IdOutcome.REFUSED),
        PersonIdCase("18446744073709551616", IdOutcome.REFUSED),
        // The scan-order pair, one digit apart and on opposite sides of the
        // boundary: ParseUint checks the magnitude INSIDE the loop, so the
        // second overflows UInt64 before the scan ever reaches the junk byte.
        PersonIdCase("18446744073709551615x", IdOutcome.SENTINEL),
        PersonIdCase("18446744073709551616x", IdOutcome.REFUSED),
        PersonIdCase("1844674407370955161x", IdOutcome.SENTINEL),
        PersonIdCase("-18446744073709551615x", IdOutcome.SENTINEL),
        PersonIdCase("-18446744073709551616x", IdOutcome.REFUSED),
        PersonIdCase("99999999999999999999999", IdOutcome.REFUSED),
        PersonIdCase("99999999999999999999999x", IdOutcome.REFUSED),
        PersonIdCase("00000000000000000000018446744073709551616", IdOutcome.REFUSED),
        PersonIdCase("0000000000000000000009223372036854775807", IdOutcome.VALUE, 9223372036854775807),
        // 2^53 and its neighbours: real int64 ids a JavaScript number cannot hold.
        PersonIdCase("9007199254740991", IdOutcome.VALUE, 9007199254740991),
        PersonIdCase("9007199254740992", IdOutcome.VALUE, 9007199254740992),
        PersonIdCase("9007199254740993", IdOutcome.VALUE, 9007199254740993),
        PersonIdCase("90071992547409931", IdOutcome.VALUE, 90071992547409931),
        PersonIdCase("-9007199254740993", IdOutcome.VALUE, -9007199254740993),
        PersonIdCase("+9223372036854775807", IdOutcome.VALUE, 9223372036854775807),
        PersonIdCase("+9223372036854775808", IdOutcome.REFUSED),
        PersonIdCase("00", IdOutcome.VALUE, 0),
        PersonIdCase("0000", IdOutcome.VALUE, 0),
        PersonIdCase("-00", IdOutcome.VALUE, 0),
        PersonIdCase("٠", IdOutcome.SENTINEL),
        PersonIdCase("৭7", IdOutcome.SENTINEL),
        PersonIdCase("7৭", IdOutcome.SENTINEL),
    )

    /** The raw string as a JSON string literal, escaped as the wire would carry it. */
    private fun quoted(raw: String): String = JsonPrimitive(raw).toString()

    @Test
    fun flexibleLongReadsEveryCorpusRowAsGoDoes() {
        assertEquals(74, personIdCorpus.size, "the oracle's corpus is 74 rows")
        for (case in personIdCorpus) {
            val body = """{"id":${quoted(case.raw)}}"""
            when (case.outcome) {
                IdOutcome.VALUE -> assertEquals(
                    case.value,
                    json.decodeFromString<Wrapper>(body).id,
                    "FlexibleLong(${quoted(case.raw)})",
                )

                IdOutcome.SENTINEL -> assertEquals(
                    0L,
                    json.decodeFromString<Wrapper>(body).id,
                    "FlexibleLong(${quoted(case.raw)}) must be the sentinel 0",
                )

                IdOutcome.REFUSED -> assertFailsWith<SerializationException>(
                    "FlexibleLong(${quoted(case.raw)}) must fail the read, not substitute a sentinel",
                ) { json.decodeFromString<Wrapper>(body) }
            }
        }
    }

    /**
     * The property card 34 asked for, in Kotlin: the pre-decode normalizer
     * accepts exactly what [FlexibleLongSerializer] accepts, reads it as the
     * same number, and fails the read exactly where that serializer throws.
     *
     * Run the way [com.basecamp.sdk.services.BaseService] runs it — normalize
     * the body text, then decode it onto the model — so a disagreement between
     * the two layers shows up as the wrong person, or a lost read, rather than
     * as a unit-test detail about one of them.
     */
    @Test
    fun normalizePersonIdsAgreesWithTheReaderOnEveryCorpusRow() {
        for (case in personIdCorpus) {
            val body = """{"creator":{"id":${quoted(case.raw)},"name":"x","personable_type":"User"}}"""
            val normalized = normalizePersonIds(body, json)
            when (case.outcome) {
                IdOutcome.VALUE -> {
                    val creator = json.decodeFromString<CreatorEnvelope>(normalized).creator
                    assertEquals(case.value, creator.id, "normalized ${quoted(case.raw)}")
                    assertNull(
                        creator.systemLabel,
                        "a numeric id ${quoted(case.raw)} must not be labelled a sentinel",
                    )
                }

                IdOutcome.SENTINEL -> {
                    val creator = json.decodeFromString<CreatorEnvelope>(normalized).creator
                    assertEquals(0L, creator.id, "normalized ${quoted(case.raw)} must be the sentinel 0")
                    assertEquals(
                        case.raw,
                        creator.systemLabel,
                        "the original string must survive as system_label",
                    )
                }

                // The string is LEFT ALONE — no sentinel substituted — so the
                // decoder is what refuses. That is the difference between a
                // failed read and an oversized id silently becoming the system
                // actor, and it is the direction that matters.
                IdOutcome.REFUSED -> assertFailsWith<SerializationException>(
                    "the wrapper path must fail the read for ${quoted(case.raw)}",
                ) { json.decodeFromString<CreatorEnvelope>(normalized) }
            }
        }
    }

    /**
     * A `system_label` the BODY carries must never survive a sentinel id.
     *
     * The normalizer rebuilds the object by replaying its members in order, and a
     * JSON object's members ARE ordered, so a `system_label` spelled after `id`
     * used to be copied over the one the sentinel branch had just written. The
     * reference cannot have that bug and so has no branch for it: `coercePersonID`
     * assigns into a map (`go/pkg/basecamp/normalize.go:66-67`), and the label it
     * writes wins however the body was spelled.
     *
     * It matters because the value being overwritten came off the wire. A response
     * that carries its own `system_label` could otherwise choose the label this SDK
     * reports for the system actor, which is the one field a caller reads to find
     * out WHICH actor it was handed. Both orders are pinned, because only one of
     * them was ever wrong and a test holding the other proves nothing.
     */
    @Test
    fun aSentinelIdOverwritesAnyIncomingSystemLabelInEitherOrder() {
        for (body in listOf(
            """{"creator":{"id":"basecamp","system_label":"spoofed","name":"x","personable_type":"User"}}""",
            """{"creator":{"system_label":"spoofed","id":"basecamp","name":"x","personable_type":"User"}}""",
        )) {
            val creator = json.decodeFromString<CreatorEnvelope>(normalizePersonIds(body, json)).creator
            assertEquals(0L, creator.id, "the sentinel id")
            assertEquals("basecamp", creator.systemLabel, "the label must be the id's own text, not the body's")
        }
    }

    /**
     * The other two outcomes leave `system_label` alone, which is also the
     * reference's behaviour: it is assigned on the syntax path and nowhere else.
     * Pinned so the fix above cannot be widened into dropping the key outright.
     */
    @Test
    fun anIncomingSystemLabelSurvivesAValueAndARangeRefusal() {
        val value = json.decodeFromString<CreatorEnvelope>(
            normalizePersonIds(
                """{"creator":{"id":"7","system_label":"kept","name":"x","personable_type":"User"}}""",
                json,
            ),
        ).creator
        assertEquals(7L, value.id)
        assertEquals("kept", value.systemLabel)

        val refused = normalizePersonIds(
            """{"creator":{"id":"9223372036854775808","system_label":"kept","name":"x","personable_type":"User"}}""",
            json,
        )
        assertTrue(refused.contains("\"kept\""), "a range refusal must not touch system_label")
    }

    // The NUMBER path: an unquoted literal where a person id belongs.
    //
    // It has only TWO outcomes, and that is the point. The sentinel belongs to
    // the quoted path alone — "basecamp" is a string — so a bare literal is
    // either an integer in range or a malformed body. Answering 0 for one is not
    // a neutral failure: 0 is the SYSTEM ACTOR (LocalPerson, "basecamp",
    // "campfire"), on the one field that says who acted.
    //
    // Before this corpus existed the path diverged from Go on 7 of the 25 rows
    // below, all in the accepting direction:
    //
    //   - `[7]`, `[]`, `{"a":7}`, `{}` fell off the end of `deserialize` to a
    //     trailing `return 0L`. Go decodes the number path into a `json.Number`
    //     through `encoding/json`, and neither shape unmarshals into one, so both
    //     fail the read there.
    //   - `1e3` and `1E3` read 1000 through `JsonPrimitive.long`, which in
    //     kotlinx 1.11.0 accepts an exponent. `json.Number.Int64()` is
    //     `strconv.ParseInt(text, 10, 64)` on the token verbatim, which does not.
    //   - `007` read 7. It is not valid JSON at all: `json.Unmarshal` scans the
    //     document for validity before it calls any `UnmarshalJSON`, so the whole
    //     read fails in Go. kotlinx's number lexer is lenient about token shape
    //     and hands the literal through, so the grammar check has to be here.
    //
    // `+7`, `null`, `true`, `7.5` and both int64 boundaries were already right,
    // and are pinned so the fix cannot drift off them.
    private data class BareCase(val literal: String, val outcome: IdOutcome, val value: Long = 0L)

    private val bareNumberCorpus = listOf(
        BareCase("7", IdOutcome.VALUE, 7),
        BareCase("-7", IdOutcome.VALUE, -7),
        BareCase("0", IdOutcome.VALUE, 0),
        // Valid JSON, and `ParseInt` reads it as zero.
        BareCase("-0", IdOutcome.VALUE, 0),
        // Not a float anywhere in this path: 2^53 and its neighbours are real
        // int64 ids, and a conversion through Double would round them.
        BareCase("9007199254740991", IdOutcome.VALUE, 9007199254740991),
        BareCase("9007199254740992", IdOutcome.VALUE, 9007199254740992),
        BareCase("9007199254740993", IdOutcome.VALUE, 9007199254740993),
        BareCase("9223372036854775807", IdOutcome.VALUE, 9223372036854775807),
        BareCase("-9223372036854775808", IdOutcome.VALUE, Long.MIN_VALUE),
        // Range: the token is well formed and does not fit 64 bits.
        BareCase("9223372036854775808", IdOutcome.REFUSED),
        BareCase("-9223372036854775809", IdOutcome.REFUSED),
        BareCase("99999999999999999999999", IdOutcome.REFUSED),
        // Not integer tokens. `1e3` is the one that read 1000.
        BareCase("1e3", IdOutcome.REFUSED),
        BareCase("1E3", IdOutcome.REFUSED),
        BareCase("1e100", IdOutcome.REFUSED),
        BareCase("7.5", IdOutcome.REFUSED),
        BareCase("1.0", IdOutcome.REFUSED),
        BareCase("-0.0", IdOutcome.REFUSED),
        // Shapes kotlinx's lenient number lexer admits and JSON's grammar does
        // not, so the reference never gets them as far as `ParseInt` — which
        // would have taken both.
        BareCase("007", IdOutcome.REFUSED),
        BareCase("-007", IdOutcome.REFUSED),
        BareCase("+7", IdOutcome.REFUSED),
        // Literals that are not numbers at all. `null` is NOT the absent id —
        // see [anAbsentIdNeverReachesThisSerializer].
        BareCase("null", IdOutcome.REFUSED),
        BareCase("true", IdOutcome.REFUSED),
        BareCase("false", IdOutcome.REFUSED),
        // Not primitives at all: the rows that answered the system actor.
        BareCase("[7]", IdOutcome.REFUSED),
        BareCase("[]", IdOutcome.REFUSED),
        BareCase("""{"a":7}""", IdOutcome.REFUSED),
        BareCase("{}", IdOutcome.REFUSED),
    )

    @Test
    fun theNumberPathReadsEveryBareLiteralAsGoDoes() {
        for (case in bareNumberCorpus) {
            val body = """{"id": ${case.literal}}"""
            when (case.outcome) {
                IdOutcome.VALUE -> assertEquals(
                    case.value,
                    json.decodeFromString<Wrapper>(body).id,
                    "bare ${case.literal}",
                )

                IdOutcome.REFUSED -> assertFailsWith<SerializationException>(
                    "bare ${case.literal} must fail the read, not answer the system actor",
                ) { json.decodeFromString<Wrapper>(body) }

                // The sentinel is the quoted path's alone. A row claiming one
                // here would mean someone taught this path to invent an actor.
                IdOutcome.SENTINEL -> fail("the number path has no sentinel outcome")
            }
        }
    }

    /**
     * The same verdicts through the wrapper path, as `BaseService` runs it. The
     * normalizer only rewrites a STRING id, so every row here reaches the
     * decoder untouched — which is the property worth pinning: a malformed
     * embedded person fails the read rather than being normalized into one.
     */
    @Test
    fun theNumberPathIsTheSameThroughTheNormalizer() {
        for (case in bareNumberCorpus) {
            val body = """{"creator":{"id": ${case.literal},"name":"x","personable_type":"User"}}"""
            val normalized = normalizePersonIds(body, json)
            when (case.outcome) {
                IdOutcome.VALUE -> {
                    val creator = json.decodeFromString<CreatorEnvelope>(normalized).creator
                    assertEquals(case.value, creator.id, "normalized bare ${case.literal}")
                    assertNull(creator.systemLabel, "a bare literal never earns a system_label")
                }

                IdOutcome.REFUSED -> assertFailsWith<SerializationException>(
                    "the wrapper path must fail the read for bare ${case.literal}",
                ) { json.decodeFromString<CreatorEnvelope>(normalized) }

                IdOutcome.SENTINEL -> fail("the number path has no sentinel outcome")
            }
        }
    }

    /**
     * A `null` id is not an absent one, and the two differ in the reference for a
     * reason worth restating: `encoding/json` calls `UnmarshalJSON` for a null,
     * so `FlexibleInt64`'s number path decodes it into an empty `json.Number` and
     * `ParseInt("")` fails — `{"id": null}` FAILS THE READ. A missing key never
     * reaches the decoder at all and is the zero value with no error. A plain
     * `int64` field has no `UnmarshalJSON`, so json handles its null itself and
     * both are 0; only a flexible field tells them apart.
     * `ruby/lib/basecamp/ids.rb`'s `person_from_wire` documents that asymmetry,
     * measured through the real decode path.
     *
     * Kotlin gets the null half right by a different route (the literal's text is
     * "null", which is not an integer token) and is pinned above. The absent half
     * it does NOT match, and the divergence is in the model rather than here:
     * `Person.id` is a required field with no default, so an absent id is a
     * `MissingFieldException` where Go reads 0. That is a decode failure — a
     * `SerializationException`, so `BaseService` still renders it as a
     * malformed body — which means the divergence is in the REFUSING direction:
     * the SDK declines to invent a person rather than naming actor 0. Pinned as
     * measured, not as wished, so a reader finds the difference instead of
     * discovering it.
     */
    @Test
    fun anAbsentIdNeverReachesThisSerializer() {
        val failure = assertFailsWith<SerializationException> {
            json.decodeFromString<Person>("""{"name":"x"}""")
        }
        assertContains(failure.message!!, "id")

        val nulled = assertFailsWith<SerializationException> {
            json.decodeFromString<Person>("""{"id": null,"name":"x"}""")
        }
        assertContains(nulled.message!!, "null")
    }

    /**
     * The refusal names the SHAPE, never its content. An id that arrives as an
     * object is an embedded person, and an embedded person carries a name and an
     * email address; this message reaches logs through the SPEC §6 malformed-body
     * error.
     */
    @Test
    fun theRefusalForAnObjectIdDoesNotCarryItsContents() {
        val failure = assertFailsWith<SerializationException> {
            json.decodeFromString<Wrapper>("""{"id": {"email_address":"person@example.com"}}""")
        }
        assertFalse(
            failure.message!!.contains("person@example.com"),
            "the decode failure must not quote the body: ${failure.message}",
        )
        assertContains(failure.message!!, "object")
    }

    @Serializable
    private data class CreatorEnvelope(val creator: Person)
}
