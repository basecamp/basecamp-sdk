import Foundation

/// One runner's exclusion set for the cross-runner gate (#602).
///
/// The case census answers "did THIS runner account for every case". A case
/// every runner excludes leaves all six censuses green, because each one
/// counted its own skip — only `scripts/check-fixture-execution.rb`, comparing
/// these manifests, can see it.
///
/// `executed` is recorded alongside the exclusions and asserted against the
/// census total. Without it a case a runner silently dropped is simply absent
/// from the exclusion set, and "absent" reads identically to "ran fine" — the
/// collecting gate would conclude the case is covered by this runner precisely
/// when it was not.
///
/// Swift excludes a case through THREE paths — the `link-header` tag branch,
/// the `swiftSkips` roster, and a runtime `TestResult.skipped` — and all three
/// record here, from the same branches that increment `skipped`.
///
/// It lives in this SDK-free target so the writer's own failure modes are
/// unit-testable, for the reason the target exists.
public enum ExecutionManifest {
    public struct Exclusion: Codable, Sendable, Equatable {
        public let name: String
        public let reason: String

        public init(name: String, reason: String) {
            self.name = name
            self.reason = reason
        }
    }

    public enum ManifestError: Error, CustomStringConvertible {
        case inconsistent(String)
        case unwritable(String)

        public var description: String {
            switch self {
            case .inconsistent(let message): message
            case .unwritable(let message): message
            }
        }
    }

    private struct Body: Codable {
        let runner: String
        let total_non_live: Int
        let executed: Int
        let excluded: [Exclusion]
    }

    /// Sorted, so a re-run is byte-identical.
    public static func write(
        runner: String, total: Int, executed: Int, excluded: [Exclusion], to directory: URL
    ) throws {
        guard executed + excluded.count == total else {
            throw ManifestError.inconsistent(
                "manifest for \(runner) is internally inconsistent: \(executed) executed + "
                    + "\(excluded.count) excluded != \(total) non-live cases; the run dropped a "
                    + "case without recording it as either")
        }

        let body = Body(
            runner: runner, total_non_live: total, executed: executed,
            excluded: excluded.sorted { $0.name < $1.name })

        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]

        do {
            try FileManager.default.createDirectory(
                at: directory, withIntermediateDirectories: true)
            var data = try encoder.encode(body)
            data.append(0x0A)
            try data.write(to: directory.appendingPathComponent("\(runner).json"))
        } catch {
            throw ManifestError.unwritable("could not write \(runner) manifest: \(error)")
        }
    }
}
