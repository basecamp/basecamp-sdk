import Foundation

/// Structured error type for Basecamp API errors.
///
/// Uses an enum with associated values for exhaustive `switch` matching.
/// Each case carries context-specific metadata (message, hint, status codes, etc.).
///
/// ```swift
/// do {
///     let todo = try await account.todos.get(projectId: 123, todoId: 456)
/// } catch let error as BasecampError {
///     switch error {
///     case .notFound(let message, _, _):
///         print("Not found: \(message)")
///     case .rateLimit(_, let retryAfter, _, _):
///         if let seconds = retryAfter {
///             try await Task.sleep(for: .seconds(seconds))
///         }
///     default:
///         print(error.localizedDescription)
///     }
/// }
/// ```
public enum BasecampError: Error, Sendable, LocalizedError {
    /// Authentication required (HTTP 401).
    case auth(message: String, hint: String?, requestId: String?)

    /// Access denied (HTTP 403).
    case forbidden(message: String, hint: String?, requestId: String?)

    /// Resource not found (HTTP 404).
    case notFound(message: String, hint: String?, requestId: String?)

    /// Rate limit exceeded (HTTP 429). Retryable.
    case rateLimit(message: String, retryAfterSeconds: Int?, hint: String?, requestId: String?)

    /// Network connectivity error. Retryable.
    case network(message: String, cause: (any Error & Sendable)?)

    /// Server or API error (typically 5xx).
    case api(message: String, httpStatus: Int?, hint: String?, requestId: String?)

    /// Validation error (HTTP 400, 422).
    case validation(message: String, httpStatus: Int, hint: String?, requestId: String?)

    /// Multiple matches found for a name or identifier.
    case ambiguous(resource: String, matches: [String], hint: String?)

    /// Client usage error (invalid arguments, bad configuration).
    case usage(message: String, hint: String?)

    // MARK: - Computed Properties

    /// Whether this error can be retried.
    public var isRetryable: Bool {
        switch self {
        case .rateLimit: true
        case .network: true
        case .api(_, let status, _, _): status.map { $0 >= 500 } ?? false
        case .ambiguous: false
        default: false
        }
    }

    /// The HTTP status code, if applicable.
    public var httpStatusCode: Int? {
        switch self {
        case .auth: 401
        case .forbidden: 403
        case .notFound: 404
        case .rateLimit: 429
        case .validation(_, let status, _, _): status
        case .api(_, let status, _, _): status
        case .ambiguous: nil
        case .network: nil
        case .usage: nil
        }
    }

    /// Exit code for CLI applications, matching Go/TS conventions.
    public var exitCode: Int {
        switch self {
        case .usage: 1
        case .notFound: 2
        case .auth: 3
        case .forbidden: 4
        case .rateLimit: 5
        case .network: 6
        case .api: 7
        case .ambiguous: 8
        case .validation: 9
        }
    }

    /// User-friendly hint for resolving the error.
    public var hint: String? {
        switch self {
        case .auth(_, let hint, _): hint
        case .forbidden(_, let hint, _): hint
        case .notFound(_, let hint, _): hint
        case .rateLimit(_, _, let hint, _): hint
        case .network: "Check your network connection"
        case .api(_, _, let hint, _): hint
        case .ambiguous(_, _, let hint): hint
        case .validation(_, _, let hint, _): hint
        case .usage(_, let hint): hint
        }
    }

    /// The error message.
    public var message: String {
        switch self {
        case .auth(let msg, _, _): msg
        case .forbidden(let msg, _, _): msg
        case .notFound(let msg, _, _): msg
        case .rateLimit(let msg, _, _, _): msg
        case .network(let msg, _): msg
        case .api(let msg, _, _, _): msg
        case .ambiguous(let resource, _, _): "Ambiguous \(resource)"
        case .validation(let msg, _, _, _): msg
        case .usage(let msg, _): msg
        }
    }

    /// Server request ID for debugging.
    public var requestId: String? {
        switch self {
        case .auth(_, _, let id): id
        case .forbidden(_, _, let id): id
        case .notFound(_, _, let id): id
        case .rateLimit(_, _, _, let id): id
        case .api(_, _, _, let id): id
        case .ambiguous: nil
        case .validation(_, _, _, let id): id
        case .network: nil
        case .usage: nil
        }
    }

    // MARK: - LocalizedError

    public var errorDescription: String? {
        if let hint {
            return "\(message): \(hint)"
        }
        return message
    }

    // MARK: - Factory Methods

    /// Creates an appropriate error from an HTTP response.
    static func fromHTTPResponse(
        status: Int,
        data: Data?,
        headers: [String: String],
        requestId: String?
    ) -> BasecampError {
        let body = data.flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: Any] }
        let message = truncate(
            (body?["error"] as? String) ?? HTTPURLResponse.localizedString(forStatusCode: status)
        )
        let hint = truncate(body?["error_description"] as? String)

        switch status {
        case 401:
            return .auth(message: message, hint: hint, requestId: requestId)
        case 403:
            return .forbidden(message: message, hint: hint, requestId: requestId)
        case 404:
            return .notFound(message: message, hint: hint, requestId: requestId)
        case 429:
            let retryAfter = parseRetryAfter(headers["Retry-After"])
            let retryHint = retryAfter.map { "Retry after \($0) seconds" } ?? hint
            return .rateLimit(
                message: message, retryAfterSeconds: retryAfter,
                hint: retryHint, requestId: requestId
            )
        case 400, 422:
            return .validation(
                message: message, httpStatus: status,
                hint: hint, requestId: requestId
            )
        default:
            return .api(
                message: message, httpStatus: status,
                hint: hint, requestId: requestId
            )
        }
    }

    // MARK: - Private Helpers

    private static let maxMessageLength = 500

    private static func truncate(_ s: String?) -> String? {
        guard let s, !s.isEmpty else { return nil }
        if s.count <= maxMessageLength { return s }
        return String(s.prefix(maxMessageLength - 3)) + "..."
    }

    private static func truncate(_ s: String) -> String {
        if s.count <= maxMessageLength { return s }
        return String(s.prefix(maxMessageLength - 3)) + "..."
    }

    /// Parses a Retry-After header value (seconds or HTTP-date).
    static func parseRetryAfter(_ value: String?) -> Int? {
        guard let value, !value.isEmpty else { return nil }
        if let seconds = Int(value), seconds > 0 {
            return seconds
        }
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.dateFormat = "EEE, dd MMM yyyy HH:mm:ss zzz"
        if let date = formatter.date(from: value) {
            let seconds = Int(date.timeIntervalSinceNow.rounded(.up))
            return seconds > 0 ? seconds : nil
        }
        return nil
    }
}
