import XCTest

@testable import ConformanceSupport

/// Both directions of the errorRaised assertion contract.
///
/// Only the passing direction ever runs against a committed fixture: every case
/// declaring errorRaised is one the SDK does refuse. A handler that accepted
/// everything would therefore look green in all six runners at once, which is
/// exactly how #563 shipped a vacuous delayBetweenRequests check.
final class ErrorRaisedTests: XCTestCase {
    /// Asserted verbatim here and in the five sibling runners. A fixture
    /// debugged in one language should not read differently in another.
    private let message = "Expected the call to fail, but it succeeded"

    func testFailedDispatchSatisfiesTheAssertion() {
        XCTAssertNil(errorRaisedFailure(dispatchFailed: true))
    }

    func testSuccessfulDispatchFailsTheAssertion() {
        // The branch under test. It is unreachable from conformance/tests/, so
        // without this case the handler could accept everything undetected.
        XCTAssertEqual(message, errorRaisedFailure(dispatchFailed: false))
    }
}
