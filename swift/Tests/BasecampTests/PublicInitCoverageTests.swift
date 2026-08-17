import XCTest

@testable import BasecampGenerator

/// #735: a generated model with no required member got no `public init`, and
/// Swift's implicit memberwise initializer is `internal` — so 35 models, two of
/// them request payloads, were unconstructible outside the module. Every test
/// source here that imports the SDK imports it as `@testable import Basecamp`,
/// which raises internal to visible, and none plain-imports it — so the in-repo
/// suite compiled against a surface no consumer has.
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

    /// The three loops that build a struct — declare, parameterize, assign —
    /// must agree on which properties exist. Only the first two skipped a
    /// property whose schema is not a dictionary; the third would have emitted
    /// `self.x = x` for a parameter that was never declared. Unreachable through
    /// today's `openapi.json`, where every property is a dictionary, but the
    /// init is now emitted for every model rather than only those with a
    /// required member, so the disagreement had more surface to bite on.
    func testNonDictionaryPropertyIsSkippedByAllThreeLoops() {
        let schemas: [String: Any] = [
            "Mixed": [
                "type": "object",
                "properties": [
                    "title": ["type": "string"],
                    "bogus": "not-a-schema",
                ],
            ]
        ]
        let code = emitEntityModel(schemaName: "Mixed", schemas: schemas)

        // Positive control, one per loop: the valid property is declared,
        // parameterized and assigned.
        XCTAssertTrue(code.contains("public var title: String?"), code)
        XCTAssertTrue(code.contains("public init(title: String? = nil) {"), code)
        XCTAssertTrue(code.contains("self.title = title"), code)

        // The non-schema property, asserted per loop so a failure names the one
        // that regressed rather than just "it leaked somewhere".
        XCTAssertFalse(
            code.contains("public var bogus"), "declaration loop emitted it:\n\(code)")
        XCTAssertFalse(code.contains("bogus:"), "init-parameter loop emitted it:\n\(code)")
        XCTAssertFalse(code.contains("self.bogus"), "assignment loop emitted it:\n\(code)")

        // And the blanket check, which is what actually forbids it: the three
        // above name the known emission shapes, this one catches any other.
        XCTAssertFalse(code.contains("bogus"), "a non-schema property must not be emitted:\n\(code)")
    }

    // MARK: - Memberwise-initializer recognition

    /// The stored properties a generated model declares, paired with whether the
    /// caller must supply one at construction. The emitter writes required
    /// members as `let` and optional ones as `var`, so the keyword carries that.
    static func declaredProperties(in source: String) -> [(name: String, required: Bool)] {
        source.components(separatedBy: "\n").compactMap { line in
            let keyword = [("    public let ", true), ("    public var ", false)]
                .first { line.hasPrefix($0.0) }
            return keyword.flatMap { prefix, required in
                line.dropFirst(prefix.count)
                    .components(separatedBy: ":").first
                    .map { (name: $0.trimmingCharacters(in: .whitespaces), required: required) }
            }
        }
    }

    /// The external argument labels of every `public init` the source declares,
    /// one entry per initializer.
    ///
    /// Both shapes the emitter writes are handled: up to three parameters go on
    /// the declaration line, more are wrapped one per line and closed by `) {`.
    /// Accumulating until the first `)` covers each without a second code path,
    /// and covers the hand-written `init(from decoder:) throws` — whose closing
    /// paren is not the end of its line — for free.
    static func publicInitParameterLabels(in source: String) -> [[String]] {
        let lines = source.components(separatedBy: "\n")
        var labels: [[String]] = []
        for (start, line) in lines.enumerated() where line.hasPrefix("    public init(") {
            var declaration = ""
            for candidate in lines[start...] {
                declaration += candidate
                if candidate.contains(")") { break }
            }
            guard let open = declaration.firstIndex(of: "("),
                let close = declaration.lastIndex(of: ")"), open < close
            else { continue }
            labels.append(
                declaration[declaration.index(after: open)..<close]
                    .components(separatedBy: ",")
                    // The label is the first word before the type. A memberwise
                    // parameter's is its only name; `from decoder: any Decoder`
                    // labels itself `from`. An empty list yields no labels.
                    .compactMap {
                        $0.components(separatedBy: ":").first?
                            .components(separatedBy: .whitespaces)
                            .first { !$0.isEmpty }
                    })
        }
        return labels
    }

    /// Whether a consumer outside the module can build this model from its
    /// property values.
    ///
    /// The bar is deliberately *not* "declares some `public init`". Codable's
    /// `public init(from decoder: any Decoder) throws` satisfies that reading,
    /// and nine generated models carry both it and the memberwise initializer —
    /// so a scan for a bare `public init` would stay green after the memberwise
    /// one was deleted from any of them, which is precisely the regression this
    /// file exists to catch. Blocklisting the string `from decoder` would fix
    /// that one spelling and nothing else; recognizing the memberwise form by
    /// what it *is* judges any future initializer by the same rule.
    ///
    /// So: an initializer counts when every parameter it takes names one of the
    /// struct's own properties, and it takes every property the struct requires
    /// at construction. The decoding initializer fails the first clause — no
    /// model declares a property called `from` — and a partial initializer that
    /// omits a `let` member fails the second.
    ///
    /// The last clause closes the way this could pass for the wrong reason: if
    /// label extraction ever broke and returned nothing, "every parameter names
    /// a property" would hold vacuously for every model. A struct with
    /// properties must therefore be built by an initializer that takes at least
    /// one. Both parsers otherwise fail closed — a collapsed property scan makes
    /// every parameter unrecognized, a collapsed init scan leaves no candidate.
    static func declaresMemberwiseInit(in source: String) -> Bool {
        let properties = declaredProperties(in: source)
        let names = Set(properties.map(\.name))
        let required = Set(properties.filter(\.required).map(\.name))

        return publicInitParameterLabels(in: source).contains { labels in
            let taken = Set(labels)
            return taken.isSubset(of: names) && required.isSubset(of: taken)
                && (names.isEmpty || !taken.isEmpty)
        }
    }

    /// Table-drives the matcher over the shapes the emitter writes and the ones
    /// that must not be mistaken for them. The disk scan below can only ever see
    /// the models that exist today, and today none of them is missing an init —
    /// so without these the matcher's negative half is never exercised.
    func testMemberwiseInitRecognition() {
        // `SearchType` in miniature: a required-nullable member gives it both a
        // memberwise and a decoding initializer.
        let both = """
            public struct SearchType: Codable, Sendable {
                public let key: String?
                public let value: String

                public init(key: String?, value: String) {
                    self.key = key
                }

                public init(from decoder: any Decoder) throws {
                    self.key = try container.decode(String?.self, forKey: .key)
                }
            }
            """
        XCTAssertTrue(Self.declaresMemberwiseInit(in: both))

        // The regression: the memberwise initializer is gone and only Codable's
        // remains. A scan for a bare `public init` reads this as constructible,
        // which is why the assertion below is the one that carries the contract.
        let decoderOnly = """
            public struct SearchType: Codable, Sendable {
                public let key: String?
                public let value: String

                public init(from decoder: any Decoder) throws {
                    self.key = try container.decode(String?.self, forKey: .key)
                }
            }
            """
        XCTAssertTrue(
            decoderOnly.components(separatedBy: "\n")
                .contains { $0.hasPrefix("    public init(") },
            "the mutated source must still declare a public init, or this proves nothing")
        XCTAssertFalse(
            Self.declaresMemberwiseInit(in: decoderOnly),
            "init(from decoder:) does not make a model constructible from its values")

        // Over three parameters, the emitter wraps them one per line.
        XCTAssertTrue(
            Self.declaresMemberwiseInit(
                in: """
                    public struct Wrapped: Codable, Sendable {
                        public var a: String?
                        public var b: String?
                        public var c: String?
                        public var d: String?

                        public init(
                            a: String? = nil,
                            b: String? = nil,
                            c: String? = nil,
                            d: String? = nil
                        ) {
                            self.a = a
                        }
                    }
                    """))

        // A struct with no stored properties is fully built by `init()`.
        XCTAssertTrue(
            Self.declaresMemberwiseInit(
                in: """
                    public struct Empty: Codable, Sendable {
                        public init() {
                        }
                    }
                    """))

        // ...but an empty parameter list is not a licence for a struct that has
        // properties, which is what a broken label parser would produce.
        XCTAssertFalse(
            Self.declaresMemberwiseInit(
                in: """
                    public struct Broken: Codable, Sendable {
                        public var title: String?

                        public init() {
                        }
                    }
                    """))

        // An initializer that skips a required member leaves it unsettable.
        XCTAssertFalse(
            Self.declaresMemberwiseInit(
                in: """
                    public struct Partial: Codable, Sendable {
                        public let id: Int
                        public var title: String?

                        public init(title: String? = nil) {
                            self.title = title
                        }
                    }
                    """),
            "a required member absent from the parameter list cannot be supplied")

        // The `FlexibleInt` companion property is emitted alongside the member
        // it labels and deliberately takes no parameter. It is a `var`, so the
        // initializer still covers everything required.
        XCTAssertTrue(
            Self.declaresMemberwiseInit(
                in: """
                    public struct Companion: Codable, Sendable {
                        public let id: FlexibleInt
                        public var systemLabel: String?

                        public init(id: FlexibleInt) {
                            self.id = id
                        }
                    }
                    """))
    }

    // MARK: - Roster scan

    /// Covers models added after the consumer target's roster was written: every
    /// generated `public struct` must carry a memberwise `public init`. Enums and
    /// typealiases are exempt — an enum case is already a public constructor and
    /// a typealias has no initializer of its own.
    func testEveryGeneratedModelStructDeclaresAPublicInit() throws {
        let modelsDir = repoRoot.appendingPathComponent("Sources/Basecamp/Generated/Models")
        let files = try FileManager.default.contentsOfDirectory(atPath: modelsDir.path)
            .filter { $0.hasSuffix(".swift") }
            .sorted()

        // An extraction floor, not a count assertion — and the distinction is the
        // whole point. The real failure it stops is a *collapsed scan*: a renamed
        // or moved Models directory, or a filter that stops matching, leaves
        // `files` empty, the loop below never runs, `missing` stays `[]`, and the
        // test passes while checking nothing. Asserting only "non-empty" would
        // also catch the total collapse, but not a partial one — a scan that
        // finds three files is just as vacuous and looks just as green.
        //
        // So the floor sits an order of magnitude below the true count (220 at
        // the time of writing) rather than just under it. A tight bound would
        // turn ordinary spec churn into an unrelated failure here, which is the
        // brittleness the review flagged; this one only trips if the scan has
        // lost ~90% of the roster, which is never legitimate churn. It is
        // deliberately NOT a tripwire for models being added or removed — the
        // consumer target's roster already fails to compile on a rename, and
        // `missing` below is the assertion that carries the actual contract.
        XCTAssertGreaterThan(
            files.count, 20,
            "only \(files.count) model files found — the scan has collapsed, so "
                + "an empty `missing` below would prove nothing")

        var missing: [String] = []
        var structs = 0
        for file in files {
            let source = try String(
                contentsOf: modelsDir.appendingPathComponent(file), encoding: .utf8)
            let lines = source.components(separatedBy: "\n")
            guard lines.contains(where: { $0.hasPrefix("public struct ") }) else { continue }
            structs += 1
            if !Self.declaresMemberwiseInit(in: source) {
                missing.append(file)
            }
        }

        // Same floor, same reason: the files could all be read and none of them
        // recognized as a struct — a changed emitted prefix would do it — which
        // again leaves `missing` empty for the wrong reason.
        XCTAssertGreaterThan(
            structs, 20,
            "only \(structs) of \(files.count) model files parsed as a public "
                + "struct — the struct detection has collapsed")
        XCTAssertEqual(
            missing, [],
            "these generated models declare no memberwise public init, so no "
                + "consumer that plain-`import`s Basecamp can construct them "
                + "from their property values (#735)")
    }

    // MARK: - Plain-import recognition

    /// Recognizes a plain `import Basecamp` declaration.
    ///
    /// Exact string equality was the first cut and it was too strict: a trailing
    /// comment or a doubled space made a genuine plain import invisible to the
    /// guard. A false negative here is the worst kind — the guard's whole job is
    /// to notice when the consumer target stops importing the SDK the way a
    /// customer does, and an unrecognized-but-real import reads exactly like a
    /// missing one. That is the same brittleness class as the `@testable` blind
    /// spot this PR exists to close, one level up.
    ///
    /// A prefix check would be the easy fix and is wrong in the other direction:
    /// `import BasecampGenerator` starts with `import Basecamp`. So tokenize
    /// instead — drop any `//` comment, ignore surrounding whitespace and a
    /// trailing semicolon, and require exactly the two tokens. That accepts the
    /// real spellings and still rejects the generator import and `@testable`.
    ///
    /// Not handled, deliberately: a `/* … */` block comment mid-declaration.
    /// Nothing in the repo writes one, and the tokenizer would have to become a
    /// lexer to see it. If that ever appears, the guard fails closed — it reports
    /// no plain import, which is a loud failure rather than a silent pass.
    static func isPlainBasecampImport(_ line: String) -> Bool {
        let withoutComment = line.components(separatedBy: "//")[0]
        let tokens =
            withoutComment
            .trimmingCharacters(in: CharacterSet(charactersIn: " \t;"))
            .components(separatedBy: .whitespaces)
            .filter { !$0.isEmpty }
        return tokens == ["import", "Basecamp"]
    }

    /// Table-drives the matcher over the spellings that must and must not count.
    /// The disk scan below cannot cover these — it only ever sees the one line
    /// the consumer target happens to contain today.
    func testPlainImportRecognition() {
        // Real plain imports, however they are spelled.
        XCTAssertTrue(Self.isPlainBasecampImport("import Basecamp"))
        XCTAssertTrue(
            Self.isPlainBasecampImport("import Basecamp  // trailing comment"),
            "a trailing comment does not stop it being a plain import")
        XCTAssertTrue(Self.isPlainBasecampImport("  import   Basecamp  "))
        XCTAssertTrue(Self.isPlainBasecampImport("import Basecamp;"))

        // Not plain imports of this module.
        XCTAssertFalse(
            Self.isPlainBasecampImport("@testable import Basecamp"),
            "@testable is the thing the guard exists to catch")
        XCTAssertFalse(
            Self.isPlainBasecampImport("import BasecampGenerator"),
            "a prefix check would wrongly accept this")
        XCTAssertFalse(Self.isPlainBasecampImport("import Foundation"))
        XCTAssertFalse(Self.isPlainBasecampImport("// import Basecamp"))
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
                if Self.isPlainBasecampImport(line) { sawPlainImport = true }
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
