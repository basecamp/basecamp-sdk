/**
 * The `errorRaised` assertion contract, kept apart from the runner so its
 * failing branch is unit-testable (error-raised.test.ts).
 */

/**
 * Validates one assertion, returning `undefined` when it holds and a failure
 * message otherwise.
 *
 * The inverse of `noError`, and deliberately code-agnostic. The
 * malformed-response family (#576) is refused by a hand-written guard in
 * TypeScript, Python and Ruby and by the model decoder in Go, Kotlin and
 * Swift; those two mechanisms share no canonical error code, so pinning
 * `errorType` would make the fixture unwritable. What all six agree on is that
 * the call fails at all — which, paired with `requestCount`, is the whole
 * contract: the composite refused the field instead of writing it.
 *
 * NO COMMITTED FIXTURE CAN REACH THE FAILING BRANCH: every case declaring
 * `errorRaised` is one the SDK does refuse, so a handler that accepted
 * everything would report green in all six runners at once. That is the #563
 * shape — a `delayBetweenRequests` check that passed vacuously because no
 * fixture supplied a gap it could fail on — and the reason
 * `make conformance-runner-tests` exists.
 *
 * The message is pinned verbatim by the unit tests in all six runners: a
 * fixture debugged in one language should not read differently in another.
 */
export function errorRaisedFailure(dispatchFailed: boolean): string | undefined {
  return dispatchFailed ? undefined : "Expected the call to fail, but it succeeded";
}
