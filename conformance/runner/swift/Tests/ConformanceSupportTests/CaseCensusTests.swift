import Foundation
import XCTest

@testable import ConformanceSupport

/// Case-census contract (#602).
///
/// The check is green on the real fixture tree by construction, so a live run
/// only ever proves it can say yes. These cases run it against a SYNTHETIC
/// fixture set and prove it can say no — the `mode: "moc"` case in particular,
/// which every runner's "mock unless told otherwise" filter drops with nothing
/// printed. That divergence is asserted here against `isMockMode`, the predicate
/// `TestCase.isMock` in the run loop calls through to.
final class CaseCensusTests: XCTestCase {
    /// One case of each kind: a plain mock case (no `mode` at all, the common
    /// spelling), a live case the runners are meant to drop, and a typo'd mode
    /// that nothing recognizes.
    private let fixture = """
        [
          {"name": "plain", "operation": "GetProject"},
          {"name": "live one", "operation": "GetProject", "mode": "live"},
          {"name": "typo", "operation": "GetProject", "mode": "moc"}
        ]
        """

    /// Builds a throwaway fixture tree. Setup uses `try`, not `try?`: a write
    /// that silently failed would leave a tree the census legitimately reports
    /// as empty, turning a broken test into a passing one.
    private func withFixtureTree(
        _ files: [String: String], _ body: (URL) throws -> Void
    ) throws {
        let dir = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("case-census-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

        for (relative, content) in files {
            let file = dir.appendingPathComponent(relative)
            try FileManager.default.createDirectory(
                at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
            try Data(content.utf8).write(to: file)
        }
        try body(dir)
    }

    func testCensusCountsEveryCaseThatIsNotExplicitlyLive() throws {
        try withFixtureTree(["cases.json": fixture]) { dir in
            XCTAssertEqual(try CaseCensus.nonLiveCaseCount(in: dir), 2)
        }
    }

    func testATypoedModeMakesTheCountCheckFail() throws {
        // The regression this whole check exists for. The run loop's filter
        // keeps one case; the census counts two; the difference is the case
        // executed by nothing.
        try withFixtureTree(["cases.json": fixture]) { dir in
            let modes: [String?] = [nil, "live", "moc"]
            let ran = modes.filter { CaseCensus.isMockMode($0) }.count
            XCTAssertEqual(ran, 1, "the run loop should keep only the plain case")

            let failure = CaseCensus.countFailure(
                ran: ran, expected: try CaseCensus.nonLiveCaseCount(in: dir))

            let message = try XCTUnwrap(
                failure, "a case no runner recognizes must fail the count check")
            XCTAssertTrue(
                message.contains("1 executed by nothing"),
                "failure should name how many cases went unrun; got \(message)")
        }
    }

    func testCensusFindsFixturesNestedBelowTheTestsDirectory() throws {
        // No runner lists recursively, so a case parked one directory down is
        // run by nothing. The census walks, which is what makes that visible.
        try withFixtureTree(["nested/cases.json": fixture]) { dir in
            XCTAssertEqual(try CaseCensus.nonLiveCaseCount(in: dir), 2)
        }
    }

    func testCensusRejectsAFixtureThatDoesNotParse() throws {
        try withFixtureTree(["broken.json": #"[{"name": "truncated""#]) { dir in
            XCTAssertThrowsError(try CaseCensus.nonLiveCaseCount(in: dir))
        }
    }

    func testCensusRejectsAFixtureThatIsNotAnArray() throws {
        try withFixtureTree(["object.json": #"{"name": "not a list"}"#]) { dir in
            XCTAssertThrowsError(try CaseCensus.nonLiveCaseCount(in: dir))
        }
    }

    func testCensusRejectsAnEmptyTree() throws {
        // A census that counted nothing certifies nothing: zero on both sides
        // is the shape a broken walk takes.
        try withFixtureTree([:]) { dir in
            XCTAssertThrowsError(try CaseCensus.nonLiveCaseCount(in: dir))
        }
    }

    func testCensusAcceptsATruncatedFixtureAsZeroCases() throws {
        // `[]` parses, so the census counts zero for it — and the runner counts
        // zero too. The mismatch this produces is against the OTHER files'
        // cases, which is why the count is taken over the whole tree rather
        // than per file.
        try withFixtureTree(["empty.json": "[]", "cases.json": fixture]) { dir in
            XCTAssertEqual(try CaseCensus.nonLiveCaseCount(in: dir), 2)
        }
    }

    func testCountFailureAcceptsAgreement() {
        XCTAssertNil(CaseCensus.countFailure(ran: 42, expected: 42))
    }

    func testCountFailureNamesAnOverCount() throws {
        let message = try XCTUnwrap(CaseCensus.countFailure(ran: 43, expected: 42))
        XCTAssertTrue(
            message.contains("1 more than the fixtures declare"),
            "failure should name the over-count; got \(message)")
    }

    func testIsMockModeTreatsAbsenceAsMock() {
        XCTAssertTrue(CaseCensus.isMockMode(nil))
        XCTAssertTrue(CaseCensus.isMockMode("mock"))
        XCTAssertFalse(CaseCensus.isMockMode("live"))
        // The census is what catches this one; the filter must not run it.
        XCTAssertFalse(CaseCensus.isMockMode("moc"))
    }
}
