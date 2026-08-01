import XCTest
@testable import Basecamp

final class ErrorTests: XCTestCase {

    // MARK: - Error Codes & Exit Codes

    func testAuthErrorProperties() {
        let error = BasecampError.auth(message: "Unauthorized", hint: "Check token", requestId: "req-1")
        XCTAssertEqual(error.httpStatusCode, 401)
        XCTAssertEqual(error.exitCode, 3)
        XCTAssertFalse(error.isRetryable)
        XCTAssertEqual(error.hint, "Check token")
        XCTAssertEqual(error.message, "Unauthorized")
        XCTAssertEqual(error.requestId, "req-1")
    }

    func testForbiddenErrorProperties() {
        let error = BasecampError.forbidden(message: "Denied", hint: nil, requestId: nil)
        XCTAssertEqual(error.httpStatusCode, 403)
        XCTAssertEqual(error.exitCode, 4)
        XCTAssertFalse(error.isRetryable)
    }

    func testNotFoundErrorProperties() {
        let error = BasecampError.notFound(message: "Not found", hint: nil, requestId: nil)
        XCTAssertEqual(error.httpStatusCode, 404)
        XCTAssertEqual(error.exitCode, 2)
        XCTAssertFalse(error.isRetryable)
    }

    func testRateLimitErrorProperties() {
        let error = BasecampError.rateLimit(
            message: "Rate limited", retryAfterSeconds: 30,
            hint: "Retry after 30 seconds", requestId: nil
        )
        XCTAssertEqual(error.httpStatusCode, 429)
        XCTAssertEqual(error.exitCode, 5)
        XCTAssertTrue(error.isRetryable)
    }

    func testNetworkErrorProperties() {
        let error = BasecampError.network(message: "Connection failed", cause: nil)
        XCTAssertNil(error.httpStatusCode)
        XCTAssertEqual(error.exitCode, 6)
        XCTAssertTrue(error.isRetryable)
        XCTAssertEqual(error.hint, "Check your network connection")
    }

    func testApiErrorProperties() {
        let error = BasecampError.api(message: "Server error", httpStatus: 500, hint: nil, requestId: nil)
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
        XCTAssertEqual(error.httpStatusCode, 422)
        XCTAssertEqual(error.exitCode, 9)
        XCTAssertFalse(error.isRetryable)
    }

    func testAmbiguousErrorProperties() {
        let error = BasecampError.ambiguous(resource: "project", matches: ["Project A", "Project B"], hint: "Did you mean: Project A, Project B")
        XCTAssertNil(error.httpStatusCode)
        XCTAssertEqual(error.exitCode, 8)
        XCTAssertFalse(error.isRetryable)
        XCTAssertEqual(error.message, "Ambiguous project")
        XCTAssertEqual(error.hint, "Did you mean: Project A, Project B")
    }

    func testUsageErrorProperties() {
        let error = BasecampError.usage(message: "Bad argument", hint: "Use --flag")
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

    // MARK: - Field-keyed 422 bodies
    //
    // Native mirrors of the conformance error-mapping "field-errors" cases:
    // Swift has no conformance runner, so these tests replay the same fixture
    // bodies and assert the same flattened-message substrings. The structured
    // fieldErrors slot is deliberately absent here — extending the .validation
    // associated values is source-breaking and awaits a deliberate break.

    func testFromHTTPResponse422FieldKeyedFlattensIntoMessage() {
        let body = try! JSONSerialization.data(
            withJSONObject: ["errors": ["color": ["is not a valid color"]]]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        if case .validation(let message, let status, _, _) = error {
            XCTAssertEqual(message, "color: is not a valid color")
            XCTAssertEqual(status, 422)
        } else {
            XCTFail("Expected .validation")
        }
    }

    func testFromHTTPResponse422FieldKeyedSortsAndJoins() {
        let body = try! JSONSerialization.data(
            withJSONObject: [
                "errors": [
                    "name": ["can't be blank", "is too short"],
                    "color": ["is not a valid color"],
                ]
            ]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        XCTAssertEqual(error.message, "color: is not a valid color, name: can't be blank; is too short")
    }

    func testFromHTTPResponse422FieldKeyedAppendsToTopLevelError() {
        let body = try! JSONSerialization.data(
            withJSONObject: [
                "error": "Validation failed",
                "errors": ["color": ["is not a valid color"]],
            ]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        XCTAssertEqual(error.message, "Validation failed (color: is not a valid color)")
    }

    func testFromHTTPResponse400FieldKeyedExtractsToo() {
        let body = try! JSONSerialization.data(
            withJSONObject: ["errors": ["color": ["is not a valid color"]]]
        )
        let error = BasecampError.fromHTTPResponse(status: 400, data: body, headers: [:], requestId: nil)
        if case .validation(let message, let status, _, _) = error {
            XCTAssertEqual(message, "color: is not a valid color")
            XCTAssertEqual(status, 400)
        } else {
            XCTFail("Expected .validation")
        }
    }

    func testFromHTTPResponse403DoesNotFlattenFieldErrors() {
        let body = try! JSONSerialization.data(
            withJSONObject: ["errors": ["color": ["is not a valid color"]]]
        )
        let error = BasecampError.fromHTTPResponse(status: 403, data: body, headers: [:], requestId: nil)
        XCTAssertFalse(error.message.contains("is not a valid color"))
    }

    func testFromHTTPResponse422FieldKeyedSkipsMalformedEntries() {
        let body = try! JSONSerialization.data(
            withJSONObject: [
                "errors": [
                    "color": "not an array",
                    "name": ["can't be blank"],
                    "empty": [],
                    "mixed": [42, "is invalid"],
                ] as [String: Any]
            ]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        XCTAssertEqual(error.message, "mixed: is invalid, name: can't be blank")
    }

    func testFromHTTPResponse422UnusableErrorsShapeFallsBack() {
        let shapes: [Any] = [["color": "not an array"], [Any](), "nope", [String: Any]()]
        for shape in shapes {
            let body = try! JSONSerialization.data(withJSONObject: ["errors": shape])
            let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
            XCTAssertFalse(error.message.contains(":"), "unexpected flattening for \(shape)")
        }
    }

    func testFromHTTPResponse422SurvivesNonStringErrorSibling() {
        let body = try! JSONSerialization.data(
            withJSONObject: [
                "error": ["base": 1],
                "error_description": 42,
                "errors": ["color": ["is not a valid color"]],
            ] as [String: Any]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        XCTAssertEqual(error.message, "color: is not a valid color")
        XCTAssertNil(error.hint)
    }

    func testFromHTTPResponse422AppendsAfterMessageFallback() {
        let body = try! JSONSerialization.data(
            withJSONObject: [
                "message": "Validation failed",
                "errors": ["color": ["is not a valid color"]],
            ] as [String: Any]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        XCTAssertEqual(error.message, "Validation failed (color: is not a valid color)")
    }

    func testFromHTTPResponseErrorKeyWinsOverMessageKey() {
        let body = try! JSONSerialization.data(
            withJSONObject: ["error": "from error", "message": "from message"] as [String: Any]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        XCTAssertEqual(error.message, "from error")
    }

    func testFromHTTPResponse422ProtoFieldNameIsOrdinary() {
        let body = try! JSONSerialization.data(
            withJSONObject: [
                "errors": ["__proto__": ["is reserved"], "color": ["is not a valid color"]],
            ] as [String: Any]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        XCTAssertEqual(error.message, "__proto__: is reserved, color: is not a valid color")
    }

    func testFromHTTPResponse422FieldKeyedTruncatesAfterFlattening() {
        let longMessage = String(repeating: "x", count: 600)
        let body = try! JSONSerialization.data(
            withJSONObject: ["errors": ["color": [longMessage]]]
        )
        let error = BasecampError.fromHTTPResponse(status: 422, data: body, headers: [:], requestId: nil)
        XCTAssertEqual(error.message.count, 500)
        XCTAssertTrue(error.message.hasPrefix("color: xxx"))
        XCTAssertTrue(error.message.hasSuffix("..."))
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
