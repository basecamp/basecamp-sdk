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
   * @param options - Optional query parameters
   * @returns All ClientApproval across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.clientApprovals.list();
   * ```
   */
  async list(options?: ListClientApprovalOptions): Promise<ListResult<ClientApproval>> {
    return this.requestPaginated(
      {
        service: "ClientApprovals",
        operation: "ListClientApprovals",
        resourceType: "client_approval",
        isMutation: false,
      },
      () =>
        this.client.GET("/client/approvals.json", {
        })
      , options
    );
  }

  /**
   * Get a single client approval by id
   * @param approvalId - The approval ID
   * @returns The ClientApproval
   * @throws {BasecampError} If the resource is not found
   *
   * @example
   * ```ts
   * const result = await client.clientApprovals.get(123);
   * ```
   */
  async get(approvalId: number): Promise<ClientApproval> {
    const response = await this.request(
      {
        service: "ClientApprovals",
        operation: "GetClientApproval",
        resourceType: "client_approval",
        isMutation: false,
        resourceId: approvalId,
      },
      () =>
        this.client.GET("/client/approvals/{approvalId}", {
          params: {
            path: { approvalId },
          },
        })
    );
    return response;
  }
}