package com.basecamp.sdk

/**
 * Sealed class hierarchy for Basecamp API errors.
 *
 * Enables exhaustive `when` matching for error handling:
 * ```kotlin
 * try {
 *     account.todos.get(projectId, todoId)
 * } catch (e: BasecampException) {
 *     when (e) {
 *         is BasecampException.Auth -> println("Token expired")
 *         is BasecampException.NotFound -> println("Not found")
 *         is BasecampException.RateLimit -> println("Retry in ${e.retryAfterSeconds}s")
 *         is BasecampException.Forbidden -> println("Access denied")
 *         is BasecampException.Validation -> println("Invalid input: ${e.message}")
 *         is BasecampException.Network -> println("Network error")
 *         is BasecampException.Api -> println("Server error: ${e.httpStatus}")
 *         is BasecampException.Usage -> println("Bad arguments: ${e.message}")
 *     }
 * }
 * ```
 */
sealed class BasecampException(
    message: String,
    /** Error category code matching the Go/TS/Ruby SDKs. */
    val code: String,
    /** User-friendly hint for resolving the error. */
    val hint: String? = null,
    /** HTTP status code that caused the error, if applicable. */
    val httpStatus: Int? = null,
    /** Whether the operation can be retried. */
    val retryable: Boolean = false,
    /** Request ID from the server for debugging. */
    val requestId: String? = null,
    cause: Throwable? = null,
) : Exception(message, cause) {

    /** Exit code for CLI applications (matches Go/TS/Ruby SDKs). */
    val exitCode: Int get() = exitCodeFor(code)

    /** Authentication error (401). */
    class Auth(
        message: String = "Authentication required",
        hint: String? = "Check your access token or refresh it if expired",
        requestId: String? = null,
        cause: Throwable? = null,
    ) : BasecampException(message, CODE_AUTH, hint, 401, false, requestId, cause)

    /** Forbidden error (403). */
    class Forbidden(
        message: String = "Access denied",
        hint: String? = "You do not have permission to access this resource",
        requestId: String? = null,
        cause: Throwable? = null,
    ) : BasecampException(message, CODE_FORBIDDEN, hint, 403, false, requestId, cause)

    /** Not found error (404). */
    class NotFound(
        message: String = "Resource not found",
        hint: String? = null,
        requestId: String? = null,
        cause: Throwable? = null,
    ) : BasecampException(message, CODE_NOT_FOUND, hint, 404, false, requestId, cause)

    /** Rate limit error (429). Retryable with optional Retry-After. */
    class RateLimit(
        /** Number of seconds to wait before retrying, from the Retry-After header. */
        val retryAfterSeconds: Int? = null,
        message: String = "Rate limit exceeded",
        hint: String? = retryAfterSeconds?.let { "Retry after $it seconds" } ?: "Please slow down requests",
        requestId: String? = null,
        cause: Throwable? = null,
    ) : BasecampException(message, CODE_RATE_LIMIT, hint, 429, true, requestId, cause)

    /** Network error (connection failures, DNS, timeout). Retryable. */
    class Network(
        message: String = "Network error",
        hint: String? = "Check your network connection",
        cause: Throwable? = null,
    ) : BasecampException(message, CODE_NETWORK, hint, null, true, null, cause)

    /** Generic API error (5xx or unexpected status codes). */
    class Api(
        message: String,
        httpStatus: Int,
        hint: String? = null,
        retryable: Boolean = httpStatus in 500..599,
        requestId: String? = null,
        cause: Throwable? = null,
    ) : BasecampException(message, CODE_API, hint, httpStatus, retryable, requestId, cause)

    /** Validation error (400, 422). */
    class Validation(
        message: String,
        hint: String? = null,
        httpStatus: Int = 422,
        requestId: String? = null,
    ) : BasecampException(message, CODE_VALIDATION, hint, httpStatus, false, requestId)

    /** Usage error (bad arguments, configuration errors). */
    class Usage(
        message: String,
        hint: String? = null,
    ) : BasecampException(message, CODE_USAGE, hint)

    companion object {
        const val CODE_AUTH = "auth_required"
        const val CODE_FORBIDDEN = "forbidden"
        const val CODE_NOT_FOUND = "not_found"
        const val CODE_RATE_LIMIT = "rate_limit"
        const val CODE_NETWORK = "network"
        const val CODE_API = "api_error"
        const val CODE_VALIDATION = "validation"
        const val CODE_USAGE = "usage"

        private const val EXIT_OK = 0
        private const val EXIT_USAGE = 1
        private const val EXIT_NOT_FOUND = 2
        private const val EXIT_AUTH = 3
        private const val EXIT_FORBIDDEN = 4
        private const val EXIT_RATE_LIMIT = 5
        private const val EXIT_NETWORK = 6
        private const val EXIT_API = 7

        /** Maps an error code to a CLI exit code. */
        fun exitCodeFor(code: String): Int = when (code) {
            CODE_USAGE, CODE_VALIDATION -> EXIT_USAGE
            CODE_NOT_FOUND -> EXIT_NOT_FOUND
            CODE_AUTH -> EXIT_AUTH
            CODE_FORBIDDEN -> EXIT_FORBIDDEN
            CODE_RATE_LIMIT -> EXIT_RATE_LIMIT
            CODE_NETWORK -> EXIT_NETWORK
            CODE_API -> EXIT_API
            else -> EXIT_API
        }

        /** Maximum length for error messages to prevent unbounded memory growth. */
        private const val MAX_ERROR_MESSAGE_LENGTH = 500

        /** Truncates error messages to a safe length. */
        internal fun truncateMessage(s: String): String =
            if (s.length <= MAX_ERROR_MESSAGE_LENGTH) s
            else s.take(MAX_ERROR_MESSAGE_LENGTH - 3) + "..."

        /** Creates a [BasecampException] from an HTTP status code and response body. */
        fun fromHttpStatus(
            httpStatus: Int,
            message: String? = null,
            hint: String? = null,
            requestId: String? = null,
            retryAfterSeconds: Int? = null,
        ): BasecampException {
            val msg = truncateMessage(message ?: "Request failed (HTTP $httpStatus)")
            return when (httpStatus) {
                401 -> Auth(msg, hint, requestId)
                403 -> Forbidden(msg, hint, requestId)
                404 -> NotFound(msg, hint, requestId)
                429 -> RateLimit(retryAfterSeconds, msg, hint, requestId)
                400, 422 -> Validation(msg, hint, httpStatus, requestId)
                else -> Api(msg, httpStatus, hint, httpStatus in 500..599, requestId)
            }
        }
    }
}
