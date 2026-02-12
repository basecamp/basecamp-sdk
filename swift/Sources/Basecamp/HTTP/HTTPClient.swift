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
    private let tokenProvider: any TokenProvider
    private let config: BasecampConfig
    private let hooks: any BasecampHooks
    private let cache: ETagCache?

    private static let maxJitterMs: UInt64 = 100
    private static let defaultBaseDelayMs: UInt64 = 1_000

    package init(
        transport: any Transport,
        tokenProvider: any TokenProvider,
        config: BasecampConfig,
        hooks: any BasecampHooks,
        cache: ETagCache?
    ) {
        self.transport = transport
        self.tokenProvider = tokenProvider
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
    /// - Returns: A tuple of (data, HTTPURLResponse).
    package func performRequest(
        method: String,
        url: String,
        body: Data? = nil,
        contentType: String? = nil,
        retryConfig: RetryConfig? = nil
    ) async throws -> (Data, HTTPURLResponse) {
        let effectiveConfig = retryConfig ?? .default

        guard let requestURL = URL(string: url) else {
            throw BasecampError.usage(message: "Invalid URL: \(url)", hint: nil)
        }

        var request = URLRequest(url: requestURL)
        request.httpMethod = method
        request.timeoutInterval = config.timeoutInterval

        // Set auth and standard headers
        let token = try await tokenProvider.accessToken()
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
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

        // Retry loop
        let maxAttempts = config.enableRetry ? effectiveConfig.maxAttempts : 1

        for attempt in 1...maxAttempts {
            let info = RequestInfo(method: method, url: url, attempt: attempt)

            // Notify hooks
            safeInvokeHooks { $0.onRequestStart(info) }

            let startTime = CFAbsoluteTimeGetCurrent()

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
                    return (cached, syntheticResponse)
                }

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
                    safeInvokeHooks { $0.onRetry(info, attempt: attempt, error: error, delaySeconds: delaySeconds) }

                    try await Task.sleep(nanoseconds: UInt64(delaySeconds * 1_000_000_000))

                    // Refresh token for retry
                    let freshToken = try await tokenProvider.accessToken()
                    request.setValue("Bearer \(freshToken)", forHTTPHeaderField: "Authorization")
                    continue
                }

                return (data, httpResponse)

            } catch let error as BasecampError {
                throw error
            } catch {
                // Network-level error
                let durationMs = Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
                safeInvokeHooks {
                    $0.onRequestEnd(info, result: RequestResult(statusCode: 0, durationMs: durationMs))
                }

                if attempt < maxAttempts {
                    let delaySeconds = calculateDelay(
                        attempt: attempt,
                        baseDelayMs: effectiveConfig.baseDelayMs,
                        backoff: effectiveConfig.backoff,
                        retryAfterHeader: nil,
                        statusCode: nil
                    )
                    safeInvokeHooks {
                        $0.onRetry(info, attempt: attempt, error: error, delaySeconds: delaySeconds)
                    }
                    try await Task.sleep(nanoseconds: UInt64(delaySeconds * 1_000_000_000))
                    continue
                }

                throw BasecampError.network(message: "Network error", cause: error)
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

        let token = try await tokenProvider.accessToken()
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue(config.userAgent, forHTTPHeaderField: "User-Agent")
        request.setValue("application/json", forHTTPHeaderField: "Accept")

        let (data, response) = try await transport.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw BasecampError.network(message: "Invalid response type", cause: nil)
        }

        return (data, httpResponse)
    }

    // MARK: - Private

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
