import Foundation

/// Abstraction over URL loading for testability.
///
/// Matches the iOS team's `Transporter` protocol pattern from core-ios-networking.
/// Production code uses `URLSessionTransport`; tests use `MockTransport`.
public protocol Transport: Sendable {
    /// Loads data for the given request.
    ///
    /// - Parameter request: The URL request to execute.
    /// - Returns: A tuple of the response data and URL response.
    func data(for request: URLRequest) async throws -> (Data, URLResponse)

    /// Loads data for the given request without following redirects.
    ///
    /// Used by `downloadURL` for the first hop where redirect capture is needed.
    func dataNoRedirect(for request: URLRequest) async throws -> (Data, URLResponse)
}

/// Production transport that delegates to `URLSession`.
public struct URLSessionTransport: Transport, Sendable {
    private let session: URLSession

    public init(session: URLSession = .shared) {
        self.session = session
    }

    public func data(for request: URLRequest) async throws -> (Data, URLResponse) {
        try await session.data(for: request)
    }

    public func dataNoRedirect(for request: URLRequest) async throws -> (Data, URLResponse) {
        // Create a one-shot session that inherits the caller's configuration (timeouts,
        // TLS, proxy) but blocks redirects via a delegate. Custom delegate behavior
        // (auth challenges, metrics) from the caller's session is not preserved —
        // only configuration is carried over.
        let delegate = RedirectBlockingDelegate()
        let noRedirectSession = URLSession(
            configuration: session.configuration,
            delegate: delegate,
            delegateQueue: nil
        )
        defer { noRedirectSession.finishTasksAndInvalidate() }
        return try await noRedirectSession.data(for: request)
    }
}

/// URLSession delegate that blocks all redirects, causing the redirect response
/// to be returned directly to the caller.
private final class RedirectBlockingDelegate: NSObject, URLSessionTaskDelegate, Sendable {
    func urlSession(
        _ session: URLSession,
        task: URLSessionTask,
        willPerformHTTPRedirection response: HTTPURLResponse,
        newRequest request: URLRequest,
        completionHandler: @escaping (URLRequest?) -> Void
    ) {
        // Return nil to block the redirect and return the 3xx response as-is
        completionHandler(nil)
    }
}
