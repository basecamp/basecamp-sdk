/**
 * ClientApprovals service for the Basecamp API.
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

/** ClientApproval entity from the Basecamp API. */
export type ClientApproval = components["schemas"]["ClientApproval"];

/**
 * Options for list.
 */
export interface ListClientApprovalOptions extends PaginationOptions {
}


// =============================================================================
// Service
// =============================================================================

/**
 * Service for ClientApprovals operations.
 */
export class ClientApprovalsService extends BaseService {

  /**
   * List all client approvals in a project
   * @param projectId - The project ID
   * @param options - Optional query parameters
   * @returns All ClientApproval across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.clientApprovals.list(123);
   * ```
   */
  async list(projectId: number, options?: ListClientApprovalOptions): Promise<ListResult<ClientApproval>> {
    return this.requestPaginated(
      {
        service: "ClientApprovals",
        operation: "ListClientApprovals",
        resourceType: "client_approval",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/approvals.json", {
          params: {
            path: { projectId },
          },
        })
      , options
    );
  }

  /**
   * Get a single client approval by id
   * @param projectId - The project ID
   * @param approvalId - The approval ID
   * @returns The ClientApproval
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.clientApprovals.get(123, 123);
   * ```
   */
  async get(projectId: number, approvalId: number): Promise<ClientApproval> {
    const response = await this.request(
      {
        service: "ClientApprovals",
        operation: "GetClientApproval",
        resourceType: "client_approval",
        isMutation: false,
        projectId,
        resourceId: approvalId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/approvals/{approvalId}", {
          params: {
            path: { projectId, approvalId },
          },
        })
    );
    return response;
  }
}