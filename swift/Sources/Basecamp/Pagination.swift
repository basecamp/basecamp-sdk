import Foundation

/// Metadata about a paginated list response.
public struct ListMeta: Sendable, Equatable {
    /// Total number of items across all pages (from `X-Total-Count` header).
    public let totalCount: Int
    /// True when results were truncated (by `maxItems` or page safety cap).
    public let truncated: Bool

    public init(totalCount: Int = 0, truncated: Bool = false) {
        self.totalCount = totalCount
        self.truncated = truncated
    }
}

/// Options for controlling pagination behavior.
public struct PaginationOptions: Sendable {
    /// Maximum number of items to return across all pages.
    /// When nil or 0, all pages are fetched.
    public let maxItems: Int?

    public init(maxItems: Int? = nil) {
        self.maxItems = maxItems
    }
}

/// A paginated list result that conforms to `RandomAccessCollection`.
///
/// Acts like a Swift `Array` — supports `for-in`, `.count`, subscripting,
/// `.map()`, `.filter()`, and all other collection operations. The `.meta`
/// property provides pagination metadata.
///
/// ```swift
/// let todos = try await account.todos.list(projectId: 123, todolistId: 456)
/// print("Showing \(todos.count) of \(todos.meta.totalCount) todos")
/// for todo in todos { print(todo.content) }
/// let titles = todos.map(\.content) // returns [String]
/// ```
public struct ListResult<Element: Sendable>: Sendable {
    /// The underlying items.
    public let items: [Element]
    /// Pagination metadata.
    public let meta: ListMeta

    /// Creates a new list result.
    public init(_ items: [Element], meta: ListMeta) {
        self.items = items
        self.meta = meta
    }

    /// Creates an empty list result.
    public init() {
        self.items = []
        self.meta = ListMeta()
    }
}

// MARK: - RandomAccessCollection

extension ListResult: RandomAccessCollection {
    public typealias Index = Int

    public var startIndex: Int { items.startIndex }
    public var endIndex: Int { items.endIndex }

    public subscript(position: Int) -> Element {
        items[position]
    }
}

// MARK: - Pagination Utilities

/// Parses the next URL from a Link header.
///
/// Looks for `rel="next"` in the header value.
///
/// - Parameter linkHeader: The Link header value.
/// - Returns: The URL for the next page, or nil if not found.
func parseNextLink(_ linkHeader: String?) -> String? {
    guard let linkHeader, !linkHeader.isEmpty else { return nil }

    for part in linkHeader.split(separator: ",") {
        let trimmed = part.trimmingCharacters(in: .whitespaces)
        if trimmed.contains("rel=\"next\"") {
            // Extract URL between < and >
            guard let start = trimmed.firstIndex(of: "<"),
                  let end = trimmed.firstIndex(of: ">"),
                  start < end
            else { continue }
            return String(trimmed[trimmed.index(after: start)..<end])
        }
    }
    return nil
}

/// Resolves a possibly-relative URL against a base URL.
///
/// If target is already absolute, it is returned unchanged.
func resolveURL(base: String, target: String) -> String {
    guard let baseURL = URL(string: base) else { return target }
    guard let resolved = URL(string: target, relativeTo: baseURL) else { return target }
    return resolved.absoluteString
}

/// Checks whether two absolute URLs share the same origin (scheme + host + port).
///
/// Parses with `URL(string:)` — the SAME parser `HTTPClient` uses to dial —
/// so the guard can never disagree with the transport about which host a URL
/// targets. `URLComponents(string:)` is a distinct Foundation parser that can
/// diverge on malformed input (a parser-differential bypass).
func isSameOrigin(_ a: String, _ b: String) -> Bool {
    guard let urlA = URL(string: a),
          let urlB = URL(string: b),
          // Fail closed on scheme-less/host-less (relative) input.
          let schemeA = urlA.scheme?.lowercased(),
          let schemeB = urlB.scheme?.lowercased(),
          let hostA = urlA.host?.lowercased(),
          let hostB = urlB.host?.lowercased()
    else { return false }

    // Scheme and host are case-insensitive (RFC 3986); normalize before comparing.
    return schemeA == schemeB
        && hostA == hostB
        && (urlA.port ?? defaultPort(for: schemeA)) == (urlB.port ?? defaultPort(for: schemeB))
}

/// Parses the `X-Total-Count` header from a URL response.
func parseTotalCount(_ response: HTTPURLResponse) -> Int {
    guard let header = response.value(forHTTPHeaderField: "X-Total-Count"),
          let count = Int(header)
    else { return 0 }
    return count
}

private func defaultPort(for scheme: String?) -> Int? {
    switch scheme {
    case "https": 443
    case "http": 80
    default: nil
    }
}

/// Checks whether a URL points to localhost over HTTP(S) (for dev/test carve-out).
///
/// Parses with `URL(string:)` — the SAME parser `HTTPClient` uses to dial —
/// see `isSameOrigin` for why the guard must not use a second parser.
func isLocalhost(_ urlString: String) -> Bool {
    guard let url = URL(string: urlString),
          var host = url.host?.lowercased() else { return false }
    // The carve-out is limited to HTTP(S) so credential guards fail closed on
    // any other scheme (e.g. ws://localhost).
    switch url.scheme?.lowercased() {
    case "http", "https": break
    default: return false
    }
    // Strip IPv6 brackets if the platform's URL.host retains them.
    if host.hasPrefix("["), host.hasSuffix("]") {
        host = String(host.dropFirst().dropLast())
    }
    return host == "localhost" || host == "127.0.0.1" || host == "::1" || host.hasSuffix(".localhost")
}
