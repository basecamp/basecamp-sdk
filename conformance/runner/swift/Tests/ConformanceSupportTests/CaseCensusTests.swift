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

    func testCensusRejectsAnEmptiedFixture() throws {
        // The one truncation both sides read identically: the runner registers
        // nothing from the file and the census would expect nothing, so the
        // totals fall together and no mismatch appears. Counting it as zero is
        // what would make the whole-file guarantee a lie, so the census refuses
        // it instead.
        try withFixtureTree(["cases.json": fixture, "emptied.json": "[]"]) { dir in
            XCTAssertThrowsError(try CaseCensus.nonLiveCaseCount(in: dir))
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
        // Null-coalescing, not falsiness: an explicit empty mode is not an
        // absent one. Python defaulted on falsiness and ran it.
        XCTAssertFalse(CaseCensus.isMockMode(""))
    }

    func testCensusReportsAnUnreadableSubtree() throws {
        // Without an errorHandler the enumerator skips the unreadable subtree
        // and keeps going, so its cases leave both sides of the census at once
        // and the totals still agree — a fail-closed check failing open.
        //
        // Root reads through a 0o000 directory, so under root there is no
        // unreadable subtree to report and the assertion becomes "the cases are
        // still counted". Either way the census must never silently drop them,
        // which is the failure this pins.
        try withFixtureTree(["cases.json": fixture, "locked/nested.json": fixture]) { dir in
            let locked = dir.appendingPathComponent("locked", isDirectory: true)
            try FileManager.default.setAttributes([.posixPermissions: 0o000], ofItemAtPath: locked.path)
            defer {
                try? FileManager.default.setAttributes(
                    [.posixPermissions: 0o755], ofItemAtPath: locked.path)
            }

            if getuid() == 0 {
                XCTAssertEqual(try CaseCensus.nonLiveCaseCount(in: dir), 4)
            } else {
                XCTAssertThrowsError(try CaseCensus.nonLiveCaseCount(in: dir)) { error in
                    guard case CaseCensus.CensusError.unreadableTree = error else {
                        return XCTFail("expected an unreadable-tree failure; got \(error)")
                    }
                }
            }
        }
    }
}
