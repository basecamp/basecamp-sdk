/**
 * The `{{httpdate+Ns}}` header token (SPEC §19, conformance/schema.json).
 *
 * A static fixture has no clock, so the positive half of SPEC §6's HTTP-date
 * branch was unpinnable until this token (#780). These cases pin the resolver's
 * arithmetic against a frozen instant so the fixture's one-sided timing floor
 * rests on a deterministic contract.
 */
import { describe, it, expect } from "vitest";
import { resolveHeaderValue } from "./header-tokens.js";

// A quarter-second into 10:18:14 UTC, so floor and round-up differ.
const NOW_MS = 1_623_233_894_250;

describe("resolveHeaderValue", () => {
  it("passes plain values through", () => {
    for (const value of ["", "2", "Wed, 09 Jun 2021 10:18:14 GMT", "application/json", "{not a token}"]) {
      expect(resolveHeaderValue(value, NOW_MS)).toBe(value);
    }
  });

  it("resolves httpdate to the whole second past N", () => {
    expect(resolveHeaderValue("{{httpdate+2s}}", NOW_MS)).toBe("Wed, 09 Jun 2021 10:18:17 GMT");
    expect(resolveHeaderValue("{{httpdate+0s}}", NOW_MS)).toBe("Wed, 09 Jun 2021 10:18:15 GMT");
    expect(resolveHeaderValue("{{httpdate+10s}}", NOW_MS)).toBe("Wed, 09 Jun 2021 10:18:25 GMT");
  });

  it("throws on an unknown token rather than serving it literally", () => {
    for (const value of ["{{httpdate}}", "{{httpdate+2}}", "{{httpdate-2s}}", "{{now}}", "{{}}", "{{httpdate+1000000000s}}"]) {
      expect(() => resolveHeaderValue(value, NOW_MS)).toThrow(value);
    }
  });
});
