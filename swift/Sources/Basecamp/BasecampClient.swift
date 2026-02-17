import Foundation

/// The main entry point for the Basecamp SDK.
///
/// Creates an HTTP client configured with authentication, retry,
/// caching, and hooks. Use `forAccount(_:)` to get an `AccountClient`
/// bound to a specific Basecamp account.
///
/// ```swift
/// let client = BasecampClient(
///     accessToken: "your-token",
///     userAgent: "my-app/1.0 (you@example.com)"
/// )
///
/// let account = client.forAccount("12345")
/// let projects = try await account.projects.list()
/// ```
public final class BasecampClient: Sendable {
    /// The client configuration.
    public let config: BasecampConfig

    /// The hooks for observability.
    public let hooks: any BasecampHooks

    /// The internal HTTP client used by all services.
    package let httpClient: HTTPClient

    /// Creates a client with a static access token.
    ///
    /// - Parameters:
    ///   - accessToken: OAuth access token string.
    ///   - userAgent: Required User-Agent identifying your app (e.g., "MyApp/1.0 (you@example.com)").
    ///   - config: Configuration options (defaults are sensible for most uses).
    ///   - hooks: Optional observability hooks.
    public convenience init(
        accessToken: String,
        userAgent: String,
        config: BasecampConfig = BasecampConfig(),
        hooks: (any BasecampHooks)? = nil
    ) {
        self.init(
            tokenProvider: StaticTokenProvider(accessToken),
            userAgent: userAgent,
            config: config,
            hooks: hooks
        )
    }

    /// Creates a client with a custom token provider.
    ///
    /// Use this initializer for token refresh scenarios.
    ///
    /// - Parameters:
    ///   - tokenProvider: A provider that returns access tokens.
    ///   - userAgent: Required User-Agent identifying your app.
    ///   - config: Configuration options.
    ///   - hooks: Optional observability hooks.
    ///   - transport: Optional custom transport (for testing).
    public convenience init(
        tokenProvider: any TokenProvider,
        userAgent: String,
        config: BasecampConfig = BasecampConfig(),
        hooks: (any BasecampHooks)? = nil,
        transport: (any Transport)? = nil
    ) {
        self.init(
            auth: BearerAuth(tokenProvider: tokenProvider),
            userAgent: userAgent,
            config: config,
            hooks: hooks,
            transport: transport
        )
    }

    /// Creates a client with a custom authentication strategy.
    ///
    /// Use this initializer for non-Bearer authentication schemes
    /// such as cookie-based auth, API keys, or mutual TLS.
    ///
    /// - Parameters:
    ///   - auth: An authentication strategy applied to every request.
    ///   - userAgent: Required User-Agent identifying your app.
    ///   - config: Configuration options.
    ///   - hooks: Optional observability hooks.
    ///   - transport: Optional custom transport (for testing).
    public init(
        auth: any AuthStrategy,
        userAgent: String,
        config: BasecampConfig = BasecampConfig(),
        hooks: (any BasecampHooks)? = nil,
        transport: (any Transport)? = nil
    ) {
        let effectiveConfig = BasecampConfig(
            baseURL: config.baseURL,
            userAgent: userAgent,
            enableRetry: config.enableRetry,
            enableCache: config.enableCache,
            maxPages: config.maxPages,
            timeoutInterval: config.timeoutInterval
        )
        let effectiveHooks = hooks ?? NoopHooks()
        let effectiveTransport = transport ?? URLSessionTransport()
        let cache = config.enableCache ? ETagCache() : nil

        // Validate base URL uses HTTPS (skip for localhost in tests)
        if let url = URL(string: effectiveConfig.baseURL) {
            let host = url.host ?? ""
            let isLocalhost = host == "localhost" || host == "127.0.0.1" || host == "::1"
            if url.scheme != "https" && !isLocalhost {
                preconditionFailure("Base URL must use HTTPS: \(effectiveConfig.baseURL)")
            }
        }

        self.config = effectiveConfig
        self.hooks = effectiveHooks
        self.httpClient = HTTPClient(
            transport: effectiveTransport,
            authStrategy: auth,
            config: effectiveConfig,
            hooks: effectiveHooks,
            cache: cache
        )
    }

    /// Returns an `AccountClient` bound to the specified Basecamp account.
    ///
    /// The account ID must be a numeric string (e.g., "12345"). This matches
    /// the account ID in your Basecamp URL.
    ///
    /// ```swift
    /// let account = client.forAccount("12345")
    /// let projects = try await account.projects.list()
    /// ```
    ///
    /// - Parameter accountId: The Basecamp account ID.
    /// - Returns: An `AccountClient` for the specified account.
    public func forAccount(_ accountId: String) -> AccountClient {
        precondition(!accountId.isEmpty, "Account ID must not be empty")
        precondition(accountId.allSatisfy(\.isNumber), "Account ID must be numeric, got: \(accountId)")

        return AccountClient(client: self, accountId: accountId)
    }
}
