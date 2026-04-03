import XCTest
@testable import Basecamp

final class ErrorTests: XCTestCase {

    // MARK: - Error Codes & Exit Codes

    func testAuthErrorProperties() {
        let error = BasecampError.auth(message: "Unauthorized", hint: "Check token", requestId: "req-1")
        XCTAssertEqual(error.code, "auth_required")
        XCTAssertEqual(error.httpStatusCode, 401)
        XCTAssertEqual(error.exitCode, 3)
        XCTAssertFalse(error.isRetryable)
        XCTAssertEqual(error.hint, "Check token")
        XCTAssertEqual(error.message, "Unauthorized")
        XCTAssertEqual(error.requestId, "req-1")
    }

    func testForbiddenErrorProperties() {
        let error = BasecampError.forbidden(message: "Denied", hint: nil, requestId: nil)
        XCTAssertEqual(error.code, "forbidden")
        XCTAssertEqual(error.httpStatusCode, 403)
        XCTAssertEqual(error.exitCode, 4)
        XCTAssertFalse(error.isRetryable)
    }

    func testNotFoundErrorProperties() {
        let error = BasecampError.notFound(message: "Not found", hint: nil, requestId: nil)
        XCTAssertEqual(error.code, "not_found")
        XCTAssertFalse(error.isAPIDisabled)
        XCTAssertEqual(error.httpStatusCode, 404)
        XCTAssertEqual(error.exitCode, 2)
        XCTAssertFalse(error.isRetryable)
    }

    func testRateLimitErrorProperties() {
        let error = BasecampError.rateLimit(
            message: "Rate limited", retryAfterSeconds: 30,
            hint: "Retry after 30 seconds", requestId: nil
        )
        XCTAssertEqual(error.code, "rate_limit")
        XCTAssertEqual(error.httpStatusCode, 429)
        XCTAssertEqual(error.exitCode, 5)
        XCTAssertTrue(error.isRetryable)
    }

    func testNetworkErrorProperties() {
        let error = BasecampError.network(message: "Connection failed", cause: nil)
        XCTAssertEqual(error.code, "network")
        XCTAssertNil(error.httpStatusCode)
        XCTAssertEqual(error.exitCode, 6)
        XCTAssertTrue(error.isRetryable)
        XCTAssertEqual(error.hint, "Check your network connection")
    }

    func testApiErrorProperties() {
        let error = BasecampError.api(message: "Server error", httpStatus: 500, hint: nil, requestId: nil)
        XCTAssertEqual(error.code, "api_error")
        XCTAssertEqual(error.httpStatusCode, 500)
        XCTAssertEqual(error.exitCode, 7)
        XCTAssertTrue(error.isRetryable)
    }

    func testApiError4xxNotRetryable() {
        let error = BasecampError.api(message: "Bad", httpStatus: 418, hint: nil, requestId: nil)
        XCTAssertFalse(error.isRetryable)
    }

    func testValidationErrorProperties() {
        let error = BasecampError.validation(message: "Invalid", httpStatus: 422, hint: nil, requestId: nil)
        XCTAssertEqual(error.code, "validation")
        XCTAssertEqual(error.httpStatusCode, 422)
        XCTAssertEqual(error.exitCode, 9)
        XCTAssertFalse(error.isRetryable)
    }

    func testAmbiguousErrorProperties() {
        let error = BasecampError.ambiguous(resource: "project", matches: ["Project A", "Project B"], hint: "Did you mean: Project A, Project B")
        XCTAssertEqual(error.code, "ambiguous")
        XCTAssertNil(error.httpStatusCode)
        XCTAssertEqual(error.exitCode, 8)
        XCTAssertFalse(error.isRetryable)
        XCTAssertEqual(error.message, "Ambiguous project")
        XCTAssertEqual(error.hint, "Did you mean: Project A, Project B")
    }

    func testApiDisabledErrorProperties() {
        let error = BasecampError.fromHTTPResponse(
            status: 404, data: nil, headers: ["Reason": "API Disabled"], requestId: "req-1"
        )
        if case .notFound(let message, let hint, let requestId) = error {
            XCTAssertEqual(message, "API access is disabled for this account")
            XCTAssertEqual(hint, "An administrator can re-enable it in Adminland under Manage API access")
            XCTAssertEqual(requestId, "req-1")
        } else {
            XCTFail("Expected .notFound, got \(error)")
        }
        XCTAssertEqual(error.code, "api_disabled")
        XCTAssertTrue(error.isAPIDisabled)
        XCTAssertEqual(error.httpStatusCode, 404)
        XCTAssertEqual(error.exitCode, 10)
        XCTAssertFalse(error.isRetryable)
    }

    func testUsageErrorProperties() {
        let error = BasecampError.usage(message: "Bad argument", hint: "Use --flag")
        XCTAssertEqual(error.code, "usage")
        XCTAssertNil(error.httpStatusCode)
        XCTAssertEqual(error.exitCode, 1)
        XCTAssertFalse(error.isRetryable)
    }

    // MARK: - Factory: fromHTTPResponse

    func testFromHTTPResponse401() {
        let error = BasecampError.fromHTTPResponse(status: 401, data: nil, headers: [:], requestId: "r1")
        if case .auth(_, _, let requestId) = error {
            XCTAssertEqual(requestId, "r1")
        } else {
            XCTFail("Expected .auth, got \(error)")
        }
    }

    func testFromHTTPResponse403() {
        let error = BasecampError.fromHTTPResponse(status: 403, data: nil, headers: [:], requestId: nil)
        if case .forbidden = error { } else { XCTFail("Expected .forbidden") }
    }

    func testFromHTTPResponse404() {
        let error = BasecampError.fromHTTPResponse(status: 404, data: nil, headers: [:], requestId: nil)
        if case .notFound = error { } else { XCTFail("Expected .notFound") }
        XCTAssertEqual(error.code, "not_found")
    }

    func testFromHTTPResponse404APIDisabled() {
        let error = BasecampError.fromHTTPResponse(
            status: 404, data: nil, headers: ["Reason": "API Disabled"], requestId: "req-1"
        )
        if case .notFound(let message, let hint, let requestId) = error {
            XCTAssertTrue(message.contains("disabled"))
            XCTAssertNotNil(hint)
            XCTAssertTrue(hint!.contains("Adminland"))
            XCTAssertEqual(requestId, "req-1")
        } else {
            XCTFail("Expected .notFound, got \(error)")
        }
        XCTAssertEqual(error.code, "api_disabled")
        XCTAssertEqual(error.exitCode, 10)
    }

    func testFromHTTPResponse404APIDisabledLowercaseHeaderName() {
        let error = BasecampError.fromHTTPResponse(
            status: 404, data: nil, headers: ["reason": "API Disabled"], requestId: nil
        )
        XCTAssertEqual(error.code, "api_disabled")
        XCTAssertTrue(error.isAPIDisabled)
    }

    func testCallerConstructedNotFoundWithMatchingMessageOnlyIsNotApiDisabled() {
        let error = BasecampError.notFound(
            message: "API access is disabled for this account",
            hint: "Some other hint",
            requestId: nil
        )
        XCTAssertEqual(error.code, "not_found")
        XCTAssertFalse(error.isAPIDisabled)
    }

    func testFromHTTPResponse404AccountInactive() {
        let error = BasecampError.fromHTTPResponse(
            status: 404, data: nil, headers: ["Reason": "Account Inactive"], requestId: nil
        )
        if case .notFound(let message, let hint, _) = error {
            XCTAssertTrue(message.contains("inactive"))
            XCTAssertNotNil(hint)
            XCTAssertTrue(hint!.contains("expired trial"))
        } else {
            XCTFail("Expected .notFound with account inactive, got \(error)")
        }
        XCTAssertEqual(error.code, "not_found")
    }

    func testFromHTTPResponse429() {
        let error = BasecampError.fromHTTPResponse(
            status: 429, data: nil, headers: ["Retry-After": "30"], requestId: nil
        )
        if case .rateLimit(_, let retryAfter, _, _) = error {
            XCTAssertEqual(retryAfter, 30)
        } else {
            XCTFail("Expected .rateLimit")
        }
    }

    func testFromHTTPResponse429LowercaseRetryAfterHeaderName() {
        let error = BasecampError.fromHTTPResponse(
            status: 429, data: nil, headers: ["retry-after": "30"], requestId: nil
        )
        if case .rateLimit(_, let retryAfter, _, _) = error {
            XCTAssertEqual(retryAfter, 30)
        } else {
            XCTFail("Expected .rateLimit")
        }
    }

    func testFromHTTPResponse422() {
        let body = try! JSONSerialization.data(withJSONObject: ["error": "Name is required"])
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        if case .validation(let message, let status, _, _) = error {
            XCTAssertEqual(message, "Name is required")
            XCTAssertEqual(status, 422)
        } else {
            XCTFail("Expected .validation")
        }
    }

    func testFromHTTPResponse500() {
        let error = BasecampError.fromHTTPResponse(status: 500, data: nil, headers: [:], requestId: nil)
        if case .api(_, let status, _, _) = error {
            XCTAssertEqual(status, 500)
        } else {
            XCTFail("Expected .api")
        }
    }

    // MARK: - Retry-After Parsing

    func testParseRetryAfterSeconds() {
        XCTAssertEqual(BasecampError.parseRetryAfter("30"), 30)
    }

    func testParseRetryAfterNil() {
        XCTAssertNil(BasecampError.parseRetryAfter(nil))
    }

    func testParseRetryAfterEmpty() {
        XCTAssertNil(BasecampError.parseRetryAfter(""))
    }

    func testParseRetryAfterZero() {
        XCTAssertNil(BasecampError.parseRetryAfter("0"))
    }

    // MARK: - LocalizedError

    func testLocalizedDescriptionWithHint() {
        let error = BasecampError.usage(message: "Bad arg", hint: "Use --flag")
        XCTAssertEqual(error.localizedDescription, "Bad arg: Use --flag")
    }

    func testLocalizedDescriptionWithoutHint() {
        let error = BasecampError.notFound(message: "Not found", hint: nil, requestId: nil)
        XCTAssertEqual(error.localizedDescription, "Not found")
    }

    // MARK: - Truncation

    func testLongErrorMessageTruncated() {
        let longMessage = String(repeating: "x", count: 600)
        let body = try! JSONSerialization.data(withJSONObject: ["error": longMessage])
        let error = BasecampError.fromHTTPResponse(status: 500, data: body, headers: [:], requestId: nil)
        XCTAssertLessThanOrEqual(error.message.count, 500)
        XCTAssertTrue(error.message.hasSuffix("..."))
    }
}
