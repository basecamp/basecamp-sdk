import Basecamp
import Foundation

/// Default account ID for conformance tests. Not private: the path invariant
/// in the evaluator needs it to reconstruct the account-scoped URL the SDK
/// builds from an account-relative fixture path.
let testAccountID = "999"

/// Operations whose dispatch arm passes `configOverrides.maxItems` into the
/// SDK. Kept beside the backstop that enforces it so the two cannot drift:
/// widening one without the other is caught the first time a fixture asks.
private let operationsHonoringMaxItems: Set<String> = ["ListProjects"]

/// Same contract for `configOverrides.page`, which is a stronger claim: a
/// fixture setting it asserts a SINGLE request, so an unthreaded page would
/// let the SDK walk the whole collection while the fixture believed it had
/// pinned one page.
private let operationsHonoringPage: Set<String> = ["ListProjects"]

/// Temporary capability skips, keyed by exact test name.
///
/// EMPTY, and meant to stay that way. Swift is three-gate (status, network and
/// idempotent-POST retry) and since #563 retries the authenticated download hop
/// too, so no fixture asks for a capability the SDK lacks. The one standing
/// exclusion is architectural rather than a gap — the `link-header` tag branch
/// in the run loop, which no name-keyed entry can express.
private let temporarySkips: [String: String] = [:]

/// The roster the run loop consults. `SWIFT_CONFORMANCE_NO_SKIPS=1` empties it,
/// so a temporary skip can be proven genuine before it is added and proven
/// ready to flip once the capability lands. With `temporarySkips` empty the
/// switch is a no-op — it is the mechanism kept live, not a claim that anything
/// is being skipped.
///
/// The value is compared exactly: an inherited empty or `=0` variable must not
/// quietly change what the suite covers.
private let swiftSkips: [String: String] =
    ProcessInfo.processInfo.environment["SWIFT_CONFORMANCE_NO_SKIPS"] == "1" ? [:] : temporarySkips

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
        // Defense-in-depth backstops for fixture shapes that would otherwise
        // produce a PASS while testing nothing. The AUTHORITATIVE enforcement
        // is `make conformance-fixtures-check` against conformance/schema.json,
        // which runs before the runners — but a runner that reports green on a
        // fixture it cannot honor is the failure mode this whole rock exists to
        // remove, so it fails loudly here too.

        // A case with no assertions runs an operation and verifies nothing.
        // An empty `assertions: []` is schema-legal, so the schema gate does
        // not catch it.
        if tc.allAssertions.isEmpty {
            return .fail("test case declares no assertions — it would pass without verifying anything")
        }

        // An EMPTY response queue is a deliberate declaration (the HTTPS
        // enforcement case makes no request at all); an ABSENT key is a
        // malformed fixture, and the two must not collapse.
        if !tc.declaresMockResponses {
            return .fail("mock test case is missing mockResponses (an empty queue must be stated explicitly)")
        }

        for (i, mock) in tc.responses.enumerated() {
            // The schema pins the literal `true`. `networkError: false`
            // alongside a status otherwise reads as a plain success and slips
            // past the exactly-one-of check below.
            if let flag = mock.networkError, !flag {
                return .fail("mockResponses[\(i)]: networkError must be the literal true when present, got false")
            }
            // Neither mode set would be served as `status ?? 200`, a false
            // positive; both active is ambiguous.
            if (mock.status != nil) == mock.isNetworkError {
                return .fail("mockResponses[\(i)] must set exactly one of status or networkError (got status=\(mock.status.map(String.init) ?? "nil"), networkError=\(mock.isNetworkError))")
            }
        }

        // maxItems reaches the SDK only through the per-operation options of
        // the arms that thread it. Any other operation would run UNBOUNDED
        // pagination while the fixture believed it had capped the walk, so the
        // request-count assertion would be measuring something else entirely.
        // Fail rather than silently ignore; add the operation to the dispatch
        // arm and to this roster together.
        if tc.configOverrides?.maxItems != nil, !operationsHonoringMaxItems.contains(tc.operation) {
            return .fail("configOverrides.maxItems is set but \(tc.operation)'s dispatch does not thread it through — it would paginate unbounded")
        }

        if tc.configOverrides?.page != nil, !operationsHonoringPage.contains(tc.operation) {
            return .fail("configOverrides.page is set but \(tc.operation)'s dispatch does not thread it through — it would paginate past the pinned page")
        }

        // Detect if the fixture uses Link next headers (SDK will auto-paginate).
        // This only relaxes the TRANSPORT, which answers an over-walk with a
        // terminal empty page instead of a 500. The count assertion stays exact,
        // so an SDK that fetches too many pages reports a clean
        // "Expected N requests, got N+1" rather than a decode error.
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

        // Set when the dispatch failed by ANY mechanism, including a decoder
        // rejection that never becomes a BasecampError. Only `errorRaised`
        // reads it; errorType, errorCode and errorMessage still read
        // caughtError, so a fixture pinning a canonical code cannot be
        // satisfied by a raw DecodingError.
        var dispatchFailed = false
        // A fixture declaring errorRaised is asserting that the body is
        // DELIBERATELY malformed and that refusing it is the behaviour under
        // test (#576). That flips the usual reading of a decoder rejection
        // below: it is the point of the case, not an under-specified mock body
        // to repair.
        let expectsFailure = tc.allAssertions.contains { $0.type == "errorRaised" }

        if requiresHTTPSCrashProbe(baseURL) {
            // The SDK enforces HTTPS with preconditionFailure — a trap, not a
            // thrown error — so it can only be observed from outside the
            // process. The probe re-runs this binary in --https-probe mode and
            // expects the child to die; a surviving child means enforcement
            // did not fire.
            switch runHTTPSProbe(baseURL) {
            case .enforced:
                caughtError = .usage(message: "Base URL must use HTTPS: \(baseURL)", hint: nil)
                // The child died as required, so the dispatch DID fail — by a
                // trap rather than a throw. Recording only caughtError left
                // this the one failure path that did not flag itself, and an
                // errorRaised fixture with an http:// configOverrides.baseUrl
                // would have read it as a call that succeeded.
                dispatchFailed = true
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
                    // Swift exposes no numeric cap by design (SPEC section 2
                    // validation step 4): its loop is driven by the per-operation
                    // retry.max ceiling, so the cap maps onto the on/off knob that
                    // IS its spelling of the same contract.
                    //
                    // The cutoff is > 1, not > 0, because the key is a TOTAL
                    // attempt count: 0 and 1 both mean exactly one attempt, and
                    // "one attempt" is what enableRetry: false spells here.
                    // Mapping 1 to enabled would hand this runner the
                    // per-operation ceiling of 2-3 attempts while the four
                    // numeric SDKs made exactly one. Matches shouldEnableRetry
                    // in the TypeScript runner.
                    enableRetry: (tc.configOverrides?.maxRetries).map { $0 > 1 } ?? true,
                    maxPages: tc.configOverrides?.maxPages ?? 10_000
                ),
                transport: transport
            )
            let account = client.forAccount(testAccountID)

            do {
                dispatch = try await dispatchOperation(tc, account)
                httpStatus = transport.lastConsumedIndex.flatMap { tc.responses[$0].status }
            } catch let error as BasecampError {
                // Since #604 the SDK maps a body the model refuses into the
                // SPEC §6 malformed-2xx-body shape — a statusless `.api` —
                // rather than letting the `DecodingError` out raw. That arrives
                // here, not in the `DecodingError` branch below, so the #555
                // policy is re-applied at this seam: routing it onward would
                // both lose the loud "fix the fixture body" failure and let a
                // fixture pinning `errorCode: api_error` be satisfied by a
                // decoder rejection, which is exactly what `caughtError` is
                // withheld to prevent.
                if let decodeFailure = malformedBodyMessage(error) {
                    guard expectsFailure else {
                        return .fail("Mock body lacks required Swift model fields: \(decodeFailure)")
                    }
                    dispatchFailed = true
                } else {
                    caughtError = error
                    dispatchFailed = true
                    httpStatus = error.httpStatusCode
                }
            } catch let error as RunnerError {
                // A fixture the dispatch table cannot honor as written: an
                // unknown operation, or a parameter that would have been
                // coerced into a call against the wrong resource. Both are
                // fixture bugs to fix, not runner limitations to skip.
                return .fail(error.description)
            } catch let error as DecodingError {
                // Reached only by a decode the SDK's primitives do not own —
                // the wrapper-field decode a generated wrapped-list method runs
                // after the primitive returns. Everything the primitives decode
                // arrives above as a statusless `.api` (#604), and both paths
                // apply the same policy.
                //
                // A mock body that fails the model's required-field validation
                // is a fixture bug, not a runner limitation: fail loudly so it
                // gets fixed (canonical bodies live in spec/fixtures/) instead
                // of silently degrading coverage. Kotlin's #555 policy, adopted
                // from day one.
                //
                // Unless the fixture declares errorRaised: then the body is
                // deliberately malformed and Codable refusing it IS the
                // behaviour under test. That is how Swift satisfies the #576
                // kill cases — the decoder is its guard, where TypeScript,
                // Python and Ruby need a hand-written one.
                guard expectsFailure else {
                    return .fail("Mock body lacks required Swift model fields: \(describeDecodingError(error))")
                }
                dispatchFailed = true
            } catch {
                return .fail("Unexpected exception: \(type(of: error)): \(error)")
            }
        }

        return evaluateAssertions(
            tc,
            transport: transport,
            caughtError: caughtError,
            dispatchFailed: dispatchFailed,
            httpStatus: httpStatus,
            dispatch: dispatch
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
///
/// The SDK traps on EVERY parsed non-HTTPS scheme outside the carve-out, not
/// just `http` — `BasecampClient` tests `scheme != "https"`. Routing only
/// `http` through the probe meant a fixture with, say, `ftp://` or
/// `ws://localhost` would trap in-process and take the entire conformance run
/// down with it, losing every result after it.
private func requiresHTTPSCrashProbe(_ baseURL: String) -> Bool {
    // An unparseable URL never reaches the scheme check in the SDK either: the
    // guard is `if let url = URL(string:)`, so construction survives.
    guard let url = URL(string: baseURL) else { return false }
    let scheme = url.scheme?.lowercased()
    if scheme == "https" { return false }
    // The carve-out is HTTP(S)-only, matching the SDK's isLocalhost.
    guard scheme == "http" else { return true }
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

/// The message of a SPEC §6 malformed-2xx-body error, or nil for any other
/// `BasecampError`.
///
/// The SDK maps a decode failure to a statusless `api_error` (#604) and cannot
/// carry the `DecodingError` structurally — `BasecampError.api` has no `cause`
/// slot — so the shape is what identifies it: every other `.api` the runner can
/// provoke comes from an HTTP response and carries that response's status.
private func malformedBodyMessage(_ error: BasecampError) -> String? {
    guard case .api(let message, let httpStatus, _, _) = error, httpStatus == nil else {
        return nil
    }
    return message
}

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
