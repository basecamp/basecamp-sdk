import Foundation

/// Information about a high-level service operation.
public struct OperationInfo: Sendable {
    /// Service name (e.g., "Todos", "Projects").
    public let service: String
    /// Operation name (e.g., "List", "Get", "Create").
    public let operation: String
    /// Type of resource being accessed.
    public let resourceType: String
    /// Whether this operation modifies data.
    public let isMutation: Bool
    /// Project ID if the operation is scoped to a project.
    public let projectId: Int?
    /// Resource ID if the operation targets a specific resource.
    public let resourceId: Int?

    public init(
        service: String,
        operation: String,
        resourceType: String,
        isMutation: Bool,
        projectId: Int? = nil,
        resourceId: Int? = nil
    ) {
        self.service = service
        self.operation = operation
        self.resourceType = resourceType
        self.isMutation = isMutation
        self.projectId = projectId
        self.resourceId = resourceId
    }
}

/// Result of a service operation.
public struct OperationResult: Sendable {
    /// Operation duration in milliseconds.
    public let durationMs: Int
    /// Error if the operation failed.
    public let error: (any Error)?

    public init(durationMs: Int, error: (any Error)? = nil) {
        self.durationMs = durationMs
        self.error = error
    }
}

/// Information about an HTTP request.
public struct RequestInfo: Sendable {
    /// HTTP method.
    public let method: String
    /// Full request URL.
    public let url: String
    /// Current attempt number (1-based).
    public let attempt: Int

    public init(method: String, url: String, attempt: Int) {
        self.method = method
        self.url = url
        self.attempt = attempt
    }
}

/// Result of an HTTP request.
public struct RequestResult: Sendable {
    /// HTTP status code.
    public let statusCode: Int
    /// Request duration in milliseconds.
    public let durationMs: Int
    /// Whether the response was served from cache.
    public let fromCache: Bool

    public init(statusCode: Int, durationMs: Int, fromCache: Bool = false) {
        self.statusCode = statusCode
        self.durationMs = durationMs
        self.fromCache = fromCache
    }
}

/// Hooks for observing SDK operations and HTTP requests.
///
/// All methods have default no-op implementations.
/// Implement only what you need for logging, metrics, or tracing.
///
/// ```swift
/// struct LoggingHooks: BasecampHooks {
///     func onOperationEnd(_ info: OperationInfo, result: OperationResult) {
///         print("\(info.service).\(info.operation) completed in \(result.durationMs)ms")
///     }
/// }
/// ```
public protocol BasecampHooks: Sendable {
    /// Called when a service operation starts.
    func onOperationStart(_ info: OperationInfo)

    /// Called when a service operation completes (success or failure).
    func onOperationEnd(_ info: OperationInfo, result: OperationResult)

    /// Called when an HTTP request starts (called for each attempt including retries).
    func onRequestStart(_ info: RequestInfo)

    /// Called when an HTTP request completes (called for each attempt including retries).
    func onRequestEnd(_ info: RequestInfo, result: RequestResult)

    /// Called before a retry attempt.
    ///
    /// - Parameters:
    ///   - info: Request information.
    ///   - attempt: The attempt number (1-based).
    ///   - error: The error that triggered the retry.
    ///   - delaySeconds: The delay before the retry, in seconds.
    func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval)
}

// Default no-op implementations.
extension BasecampHooks {
    public func onOperationStart(_ info: OperationInfo) {}
    public func onOperationEnd(_ info: OperationInfo, result: OperationResult) {}
    public func onRequestStart(_ info: RequestInfo) {}
    public func onRequestEnd(_ info: RequestInfo, result: RequestResult) {}
    public func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval) {}
}

/// A no-op hooks implementation for zero overhead when hooks are not needed.
public struct NoopHooks: BasecampHooks, Sendable {
    public init() {}
}

/// Composes multiple ``BasecampHooks`` implementations into one.
///
/// Start events are called in order; end events are called in reverse order.
///
/// ```swift
/// let client = BasecampClient(
///     accessToken: "token",
///     userAgent: "app/1.0",
///     hooks: ChainHooks(LoggingHooks(), MetricsHooks())
/// )
/// ```
public struct ChainHooks: BasecampHooks {
    private let hooks: [any BasecampHooks]

    public init(_ hooks: any BasecampHooks...) {
        self.hooks = hooks
    }

    public init(_ hooks: [any BasecampHooks]) {
        self.hooks = hooks
    }

    public func onOperationStart(_ info: OperationInfo) {
        for hook in hooks { hook.onOperationStart(info) }
    }

    public func onOperationEnd(_ info: OperationInfo, result: OperationResult) {
        for hook in hooks.reversed() { hook.onOperationEnd(info, result: result) }
    }

    public func onRequestStart(_ info: RequestInfo) {
        for hook in hooks { hook.onRequestStart(info) }
    }

    public func onRequestEnd(_ info: RequestInfo, result: RequestResult) {
        for hook in hooks.reversed() { hook.onRequestEnd(info, result: result) }
    }

    public func onRetry(_ info: RequestInfo, attempt: Int, error: any Error, delaySeconds: TimeInterval) {
        for hook in hooks { hook.onRetry(info, attempt: attempt, error: error, delaySeconds: delaySeconds) }
    }
}
