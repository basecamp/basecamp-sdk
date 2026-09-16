package com.basecamp.sdk.serialization

/**
 * Which way [parseInt64] refused. Keeping the two apart is the whole point:
 * Go answers `0` to one and an error to the other, and no single "is this a
 * number?" predicate can tell them apart, because *which* refusal comes first
 * depends on where in the string each disqualifying byte sits.
 */
internal sealed interface ParsedInt64 {
    /** `ParseInt` accepted, and the id is this number. */
    data class Value(val value: Long) : ParsedInt64

    /**
     * `strconv.ErrSyntax`. Go's non-numeric sentinel: the reader answers `0`
     * (`go/pkg/types/flexible_int64.go:46`) and the normalizer writes id `0`
     * with the original string kept as `system_label`
     * (`go/pkg/basecamp/normalize.go:66-67`). That `0` is the SYSTEM ACTOR —
     * LocalPerson, `"basecamp"`, `"campfire"` — which is why landing here for a
     * value Go reads as a number names the wrong actor.
     */
    data object Syntax : ParsedInt64

    /**
     * `strconv.ErrRange`. The reader raises (`go/pkg/types/flexible_int64.go:43`)
     * and the normalizer leaves the string untouched so the read fails
     * (`go/pkg/basecamp/normalize.go:62-64`) — a failed read rather than an
     * oversized id silently becoming the system actor.
     */
    data object Range : ParsedInt64
}

/**
 * `strconv.ParseInt(s, 10, 64)`, scan order included.
 *
 * It takes one optional ASCII sign, then one or more ASCII digits, and nothing
 * else: no whitespace (Go trims none, so `" 7"` is a syntax error and reads
 * `0`), a leading `+` accepted, `_` never a separator at base 10 (only base 0
 * allows it), and ASCII digits alone, so a fullwidth `７` is not a digit.
 *
 * Hand-rolled because Kotlin's own parser is neither of those things.
 * `String.toLongOrNull` goes through `digitOf`, which on JVM is
 * `Character.digit` (`kotlin.text.CharsKt__CharJVMKt.digitOf`, verified against
 * the 2.4.20 stdlib) — Unicode-aware, so `"１２３"` reads 123, `"٠١٢"` reads 12
 * and `"7৭"` reads 77, every one of them a real person id where Go reads its
 * non-numeric sentinel `0` and names the system actor instead. `digitOf` is an
 * `expect`/`actual`, so the answer is a per-target detail this module must not
 * rest on; the common `Char.digitToInt` contract it is written against is the
 * Unicode Nd category either way. The regex that used to stand in for the range
 * check, `^-?\d+$`, was wrong in the other direction: it refuses the leading
 * `+` Go accepts, so `"+9223372036854775808"` collapsed to the sentinel where
 * Go raises.
 *
 * The subtlety worth the loop: `ParseUint` checks the magnitude *inside* the
 * scan and returns `ErrRange` the moment the accumulator would overflow
 * `UInt64` — before it ever looks at the rest of the string. So the first
 * disqualifying byte wins, and `"18446744073709551616x"` is a **range** error —
 * Go raises it — while `"18446744073709551615x"`, one digit shorter, is a
 * **syntax** error and reads `0`. Testing the whole string for well-formedness
 * first gets that pair backwards. Note that the in-loop bound is `UInt64.MAX`,
 * not `Long.MAX_VALUE`; `ParseInt` applies its own narrower bound afterwards,
 * to what `ParseUint` returned.
 *
 * Note that this is *not* the rule the global-id parser applies to the person
 * id in `gid://bc3/Person/<id>`. That one walks the bytes and refuses anything
 * outside `0..9` *before* it parses
 * (`go/pkg/basecamp/mentions.go:252-256`), so it rejects a leading `+` that
 * this one accepts. Both live in this module: `personIdFromSgid` in
 * [com.basecamp.sdk] carries that digit walk, and it is *correct* to, because
 * the reference has that shape at that site — loosening it is the defect PR
 * #886 closed, where `gid://bc3/Person/+77` began naming person 77. The same
 * shape was wrong here only because the Go lines governing these two sites have
 * no pre-walk. Two rules, deliberately: do not hoist either into the other, in
 * either direction. `go/pkg/basecamp/person_id_grammar_test.go` is the oracle
 * for all three, and pins the disagreement as a fact rather than an accident.
 */
internal fun parseInt64(text: String): ParsedInt64 {
    if (text.isEmpty()) return ParsedInt64.Syntax

    // One optional sign, `+` included. `ParseUint` then gets the rest.
    val negative = text[0] == '-'
    val digitsStart = if (negative || text[0] == '+') 1 else 0
    // An empty digit run after a sign, and the empty string above: ErrSyntax.
    if (digitsStart == text.length) return ParsedInt64.Syntax

    // `ParseUint`'s loop, byte for byte.
    var magnitude = 0uL
    for (index in digitsStart until text.length) {
        val char = text[index]
        if (char < '0' || char > '9') return ParsedInt64.Syntax
        val digit = (char - '0').toULong()
        // The in-loop overflow check that decides the pair above: refuse the
        // moment the accumulator would pass UInt64.MAX, without reading on.
        if (magnitude > (ULong.MAX_VALUE - digit) / 10uL) return ParsedInt64.Range
        magnitude = magnitude * 10uL + digit
    }

    // `ParseInt`'s own bound, applied to what `ParseUint` returned: a magnitude
    // past Long.MAX_VALUE (or past 2^63 when negative) is ErrRange, and
    // "-9223372036854775808" is fine.
    val longMinMagnitude = 9223372036854775808uL
    return when {
        negative && magnitude > longMinMagnitude -> ParsedInt64.Range
        !negative && magnitude > Long.MAX_VALUE.toULong() -> ParsedInt64.Range
        negative && magnitude == longMinMagnitude -> ParsedInt64.Value(Long.MIN_VALUE)
        negative -> ParsedInt64.Value(-magnitude.toLong())
        else -> ParsedInt64.Value(magnitude.toLong())
    }
}
