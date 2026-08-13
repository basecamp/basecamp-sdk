/**
 * Case-census contract (#602).
 *
 * The check is green on the real fixture tree by construction, so a live run
 * only ever proves it can say yes. These cases run it against a SYNTHETIC
 * fixture set and prove it can say no — the `mode: "moc"` case in particular,
 * which every runner's "mock unless told otherwise" filter drops with nothing
 * printed. That divergence is asserted end-to-end here: the census and the load
 * path's own predicate (`isMockMode`, shared with `loadTestSuites`) disagree by
 * one, and `caseCountFailure` reports it.
 */

import { describe, it, expect, afterEach } from "vitest";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { caseCountFailure, countNonLiveCases, isMockMode } from "./case-census.js";

/**
 * One case of each kind: a plain mock case (no `mode` at all, the common
 * spelling), a live case the runners are meant to drop, and a typo'd mode that
 * nothing recognizes.
 */
const FIXTURE = [
  { name: "plain", operation: "GetProject" },
  { name: "live one", operation: "GetProject", mode: "live" },
  { name: "typo", operation: "GetProject", mode: "moc" },
];

const trees: string[] = [];

function fixtureTree(files: Record<string, string>): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "case-census-"));
  trees.push(dir);
  for (const [relative, content] of Object.entries(files)) {
    const file = path.join(dir, relative);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, content);
  }
  return dir;
}

afterEach(() => {
  while (trees.length > 0) {
    fs.rmSync(trees.pop()!, { recursive: true, force: true });
  }
});

describe("countNonLiveCases", () => {
  it("counts every case that is not explicitly live", () => {
    const dir = fixtureTree({ "cases.json": JSON.stringify(FIXTURE) });

    expect(countNonLiveCases(dir)).toBe(2);
  });

  it("finds fixtures nested below the tests directory", () => {
    // No runner globs recursively, so a case parked one directory down is run
    // by nothing. The census walks, which is what makes that visible.
    const dir = fixtureTree({ "nested/cases.json": JSON.stringify(FIXTURE) });

    expect(countNonLiveCases(dir)).toBe(2);
  });

  it("rejects a fixture that does not parse", () => {
    const dir = fixtureTree({ "broken.json": '[{"name": "truncated"' });

    expect(() => countNonLiveCases(dir)).toThrow();
  });

  it("rejects a fixture that is not an array", () => {
    const dir = fixtureTree({ "object.json": '{"name": "not a list"}' });

    expect(() => countNonLiveCases(dir)).toThrow();
  });

  it("rejects an empty tree", () => {
    // A census that counted nothing certifies nothing: zero on both sides is
    // the shape a broken walk takes.
    expect(() => countNonLiveCases(fixtureTree({}))).toThrow();
  });

  it("rejects an emptied fixture", () => {
    // The one truncation both sides read identically: the runner registers
    // nothing from the file and the census would expect nothing, so the totals
    // fall together and no mismatch appears. Counting it as zero is what would
    // make the whole-file guarantee a lie, so the census refuses it instead.
    const dir = fixtureTree({
      "cases.json": JSON.stringify(FIXTURE),
      "emptied.json": "[]",
    });

    expect(() => countNonLiveCases(dir)).toThrow();
  });
});

describe("caseCountFailure", () => {
  it("fails when a typo'd mode leaves a case registered by nothing", () => {
    // The regression this whole check exists for. The load path's filter keeps
    // one case; the census counts two; the difference is the case run by
    // nothing.
    const dir = fixtureTree({ "cases.json": JSON.stringify(FIXTURE) });
    const registered = FIXTURE.filter((tc) => isMockMode(tc.mode)).length;
    expect(registered).toBe(1);

    const failure = caseCountFailure(registered, countNonLiveCases(dir));

    expect(failure).not.toBeNull();
    expect(failure).toContain("1 executed by nothing");
  });

  it("accepts agreement", () => {
    expect(caseCountFailure(42, 42)).toBeNull();
  });

  it("names an over-count", () => {
    expect(caseCountFailure(43, 42)).toContain("1 more than the fixtures declare");
  });
});

describe("isMockMode", () => {
  it("treats absence as mock", () => {
    expect(isMockMode(undefined)).toBe(true);
    expect(isMockMode("mock")).toBe(true);
    expect(isMockMode("live")).toBe(false);
    // The census is what catches this one; the filter must not run it.
    expect(isMockMode("moc")).toBe(false);
    // `??`, not falsiness: an explicit empty mode is not an absent one.
    // Python defaulted on falsiness and ran it.
    expect(isMockMode("")).toBe(false);
  });
});
