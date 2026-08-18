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
            let url = try buildURL(path)

            let (data, response) = try await accountClient.httpClient.performRequest(
                method: method, url: url, body: bodyData, contentType: contentType,
                retryConfig: retryConfig, idempotent: Metadata.isIdempotent(for: info.operation)
            )

            let durationMs = millisSince(startTime)

            guard response.statusCode >= 200 && response.statusCode < 300 else {
                throw BasecampError.fromHTTPResponse(
                    status: response.statusCode, data: data,
                    headers: response.allHeaderFields as? [String: String] ?? [:],
                    requestId: response.value(forHTTPHeaderField: "X-Request-Id")
                )
            }

            let normalizedData = Self.normalizePersonIds(in: data)
            let decoded = try Self.decoding(info.operation) {
                try Self.decoder.decode(T.self, from: normalizedData)
            }
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
            let url = try buildURL(path)

            let (data, response) = try await accountClient.httpClient.performRequest(
                method: method, url: url, body: bodyData, contentType: contentType,
                retryConfig: retryConfig, idempotent: Metadata.isIdempotent(for: info.operation)
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
            var urlString = try buildURL(path)
            if let queryItems, !queryItems.isEmpty {
                var components = URLComponents(string: urlString)
                components?.queryItems = queryItems
                urlString = components?.string ?? urlString
            }

            let (data, response) = try await accountClient.httpClient.performRequest(
                method: "GET", url: urlString, retryConfig: retryConfig,
                idempotent: Metadata.isIdempotent(for: info.operation)
            )

            guard response.statusCode >= 200 && response.statusCode < 300 else {
                throw BasecampError.fromHTTPResponse(
                    status: response.statusCode, data: data,
                    headers: response.allHeaderFields as? [String: String] ?? [:],
                    requestId: response.value(forHTTPHeaderField: "X-Request-Id")
                )
            }

            let firstPageItems = try Self.decoding(info.operation) {
                try Self.decoder.decode([T].self, from: Self.normalizePersonIds(in: data))
            }
            let totalCount = parseTotalCount(response)
            let maxItems = paginationOpts?.maxItems

            if (paginationOpts?.page ?? 0) > 0 {
                let durationMs = millisSince(startTime)
                safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs)) }
                return selectedPageResult(
                    firstPageItems, response: response, totalCount: totalCount, maxItems: maxItems)
            }

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
                info.operation,
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

    /// Executes a paginated request for wrapped responses, assembling the result
    /// from the paginated items plus the first page's remaining wrapper members.
    ///
    /// Each page returns `{ key: [items], ... }` — items are extracted from the
    /// given key, and `buildResult` decodes what else the first page's envelope
    /// carries (`person`, for the one operation that has one). It does *not*
    /// hand the raw first page data back for the caller to decode afterwards;
    /// see `buildResult` below for why that changed.
    ///
    /// - Parameters:
    ///   - info: Operation metadata for hooks.
    ///   - path: URL path relative to the account base.
    ///   - itemsKey: The JSON key containing the array of items in each response.
    ///   - queryItems: Optional query parameters.
    ///   - paginationOpts: Optional pagination control.
    ///   - retryConfig: Optional per-operation retry configuration.
    ///   - buildResult: Decodes the first page's remaining wrapper members and
    ///     combines them with the accumulated items. It runs **inside this
    ///     primitive's decode isolation**, which is why it is a parameter rather
    ///     than the caller's next statement: for as long as this function handed
    ///     back raw `Data`, the wrapper's other members were decoded by
    ///     generated code after the primitive had returned, and a malformed one
    ///     surfaced as a raw `DecodingError` instead of SPEC §6's statusless
    ///     `api_error` (#728). Threading the closure makes the isolation
    ///     structural — the wrapper body cannot be reached without passing
    ///     through `decoding(_:_:)` — so a wrapped operation added later
    ///     inherits the guarantee rather than having to remember it.
    /// - Returns: Whatever `buildResult` returns.
    public func requestPaginatedWrapped<T: Decodable & Sendable, R: Sendable>(
        _ info: OperationInfo,
        path: String,
        itemsKey: String,
        queryItems: [URLQueryItem]? = nil,
        paginationOpts: PaginationOptions? = nil,
        retryConfig: RetryConfig? = nil,
        buildResult: (Data, ListResult<T>) throws -> R
    ) async throws -> R {
        let startTime = CFAbsoluteTimeGetCurrent()

        safeInvokeHooks { $0.onOperationStart(info) }

        do {
            var urlString = try buildURL(path)
            if let queryItems, !queryItems.isEmpty {
                var components = URLComponents(string: urlString)
                components?.queryItems = queryItems
                urlString = components?.string ?? urlString
            }

            let (data, response) = try await accountClient.httpClient.performRequest(
                method: "GET", url: urlString, retryConfig: retryConfig,
                idempotent: Metadata.isIdempotent(for: info.operation)
            )

            guard response.statusCode >= 200 && response.statusCode < 300 else {
                throw BasecampError.fromHTTPResponse(
                    status: response.statusCode, data: data,
                    headers: response.allHeaderFields as? [String: String] ?? [:],
                    requestId: response.value(forHTTPHeaderField: "X-Request-Id")
                )
            }

            let firstPageData = data
            let firstPageItems: [T] = try Self.decodeWrappedItems(
                info.operation, data: data, key: itemsKey)
            let totalCount = parseTotalCount(response)
            let maxItems = paginationOpts?.maxItems

            if (paginationOpts?.page ?? 0) > 0 {
                return try finishWrapped(
                    info, startTime: startTime, wrapperData: firstPageData,
                    items: selectedPageResult(
                        firstPageItems, response: response, totalCount: totalCount,
                        maxItems: maxItems),
                    buildResult: buildResult)
            }

            // If maxItems is set and first page satisfies it, return early
            if let maxItems, maxItems > 0, firstPageItems.count >= maxItems {
                let hasMore = firstPageItems.count > maxItems
                    || parseNextLink(response.value(forHTTPHeaderField: "Link")) != nil
                return try finishWrapped(
                    info, startTime: startTime, wrapperData: firstPageData,
                    items: ListResult(
                        Array(firstPageItems.prefix(maxItems)),
                        meta: ListMeta(totalCount: totalCount, truncated: hasMore)
                    ),
                    buildResult: buildResult)
            }

            // Follow pagination
            let (allItems, truncated) = try await followWrappedPagination(
                info.operation,
                initialURL: urlString,
                initialResponse: response,
                firstPageItems: firstPageItems,
                itemsKey: itemsKey,
                maxItems: maxItems
            )

            return try finishWrapped(
                info, startTime: startTime, wrapperData: firstPageData,
                items: ListResult(
                    allItems, meta: ListMeta(totalCount: totalCount, truncated: truncated)),
                buildResult: buildResult)
        } catch {
            let durationMs = millisSince(startTime)
            safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs, error: error)) }
            throw error
        }
    }

    /// The single exit of `requestPaginatedWrapped`: decode the wrapper, then
    /// report the operation as ended.
    ///
    /// Ordering is the reason this is a function rather than three repetitions.
    /// The wrapper decode has to happen BEFORE `onOperationEnd` fires, or a
    /// malformed wrapper would be reported to hooks as a success and then thrown
    /// — which is exactly what happened while generated code did this decode
    /// after the primitive had returned. Raising here instead leaves the throw
    /// inside the caller's `do`, so the existing `catch` reports the operation as
    /// failed, once, with the error.
    private func finishWrapped<T: Decodable & Sendable, R: Sendable>(
        _ info: OperationInfo,
        startTime: CFAbsoluteTime,
        wrapperData: Data,
        items: ListResult<T>,
        buildResult: (Data, ListResult<T>) throws -> R
    ) throws -> R {
        let result = try Self.decoding(info.operation) { try buildResult(wrapperData, items) }
        let durationMs = millisSince(startTime)
        safeInvokeHooks { $0.onOperationEnd(info, result: OperationResult(durationMs: durationMs)) }
        return result
    }

    /// A pinned page is the whole answer (SPEC section 8): the caller gets
    /// exactly that page in exactly one request, and the `Link: rel="next"`
    /// follow loop never runs. `truncated` still reports whether more items
    /// existed — dropped by the cap, or reachable through the link we
    /// deliberately did not follow.
    private func selectedPageResult<T: Decodable & Sendable>(
        _ items: [T],
        response: HTTPURLResponse,
        totalCount: Int,
        maxItems: Int?
    ) -> ListResult<T> {
        let cap = maxItems.flatMap { $0 > 0 && items.count > $0 ? $0 : nil }
        let truncated = cap != nil
            || parseNextLink(response.value(forHTTPHeaderField: "Link")) != nil
        return ListResult(
            cap.map { Array(items.prefix($0)) } ?? items,
            meta: ListMeta(totalCount: totalCount, truncated: truncated)
        )
    }

    // MARK: - Pagination

    private func followPagination<T: Decodable>(
        _ operation: String,
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
                    httpStatus: nil, hint: nil, requestId: nil, decodeFailure: nil
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

            let pageItems = try Self.decoding(operation) {
                try Self.decoder.decode([T].self, from: Self.normalizePersonIds(in: data))
            }
            allItems.append(contentsOf: pageItems)

            // Check maxItems cap: truncated only when items were dropped or the
            // just-fetched page (nextResponse, not the stale loop variable) has a next Link
            if let maxItems, maxItems > 0, allItems.count >= maxItems {
                let hasMore = allItems.count > maxItems
                    || parseNextLink(nextResponse.value(forHTTPHeaderField: "Link")) != nil
                return (Array(allItems.prefix(maxItems)), hasMore)
            }

            response = nextResponse
        }

        // If we hit the page cap and there's still a next link, results are truncated
        let hasMore = parseNextLink(response.value(forHTTPHeaderField: "Link")) != nil
        return (allItems, hasMore)
    }

    private func followWrappedPagination<T: Decodable>(
        _ operation: String,
        initialURL: String,
        initialResponse: HTTPURLResponse,
        firstPageItems: [T],
        itemsKey: String,
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
                    httpStatus: nil, hint: nil, requestId: nil, decodeFailure: nil
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

            let pageItems: [T] = try Self.decodeWrappedItems(operation, data: data, key: itemsKey)
            allItems.append(contentsOf: pageItems)

            // Check maxItems cap: truncated only when items were dropped or the
            // just-fetched page (nextResponse, not the stale loop variable) has a next Link
            if let maxItems, maxItems > 0, allItems.count >= maxItems {
                let hasMore = allItems.count > maxItems
                    || parseNextLink(nextResponse.value(forHTTPHeaderField: "Link")) != nil
                return (Array(allItems.prefix(maxItems)), hasMore)
            }

            response = nextResponse
        }

        // If we hit the page cap and there's still a next link, results are truncated
        let hasMore = parseNextLink(response.value(forHTTPHeaderField: "Link")) != nil
        return (allItems, hasMore)
    }

    /// A wrapper member named at runtime, so a refusal below can report the key
    /// it was looking for the way `Decodable` would have.
    private struct WrapperKey: CodingKey {
        let stringValue: String
        var intValue: Int? { nil }
        init(_ stringValue: String) { self.stringValue = stringValue }
        init?(stringValue: String) { self.init(stringValue) }
        init?(intValue: Int) { nil }
    }

    /// Decodes items from a wrapped JSON response by extracting the array at the given key.
    ///
    /// All four steps are the decode of this response and nothing else, so the
    /// whole body is the decode expression `decoding(_:_:)` isolates.
    ///
    /// **Every shape but the right one is refused.** Two of the three guards
    /// used to be `?? [:]` and `else { return [] }`, and the difference between
    /// them is worth keeping: an absent key made this helper return an empty
    /// list *and the operation succeed*, because the wrapper decode that ran
    /// next was happy with the rest of the body — a successful read of a
    /// response the SDK had not understood, where Kotlin threw. A non-object
    /// body also reached that fallback, but the wrapper decode then rejected the
    /// same body, so the operation failed with an unmapped `DecodingError`
    /// rather than succeeding (#728). One was a wrong answer, the other a raw
    /// error; both are the statusless `api_error` now. The third,
    /// on the member's type, is new for a different reason: only a dictionary or
    /// an array is valid input to `JSONSerialization.data(withJSONObject:)`, and
    /// a string or number at this key made it answer with an
    /// `NSInvalidArgumentException` — not a Swift error, and not catchable. BC3 settles
    /// which reading is right: the wrapped-pagination envelope is rendered by
    /// `app/views/api/users/timelines/show.json.jbuilder`, two unconditional
    /// `json.` lines, so an absent member is a malformed body and never an empty
    /// result. `DecodingError` is the type the guards raise because it is what
    /// `decoding(_:_:)` maps to SPEC §6's statusless `api_error` — the same
    /// answer the typed decoder on the next line would give.
    private static func decodeWrappedItems<T: Decodable>(
        _ operation: String, data: Data, key: String
    ) throws -> [T] {
        try decoding(operation) {
            guard let json = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
                throw DecodingError.typeMismatch(
                    [String: Any].self,
                    DecodingError.Context(
                        codingPath: [],
                        debugDescription: "response wrapper is not a JSON object"))
            }
            guard let member = json[key] else {
                throw DecodingError.keyNotFound(
                    WrapperKey(key),
                    DecodingError.Context(
                        codingPath: [],
                        debugDescription: "required member '\(key)' is absent from the response wrapper"))
            }
            guard let itemsArray = member as? [Any] else {
                throw DecodingError.typeMismatch(
                    [Any].self,
                    DecodingError.Context(
                        codingPath: [WrapperKey(key)],
                        debugDescription: "member '\(key)' is not an array"))
            }
            let itemsData = try JSONSerialization.data(withJSONObject: itemsArray)
            return try decoder.decode([T].self, from: Self.normalizePersonIds(in: itemsData))
        }
    }

    // MARK: - Decode Isolation

    /// Runs the response decoder — and nothing else — mapping a decode failure
    /// into the SPEC §6 statusless `api_error` shape.
    ///
    /// Statusless because the request succeeded, so no HTTP status describes the
    /// failure; non-retryable because re-requesting cannot repair a malformed
    /// body (`isRetryable` reads `false` off the nil status). The underlying
    /// error goes in two places: interpolated into the message, for a human
    /// reading a log, and structurally into ``BasecampError/decodeFailure``, for
    /// code that has to tell this failure from any other statusless `.api`
    /// (#750). This is the one producer of that slot besides the §18 composites'
    /// restatement of the same failure.
    ///
    /// **Wrap the decode expression, never the block.** Each primitive above
    /// runs encode → URL build → auth → transport → status check → decode inside
    /// one `do` whose `catch` maps nothing, which is why a malformed 2xx body
    /// used to surface as a raw `DecodingError`, indistinguishable from the auth
    /// strategy throwing or the socket dropping. Only the decode call is
    /// wrapped: `try Self.encoder.encode(body)` runs inside the same `do`, and
    /// wrapping the block would put the *request* body's encoding inside the
    /// decoder's error mapping — the same conflation in a new shape.
    static func decoding<T>(_ operation: String, _ decode: () throws -> T) throws -> T {
        do {
            return try decode()
        } catch let error as DecodingError {
            throw malformedBody(operation, error)
        } catch let error as CocoaError {
            // A body that is not JSON at all reaches `JSONSerialization` on the
            // wrapped-list path (the typed decoder never sees it), and that
            // reports a `CocoaError`, not a `DecodingError`. Same failure, same
            // shape. Nothing else in this closure can raise one — it is only
            // ever a decode.
            throw malformedBody(operation, error)
        }
    }

    /// The one place a decode failure is rendered.
    ///
    /// The message still says "returned a body that does not decode", but that
    /// wording is no longer a contract: it used to be read back by a
    /// `malformedBodyMessage(_:)` substring test that the composites and the
    /// conformance runner asked "is this a malformed body?", so rewording it
    /// moved the answer and a caller composing their own `.api` around the
    /// phrase forged one. The answer is ``BasecampError/decodeFailure`` now
    /// (#750), which is also what Kotlin's `BasecampException.Api.decodeFailure`
    /// has answered since #730.
    private static func malformedBody(_ operation: String, _ error: any Error & Sendable) -> BasecampError {
        .api(
            message: BasecampError.truncate(
                "\(operation) returned a body that does not decode: \(error)"),
            httpStatus: nil,
            hint: nil,
            requestId: nil,
            decodeFailure: error
        )
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

    /// Builds the full request URL for an account-relative path. Absolute URLs
    /// must target the configured origin (localhost carve-out) so the bearer
    /// token is never attached to a foreign host.
    private func buildURL(_ path: String) throws -> String {
        // Scheme detection is case-insensitive (RFC 3986) while the original
        // string is preserved for the request.
        let lowercased = path.lowercased()
        let isHTTP = lowercased.hasPrefix("http://")
        let isHTTPS = lowercased.hasPrefix("https://")
        if isHTTP || isHTTPS {
            // Localhost is carved out for local development and may use plain
            // HTTP; every other origin must be same-origin and use HTTPS so the
            // bearer token is never attached to a foreign host.
            if isLocalhost(path) {
                return path
            }
            if isHTTP {
                throw BasecampError.usage(message: "Request URL must use HTTPS: \(BasecampError.truncate(path))", hint: nil)
            }
            if isSameOrigin(path, accountClient.baseURL) {
                return path
            }
            throw BasecampError.usage(
                message: "Request URL points to a different origin than the configured base URL: \(BasecampError.truncate(path))",
                hint: nil
            )
        }
        return accountClient.baseURL + path
    }

    private func millisSince(_ startTime: CFAbsoluteTime) -> Int {
        Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
    }

    private func safeInvokeHooks(_ invoke: (any BasecampHooks) -> Void) {
        invoke(accountClient.hooks)
    }

    /// Normalizes Person-shaped objects in raw JSON data.
    ///
    /// The BC3 API conflates real Person records (numeric id) with system
    /// actors like LocalPerson (symbolic id: "basecamp", "campfire").
    /// For objects with `personable_type` and a string `id`:
    /// - Numeric strings: coerced to Int, no system_label
    /// - Numeric overflow: left as string for FlexibleInt to reject
    /// - Non-numeric sentinels: id becomes 0, original preserved as system_label
    static func normalizePersonIds(in data: Data) -> Data {
        // Short-circuit: skip parsing if no Person-shaped objects
        guard data.range(of: Data("personable_type".utf8)) != nil else { return data }

        guard let json = try? JSONSerialization.jsonObject(with: data),
              let mutable = deepMutableCopy(json) else {
            return data
        }
        normalizeWalk(mutable)
        guard let result = try? JSONSerialization.data(withJSONObject: mutable) else {
            return data
        }
        return result
    }

    private static func normalizeWalk(_ obj: Any) {
        if let dict = obj as? NSMutableDictionary {
            if dict["personable_type"] != nil, let idStr = dict["id"] as? String {
                if let n = Int(idStr) {
                    dict["id"] = n
                } else if idStr.range(of: #"^-?\d+$"#, options: .regularExpression) != nil {
                    // Numeric overflow — leave as string, FlexibleInt will reject
                } else {
                    // Non-numeric sentinel
                    dict["system_label"] = idStr
                    dict["id"] = 0
                }
            }
            for (_, val) in dict {
                normalizeWalk(val)
            }
        } else if let arr = obj as? NSMutableArray {
            for item in arr {
                normalizeWalk(item)
            }
        }
    }

    private static func deepMutableCopy(_ obj: Any) -> Any? {
        if let dict = obj as? [String: Any] {
            let mutable = NSMutableDictionary()
            for (k, v) in dict {
                if let child = deepMutableCopy(v) {
                    mutable[k] = child
                }
            }
            return mutable
        } else if let arr = obj as? [Any] {
            let mutable = NSMutableArray()
            for item in arr {
                if let child = deepMutableCopy(item) {
                    mutable.add(child)
                }
            }
            return mutable
        }
        return obj
    }
}
