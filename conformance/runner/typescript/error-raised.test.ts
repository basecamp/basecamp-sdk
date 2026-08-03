/**
 * Both directions of the errorRaised assertion contract.
 *
 * Only the passing direction ever runs against a committed fixture: every case
 * declaring errorRaised is one the SDK does refuse. A handler that accepted
 * everything would therefore look green in all six runners at once, which is
 * exactly how #563 shipped a vacuous delayBetweenRequests check.
 */
import { describe, it, expect } from "vitest";
import { errorRaisedFailure } from "./error-raised.js";

// Asserted verbatim here and in the five sibling runners. A fixture debugged in
// one language should not read differently in another.
const MESSAGE = "Expected the call to fail, but it succeeded";

describe("errorRaisedFailure", () => {
  it("is satisfied by a dispatch that failed", () => {
    expect(errorRaisedFailure(true)).toBeUndefined();
  });

  it("fails a dispatch that succeeded", () => {
    // The branch under test. It is unreachable from conformance/tests/, so
    // without this case the handler could accept everything undetected.
    expect(errorRaisedFailure(false)).toBe(MESSAGE);
  });
});
