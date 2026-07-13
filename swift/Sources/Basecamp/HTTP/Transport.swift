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
        // The task-level delegate strips the Authorization header when a
        // redirect leaves the original request's origin — URLSession would
        // otherwise forward it, leaking the bearer token to the foreign
        // Location target. A task delegate only shadows the session delegate
        // for the callbacks it implements (here: redirects), so the caller's
        // session delegate still handles auth challenges, pinning, metrics.
        try await session.data(for: request, delegate: CredentialSanitizingRedirectDelegate())
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

/// Strips credentials from a redirect request when it leaves the origin of the
/// original request, mirroring the Go SDK's redirect policy. Factored out of
/// the delegate so the policy is unit-testable without a live URLSession.
func sanitizedRedirectRequest(_ request: URLRequest, originalURL: URL?) -> URLRequest {
    guard let target = request.url?.absoluteString,
          let original = originalURL?.absoluteString,
          isSameOrigin(target, original) else {
        var stripped = request
        stripped.setValue(nil, forHTTPHeaderField: "Authorization")
        return stripped
    }
    return request
}

/// URLSession delegate that follows redirects but strips the Authorization
/// header when the redirect target is a different origin than the original
/// request, so the bearer token never travels to a foreign host.
private final class CredentialSanitizingRedirectDelegate: NSObject, URLSessionTaskDelegate, @unchecked Sendable {
    func urlSession(
        _ session: URLSession,
        task: URLSessionTask,
        willPerformHTTPRedirection response: HTTPURLResponse,
        newRequest request: URLRequest,
        completionHandler: @escaping (URLRequest?) -> Void
    ) {
        completionHandler(sanitizedRedirectRequest(request, originalURL: task.originalRequest?.url))
    }
}

/// URLSession delegate that blocks all redirects, causing the redirect response
/// to be returned directly to the caller.
private final class RedirectBlockingDelegate: NSObject, URLSessionTaskDelegate, @unchecked Sendable {
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
