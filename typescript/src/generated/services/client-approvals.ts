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
   * @param projectId - The project ID
   * @returns Array of ClientApproval
   *
   * @example
   * ```ts
   * const result = await client.clientApprovals.list(123);
   * ```
   */
  async list(projectId: number): Promise<ClientApproval[]> {
    const response = await this.request(
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
    );
    return response ?? [];
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