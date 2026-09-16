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
 *
 * The string path is `strconv.ParseInt(s, 10, 64)` and nothing looser — see
 * [parseInt64] for the grammar, for which refusal Go turns into `0` and which
 * it raises, and for why the two cannot be told apart without walking the
 * string the way Go walks it. `go/pkg/types/flexible_int64.go:34-47` is the
 * line being matched; the same scan runs one layer earlier in
 * [normalizePersonIds], so the two agree on every value.
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
                return when (val parsed = parseInt64(s)) {
                    is ParsedInt64.Value -> parsed.value
                    ParsedInt64.Syntax -> 0L // non-numeric sentinel
                    ParsedInt64.Range ->
                        throw SerializationException("FlexibleLong: \"$s\" overflows Long")
                }
            }
            // `JsonPrimitive.long` is `content.toLong()`, which raises
            // NumberFormatException — not SerializationException — on an
            // unquoted fractional or out-of-range literal. Deserialization
            // failures are supposed to speak kotlinx's own type: the SDK maps
            // that one to the SPEC §6 malformed-body error (#604), and the §18
            // composites and the conformance runner both read the mapped
            // error's `cause` to tell a decoder rejection from a real API
            // failure. Leaking the numeric type escapes all three.
            return try {
                element.long
            } catch (e: NumberFormatException) {
                throw SerializationException("FlexibleLong: ${element.content} is not a Long", e)
            }
        }
        return 0L
    }
}
