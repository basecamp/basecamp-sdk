import XCTest

@testable import BasecampGenerator

/// #735: a generated model with no required member got no `public init`, and
/// Swift's implicit memberwise initializer is `internal` — so 35 models, two of
/// them request payloads, were unconstructible outside the module. Every file in
/// this test target uses `@testable import`, which raises internal to visible,
/// so the whole in-repo suite compiled against a surface no consumer has.
///
/// Two instruments, deliberately different in what they can see:
///
///   * the unit tests below hold the *emitter's* shape — they run the generator
///     directly, so they fail on the source change alone, before regeneration;
///   * `Sources/BasecampPublicAPIConsumer` holds the *behavior* — it plain-
///     `import`s the SDK, so it observes real access control rather than source
///     text, and it is compiled by `swift build`.
///
/// Neither covers the other's gap. The consumer target names a fixed roster and
/// cannot see a model added next week; the roster scan here sees every model but
/// only as text. The `@testable` guard at the end is what keeps the consumer
/// target from quietly becoming a second copy of this one.
final class PublicInitCoverageTests: XCTestCase {
    private var repoRoot: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()  // BasecampTests
            .deletingLastPathComponent()  // Tests
            .deletingLastPathComponent()  // swift
    }

    // MARK: - Emitter shape

    /// The regression itself: an all-optional entity schema must still get an
    /// explicit `public init`. Against pre-fix `ModelEmitter.swift` this fails.
    func testAllOptionalEntityModelGetsAPublicInit() {
        let schemas: [String: Any] = [
            "GaugeNeedleUpdatePayload": [
                "type": "object",
                "properties": [
                    "description": ["type": "string"]
                ],
            ]
        ]
        let code = emitEntityModel(schemaName: "GaugeNeedleUpdatePayload", schemas: schemas)

        XCTAssertTrue(
            code.contains("public init(description: String? = nil) {"),
            "an all-optional model must be constructible outside the module:\n\(code)")
        XCTAssertTrue(code.contains("self.description = description"), code)
    }

    /// A model with no properties at all is still a struct a consumer may be
    /// handed back and want to build. The zero-parameter init is valid Swift and
    /// must be emitted rather than skipped as a special case.
    func testEmptyEntityModelGetsAZeroParameterPublicInit() {
        let schemas: [String: Any] = [
            "Empty": ["type": "object", "properties": [String: Any]()]
        ]
        let code = emitEntityModel(schemaName: "Empty", schemas: schemas)

        XCTAssertTrue(code.contains("public init() {"), code)
    }

    /// The fix widened *when* an init is emitted; it must not have changed
    /// *what* is emitted for models that already had one. Required members keep
    /// `let` with no default; optional members keep `var` with `= nil`.
    func testRequiredMemberModelKeepsItsInitShape() {
        let schemas: [String: Any] = [
            "GaugeNeedlePayload": [
                "type": "object",
                "required": ["position"],
                "properties": [
                    "position": ["type": "integer", "format": "int32"],
                    "color": ["type": "string"],
                ],
            ]
        ]
        let code = emitEntityModel(schemaName: "GaugeNeedlePayload", schemas: schemas)

        XCTAssertTrue(code.contains("public let position: Int32"), code)
        XCTAssertTrue(code.contains("public var color: String?"), code)
        XCTAssertTrue(
            code.contains("public init(position: Int32, color: String? = nil) {"),
            "required members take no default, optional members default to nil:\n\(code)")
    }

    // MARK: - Roster scan

    /// Covers models added after the consumer target's roster was written: every
    /// generated `public struct` must carry a `public init`. Enums and
    /// typealiases are exempt — an enum case is already a public constructor and
    /// a typealias has no initializer of its own.
    func testEveryGeneratedModelStructDeclaresAPublicInit() throws {
        let modelsDir = repoRoot.appendingPathComponent("Sources/Basecamp/Generated/Models")
        let files = try FileManager.default.contentsOfDirectory(atPath: modelsDir.path)
            .filter { $0.hasSuffix(".swift") }
            .sorted()

        XCTAssertGreaterThan(files.count, 200, "expected the full generated model roster")

        var missing: [String] = []
        var structs = 0
        for file in files {
            let source = try String(
                contentsOf: modelsDir.appendingPathComponent(file), encoding: .utf8)
            let lines = source.components(separatedBy: "\n")
            guard lines.contains(where: { $0.hasPrefix("public struct ") }) else { continue }
            structs += 1
            if !lines.contains(where: { $0.hasPrefix("    public init(") }) {
                missing.append(file)
            }
        }

        XCTAssertGreaterThan(structs, 180, "expected most generated models to be structs")
        XCTAssertEqual(
            missing, [],
            "these generated models have no public init, so no consumer that "
                + "plain-`import`s Basecamp can construct them (#735)")
    }

    // MARK: - Blind-spot guards

    /// The consumer target only proves anything while it imports the SDK the way
    /// a customer does. `@testable` there would raise internal to visible again
    /// and silently restore the exact gap — and it is the obvious way to make a
    /// compile error in that target go away.
    func testConsumerTargetImportsBasecampWithoutTestable() throws {
        let consumerDir = repoRoot.appendingPathComponent("Sources/BasecampPublicAPIConsumer")
        let files = try FileManager.default.contentsOfDirectory(atPath: consumerDir.path)
            .filter { $0.hasSuffix(".swift") }
            .sorted()

        XCTAssertFalse(files.isEmpty, "the public-API consumer target has no sources")

        var sawPlainImport = false
        for file in files {
            let source = try String(
                contentsOf: consumerDir.appendingPathComponent(file), encoding: .utf8)
            for line in source.components(separatedBy: "\n") {
                let trimmed = line.trimmingCharacters(in: .whitespaces)
                XCTAssertFalse(
                    trimmed.hasPrefix("@testable"),
                    "\(file) uses @testable, which re-opens the #735 blind spot this "
                        + "target exists to close")
                if trimmed == "import Basecamp" { sawPlainImport = true }
            }
        }
        XCTAssertTrue(sawPlainImport, "no source in the consumer target plain-imports Basecamp")
    }

    /// Declared with `.target`, not `.testTarget`, on purpose: `swift build`
    /// compiles it, so `make swift-build`, `make swift-check`, the Swift CI job,
    /// the release workflow and the CodeQL Swift build all cover it. Demoting it
    /// to a test target would narrow that to `swift test` without any visible
    /// signal.
    func testConsumerIsDeclaredAsANonTestTarget() throws {
        let manifest = try String(
            contentsOf: repoRoot.appendingPathComponent("Package.swift"), encoding: .utf8)

        guard let nameRange = manifest.range(of: "name: \"BasecampPublicAPIConsumer\"") else {
            return XCTFail("Package.swift does not declare BasecampPublicAPIConsumer")
        }

        let preceding = manifest[manifest.startIndex..<nameRange.lowerBound]
        let kinds = [".target(", ".testTarget(", ".executableTarget(", ".binaryTarget(", ".plugin("]
        let nearest =
            kinds
            .compactMap { kind in
                preceding.range(of: kind, options: .backwards).map { (kind, $0.lowerBound) }
            }
            .max { $0.1 < $1.1 }?.0

        XCTAssertEqual(
            nearest, ".target(",
            "the consumer must stay a plain target so `swift build` compiles it")
    }
}
