package com.basecamp.sdk

import com.basecamp.sdk.serialization.FlexibleLongSerializer
import com.basecamp.sdk.serialization.normalizePersonIds
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.Json
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertContains
import kotlin.test.assertFailsWith
import kotlin.test.assertFalse

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
}
