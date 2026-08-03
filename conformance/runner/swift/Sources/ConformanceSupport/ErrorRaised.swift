/// Validates one `errorRaised` assertion, returning `nil` when it holds and a
/// failure message otherwise.
///
/// The inverse of `noError`, and deliberately code-agnostic. The
/// malformed-response family (#576) is refused by a hand-written guard in
/// TypeScript, Python and Ruby and by the model decoder in Go, Kotlin and
/// Swift; those two mechanisms share no canonical error code, so pinning
/// `errorType` would make the fixture unwritable. What all six agree on is that
/// the call fails at all — which, paired with `requestCount`, is the whole
/// contract: the composite refused the field instead of writing it.
///
/// It lives here rather than in Assertions.swift for the reason the whole
/// ConformanceSupport target exists: a target carrying `@main` cannot host
/// XCTest cleanly, and this branch never executes against a fixture that
/// passes. NO COMMITTED FIXTURE CAN REACH IT — every case declaring
/// `errorRaised` is one the SDK does refuse, so a handler that accepted
/// everything would report green in all six runners at once. That is the #563
/// shape, and the reason `make conformance-runner-tests` exists.
///
/// `dispatchFailed` is the union of every failure mechanism, not just the ones
/// that produce a `BasecampError`: a `DecodingError` never becomes one, and the
/// HTTPS-enforcement probe observes a trapped child process rather than a throw.
///
/// The message is pinned verbatim by the unit tests in all six runners: a
/// fixture debugged in one language should not read differently in another.
public func errorRaisedFailure(dispatchFailed: Bool) -> String? {
    dispatchFailed ? nil : "Expected the call to fail, but it succeeded"
}
