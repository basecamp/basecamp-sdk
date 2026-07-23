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
import kotlin.math.truncate

/**
 * A serializer for Int fields that flexibly handles the BC3 API's
 * float-encoded integer dimensions (e.g. `1024.0`). kotlinx's default Int
 * decoder rejects a JSON number carrying a fractional token, so pixel
 * dimensions like a rich-text attachment's `width`/`height` — which the API
 * may serialize float-spelled — need this bridge. Mirrors
 * [FlexibleLongSerializer] and matches Go's `types.FlexInt`.
 *
 * - JSON number `1024` → `1024`
 * - JSON number `1024.0` → `1024`
 * - JSON string `"1024"` → `1024`
 * - JSON number `1024.5` → throws (non-integral)
 * - out-of-Int-range → throws
 *
 * A JSON `null` is handled by the nullable field wrapper (`Int?`) before this
 * serializer is invoked, so a null dimension stays null rather than being
 * coerced to a sentinel 0.
 */
object FlexibleIntSerializer : KSerializer<Int> {
    override val descriptor: SerialDescriptor =
        PrimitiveSerialDescriptor("FlexibleInt", PrimitiveKind.INT)

    override fun serialize(encoder: Encoder, value: Int) {
        encoder.encodeInt(value)
    }

    override fun deserialize(decoder: Decoder): Int {
        val jsonDecoder = decoder as? JsonDecoder
            ?: return decoder.decodeInt()

        val element = jsonDecoder.decodeJsonElement()
        if (element is JsonPrimitive) {
            val d = element.content.toDoubleOrNull()
                ?: throw SerializationException("FlexibleInt: ${element.content} is not a number")
            if (d != truncate(d)) {
                throw SerializationException("FlexibleInt: $d is not an integer")
            }
            if (d < Int.MIN_VALUE.toDouble() || d > Int.MAX_VALUE.toDouble()) {
                throw SerializationException("FlexibleInt: $d overflows Int")
            }
            return d.toInt()
        }
        throw SerializationException("FlexibleInt: expected a JSON number")
    }
}
