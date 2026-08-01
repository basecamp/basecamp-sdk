import Basecamp
import Foundation

/// Default account ID for conformance tests.
private let testAccountID = "999"

/// Tests the Swift runner must skip, with reasons. Keep near-empty: Swift is
/// three-gate (status, network, and idempotent-POST retry), so capability
/// skips are rare.
///
/// SWIFT_CONFORMANCE_NO_SKIPS=1 empties the roster so a skip can be proven to
/// still fail (or shown ready to flip live once the capability lands).
private let swiftSkips: [String: String] = ProcessInfo.processInfo.environment["SWIFT_CONFORMANCE_NO_SKIPS"] != nil ? [:] : [
    // TEMPORARY until B4 (Wave 2) threads retry through the download first
    // hop: performDownloadRequest is one-shot today, so hop-1 retry fixtures
    // fail against a correct runner. Flip these live when B4 merges.
    "DownloadURL retries on 503 at the auth'd first hop":
        "Swift download hop 1 does not retry yet (B4)",
    "DownloadURL honors Retry-After on 429 at the auth'd first hop":
        "Swift download hop 1 does not retry yet (B4)",
]

@main
struct Runner {
    static func main() async {
        // Child-process mode for the HTTPS-enforcement probe: constructing a
        // client with a non-HTTPS, non-localhost base URL must trap
        // (preconditionFailure), which only a subprocess can observe. Exit 0
        // means construction survived — the parent treats that as a failure.
        if CommandLine.arguments.count >= 3, CommandLine.arguments[1] == "--https-probe" {
            _ = BasecampClient(
                tokenProvider: StaticTokenProvider("conformance-test-token"),
                userAgent: "basecamp-conformance-runner/1.0",
                config: BasecampConfig(baseURL: CommandLine.arguments[2]),
                transport: ScriptedTransport(responses: [], autoPaginates: false)
            )
            exit(0)
        }

        let testsDir = URL(fileURLWithPath: "../../tests", isDirectory: true)
        let files: [URL]
        do {
            files = try FileManager.default
                .contentsOfDirectory(at: testsDir, includingPropertiesForKeys: nil)
                .filter { $0.pathExtension == "json" }
                .sorted { $0.lastPathComponent < $1.lastPathComponent }
        } catch {
            print("No test files found in \(testsDir.path): \(error)")
            exit(1)
        }
        if files.isEmpty {
            print("No test files found in \(testsDir.path)")
            exit(1)
        }

        var passed = 0
        var failed = 0
        var skipped = 0

        for file in files {
            // Live tests are TS-only (canonical wire-capturer); filter them out
            // so the offline Swift runner never sees unresolved ${...} fixtures.
            let testCases: [TestCase]
            do {
                let data = try Data(contentsOf: file)
                testCases = try JSONDecoder().decode([TestCase].self, from: data).filter(\.isMock)
            } catch {
                print("\n=== \(file.lastPathComponent) ===")
                print("  FAIL: could not decode fixture file: \(error)")
                failed += 1
                continue
            }
            if testCases.isEmpty { continue }
            print("\n=== \(file.lastPathComponent) ===")

            for tc in testCases {
                // The Swift SDK auto-paginates list operations (like TS and
                // Kotlin), so tests that assert requestCount=1 with Link
                // headers are not applicable.
                if tc.allTags.contains("link-header") {
                    skipped += 1
                    print("  SKIP: \(tc.name)")
                    print("        Swift SDK auto-paginates (follows Link headers by design)")
                    continue
                }
                if let reason = swiftSkips[tc.name] {
                    skipped += 1
                    print("  SKIP: \(tc.name)")
                    print("        \(reason)")
                    continue
                }

                let result = await runTest(tc)
                if result.skipped {
                    skipped += 1
                    print("  SKIP: \(tc.name)")
                    print("        \(result.message)")
                } else if result.passed {
                    passed += 1
                    print("  PASS: \(tc.name)")
                } else {
                    failed += 1
                    print("  FAIL: \(tc.name)")
                    print("        \(result.message)")
                }
            }
        }

        print("\n=== Summary ===")
        print("Passed: \(passed), Failed: \(failed), Skipped: \(skipped), Total: \(passed + failed + skipped)")

        exit(failed > 0 ? 1 : 0)
    }

    static func runTest(_ tc: TestCase) async -> TestResult {
        // Defense-in-depth backstop for the operationally-harmful mockResponses
        // shapes: neither mode set (would be served as `status ?? 200`, a false
        // positive) or both active. The AUTHORITATIVE oneOf enforcement is
        // `make conformance-fixtures-check`, which runs before the runners.
        for (i, mock) in tc.responses.enumerated() {
            if (mock.status != nil) == mock.isNetworkError {
                return .fail("mockResponses[\(i)] must set exactly one of status or networkError (got status=\(mock.status.map(String.init) ?? "nil"), networkError=\(mock.isNetworkError))")
            }
        }

        // Detect if the fixture uses Link next headers (SDK will auto-paginate).
        let autoPaginates = tc.responses.contains { mock in
            mock.allHeaders.contains { key, value in
                key.lowercased() == "link" && value.contains("rel=\"next\"")
            }
        }

        let transport = ScriptedTransport(responses: tc.responses, autoPaginates: autoPaginates)
        let baseURL = tc.configOverrides?.baseUrl ?? "http://localhost:3000"

        var caughtError: BasecampError?
        var httpStatus: Int?
        var dispatch = DispatchResult()

        if requiresHTTPSCrashProbe(baseURL) {
            // The SDK enforces HTTPS with preconditionFailure — a trap, not a
            // thrown error — so it can only be observed from outside the
            // process. The probe re-runs this binary in --https-probe mode and
            // expects the child to die; a surviving child means enforcement
            // did not fire.
            switch runHTTPSProbe(baseURL) {
            case .enforced:
                caughtError = .usage(message: "Base URL must use HTTPS: \(baseURL)", hint: nil)
            case .constructionSucceeded:
                return .fail("client construction with non-HTTPS base URL unexpectedly succeeded")
            case .probeFailure(let message):
                return .fail("HTTPS probe failed to run: \(message)")
            }
        } else {
            let client = BasecampClient(
                tokenProvider: StaticTokenProvider("conformance-test-token"),
                userAgent: "basecamp-conformance-runner/1.0",
                config: BasecampConfig(
                    baseURL: baseURL,
                    maxPages: tc.configOverrides?.maxPages ?? 10_000
                ),
                transport: transport
            )
            let account = client.forAccount(testAccountID)

            do {
                dispatch = try await dispatchOperation(tc, account)
                httpStatus = transport.lastConsumedIndex.flatMap { tc.responses[$0].status }
            } catch let error as BasecampError {
                caughtError = error
                httpStatus = error.httpStatusCode
            } catch let error as DecodingError {
                // A mock body that fails the model's required-field validation
                // is a fixture bug, not a runner limitation: fail loudly so it
                // gets fixed (canonical bodies live in spec/fixtures/) instead
                // of silently degrading coverage. Kotlin's #555 policy, adopted
                // from day one.
                return .fail("Mock body lacks required Swift model fields: \(describeDecodingError(error))")
            } catch {
                return .fail("Unexpected exception: \(type(of: error)): \(error)")
            }
        }

        return evaluateAssertions(
            tc,
            transport: transport,
            caughtError: caughtError,
            httpStatus: httpStatus,
            dispatch: dispatch,
            autoPaginates: autoPaginates
        )
    }
}

// MARK: - HTTPS enforcement probe

private enum HTTPSProbeOutcome {
    case enforced
    case constructionSucceeded
    case probeFailure(String)
}

/// Mirrors the SDK's localhost carve-out (loopback, *.localhost per RFC 6761,
/// HTTP(S)-only) just to ROUTE the test: carved-out URLs are safe to construct
/// in-process; everything else must go through the crash probe. The probe
/// itself exercises the SDK's real enforcement, so a routing mistake here
/// surfaces as a loud failure, never a silent pass.
private func requiresHTTPSCrashProbe(_ baseURL: String) -> Bool {
    guard let url = URL(string: baseURL), url.scheme?.lowercased() == "http" else {
        return false
    }
    guard var host = url.host?.lowercased() else { return true }
    if host.hasPrefix("["), host.hasSuffix("]") {
        host = String(host.dropFirst().dropLast())
    }
    let isLocal = host == "localhost" || host == "127.0.0.1" || host == "::1"
        || host.hasSuffix(".localhost")
    return !isLocal
}

private func runHTTPSProbe(_ baseURL: String) -> HTTPSProbeOutcome {
    let probe = Process()
    probe.executableURL = URL(fileURLWithPath: CommandLine.arguments[0])
    probe.arguments = ["--https-probe", baseURL]
    probe.standardError = Pipe()  // suppress the expected crash banner
    probe.standardOutput = Pipe()
    do {
        try probe.run()
        probe.waitUntilExit()
    } catch {
        return .probeFailure("\(error)")
    }
    let crashed = probe.terminationReason == .uncaughtSignal || probe.terminationStatus != 0
    return crashed ? .enforced : .constructionSucceeded
}

// MARK: - Decoding-error rendering

/// Renders a DecodingError with the missing key and coding path, which is the
/// actionable part when a fixture body under-specifies a model.
private func describeDecodingError(_ error: DecodingError) -> String {
    func renderPath(_ path: [any CodingKey]) -> String {
        path.map(\.stringValue).joined(separator: ".")
    }
    switch error {
    case .keyNotFound(let key, let context):
        return "missing key \"\(key.stringValue)\" at \(renderPath(context.codingPath))"
    case .typeMismatch(let type, let context):
        return "type mismatch (expected \(type)) at \(renderPath(context.codingPath)): \(context.debugDescription)"
    case .valueNotFound(let type, let context):
        return "null for non-optional \(type) at \(renderPath(context.codingPath))"
    case .dataCorrupted(let context):
        return "corrupted data at \(renderPath(context.codingPath)): \(context.debugDescription)"
    @unknown default:
        return "\(error)"
    }
}
