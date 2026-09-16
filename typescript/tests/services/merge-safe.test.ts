/**
 * The id-list guard on its own, with no normalizer walk in front of it.
 *
 * The composite tests in todos.test.ts and schedules.test.ts go through the
 * transport, and on a base whose pre-decode walk covers `assignees` those
 * string ids are already numbers by the time the guard sees them — so they
 * pass whether or not the guard reads a string. These call `writableIdList`
 * directly, which is the only way to pin what the guard itself does with a
 * person the walk did not reach.
 *
 * Every expectation is the reference's, measured through its own todos Update
 * and schedules EditEntry composites (`Person.Id` is `types.FlexibleInt64`, and
 * `fieldsFromTodo` appends whatever that produced with no filter) — with ONE
 * deliberate exception, marked where it appears: an id past 2^53 the reference
 * reads as a number is refused here, because JavaScript has already rounded it
 * and the only alternative is writing a different person's id.
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

  it("refuses a non-object element and a non-array field", () => {
    expect(() => read([5])).toThrow(/"assignees"\[0\] is not an object/);
    expect(() => read({ id: 7 })).toThrow(/"assignees" is not an array/);
  });
});
