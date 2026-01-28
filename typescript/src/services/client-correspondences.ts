/**
 * Client Correspondences service for the Basecamp API.
 *
 * Client correspondences are messages sent to and from clients
 * within a project's client portal.
 *
 * @example
 * ```ts
 * const correspondences = await client.clientCorrespondences.list(projectId);
 * const correspondence = await client.clientCorrespondences.get(projectId, correspondenceId);
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
 * A Basecamp client correspondence.
 */
export interface ClientCorrespondence {
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
  replies_count: number;
  replies_url: string;
  parent?: ParentRef;
  bucket?: BucketRef;
  creator?: PersonRef;
}

// =============================================================================
// Service
// =============================================================================

/**
 * Service for managing client correspondences in Basecamp.
 */
export class ClientCorrespondencesService extends BaseService {
  /**
   * Lists all client correspondences in a project.
   *
   * @param projectId - The project (bucket) ID
   * @returns Array of client correspondences
   *
   * @example
   * ```ts
   * const correspondences = await client.clientCorrespondences.list(projectId);
   * correspondences.forEach(c => console.log(c.subject, c.replies_count));
   * ```
   */
  async list(projectId: number): Promise<ClientCorrespondence[]> {
    const response = await this.request(
      {
        service: "ClientCorrespondences",
        operation: "List",
        resourceType: "client_correspondence",
        isMutation: false,
        projectId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/correspondences.json", {
          params: { path: { projectId } },
        })
    );

    return (response?.correspondences ?? []) as unknown as ClientCorrespondence[];
  }

  /**
   * Gets a client correspondence by ID.
   *
   * @param projectId - The project (bucket) ID
   * @param correspondenceId - The client correspondence ID
   * @returns The client correspondence
   * @throws BasecampError with code "not_found" if correspondence doesn't exist
   *
   * @example
   * ```ts
   * const correspondence = await client.clientCorrespondences.get(projectId, correspondenceId);
   * console.log(correspondence.subject, correspondence.content);
   * ```
   */
  async get(projectId: number, correspondenceId: number): Promise<ClientCorrespondence> {
    const response = await this.request(
      {
        service: "ClientCorrespondences",
        operation: "Get",
        resourceType: "client_correspondence",
        isMutation: false,
        projectId,
        resourceId: correspondenceId,
      },
      () =>
        this.client.GET("/buckets/{projectId}/client/correspondences/{correspondenceId}", {
          params: { path: { projectId, correspondenceId } },
        })
    );

    return response.correspondence as unknown as ClientCorrespondence;
  }
}
