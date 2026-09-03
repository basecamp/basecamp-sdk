import Basecamp
import ConformanceSupport
import Foundation

/// Outcome of a single conformance test.
struct TestResult {
    let passed: Bool
    let message: String
    var skipped: Bool = false

    static func fail(_ message: String) -> TestResult {
        TestResult(passed: false, message: message)
    }
}

/// SDK-observed values captured from a dispatched operation.
struct DispatchResult {
    /// X-Total-Count as parsed by the SDK into ListResult.meta.totalCount.
    var totalCount: Int? = nil
    /// True when the SDK truncated results (maxPages/maxItems cap hit).
    var truncated: Bool? = nil
    /// The deserialized SDK response re-serialized to JSON (responseBody assertions).
    var resultJSON: JSON? = nil
}

/// Maps a BasecampError onto the conformance error-code vocabulary shared by
/// every runner (auth_required, not_found, ...).
func conformanceCode(_ error: BasecampError) -> String {
    switch error {
    case .auth: "auth_required"
    case .forbidden: "forbidden"
    case .notFound: "not_found"
    case .rateLimit: "rate_limit"
    case .validation, .peopleConfirmationRequired: "validation"
    case .api: "api_error"
    case .network: "network"
    case .usage: "usage"
    case .ambiguous: "ambiguous"
    case .limitExceeded: "limit_exceeded"
    }
}

/// The vocabulary `conformanceCode` can produce. It must stay in sync with that
/// mapping: a member missing here can never be asserted, so the guard meant to
/// catch a typo'd error type silently forbids a real one instead.
private let knownErrorTypes: Set<String> = [
    "not_found", "auth_required", "forbidden", "rate_limit",
    "validation", "api_error", "usage", "network", "ambiguous", "limit_exceeded",
]

/// Compares an expected fixture value against an actual JSON value,
/// preserving 64-bit integer precision. Nil means equal.
private func compareJSON(_ label: String, _ expected: JSON?, _ actual: JSON) -> String? {
    guard let expected else { return "\(label): assertion has no expected value" }
    if let expInt = expected.intValue, let actInt = actual.intValue {
        return expInt == actInt ? nil : "Expected \(label) = \(expInt), got \(actInt)"
    }
    return expected == actual ? nil : "Expected \(label) = \(expected.display), got \(actual.display)"
}

/// Compares an expected fixture value against a loosely-typed actual value
/// (error fields, response meta). Nil means equal.
private func compareValue(_ label: String, _ expected: JSON?, _ actual: Any?) -> String? {
    guard let expected else { return "\(label): assertion has no expected value" }
    switch expected {
    case .null:
        // An explicit `expected: null` asserts the observed field is absent.
        // Falling through to the string fallback compared nil against the
        // literal "null" and always failed.
        if let actual { return "Expected \(label) = null, got \(actual)" }
    case .bool(let b):
        if (actual as? Bool) != b { return "Expected \(label) = \(b), got \(String(describing: actual))" }
    case .int(let i):
        let actualInt: Int64? = switch actual {
        case let n as Int: Int64(n)
        case let n as Int64: n
        default: nil
        }
        if actualInt != i { return "Expected \(label) = \(i), got \(String(describing: actual))" }
    case .string(let s):
        let actualString = actual.map { "\($0)" }
        if actualString != s { return "Expected \(label) = \"\(s)\", got \(String(describing: actual))" }
    default:
        let actualString = actual.map { "\($0)" }
        if actualString != expected.display { return "Expected \(label) = \(expected.display), got \(String(describing: actual))" }
    }
    return nil
}

/// Evaluates every assertion in the fixture against the recorded transport
/// traffic and dispatch outcome. Direct port of the Kotlin evaluator.
func evaluateAssertions(
    _ tc: TestCase,
    transport: ScriptedTransport,
    caughtError: BasecampError?,
    // No default. The one call site is required to say whether the dispatch
    // failed, because a defaulted `false` fails CLOSED: a future call site that
    // omitted it would report "the call succeeded" on a call that did not, and
    // every errorRaised fixture would go red for a reason nowhere near the bug.
    dispatchFailed: Bool,
    httpStatus: Int?,
    dispatch: DispatchResult
) -> TestResult {
    let captured = transport.captured
    let requestCount = captured.count

    // A fixture that queues responses is testing a wire operation, so one must
    // have happened. Every invariant below is guarded on having captured a
    // request, so an operation short-circuited before the transport would slip
    // through all of them and pass on a bare noError assertion — the runner
    // reporting green on a call it never watched.
    //
    // An EMPTY queue is the deliberate no-request case: the HTTPS-enforcement
    // fixture makes no call at all, and says so with requestCount 0.
    if !tc.responses.isEmpty, captured.isEmpty {
        return .fail("fixture queues \(tc.responses.count) mock response(s) but the operation made no request — it never reached the transport")
    }

    // The implicit invariants below defer to an explicit assertion, but only
    // for the exact request that assertion names. "Any assertion of this type
    // exists" is too coarse: the EditTodo edit-clear fixture pins requestPath
    // at index 1 only, so a coarse exemption left the composite's leading GET
    // unchecked — a regression there would keep the method, body, count and
    // index-1 path assertions all green.
    func explicitAssertionCovers(_ type: String, request index: Int) -> Bool {
        tc.allAssertions.contains {
            $0.type == type && resolveRequestIndex($0.requestIndex, requestCount) == index
        }
    }

    // Implicit method invariant: the scripted transport answers any verb, so a
    // wrong-verb request (e.g. a PUT regressing to POST) would consume a queued
    // response silently.
    //
    // EVERY hop, like the path invariant below. Retries repeat the verb, a
    // redirect followed after a GET stays a GET, and the read-modify-write
    // composites — the one place a later hop legitimately differs — pin their
    // hops with indexed requestMethod assertions already. Checking only the
    // first left the download flows able to POST their signed final hop and
    // still satisfy path, authorization, count and noError.
    let fixtureMethod = tc.fixtureMethod.uppercased()
    if !fixtureMethod.isEmpty {
        for (i, request) in captured.enumerated() {
            if explicitAssertionCovers("requestMethod", request: i) { continue }
            if request.method != fixtureMethod {
                return .fail("Expected request \(i) to use method \(fixtureMethod), got \(request.method)")
            }
        }
    }

    // Requests that follow a rel="next" link are governed by the LINK
    // invariant below instead: they go where the previous response SAID to go,
    // which the fixtures state root-relative, so they legitimately arrive
    // unscoped. Each hop is pinned by exactly one rule — the most specific one
    // that applies — rather than by two rules that disagree.
    let linkFollowers: Set<Int> = Set(
        tc.responses.enumerated().compactMap { i, mock in
            mock.allHeaders.contains { $0.key.lowercased() == "link" && nextLinkTarget($0.value) != nil }
                ? i + 1 : nil
        }
    )

    // Implicit PATH invariant, for the same reason: the transport answers any
    // URL, so an operation aimed at the wrong endpoint consumes the queued
    // responses and passes its retry, status, auth and pagination assertions
    // against a resource the fixture never named. Checking the verb alone left
    // that open.
    //
    // EVERY hop, not just the first. Retries and pagination stay on the
    // fixture's path, and so do the read-modify-write composites — a card or
    // todo edit GETs and PUTs the same resource, so a regression in either
    // hop alone is exactly what this catches. The hops that legitimately go
    // elsewhere are the download redirect and delegation flows, and those say
    // so with their own indexed requestPath assertions rather than being
    // waved through by a rule in here.
    if !tc.fixturePath.isEmpty, !captured.isEmpty {
        let params = (tc.pathParams ?? [:]).compactMapValues {
            $0.stringValue ?? $0.intValue.map(String.init)
        }
        switch renderFixturePath(tc.fixturePath, params) {
        case .unsubstituted(let name):
            return .fail("fixture path \"\(tc.fixturePath)\" has no pathParams entry for \"\(name)\"")
        case .rendered(let expected):
            for (i, request) in captured.enumerated() {
                if linkFollowers.contains(i) { continue }
                if explicitAssertionCovers("requestPath", request: i) { continue }
                if !requestPathMatches(request.path, fixturePath: expected, accountID: testAccountID) {
                    let want = expectedRequestPath(expected, accountID: testAccountID)
                    return .fail("Expected request \(i) at path \(want), got \(request.path)")
                }
            }
        }
    }

    // Implicit LINK invariant: a response that advertises rel="next" says
    // exactly which URL to fetch, so the following request must be that URL —
    // query string included. The path check above cannot see the query, and
    // the transport answers any URL from the same queue, so pagination that
    // refetched page 1 was handed page 2's body and reported three requests,
    // three pages, all green.
    //
    // Only constrains a hop that actually happened: a walk stopped by a cap,
    // or a link the SDK is meant to refuse (cross-origin, protocol downgrade),
    // simply has no following request to check.
    for (i, mock) in tc.responses.enumerated() where i + 1 < captured.count {
        guard let link = mock.allHeaders.first(where: { $0.key.lowercased() == "link" })?.value,
              let target = nextLinkTarget(link)
        else { continue }
        let follower = captured[i + 1]
        // Resolve against the request that carried the link, so a relative
        // target is compared the way the SDK had to resolve it.
        let resolved = URL(string: target, relativeTo: follower.request.url)
        let wanted = resolved.map { url -> String in
            guard let query = url.query, !query.isEmpty else { return url.path }
            return "\(url.path)?\(query)"
        } ?? target
        if follower.pathAndQuery != wanted {
            return .fail("Response \(i) advertised rel=\"next\" \(target), so request \(i + 1) must fetch \(wanted), got \(follower.pathAndQuery)")
        }
    }

    for assertion in tc.allAssertions {
        switch assertion.type {
        case "requestCount":
            guard let expected = assertion.expected?.intValue.map(Int.init) else {
                return .fail("requestCount assertion missing expected value")
            }
            // Exact, including the auto-paginating fixtures. A lower bound
            // makes the cap assertions vacuous in the direction that matters:
            // "Pagination stops at maxPages safety cap" and "maxItems caps
            // results across pages" both queue THREE pages and expect TWO
            // requests, so `>=` passes an SDK that ignored the cap and walked
            // every page. The first-page-only fixture, the one case where the
            // count genuinely does not apply to an auto-paginating SDK, is
            // excluded by its own `link-header` tag before it reaches here.
            //
            // Swift excludes the whole CASE where Go, Python, Ruby and
            // TypeScript exclude only this ASSERTION (#573). Deliberate, not
            // drift: `httpStatus` is the status of the last mock response the
            // SDK consumed, and an auto-paginating SDK walks past the end of a
            // one-response queue, so the fixture's `statusCode: 200` cannot be
            // satisfied. Narrowing this arm was tried and reverted — `make
            // conformance-swift` then reports `Expected status code 200, but
            // got no response` and exits 2. Do not "align" it with the other
            // four without widening the status model first.
            if requestCount != expected {
                return .fail("Expected \(expected) requests, got \(requestCount)")
            }

        case "statusCode", "responseStatus":
            guard let expected = assertion.expected?.intValue.map(Int.init) else {
                return .fail("\(assertion.type) assertion missing expected value")
            }
            guard let actual = httpStatus else {
                return .fail("Expected status code \(expected), but got no response")
            }
            if actual != expected {
                return .fail("Expected status code \(expected), got \(actual)")
            }

        case "responseBody":
            let fieldPath = assertion.fieldPath
            guard let result = dispatch.resultJSON else {
                return .fail("responseBody.\(fieldPath): no result captured from operation")
            }
            guard let actual = result.navigate(fieldPath) else {
                return .fail("responseBody.\(fieldPath): field not found in result")
            }
            if let failure = compareJSON("responseBody.\(fieldPath)", assertion.expected, actual) {
                return .fail(failure)
            }

        case "noError":
            if let caughtError {
                return .fail("Expected no error, got: \(caughtError.message)")
            }

        // The inverse of noError, and deliberately code-agnostic. See
        // errorRaisedFailure (ConformanceSupport/ErrorRaised.swift) for the
        // contract and for why the branch lives there rather than inline: no
        // committed fixture can reach its failing side, so it is unit-tested
        // instead.
        //
        // Read from BOTH signals: every path that records caughtError also sets
        // dispatchFailed, and the union keeps that true by construction rather
        // than by call-site discipline.
        case "errorRaised":
            if let message = errorRaisedFailure(dispatchFailed: dispatchFailed || caughtError != nil) {
                return .fail(message)
            }

        case "requestPath":
            guard let expected = assertion.expected?.stringValue else {
                return .fail("requestPath assertion missing expected value")
            }
            guard let idx = resolveRequestIndex(assertion.requestIndex, requestCount) else {
                return .fail("requestPath[\(assertion.requestIndex)]: no request recorded at that index (\(requestCount) requests)")
            }
            if captured[idx].path != expected {
                return .fail("Expected request path \"\(expected)\" at index \(assertion.requestIndex), got \"\(captured[idx].path)\"")
            }

        case "requestMethod":
            guard let expected = assertion.expected?.stringValue?.uppercased() else {
                return .fail("requestMethod assertion missing expected value")
            }
            guard let idx = resolveRequestIndex(assertion.requestIndex, requestCount) else {
                return .fail("requestMethod[\(assertion.requestIndex)]: no request recorded at that index (\(requestCount) requests)")
            }
            if captured[idx].method != expected {
                return .fail("Expected request method \(expected) at index \(assertion.requestIndex), got \(captured[idx].method)")
            }

        case "requestBody":
            let key = assertion.fieldPath
            guard let idx = resolveRequestIndex(assertion.requestIndex, requestCount) else {
                return .fail("requestBody.\(key)[\(assertion.requestIndex)]: no request recorded at that index (\(requestCount) requests)")
            }
            guard let body = captured[idx].bodyJSON else {
                return .fail("requestBody.\(key)[\(assertion.requestIndex)]: request has no JSON body")
            }
            guard let actual = body.navigate(key) else {
                return .fail("requestBody.\(key)[\(assertion.requestIndex)]: key not present in request body")
            }
            if let failure = compareJSON("requestBody.\(key)[\(assertion.requestIndex)]", assertion.expected, actual) {
                return .fail(failure)
            }

        case "requestBodyAbsent":
            let key = assertion.fieldPath
            guard let idx = resolveRequestIndex(assertion.requestIndex, requestCount) else {
                return .fail("requestBodyAbsent.\(key)[\(assertion.requestIndex)]: no request recorded at that index (\(requestCount) requests)")
            }
            if let body = captured[idx].bodyJSON, body.navigate(key) != nil {
                return .fail("requestBodyAbsent.\(key)[\(assertion.requestIndex)]: key unexpectedly present in request body")
            }

        case "delayBetweenRequests":
            // Delegated to ConformanceSupport so the bounds branches are
            // unit-testable. This evaluator used to measure gap 0 and ignore
            // the assertion's index, and skipped the check entirely on a
            // single-request run — the #563/#568 false-green, which downloads
            // fixtures (index 0 AND index 1) walk straight into.
            if let failure = checkDelayGaps(
                captured.map(\.monotonicMs),
                minDelayMs: assertion.minDelayMs,
                index: assertion.gapIndex
            ) {
                return .fail(failure)
            }

        case "headerValue":
            let headerName = assertion.fieldPath
            guard let expected = assertion.expected?.stringValue else {
                return .fail("headerValue assertion missing expected value")
            }
            if headerName.lowercased() == "x-total-count" {
                let actual = dispatch.totalCount.map(String.init)
                if actual != expected {
                    return .fail("SDK meta.totalCount: expected \(expected), got \(actual ?? "nil")")
                }
            } else {
                guard let first = tc.responses.first else {
                    return .fail("Expected response header \(headerName)=\(expected), but no mock responses defined")
                }
                let actual = first.allHeaders[headerName]
                if actual != expected {
                    return .fail("Expected response header \(headerName)=\(expected), got \(actual ?? "nil")")
                }
            }

        case "errorType", "errorCode":
            guard let expected = assertion.expected?.stringValue else {
                return .fail("\(assertion.type) assertion missing expected value")
            }
            guard let caughtError else {
                return .fail("Expected error \(assertion.type == "errorType" ? "type" : "code") \"\(expected)\", but got no error")
            }
            if assertion.type == "errorType", !knownErrorTypes.contains(expected) {
                return .fail("Unknown conformance error type \"\(expected)\"")
            }
            let actual = conformanceCode(caughtError)
            if actual != expected {
                return .fail("Expected error code \"\(expected)\", got \"\(actual)\"")
            }

        case "errorMessage":
            guard let expected = assertion.expected?.stringValue else {
                return .fail("errorMessage assertion missing expected value")
            }
            guard let caughtError else {
                return .fail("Expected error message containing \"\(expected)\", but got no error")
            }
            if !caughtError.message.contains(expected) {
                return .fail("Expected error message containing \"\(expected)\", got \"\(caughtError.message)\"")
            }

        case "errorField":
            let fieldPath = assertion.fieldPath
            guard let caughtError else {
                return .fail("Expected error field \(fieldPath), but got no error")
            }
            let actual: Any? = switch fieldPath {
            case "httpStatus": caughtError.httpStatusCode
            case "retryable": caughtError.isRetryable
            case "code": conformanceCode(caughtError)
            case "message": caughtError.message
            case "requestId": caughtError.requestId
            case "confirmationPeople.0.id": caughtError.confirmationPeople?.first?.id
            default: nil
            }
            if actual == nil, !["httpStatus", "retryable", "code", "message", "requestId", "confirmationPeople.0.id"].contains(fieldPath) {
                return .fail("Unknown error field: \(fieldPath)")
            }
            if let failure = compareValue("error.\(fieldPath)", assertion.expected, actual) {
                return .fail(failure)
            }

        case "headerInjected":
            let headerName = assertion.fieldPath
            guard let expected = assertion.expected?.stringValue else {
                return .fail("headerInjected assertion missing expected value")
            }
            // Index-aware like headerPresent/headerAbsent and the other
            // runners: reading captured.first regardless would validate the
            // initial attempt when the fixture named a retry or a second hop.
            guard let idx = resolveRequestIndex(assertion.requestIndex, requestCount) else {
                return .fail("headerInjected \(headerName)[\(assertion.requestIndex)]: no request recorded at that index (\(requestCount) requests)")
            }
            let actual = captured[idx].header(headerName)
            // Content-Type may include charset (e.g., "application/json; charset=utf-8")
            let matches: Bool = if headerName.lowercased() == "content-type" {
                actual?.lowercased().hasPrefix(expected.lowercased()) ?? false
            } else {
                actual == expected
            }
            if !matches {
                return .fail("Expected header \(headerName)=\"\(expected)\" on request index \(idx), got \"\(actual ?? "nil")\"")
            }

        case "headerPresent":
            let headerName = assertion.fieldPath
            guard let idx = resolveRequestIndex(assertion.requestIndex, requestCount) else {
                return .fail("headerPresent \(headerName)[\(assertion.requestIndex)]: no request recorded at that index (\(requestCount) requests)")
            }
            let actual = captured[idx].header(headerName)
            if actual == nil || actual?.isEmpty == true {
                return .fail("Expected header \(headerName) present on request index \(idx), but it was empty or missing")
            }

        case "headerAbsent":
            let headerName = assertion.fieldPath
            guard let idx = resolveRequestIndex(assertion.requestIndex, requestCount) else {
                return .fail("headerAbsent \(headerName)[\(assertion.requestIndex)]: no request recorded at that index (\(requestCount) requests)")
            }
            // A present-but-empty header must fail an absence assertion, same
            // as the Go runner's Values check.
            if let actual = captured[idx].header(headerName) {
                return .fail("Expected header \(headerName) absent on request index \(idx), got \"\(actual)\"")
            }

        case "requestScheme":
            if assertion.expected?.stringValue == "https", caughtError == nil {
                return .fail("Expected HTTPS enforcement error, but request succeeded over HTTP")
            }

        case "urlOrigin":
            if assertion.expected?.stringValue == "rejected", requestCount > 1 {
                return .fail("Expected cross-origin URL rejection (1 request), but \(requestCount) requests were made")
            }

        case "responseMeta":
            let fieldPath = assertion.fieldPath
            let actual: Any? = switch fieldPath {
            case "totalCount": dispatch.totalCount
            case "truncated": dispatch.truncated
            default: nil
            }
            if !["totalCount", "truncated"].contains(fieldPath) {
                return .fail("Unknown response meta field: \(fieldPath)")
            }
            if let failure = compareValue("meta.\(fieldPath)", assertion.expected, actual) {
                return .fail(failure)
            }

        default:
            return .fail("Unknown assertion type: \(assertion.type)")
        }
    }

    return TestResult(passed: true, message: "All assertions passed")
}
