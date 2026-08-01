import Basecamp
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
    case .validation: "validation"
    case .api: "api_error"
    case .network: "network"
    case .usage: "usage"
    case .ambiguous: "ambiguous"
    }
}

private let knownErrorTypes: Set<String> = [
    "not_found", "auth_required", "forbidden", "rate_limit",
    "validation", "api_error", "usage", "network",
]

/// Resolves a per-request assertion index (0-based; negative = from the end)
/// against the number of recorded requests. Nil when out of range.
private func resolveRequestIndex(_ index: Int, _ count: Int) -> Int? {
    let resolved = index < 0 ? count + index : index
    return (0..<count).contains(resolved) ? resolved : nil
}

/// Compares an expected fixture value against an actual JSON value,
/// preserving 64-bit integer precision. Nil means equal.
private func compareJSON(_ label: String, _ expected: JSON?, _ actual: JSON) -> String? {
    guard let expected else { return "\(label): expected value is null in assertion" }
    if let expInt = expected.intValue, let actInt = actual.intValue {
        return expInt == actInt ? nil : "Expected \(label) = \(expInt), got \(actInt)"
    }
    return expected == actual ? nil : "Expected \(label) = \(expected.display), got \(actual.display)"
}

/// Compares an expected fixture value against a loosely-typed actual value
/// (error fields, response meta). Nil means equal.
private func compareValue(_ label: String, _ expected: JSON?, _ actual: Any?) -> String? {
    guard let expected else { return "\(label): expected value is null in assertion" }
    switch expected {
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
    httpStatus: Int?,
    dispatch: DispatchResult,
    autoPaginates: Bool
) -> TestResult {
    let captured = transport.captured
    let requestCount = captured.count

    // Implicit method invariant: the scripted transport answers any verb, so a
    // wrong-verb request (e.g. a PUT regressing to POST) would consume a queued
    // response silently. When the fixture declares a method and carries no
    // explicit requestMethod assertions, the first request must use it.
    let fixtureMethod = tc.fixtureMethod.uppercased()
    if !fixtureMethod.isEmpty,
       !tc.allAssertions.contains(where: { $0.type == "requestMethod" }),
       let first = captured.first, first.method != fixtureMethod {
        return .fail("Expected first request method \(fixtureMethod), got \(first.method)")
    }

    for assertion in tc.allAssertions {
        switch assertion.type {
        case "requestCount":
            guard let expected = assertion.expected?.intValue.map(Int.init) else {
                return .fail("requestCount assertion missing expected value")
            }
            if autoPaginates {
                if requestCount < expected {
                    return .fail("Expected >= \(expected) requests (SDK auto-paginates), got \(requestCount)")
                }
            } else if requestCount != expected {
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
            if captured.count >= 2 {
                let delay = captured[1].monotonicMs - captured[0].monotonicMs
                let minDelay = UInt64(assertion.minValue)
                if delay < minDelay {
                    return .fail("Expected delay >= \(minDelay)ms, got \(delay)ms")
                }
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
            default: nil
            }
            if actual == nil, !["httpStatus", "retryable", "code", "message", "requestId"].contains(fieldPath) {
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
            guard let first = captured.first else {
                return .fail("Expected header \(headerName)=\"\(expected)\", but no requests were recorded")
            }
            let actual = first.header(headerName)
            // Content-Type may include charset (e.g., "application/json; charset=utf-8")
            let matches: Bool = if headerName.lowercased() == "content-type" {
                actual?.lowercased().hasPrefix(expected.lowercased()) ?? false
            } else {
                actual == expected
            }
            if !matches {
                return .fail("Expected header \(headerName)=\"\(expected)\", got \"\(actual ?? "nil")\"")
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
