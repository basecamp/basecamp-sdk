package com.basecamp.sdk.serialization

import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerializationException
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.JsonDecoder
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.long

/**
 * A serializer for Long fields that flexibly handles both JSON numbers and strings.
 *
 * The Basecamp API sometimes returns person IDs as strings (e.g. `"12345"`)
 * instead of numbers, and uses non-numeric sentinels like `"basecamp"` for
 * system-generated entities. This serializer handles all three wire formats:
 *
 * - JSON number `12345` → `12345L`
 * - JSON string `"12345"` → `12345L`
 * - JSON string `"basecamp"` → `0L` (non-numeric sentinel)
 * - JSON string `"9223372036854775808"` → throws (numeric overflow)
 */
object FlexibleLongSerializer : KSerializer<Long> {
    override val descriptor: SerialDescriptor =
        PrimitiveSerialDescriptor("FlexibleLong", PrimitiveKind.LONG)

    override fun serialize(encoder: Encoder, value: Long) {
        encoder.encodeLong(value)
    }

    override fun deserialize(decoder: Decoder): Long {
        val jsonDecoder = decoder as? JsonDecoder
            ?: return decoder.decodeLong()

        val element = jsonDecoder.decodeJsonElement()
        if (element is JsonPrimitive) {
            if (element.isString) {
                val s = element.content
                return s.toLongOrNull()
                    ?: if (Regex("^-?\\d+$").matches(s)) {
                        throw SerializationException("FlexibleLong: \"$s\" overflows Long")
                    } else {
                        0L // non-numeric sentinel
                    }
            }
            return element.long
        }
        return 0L
    }
}
