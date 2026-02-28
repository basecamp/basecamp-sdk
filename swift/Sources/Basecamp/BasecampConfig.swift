import Foundation

/// Configuration for a Basecamp SDK client.
///
/// Provides sensible defaults matching the other SDK implementations.
/// All properties are immutable after construction.
public struct BasecampConfig: Sendable {
    /// Base URL for the Basecamp API.
    public let baseURL: String

    /// User-Agent header value sent with every request.
    public let userAgent: String

    /// Whether to automatically retry on 429/503 responses.
    public let enableRetry: Bool

    /// Whether to enable ETag-based HTTP caching.
    public let enableCache: Bool

    /// Maximum number of pages to follow during pagination (safety cap).
    public let maxPages: Int

    /// Request timeout interval in seconds.
    public let timeoutInterval: TimeInterval

    /// SDK version string.
    public static let version = "0.2.1"

    /// Basecamp API version this SDK targets.
    public static let apiVersion = "2026-01-26"

    /// Default User-Agent header value.
    public static let defaultUserAgent = "basecamp-sdk-swift/\(version) (api:\(apiVersion))"

    /// Default base URL for the Basecamp API.
    public static let defaultBaseURL = "https://3.basecampapi.com"

    /// Creates a new configuration with the given options.
    ///
    /// - Parameters:
    ///   - baseURL: API base URL (default: `https://3.basecampapi.com`)
    ///   - userAgent: User-Agent header (default: `basecamp-sdk-swift/VERSION (api:API_VERSION)`)
    ///   - enableRetry: Enable automatic retry on 429/503 (default: `true`)
    ///   - enableCache: Enable ETag-based caching (default: `false`)
    ///   - maxPages: Maximum pages to follow (default: `10_000`)
    ///   - timeoutInterval: Request timeout in seconds (default: `30`)
    public init(
        baseURL: String = defaultBaseURL,
        userAgent: String = defaultUserAgent,
        enableRetry: Bool = true,
        enableCache: Bool = false,
        maxPages: Int = 10_000,
        timeoutInterval: TimeInterval = 30
    ) {
        self.baseURL = baseURL.hasSuffix("/") ? String(baseURL.dropLast()) : baseURL
        self.userAgent = userAgent
        self.enableRetry = enableRetry
        self.enableCache = enableCache
        self.maxPages = maxPages
        self.timeoutInterval = timeoutInterval
    }
}
