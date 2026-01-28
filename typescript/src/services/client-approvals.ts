/**
 * Client Approvals service for the Basecamp API.
 *
 * Client approvals allow you to request approval from clients
 * on specific deliverables or decisions within a project.
 *
 * @example
 * ```ts
 * const approvals = await client.clientApprovals.list(projectId);
 * const approval = await client.clientApprovals.get(projectId, approvalId);
 * ```
 */

import { BaseService } from "./base.js";

// =============================================================================
// Types
// =============================================================================

/**
 * A person reference (simplified).
 */
export interface PersonRef {
  id: number;
  name: string;
  email_address?: string;
  avatar_url?: string;
  admin?: boolean;
  owner?: boolean;
}

/**
 * A bucket (project) reference.
 */
export interface BucketRef {
  id: number;
  name: string;
  type: string;
}

/**
 * A parent reference.
 */
export interface ParentRef {
  id: number;
  title: string;
  type: string;
  url: string;
  app_url: string;
}

/**
 * A response to a client approval.
 */
export interface ClientApprovalResponse {
  id: number;
  status: string;
  visible_to_clients: boolean;
  created_at: string;
  updated_at: string;
  title: string;
  inherits_status: boolean;
  type: string;
  app_url: string;
  bookmark_url: string;
  content: string;
  approved: boolean;
  parent?: ParentRef;
  bucket?: BucketRef;
  creator?: PersonRef;
}

/**
 * A Basecamp client approval request.
 */
export interface ClientApproval {
  id: number;
  status: string;
  visible_to_clients: boolean;
  created_at: string;
  updated_at: string;
  title: string;
  inherits_status: boolean;
  type: string;
  url: string;
  app_url: string;
  bookmark_url: string;
  subscription_url: string;
  content: string;
  subject: string;
  due_on?: string;
  replies_count: number;
  replies_url: string;
  approval_status: string;
  parent?: ParentRef;
  bucket?: BucketRef;
  creator?: PersonRef;
  approver?: PersonRef;
  responses?: ClientApprovalResponse[];
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing client approvals in Basecamp.
 */
export class ClientApprovalsService extends BaseService {
  /**
   * Lists all client approvals in a project.
   *
   * @param projectId - The project (bucket) ID
   * @returns Array of client approvals
   *
   * @example
   * ```ts
   * const approvals = await client.clientApprovals.list(projectId);
   * approvals.forEach(a => console.log(a.subject, a.approval_status));
   * ```
   */
  async list(projectId: number): Promise<ClientApproval[]> {
    const response = await this.request(
      {
        service: "ClientApprovals",
        operation: "List",
        resourceType: "client_approval",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/approvals.json", {
          params: { path: { projectId } },
        })
    );

    return (response?.approvals ?? []) as unknown as ClientApproval[];
  }

  /**
   * Gets a client approval by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param approvalId - The client approval ID
   * @returns The client approval
   * @throws BasecampError with code "not_found" if approval doesn't exist
   *
   * @example
   * ```ts
   * const approval = await client.clientApprovals.get(projectId, approvalId);
   * console.log(approval.subject, approval.approval_status);
   * console.log(approval.responses?.length, "responses");
   * ```
   */
  async get(projectId: number, approvalId: number): Promise<ClientApproval> {
    const response = await this.request(
      {
        service: "ClientApprovals",
        operation: "Get",
        resourceType: "client_approval",
        isMutation: false,
        projectId,
        resourceId: approvalId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/approvals/{approvalId}", {
          params: { path: { projectId, approvalId } },
        })
    );

    return response.approval as unknown as ClientApproval;
  }
}
