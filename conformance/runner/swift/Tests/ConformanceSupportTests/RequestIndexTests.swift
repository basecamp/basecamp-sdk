import XCTest

@testable import ConformanceSupport

/// The per-request `index` contract, pinned alongside the delay-gap one
/// because they share a fixture key and disagree about negatives on purpose.
///
/// Characterization rather than regression: this resolver was already correct,
/// and every case here passes against the pre-fix code. It earns its place by
/// making the disagreement with `checkDelayGaps` explicit, so a later "tidy-up"
/// cannot quietly unify them.
final class RequestIndexTests: XCTestCase {
    func testZeroSelectsTheFirstRequest() {
        XCTAssertEqual(0, resolveRequestIndex(0, 3))
    }

    func testPositiveIndexSelectsThatRequest() {
        XCTAssertEqual(2, resolveRequestIndex(2, 3))
    }

    func testNegativeIndexWrapsFromTheEnd() {
        // Unlike checkDelayGaps, which rejects negatives outright.
        XCTAssertEqual(2, resolveRequestIndex(-1, 3))
        XCTAssertEqual(0, resolveRequestIndex(-3, 3))
    }

    func testOutOfRangeIsNilRatherThanClamped() {
        // A clamp would validate a different request than the fixture named,
        // which is the shape that lets a dropped retry pass unnoticed.
        XCTAssertNil(resolveRequestIndex(3, 3))
        XCTAssertNil(resolveRequestIndex(-4, 3))
    }

    func testNoRequestsRecordedResolvesToNil() {
        XCTAssertNil(resolveRequestIndex(0, 0))
        XCTAssertNil(resolveRequestIndex(-1, 0))
    }

    func testExtremeIndexesResolveToNil() {
        // No overflow to guard against here — `count` is a non-negative length
        // and the negative branch only ever moves the sum toward zero, which
        // is why this pair passes against the pre-fix resolver too.
        XCTAssertNil(resolveRequestIndex(Int.min, 3))
        XCTAssertNil(resolveRequestIndex(Int.max, 3))
    }
}
