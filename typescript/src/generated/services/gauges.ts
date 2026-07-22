/**
 * Gauges service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";
import { ListResult } from "../../pagination.js";
import type { PaginationOptions } from "../../pagination.js";
import { Errors } from "../../errors.js";

// =============================================================================
// Types
// =============================================================================


/**
 * Request parameters for updateGaugeNeedle.
 */
export interface UpdateGaugeNeedleGaugeRequest {
  /** Gauge needle */
  gaugeNeedle?: components["schemas"]["GaugeNeedleUpdatePayload"];
}

/**
 * Request parameters for toggleGauge.
 */
export interface ToggleGaugeGaugeRequest {
  /** Gauge */
  gauge: components["schemas"]["GaugeTogglePayload"];
}

/**
 * Options for listGaugeNeedles.
 */
export interface ListGaugeNeedlesGaugeOptions extends PaginationOptions {
}

/**
 * Request parameters for createGaugeNeedle.
 */
export interface CreateGaugeNeedleGaugeRequest {
  /** Gauge needle */
  gaugeNeedle: components["schemas"]["GaugeNeedlePayload"];
  /** Who to notify: "everyone", "working_on", "custom", or omit for nobody */
  notify?: string;
  /** Array of people IDs to notify (only used when notify is "custom") */
  subscriptions?: number[];
}

/**
 * Options for listGauges.
 */
export interface ListGaugesGaugeOptions extends PaginationOptions {
  /** Comma-separated list of project IDs. When provided, results are returned
in the order specified instead of by risk level. */
  bucketIds?: string;
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for Gauges operations.
 */
export class GaugesService extends BaseService {

  /**
   * Get a gauge needle by ID
   * @param needleId - The needle ID
   * @returns The gauge_needle
   *
   * @example
   * ```ts
   * const result = await client.gauges.gaugeNeedle(123);
   * ```
   */
  async gaugeNeedle(needleId: number): Promise<components["schemas"]["GetGaugeNeedleResponseContent"]> {
    const response = await this.request(
      {
        service: "Gauges",
        operation: "GetGaugeNeedle",
        resourceType: "gauge_needle",
        isMutation: false,
        resourceId: needleId,
      },
      () =>
        this.client.GET("/gauge_needles/{needleId}", {
          params: {
            path: { needleId },
          },
        })
    );
    return response;
  }

  /**
   * Update a gauge needle's description. Position and color are immutable.
   * @param needleId - The needle ID
   * @param req - Gauge_needle update parameters
   * @returns The gauge_needle
   * @throws {BasecampError} If the resource is not found or fields are invalid
   *
   * @example
   * ```ts
   * const result = await client.gauges.updateGaugeNeedle(123, { });
   * ```
   */
  async updateGaugeNeedle(needleId: number, req: UpdateGaugeNeedleGaugeRequest): Promise<components["schemas"]["UpdateGaugeNeedleResponseContent"]> {
    const response = await this.request(
      {
        service: "Gauges",
        operation: "UpdateGaugeNeedle",
        resourceType: "gauge_needle",
        isMutation: true,
        resourceId: needleId,
      },
      () =>
        this.client.PUT("/gauge_needles/{needleId}", {
          params: {
            path: { needleId },
          },
          body: {
            gauge_needle: req.gaugeNeedle,
          },
        })
    );
    return response;
  }

  /**
   * Destroy a gauge needle
   * @param needleId - The needle ID
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.gauges.destroyGaugeNeedle(123);
   * ```
   */
  async destroyGaugeNeedle(needleId: number): Promise<void> {
    await this.request(
      {
        service: "Gauges",
        operation: "DestroyGaugeNeedle",
        resourceType: "resource",
        isMutation: true,
        resourceId: needleId,
      },
      () =>
        this.client.DELETE("/gauge_needles/{needleId}", {
          params: {
            path: { needleId },
          },
        })
    );
  }

  /**
   * Enable or disable the gauge for a project. Only project admins can toggle gauges.
   * @param projectId - The project ID
   * @param req - Resource request parameters
   * @returns void
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.gauges.toggleGauge(123, { gauge: { enabled: true } });
   * ```
   */
  async toggleGauge(projectId: number, req: ToggleGaugeGaugeRequest): Promise<void> {
    if (!req.gauge) {
      throw Errors.validation("Gauge is required");
    }
    await this.request(
      {
        service: "Gauges",
        operation: "ToggleGauge",
        resourceType: "resource",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.PUT("/projects/{projectId}/gauge.json", {
          params: {
            path: { projectId },
          },
          body: {
            gauge: req.gauge,
          },
        })
    );
  }

  /**
   * List gauge needles for a project, ordered newest first.
   * @param projectId - The project ID
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.gauges.listGaugeNeedles(123);
   * ```
   */
  async listGaugeNeedles(projectId: number, options?: ListGaugeNeedlesGaugeOptions): Promise<components["schemas"]["ListGaugeNeedlesResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Gauges",
        operation: "ListGaugeNeedles",
        resourceType: "gauge_needle",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/projects/{projectId}/gauge/needles.json", {
          params: {
            path: { projectId },
          },
        })
      , options
    );
  }

  /**
   * Create a gauge needle (progress update) for a project
   * @param projectId - The project ID
   * @param req - Gauge_needle creation parameters
   * @returns The gauge_needle
   * @throws {BasecampError} If required fields are missing or invalid
   *
   * @example
   * ```ts
   * const result = await client.gauges.createGaugeNeedle(123, { gaugeNeedle: { position: 1 } });
   * ```
   */
  async createGaugeNeedle(projectId: number, req: CreateGaugeNeedleGaugeRequest): Promise<components["schemas"]["CreateGaugeNeedleResponseContent"]> {
    if (!req.gaugeNeedle) {
      throw Errors.validation("Gauge needle is required");
    }
    const response = await this.request(
      {
        service: "Gauges",
        operation: "CreateGaugeNeedle",
        resourceType: "gauge_needle",
        isMutation: true,
        projectId,
      },
      () =>
        this.client.POST("/projects/{projectId}/gauge/needles.json", {
          params: {
            path: { projectId },
          },
          body: {
            gauge_needle: req.gaugeNeedle,
            notify: req.notify,
            subscriptions: req.subscriptions,
          },
        })
    );
    return response;
  }

  /**
   * List gauges across all projects the authenticated user has access to.
   * @param options - Optional query parameters
   * @returns All results across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.gauges.listGauges();
   *
   * // With options
   * const filtered = await client.gauges.listGauges({ bucketIds: "example" });
   * ```
   */
  async listGauges(options?: ListGaugesGaugeOptions): Promise<components["schemas"]["ListGaugesResponseContent"]> {
    return this.requestPaginated(
      {
        service: "Gauges",
        operation: "ListGauges",
        resourceType: "gauge",
        isMutation: false,
      },
      () =>
        this.client.GET("/reports/gauges.json", {
          params: {
            query: { "bucket_ids": options?.bucketIds },
          },
        })
      , options
    );
  }
}