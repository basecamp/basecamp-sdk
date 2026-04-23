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

    /// Key within the response object containing the paginated array.
    /// When present, the response is a wrapper object (not a bare array)
    /// and the paginated items live under this key.
    key: String
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

/// Multipart file upload semantics for Basecamp API operations.
/// Emits x-basecamp-multipart extension to OpenAPI for SDK code generators.
/// The Smithy→OpenAPI mapper rewrites the request body from octet-stream
/// to multipart/form-data with the specified field name.
@trait(selector: "operation")
@specificationExtension(as: "x-basecamp-multipart")
structure basecampMultipart {
    /// The form field name for the file (e.g., "logo")
    @required
    field: String
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

/// Marks a URL member whose value must be fetched through the configured
/// API host with the SDK's Bearer credential; the API host 302s to a
/// short-lived signed URL that the consumer follows without auth.
///
/// Contract for consumers (SDKs and hand-rolled helpers):
///   1. Rewrite the URL's scheme + host to the configured API base URL,
///      preserving path, query, and fragment.
///   2. Issue an authenticated GET (Bearer credential + SDK User-Agent)
///      with auto-redirect disabled so the 3xx is captured.
///   3. On 301/302/303/307/308, read Location, close the first body,
///      and GET the resolved URL with a bare transport — no auth
///      headers, no logging middleware. The signature is the credential.
///   4. On direct 2xx, stream the first response body as-is.
///   5. Tests that exercise the download flow MUST stub with a URL
///      whose host matches the configured API base and whose path
///      matches the fixture shape (e.g.
///      `/{accountId}/buckets/{bucketId}/uploads/{id}/download/{filename}`),
///      AND MUST assert the unauthenticated hop actually fired.
///      "No assertion fired" and "assertion fired and passed" are
///      indistinguishable otherwise — both masked the bug behind
///      PR #278. Schema-shape-only tests (unmarshaling assertions that
///      set `download_url` without exercising transport) are exempt but
///      should still use an API-host-shaped URL for clarity.
///
/// The two-hop flow is not automatically retried end-to-end: streaming
/// body ownership passes to the caller after hop 2, so a retry would
/// double-consume. The authenticated first hop may be retried internally
/// per the SDK's standard retry policy (see PR #278 in the Go SDK).
///
/// This trait does NOT apply to pagination `Link: rel=next` URLs,
/// which are fetched as same-origin authed GETs without a redirect
/// step (see Go `Client.followPagination` at `client.go:499`).
///
/// Every SDK implements hops 1–4 in a language-native primitive;
/// external (cross-package/application-level) consumers MUST call the
/// public client method, not the raw HTTP client. References: Go
/// `AccountClient.DownloadURL`, TypeScript `BasecampClient.downloadURL`
/// (backed by `createDownloadURL`), Python `AccountClient.download_url`
/// / `AsyncAccountClient.download_url` (backed by `download_sync` /
/// `download_async`), Ruby `AccountClient#download_url`, Swift
/// `AccountClient.downloadURL`, Kotlin `AccountClient.downloadURL`.
///
/// SDK-internal service code (e.g., Go service methods that already
/// own an `OperationInfo` lifecycle, like `UploadsService.Download`)
/// MUST call the internal two-hop helper directly (Go:
/// `Client.fetchAPIDownload`) rather than the public primitive —
/// otherwise the public primitive's own `OnOperationStart`/`OnOperationEnd`
/// fires nested inside the caller's, creating a double-logged request.
///
/// Emits x-basecamp-auth-routable-url extension to OpenAPI for SDK code generators.
@trait(selector: "structure > member")
@specificationExtension(as: "x-basecamp-auth-routable-url")
structure basecampAuthRoutableUrl {}

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
