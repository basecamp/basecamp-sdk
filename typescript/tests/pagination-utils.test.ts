/**
 * Link-header parsing tests.
 *
 * parseNextLink had no dedicated unit test — it was only ever exercised
 * indirectly through pagination flows, always with well-formed headers. The
 * Link header is attacker-influenced (isSameOrigin exists to stop SSRF through
 * a poisoned one), so its behaviour on malformed input is part of the contract.
 * The adversarial cases below exist in all six SDKs.
 */
import { describe, it, expect } from "vitest";
import { parseNextLink } from "../src/pagination-utils.js";

describe("parseNextLink", () => {
  it("extracts the url from a standard header", () => {
    expect(parseNextLink('<https://api.example.com/items?page=2>; rel="next"')).toBe(
      "https://api.example.com/items?page=2",
    );
  });

  it("picks next out of several rels", () => {
    const header =
      '<https://api.example.com/items?page=1>; rel="first", ' +
      '<https://api.example.com/items?page=2>; rel="next", ' +
      '<https://api.example.com/items?page=10>; rel="last"';
    expect(parseNextLink(header)).toBe("https://api.example.com/items?page=2");
  });

  it("returns null when there is no next rel", () => {
    expect(parseNextLink('<https://api.example.com/items?page=1>; rel="first"')).toBeNull();
  });

  it("returns null for empty and null headers", () => {
    expect(parseNextLink("")).toBeNull();
    expect(parseNextLink(null)).toBeNull();
  });
});

describe("parseNextLink — adversarial input", () => {
  it("returns null when the bracket never closes", () => {
    expect(parseNextLink('<https://api.example.com/page2; rel="next"')).toBeNull();
  });

  it("reads a closing bracket that precedes the opening bracket", () => {
    expect(parseNextLink('>x<https://api.example.com/page2>; rel="next"')).toBe(
      "https://api.example.com/page2",
    );
  });

  it("truncates a url at its first raw closing bracket", () => {
    // Parity with the old /<([^>]+)>/ spelling: [^>] cannot span a ">".
    expect(parseNextLink('<https://api.example.com/page2?q=a>b>; rel="next"')).toBe(
      "https://api.example.com/page2?q=a",
    );
  });

  it("takes the first of multiple bracket pairs in one part", () => {
    // Parity with the old spelling: leftmost match wins.
    expect(
      parseNextLink('<https://api.example.com/a> <https://api.example.com/b>; rel="next"'),
    ).toBe("https://api.example.com/a");
  });

  it("skips an empty bracket pair rather than returning it", () => {
    // Parity with the old spelling: [^>]+ requires at least one character, so
    // an empty <> is not a match and the scan moves on. A naive
    // indexOf(">", start + 1) without this check would return "".
    expect(parseNextLink('<> <https://api.example.com/page2>; rel="next"')).toBe(
      "https://api.example.com/page2",
    );
  });

  it("keeps scanning past a malformed part instead of short-circuiting", () => {
    // Deliberate behaviour change. This function used to `return match?.[1] ??
    // null` on the first part containing rel="next", so one unparseable segment
    // collapsed the whole header to null. The other five SDKs fall through and
    // keep looking; TypeScript now does too.
    expect(
      parseNextLink('<malformed; rel="next", <https://api.example.com/page2>; rel="next"'),
    ).toBe("https://api.example.com/page2");
  });

  it("handles a pathological header", () => {
    // Many "<" start positions with no reachable ">" — the shape that punishes
    // a backtracking regex. The /<([^>]+)>/ spelling took 1.6s at 32k
    // characters (alert 43); this is 50k. Asserting behaviour and completion,
    // not elapsed time: this suite already has timing flakiness (#655) and a
    // duration bound would add more.
    const many = "<".repeat(50_000);
    expect(parseNextLink(`${many}; rel="next"`)).toBeNull();
    // A ">" present but unreachable defeats the literal-prescan shortcut some
    // regex engines use to bail early.
    expect(parseNextLink(`>${many}; rel="next"`)).toBeNull();
  });

  it("handles a header of many empty bracket pairs", () => {
    // The pathological case for the scan that replaced the regex, which is a
    // different shape from the one above: that header returns after a single
    // ">"-search and never takes the empty-<> branch, so the skip loop's own
    // worst case went untested. Every "<>" here advances the cursor by one and
    // goes round again — the only path where a non-constant-time index lookup
    // would compound into quadratic behaviour.
    const pairs = "<>".repeat(50_000);
    // No non-empty pair anywhere: every iteration skips, then the scan runs out.
    expect(parseNextLink(`${pairs}; rel="next"`)).toBeNull();
    // Same prefix, but the skips have to land on a real pair at the end.
    expect(parseNextLink(`${pairs}<https://api.example.com/page2>; rel="next"`)).toBe(
      "https://api.example.com/page2",
    );
  });
});
