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
                    throw BasecampError.network(message: "Invalid response type", cause: nil)
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
            } catch let error as BasecampError {
                // A BasecampError thrown by the transport (or the response-type
                // guard) is rethrown untouched, with no request-end event.
                directive = .fail(error)
            } catch {
                // Network-level error
                let durationMs = Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
                safeInvokeHooks {
                    $0.onRequestEnd(info, result: RequestResult(statusCode: 0, durationMs: durationMs))
                }

                if error is CancellationError {
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
                    directive = .fail(BasecampError.network(message: "Network error", cause: error))
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
            throw BasecampError.network(message: "Invalid response type", cause: nil)
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
            throw BasecampError.usage(message: "Invalid URL: \(url)", hint: nil)
        }

        var request = URLRequest(url: requestURL)
        request.httpMethod = "GET"
        request.timeoutInterval = config.timeoutInterval

        // Attempt 1's auth runs here, outside the loop, matching performRequest:
        // a failing strategy surfaces raw, before any request hook fires.
        try await authStrategy.authenticate(&request)
        request.setValue(config.userAgent, forHTTPHeaderField: "User-Agent")

        let maxAttempts = config.enableRetry ? Self.downloadMaxAttempts : 1

        /// Outcome of a single attempt, computed inside the do/catch and acted
        /// on at the loop tail — the same directive shape performRequest uses,
        /// so retry side effects never run inside a catch clause.
        enum AttemptDirective {
            case done(Data, HTTPURLResponse)
            case retry(error: any Error, delaySeconds: TimeInterval)
            case fail(any Error)
        }

        for attempt in 1...maxAttempts {
            let info = RequestInfo(method: "GET", url: url, attempt: attempt)
            safeInvokeHooks { $0.onRequestStart(info) }

            let startTime = CFAbsoluteTimeGetCurrent()
            let directive: AttemptDirective

            do {
                let (data, response) = try await transport.dataNoRedirect(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw BasecampError.network(message: "Invalid response type", cause: nil)
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
            } catch let error as BasecampError {
                directive = .fail(error)
            } catch {
                let durationMs = Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
                safeInvokeHooks {
                    $0.onRequestEnd(info, result: RequestResult(statusCode: 0, durationMs: durationMs))
                }

                if error is CancellationError {
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
                    directive = .fail(BasecampError.network(message: "Network error", cause: error))
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

    /// Unauthenticated GET via bare transport. No hooks.
    /// Used by downloadURL for the signed-URL hop.
    package func fetchSignedDownload(url: String) async throws -> (Data, HTTPURLResponse) {
        guard let requestURL = URL(string: url) else {
            throw BasecampError.usage(message: "Invalid URL: \(url)", hint: nil)
        }

        var request = URLRequest(url: requestURL)
        request.httpMethod = "GET"
        request.timeoutInterval = config.timeoutInterval

        do {
            let (data, response) = try await transport.data(for: request)

            guard let httpResponse = response as? HTTPURLResponse else {
                throw BasecampError.network(message: "Invalid response type", cause: nil)
            }

            return (data, httpResponse)
        } catch let error as BasecampError {
            throw error
        } catch {
            throw BasecampError.network(message: "Download failed", cause: error)
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

        let base: UInt64
        switch backoff {
        case .exponential:
            base = baseDelayMs * (1 << UInt64(attempt - 1))
        case .linear:
            base = baseDelayMs * UInt64(attempt)
        case .constant:
            base = baseDelayMs
        }

        // Add jitter (0-100ms)
        let jitter = UInt64.random(in: 0...Self.maxJitterMs)
        return TimeInterval(base + jitter) / 1000.0
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
