import XCTest

@testable import ConformanceSupport

/// The `{{httpdate+Ns}}` header token (SPEC §19, conformance/schema.json).
///
/// A static fixture has no clock, so the positive half of SPEC §6's HTTP-date
/// branch was unpinnable until this token (#780). These cases pin the
/// resolver's arithmetic against a frozen instant so the fixture's one-sided
/// timing floor rests on a deterministic contract.
final class HeaderTokensTests: XCTestCase {
    /// A quarter-second into 10:18:14 UTC, so floor and round-up differ.
    private let now = Date(timeIntervalSince1970: 1_623_233_894.25)

    func testPlainValuesPassThrough() throws {
        for value in ["", "2", "Wed, 09 Jun 2021 10:18:14 GMT", "application/json", "{not a token}"] {
            XCTAssertEqual(try resolveHeaderValue(value, now: now), value)
        }
    }

    func testHttpdateResolvesToTheWholeSecondPastN() throws {
        XCTAssertEqual(try resolveHeaderValue("{{httpdate+2s}}", now: now), "Wed, 09 Jun 2021 10:18:17 GMT")
        XCTAssertEqual(try resolveHeaderValue("{{httpdate+0s}}", now: now), "Wed, 09 Jun 2021 10:18:15 GMT")
        XCTAssertEqual(try resolveHeaderValue("{{httpdate+10s}}", now: now), "Wed, 09 Jun 2021 10:18:25 GMT")
    }

    func testUnknownTokensAreErrorsNotLiterals() {
        for value in ["{{httpdate}}", "{{httpdate+2}}", "{{httpdate-2s}}", "{{now}}", "{{}}"] {
            XCTAssertThrowsError(try resolveHeaderValue(value, now: now), value) { error in
                XCTAssertTrue("\(error)".contains(value), "error for \(value) does not name the token: \(error)")
            }
        }
    }
}
