/**
 * HillCharts service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";

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

/**
 * Options for listVersions.
 */
export interface ListVersionsHillChartOptions extends PaginationOptions {
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
}

/**
 * Request parameters for createVersion.
 */
export interface CreateVersionHillChartRequest {
  /** A note about the update, as rich text (HTML). */
  description?: string;
  /** Todolists to move, each with its ID and new position. Todolists left
out keep their current position. */
  dots?: components["schemas"]["HillChartDotPosition"][];
  /** People to notify of the update and subscribe to it. If absent, nobody
else is notified. */
  subscriptions?: number[];
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
   * @param req - Hill_chart update parameters
   * @returns The hill_chart
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
        resourceType: "hill_chart",
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

  /**
   * List the progress updates on a todoset's hill chart, newest first.
   * @param todosetId - The todoset ID
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.hillCharts.listVersions(123);
   *
   * // With options
   * const filtered = await client.hillCharts.listVersions(123, { page: 1 });
   * ```
   */
  async listVersions(todosetId: number, options?: ListVersionsHillChartOptions): Promise<ListResult<components["schemas"]["HillChartVersion"]>> {
    return this.requestPaginated(
      {
        service: "HillCharts",
        operation: "ListHillChartVersions",
        resourceType: "hill_chart_version",
        isMutation: false,
        resourceId: todosetId,
      },
      () =>
        this.client.GET("/todosets/{todosetId}/hills/versions.json", {
          params: {
            path: { todosetId },
            query: { page: options?.page },
          },
        })
      , options
    );
  }

  /**
   * Post a progress update on a todoset's hill chart.
   * @param todosetId - The todoset ID
   * @param req - Hill_chart_version creation parameters
   * @returns The hill_chart_version
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.hillCharts.createVersion(123, { });
   * ```
   */
  async createVersion(todosetId: number, req: CreateVersionHillChartRequest): Promise<components["schemas"]["CreateHillChartVersionResponseContent"]> {
    const response = await this.request(
      {
        service: "HillCharts",
        operation: "CreateHillChartVersion",
        resourceType: "hill_chart_version",
        isMutation: true,
        resourceId: todosetId,
      },
      () =>
        this.client.POST("/todosets/{todosetId}/hills/versions.json", {
          params: {
            path: { todosetId },
          },
          body: {
            description: req.description,
            dots: req.dots,
            subscriptions: req.subscriptions,
          },
        })
    );
    return response;
  }

  /**
   * Get one progress update from a todoset's hill chart
   * @param todosetId - The todoset ID
   * @param versionId - The version ID
   * @returns The hill_chart_version
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.hillCharts.getVersion(123, 123);
   * ```
   */
  async getVersion(todosetId: number, versionId: number): Promise<components["schemas"]["GetHillChartVersionResponseContent"]> {
    const response = await this.request(
      {
        service: "HillCharts",
        operation: "GetHillChartVersion",
        resourceType: "hill_chart_version",
        isMutation: false,
        resourceId: versionId,
      },
      () =>
        this.client.GET("/todosets/{todosetId}/hills/versions/{versionId}", {
          params: {
            path: { todosetId, versionId },
          },
        })
    );
    return response;
  }
}