import Foundation

/// Controls how authentication is applied to HTTP requests.
///
/// The default strategy is ``BearerAuth``, which uses a ``TokenProvider``
/// to set the Authorization header with a Bearer token.
///
/// Custom strategies can implement alternative auth schemes such as
/// cookie-based auth, API keys, or mutual TLS.
///
/// ```swift
/// struct CookieAuth: AuthStrategy {
///     let sessionToken: String
///     func authenticate(_ request: inout URLRequest) async throws {
///         request.setValue("session=\(sessionToken)", forHTTPHeaderField: "Cookie")
///     }
/// }
/// ```
public protocol AuthStrategy: Sendable {
    /// Apply authentication to the given URL request.
    func authenticate(_ request: inout URLRequest) async throws
}

/// Bearer token authentication strategy (default).
///
/// Sets the Authorization header with "Bearer {token}" from a ``TokenProvider``.
public struct BearerAuth: AuthStrategy {
    private let tokenProvider: any TokenProvider

    /// Creates a BearerAuth strategy from a token provider.
    public init(tokenProvider: any TokenProvider) {
        self.tokenProvider = tokenProvider
    }

    public func authenticate(_ request: inout URLRequest) async throws {
        let token = try await tokenProvider.accessToken()
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
    }
}
