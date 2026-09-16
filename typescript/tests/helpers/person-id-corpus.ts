/**
 * The measured person-id corpus: 74 strings, and the verdict Go reaches on each.
 *
 * Every row is a MEASUREMENT, not a reading of the docs: a probe linked against
 * the real `go/pkg/types.FlexibleInt64` and the real `normalizeEmbeddedPeopleJSON`
 * produced the third column. `strconv.ParseInt(s, 10, 64)` has exactly three
 * outcomes and the two sites that read a string person id in this SDK — the
 * pre-decode normalizer in `src/services/base.ts` and the read-side comparison in
 * `src/services/mentions.ts` — each map all three:
 *
 * | Go says | the normalizer writes                  | the reader returns |
 * |---------|----------------------------------------|--------------------|
 * | value n | the number n, no `system_label`        | n                  |
 * | SYNTAX  | id `0` + `system_label` = the raw text | 0                  |
 * | RANGE   | nothing — the string stays put         | `undefined`        |
 *
 * `value` carries a `bigint` on purpose. Go's answer is an `int64`, and eleven
 * of these rows name one a JS `number` cannot hold; writing them here as
 * `number` literals would round the reference itself and the table would then
 * agree with the port by construction. The tests derive what each site may do
 * with an unrepresentable value from `fitsNumber` below, so every residual
 * divergence is visible as a residual rather than hidden in a rounded constant.
 *
 * Rows that exist to discriminate, and must not be dropped as redundant:
 * `"+7"`/`"+007"` (the sign a `^-?\d+$` regex refuses and ParseInt takes);
 * `"007"`, `"010"`, `"0009223372036854775807"` (leading zeros carry no meaning —
 * `"010"` is TEN); the Unicode digits; the whitespace forms Go trims none of;
 * the `_` separators base 10 does not take; `"18446744073709551615x"` against
 * `"18446744073709551616x"` (one digit apart, opposite refusals — the scan-order
 * pair); and `"9007199254740992"`/`"9007199254740993"`, real int64 ids past
 * JS's safe-integer range.
 */
export type CorpusRow = {
  readonly id: string;
  /** What `strconv.ParseInt(id, 10, 64)` does with it. */
  readonly go: "value" | "syntax" | "range";
  /** The int64 Go read, for `go: "value"` rows only. */
  readonly value?: bigint;
};

/** Whether a JS `number` can hold Go's answer without rounding it. */
export function fitsNumber(value: bigint): boolean {
  return value >= BigInt(Number.MIN_SAFE_INTEGER) && value <= BigInt(Number.MAX_SAFE_INTEGER);
}

const value = (id: string, v: bigint): CorpusRow => ({ id, go: "value", value: v });
const syntax = (id: string): CorpusRow => ({ id, go: "syntax" });
const range = (id: string): CorpusRow => ({ id, go: "range" });

export const PERSON_ID_CORPUS: readonly CorpusRow[] = [
  value("7", 7n),
  value("0", 0n),
  value("-0", 0n),
  value("+0", 0n),
  value("+7", 7n),
  value("-7", -7n),
  value("007", 7n),
  value("+007", 7n),
  value("-007", -7n),
  value("0009223372036854775807", 9223372036854775807n),
  value("0000000000000000000000009", 9n),
  syntax(""),
  syntax(" "),
  syntax("+"),
  syntax("-"),
  syntax(" 7"),
  syntax("7 "),
  syntax(" 7 "),
  syntax("\n7"),
  syntax("7\n"),
  syntax("\t7"),
  syntax("7\t"),
  syntax("1_0"),
  syntax("1_2"),
  syntax("0x10"),
  syntax("0b11"),
  syntax("0o17"),
  value("010", 10n),
  syntax("0X1F"),
  syntax("7x"),
  syntax("x7"),
  syntax("12.0"),
  syntax("1e3"),
  syntax("12,3"),
  syntax("basecamp"),
  syntax("campfire"),
  syntax("LocalPerson"),
  syntax("１２３"),
  syntax("７"),
  syntax("٠١٢"),
  syntax("৭"),
  syntax("۷"),
  value("9223372036854775806", 9223372036854775806n),
  value("9223372036854775807", 9223372036854775807n),
  range("9223372036854775808"),
  range("9223372036854775809"),
  value("-9223372036854775807", -9223372036854775807n),
  value("-9223372036854775808", -9223372036854775808n),
  range("-9223372036854775809"),
  range("18446744073709551614"),
  range("18446744073709551615"),
  range("18446744073709551616"),
  syntax("18446744073709551615x"),
  range("18446744073709551616x"),
  syntax("1844674407370955161x"),
  syntax("-18446744073709551615x"),
  range("-18446744073709551616x"),
  range("99999999999999999999999"),
  range("99999999999999999999999x"),
  range("00000000000000000000018446744073709551616"),
  value("0000000000000000000009223372036854775807", 9223372036854775807n),
  value("9007199254740991", 9007199254740991n),
  value("9007199254740992", 9007199254740992n),
  value("9007199254740993", 9007199254740993n),
  value("90071992547409931", 90071992547409931n),
  value("-9007199254740993", -9007199254740993n),
  value("+9223372036854775807", 9223372036854775807n),
  range("+9223372036854775808"),
  value("00", 0n),
  value("0000", 0n),
  value("-00", 0n),
  syntax("٠"),
  syntax("৭7"),
  syntax("7৭"),
];
