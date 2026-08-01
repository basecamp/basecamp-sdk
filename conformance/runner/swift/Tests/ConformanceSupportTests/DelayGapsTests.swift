import XCTest

@testable import ConformanceSupport

/// Bounds and every-gap contract for the `delayBetweenRequests` assertion.
///
/// Each case names a behavior that regressed on #563/#568: an assertion that
/// looked like it covered a timing gap and did not. Swift shipped the same
/// gap-0-only evaluator the other three runners did, so the same roster
/// applies here.
final class DelayGapsTests: XCTestCase {
    /// Builds request timestamps from successive gaps in milliseconds.
    private func times(_ gapsMs: UInt64...) -> [UInt64] {
        var out: [UInt64] = [0]
        for ms in gapsMs { out.append(out[out.count - 1] + ms) }
        return out
    }

    func testOmittedIndexCatchesALaterFailingGap() throws {
        // Swift measured gap 0 and stopped, so a second backoff that never
        // happened passed unnoticed.
        let failure = try XCTUnwrap(checkDelayGaps(times(1000, 5), minDelayMs: 500, index: nil))
        XCTAssertTrue(failure.contains("at gap 1"), failure)
    }

    func testOmittedIndexPassesWhenEveryGapClearsTheMinimum() {
        XCTAssertNil(checkDelayGaps(times(1000, 2000, 800), minDelayMs: 500, index: nil))
    }

    func testOmittedIndexFailsWhenThereAreNoGapsAtAll() {
        // An "every gap" rule with no gaps left must not wave the run through:
        // a fully dropped retry lands exactly here. The old evaluator's
        // `if captured.count >= 2` guard made this the silent pass.
        XCTAssertEqual(
            "Expected a delay between requests, but only 1 request(s) were made",
            checkDelayGaps(times(), minDelayMs: 500, index: nil)
        )
    }

    func testNamedGapFailsWhenTheRunNeverProducedIt() {
        XCTAssertEqual(
            "Expected a delay at gap 1, but only 2 request(s) were made",
            checkDelayGaps(times(1000), minDelayMs: 500, index: 1)
        )
    }

    func testNamedGapFailsOnASingleRequestRun() {
        XCTAssertEqual(
            "Expected a delay at gap 0, but only 1 request(s) were made",
            checkDelayGaps(times(), minDelayMs: 500, index: 0)
        )
    }

    func testNegativeGapIndexIsRejected() {
        // Rejected categorically, not wrapped to the end the way
        // headerPresent's index is.
        XCTAssertEqual(
            "delayBetweenRequests gap index must be non-negative, got -1",
            checkDelayGaps(times(1000, 2000), minDelayMs: 500, index: -1)
        )
    }

    func testIntMaxGapIndexFailsWithoutOverflowing() throws {
        // `gap + 1 >= count` computes the addition first; in Swift that traps
        // on Int.max and takes the runner down rather than failing the
        // assertion.
        let failure = try XCTUnwrap(
            checkDelayGaps(times(1000, 2000), minDelayMs: 500, index: Int.max))
        XCTAssertTrue(failure.contains("Expected a delay at gap"), failure)
    }

    func testZeroMinimumStillAssertsThatTheGapExists() {
        // A zero minimum is trivially met; the EXISTENCE requirement is not.
        XCTAssertEqual(
            "Expected a delay between requests, but only 1 request(s) were made",
            checkDelayGaps(times(), minDelayMs: 0, index: nil)
        )
        XCTAssertEqual(
            "Expected a delay at gap 0, but only 1 request(s) were made",
            checkDelayGaps(times(), minDelayMs: 0, index: 0)
        )
        XCTAssertNil(checkDelayGaps(times(5), minDelayMs: 0, index: nil))
    }

    func testAbsentMinimumStillAssertsThatTheGapExists() {
        // The default lands inside the function, so an omitted `min` cannot
        // gate the whole assertion away at the call site.
        XCTAssertEqual(
            "Expected a delay between requests, but only 1 request(s) were made",
            checkDelayGaps(times(), minDelayMs: nil, index: nil)
        )
        XCTAssertNil(checkDelayGaps(times(5), minDelayMs: nil, index: nil))
    }

    func testNamedGapPassesWhenItClearsTheMinimum() {
        XCTAssertNil(checkDelayGaps(times(5, 2000), minDelayMs: 500, index: 1))
    }

    func testNamedGapFailsWhenItIsBelowTheMinimum() throws {
        let failure = try XCTUnwrap(checkDelayGaps(times(2000, 5), minDelayMs: 500, index: 1))
        XCTAssertTrue(failure.contains("at gap 1"), failure)
    }

    func testIntegralMinimumRendersWithoutATrailingDecimal() throws {
        let failure = try XCTUnwrap(checkDelayGaps(times(5), minDelayMs: 1000, index: 0))
        XCTAssertEqual("Expected delay >= 1000ms at gap 0, got 5ms", failure)
    }

    func testEmptyRequestListFailsRatherThanReadingOutOfBounds() {
        XCTAssertEqual(
            "Expected a delay between requests, but only 0 request(s) were made",
            checkDelayGaps([], minDelayMs: 500, index: nil)
        )
        XCTAssertEqual(
            "Expected a delay at gap 0, but only 0 request(s) were made",
            checkDelayGaps([], minDelayMs: 500, index: 0)
        )
    }
}
