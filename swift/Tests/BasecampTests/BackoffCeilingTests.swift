import XCTest
@testable import Basecamp

/// SPEC §7 "Backoff Ceiling" (#577).
///
/// Swift's failure mode is the worst of the six SDKs and was tracked nowhere:
/// `baseDelayMs * (1 << UInt64(attempt - 1))` **traps**. `<<` on an unsigned
/// integer is a smart shift, so an over-shift silently yields `0` — a tight
/// retry loop against a server already answering 429/503 — but at `1 << 63` the
/// multiply overflows `UInt64` and the process dies with
/// "Swift runtime failure: arithmetic overflow". A crash is never the right
/// answer to a run of 503s.
final class BackoffCeilingTests: XCTestCase {

    /// With the 1000ms default base, attempt 6 is the first whose unclamped
    /// term (32,000ms) exceeds the 30,000ms ceiling, so every attempt from
    /// there on must sit at the cap.
    ///
    /// The bound is two-sided on purpose: a one-sided "never too long" check
    /// would pass on the over-shift attempts, which return 0 and are the tight
    /// loop itself.
    func testExponentialBackoffSaturates() {
        for attempt in [6, 10, 40, 62, 63, 64, 65, 128, Int.max] {
            let delay = HTTPClient.backoffDelayMs(
                baseDelayMs: 1_000, backoff: .exponential, attempt: attempt)
            XCTAssertEqual(
                delay, HTTPClient.maxBackoffDelayMs,
                "attempt \(attempt) produced \(delay)ms")
        }
    }

    /// The delays a shipped configuration actually reaches are untouched:
    /// `behavior-model.json` tops out at `base_delay_ms: 2000` over at most
    /// three attempts, so nothing on a default path approaches the ceiling.
    func testDelaysBelowTheCeilingAreExact() {
        for (attempt, want) in [(1, UInt64(1_000)), (2, 2_000), (3, 4_000)] {
            XCTAssertEqual(
                HTTPClient.backoffDelayMs(baseDelayMs: 1_000, backoff: .exponential, attempt: attempt),
                want)
        }
    }

    func testLinearAndConstantBackoffAreClampedToo() {
        XCTAssertEqual(
            HTTPClient.backoffDelayMs(baseDelayMs: 1_000, backoff: .linear, attempt: Int.max),
            HTTPClient.maxBackoffDelayMs)
        // SPEC §7 requirement 3: the ceiling binds base_delay_ms itself, with
        // no carve-out for the first sleep.
        XCTAssertEqual(
            HTTPClient.backoffDelayMs(baseDelayMs: 600_000, backoff: .constant, attempt: 1),
            HTTPClient.maxBackoffDelayMs)
    }

    /// A zero base delay must stay at zero rather than saturating — and must
    /// not divide by zero on the way there.
    func testZeroBaseDelayStaysZero() {
        XCTAssertEqual(
            HTTPClient.backoffDelayMs(baseDelayMs: 0, backoff: .exponential, attempt: 90), 0)
    }

    /// `UInt64(attempt - 1)` traps on a negative operand, so a zero or negative
    /// attempt must be floored rather than converted.
    func testNonPositiveAttemptDoesNotTrap() {
        XCTAssertEqual(
            HTTPClient.backoffDelayMs(baseDelayMs: 1_000, backoff: .exponential, attempt: 0), 1_000)
    }
}
