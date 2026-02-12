import Foundation

/// Base class for all Basecamp API services.
///
/// Provides shared functionality for making API requests, handling errors,
/// automatic pagination via Link headers, and hooks integration.
///
/// Generated services inherit from this class. The `request()`, `requestVoid()`,
/// and `requestPaginated()` methods handle the full operation lifecycle.
open class BaseService: @unchecked Sendable {
    /// The account client this service is bound to.
    public let accountClient: AccountClient

    /// Creates a service bound to an account client.
    public init(accountClient: AccountClient) {
        self.accountClient = accountClient
    }

    // MARK: - Request Methods

    /// Executes an API request and decodes the response.
    ///
    /// Handles error mapping, hooks lifecycle (operation start/end),
    /// and response deserialization.
    ///
    /// - Parameters:
    ///   - info: Operation metadata for hooks.
    ///   - method: HTTP method.
    ///   - path: URL path relative to the account base (e.g., "/projects.json").
    ///   - body: Optional encodable request body.
    ///   - retryConfig: Optional per-operation retry configuration.
    /// - Returns: The decoded response.
    public func request<T: Decodable & Sendable>(
        _ info: OperationInfo,
        method: String,
        path: String,
        body: (any Encodable & Sendable)? = nil,
        contentType: String? = nil,
        retryConfig: RetryConfig? = nil
    ) async throws -> T {
        let startTime = CFAbsoluteTimeGetCurrent()

        safeInvokeHooks { $0.onOperationStart(info) }

        do {
            let bodyData: Data?
            if contentType != nil {
                bodyData = body as? Data
            } else {
                bodyData = try body.map { try Self.encoder.encode($0) }
            }
            let url = accountClient.baseURL + path

            let (data, response) = try await accountClient.httpClient.performRequest(
                method: method, url: url, body: bodyData, contentType: contentType, retryConfig: retryConfig
            )

            let durationMs = millisSince(startTime)

            guard response.statusCode >= 200 && response.statusCode < 300 else {
                throw BasecampError.fromHTTPResponse(
                    status: response.statusCode, data: data,
                    headers: response.allHeaderFields as? [String: String] ?? [:],
                    requestId: response.value(forHTTPHeaderField: "X-Request-Id")
                )
            }

            let decoded = try Self.decoder.decode(T.self, from: data)
            safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs)) }
            return decoded
        } catch {
            let durationMs = millisSince(startTime)
            safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs, error: error)) }
            throw error
        }
    }

    /// Executes an API request that returns no response body (e.g., DELETE, PUT status changes).
    ///
    /// - Parameters:
    ///   - info: Operation metadata for hooks.
    ///   - method: HTTP method.
    ///   - path: URL path relative to the account base.
    ///   - body: Optional encodable request body.
    ///   - retryConfig: Optional per-operation retry configuration.
    public func requestVoid(
        _ info: OperationInfo,
        method: String,
        path: String,
        body: (any Encodable & Sendable)? = nil,
        contentType: String? = nil,
        retryConfig: RetryConfig? = nil
    ) async throws {
        let startTime = CFAbsoluteTimeGetCurrent()

        safeInvokeHooks { $0.onOperationStart(info) }

        do {
            let bodyData: Data?
            if contentType != nil {
                bodyData = body as? Data
            } else {
                bodyData = try body.map { try Self.encoder.encode($0) }
            }
            let url = accountClient.baseURL + path

            let (data, response) = try await accountClient.httpClient.performRequest(
                method: method, url: url, body: bodyData, contentType: contentType, retryConfig: retryConfig
            )

            let durationMs = millisSince(startTime)

            guard response.statusCode >= 200 && response.statusCode < 300 else {
                throw BasecampError.fromHTTPResponse(
                    status: response.statusCode, data: data,
                    headers: response.allHeaderFields as? [String: String] ?? [:],
                    requestId: response.value(forHTTPHeaderField: "X-Request-Id")
                )
            }

            safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs)) }
        } catch {
            let durationMs = millisSince(startTime)
            safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs, error: error)) }
            throw error
        }
    }

    /// Executes a paginated API request, automatically following Link headers.
    ///
    /// Returns a `ListResult<T>` conforming to `RandomAccessCollection`.
    ///
    /// - Parameters:
    ///   - info: Operation metadata for hooks.
    ///   - path: URL path relative to the account base.
    ///   - queryItems: Optional query parameters.
    ///   - paginationOpts: Optional pagination control.
    ///   - retryConfig: Optional per-operation retry configuration.
    /// - Returns: A `ListResult` containing all items across pages.
    public func requestPaginated<T: Decodable & Sendable>(
        _ info: OperationInfo,
        path: String,
        queryItems: [URLQueryItem]? = nil,
        paginationOpts: PaginationOptions? = nil,
        retryConfig: RetryConfig? = nil
    ) async throws -> ListResult<T> {
        let startTime = CFAbsoluteTimeGetCurrent()

        safeInvokeHooks { $0.onOperationStart(info) }

        do {
            var urlString = accountClient.baseURL + path
            if let queryItems, !queryItems.isEmpty {
                var components = URLComponents(string: urlString)
                components?.queryItems = queryItems
                urlString = components?.string ?? urlString
            }

            let (data, response) = try await accountClient.httpClient.performRequest(
                method: "GET", url: urlString, retryConfig: retryConfig
            )

            guard response.statusCode >= 200 && response.statusCode < 300 else {
                throw BasecampError.fromHTTPResponse(
                    status: response.statusCode, data: data,
                    headers: response.allHeaderFields as? [String: String] ?? [:],
                    requestId: response.value(forHTTPHeaderField: "X-Request-Id")
                )
            }

            let firstPageItems = try Self.decoder.decode([T].self, from: data)
            let totalCount = parseTotalCount(response)
            let maxItems = paginationOpts?.maxItems

            // If maxItems is set and first page satisfies it, return early
            if let maxItems, maxItems > 0, firstPageItems.count >= maxItems {
                let hasMore = firstPageItems.count > maxItems
                    || parseNextLink(response.value(forHTTPHeaderField: "Link")) != nil
                let durationMs = millisSince(startTime)
                safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs)) }
                return ListResult(
                    Array(firstPageItems.prefix(maxItems)),
                    meta: ListMeta(totalCount: totalCount, truncated: hasMore)
                )
            }

            // Follow pagination
            let (allItems, truncated) = try await followPagination(
                initialURL: urlString,
                initialResponse: response,
                firstPageItems: firstPageItems,
                maxItems: maxItems
            )

            let durationMs = millisSince(startTime)
            safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs)) }
            return ListResult(allItems, meta: ListMeta(totalCount: totalCount, truncated: truncated))
        } catch {
            let durationMs = millisSince(startTime)
            safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs, error: error)) }
            throw error
        }
    }

    // MARK: - Pagination

    private func followPagination<T: Decodable>(
        initialURL: String,
        initialResponse: HTTPURLResponse,
        firstPageItems: [T],
        maxItems: Int?
    ) async throws -> (items: [T], truncated: Bool) {
        var allItems = firstPageItems
        var response = initialResponse
        let maxPages = accountClient.maxPages

        for _ in 1..<maxPages {
            guard let rawNextURL = parseNextLink(response.value(forHTTPHeaderField: "Link")) else {
                break
            }

            let nextURL = resolveURL(base: response.url?.absoluteString ?? initialURL, target: rawNextURL)

            // Validate same-origin to prevent SSRF / token leakage
            guard isSameOrigin(nextURL, initialURL) else {
                throw BasecampError.api(
                    message: "Pagination Link header points to different origin: \(nextURL)",
                    httpStatus: nil, hint: nil, requestId: nil
                )
            }

            let (data, nextResponse) = try await accountClient.httpClient.fetchPage(url: nextURL)

            guard nextResponse.statusCode >= 200 && nextResponse.statusCode < 300 else {
                throw BasecampError.fromHTTPResponse(
                    status: nextResponse.statusCode, data: data,
                    headers: nextResponse.allHeaderFields as? [String: String] ?? [:],
                    requestId: nextResponse.value(forHTTPHeaderField: "X-Request-Id")
                )
            }

            let pageItems = try Self.decoder.decode([T].self, from: data)
            allItems.append(contentsOf: pageItems)

            // Check maxItems cap
            if let maxItems, maxItems > 0, allItems.count >= maxItems {
                return (Array(allItems.prefix(maxItems)), true)
            }

            response = nextResponse
        }

        // If we hit the page cap and there's still a next link, results are truncated
        let hasMore = parseNextLink(response.value(forHTTPHeaderField: "Link")) != nil
        return (allItems, hasMore)
    }

    // MARK: - Shared Coders

    /// Shared JSON decoder configured for the Basecamp API.
    public static let decoder: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return decoder
    }()

    /// Shared JSON encoder configured for the Basecamp API.
    public static let encoder: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.keyEncodingStrategy = .convertToSnakeCase
        return encoder
    }()

    // MARK: - Helpers

    private func millisSince(_ startTime: CFAbsoluteTime) -> Int {
        Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
    }

    private func safeInvokeHooks(_ invoke: (any BasecampHooks) -> Void) {
        invoke(accountClient.hooks)
    }
}
