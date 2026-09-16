/**
 * Go's `strconv.ParseInt(s, 10, 64)` over a person id, written out.
 *
 * Three places in this SDK turn a person id that arrived as a JSON STRING into
 * a number, and they answer to the same Go line: the pre-decode normalizer in
 * `services/base.ts` stands where `coercePersonID` stands
 * (`go/pkg/basecamp/normalize.go:45`), and the read-side comparison in
 * `services/mentions.ts` and the merge-safe id-list guard in
 * `services/merge-safe.ts` stand where `FlexibleInt64` stands
 * (`go/pkg/types/flexible_int64.go:34`). All of them call
 * {@link scanPersonId}, so the rule cannot drift into rules that disagree about
 * the same string.
 *
 * Do not reach for `Number()`, `parseInt()`, or a `^-?\d+$` test in their place.
 * Each is wrong in a direction that matters and the three are wrong differently:
 * `Number()` accepts `"1e3"`, `"0x10"`, `"12.0"` and `" 7"`, all of which Go
 * refuses; `parseInt()` accepts a trailing tail, so `"7x"` reads 7; and the
 * regex refuses the leading `+` Go accepts, which is how `"+7"` became a system
 * actor. What none of them can do at all is the thing this file exists for —
 * tell Go's two refusals apart. See {@link PersonIdScan}.
 */

/**
 * What `strconv.ParseInt(s, 10, 64)` answered: a value, or WHICH WAY it refused.
 *
 * Keeping the two refusals apart is the whole point, because Go does two
 * different things with them and no single "is this a number?" predicate can
 * separate them. A SYNTAX error reads as `0` — Go's non-numeric sentinel, the
 * id of a `LocalPerson` (`"basecamp"`, `"campfire"`) — while a RANGE error is
 * raised and fails the read (`go/pkg/types/flexible_int64.go:43` against `:46`;
 * `go/pkg/basecamp/normalize.go:62-63` against `:66-67`). Which one you get
 * depends on where in the string the first disqualifying byte sits, so it
 * cannot be recovered after the fact from a yes/no answer.
 *
 * `value` is a `bigint` because Go's answer is an `int64` and a JS `number` is
 * not one. Rounding it here would erase, inside the scan, the very distinction
 * each caller has to make a decision about — see {@link personIdNumber}.
 */
export type PersonIdScan =
  | { readonly kind: "value"; readonly value: bigint }
  | { readonly kind: "syntax" }
  | { readonly kind: "range" };

const SYNTAX: PersonIdScan = { kind: "syntax" };
const RANGE: PersonIdScan = { kind: "range" };

const ZERO = 0x30;
const NINE = 0x39;
const PLUS = 0x2b;
const MINUS = 0x2d;

/** `ParseUint`'s bound: the magnitude is checked against u64, not i64. */
const U64_MAX = 18446744073709551615n;
/** `ParseInt`'s own bounds, applied to what `ParseUint` returned. */
const I64_MAX = 9223372036854775807n;
const I64_MIN_MAGNITUDE = 9223372036854775808n;

/**
 * `strconv.ParseInt(text, 10, 64)`, scan order included.
 *
 * The grammar is one optional ASCII sign, then one or more ASCII digits, and
 * nothing else:
 *
 * - A leading `+` is ACCEPTED, and a `^-?\d+$` regex is exactly the port defect
 *   this replaces — `"+7"` is 7, not a sentinel.
 * - No whitespace of any kind, because Go trims none: `" 7"`, `"7 "`, `"\t7"`
 *   are syntax errors and read `0`.
 * - Leading zeros carry no meaning at base 10, so `"007"` is 7 and `"010"` is
 *   TEN, not eight. (`ParseInt` only reads a prefix as a base when `base` is 0,
 *   which is why `"0x10"` is a syntax error here rather than 16.)
 * - `_` is a digit separator only at base 0, so `"1_0"` is a syntax error.
 * - ASCII `0`-`9` and nothing else: a fullwidth `７`, an Arabic-Indic `٧` and a
 *   Bengali `৭` are not digits, however `\p{Nd}`-aware a regex engine is. This
 *   scans UTF-16 units rather than code points on purpose — every unit of a
 *   non-ASCII character is outside `0x30..0x39`, including a lone surrogate, so
 *   the first one refuses exactly where Go's byte scan refuses.
 *
 * The subtlety that earns the hand-rolled loop: `ParseUint` checks the
 * magnitude INSIDE the scan and returns `ErrRange` the moment the accumulator
 * would overflow `uint64`, before it ever looks at the rest of the string. The
 * first disqualifying byte wins, and the bound it wins against is u64, not i64.
 * So `"18446744073709551616x"` is a RANGE error, which fails the read, while
 * `"18446744073709551615x"` — one digit smaller — is a SYNTAX error, which
 * reads `0`. Testing the whole string for well-formedness first and only then
 * checking the magnitude gets that pair backwards, which is the second defect
 * this replaces.
 *
 * This is NOT the rule the gid parser applies to the person id in
 * `gid://bc3/Person/<id>`. That one walks the bytes and refuses anything
 * outside `0..=9` BEFORE it parses (`go/pkg/basecamp/mentions.go:252-256`),
 * so it rejects the leading `+` this one accepts, and then refuses `id <= 0`.
 * Both rules live in this SDK: `personIdFromSGID` in `services/mentions.ts`
 * carries that digit walk, and it is CORRECT to, because the reference has that
 * shape at that site — a `+`-tolerant gid parser is the `+77` defect PR #886
 * closed. The same shape was wrong here only because the Go lines governing
 * these two sites have no pre-walk. Two sites, two rules, deliberately: do not
 * hoist either into the other, in either direction.
 */
export function scanPersonId(text: string): PersonIdScan {
  let index = 0;
  let negative = false;
  if (text.length > 0) {
    const sign = text.charCodeAt(0);
    if (sign === PLUS || sign === MINUS) {
      negative = sign === MINUS;
      index = 1;
    }
  }
  // An empty digit run is a syntax error: `""`, `"+"`, `"-"`.
  if (index === text.length) return SYNTAX;

  // `ParseUint`'s loop, unit for unit.
  let magnitude = 0n;
  for (; index < text.length; index++) {
    const code = text.charCodeAt(index);
    if (code < ZERO || code > NINE) return SYNTAX;
    magnitude = magnitude * 10n + BigInt(code - ZERO);
    // Go returns here, mid-scan, without reading what follows.
    if (magnitude > U64_MAX) return RANGE;
  }

  // `ParseInt`'s own bound, applied to what `ParseUint` returned. `-2^63` fits
  // and `+2^63` does not, so the two signs have different bounds.
  if (negative) {
    if (magnitude > I64_MIN_MAGNITUDE) return RANGE;
    // `-0n` is not a thing in BigInt, so `"-0"` yields `0n` and, through
    // `personIdNumber`, `+0` — where `Number("-0")` yielded `-0`, which
    // `Object.is` separates from the `0` Go reads.
    return { kind: "value", value: -magnitude };
  }
  if (magnitude > I64_MAX) return RANGE;
  return { kind: "value", value: magnitude };
}

const MIN_SAFE = BigInt(Number.MIN_SAFE_INTEGER);
const MAX_SAFE = BigInt(Number.MAX_SAFE_INTEGER);

/**
 * The id Go read, as a JS `number` — or `undefined` when a `number` cannot hold
 * it.
 *
 * This is the port's one unclosable gap, and the only judgment call in this
 * file. `ParseInt` read a REAL PERSON here: `"9007199254740993"` is an id, not
 * a refusal. A `number` cannot carry it — past `Number.MAX_SAFE_INTEGER` two
 * distinct int64s land on the same double, so `9007199254740993` would be
 * handed to the caller as `9007199254740992`, a different person — and the
 * SDK's whole surface types every id as `number`. `conformance/tests/
 * integer-precision.json` records the same constraint for the JSON-number path
 * in as many words: returning bigint would break the number-typed API surface.
 * So there is no value this can return, and TypeScript has no runtime decoder
 * downstream to perform Go's RANGE refusal on its behalf. Whatever is chosen, a
 * caller sees it. Three directions were available:
 *
 * - **Write `0` and a `system_label`** — what the normalizer did before this
 *   file existed. That is Go's NON-NUMERIC sentinel: the id of `LocalPerson`,
 *   `"basecamp"`, `"campfire"`. It takes a real person, hands back the system
 *   actor, and says nothing — in the exact shape a caller is meant to trust. A
 *   wrong id that looks right is the one outcome nothing downstream can defend
 *   against, and it is the defect this work exists to remove.
 * - **Throw** — refuses a response Go reads fine, and refuses ALL of it. One
 *   unrepresentable id in one embedded person would discard every other record
 *   in the body, including the 99 people that were perfectly readable, and it
 *   would do so from inside a normalizer that runs over every response. The id
 *   is safe; everything beside it pays.
 * - **Say "unreadable"** — what this returns, and what each site then resolves
 *   locally with the treatment the measured table already assigns to a RANGE
 *   refusal. The normalizer LEAVES THE STRING in place: nothing is rounded,
 *   nothing is invented, the digits survive verbatim for a caller that can hold
 *   them, and `typeof person.id === "string"` is a check a caller can actually
 *   make. The reader returns `undefined`, and `mentionMarkup` refuses to write
 *   a mention it cannot verify. Both are refusals of the ID, not of the
 *   response.
 *
 * `personIdValue` in `services/mentions.ts` already answered it this way before
 * the two sites shared a rule, which settles the consistency question in the
 * same direction: the normalizer joins the reader.
 *
 * This is a RESIDUAL DIVERGENCE and is recorded as one, not a fix. Over the
 * measured corpus, the 11 rows whose value falls outside ±(2^53 - 1) still
 * disagree with Go at both sites, and will until the SDK's ids become `bigint`.
 */
export function personIdNumber(value: bigint): number | undefined {
  if (value < MIN_SAFE || value > MAX_SAFE) return undefined;
  return Number(value);
}
