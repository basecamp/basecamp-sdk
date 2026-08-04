package com.basecamp.sdk

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

/**
 * Which decoder flag actually coerces a wrong-typed scalar (#598)?
 *
 * The bug report and the release plan disagreed about the cause — one blamed
 * `coerceInputValues`, the other `isLenient`. They are not interchangeable, and
 * removing the wrong one ships a no-op as a fix. These tests pin the semantics
 * so the answer is in the tree rather than in anyone's recollection:
 *
 *  - `isLenient` relaxes RFC-4627 and accepts unquoted/loosely-typed literals,
 *    so a JSON number or boolean is read into a declared String.
 *  - `coerceInputValues` coerces `null` (and unknown enum values) to the
 *    declared default for a non-nullable property. It has nothing to say about
 *    a number-where-a-String-belongs.
 */
class DecoderStrictnessTest {
    @Serializable
    private data class Probe(val description: String? = null)

    @Serializable
    private data class NonNullProbe(val description: String = "default")

    private val lenient = Json { ignoreUnknownKeys = true; isLenient = true }
    private val coercing = Json { ignoreUnknownKeys = true; coerceInputValues = true }
    private val strict = Json { ignoreUnknownKeys = true }

    // The defect: a number on the wire becomes a String the composite cannot
    // tell from a real one, and then gets PUT back to a full-replace endpoint.
    @Test
    fun lenientCoercesNumberIntoString() {
        assertEquals("42", lenient.decodeFromString<Probe>("""{"description": 42}""").description)
    }

    @Test
    fun lenientCoercesBooleanIntoString() {
        assertEquals("false", lenient.decodeFromString<Probe>("""{"description": false}""").description)
    }

    // The flag the plan blamed. If this threw, `coerceInputValues` would be the
    // culprit and dropping `isLenient` alone would not be enough.
    @Test
    fun coerceInputValuesDoesNotCoerceAScalarType() {
        assertFailsWith<Exception> {
            coercing.decodeFromString<Probe>("""{"description": 42}""")
        }
    }

    @Test
    fun strictRejectsNumberForString() {
        assertFailsWith<Exception> {
            strict.decodeFromString<Probe>("""{"description": 42}""")
        }
    }

    @Test
    fun strictRejectsBooleanForString() {
        assertFailsWith<Exception> {
            strict.decodeFromString<Probe>("""{"description": false}""")
        }
    }

    // What `coerceInputValues` IS for, and therefore what dropping it would
    // change: an explicit null into a non-nullable property with a default.
    @Test
    fun coerceInputValuesRewritesNullToTheDeclaredDefault() {
        assertEquals(
            "default",
            coercing.decodeFromString<NonNullProbe>("""{"description": null}""").description,
        )
    }

    @Test
    fun withoutCoerceInputValuesAnExplicitNullIsRejected() {
        assertFailsWith<Exception> {
            strict.decodeFromString<NonNullProbe>("""{"description": null}""")
        }
    }

    // A nullable property takes an explicit null on every config — this is the
    // shape the SDK's own models use, and it is why dropping coerceInputValues
    // is survivable.
    @Test
    fun nullableFieldsAcceptExplicitNullEverywhere() {
        assertNull(strict.decodeFromString<Probe>("""{"description": null}""").description)
        assertNull(lenient.decodeFromString<Probe>("""{"description": null}""").description)
        assertNull(coercing.decodeFromString<Probe>("""{"description": null}""").description)
    }

    // Structural mismatches were already refused — this is the half #576
    // correctly called safe, and it must stay that way.
    @Test
    fun structuralMismatchIsRejectedEvenWhenLenient() {
        assertFailsWith<Exception> {
            lenient.decodeFromString<Probe>("""{"description": []}""")
        }
        assertFailsWith<Exception> {
            lenient.decodeFromString<Probe>("""{"description": {}}""")
        }
    }
}
