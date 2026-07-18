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
 *         is BasecampException.Ambiguous -> println("Ambiguous: ${e.message}")
 *         is BasecampException.DiscoverySelection -> println("OAuth discovery: ${e.reason}")
 *         is BasecampException.DeviceFlow -> println("Device flow: ${e.reason}")
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

    /** Ambiguous match error (multiple resources match a name/identifier). */
    class Ambiguous(
        /** The type of resource that was ambiguous. */
        val resource: String,
        /** The matching resources. */
        val matches: List<String> = emptyList(),
        hint: String? = if (matches.isNotEmpty() && matches.size <= 5)
            "Did you mean: ${matches.joinToString(", ")}" else "Be more specific",
    ) : BasecampException("Ambiguous $resource", CODE_AMBIGUOUS, hint)

    /** Usage error (bad arguments, configuration errors). */
    class Usage(
        message: String,
        hint: String? = null,
    ) : BasecampException(message, CODE_USAGE, hint)

    /**
     * Hard resource-first OAuth discovery selection/validation failure
     * (SPEC.md §16). THROWN, never returned as a Launchpad fallback, so no
     * consumer can convert it into a Launchpad request. [reason] carries the
     * typed failure token (e.g. `ambiguous_issuers`, `issuer_mismatch`).
     *
     * Its [code] is `validation` for consumer/capability-shaped reasons
     * (`capability_unavailable`) and `api_error` for advertised-metadata faults,
     * mirroring the TypeScript reference.
     */
    class DiscoverySelection(
        /** Typed selection failure token; see SPEC.md §16 fallback table. */
        val reason: String,
        message: String,
        hint: String? = null,
        cause: Throwable? = null,
    ) : BasecampException(
        message,
        if (reason == "capability_unavailable") CODE_VALIDATION else CODE_API,
        hint,
        null,
        false,
        null,
        cause,
    )

    /**
     * Terminal RFC 8628 device authorization grant failure (SPEC.md §16). A
     * single [reason] carries the precise outcome; the parent [code] (and thus
     * [exitCode]) is DERIVED from it so callers can branch on either the precise
     * [reason] or the coarse [code]:
     *
     * | reason          | code            | retryable |
     * |-----------------|-----------------|-----------|
     * | `access_denied` | `auth_required` | no        |
     * | `expired`       | `auth_required` | no        |
     * | `transport`     | `network`       | yes       |
     * | `unavailable`   | `validation`    | no        |
     * | `cancelled`     | `usage`         | no        |
     *
     * Native coroutine cancellation propagates as [kotlin.coroutines.cancellation.CancellationException]
     * rather than becoming `DeviceFlow(cancelled)`; the `cancelled` reason exists
     * only for a non-native cancel signal (SPEC.md §16 terminal-outcomes table).
     */
    class DeviceFlow(
        /** Typed device-flow outcome; see the table above. */
        val reason: String,
        message: String = deviceFlowDefaultMessage(reason),
        cause: Throwable? = null,
    ) : BasecampException(
        message,
        deviceFlowCode(reason),
        null,
        null,
        reason == DEVICE_TRANSPORT,
        null,
        cause,
    )

    companion object {
        const val CODE_AUTH = "auth_required"
        const val CODE_FORBIDDEN = "forbidden"
        const val CODE_NOT_FOUND = "not_found"
        const val CODE_RATE_LIMIT = "rate_limit"
        const val CODE_NETWORK = "network"
        const val CODE_API = "api_error"
        const val CODE_VALIDATION = "validation"
        const val CODE_AMBIGUOUS = "ambiguous"
        const val CODE_USAGE = "usage"

        // RFC 8628 device-flow reasons (see [DeviceFlow]).
        const val DEVICE_ACCESS_DENIED = "access_denied"
        const val DEVICE_EXPIRED = "expired"
        const val DEVICE_TRANSPORT = "transport"
        const val DEVICE_UNAVAILABLE = "unavailable"
        const val DEVICE_CANCELLED = "cancelled"

        /** Derives a [DeviceFlow]'s parent error code from its reason. */
        private fun deviceFlowCode(reason: String): String = when (reason) {
            DEVICE_ACCESS_DENIED, DEVICE_EXPIRED -> CODE_AUTH
            DEVICE_TRANSPORT -> CODE_NETWORK
            DEVICE_UNAVAILABLE -> CODE_VALIDATION
            DEVICE_CANCELLED -> CODE_USAGE
            else -> CODE_API
        }

        /** Default human-readable message for a [DeviceFlow] reason. */
        private fun deviceFlowDefaultMessage(reason: String): String = when (reason) {
            DEVICE_ACCESS_DENIED -> "The authorization request was denied"
            DEVICE_EXPIRED -> "Device code expired before authorization completed"
            DEVICE_TRANSPORT -> "Device flow transport failure"
            DEVICE_UNAVAILABLE ->
                "The selected authorization server does not support the device authorization grant"
            DEVICE_CANCELLED -> "Device flow cancelled"
            else -> "Device flow failed: $reason"
        }

        private const val EXIT_OK = 0
        private const val EXIT_USAGE = 1
        private const val EXIT_NOT_FOUND = 2
        private const val EXIT_AUTH = 3
        private const val EXIT_FORBIDDEN = 4
        private const val EXIT_RATE_LIMIT = 5
        private const val EXIT_NETWORK = 6
        private const val EXIT_API = 7
        private const val EXIT_AMBIGUOUS = 8
        private const val EXIT_VALIDATION = 9

        /** Maps an error code to a CLI exit code. */
        fun exitCodeFor(code: String): Int = when (code) {
            CODE_USAGE -> EXIT_USAGE
            CODE_NOT_FOUND -> EXIT_NOT_FOUND
            CODE_AUTH -> EXIT_AUTH
            CODE_FORBIDDEN -> EXIT_FORBIDDEN
            CODE_RATE_LIMIT -> EXIT_RATE_LIMIT
            CODE_NETWORK -> EXIT_NETWORK
            CODE_API -> EXIT_API
            CODE_AMBIGUOUS -> EXIT_AMBIGUOUS
            CODE_VALIDATION -> EXIT_VALIDATION
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
