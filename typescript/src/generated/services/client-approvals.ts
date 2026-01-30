/**
 * Service for ClientApprovals operations
 *
 * @generated from OpenAPI spec
 */

import { BaseService } from "../../services/base.js";
import type { components } from "../schema.js";

/**
 * Service for ClientApprovals operations
 */
export class ClientApprovalsService extends BaseService {

  /**
   * List all client approvals in a project
   */
  async list(projectId: number): Promise<components["schemas"]["ListClientApprovalsResponseContent"]> {
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
   */
  async get(projectId: number, approvalId: number): Promise<components["schemas"]["GetClientApprovalResponseContent"]> {
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