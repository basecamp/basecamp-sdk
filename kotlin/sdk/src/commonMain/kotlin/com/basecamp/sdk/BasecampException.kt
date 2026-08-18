package com.basecamp.sdk

import kotlinx.serialization.SerializationException

/**
 * Sealed class hierarchy for Basecamp API errors.
 *
 * Enables exhaustive `when` matching for error handling:
 * ```kotlin
 * try {
 *     account.todos.get(todoId)
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

    /**
     * Generic API error (5xx or unexpected status codes).
     *
     * [httpStatus] is nullable because not every API error has one. A composite
     * (SPEC §18) can fail on a *successful* response whose body is malformed —
     * the transport returned 2xx, so no status describes the failure. SPEC §6
     * sanctions that shape as a statusless, non-retryable `api_error`; passing a
     * placeholder like `0` would claim an HTTP status that never existed.
     */
    class Api private constructor(
        message: String,
        httpStatus: Int?,
        hint: String?,
        retryable: Boolean,
        requestId: String?,
        cause: Throwable?,
        /**
         * The response decoder's own refusal, set on exactly two things: the
         * exception [com.basecamp.sdk.services.BaseService] raises for a
         * malformed 2xx body, and the SPEC §18 composites' restatement of that
         * same failure with their own escape hatch attached. Null on every
         * other [Api].
         *
         * The SPEC §18 composites re-hint a decode failure of the GET they
         * issue, so what they have to answer is "did this come out of the
         * *response decoder*". `cause is SerializationException` reads like an
         * answer and is not one: `BasecampHttpClient` propagates an
         * already-classified [BasecampException] from the auth strategy
         * untouched, so a token provider that classifies its own JSON failure
         * as `Api(cause = SerializationException(...))` matches that test and
         * gets relabelled a malformed response body — for a request that was
         * never sent (#730).
         *
         * Presence is the discriminator. The only constructor that sets this
         * slot is private, reached through the internal [Companion.malformedBody]
         * factory the decoder wrapper alone calls, so no caller reaches it by
         * writing the natural thing. The value is the decoder's own message,
         * which is what the composites were reaching into [cause] for.
         *
         * That is an accident guarantee and not an unforgeable one — see
         * [Companion.malformedBody] for what it does and does not stop.
         *
         * **Readable everywhere, settable in one place.** The guarantee above is
         * a property of the *producer*, not of who can look: the constructor
         * that fills this slot is private and the factory that reaches it is
         * internal, so widening the getter takes nothing away. It has to be
         * public. The Kotlin conformance runner asks this same question and
         * `:conformance` is a separate Gradle module, so `internal` is invisible
         * to it — it read `cause is SerializationException` instead and carried
         * the #730 misread one module over until #750. `@PublishedApi internal`
         * does not help: it lifts an internal declaration into the ABI for
         * public inline functions *of the same module* and still cannot be
         * named from another module's source. Swift's `BasecampError`
         * `decodeFailure` is public for the same reason and answers the same
         * question (#750).
         *
         * [cause] still carries the same exception it always did, and a
         * [kotlinx.serialization.MissingFieldException] there still reports its
         * `missingFields`. It is not the discriminator: reading it as one is
         * exactly the #730 bug.
         */
        val decodeFailure: SerializationException?,
    ) : BasecampException(message, CODE_API, hint, httpStatus, retryable, requestId, cause) {

        constructor(
            message: String,
            httpStatus: Int? = null,
            hint: String? = null,
            retryable: Boolean = httpStatus != null && httpStatus in 500..599,
            requestId: String? = null,
            cause: Throwable? = null,
        ) : this(message, httpStatus, hint, retryable, requestId, cause, decodeFailure = null)

        internal companion object {
            /**
             * The SPEC §6 malformed-2xx-body shape, and the only producer of
             * [decodeFailure]. Statusless because the request succeeded, so no
             * HTTP status describes the failure, and non-retryable because
             * re-requesting cannot repair a malformed body — neither is a
             * caller's to vary, which is why they are fixed here rather than
             * passed.
             *
             * [hint] is passed, because the §18 composites restate this failure
             * with their own escape hatch attached and the restatement is still
             * the same malformed body. Restating it through the public
             * constructor would have dropped the marker, which reads as "this
             * was not a decode failure after all" to everything downstream —
             * the conformance runner included, where it means "a mock body that
             * needs repairing" is reported as an ordinary `api_error` instead
             * (#750).
             *
             * A factory and not a constructor, because `internal` does not
             * survive the JVM boundary for `<init>`: constructors cannot be
             * name-mangled, so an internal *constructor* is emitted public and
             * Java sees `Api(String, SerializationException)`. That would be the
             * shortest overload Java is offered — Kotlin default arguments do
             * not exist for Java callers, which see only the full-arity public
             * constructor — so a Java-authored `AuthStrategy` writing the
             * natural `new Api(message, decodeError)` would set this slot and
             * bring back the exact bug the slot exists to kill. Internal
             * *functions* are name-mangled (`malformedBody$…`), so this one is
             * not what a Java caller reaches for. `ApiConstructorSurfaceTest`
             * holds both halves of that on the JVM target.
             *
             * **What this does not claim.** A mangled name is still a legal Java
             * identifier, so Java source CAN call
             * `Api.Companion.malformedBody$…` on purpose and set the marker.
             * That is left open deliberately. The failure being prevented is an
             * accident — picking the convenient overload — and the deliberate
             * case buys its author nothing: the same `AuthStrategy` can already
             * throw, through the *public* constructor, an `Api` carrying the
             * composite's own message and hint verbatim, for an identical
             * user-visible outcome. Hiding this factory behind a synthetic
             * bridge would close a path whose equivalent stays one line away,
             * which is armour, not a guarantee.
             */
            internal fun malformedBody(
                message: String,
                hint: String? = null,
                decodeFailure: SerializationException,
            ): Api = Api(
                message,
                httpStatus = null,
                hint = hint,
                retryable = false,
                requestId = null,
                cause = decodeFailure,
                decodeFailure = decodeFailure,
            )
        }
    }

    /** Validation error (400, 422). */
    class Validation(
        message: String,
        hint: String? = null,
        httpStatus: Int = 422,
        requestId: String? = null,
        /**
         * Field-keyed validation messages from a 400/422 body — either
         * `{"errors": {"field": ["msg", ...]}}`, the Rails RecordInvalid
         * rendering, or the same map with no wrapper at all
         * (`{"field": ["msg", ...]}`), which some controllers emit. Null for
         * every other error shape. The flattened form is
         * also folded into the message; this slot preserves the raw,
         * untruncated per-field messages.
         */
        val fieldErrors: Map<String, List<String>>? = null,
    ) : BasecampException(message, CODE_VALIDATION, hint, httpStatus, false, requestId)

    /**
     * An account limit blocks the request (507) — file storage exhausted, or a
     * webhook ceiling reached.
     *
     * Never retryable: no amount of backoff frees storage or raises a plan
     * limit. Distinct from [Api] for exactly that reason, since a 507 would
     * otherwise land there as a retryable 5xx.
     */
    class LimitExceeded(
        message: String = "Account limit reached",
        hint: String? = null,
        requestId: String? = null,
    ) : BasecampException(message, CODE_LIMIT_EXCEEDED, hint, 507, false, requestId)

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
        const val CODE_LIMIT_EXCEEDED = "limit_exceeded"

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
        private const val EXIT_LIMIT_EXCEEDED = 10

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
            CODE_LIMIT_EXCEEDED -> EXIT_LIMIT_EXCEEDED
            else -> EXIT_API
        }

        /** Maximum length for error messages to prevent unbounded memory growth. */
        private const val MAX_ERROR_MESSAGE_LENGTH = 500

        /** Truncates error messages to a safe length. */
        internal fun truncateMessage(s: String): String =
            if (s.length <= MAX_ERROR_MESSAGE_LENGTH) s
            else s.take(MAX_ERROR_MESSAGE_LENGTH - 3) + "..."

        /**
         * Flattens a field-keyed errors map as "field: msg1; msg2, other: msg"
         * — fields sorted lexicographically, a field's messages joined with
         * "; ", fields joined with ", ". This shape is shared by all six SDKs;
         * change it everywhere or nowhere.
         */
        internal fun flattenFieldErrors(fieldErrors: Map<String, List<String>>): String =
            fieldErrors.keys.sorted()
                .joinToString(", ") { field -> "$field: ${fieldErrors.getValue(field).joinToString("; ")}" }

        /** Creates a [BasecampException] from an HTTP status code and response body. */
        fun fromHttpStatus(
            httpStatus: Int,
            message: String? = null,
            hint: String? = null,
            requestId: String? = null,
            retryAfterSeconds: Int? = null,
            fieldErrors: Map<String, List<String>>? = null,
        ): BasecampException {
            val msg = truncateMessage(message ?: "Request failed (HTTP $httpStatus)")
            return when (httpStatus) {
                401 -> Auth(msg, hint, requestId)
                403 -> Forbidden(msg, hint, requestId)
                404 -> NotFound(msg, hint, requestId)
                429 -> RateLimit(retryAfterSeconds, msg, hint, requestId)
                400, 422 -> Validation(msg, hint, httpStatus, requestId, fieldErrors)
                // A 5xx status carrying a client fact: the account is out of
                // storage, or at its webhook ceiling. Matched before the else
                // arm, which would make it a retryable Api.
                507 -> LimitExceeded(msg, hint, requestId)
                else -> Api(msg, httpStatus, hint, httpStatus in 500..599, requestId)
            }
        }
    }
}
