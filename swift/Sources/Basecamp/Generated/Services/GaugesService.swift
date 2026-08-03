// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListGaugeNeedlesGaugeOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}

public struct ListGaugesGaugeOptions: Sendable {
    /// Comma-separated list of project IDs. When provided, results are returned in the order specified instead of by risk level.
    public var bucketIds: String?
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(bucketIds: String? = nil, page: Int? = nil, maxItems: Int? = nil) {
        self.bucketIds = bucketIds
        self.page = page
        self.maxItems = maxItems
    }
}


public final class GaugesService: BaseService, @unchecked Sendable {
    public func createGaugeNeedle(projectId: Int, req: CreateGaugeNeedleRequest) async throws -> GaugeNeedle {
        return try await request(
            OperationInfo(service: "Gauges", operation: "CreateGaugeNeedle", resourceType: "gauge_needle", isMutation: true, projectId: projectId),
            method: "POST",
            path: "/projects/\(projectId)/gauge/needles.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "CreateGaugeNeedle")
        )
    }

    public func destroyGaugeNeedle(needleId: Int) async throws {
        try await requestVoid(
            OperationInfo(service: "Gauges", operation: "DestroyGaugeNeedle", resourceType: "resource", isMutation: true, resourceId: needleId),
            method: "DELETE",
            path: "/gauge_needles/\(needleId)",
            retryConfig: Metadata.retryConfig(for: "DestroyGaugeNeedle")
        )
    }

    public func gaugeNeedle(needleId: Int) async throws -> GaugeNeedle {
        return try await request(
            OperationInfo(service: "Gauges", operation: "GetGaugeNeedle", resourceType: "gauge_needle", isMutation: false, resourceId: needleId),
            method: "GET",
            path: "/gauge_needles/\(needleId)",
            retryConfig: Metadata.retryConfig(for: "GetGaugeNeedle")
        )
    }

    public func listGaugeNeedles(projectId: Int, options: ListGaugeNeedlesGaugeOptions? = nil) async throws -> ListResult<GaugeNeedle> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Gauges", operation: "ListGaugeNeedles", resourceType: "gauge_needle", isMutation: false, projectId: projectId),
            path: "/projects/\(projectId)/gauge/needles.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListGaugeNeedles")
        )
    }

    public func listGauges(options: ListGaugesGaugeOptions? = nil) async throws -> ListResult<Gauge> {
        var queryItems: [URLQueryItem] = []
        if let bucketIds = options?.bucketIds {
            queryItems.append(URLQueryItem(name: "bucket_ids", value: bucketIds))
        }
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "Gauges", operation: "ListGauges", resourceType: "gauge", isMutation: false),
            path: "/reports/gauges.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListGauges")
        )
    }

    public func toggleGauge(projectId: Int, req: ToggleGaugeRequest) async throws {
        try await requestVoid(
            OperationInfo(service: "Gauges", operation: "ToggleGauge", resourceType: "resource", isMutation: true, projectId: projectId),
            method: "PUT",
            path: "/projects/\(projectId)/gauge.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "ToggleGauge")
        )
    }

    public func updateGaugeNeedle(needleId: Int, req: UpdateGaugeNeedleRequest) async throws -> GaugeNeedle {
        return try await request(
            OperationInfo(service: "Gauges", operation: "UpdateGaugeNeedle", resourceType: "gauge_needle", isMutation: true, resourceId: needleId),
            method: "PUT",
            path: "/gauge_needles/\(needleId)",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateGaugeNeedle")
        )
    }
}
