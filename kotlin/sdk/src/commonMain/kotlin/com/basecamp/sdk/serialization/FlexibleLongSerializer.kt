package com.basecamp.sdk.serialization

import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerializationException
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonDecoder
import kotlinx.serialization.json.JsonPrimitive

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
 * - anything else — `null`, `true`, `1e3`, `7.5`, an array, an object → throws
 *
 * The sentinel is the QUOTED path's alone. An unquoted value that is not an
 * integer token in range is a malformed body and fails the read, because 0 is
 * not a neutral answer: it is the system actor. See [longFromLiteral].
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

        // NOT A PRIMITIVE AT ALL — an array or an object where an id belongs.
        // The reference's number path decodes into a `json.Number` through
        // `encoding/json` (`go/pkg/types/flexible_int64.go:52-58`), and neither
        // shape unmarshals into one ("cannot unmarshal array into Go value of
        // type json.Number"), so both FAIL THE READ there. This used to fall off
        // the end of the function to `return 0L`, which is not a neutral answer:
        // 0 is the SYSTEM ACTOR — LocalPerson, "basecamp", "campfire" — so a
        // malformed body silently named an actor on the one field that says who
        // acted. Its sibling [FlexibleIntSerializer] already refuses this shape.
        if (element !is JsonPrimitive) {
            val kind = if (element is JsonArray) "array" else "object"
            // The shape, never its content: an embedded person object carries a
            // name and an email address, and this message reaches logs.
            throw SerializationException("FlexibleLong: a JSON $kind is not a Long")
        }

        if (element.isString) {
            val s = element.content
            return when (val parsed = parseInt64(s)) {
                is ParsedInt64.Value -> parsed.value
                ParsedInt64.Syntax -> 0L // non-numeric sentinel
                ParsedInt64.Range ->
                    throw SerializationException("FlexibleLong: \"$s\" overflows Long")
            }
        }

        return longFromLiteral(element.content)
    }

    /**
     * The unquoted path: a bare JSON literal where a person id belongs.
     *
     * Two stages, because the reference is two stages. `encoding/json` validates
     * the token against the JSON number grammar before it ever calls
     * `UnmarshalJSON` — `json.Unmarshal` scans the whole document for validity
     * first — and only then does `FlexibleInt64` hand the token to
     * `json.Number.Int64()` (`go/pkg/types/flexible_int64.go:52-62`), which is
     * `strconv.ParseInt(text, 10, 64)`. So the accepted set is the intersection:
     * a JSON *integer* token, in `Int64` range. Everything else fails the read.
     *
     * There is no sentinel on this path, and that asymmetry is the reference's:
     * `"basecamp"` is a string, so only the quoted path can answer `0` without
     * having read a number. A bare literal that is not an integer is a malformed
     * body, not a system actor.
     *
     * Both stages are load-bearing, in opposite directions:
     *
     * - Without the grammar check, [parseInt64] would ACCEPT what the reference's
     *   lexer refuses. kotlinx's number lexer is lenient about token shape — it
     *   hands back `+7` and `007` as literals, measured — while `strconv.ParseInt`
     *   takes a leading `+` and leading zeros. `{"id": 007}` is not valid JSON at
     *   all and fails the whole read in Go; reading it as 7 here would be a person
     *   invented from a malformed body. (QUOTED `"007"` is still 7 — that one
     *   reaches `ParseInt` in the reference too. The asymmetry is real.)
     * - Without [parseInt64], the grammar check alone says nothing about range,
     *   and `9223372036854775808` would have to be converted by something that
     *   either throws a foreign exception or, worse, saturates.
     *
     * `JsonPrimitive.long` served as the second stage and cannot: in kotlinx
     * 1.11.0 it reads `1e3` as 1000 (measured), where `json.Number.Int64()` calls
     * `ParseInt` on the token verbatim and refuses an exponent. The comment it
     * carried was right about one thing that still holds and is why nothing here
     * reaches for a numeric conversion that can raise: a decode failure must
     * speak kotlinx's own type. `SerializationException` is what
     * `BaseService.decodeOrApiError` catches to raise the SPEC §6 malformed-body
     * error (#604), and what the §18 composites and the conformance runner read
     * back out of `BasecampException.Api.decodeFailure` to tell a decoder
     * rejection from a real API failure. A `NumberFormatException` escapes all
     * three. Here that is structural rather than caught: the scan returns a
     * verdict instead of throwing one.
     */
    private fun longFromLiteral(text: String): Long {
        // JSON's own grammar first — including `null`, `true` and `false`, which
        // arrive here as literals whose text is not a number. A `null` is the one
        // shape worth stating outright, because it is NOT the absent id: Go calls
        // `UnmarshalJSON` for a null, its number path decodes it into an empty
        // `json.Number`, and `ParseInt("")` fails — so `{"id": null}` fails the
        // read while a missing `id` is the zero value with no error at all. A
        // plain `int64` field has no `UnmarshalJSON`, so `encoding/json` handles
        // its null itself and both are 0; only a flexible field tells them apart.
        // `ruby/lib/basecamp/ids.rb`'s `person_from_wire` documents that
        // asymmetry, measured through the real decode path. Refusing here agrees
        // with it — by a different route, since kotlinx hands this path the text
        // "null" rather than an empty buffer.
        if (!isJsonInteger(text)) {
            throw SerializationException("FlexibleLong: $text is not a Long")
        }
        return when (val parsed = parseInt64(text)) {
            is ParsedInt64.Value -> parsed.value
            // Unreachable for Syntax: a JSON integer token is always a run of
            // ASCII digits behind an optional `-`, which [parseInt64] accepts by
            // construction. Range is the live branch — the token is well formed
            // and does not fit 64 bits — and the reference gives both refusals
            // one message on this path (`go/pkg/types/flexible_int64.go:61`),
            // unlike the quoted path, where only one of them is an error at all.
            ParsedInt64.Syntax, ParsedInt64.Range ->
                throw SerializationException("FlexibleLong: $text is not a Long")
        }
    }

    /**
     * Whether [text] is a JSON *integer* token: RFC 8259 §6's `int` production,
     * with neither `frac` nor `exp`. An optional `-`, then `0` alone or a digit
     * run that does not begin with `0`.
     *
     * Deliberately narrower than [parseInt64] in the two places the JSON grammar
     * is narrower than `strconv.ParseInt`: no leading `+`, and no leading zeros.
     * A token the reference's lexer would have rejected never reaches its
     * `ParseInt`, so it must not reach this one either.
     */
    private fun isJsonInteger(text: String): Boolean {
        val digits = if (text.startsWith("-")) text.substring(1) else text
        if (digits.isEmpty()) return false
        for (char in digits) {
            if (char < '0' || char > '9') return false
        }
        return digits.length == 1 || digits[0] != '0'
    }
}
