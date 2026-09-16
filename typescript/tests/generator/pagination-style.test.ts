import { describe, it, expect } from "vitest";
import { parseOperation, type Operation } from "../../scripts/generate-services.js";

// Pins TypeScript's refusal of a pagination style it does not implement.
//
// `hasPagination` keys off `style === "link"`, which makes an unrecognised value
// MORE dangerous than the old presence check was: read as "not paginated" it
// ships a method that never walks, silently, with nothing downstream to catch
// it. So every unimplemented spelling has to fail generation by name.
//
// The malformed-trait cases are the ones that actually bite. A truthy guard —
// `if (operation["x-basecamp-pagination"] && ...)` — exempts a present-but-falsy
// trait from the refusal and reads it as unpaginated, which is the exact silent
// pass the refusal exists to close. Swift had the same hole through
// `as? [String: Any]` and closed it by testing presence before the cast; Kotlin
// had it through `as? JsonObject`. These pin that TypeScript does the same.
//
// The declared `Operation` type says the trait is always an object or absent.
// The value comes from parsed JSON, so it can be anything; the casts below are
// deliberate, and removing them would hide the very cases under test.

function operationWith(pagination: unknown): Operation {
  const operation: Record<string, unknown> = {
    operationId: "ListWidgets",
    responses: {},
  };
  if (pagination !== undefined) {
    operation["x-basecamp-pagination"] = pagination;
  }
  return operation as unknown as Operation;
}

const parse = (pagination: unknown) => parseOperation("/widgets", "get", operationWith(pagination));

describe("pagination style refusal", () => {
  it("paginates on the link style and keeps its key", () => {
    const parsed = parse({ style: "link", key: "widgets" });

    expect(parsed.hasPagination).toBe(true);
    expect(parsed.paginationKey).toBe("widgets");
  });

  it("does not paginate on the cursor style, and withholds its key", () => {
    // The key is withheld at the source rather than gated at each consumer:
    // findUnderlyingEntitySchema unwraps an envelope whenever the key is set and
    // never consults hasPagination, so a cursor operation carrying a key would be
    // typed as the item under it instead of the envelope the wire actually sends.
    const parsed = parse({ style: "cursor", key: "events" });

    expect(parsed.hasPagination).toBe(false);
    expect(parsed.paginationKey).toBeUndefined();
  });

  it.each([
    ["absent", undefined],
    ["a literal null", null],
  ])("treats %s trait as unpaginated", (_label, pagination) => {
    const parsed = parse(pagination);

    expect(parsed.hasPagination).toBe(false);
    expect(parsed.paginationKey).toBeUndefined();
  });

  it.each([
    ["the retired page style", { style: "page" }, '"page"'],
    ["a typo", { style: "linkk" }, '"linkk"'],
    ["a capitalised link", { style: "Link" }, '"Link"'],
    ["an empty style", { style: "" }, '""'],
    ["a declared trait with no style", { maxPageSize: 50 }, "undefined"],
    ["a non-string style", { style: { name: "link" } }, "undefined"],
  ])("refuses %s by name", (_label, pagination, spelling) => {
    expect(() => parse(pagination)).toThrow(
      `ListWidgets: unsupported pagination style ${spelling} (expected "link" or "cursor")`,
    );
  });

  it.each([
    ["a bare string", "page"],
    ["an array", ["link"]],
    ["false", false],
    ["zero", 0],
    ["an empty string", ""],
  ])("refuses %s rather than reading it as unpaginated", (_label, pagination) => {
    // `false`, `0` and `""` are the cases a truthiness guard lets through.
    expect(() => parse(pagination)).toThrow(/unsupported pagination style/);
  });
});
