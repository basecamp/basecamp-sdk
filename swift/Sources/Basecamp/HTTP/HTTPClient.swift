import Foundation

/// Internal HTTP client that wraps a `Transport` with authentication,
/// retry with exponential backoff, ETag caching, and hooks lifecycle.
///
/// This is the workhorse of the SDK. It handles the full request lifecycle:
/// 1. Token injection (Bearer auth + User-Agent)
/// 2. Hooks notifications (request start/end)
/// 3. ETag cache check/store
/// 4. Retry with exponential backoff + jitter on 429/503
package final class HTTPClient: Sendable {
    private let transport: any Transport
    private let authStrategy: any AuthStrategy
    private let config: BasecampConfig
    private let hooks: any BasecampHooks
    private let cache: ETagCache?

    private static let maxJitterMs: UInt64 = 100
    private static let defaultBaseDelayMs: UInt64 = 1_000

    /// Ceiling on the backoff term (SPEC §7, "Backoff Ceiling"). Jitter is
    /// added after the clamp, so the longest single backoff sleep is this plus
    /// ``maxJitterMs``.
    static let maxBackoffDelayMs: UInt64 = 30_000

    /// HTTP methods that are naturally idempotent and therefore always
    /// retry-eligible (SPEC §7). Hoisted to a static constant so the retry gate
    /// on the hot path does not allocate a `Set` per request.
    private static let retryableMethods: Set<String> = ["GET", "HEAD", "PUT", "DELETE"]

    /// The download hop-1 retry policy (SPEC §14): a fixed three attempts when
    /// retry is enabled, retrying network errors plus these statuses — never
    /// 500, which stays aligned with the main GET loop's declared-set
    /// discipline rather than the error taxonomy's broader "all 5xx retryable"
    /// flag. Backoff is exponential from ``defaultBaseDelayMs``, honoring
    /// `Retry-After` on 429. The signed second hop is exempt: no retry, no auth.
    private static let downloadMaxAttempts = 3
    private static let downloadRetryOn: Set<Int> = [429, 502, 503, 504]

    /// Raised by the response-type guard when a `Transport` hands back a
    /// `URLResponse` that is not an `HTTPURLResponse`.
    ///
    /// It needs its own type rather than reusing `BasecampError.network`.
    /// Since #567 the retry loops route a transport-thrown `.network` to the
    /// retry branch, and a non-HTTP response is a deterministic programming
    /// error — retrying it three times just repeats it. This type never
    /// escapes the SDK: every site that raises it surfaces
    /// ``invalidResponseType`` instead, so the public error contract is
    /// exactly what it was.
    private struct InvalidResponseTypeError: Error {}

    /// The caller-facing error for a non-HTTP `URLResponse`. Unchanged from
    /// before #567 — same case, same message.
    private static var invalidResponseType: BasecampError {
        .network(message: "Invalid response type", cause: nil)
    }

    /// Whether a thrown error is the transport reporting a connectivity
    /// failure, and therefore eligible for the retry branch (SPEC §7).
    ///
    /// `Transport` is `public` and documented as a seam, so a consumer wiring
    /// in their own networking stack will normalize connection resets and
    /// timeouts into `BasecampError.network` — that is precisely the
    /// classification §6 defines for the condition, `retryable: true`. Before
    /// #567 both retry loops classified *every* `BasecampError` out of the
    /// transport as terminal, so the more carefully an implementer read the
    /// error taxonomy, the more certainly they disabled their own retries.
    ///
    /// Any other `BasecampError` (`.auth`, `.usage`, `.api`, …) is the
    /// transport's own final verdict on the request and stays terminal on
    /// sight, as does ``InvalidResponseTypeError``, which is not a transport
    /// failure at all.
    private static func isTransportNetworkFailure(_ error: any Error) -> Bool {
        guard let basecampError = error as? BasecampError else { return true }
        if case .network = basecampError { return true }
        return false
    }

    /// Whether an error represents cooperative cancellation.
    ///
    /// Swift concurrency raises `CancellationError`, but `URLSession` reports a
    /// cancelled task as `URLError(.cancelled)` — so checking only the former
    /// would treat a real cancelled download as a retryable network blip and
    /// spend the whole budget on a request the caller already abandoned.
    ///
    /// It also looks *through* ``BasecampError/network(message:cause:)``. A
    /// `Transport` that normalizes its failures into the SDK's own taxonomy —
    /// the shape #567 exists to support — reports a cancelled request as
    /// `.network(cause: CancellationError())` or `.network(cause:
    /// URLError(.cancelled))`. Testing only the outer type would classify one
    /// cancellation two ways: terminal when raw, retried (announced, slept on,
    /// re-authenticated, re-sent) when wrapped. Classifying a network error by
    /// meaning rather than by type is the whole point of #567, and cancellation
    /// is part of that meaning.
    ///
    /// The walk is bounded rather than recursive: the `cause` chain is
    /// caller-supplied, and a cycle in it must not hang the retry loop.
    private static func isCancellation(_ error: any Error) -> Bool {
        var current: (any Error)? = error

        for _ in 0..<maxCauseChainDepth {
            guard let candidate = current else { return false }
            if candidate is CancellationError { return true }
            if (candidate as? URLError)?.code == .cancelled { return true }
            guard let basecampError = candidate as? BasecampError,
                  case .network(_, let cause) = basecampError else { return false }
            current = cause
        }

        return false
    }

    /// How far ``isCancellation(_:)`` walks a `.network` cause chain. Any real
    /// chain is one or two links deep; the bound is what keeps a caller-built
    /// cycle from spinning.
    private static let maxCauseChainDepth = 8

    /// SPEC §9 projection of a download transport error's cause: the raw error
    /// renders the URL it failed on, and download URLs carry credentials, so it
    /// is never chained. Cancellation is the one meaning the chain must keep —
    /// ``isCancellation(_:)`` reads through `.network` causes — so it survives
    /// as a fresh, URL-free instance of the same shape; everything else is
    /// severed.
    private static func projectedDownloadCause(_ error: any Error) -> (any Error)? {
        guard isCancellation(error) else { return nil }
        if (error as? URLError)?.code == .cancelled { return URLError(.cancelled) }
        return CancellationError()
    }

    /// SPEC §9 projection of a credential-bearing URL for rendering: the origin
    /// from a successful parse, or the fixed token when no complete origin
    /// exists.
    private static func urlOriginForDisplay(_ url: String) -> String {
        guard let components = URLComponents(string: url),
              let scheme = components.scheme,
              let host = components.host, !host.isEmpty
        else { return "unparsable" }
        return "\(scheme)://\(host)"
    }

    /// Renders a download URL for hooks: origin and path only — the query is
    /// where a signed credential rides (SPEC §9).
    private static func stripQueryAndFragment(_ url: String) -> String {
        String(url.prefix(while: { $0 != "?" && $0 != "#" }))
    }

    /// Converts a backoff interval to nanoseconds without trapping.
    ///
    /// `UInt64(_:)` on an out-of-range `Double` is a runtime trap, not an
    /// error, and a hostile or simply buggy `Retry-After` can name a delay
    /// whose nanosecond product overflows `UInt64` — `Retry-After: 99999999999`
    /// is 9.9e19 ns against a 1.8e19 ceiling. Clamp to a day instead: no SDK
    /// retry is worth sleeping longer, and a crash is never the right answer
    /// to a response header.
    private static func sleepNanoseconds(_ seconds: TimeInterval) -> UInt64 {
        guard seconds.isFinite, seconds > 0 else { return 0 }
        return UInt64(min(seconds, 86_400) * 1_000_000_000)
    }

    package init(
        transport: any Transport,
        authStrategy: any AuthStrategy,
        config: BasecampConfig,
        hooks: any BasecampHooks,
        cache: ETagCache?
    ) {
        self.transport = transport
        self.authStrategy = authStrategy
        self.config = config
        self.hooks = hooks
        self.cache = cache
    }

    /// Performs an HTTP request with full lifecycle (auth, retry, cache, hooks).
    ///
    /// - Parameters:
    ///   - method: HTTP method.
    ///   - url: Full URL string.
    ///   - body: Optional request body data.
    ///   - retryConfig: Optional per-operation retry configuration.
    ///   - idempotent: Whether the operation's effect is idempotent. Retries are
    ///     gated on this so non-idempotent POSTs are attempted exactly once; the
    ///     default is `false` so a caller that omits it never over-retries.
    /// - Returns: A tuple of (data, HTTPURLResponse).
    package func performRequest(
        method: String,
        url: String,
        body: Data? = nil,
        contentType: String? = nil,
        retryConfig: RetryConfig? = nil,
        idempotent: Bool = false
    ) async throws -> (Data, HTTPURLResponse) {
        let effectiveConfig = retryConfig ?? .default

        guard let requestURL = URL(string: url) else {
            throw BasecampError.usage(message: "Invalid URL: \(url)", hint: nil)
        }

        var request = URLRequest(url: requestURL)
        request.httpMethod = method
        request.timeoutInterval = config.timeoutInterval

        // Set auth and standard headers
        try assertSameOrigin(url)
        try await authStrategy.authenticate(&request)
        request.setValue(config.userAgent, forHTTPHeaderField: "User-Agent")
        request.setValue("application/json", forHTTPHeaderField: "Accept")

        if body != nil {
            request.setValue(contentType ?? "application/json", forHTTPHeaderField: "Content-Type")
        }
        request.httpBody = body

        // ETag cache: add If-None-Match for GET requests
        if method == "GET", let cache, let etag = cache.etag(for: url) {
            request.setValue(etag, forHTTPHeaderField: "If-None-Match")
        }

        // Retry loop — gated on operation idempotency (SPEC §7). An operation is
        // retry-eligible iff its method is naturally idempotent OR the operation
        // is explicitly marked idempotent. Allowlisting the naturally-idempotent
        // methods (rather than excluding POST) keeps PATCH/OPTIONS and any future
        // method fail-closed. This single gate covers both retry paths below: the
        // status retry and the network-error retry key off `attempt < maxAttempts`.
        let retryable = Self.retryableMethods.contains(method.uppercased()) || idempotent
        let maxAttempts = max((config.enableRetry && retryable) ? effectiveConfig.maxAttempts : 1, 1)

        /// Outcome of a single attempt. Computed inside the do/catch below —
        /// which covers only the transport call and response classification —
        /// and acted on at the loop tail, so retry side effects (onRetry,
        /// backoff sleep, reauth) never run inside a catch clause.
        enum AttemptDirective {
            case done(Data, HTTPURLResponse)
            case retry(error: any Error, delaySeconds: TimeInterval)
            case fail(any Error)
        }

        for attempt in 1...maxAttempts {
            let info = RequestInfo(method: method, url: url, attempt: attempt)

            // Notify hooks
            safeInvokeHooks { $0.onRequestStart(info) }

            let startTime = CFAbsoluteTimeGetCurrent()
            let directive: AttemptDirective

            do {
                let (data, response) = try await transport.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw InvalidResponseTypeError()
                }

                let durationMs = Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)

                // Handle 304 Not Modified — return cached data with a synthetic 200 response
                // so callers don't need to special-case 304 in their status checks.
                if httpResponse.statusCode == 304, let cache, let cached = cache.data(for: url) {
                    safeInvokeHooks {
                        $0.onRequestEnd(info, result: RequestResult(
                            statusCode: 200, durationMs: durationMs, fromCache: true))
                    }
                    let syntheticResponse = HTTPURLResponse(
                        url: httpResponse.url ?? requestURL,
                        statusCode: 200,
                        httpVersion: "HTTP/1.1",
                        headerFields: httpResponse.allHeaderFields as? [String: String] ?? [:]
                    )!
                    directive = .done(cached, syntheticResponse)
                } else {
                    // Cache successful GET responses with ETag
                    if method == "GET", httpResponse.statusCode == 200,
                       let cache, let etag = httpResponse.value(forHTTPHeaderField: "ETag") {
                        cache.store(url: url, data: data, etag: etag)
                    }

                    safeInvokeHooks {
                        $0.onRequestEnd(info, result: RequestResult(
                            statusCode: httpResponse.statusCode, durationMs: durationMs))
                    }

                    // Check if we should retry
                    let statusCode = httpResponse.statusCode
                    if effectiveConfig.retryOn.contains(statusCode), attempt < maxAttempts {
                        let delaySeconds = calculateDelay(
                            attempt: attempt,
                            baseDelayMs: effectiveConfig.baseDelayMs,
                            backoff: effectiveConfig.backoff,
                            retryAfterHeader: httpResponse.value(forHTTPHeaderField: "Retry-After"),
                            statusCode: statusCode
                        )
                        let error = BasecampError.fromHTTPResponse(
                            status: statusCode, data: data,
                            headers: httpResponse.allHeaderFields as? [String: String] ?? [:],
                            requestId: httpResponse.value(forHTTPHeaderField: "X-Request-Id")
                        )
                        directive = .retry(error: error, delaySeconds: delaySeconds)
                    } else {
                        directive = .done(data, httpResponse)
                    }
                }
            } catch is InvalidResponseTypeError {
                // A deterministic programming error, not a transport blip:
                // terminal on sight, with no request-end event, exactly as it
                // behaved when the guard raised BasecampError.network directly.
                directive = .fail(Self.invalidResponseType)
            } catch let error as BasecampError where !Self.isTransportNetworkFailure(error) {
                // A non-network BasecampError thrown by the transport is its
                // own final verdict: rethrown untouched, with no request-end
                // event.
                directive = .fail(error)
            } catch {
                // Network-level error — a raw transport failure, or a Transport
                // that reports connectivity failure as BasecampError.network
                // (#567). Both mean the same thing, so both retry.
                let durationMs = Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
                safeInvokeHooks {
                    $0.onRequestEnd(info, result: RequestResult(statusCode: 0, durationMs: durationMs))
                }

                if Self.isCancellation(error) {
                    // Cooperative cancellation is terminal, not a transport
                    // blip: retrying would announce and start an attempt the
                    // caller has already abandoned. It propagates raw rather
                    // than wrapped, and the attempt is finalized above so
                    // start/end stay paired.
                    directive = .fail(error)
                } else if attempt < maxAttempts {
                    let delaySeconds = calculateDelay(
                        attempt: attempt,
                        baseDelayMs: effectiveConfig.baseDelayMs,
                        backoff: effectiveConfig.backoff,
                        retryAfterHeader: nil,
                        statusCode: nil
                    )
                    directive = .retry(error: error, delaySeconds: delaySeconds)
                } else {
                    // A transport that already speaks BasecampError keeps its
                    // own message; wrapping it in a second, vaguer .network
                    // would discard the only diagnostic the caller has.
                    directive = .fail(
                        error as? BasecampError
                            ?? BasecampError.network(message: "Network error", cause: error))
                }
            }

            // Loop tail — SPEC §7 steps 3.j (on_retry), 3.k (sleep), 3.l (refresh
            // auth), shared by the status and network retry paths. These run
            // outside any catch, so a CancellationError from the backoff sleep or
            // an error from re-authentication propagates raw — no phantom request
            // events for an attempt that already ended, no wrapping.
            switch directive {
            case .done(let data, let response):
                return (data, response)
            case .fail(let error):
                throw error
            case .retry(let error, let delaySeconds):
                safeInvokeHooks { $0.onRetry(info, attempt: attempt + 1, error: error, delaySeconds: delaySeconds) }

                try await Task.sleep(nanoseconds: Self.sleepNanoseconds(delaySeconds))

                // Re-authenticate for retry (e.g. refresh expired token)
                try await authStrategy.authenticate(&request)
            }
        }

        // Should not reach here, but just in case
        throw BasecampError.network(message: "Request failed after \(maxAttempts) attempts", cause: nil)
    }

    /// Fetches a pagination follow-up page using the same auth context.
    package func fetchPage(url: String) async throws -> (Data, HTTPURLResponse) {
        guard let requestURL = URL(string: url) else {
            throw BasecampError.usage(message: "Invalid pagination URL: \(url)", hint: nil)
        }

        var request = URLRequest(url: requestURL)
        request.httpMethod = "GET"
        request.timeoutInterval = config.timeoutInterval

        try assertSameOrigin(url)
        try await authStrategy.authenticate(&request)
        request.setValue(config.userAgent, forHTTPHeaderField: "User-Agent")
        request.setValue("application/json", forHTTPHeaderField: "Accept")

        let (data, response) = try await transport.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            // Neither of these two helpers retries, so the guard can surface
            // the caller-facing error directly. Routed through the shared
            // constant so the message stays defined in exactly one place.
            throw Self.invalidResponseType
        }

        return (data, httpResponse)
    }

    /// Authenticated GET without redirect following, under the SPEC §14 hop-1
    /// retry policy. Fires request hooks. Used by downloadURL for the first hop.
    ///
    /// The policy is fixed and passed here directly rather than looked up by
    /// operation: `DownloadURL` is deliberately absent from
    /// `behavior-model.json`, so there is no per-operation `RetryConfig` to
    /// resolve and no public numeric knob for the attempt count. Disabling
    /// retry collapses the hop to exactly one attempt.
    package func performDownloadRequest(url: String) async throws -> (Data, HTTPURLResponse) {
        guard let requestURL = URL(string: url) else {
            // SPEC §9: this URL can carry a signed credential — render its
            // origin alone, projected from a parse, or the fixed token.
            throw BasecampError.usage(message: "Invalid URL: \(Self.urlOriginForDisplay(url))", hint: nil)
        }

        var request = URLRequest(url: requestURL)
        request.httpMethod = "GET"
        request.timeoutInterval = config.timeoutInterval

        // Attempt 1's auth runs here, outside the loop, matching performRequest:
        // a failing strategy surfaces raw, before any request hook fires.
        try await authStrategy.authenticate(&request)
        request.setValue(config.userAgent, forHTTPHeaderField: "User-Agent")

        let maxAttempts = config.enableRetry ? Self.downloadMaxAttempts : 1

        // Hooks render this flow's URL as origin+path only (SPEC §9): the
        // caller's URL can smuggle a signed query through the rewrite into
        // hop 1. The wire request keeps the query; only the rendering is
        // projected.
        let hookURL = Self.stripQueryAndFragment(url)

        /// Outcome of a single attempt, computed inside the do/catch and acted
        /// on at the loop tail — the same directive shape performRequest uses,
        /// so retry side effects never run inside a catch clause.
        enum AttemptDirective {
            case done(Data, HTTPURLResponse)
            case retry(error: any Error, delaySeconds: TimeInterval)
            case fail(any Error)
        }

        for attempt in 1...maxAttempts {
            let info = RequestInfo(method: "GET", url: hookURL, attempt: attempt)
            safeInvokeHooks { $0.onRequestStart(info) }

            let startTime = CFAbsoluteTimeGetCurrent()
            let directive: AttemptDirective

            do {
                let (data, response) = try await transport.dataNoRedirect(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw InvalidResponseTypeError()
                }

                let durationMs = Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
                safeInvokeHooks {
                    $0.onRequestEnd(info, result: RequestResult(
                        statusCode: httpResponse.statusCode, durationMs: durationMs))
                }

                let statusCode = httpResponse.statusCode
                if Self.downloadRetryOn.contains(statusCode), attempt < maxAttempts {
                    let delaySeconds = calculateDelay(
                        attempt: attempt,
                        baseDelayMs: Self.defaultBaseDelayMs,
                        backoff: .exponential,
                        retryAfterHeader: httpResponse.value(forHTTPHeaderField: "Retry-After"),
                        statusCode: statusCode
                    )
                    let error = BasecampError.fromHTTPResponse(
                        status: statusCode, data: data,
                        headers: httpResponse.allHeaderFields as? [String: String] ?? [:],
                        requestId: httpResponse.value(forHTTPHeaderField: "X-Request-Id")
                    )
                    directive = .retry(error: error, delaySeconds: delaySeconds)
                } else {
                    directive = .done(data, httpResponse)
                }
            } catch is InvalidResponseTypeError {
                directive = .fail(Self.invalidResponseType)
            } catch let error as BasecampError where !Self.isTransportNetworkFailure(error) {
                directive = .fail(error)
            } catch {
                // Both loops classify identically (#567): a transport-thrown
                // BasecampError.network is a connectivity failure and retries,
                // the same as a raw URLError would. Splitting the two loops
                // here would be worse than the gap it closes.
                let durationMs = Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
                safeInvokeHooks {
                    $0.onRequestEnd(info, result: RequestResult(statusCode: 0, durationMs: durationMs))
                }

                if Self.isCancellation(error) {
                    // Cooperative cancellation is terminal, not a transport
                    // blip: retrying would announce and start an attempt the
                    // caller has already abandoned. It propagates raw rather
                    // than wrapped, and the attempt is finalized above so
                    // start/end stay paired.
                    directive = .fail(error)
                } else if attempt < maxAttempts {
                    let delaySeconds = calculateDelay(
                        attempt: attempt,
                        baseDelayMs: Self.defaultBaseDelayMs,
                        backoff: .exponential,
                        retryAfterHeader: nil,
                        statusCode: nil
                    )
                    directive = .retry(error: error, delaySeconds: delaySeconds)
                } else {
                    // SPEC §9: the transport error renders the hop-1 URL (and
                    // any signed query smuggled into it), so it is not chained
                    // raw — only its cancellation meaning is projected, and
                    // cancellation failed raw above, so the projection is nil
                    // here in practice.
                    directive = .fail(
                        error as? BasecampError
                            ?? BasecampError.network(
                                message: "Network error",
                                cause: Self.projectedDownloadCause(error)))
                }
            }

            // Loop tail, outside any catch: a CancellationError from the backoff
            // sleep or an error from re-authentication propagates raw — no
            // phantom request events for an attempt that already ended.
            switch directive {
            case .done(let data, let response):
                return (data, response)
            case .fail(let error):
                throw error
            case .retry(let error, let delaySeconds):
                safeInvokeHooks { $0.onRetry(info, attempt: attempt + 1, error: error, delaySeconds: delaySeconds) }

                try await Task.sleep(nanoseconds: Self.sleepNanoseconds(delaySeconds))

                // Re-authenticate every attempt so a rotated token is picked up.
                try await authStrategy.authenticate(&request)
            }
        }

        throw BasecampError.network(message: "Download failed after \(maxAttempts) attempts", cause: nil)
    }

    /// Unauthenticated GET via bare transport. No hooks, and no redirect
    /// following: the signed URL is the one destination the API host named,
    /// and a redirect from it is handed back for `downloadURL` to refuse (SPEC §14
    /// "Hop-2 Redirect Policy"). Used by downloadURL for the signed-URL hop.
    package func fetchSignedDownload(url: String) async throws -> (Data, HTTPURLResponse) {
        guard let requestURL = URL(string: url) else {
            // SPEC §9: the signed URL is a credential — render its origin
            // alone, projected from a parse, or the fixed token.
            throw BasecampError.usage(message: "Invalid URL: \(Self.urlOriginForDisplay(url))", hint: nil)
        }

        var request = URLRequest(url: requestURL)
        request.httpMethod = "GET"
        request.timeoutInterval = config.timeoutInterval

        do {
            // The no-redirect entry point, as on hop 1. `data(for:)` would follow
            // wherever the storage host pointed — stripping credentials on a
            // cross-origin hop, but still delivering the final body as if it
            // were the requested file (#805).
            let (data, response) = try await transport.dataNoRedirect(for: request)

            guard let httpResponse = response as? HTTPURLResponse else {
                // Neither of these two helpers retries, so the guard can surface
                // the caller-facing error directly. Routed through the shared
                // constant so the message stays defined in exactly one place.
                throw Self.invalidResponseType
            }

            return (data, httpResponse)
        } catch let error as BasecampError {
            throw error
        } catch {
            // SPEC §9: the transport error renders the signed URL, so it is
            // not chained raw — only its cancellation meaning is projected,
            // as a fresh, URL-free instance `isCancellation` still matches.
            throw BasecampError.network(
                message: "Download failed", cause: Self.projectedDownloadCause(error))
        }
    }

    // MARK: - Private

    /// Attach-point backstop: refuse to attach credentials to a foreign origin.
    /// Localhost is carved out for dev/test.
    private func assertSameOrigin(_ url: String) throws {
        if isLocalhost(url) || isSameOrigin(url, config.baseURL) {
            return
        }
        throw BasecampError.usage(
            message: "Refusing to send credentials to a different origin than the configured base URL: \(BasecampError.truncate(url))",
            hint: nil
        )
    }

    private func calculateDelay(
        attempt: Int,
        baseDelayMs: UInt64,
        backoff: RetryBackoff,
        retryAfterHeader: String?,
        statusCode: Int?
    ) -> TimeInterval {
        // For 429, respect Retry-After header
        if statusCode == 429, let retryAfter = BasecampError.parseRetryAfter(retryAfterHeader) {
            return TimeInterval(retryAfter)
        }

        let base = Self.backoffDelayMs(baseDelayMs: baseDelayMs, backoff: backoff, attempt: attempt)

        // Add jitter (0-100ms)
        let jitter = UInt64.random(in: 0...Self.maxJitterMs)
        return TimeInterval(base + jitter) / 1000.0
    }

    /// The backoff term in milliseconds for a 1-based attempt, saturating at
    /// ``maxBackoffDelayMs`` (SPEC §7, "Backoff Ceiling").
    ///
    /// The clamp is load-bearing rather than defensive, and Swift's failure
    /// without it is the worst of the six SDKs: `baseDelayMs * (1 << ...)`
    /// **traps**. `<<` on an unsigned integer is a smart shift, so an
    /// over-shift silently yields `0` — the tight retry loop against a server
    /// already answering 429/503 — but at `1 << 63` the multiply overflows
    /// `UInt64` and the process dies. `UInt64(attempt - 1)` traps on a
    /// negative operand for the same reason, so the exponent is floored, not
    /// converted.
    ///
    /// The multiplier is compared against `maxBackoffDelayMs / baseDelayMs`
    /// before multiplying, so no intermediate can leave `UInt64` range.
    static func backoffDelayMs(baseDelayMs: UInt64, backoff: RetryBackoff, attempt: Int) -> UInt64 {
        guard baseDelayMs > 0 else { return 0 }

        let multiplier: UInt64
        switch backoff {
        case .exponential:
            // 63 is the first shift that sets the sign bit's worth of
            // magnitude; at or past it the ceiling is reached regardless.
            let exponent = max(attempt - 1, 0)
            guard exponent < 63 else { return maxBackoffDelayMs }
            multiplier = 1 << UInt64(exponent)
        case .linear:
            multiplier = UInt64(max(attempt, 1))
        case .constant:
            multiplier = 1
        }

        guard multiplier <= maxBackoffDelayMs / baseDelayMs else { return maxBackoffDelayMs }
        return baseDelayMs * multiplier
    }

    private func safeInvokeHooks(_ invoke: (any BasecampHooks) -> Void) {
        invoke(hooks)
    }
}

// MARK: - Retry Configuration

/// Per-operation retry configuration, sourced from behavior-model.json.
public struct RetryConfig: Sendable {
    /// Maximum number of attempts (including the initial request).
    public let maxAttempts: Int
    /// Base delay in milliseconds between retries.
    public let baseDelayMs: UInt64
    /// Backoff strategy.
    public let backoff: RetryBackoff
    /// HTTP status codes that trigger a retry.
    public let retryOn: Set<Int>

    public init(maxAttempts: Int, baseDelayMs: UInt64, backoff: RetryBackoff, retryOn: Set<Int>) {
        self.maxAttempts = maxAttempts
        self.baseDelayMs = baseDelayMs
        self.backoff = backoff
        self.retryOn = retryOn
    }

    /// Default retry configuration: 3 attempts, exponential backoff, retry on 429/503.
    public static let `default` = RetryConfig(
        maxAttempts: 3,
        baseDelayMs: 1_000,
        backoff: .exponential,
        retryOn: [429, 503]
    )
}

/// Backoff strategy for retries.
public enum RetryBackoff: String, Sendable {
    case exponential
    case linear
    case constant
}
