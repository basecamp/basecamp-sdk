package com.basecamp.sdk

import com.basecamp.sdk.serialization.FlexibleIntSerializer
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFailsWith
import kotlin.test.assertNull

class FlexibleIntSerializerTest {
    private val json = Json { ignoreUnknownKeys = true }

    @Serializable
    data class Wrapper(
        @Serializable(with = FlexibleIntSerializer::class)
        val width: Int? = null
    )

    @Test
    fun decodesJsonInteger() {
        assertEquals(1024, json.decodeFromString<Wrapper>("""{"width": 1024}""").width)
    }

    @Test
    fun decodesFloatSpelledInteger() {
        // The BC3 API serializes pixel dimensions float-spelled (1024.0).
        assertEquals(1024, json.decodeFromString<Wrapper>("""{"width": 1024.0}""").width)
    }

    @Test
    fun decodesNumericString() {
        assertEquals(1024, json.decodeFromString<Wrapper>("""{"width": "1024"}""").width)
    }

    @Test
    fun decodesNullAsNull() {
        // A non-image blob's null dimension stays null, not a sentinel 0.
        assertNull(json.decodeFromString<Wrapper>("""{"width": null}""").width)
    }

    @Test
    fun decodesAbsentAsNull() {
        assertNull(json.decodeFromString<Wrapper>("""{}""").width)
    }

    @Test
    fun rejectsFractionalValue() {
        assertFailsWith<SerializationException> {
            json.decodeFromString<Wrapper>("""{"width": 1024.5}""")
        }
    }

    @Test
    fun rejectsOverflow() {
        // 2^31 overflows Int (max 2147483647).
        assertFailsWith<SerializationException> {
            json.decodeFromString<Wrapper>("""{"width": 2147483648}""")
        }
    }

    @Test
    fun encodesAsNumber() {
        assertEquals("""{"width":42}""", json.encodeToString(Wrapper.serializer(), Wrapper(width = 42)))
    }
}
