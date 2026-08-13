import Foundation

/// Case census (#602): every non-live fixture case must be accounted for by the
/// run.
///
///     passed + failed + skipped  ==  every JSON case under conformance/tests,
///                                    recursively, whose mode != "live"
///
/// The left side is what the runner actually did. The right side is counted by
/// `CaseCensus.nonLiveCaseCount` below — a SEPARATE walk and parse, deliberately
/// not the runner's own load path. That independence is the entire point: a
/// check fed by the load path can only confirm the load path agrees with itself.
///
/// Why `mode != "live"` rather than `mode == "mock"`: all six runners select
/// with "mock unless told otherwise" (`isMockMode` here, and its five
/// equivalents), so a typo'd `mode: "moc"` is dropped by every runner at once
/// with nothing printed anywhere. Counting the expected side as "not explicitly
/// live" turns that silent divergence into arithmetic.
///
/// Catches: an unrecognized `mode`; a fixture that failed to parse or was never
/// listed (including one nested below `conformance/tests`, which no runner
/// discovers — hence the recursive walk); a case dropped between load and
/// dispatch; a fixture emptied to `[]` (which the census REFUSES rather than
/// counts — see `nonLiveCaseCount`, and note that counting it would make this
/// bullet a lie); and any future skip channel that bypasses the counters,
/// because the counters are what it reads.
///
/// The typo is not this check's alone to catch, and saying so is what keeps the
/// rest of the list honest: `make conformance-fixtures-check` validates
/// `conformance/tests/*.json` against `conformance/schema.json`, whose `mode` is
/// `enum: ["mock", "live"]`, so a typo in a TOP-LEVEL fixture fails there first
/// and this census is defense in depth for that one case. What that gate
/// structurally cannot see is everything else above — its glob is not recursive,
/// so a fixture nested below `conformance/tests` is validated by nothing AND run
/// by nothing (verified: such a file passes the schema gate and fails this
/// census); a fixture truncated to `[]` is a valid array of zero cases; and a
/// case dropped between load and dispatch is not a fixture-format question at
/// all. Nor does that gate run when `make conformance-<lang>` is invoked alone.
///
/// Does NOT catch the all-six case #602 names — one case every runner excludes
/// for its own reason, which leaves each runner's own census green. That needs
/// the six exclusion sets in one place, hence artifact plumbing across six CI
/// jobs; #602 stays open for it.
///
/// It lives in this SDK-free target for the reason the target exists: a target
/// carrying `@main` cannot host XCTest cleanly, and a check that is green on the
/// real fixture tree by construction can only ever be proven to say NO against a
/// synthetic one (`CaseCensusTests`).
///
/// SWIFT COVERAGE IS macOS-ONLY. `conformance-swift` is guarded by `IS_MACOS` in
/// the Makefile and CI runs it on `macos-15`, so a green Linux run does not
/// exercise this arm at all. Five runners, not six, on Linux.
public enum CaseCensus {
    /// Every fail-closed condition below, as one type: a caller cannot catch
    /// the parse failure and miss the empty-tree one.
    public enum CensusError: Error, CustomStringConvertible {
        case unreadableTree(String)
        case unparseableFixture(String)
        case emptiedFixture(String)
        case noFixtures(String)

        public var description: String {
            switch self {
            case .unreadableTree(let message): message
            case .unparseableFixture(let message): message
            case .emptiedFixture(let message): message
            case .noFixtures(let message): message
            }
        }
    }

    /// One fixture case, reduced to the only field the census reads.
    ///
    /// The census must survive a fixture whose other fields this runner cannot
    /// model, or it would report a failure for a case the run itself handled
    /// fine.
    private struct ModeOnly: Decodable {
        let mode: String?
    }

    /// Whether a fixture case's `mode` selects this runner.
    ///
    /// Absent means mock: live cases are TS-only (the canonical wire-capturer),
    /// and every other value is nobody's. `TestCase.isMock` in the runner calls
    /// through to this, so the rule the run loop applies is the rule under test
    /// rather than a copy of it.
    public static func isMockMode(_ mode: String?) -> Bool {
        (mode ?? "mock") == "mock"
    }

    /// Counts fixture cases whose mode is not `"live"`, recursively.
    ///
    /// Fail-closed in three places, each a way the count could certify nothing
    /// while looking green: an unreadable tree, a fixture that does not parse,
    /// and a walk that found no fixture files at all.
    public static func nonLiveCaseCount(in testsDir: URL) throws -> Int {
        // The errorHandler is not optional decoration. WITHOUT one, the
        // enumerator silently skips a descendant it cannot read and keeps
        // going: the subtree vanishes from the census, the runner never listed
        // it either (it lists only the top level), and the two sides agree on a
        // count that omits it — a fail-closed check quietly failing open.
        // Returning false aborts the walk so the failure is reported.
        var traversalFailure: String?
        guard let walker = FileManager.default.enumerator(
            at: testsDir,
            includingPropertiesForKeys: [.isRegularFileKey],
            options: [],
            errorHandler: { url, error in
                traversalFailure = "could not walk \(url.path): \(error)"
                return false
            }
        ) else {
            throw CensusError.unreadableTree("could not walk \(testsDir.path)")
        }

        var files: [URL] = []
        for case let url as URL in walker where url.pathExtension == "json" {
            let isRegular = (try? url.resourceValues(forKeys: [.isRegularFileKey]))?.isRegularFile
            if isRegular == true { files.append(url) }
        }
        if let traversalFailure {
            throw CensusError.unreadableTree(traversalFailure)
        }
        guard !files.isEmpty else {
            throw CensusError.noFixtures("no *.json fixture files found under \(testsDir.path)")
        }

        var cases = 0
        for url in files.sorted(by: { $0.path < $1.path }) {
            let parsed: [ModeOnly]
            do {
                parsed = try JSONDecoder().decode([ModeOnly].self, from: try Data(contentsOf: url))
            } catch {
                throw CensusError.unparseableFixture("\(url.path): \(error)")
            }
            // An emptied fixture is REFUSED, not counted as zero, and this is
            // the one rejection that carries the whole-file guarantee. It is
            // the single truncation both sides of the census read identically:
            // the runner registers nothing from the file and the census expects
            // nothing, so the two totals fall together and no mismatch ever
            // appears. Counting it would make "a fixture truncated to []" a
            // claim this check cannot keep. A file declaring no cases tests
            // nothing, so refusing it costs nothing — and it closes the same
            // hole in conformance-fixtures-check, where an empty array is a
            // schema-valid list of zero items.
            guard !parsed.isEmpty else {
                throw CensusError.emptiedFixture(
                    "\(url.path): fixture declares no cases; delete the file or restore its cases")
            }
            cases += parsed.filter { $0.mode != "live" }.count
        }
        return cases
    }

    /// Compares what the run accounted for against the census, returning `nil`
    /// when they agree and a message naming the short side otherwise.
    public static func countFailure(ran: Int, expected: Int) -> String? {
        if ran == expected { return nil }
        if ran < expected {
            return """
                case census: the run accounted for \(ran) case(s) (passed+failed+skipped) \
                but conformance/tests holds \(expected) non-live case(s) — \
                \(expected - ran) executed by nothing. An unrecognized `mode`, a fixture \
                that failed to parse or was never listed, or a case dropped between load \
                and dispatch will do this.
                """
        }
        return """
            case census: the run accounted for \(ran) case(s) (passed+failed+skipped) \
            but conformance/tests holds only \(expected) non-live case(s) — \
            \(ran - expected) more than the fixtures declare.
            """
    }
}
