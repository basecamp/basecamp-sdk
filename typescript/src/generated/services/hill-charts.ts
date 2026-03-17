/**
 * HillCharts service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for updateSettings.
 */
export interface UpdateSettingsHillChartRequest {
  /** Tracked */
  tracked?: number[];
  /** Untracked */
  untracked?: number[];
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for HillCharts operations.
 */
export class HillChartsService extends BaseService {

  /**
   * Get the hill chart for a todoset
   * @param todosetId - The todoset ID
   * @returns The hill_chart
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.hillCharts.get(123);
   * ```
   */
  async get(todosetId: number): Promise<components["schemas"]["GetHillChartResponseContent"]> {
    const response = await this.request(
      {
        service: "HillCharts",
        operation: "GetHillChart",
        resourceType: "hill_chart",
        isMutation: false,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/todosets/{todosetId}/hill.json", {
          params: {
            path: { todosetId },
          },
        })
    );
    return response;
  }

  /**
   * Track or untrack todolists on a hill chart
   * @param todosetId - The todoset ID
   * @param req - Hill_chart_setting update parameters
   * @returns The hill_chart_setting
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.hillCharts.updateSettings(123, { });
   * ```
   */
  async updateSettings(todosetId: number, req: UpdateSettingsHillChartRequest): Promise<components["schemas"]["UpdateHillChartSettingsResponseContent"]> {
    const response = await this.request(
      {
        service: "HillCharts",
        operation: "UpdateHillChartSettings",
        resourceType: "hill_chart_setting",
        isMutation: true,
        resourceId: todosetId,
      },
      () =>
        this.client.PUT("/todosets/{todosetId}/hills/settings.json", {
          params: {
            path: { todosetId },
          },
          body: {
            tracked: req.tracked,
            untracked: req.untracked,
          },
        })
    );
    return response;
  }
}