/**
 * Offline tests for the dispatch coverage gate.
 *
 * The gate must reject any operation referenced by a fixture that doesn't
 * have a dispatch entry. Critically, it must also reject inherited
 * Object.prototype keys (`toString`, `hasOwnProperty`, etc.) — pre-fix the
 * gate used `in` and would have let those slip through.
 */

import { describe, it, expect } from "vitest";
import { assertDispatchCoverage } from "./live-dispatch.js";

describe("assertDispatchCoverage", () => {
  it("does not throw for operations that have a dispatch entry", () => {
    expect(() => assertDispatchCoverage(["ListProjects", "GetProject"])).not.toThrow();
  });

  it("throws for operations missing a dispatch entry", () => {
    expect(() => assertDispatchCoverage(["NoSuchOperation"])).toThrow(
      /missing dispatch cases for: NoSuchOperation/,
    );
  });

  it("rejects Object.prototype keys via Object.hasOwn semantics", () => {
    // Pre-fix the gate used `in`, which traverses the prototype chain.
    // A fixture entry like { operation: "toString" } would have passed
    // the gate (since toString is an inherited property of all objects)
    // even though no dispatch case exists for it.
    expect(() => assertDispatchCoverage(["toString"])).toThrow(
      /missing dispatch cases for: toString/,
    );
    expect(() => assertDispatchCoverage(["hasOwnProperty"])).toThrow(
      /missing dispatch cases for: hasOwnProperty/,
    );
    expect(() => assertDispatchCoverage(["constructor"])).toThrow(
      /missing dispatch cases for: constructor/,
    );
  });

  it("collects all missing operations into a single error", () => {
    expect(() => assertDispatchCoverage(["ListProjects", "MissingA", "MissingB"])).toThrow(
      /missing dispatch cases for: MissingA, MissingB/,
    );
  });
});
