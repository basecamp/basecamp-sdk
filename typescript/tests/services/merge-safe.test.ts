/**
 * The id-list guard on its own, with no normalizer walk in front of it.
 *
 * The composite tests in todos.test.ts and schedules.test.ts go through the
 * transport, so what they prove depends on which keys the pre-decode walk
 * covers: a walk that converted a string id first would let them pass whether
 * or not the guard reads one. That happened once, while the walk still named
 * `assignees`. These call `writableIdList` directly, so the guard is pinned on
 * its own whatever the walk does.
 *
 * Every expectation is the reference's, measured through its own todos Update
 * and schedules EditEntry composites (`Person.Id` is `types.FlexibleInt64`, and
 * `fieldsFromTodo` appends whatever that produced with no filter) — with two
 * exceptions, both JavaScript's number boundary, marked where they appear. An
 * id past 2^53 the reference reads as a number is refused here, because it has
 * already been rounded and the alternative is writing a different person's id.
 * And a JSON number spelled `1024.0` or `1e3`, which the reference refuses, is
 * accepted: `JSON.parse` hands the guard `1024` and `1000`, so there is nothing
 * left to tell apart. That second row is pinned below too, so a change in
 * either direction is a decision rather than an accident.
 */
import { describe, it, expect } from "vitest";
import { writableIdList } from "../../src/services/merge-safe.js";
import { BasecampError } from "../../src/errors.js";

const opts = { record: "Todo", escape: "replace()" };

const read = (people: unknown) => writableIdList({ assignees: people }, "assignees", opts);

describe("writableIdList", () => {
  it.each([
    ["a number", 1049715914, 1049715914],
    ["a bare numeric string", "1049715914", 1049715914],
    ["leading zeros", "007", 7],
    ["a minus sign", "-5", -5],
    ["a plus sign", "+5", 5], // which /^-?\d+$/ rejects and ParseInt accepts
    ["the LocalPerson sentinel", "basecamp", 0],
    ["an empty string", "", 0],
    ["a leading space", " 12", 0], // Go trims nothing; Number(" 12") says 12
    ["a hex literal", "0x10", 0],
    ["an exponent", "1e3", 0], // Number("1e3") says 1000
    ["a decimal point", "12.0", 0],
    ["a numeric separator", "1_0", 0],
    ["a fullwidth digit", "７", 0],
    ["junk after uint64 max", "18446744073709551615x", 0], // syntax wins the scan
    ["the largest safe integer", "9007199254740991", 9007199254740991],
  ])("reads %s the way the reference does", (_label, id, expected) => {
    expect(read([{ id, name: "Jane" }])).toEqual([expected]);
  });

  it("reads an absent id as the zero value, and a null element as the zero Person", () => {
    expect(read([{ name: "Jane" }, null, { id: 7 }])).toEqual([0, 0, 7]);
  });

  it.each([
    // RANGE: the reference fails the read. "…616x" overflows inside the scan
    // before the 'x' is reached, one digit past the "…615x" sentinel above.
    ["a string overflowing int64", "9223372036854775808"],
    ["overflow before the junk", "18446744073709551616x"],
    // Past 2^53 the value is representable to the reference but not here, so
    // the id is refused rather than rounded into a different person's.
    ["a string past 2^53", "9007199254740993"],
    ["a number past 2^53", 9007199254740994],
    // An explicit null reaches the flexible decoder and fails; only ABSENT is 0.
    ["a null id", null],
    ["a boolean id", true],
    ["a fractional id", 10.5],
    ["NaN", Number.NaN],
    ["an object id", {}],
    ["an array id", [1]],
  ])("refuses %s", (_label, id) => {
    expect(() => read([{ id, name: "Jane" }])).toThrow(BasecampError);
    expect(() => read([{ id, name: "Jane" }])).toThrow(/"assignees"\[0\]\.id is not a person id/);
  });

  it("accepts an integral-float or exponent JSON number, which the reference refuses", () => {
    // A RESIDUAL DIVERGENCE, pinned so it is not mistaken for correctness: Go's
    // decoder refuses `1024.0` and `1e3` as person ids. `JSON.parse` has already
    // turned both into integers, so they are indistinguishable here.
    const parsed = JSON.parse('{"assignees":[{"id":1024.0},{"id":1e3}]}') as Record<string, unknown>;
    expect(writableIdList(parsed, "assignees", opts)).toEqual([1024, 1000]);
  });

  it("refuses a non-object element and a non-array field", () => {
    expect(() => read([5])).toThrow(/"assignees"\[0\] is not an object/);
    expect(() => read({ id: 7 })).toThrow(/"assignees" is not an array/);
  });
});
