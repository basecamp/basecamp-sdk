/**
 * ClientApprovals service for the Basecamp API.
 *
 * @generated from OpenAPI spec - do not edit directly
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

// =============================================================================
// Types
// =============================================================================

/** ClientApproval entity from the Basecamp API. */
export type ClientApproval = components["schemas"]["ClientApproval"];

// =============================================================================
// Service
// =============================================================================

/**
 * Service for ClientApprovals operations.
 */
export class ClientApprovalsService extends BaseService {

  /**
   * List all client approvals in a project
   * @returns Array of ClientApproval
   *
   * @example
   * ```ts
   * const result = await client.clientApprovals.list();
   * ```
   */
  async list(): Promise<ClientApproval[]> {
    const response = await this.request(
      {
        service: "ClientApprovals",
        operation: "ListClientApprovals",
        resourceType: "client_approval",
        isMutation: false,
      },
      () =>
        this.client.GET("/client/approvals.json", {
        })
    );
    return response ?? [];
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