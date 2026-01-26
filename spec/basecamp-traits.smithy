$version: "2"

namespace basecamp.traits

use smithy.api#documentation
use smithy.api#trait
use smithy.openapi#specificationExtension

// ============================================================================
// Bridge Traits - These emit x-basecamp-* extensions to OpenAPI
// ============================================================================

/// Retry semantics for Basecamp API operations.
/// Emits x-basecamp-retry extension to OpenAPI for SDK code generators.
@trait(selector: "operation")
@specificationExtension(as: "x-basecamp-retry")
structure basecampRetry {
    /// Maximum number of retry attempts (default: 3)
    maxAttempts: Integer

    /// Base delay in milliseconds between retries (default: 1000)
    baseDelayMs: Integer

    /// Backoff strategy: "exponential" | "linear" | "constant"
    backoff: String

    /// HTTP status codes that trigger a retry (e.g., [429, 503])
    retryOn: RetryStatusCodes
}

list RetryStatusCodes {
    member: Integer
}

/// Pagination semantics for Basecamp list operations.
/// Emits x-basecamp-pagination extension to OpenAPI for SDK code generators.
@trait(selector: "operation")
@specificationExtension(as: "x-basecamp-pagination")
structure basecampPagination {
    /// Pagination style: "link" (Link header RFC5988), "cursor", or "page"
    style: String

    /// Name of the query parameter for page number (if style is "page")
    pageParam: String

    /// Name of the response header containing total count
    totalCountHeader: String

    /// Maximum items per page (server default)
    maxPageSize: Integer
}

/// Idempotency semantics for Basecamp write operations.
/// Emits x-basecamp-idempotent extension to OpenAPI for SDK code generators.
@trait(selector: "operation")
@specificationExtension(as: "x-basecamp-idempotent")
structure basecampIdempotent {
    /// Whether the operation supports client-provided idempotency keys
    keySupported: Boolean

    /// Header name for idempotency key (if supported)
    keyHeader: String

    /// Whether the operation is naturally idempotent (same input = same result)
    natural: Boolean
}

/// Marks members containing sensitive data that should not be logged.
/// Emits x-basecamp-sensitive extension to OpenAPI for SDK code generators.
@trait(selector: "structure > member")
@specificationExtension(as: "x-basecamp-sensitive")
structure basecampSensitive {
    /// Category of sensitive data: "pii", "credential", "financial", "health"
    category: String

    /// Whether the value should be redacted in logs (default: true)
    redact: Boolean
}

// ============================================================================
// Legacy Traits - Keep for backward compatibility (not emitted to OpenAPI)
// ============================================================================

@trait(selector: "operation")
@documentation("Pagination semantics for BasecampJson protocol (legacy)")
@deprecated(message: "Use basecampPagination instead for OpenAPI bridge support")
structure pagination {
    @documentation("Pagination style: link | cursor | none")
    style: String
}

@trait(selector: "operation")
@documentation("Retry semantics for BasecampJson protocol (legacy)")
@deprecated(message: "Use basecampRetry instead for OpenAPI bridge support")
structure retry {
    @documentation("max retries, base delay, and backoff formula")
    max: Integer
    base_delay_seconds: Integer
    backoff: String
}

@trait(selector: "operation")
@documentation("Idempotency semantics for BasecampJson protocol (legacy)")
@deprecated(message: "Use basecampIdempotent instead for OpenAPI bridge support")
structure idempotency {
    @documentation("Whether idempotency keys are supported")
    supported: Boolean
}
