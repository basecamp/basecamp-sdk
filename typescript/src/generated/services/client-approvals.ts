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
  /** Filter by sort */
  sort?: "created_at" | "updated_at";
  /** Filter by direction */
  direction?: "asc" | "desc";
  /** Page number for paginating through results. Defaults to 1. A positive value selects exactly that page, not a starting offset; see SPEC section 8. */
  page?: number;
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
   * @param bucketId - The bucket ID
   * @param options - Optional query parameters
   * @returns All ClientApproval across all pages, with .meta.totalCount
   *
   * @example
   * ```ts
   * const result = await client.clientApprovals.list(123);
   *
   * // With options
   * const filtered = await client.clientApprovals.list(123, { sort: "created_at" });
   * ```
   */
  async list(bucketId: number, options?: ListClientApprovalOptions): Promise<ListResult<ClientApproval>> {
    return this.requestPaginated(
      {
        service: "ClientApprovals",
        operation: "ListClientApprovals",
        resourceType: "client_approval",
        isMutation: false,
        projectId: bucketId,
      },
      () =>
        this.client.GET("/buckets/{bucketId}/client/approvals.json", {
          params: {
            path: { bucketId },
            query: { sort: options?.sort, direction: options?.direction, page: options?.page },
          },
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