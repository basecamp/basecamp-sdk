// @generated from OpenAPI spec — do not edit directly
import Foundation

public struct ListVersionsHillChartOptions: Sendable {
    /// Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8.
    public var page: Int?
    public var maxItems: Int?

    public init(page: Int? = nil, maxItems: Int? = nil) {
        self.page = page
        self.maxItems = maxItems
    }
}


public final class HillChartsService: BaseService, @unchecked Sendable {
    public func get(todosetId: Int) async throws -> HillChart {
        return try await request(
            OperationInfo(service: "HillCharts", operation: "GetHillChart", resourceType: "hill_chart", isMutation: false, resourceId: todosetId),
            method: "GET",
            path: "/todosets/\(todosetId)/hill.json",
            retryConfig: Metadata.retryConfig(for: "GetHillChart")
        )
    }

    public func getVersion(todosetId: Int, versionId: Int) async throws -> HillChartVersion {
        return try await request(
            OperationInfo(service: "HillCharts", operation: "GetHillChartVersion", resourceType: "hill_chart_version", isMutation: false, resourceId: versionId),
            method: "GET",
            path: "/todosets/\(todosetId)/hills/versions/\(versionId)",
            retryConfig: Metadata.retryConfig(for: "GetHillChartVersion")
        )
    }

    public func listVersions(todosetId: Int, options: ListVersionsHillChartOptions? = nil) async throws -> ListResult<HillChartVersion> {
        var queryItems: [URLQueryItem] = []
        if let page = options?.page {
            queryItems.append(URLQueryItem(name: "page", value: String(page)))
        }
        return try await requestPaginated(
            OperationInfo(service: "HillCharts", operation: "ListHillChartVersions", resourceType: "hill_chart_version", isMutation: false, resourceId: todosetId),
            path: "/todosets/\(todosetId)/hills/versions.json",
            queryItems: queryItems.isEmpty ? nil : queryItems,
            paginationOpts: options.flatMap { PaginationOptions(maxItems: $0.maxItems, page: $0.page) },
            retryConfig: Metadata.retryConfig(for: "ListHillChartVersions")
        )
    }

    public func updateSettings(todosetId: Int, req: UpdateHillChartSettingsRequest) async throws -> HillChart {
        return try await request(
            OperationInfo(service: "HillCharts", operation: "UpdateHillChartSettings", resourceType: "hill_chart", isMutation: true, resourceId: todosetId),
            method: "PUT",
            path: "/todosets/\(todosetId)/hills/settings.json",
            body: req,
            retryConfig: Metadata.retryConfig(for: "UpdateHillChartSettings")
        )
    }
}
