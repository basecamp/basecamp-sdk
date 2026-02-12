/// A type that provides access tokens for API authentication.
///
/// Implement this protocol to supply OAuth tokens to the SDK.
/// Tokens are requested before each API call, allowing for
/// automatic token refresh.
public protocol TokenProvider: Sendable {
    /// Returns the current access token.
    ///
    /// Called before each API request. Implementations may cache
    /// the token and refresh it when expired.
    func accessToken() async throws -> String
}

/// A token provider that returns a static token string.
///
/// Suitable for scripts, testing, and scenarios where tokens
/// don't need to be refreshed.
///
/// ```swift
/// let client = BasecampClient(
///     accessToken: "your-token",
///     userAgent: "my-app/1.0 (you@example.com)"
/// )
/// ```
public struct StaticTokenProvider: TokenProvider, Sendable {
    private let token: String

    /// Creates a static token provider with the given token.
    public init(_ token: String) {
        self.token = token
    }

    public func accessToken() async throws -> String {
        token
    }
}
