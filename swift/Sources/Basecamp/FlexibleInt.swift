import Foundation

/// What `strconv.ParseInt(s, 10, 64)` made of a string: the value it read, or
/// which of its two refusals fired.
///
/// Keeping the refusals apart is the whole point. Go answers `0` to one and an
/// error to the other (`go/pkg/types/flexible_int64.go:43-46`), and no single
/// "is this a number?" predicate can tell them apart, because *which* refusal
/// comes first depends on where in the string the disqualifying byte sits — see
/// ``parsePersonID(_:)``.
enum PersonIDReading: Equatable, Sendable {
    /// An integer Go reads.
    case value(Int)
    /// `strconv.ErrSyntax`. Go's non-numeric sentinel: reads as the system
    /// actor `0` (`go/pkg/types/flexible_int64.go:46`,
    /// `go/pkg/basecamp/normalize.go:66-67`).
    case syntax
    /// `strconv.ErrRange`. Go raises it (`go/pkg/types/flexible_int64.go:43-44`)
    /// and leaves the string alone in the normalizer
    /// (`go/pkg/basecamp/normalize.go:62-63`), so both sites here must too.
    case range
}

/// `strconv.ParseInt(s, 10, 64)`, scan order included.
///
/// This is the one person-id grammar for this module. Both sites that read a
/// wire person id out of a string go through it — the pre-decode normalizer
/// (`BaseService.normalizeWalk`, against `go/pkg/basecamp/normalize.go:45`) and
/// the flexible reader (``FlexibleInt/init(from:)``, against
/// `go/pkg/types/flexible_int64.go:34`) — because the Go lines governing the two
/// are the same line, and two hand-written copies drift.
///
/// It takes one optional ASCII sign, then one or more ASCII digits, and nothing
/// else: no whitespace (Go trims none, so `" 7"` is a syntax error and reads
/// `0`), a leading `+` accepted, `_` never a separator at base 10 (only base 0
/// allows it), and ASCII digits alone, so a fullwidth `７` is not a digit.
///
/// **Why this walks the UTF-8 bytes instead of asking a regex.** Both sites
/// previously paired `Int(s)` with `s.range(of: #"^-?\d+$"#, options:
/// .regularExpression)` to tell an overflow from a sentinel. That pairing was
/// wrong three separate ways, in both directions at once:
///
/// 1. `NSRegularExpression` is ICU, and ICU's `\d` is `\p{Nd}` — *every* Unicode
///    decimal digit — while `Int(_:radix:)` is ASCII-strict. So `"１２３"`,
///    `"٠١٢"`, `"৭"`, `"۷"` and `"７"` failed `Int()`, matched the regex, and
///    were therefore reported as numeric **overflow** — which throws and fails
///    the whole response — where Go reads its non-numeric sentinel `0`.
/// 2. ICU's `$` also matches before a final newline, and `range(of:options:)`
///    asks for a match *somewhere* rather than over the whole string, so `"7\n"`
///    matched as well and was refused where Go labels it. Two anchors that look
///    like they pin both ends pin neither.
/// 3. In the other direction the regex refuses the leading `+` that `ParseInt`
///    accepts, so `"+9223372036854775808"` — a range error Go raises — fell
///    through to the sentinel branch and named the system actor `0`.
///
/// Measured, not inferred: restoring that pair and running the 74-row corpus in
/// `FlexibleIntTests` under Swift 6.0 diverges from Go's verdict on 13 rows at
/// each of the two sites; this scan diverges on none. Walking the bytes settles
/// all three permanently — there is no character-class, anchor or match-mode
/// semantics left to depend on a library version.
///
/// The subtlety worth the hand-rolled loop: `ParseUint` checks the magnitude
/// *inside* the scan and returns `ErrRange` the moment the accumulator would
/// overflow `uint64` — before it ever looks at the rest of the string. The first
/// disqualifying byte wins, and the boundary is `UInt64.max`, not `Int64.max`.
/// So `"18446744073709551616x"` is a **range** error that Go raises, while its
/// one-digit-shorter neighbour `"18446744073709551615x"` is a **syntax** error
/// that reads `0`. Testing the whole string for well-formedness first — which is
/// what a regex does — gets that pair backwards.
///
/// `Int` is `Int64` on every platform this package supports (`Package.swift`:
/// iOS 16, macOS 12), so `Int` here *is* Go's `int64` and the bounds below are
/// `ParseInt`'s own.
///
/// Note that this is **not** the rule the global-id parser applies to the person
/// id in `gid://bc3/Person/<id>`. That one walks the bytes and refuses anything
/// outside `0...9` *before* it parses, and then refuses `id <= 0`
/// (`go/pkg/basecamp/mentions.go:252-258`), so it rejects a leading `+` that this
/// one accepts. Both rules live in this module: ``Mentions/personId(fromGlobalId:)``
/// carries that digit walk, and it is *correct* to, because the reference has
/// that shape at that site — unifying them would reintroduce the `+77` defect
/// PR #886 closed. The same shape was wrong here only because the Go lines
/// governing these two sites have no pre-walk. Two rules, deliberately: do not
/// hoist either into the other, in either direction.
func parsePersonID(_ text: String) -> PersonIDReading {
    let zero = UInt8(ascii: "0")
    let nine = UInt8(ascii: "9")
    let plus = UInt8(ascii: "+")
    let minus = UInt8(ascii: "-")

    // Bytes, not Characters. A Swift `Character` is a grapheme cluster, so a
    // digit followed by a combining mark is one Character; the grammar is
    // defined over bytes and is read over bytes.
    let bytes = Array(text.utf8)

    var index = 0
    var negative = false
    if let first = bytes.first, first == plus || first == minus {
        negative = first == minus
        index = 1
    }
    // An empty digit run after a sign — and the empty string — is a syntax
    // error, not a range error.
    guard index < bytes.count else { return .syntax }

    // `ParseUint`'s loop, byte for byte.
    var magnitude: UInt64 = 0
    while index < bytes.count {
        let byte = bytes[index]
        guard byte >= zero, byte <= nine else { return .syntax }
        let (shifted, shiftOverflowed) = magnitude.multipliedReportingOverflow(by: 10)
        if shiftOverflowed { return .range }
        let (added, addOverflowed) = shifted.addingReportingOverflow(UInt64(byte - zero))
        if addOverflowed { return .range }
        magnitude = added
        index += 1
    }

    // `ParseInt`'s own bound, applied to what `ParseUint` returned. The negative
    // limit is one past `Int.max`: `-9223372036854775808` is in range.
    if negative {
        let limit = UInt64(Int.max) + 1
        if magnitude > limit { return .range }
        if magnitude == limit { return .value(Int.min) }
        return .value(-Int(magnitude))
    }
    if magnitude > UInt64(Int.max) { return .range }
    return .value(Int(magnitude))
}

/// An integer that decodes from either a JSON number or a JSON string.
///
/// The Basecamp API sometimes returns person IDs as strings (e.g. `"12345"`)
/// instead of numbers, and uses non-numeric sentinels like `"basecamp"` for
/// system-generated entities. The string path is
/// `strconv.ParseInt(s, 10, 64)` and nothing looser — see ``parsePersonID(_:)``
/// for the grammar and for which refusal becomes `0` and which throws.
///
/// - JSON number `12345` → `value = 12345`
/// - JSON string `"12345"` → `value = 12345`
/// - JSON string `"basecamp"` → `value = 0` (non-numeric sentinel)
/// - JSON string `"9223372036854775808"` → throws (numeric overflow)
/// - JSON number `9223372036854775808`, `7.5`, `null`, `true`, `[…]`, `{…}` →
///   throws. **Only the string path has a sentinel**; see ``init(from:)``.
public struct FlexibleInt: Codable, Sendable, Hashable, CustomStringConvertible, ExpressibleByIntegerLiteral {
    public let value: Int

    public init(_ value: Int) {
        self.value = value
    }

    public init(integerLiteral value: Int) {
        self.value = value
    }

    /// Reads the id, from a JSON number or a JSON string.
    ///
    /// **The sentinel belongs to the string path alone.** Go runs the number
    /// through `json.Decoder` with `UseNumber()` and then calls
    /// `json.Number.Int64()` (`go/pkg/types/flexible_int64.go:52-62`), and
    /// `Int64()` *is* `strconv.ParseInt(s, 10, 64)` over the number's literal
    /// text — the same scan the string path runs. But the number path has no
    /// `ErrSyntax`-becomes-`0` branch: `:60-62` returns an error for **either**
    /// refusal. So everything a person id can be spelled as that is not an
    /// integer fails the read there, and must fail it here. Answering `0` to any
    /// of them is the accepting direction on the field that says *who acted* —
    /// `0` is the system actor, `LocalPerson` / `"basecamp"` / `"campfire"` —
    /// which is the whole defect this type exists to close, reappearing on the
    /// half of it that is not a string.
    ///
    /// **`null` fails the read; an absent key does not.** `encoding/json` calls
    /// `UnmarshalJSON` for a `null`, the number path leaves the `json.Number`
    /// empty, and `ParseInt("")` is a syntax error — so `{"id": null}` fails,
    /// while a missing `"id"` never reaches the decoder and is the zero value.
    /// A plain `int64` field has no `UnmarshalJSON` and reads both as `0`, which
    /// is why only the flexible reader draws the distinction. The asymmetry is
    /// the same one `ruby/lib/basecamp/ids.rb`'s `person_from_wire` documents,
    /// measured there through the real decode path. Here it falls out of the
    /// model shape rather than needing a branch: `Person.id` is a non-optional
    /// `FlexibleInt`, so a `null` reaches this initializer and throws, and an
    /// absent key is the container's `keyNotFound`, never this code.
    ///
    /// **Do not reach for SPEC §10's rule here.** A rich-text/upload `width` or
    /// `height` is a *different type* — a bare `Int32?`, decoded by the
    /// synthesized decoder — and it is deliberately float-tolerant, because BC3
    /// really does serialize a present dimension float-spelled (`1024.0`) and
    /// `null` there is a legitimate value meaning "not an image". A pixel count
    /// may be lenient; an identity may not. The two look alike and are governed
    /// by opposite rules.
    ///
    /// **Residual, measured.** A float-spelled but integral number in `Int`
    /// range — `7.0`, `1e3`, `7e0`, `0.0`, `-0.0` — reads as that integer here
    /// and is a decode failure in Go. It is not reachable through the `Decoder`
    /// API: measured under Swift 6.0, `7` and `7.0` are indistinguishable
    /// through every accessor a `SingleValueDecodingContainer` offers
    /// (`decode(Int.self)` yields `7` for both, `Double` `7.0` for both, and
    /// there is no access to the literal text), because `JSONDecoder` unboxes
    /// through `Int(exactly: Double)`. The divergence is accepting-direction but
    /// benign in the way that matters: it reads the *correct* id for a spelling
    /// Go refuses, and can never produce the system actor or name a different
    /// person. Closing it needs the raw number literal, which only a pre-decode
    /// pass over the bytes can see.
    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let i = try? container.decode(Int.self) {
            value = i
        } else if let s = try? container.decode(String.self) {
            switch parsePersonID(s) {
            case .value(let n):
                value = n
            case .syntax:
                // Non-numeric sentinel (e.g. "basecamp"): Go's system actor
                // (`go/pkg/types/flexible_int64.go:46`).
                value = 0
            case .range:
                // `go/pkg/types/flexible_int64.go:43-44` — the caller should know.
                throw DecodingError.dataCorruptedError(
                    in: container,
                    debugDescription: "FlexibleInt: \"\(s)\" overflows Int"
                )
            }
        } else {
            // Neither an integer nor a string: a non-integral or out-of-range
            // number, `null`, a bool, an array, an object. Go fails the read on
            // every one of them (`go/pkg/types/flexible_int64.go:56-62`) — the
            // number path has no sentinel. This used to answer `0`, which is the
            // system actor.
            throw DecodingError.dataCorruptedError(
                in: container,
                debugDescription: "FlexibleInt: expected an integer or a string holding one"
            )
        }
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        try container.encode(value)
    }

    public var description: String { "\(value)" }
}
