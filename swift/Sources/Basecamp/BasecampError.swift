import Foundation

/// Structured error type for Basecamp API errors.
///
/// Uses an enum with associated values for exhaustive `switch` matching.
/// Each case carries context-specific metadata (message, hint, status codes, etc.).
///
/// ```swift
/// do {
///     let todo = try await account.todos.get(todoId: 456)
/// } catch let error as BasecampError {
///     switch error {
///     case .notFound(let message, _, _):
///         print("Not found: \(message)")
///     case .rateLimit(_, let retryAfter, _, _):
///         if let seconds = retryAfter {
///             try await Task.sleep(nanoseconds: UInt64(seconds) * 1_000_000_000)
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
    ///
    /// `decodeFailure` is the response decoder's own refusal, and it is set on
    /// exactly one thing: the SPEC §6 statusless `api_error` raised for a 2xx
    /// body the model would not decode, plus the §18 composites' restatement of
    /// that same failure with their escape hatch attached. Nil on every other
    /// `.api`, including the *other* statusless one — the pagination
    /// same-origin refusal, which is a deliberate guard and not a bad body.
    ///
    /// Read it through ``decodeFailure``, not by matching this case: that
    /// property is nil for every other error shape, and it will not move again
    /// when this case gains a sixth value.
    case api(
        message: String, httpStatus: Int?, hint: String?, requestId: String?,
        decodeFailure: (any Error & Sendable)?)

    /// Validation error (HTTP 400, 422).
    ///
    /// Field-keyed bodies — `{"errors": {"field": ["msg", ...]}}`, the Rails
    /// RecordInvalid rendering, and the unwrapped `{"field": ["msg", ...]}`
    /// some controllers emit — are flattened into `message` as
    /// "field: msg1; msg2, other: msg" and carried raw in `fieldErrors`.
    case validation(
        message: String, httpStatus: Int, hint: String?, requestId: String?,
        fieldErrors: [String: [String]]?)

    /// A template copy requires confirmation before granting project access.
    case peopleConfirmationRequired(
        message: String, httpStatus: Int, hint: String?, requestId: String?,
        fieldErrors: [String: [String]]?, people: [TemplateLibraryConfirmationPerson])

    /// An account limit blocks the request (HTTP 507) — file storage
    /// exhausted, or a webhook ceiling reached. Never retryable: no amount of
    /// backoff frees storage or raises a plan limit. Distinct from `.api` for
    /// exactly that reason, since a 507 would otherwise land there as a
    /// retryable 5xx.
    case limitExceeded(message: String, hint: String?, requestId: String?)

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
        case .api(_, let status, _, _, _): status.map { $0 >= 500 } ?? false
        case .limitExceeded: false
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
        case .validation(_, let status, _, _, _): status
        case .peopleConfirmationRequired(_, let status, _, _, _, _): status
        case .api(_, let status, _, _, _): status
        case .limitExceeded: 507
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
        case .validation, .peopleConfirmationRequired: 9
        case .limitExceeded: 10
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
        case .api(_, _, let hint, _, _): hint
        case .limitExceeded(_, let hint, _): hint
        case .ambiguous(_, _, let hint): hint
        case .validation(_, _, let hint, _, _): hint
        case .peopleConfirmationRequired(_, _, let hint, _, _, _): hint
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
        case .api(let msg, _, _, _, _): msg
        case .limitExceeded(let msg, _, _): msg
        case .ambiguous(let resource, _, _): "Ambiguous \(resource)"
        case .validation(let msg, _, _, _, _): msg
        case .peopleConfirmationRequired(let msg, _, _, _, _, _): msg
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
        case .api(_, _, _, let id, _): id
        case .limitExceeded(_, _, let id): id
        case .ambiguous: nil
        case .validation(_, _, _, let id, _): id
        case .peopleConfirmationRequired(_, _, _, let id, _, _): id
        case .network: nil
        case .usage: nil
        }
    }

    /// Field-keyed validation messages from a 400/422 body — either
    /// `{"errors": {"field": ["msg", ...]}}`, the Rails RecordInvalid
    /// rendering, or the same map with no wrapper at all, which some
    /// controllers emit. Raw and untruncated; nil for every other error shape.
    /// The flattened form is also folded into `message`.
    public var fieldErrors: [String: [String]]? {
        switch self {
        case .validation(_, _, _, _, let fieldErrors): fieldErrors
        case .peopleConfirmationRequired(_, _, _, _, let fieldErrors, _): fieldErrors
        default: nil
        }
    }

    /// The people who need destination-project access before a template copy can start.
    public var confirmationPeople: [TemplateLibraryConfirmationPerson]? {
        switch self {
        case .peopleConfirmationRequired(_, _, _, _, _, let people): people
        default: nil
        }
    }

    /// The response decoder's own refusal, for a 2xx body this SDK would not
    /// decode; nil for every other error shape.
    ///
    /// Presence is the answer to "is this a malformed response body". Statuslessness
    /// is not: the pagination same-origin guard throws a statusless `.api` too,
    /// and the message is not either — it used to be, through a
    /// `@_spi(Conformance)` substring test whose contract was the wording of a
    /// sentence, so a reworded message moved the answer and a caller composing
    /// their own `.api` containing the phrase forged one (#750).
    ///
    /// The value is a `DecodingError` for a typed decode and a `CocoaError` for
    /// a body that is not JSON at all (the wrapped-list path reaches
    /// `JSONSerialization`, which reports that instead) — so match on the
    /// concrete type when the distinction matters rather than assuming one.
    ///
    /// This is a marker against an *accident*, not a forgery: `.api` is a public
    /// case with no private payload, so a caller can construct one carrying
    /// anything. That is unchanged from the phrase it replaces, and there is
    /// nothing to gain — the same caller can already throw an `.api` carrying a
    /// composite's message and hint verbatim. What it buys is that the SDK's own
    /// two producers are the only *plausible* ones, and that nobody has to know
    /// how this SDK words a sentence to tell the shapes apart.
    public var decodeFailure: (any Error & Sendable)? {
        switch self {
        case .api(_, _, _, _, let decodeFailure): decodeFailure
        default: nil
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
        // "error" wins; "message" is the SPEC §6 step-4 fallback.
        let serverMessage = ((body?["error"] as? String) ?? (body?["message"] as? String))
            .map { truncate($0) }
        // SPEC §6 step 5: the fixed code-bearing phrase, never
        // localizedString(forStatusCode:) — the platform table is localized
        // and empty of meaning for an unregistered code.
        let message = serverMessage ?? "Request failed (HTTP \(status))"
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
            var validationMessage = message
            let fieldErrors = parseFieldErrors(body)
            if let fieldErrors {
                let flat = flattenFieldErrors(fieldErrors)
                // Appended in parentheses after a top-level message, standing
                // alone otherwise; truncated after flattening so the appended
                // tail is capped too.
                validationMessage = truncate(serverMessage.map { "\($0) (\(flat))" } ?? flat)
            }
            if status == 422, let people = parseTemplateLibraryConfirmationPeople(body) {
                return .peopleConfirmationRequired(
                    message: validationMessage, httpStatus: status,
                    hint: hint, requestId: requestId, fieldErrors: fieldErrors, people: people
                )
            }
            return .validation(
                message: validationMessage, httpStatus: status,
                hint: hint, requestId: requestId, fieldErrors: fieldErrors
            )
        case 507:
            // A 5xx status carrying a client fact: the account is out of
            // storage, or at its webhook ceiling. Decided before the default
            // arm, which would make it a retryable .api.
            return .limitExceeded(message: message, hint: hint, requestId: requestId)
        default:
            return .api(
                message: message, httpStatus: status,
                hint: hint, requestId: requestId, decodeFailure: nil
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

    /// Truncates a caller-supplied value (e.g. a URL) embedded in an error
    /// message. Internal so guard sites elsewhere in the SDK keep their error
    /// messages bounded too.
    static func truncate(_ s: String) -> String {
        if s.count <= maxMessageLength { return s }
        return String(s.prefix(maxMessageLength - 3)) + "..."
    }

    private static func parseTemplateLibraryConfirmationPeople(
        _ body: [String: Any]?
    ) -> [TemplateLibraryConfirmationPerson]? {
        guard let values = body?["people"] as? [Any], !values.isEmpty else { return nil }
        var people: [TemplateLibraryConfirmationPerson] = []
        people.reserveCapacity(values.count)
        for value in values {
            guard
                let person = value as? [String: Any],
                let id = person["id"] as? Int, id > 0,
                let name = person["name"] as? String, !name.isEmpty,
                let avatarUrl = person["avatar_url"] as? String, !avatarUrl.isEmpty
            else { return nil }
            people.append(TemplateLibraryConfirmationPerson(avatarUrl: avatarUrl, id: id, name: name))
        }
        return people
    }

    /// Extracts the field-keyed validation errors map from a parsed error
    /// body — the Rails RecordInvalid rendering
    /// `{"errors": {"field": ["msg", ...]}}`. Entries whose value is not an
    /// array are skipped, non-string elements are dropped, and a map with no
    /// usable entries is treated as absent (nil).
    private static func parseFieldErrors(_ body: [String: Any]?) -> [String: [String]]? {
        guard let errors = body?["errors"] as? [String: Any] else {
            return parseBareFieldErrors(body)
        }
        var fieldErrors: [String: [String]] = [:]
        for (field, value) in errors {
            guard let values = value as? [Any] else { continue }
            let messages = values.compactMap { $0 as? String }
            if !messages.isEmpty {
                fieldErrors[field] = messages
            }
        }
        return fieldErrors.isEmpty ? nil : fieldErrors
    }

    /// Extracts an unwrapped field map — the `render json: @webhook.errors`
    /// rendering, where the whole body is `{"field": ["msg", ...]}`. The gate is
    /// all-or-nothing by design (SPEC §6 step 2): with no `errors` key to
    /// declare intent, only shape distinguishes a field map from any other JSON
    /// object, so a single non-conforming member means this is not one. Returns
    /// nil unless every member is a non-empty array of non-empty strings.
    private static func parseBareFieldErrors(_ body: [String: Any]?) -> [String: [String]]? {
        guard let body, !body.isEmpty else { return nil }
        // Only "errors" is structurally reserved (it belongs to the wrapped
        // path). "error" and "message" are not excluded by name: a flat body
        // carries them as strings, which the shape gate below already rejects.
        guard body["errors"] == nil else { return nil }

        var fieldErrors: [String: [String]] = [:]
        for (field, value) in body {
            // Per-element, never an atomic `as? [String: [String]]` cast: JSON
            // null bridges to NSNull, which no atomic cast would tolerate.
            guard let values = value as? [Any], !values.isEmpty else { return nil }
            var messages: [String] = []
            messages.reserveCapacity(values.count)
            for element in values {
                guard let message = element as? String, !message.isEmpty else { return nil }
                messages.append(message)
            }
            fieldErrors[field] = messages
        }
        return fieldErrors
    }

    /// Flattens a field-keyed errors map as "field: msg1; msg2, other: msg" —
    /// fields sorted lexicographically, a field's messages joined with "; ",
    /// fields joined with ", ". This shape is shared by all six SDKs; change
    /// it everywhere or nowhere.
    private static func flattenFieldErrors(_ fieldErrors: [String: [String]]) -> String {
        fieldErrors.keys.sorted()
            .map { "\($0): \(fieldErrors[$0, default: []].joined(separator: "; "))" }
            .joined(separator: ", ")
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
