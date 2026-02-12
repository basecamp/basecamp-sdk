import Foundation

/// A client bound to a specific Basecamp account.
///
/// Provides lazy-initialized service accessors for all API resources.
/// Created via `BasecampClient.forAccount(_:)`.
///
/// ```swift
/// let account = client.forAccount("12345")
/// let projects = try await account.projects.list()
/// let todo = try await account.todos.get(projectId: 123, todoId: 456)
/// ```
///
/// ## Extensibility
///
/// External packages can add services for internal-only endpoints:
///
/// ```swift
/// extension AccountClient {
///     public var internalDevices: InternalDevicesService {
///         service("internalDevices") { InternalDevicesService(accountClient: self) }
///     }
/// }
/// ```
public final class AccountClient: Sendable {
    /// The parent client.
    public let client: BasecampClient

    /// The account ID this client is bound to.
    public let accountId: String

    /// Base URL for account-scoped requests (e.g., "https://3.basecampapi.com/12345").
    public var baseURL: String {
        "\(client.config.baseURL)/\(accountId)"
    }

    /// The internal HTTP client (for use by services).
    package var httpClient: HTTPClient { client.httpClient }

    /// The hooks instance (for use by services).
    package var hooks: any BasecampHooks { client.hooks }

    /// Maximum pages for pagination (for use by services).
    package var maxPages: Int { client.config.maxPages }

    // MARK: - Service Cache

    private let lock = NSLock()
    // Nonisolated(unsafe) because access is serialized by NSLock
    nonisolated(unsafe) private var serviceCache: [String: Any] = [:]

    package init(client: BasecampClient, accountId: String) {
        self.client = client
        self.accountId = accountId
    }

    /// Returns a cached service instance, creating it with the factory if needed.
    ///
    /// This method is the extension point for adding services from external packages.
    /// Services are created lazily on first access and cached for the lifetime of
    /// this `AccountClient`.
    ///
    /// - Parameters:
    ///   - key: Unique string key for the service (typically the property name).
    ///   - factory: Closure that creates the service instance.
    /// - Returns: The cached or newly created service instance.
    public func service<T>(_ key: String, factory: () -> T) -> T {
        lock.withLock {
            if let existing = serviceCache[key] as? T {
                return existing
            }
            let instance = factory()
            serviceCache[key] = instance
            return instance
        }
    }

}
