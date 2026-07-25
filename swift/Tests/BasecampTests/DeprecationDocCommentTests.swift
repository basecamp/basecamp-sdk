import XCTest

@testable import BasecampGenerator

/// Fixture for the deprecation doc-comment rendering shared by the Swift
/// generator's service and model emitters (#406). A reason sourced from a
/// multi-line OpenAPI description must never produce an un-commented source
/// line, on any of the emitted paths (optional/required query params, model
/// properties, and whole types).
final class DeprecationDocCommentTests: XCTestCase {
    private let multiline = "line one\n\nline three with \"quotes\" and a \\ backslash"

    /// Every emitted line must be a doc comment at the requested indent — no
    /// continuation of a multi-line reason may leak onto a bare source line.
    private func assertAllDocComment(_ lines: [String], indent: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertFalse(lines.isEmpty, file: file, line: line)
        for l in lines {
            XCTAssertTrue(
                l == "\(indent)///" || l.hasPrefix("\(indent)/// "),
                "line is not a \(indent)/// doc comment: \(l)",
                file: file,
                line: line,
            )
        }
    }

    func testOptionalParameterAndPropertyPath() {
        // Options field / model property: indent "    ", default "Deprecated: " leader.
        let lines = deprecationDocLines(reason: multiline, indent: "    ")
        assertAllDocComment(lines, indent: "    ")
        XCTAssertEqual(lines.first, "    /// Deprecated: line one")
        XCTAssertTrue(lines.contains("    ///"), "blank interior line should be a bare doc comment")
    }

    func testRequiredParameterPath() {
        // Required query param: method-level `- Parameter` leader.
        let lines = deprecationDocLines(
            reason: multiline,
            indent: "    ",
            leader: "- Parameter typeNames: Deprecated: ",
        )
        assertAllDocComment(lines, indent: "    ")
        XCTAssertEqual(lines.first, "    /// - Parameter typeNames: Deprecated: line one")
    }

    func testTypePath() {
        // Whole struct: top-level indent "".
        let lines = deprecationDocLines(reason: multiline, indent: "")
        assertAllDocComment(lines, indent: "")
        XCTAssertEqual(lines.first, "/// Deprecated: line one")
    }

    func testSingleLineReasonIsOneLine() {
        let lines = deprecationDocLines(reason: "prefer type_names[].", indent: "    ")
        XCTAssertEqual(lines, ["    /// Deprecated: prefer type_names[]."])
    }
}
