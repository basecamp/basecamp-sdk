// @generated from OpenAPI spec — do not edit directly
import Foundation

public final class HillChartsService: BaseService, @unchecked Sendable {
    public func get(todosetId: Int) async throws -> HillChart {
        return try await request(
            OperationInfo(service: "HillCharts", operation: "GetHillChart", resourceType: "hill_chart", isMutation: false, resourceId: todosetId),
            method: "GET",
            path: "/todosets/\(todosetId)/hill.json",
            retryConfig: Metadata.retryConfig(for: "GetHillChart")
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
